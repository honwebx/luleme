package engines

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"luleme/internal/browser"
)

const indexNowEndpoint = "https://api.indexnow.org/IndexNow"

// BingEngine mirrors core/engines/bing.py.
type BingEngine struct {
	mgr     *browser.Manager
	session *browser.Session
	client  *http.Client
	// fetchFn overrides fetch in tests (nil = live browser/HTTP path).
	fetchFn func(string) (string, error)
	// siteCache memoizes successful site: verdicts. The engine is created
	// fresh for each checker.Check run, so the cache is naturally scoped
	// to a single batch — no cross-run staleness. Single-goroutine use.
	siteCache map[string]siteVerdict
}

// siteVerdict is a successful site:domain conclusion. Presence in the map
// implies the search itself was valid; failed searches are never cached.
type siteVerdict struct {
	indexed bool
	// empty is true only for an explicit no-result page. A valid page with
	// results but no domain signal means Bing ignored the operator.
	empty bool
}

// NewBingEngine constructs a BingEngine backed by the given browser manager.
func NewBingEngine(mgr *browser.Manager) *BingEngine {
	return &BingEngine{
		mgr: mgr,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (e *BingEngine) Name() string { return "Bing" }

// Close releases the browser session, if any.
func (e *BingEngine) Close() error {
	if e.session != nil {
		e.session.Close()
		e.session = nil
	}
	return nil
}

// ensureBrowser starts a browser session lazily (selenium-first semantics).
// Returns false (no error) when no browser is available so callers fall back
// to HTTP.
func (e *BingEngine) ensureBrowser() bool {
	if e.session != nil && e.session.Started() {
		return true
	}
	if e.mgr == nil || !e.mgr.Available() {
		return false
	}
	s, err := e.mgr.Start()
	if err != nil {
		return false
	}
	e.session = s
	return true
}

// fetchSearch runs `site:...` via the browser (mirrors _fetch_search, +2s sleep).
func (e *BingEngine) fetchSearch(query string) (string, error) {
	if !e.ensureBrowser() {
		return "", fmt.Errorf("browser unavailable")
	}
	u := "https://www.bing.com/search?q=" + url.QueryEscape(query)
	html, err := e.session.GetPageSource(u, 15*time.Second)
	time.Sleep(2 * time.Second)
	return html, err
}

// fetchSearchHTTP mirrors _fetch_search_http (requests fallback).
func (e *BingEngine) fetchSearchHTTP(query string) (string, error) {
	u := "https://www.bing.com/search?q=" + url.QueryEscape(query)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "+
		"(KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	resp, err := e.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return string(b), err
}

// fetch resolves a query via browser, falling back to HTTP on error (mirrors
// check_indexed's try/except selenium->http behaviour).
func (e *BingEngine) fetch(query string) (string, error) {
	if e.fetchFn != nil {
		return e.fetchFn(query)
	}
	if e.mgr != nil && e.mgr.Available() {
		if html, err := e.fetchSearch(query); err == nil {
			return html, nil
		}
	}
	return e.fetchSearchHTTP(query)
}

// siteIndexed returns the cached-or-live site:domain verdict.
// ok=false means the search itself failed (error/challenge) — nothing cached,
// so a later call retries instead of reusing a failure.
func (e *BingEngine) siteIndexed(domain string) (indexed, empty, ok bool) {
	if v, hit := e.siteCache[domain]; hit {
		return v.indexed, v.empty, true
	}
	html, err := e.fetch("site:" + domain)
	if err != nil || isChallengePage(html) {
		return false, false, false
	}
	indexed = domainInResults(html, domain)
	empty = pageShowsNoResults(html)
	if e.siteCache == nil {
		e.siteCache = map[string]siteVerdict{}
	}
	e.siteCache[domain] = siteVerdict{indexed: indexed, empty: empty}
	return indexed, empty, true
}

// CheckIndexed implements Engine.CheckIndexed (faithful port of check_indexed).
func (e *BingEngine) CheckIndexed(ctx context.Context, raw string) (*bool, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	domain := strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
	path := strings.TrimRight(parsed.Path, "/")
	hasPath := path != "" && path != "/"

	// 0) 精确URL优先：直接搜完整地址（与手动搜索一致），在结果集里严格判，
	// 不依赖 site: 门控。先裸URL（Bing 对粘贴 URL 有导航式优待），再带引号。
	exactOK := false
	for _, q := range []string{raw, `"` + raw + `"`} {
		html, err := e.fetch(q)
		if err != nil || isChallengePage(html) {
			continue
		}
		exactOK = true
		if matchURLInResults(html, domain, path) {
			return boolPtr(true), nil
		}
	}

	// 1) site:{domain} — cached per run (see siteIndexed).
	domainIndexed, siteEmpty, siteOK := e.siteIndexed(domain)

	if !hasPath {
		// homepage: domain indexed = indexed
		if domainIndexed {
			return boolPtr(true), nil
		}
		if siteOK && siteEmpty {
			// explicit no-result page — genuinely not indexed
			return boolPtr(false), nil
		}
		// site: 无效或被 Bing 忽略（有效页却无域名信号）→ 裸域名关键词兜底。
		// 首页做导航类查询基本必回首页，严格匹配命中即收录。
		if html, err := e.fetch(domain); err == nil && !isChallengePage(html) {
			if matchURLInResults(html, domain, "") {
				return boolPtr(true), nil
			}
			// 三种问法（裸URL、引号URL、裸域名）都有效且未命中 → 未收录
			return boolPtr(false), nil
		}
		if !siteOK && !exactOK {
			// no valid search at all — unknown, not "not indexed"
			return nil, fmt.Errorf("Bing 查询受限或失败，请稍后重试")
		}
		return boolPtr(false), nil
	}

	// 整站短路：仅当 site: 明确空结果（b_no）时直接 False。有效页但无域名
	// 信号说明 Bing 忽略了 site: 算子，此时短路会误杀，继续走 specific 搜索。
	if siteOK && siteEmpty {
		return boolPtr(false), nil
	}

	// inner page: verify the specific URL is in Bing's index
	// checkSpecificURL uses quoted path + inurl: with strict path matching
	matched, urlOK := e.checkSpecificURL(e.fetch, domain, path, raw)
	if matched {
		return boolPtr(true), nil
	}

	// direct GET — only confirms NOT indexed (>=400 = page gone)
	req, err := http.NewRequest(http.MethodGet, raw, nil)
	if err == nil {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		resp, err := e.client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 400 {
				return boolPtr(false), nil
			}
		}
	}

	if !siteOK && !urlOK && !exactOK {
		// no Bing search succeeded (blocked/challenged) — unknown, not "not indexed"
		return nil, fmt.Errorf("Bing 查询受限或失败，请稍后重试")
	}
	return boolPtr(false), nil
}

// SubmitURLs implements Engine.SubmitURLs (IndexNow, 10k batches, 429 backoff).
// URLs are grouped by normalized host; each host's list is split into ≤10k
// batches. Returns an error only when the call itself is invalid (empty key);
// per-batch HTTP failures are reflected in the per-host bool instead.
func (e *BingEngine) SubmitURLs(ctx context.Context, urls []string, apiKey string) (map[string]bool, error) {
	result := map[string]bool{}
	if len(urls) == 0 {
		return result, nil
	}
	key := strings.TrimSpace(apiKey)
	if key == "" {
		return result, fmt.Errorf("缺少 IndexNow API Key，请先在设置中配置")
	}
	byHost := map[string][]string{}
	for _, u := range urls {
		host := NormalizeHost(u)
		if host == "" {
			host = hostOf(u)
		}
		byHost[host] = append(byHost[host], u)
	}
	for host, list := range byHost {
		ok := true
		for i := 0; i < len(list); i += 10000 {
			end := i + 10000
			if end > len(list) {
				end = len(list)
			}
			if r := e.SubmitBatch(ctx, host, key, list[i:end]); !r.Ok {
				ok = false
			}
		}
		result[host] = ok
	}
	return result, nil
}

// SubmitBatch POSTs one batch (≤10k URLs), retrying up to 3 times on HTTP 429
// (min(Retry-After, 60s)) and on 502/503/504 (2s, 4s backoff).
func (e *BingEngine) SubmitBatch(ctx context.Context, host, key string, batch []string) BatchResult {
	host = strings.ToLower(strings.TrimSpace(host))
	key = strings.TrimSpace(key)
	if host == "" {
		return BatchResult{Reason: "bad_request", Detail: "URL 缺少有效 host"}
	}
	if key == "" {
		return BatchResult{Reason: "bad_key", Detail: "缺少 IndexNow API Key"}
	}
	if len(batch) == 0 {
		return BatchResult{Ok: true, StatusCode: 200, Reason: "ok"}
	}
	payload := map[string]interface{}{
		"host":        host,
		"key":         key,
		"keyLocation": fmt.Sprintf("https://%s/%s.txt", host, key),
		"urlList":     batch,
	}
	var last BatchResult
	for attempt := 0; attempt < 3; attempt++ {
		select {
		case <-ctx.Done():
			return BatchResult{Reason: "cancelled", Detail: ctx.Err().Error()}
		default:
		}
		ok, status, retryAfter, detail, err := e.postIndexNow(ctx, payload)
		if err != nil {
			last = BatchResult{Reason: "network_error", Detail: truncate(detail, 200)}
			return last
		}
		if ok {
			return BatchResult{Ok: true, StatusCode: status, Reason: "ok"}
		}
		switch status {
		case http.StatusTooManyRequests:
			last = BatchResult{StatusCode: status, Reason: "rate_limited", Detail: truncate(detail, 200)}
			wait := retryAfter
			if wait > 60*time.Second {
				wait = 60 * time.Second
			}
			select {
			case <-ctx.Done():
				return BatchResult{Reason: "cancelled", Detail: ctx.Err().Error()}
			case <-time.After(wait):
			}
			continue
		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			last = BatchResult{StatusCode: status, Reason: "server_error", Detail: truncate(detail, 200)}
			select {
			case <-ctx.Done():
				return BatchResult{Reason: "cancelled", Detail: ctx.Err().Error()}
			case <-time.After(time.Duration(2*(attempt+1)) * time.Second):
			}
			continue
		default:
			return BatchResult{StatusCode: status, Reason: reasonFor(status), Detail: truncate(detail, 200)}
		}
	}
	if last.Reason == "" {
		last = BatchResult{Reason: "rate_limited", Detail: "触发限流，重试耗尽"}
	}
	return last
}

// submitBatch POSTs one batch, retrying up to 3 times on HTTP 429 with backoff
// of min(Retry-After, 60s). Kept for backward compatibility; delegates to SubmitBatch.
func (e *BingEngine) submitBatch(ctx context.Context, host, key string, batch []string) bool {
	return e.SubmitBatch(ctx, host, key, batch).Ok
}

// reasonFor maps an IndexNow HTTP status to a short reason code.
func reasonFor(status int) string {
	switch status {
	case http.StatusForbidden:
		return "bad_key"
	case http.StatusUnprocessableEntity:
		return "mismatch"
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusTooManyRequests:
		return "rate_limited"
	default:
		if status >= 500 {
			return "server_error"
		}
		return fmt.Sprintf("http_%d", status)
	}
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// postIndexNow performs the POST and interprets the response.
// Returns (ok, status, retryAfter, detail, err). retryAfter > 0 only on 429;
// detail is a truncated response body (or error text) for display.
func (e *BingEngine) postIndexNow(ctx context.Context, payload map[string]interface{}) (bool, int, time.Duration, string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return false, 0, 0, err.Error(), err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, indexNowEndpoint, strings.NewReader(string(body)))
	if err != nil {
		return false, 0, 0, err.Error(), err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := e.client.Do(req)
	if err != nil {
		return false, 0, 0, err.Error(), err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	detail := strings.TrimSpace(string(b))

	switch {
	case resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted:
		return true, resp.StatusCode, 0, detail, nil
	case resp.StatusCode == http.StatusTooManyRequests:
		if detail == "" {
			detail = "触发限流，请稍后重试"
		}
		return false, resp.StatusCode, parseRetryAfter(resp.Header.Get("Retry-After")), detail, nil
	default:
		if detail == "" {
			detail = http.StatusText(resp.StatusCode)
		}
		return false, resp.StatusCode, 0, detail, nil
	}
}

func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	// seconds form
	var secs int
	if _, err := fmt.Sscanf(v, "%d", &secs); err == nil {
		return time.Duration(secs) * time.Second
	}
	// HTTP-date form
	t, err := http.ParseTime(v)
	if err != nil {
		return 10 * time.Second // safe default
	}
	d := time.Until(t)
	if d < 0 {
		return 0
	}
	return d
}

func hostOf(raw string) string {
	return NormalizeHost(raw)
}

func boolPtr(b bool) *bool { return &b }
