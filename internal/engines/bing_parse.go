package engines

import (
	"encoding/base64"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// realURL mirrors BingEngine._real_url: decodes Bing's ck redirect wrapper
// `bing.com/ck?...&u=a1<base64url>` back to the real target URL.
func realURL(href string) string {
	if href == "" {
		return ""
	}
	if strings.Contains(href, "bing.com/ck") || strings.HasPrefix(href, "/ck/") {
		// try versioned param first (u=a1...), then any u= param
		for _, re := range []*regexp.Regexp{ckURLParamRe, ckURLParamReLoose} {
			m := re.FindStringSubmatch(href)
			if len(m) >= 2 {
				if s := decodeBingPayload(m[1]); s != "" {
					return s
				}
			}
		}
	}
	return href
}

// decodeBingPayload decodes the base64 payload from Bing's ck redirect.
// The captured value is often percent-encoded, so unescape it first
// (PathUnescape, to avoid '+' -> ' ' conversion), then try base64url
// without padding, then standard base64 with restored padding.
func decodeBingPayload(enc string) string {
	if un, err := url.PathUnescape(enc); err == nil {
		enc = un
	}
	if dec, err := base64.RawURLEncoding.DecodeString(enc); err == nil {
		if s := string(dec); strings.Contains(s, "http") {
			return s[strings.Index(s, "http"):]
		}
	}
	s2 := strings.ReplaceAll(strings.ReplaceAll(enc, "-", "+"), "_", "/")
	if m := len(s2) % 4; m != 0 {
		s2 += strings.Repeat("=", 4-m)
	}
	if dec, err := base64.StdEncoding.DecodeString(s2); err == nil {
		if s := string(dec); strings.Contains(s, "http") {
			return s[strings.Index(s, "http"):]
		}
	}
	return ""
}

var ckURLParamRe = regexp.MustCompile(`[?&]u=a1([^&]+)`)
var ckURLParamReLoose = regexp.MustCompile(`[?&]u=([^&]+)`)

var noResultPhrases = []string{
	"没有与此相关的结果", "没有找到", "There are no results",
	"No results", "didn't return any results",
}

// pageShowsNoResults reports whether html is an explicit empty-result page
// (li.b_no h1 carrying a no-result phrase). A page WITH results but no
// domain signal means Bing ignored the site: operator — that is NOT a
// negative verdict, only this explicit-empty signal is.
func pageShowsNoResults(html string) bool {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return false
	}
	results := doc.Find("#b_results")
	if results.Length() == 0 {
		results = doc.Selection
	}
	bNo := results.Find("li.b_no h1").First()
	if bNo.Length() == 0 {
		return false
	}
	t := bNo.Text()
	for _, p := range noResultPhrases {
		if strings.Contains(t, p) {
			return true
		}
	}
	return false
}

// domainInResults mirrors BingEngine._domain_in_results(html, domain).
// Only checks if the domain appears in results — use for homepage detection.
// For inner pages, checkSpecificURL does strict URL matching instead.
func domainInResults(html string, domain string) bool {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return false
	}
	results := doc.Find("#b_results")
	if results.Length() == 0 {
		results = doc.Selection
	}

	// b_no h1 "no results" guard -> false
	if bNo := results.Find("li.b_no h1").First(); bNo.Length() > 0 {
		t := bNo.Text()
		for _, p := range noResultPhrases {
			if strings.Contains(t, p) {
				return false
			}
		}
	}

	// anchors: real URL host matches
	match := false
	results.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		if match {
			return
		}
		href, _ := s.Attr("href")
		host := hostWithoutWWW(realURL(href))
		if host == domain || strings.HasSuffix(host, "."+domain) {
			match = true
		}
	})
	if match {
		return true
	}

	// cite text contains domain
	results.Find("cite").Each(func(_ int, s *goquery.Selection) {
		if match {
			return
		}
		if strings.Contains(s.Text(), domain) {
			match = true
		}
	})
	if match {
		return true
	}

	// b_algo fallback
	bAlgo := results.Find("li.b_algo")
	if bAlgo.Length() == 0 {
		return false
	}
	bAlgo.Each(func(_ int, s *goquery.Selection) {
		if match {
			return
		}
		if strings.Contains(s.Text(), domain) {
			match = true
		}
	})
	return match
}

