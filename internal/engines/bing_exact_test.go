package engines

import (
	"context"
	"net/http"
	"reflect"
	"testing"
)

const emptyResultsHTML = `<ol id="b_results"><li class="b_no"><h1>No results for xxx</h1></li></ol>`

// stubEngine 返回 fetch 可注入的引擎，并记录所有查询词。
func stubEngine(calls *[]string, byQuery map[string]string) *BingEngine {
	e := NewBingEngine(nil)
	e.fetchFn = func(q string) (string, error) {
		*calls = append(*calls, q)
		if html, ok := byQuery[q]; ok {
			return html, nil
		}
		return emptyResultsHTML, nil
	}
	return e
}

// 精确URL匹配：目标出现在第 N 条结果也必须命中。
func TestMatchURLInResultsNthHit(t *testing.T) {
	html := `<ol id="b_results">
<li class="b_algo"><h2><a href="https://other.com/x">other</a></h2><cite>other.com</cite></li>
<li class="b_algo"><h2><a href="https://wzfou.com/other-page/">other</a></h2><cite>wzfou.com › other-page</cite></li>
<li class="b_algo"><h2><a href="https://wzfou.com/2022-vps/">2022 vps</a></h2><cite>wzfou.com › 2022-vps</cite></li>
</ol>`
	if !matchURLInResults(html, "wzfou.com", "/2022-vps") {
		t.Fatal("expected 3rd result to match")
	}
}

func TestMatchURLInResultsCiteHit(t *testing.T) {
	html := `<ol id="b_results"><li class="b_algo"><h2><a href="https://wzfou.com/vps-paolu/">t</a></h2><cite>wzfou.com › vps-paolu</cite></li></ol>`
	if !matchURLInResults(html, "wzfou.com", "/vps-paolu") {
		t.Fatal("expected cite to match")
	}
}

func TestMatchURLInResultsNoResults(t *testing.T) {
	html := `<ol id="b_results"><li class="b_no"><h1>No results for xxx</h1></li></ol>`
	if matchURLInResults(html, "wzfou.com", "/2022-vps") {
		t.Fatal("expected no match on empty results")
	}
}

func TestMatchURLInResultsMiss(t *testing.T) {
	html := `<ol id="b_results"><li class="b_algo"><h2><a href="https://wzfou.com/unrelated/">u</a></h2><cite>wzfou.com › unrelated</cite></li></ol>`
	if matchURLInResults(html, "wzfou.com", "/2022-vps") {
		t.Fatal("expected no match when target absent")
	}
}

func TestMatchURLInResultsHomepage(t *testing.T) {
	html := `<ol id="b_results"><li class="b_algo"><h2><a href="https://wzfou.com/">home</a></h2><cite>wzfou.com</cite></li></ol>`
	if !matchURLInResults(html, "wzfou.com", "") {
		t.Fatal("expected homepage root link to match")
	}
}

const garbageResultsHTML = `<ol id="b_results"><li class="b_algo"><h2><a href="https://other.com/x">o</a></h2><cite>other.com</cite></li></ol>`

const homepageHitHTML = `<ol id="b_results"><li class="b_algo"><h2><a href="https://example.invalid/">home</a></h2><cite>example.invalid</cite></li></ol>`

