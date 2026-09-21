// Package crawler ports app/views/url_crawler.py crawl logic.
package crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"luleme/internal/browser"
)

const maxPages = 500

type Item struct {
	URL    string `json:"url"`
	Status int    `json:"status"`
}

// Emitter pushes events to the frontend (wired to runtime.EventsEmit by App).
type Emitter interface {
	Emit(event string, data any)
}

type Service struct {
	mgr  *browser.Manager
	stop chan struct{}
}

func New(mgr *browser.Manager) *Service {
	return &Service{mgr: mgr}
}

// Stop signals an in-progress crawl to halt.
func (s *Service) Stop() {
	if s.stop != nil {
		select {
		case <-s.stop:
		default:
			close(s.stop)
		}
	}
}

// Crawl runs the breadth-first crawl, emitting:
//   "crawl:progress" -> string
//   "crawl:item"     -> Item
//   "crawl:done"     -> Done
// It returns the full item list and the valid (200) URL list.
type Done struct {
	Valid int    `json:"valid"`
	Total int    `json:"total"`
	Error string `json:"error"`
}

func (s *Service) Crawl(ctx context.Context, startURL string, emit Emitter) ([]Item, []string, error) {
	startURL = strings.TrimSpace(startURL)
	if !regexp.MustCompile(`^(http|https)://`).MatchString(startURL) {
		startURL = "https://" + startURL
	}
	baseDomain := hostOf(startURL)
	normalized := normalizeURL(startURL)
	if normalized == "" {
		return nil, nil, fmt.Errorf("无效的 URL")
	}

	s.stop = make(chan struct{})
	var items []Item
	var valid []string
	visited := map[string]bool{}
	toVisit := []string{normalized}
	pageCount := 0

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // mirrors allow_redirects=false
		},
	}

	var session *browser.Session
	defer func() {
		if session != nil {
			session.Close()
		}
	}()

	for len(toVisit) > 0 && pageCount < maxPages {
		select {
		case <-s.stop:
			emit.Emit("crawl:done", Done{Valid: len(valid), Total: len(items)})
			return items, valid, nil
		case <-ctx.Done():
			emit.Emit("crawl:done", Done{Valid: len(valid), Total: len(items)})
			return items, valid, ctx.Err()
		default:
		}

		current := toVisit[0]
		toVisit = toVisit[1:]
		if current == "" || visited[current] {
			continue
		}
		if !sameDomain(hostOf(current), baseDomain) {
			continue
		}
		visited[current] = true
		pageCount++

		emit.Emit("crawl:progress", fmt.Sprintf("[%d] 正在请求: %s", pageCount, trunc(current, 60)))

		status, location, body, err := fetchPage(httpClient, current)
		if err != nil {
			items = append(items, Item{URL: current, Status: 0})
			emit.Emit("crawl:item", Item{URL: current, Status: 0})
			continue
		}

		// follow 3xx Location manually (same domain only), mirroring url_crawler.py
		if status >= 300 && status < 400 && location != "" {
			if loc := normalizeURL(resolveURL(current, location)); loc != "" {
				if sameDomain(hostOf(loc), baseDomain) && !visited[loc] {
					toVisit = append(toVisit, loc)
				}
			}
		}

		if status == 200 {
			valid = append(valid, current)
		}
		items = append(items, Item{URL: current, Status: status})
		emit.Emit("crawl:item", Item{URL: current, Status: status})

		if status == 200 && body != "" {
			html := body
			if doc, err := goquery.NewDocumentFromReader(strings.NewReader(html)); err == nil {
				aTags := doc.Find("a[href]")
				if isDynamic(doc, aTags.Length()) && s.mgr != nil && s.mgr.Available() {
					if session == nil {
						if sess, err := s.mgr.Start(); err == nil {
							session = sess
						}
					}
					if session != nil {
						if rendered, err := session.GetPageSource(current, 15*time.Second); err == nil {
							if d, err := goquery.NewDocumentFromReader(strings.NewReader(rendered)); err == nil {
								doc = d
								aTags = d.Find("a[href]")
								emit.Emit("crawl:progress", fmt.Sprintf("[%d] 检测到动态网页，已启用浏览器渲染", pageCount))
							}
						}
					}
				}
				aTags.Each(func(_ int, a *goquery.Selection) {
					href, ok := a.Attr("href")
					if !ok {
						return
					}
					href = strings.TrimSpace(href)
					if href == "" || strings.HasPrefix(href, "#") ||
						strings.HasPrefix(href, "javascript:") || strings.HasPrefix(href, "mailto:") {
						return
					}
					full := resolveURL(current, href)
					if n := normalizeURL(full); n != "" && sameDomain(hostOf(n), baseDomain) && !visited[n] {
						toVisit = append(toVisit, n)
					}
				})
			}
		}
	}

	emit.Emit("crawl:done", Done{Valid: len(valid), Total: len(items)})
	return items, valid, nil
}

// fetchPage GETs url without following redirects, returning status, Location
// header (for 3xx) and body (for 200).
func fetchPage(c *http.Client, url string) (int, string, string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, "", "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	resp, err := c.Do(req)
	if err != nil {
		return 0, "", "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header.Get("Location"), string(b), err
}

var spaIDRe = regexp.MustCompile(`(?i)^(root|app|__next|nuxt-app)$`)

// isDynamic mirrors the SPA/noscript/script-heavy detection in url_crawler.py.
func isDynamic(doc *goquery.Document, aCount int) bool {
	dynamic := false
	doc.Find("[id]").Each(func(_ int, s *goquery.Selection) {
		if dynamic {
			return
		}
		if id, ok := s.Attr("id"); ok && spaIDRe.MatchString(id) {
			dynamic = true
		}
	})
	if dynamic {
		return true
	}
	doc.Find("noscript").Each(func(_ int, s *goquery.Selection) {
		if dynamic {
			return
		}
		if strings.Contains(strings.ToLower(s.Text()), "javascript") {
			dynamic = true
		}
	})
	if dynamic {
		return true
	}
	if aCount < 5 {
		if doc.Find("script[src]").Length() > 0 {
			return true
		}
	}
	return false
}