// checkSpecificURL searches for the page on Bing and strictly matches
// result URLs against the target path. Tries the original quoted-path
// query first, then inurl: for broader coverage. Both use the same
// strict matcher below, so precision is preserved either way.
func (e *BingEngine) checkSpecificURL(fetch func(string) (string, error), domain, path, _ string) (bool, bool) {
	seg := lastPathSegment(path)
	if seg == "" {
		return false, false
	}
	queries := []string{
		`site:` + domain + ` "` + path + `"`,
		"site:" + domain + " inurl:" + seg,
	}
	searched := false
	for _, q := range queries {
		html, err := fetch(q)
		if err != nil || isChallengePage(html) {
			continue
		}
		searched = true
		if matchURLInResults(html, domain, path) {
			return true, true
		}
	}
	return false, searched
}

// isChallengePage reports whether Bing returned a bot-check/captcha page
// instead of search results (automated traffic is often challenged).
func isChallengePage(html string) bool {
	if html == "" {
		return true
	}
	lower := strings.ToLower(html)
	// NOTE: do NOT include "/fd/ls/lsp.aspx" here — it is Bing's normal
	// client-logging endpoint present in every regular SERP (verified
	// against live HTML), and would flag all pages as challenged.
	markers := []string{
		"unusual traffic",
		"captcha",
		"verify you are a human",
		"verify that you are human",
		"not a robot",
		"automated requests",
	}
	for _, m := range markers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

// matchURLInResults parses Bing result HTML and reports whether any result
// URL matches the target domain + path (exact or subpage). Strict: no
// content/snippet matching, URL only.
func matchURLInResults(html, domain, path string) bool {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return false
	}
	pathLower := strings.ToLower(path)
	domainLower := strings.ToLower(domain)
	match := false

	doc.Find("#b_results li.b_algo").Each(func(_ int, s *goquery.Selection) {
		if match {
			return
		}
		// Check h2 a href: decode real URL, match domain + path strictly
		h2a := s.Find("h2 a").First()
		if h2a.Length() > 0 {
			href, _ := h2a.Attr("href")
			real := realURL(href)
			hrefDomain := hostWithoutWWW(real)
			if hrefDomain == domain || strings.HasSuffix(hrefDomain, "."+domain) {
				hrefPath := strings.ToLower(strings.TrimRight(pathOf(real), "/"))
				// strict: exact match or is a subpage of path
				if hrefPath == pathLower || strings.HasPrefix(hrefPath, pathLower+"/") {
					match = true
				}
			}
		}
		// Check cite: parse as URL and compare path strictly
		if !match {
			if cite := s.Find("cite").First(); cite.Length() > 0 {
				citeText := cite.Text()
				citeURL := strings.ReplaceAll(citeText, " › ", "/")
				citeURL = strings.ReplaceAll(citeURL, " \u203a ", "/")
				citeLower := strings.ToLower(citeURL)
				// must contain domain
				if strings.Contains(citeLower, domainLower) {
					// parse cite as URL to extract and compare path
					citeForParse := citeURL
					if !strings.HasPrefix(strings.ToLower(citeForParse), "http") {
						citeForParse = "https://" + citeForParse
					}
					if u, perr := url.Parse(citeForParse); perr == nil {
						citePath := strings.ToLower(strings.TrimRight(u.Path, "/"))
						if citePath == pathLower || strings.HasPrefix(citePath, pathLower+"/") {
							match = true
						}
					}
				}
			}
		}
	})
	return match
}
