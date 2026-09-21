package crawler

import (
	"net/url"
	"strings"
)

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Host
}

// sameDomain compares netlocs after stripping port and leading "www.".
func sameDomain(a, b string) bool {
	return normDomain(a) == normDomain(b)
}

func normDomain(h string) string {
	h = strings.SplitN(h, ":", 2)[0]
	h = strings.TrimPrefix(h, "www.")
	return h
}

// resolveURL joins a (possibly relative) href against a base URL.
func resolveURL(base, href string) string {
	b, err := url.Parse(base)
	if err != nil {
		return href
	}
	r, err := url.Parse(href)
	if err != nil {
		return href
	}
	return b.ResolveReference(r).String()
}

// normalizeURL mirrors _normalize_url: scheme://host/path(?query), no fragment.
func normalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	path := u.Path
	if path == "" {
		path = "/"
	}
	u.Path = path
	u.Fragment = ""
	return u.String()
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