// 首页兜底：site: 被忽略（有效页、无域名信号、无 b_no）时，裸域名关键词
// 搜到首页即判收录。
func TestCheckIndexedHomepageKeywordFallback(t *testing.T) {
	var calls []string
	e := stubEngine(&calls, map[string]string{
		"site:example.invalid": garbageResultsHTML,
		"example.invalid":      homepageHitHTML,
	})
	raw := "https://example.invalid/"
	res, err := e.CheckIndexed(context.Background(), raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res == nil || !*res {
		t.Fatal("expected true via keyword fallback")
	}
	want := []string{raw, `"` + raw + `"`, "site:example.invalid", "example.invalid"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("expected queries %v, got %v", want, calls)
	}
}

// 首页明确空结果（b_no）→ 直接 False，不花关键词查询。
func TestCheckIndexedHomepageExplicitEmpty(t *testing.T) {
	var calls []string
	e := stubEngine(&calls, nil)
	raw := "https://example.invalid/"
	res, err := e.CheckIndexed(context.Background(), raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res == nil || *res {
		t.Fatal("expected false for explicitly empty site:")
	}
	want := []string{raw, `"` + raw + `"`, "site:example.invalid"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("expected queries %v, got %v", want, calls)
	}
}

// 首页垃圾页 + 关键词有效未命中 → False。
func TestCheckIndexedHomepageGarbageThenMiss(t *testing.T) {
	var calls []string
	e := stubEngine(&calls, map[string]string{
		"site:example.invalid": garbageResultsHTML,
	})
	raw := "https://example.invalid/"
	res, err := e.CheckIndexed(context.Background(), raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res == nil || *res {
		t.Fatal("expected false when keyword search validly misses")
	}
	if len(calls) != 4 {
		t.Fatalf("expected 4 queries, got %v", calls)
	}
}

// 内页垃圾页不短路：specific 搜索照常执行并可命中。
func TestCheckIndexedInnerGarbageNoShortCircuit(t *testing.T) {
	var calls []string
	e := stubEngine(&calls, map[string]string{
		"site:example.invalid":              garbageResultsHTML,
		`site:example.invalid "/some/page"`: `<ol id="b_results"><li class="b_algo"><h2><a href="https://example.invalid/some/page">p</a></h2><cite>example.invalid › some › page</cite></li></ol>`,
	})
	raw := "https://example.invalid/some/page"
	res, err := e.CheckIndexed(context.Background(), raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res == nil || !*res {
		t.Fatal("expected true via specific search despite garbage site: page")
	}
}

// 整站短路：site: 有效且整站无收录时，内页直接 False，
// 且只发 3 次搜索（裸URL、带引号URL、site:），不做 specific 搜索。
func TestCheckIndexedSiteShortCircuit(t *testing.T) {
	var calls []string
	e := stubEngine(&calls, nil)
	raw := "https://example.invalid/some/page"
	res, err := e.CheckIndexed(context.Background(), raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res == nil || *res {
		t.Fatal("expected false for unindexed site")
	}
	want := []string{raw, `"` + raw + `"`, "site:example.invalid"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("expected queries %v, got %v", want, calls)
	}
}

// 同批次同域名只查一次 site:，第二次走缓存。
func TestCheckIndexedSiteCache(t *testing.T) {
	var calls []string
	e := stubEngine(&calls, nil)
	for _, raw := range []string{
		"https://example.invalid/a",
		"https://example.invalid/b",
	} {
		res, err := e.CheckIndexed(context.Background(), raw)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if res == nil || *res {
			t.Fatal("expected false for unindexed site")
		}
	}
	siteCount := 0
	for _, q := range calls {
		if q == "site:example.invalid" {
			siteCount++
		}
	}
	if siteCount != 1 {
		t.Fatalf("expected 1 site: query, got %d (%v)", siteCount, calls)
	}
	if len(calls) != 5 {
		t.Fatalf("expected 5 total queries, got %d (%v)", len(calls), calls)
	}
}

// site: 搜索失败（反爬）不缓存：下次调用必须重试，而不是复用失败结论。
func TestCheckIndexedSiteFailureNotCached(t *testing.T) {
	challenge := `<html><body>unusual traffic was detected, verify you are a human</body></html>`
	var calls []string
	e := NewBingEngine(nil)
	e.fetchFn = func(q string) (string, error) {
		calls = append(calls, q)
		return challenge, nil // 所有搜索一律被反爬
	}
	// 直连客户端：example.invalid DNS 必失败，直达请求被跳过，测试不依赖外部网络/代理。
	e.client = &http.Client{}
	raw := "https://example.invalid/a"
	if _, err := e.CheckIndexed(context.Background(), raw); err == nil {
		t.Fatal("expected unknown error when all searches challenged")
	}
	if _, err := e.CheckIndexed(context.Background(), raw); err == nil {
		t.Fatal("expected unknown error on retry")
	}
	siteCount := 0
	for _, q := range calls {
		if q == "site:example.invalid" {
			siteCount++
		}
	}
	if siteCount != 2 {
		t.Fatalf("expected site: retried (2 queries), got %d (%v)", siteCount, calls)
	}
}
