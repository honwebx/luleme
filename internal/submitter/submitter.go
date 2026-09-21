// Package submitter ports app/views/url_submitter.py submission logic,
// with the per-host daily-1000 cache removed (IndexNow has no such limit;
// the batch cap of 10,000 and 429 backoff live in the engine).
package submitter

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"luleme/internal/browser"
	"luleme/internal/engines"
)

type Status string

const (
	Pending Status = "pending"
	Success Status = "success"
	Failed  Status = "failed"
)

// maxBatch is the IndexNow per-request URL cap.
const maxBatch = 10000

type Item struct {
	URL    string `json:"url"`
	Status Status `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type Emitter interface {
	Emit(event string, data any)
}

type Service struct {
	mgr *browser.Manager

	mu   sync.Mutex
	stop chan struct{}
}

func New(mgr *browser.Manager) *Service {
	return &Service{mgr: mgr}
}

func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stop != nil {
		select {
		case <-s.stop:
		default:
			close(s.stop)
		}
	}
}

func (s *Service) stopped() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stop
}

type Start struct {
	URLs []string `json:"urls"`
}
type Progress struct {
	Host    string `json:"host"`
	Done    int    `json:"done"`
	Total   int    `json:"total"`
	Batch   int    `json:"batch,omitempty"`
	Batches int    `json:"batches,omitempty"`
}
type ItemUpdate struct {
	Index  int    `json:"index"`
	Status Status `json:"status"`
	Reason string `json:"reason,omitempty"`
}
type Done struct {
	Success int    `json:"success"`
	Failed  int    `json:"failed"`
	Pending int    `json:"pending"`
	Error   string `json:"error,omitempty"`
}

// Submit dedupes urls, validates http(s) scheme + host, groups by normalized
// host, and submits each host's URLs in ≤10k batches so a single failed batch
// only marks its own slice (not the whole host).
func (s *Service) Submit(ctx context.Context, urls []string, apiKey string, emit Emitter) []Item {
	items := make([]Item, 0, len(urls))
	seen := map[string]bool{}
	for _, u := range urls {
		u = trim(u)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		items = append(items, Item{URL: u, Status: Pending})
	}

	s.mu.Lock()
	s.stop = make(chan struct{})
	s.mu.Unlock()
	stop := s.stopped()

	emit.Emit("submit:start", Start{URLs: urlsOf(items)})

	if strings.TrimSpace(apiKey) == "" {
		for i := range items {
			items[i].Status = Failed
			items[i].Reason = "缺少 API Key，请先在设置中配置"
			emit.Emit("submit:item", ItemUpdate{Index: i, Status: Failed, Reason: items[i].Reason})
		}
		d := count(items)
		d.Error = "缺少 API Key，请先在设置中配置"
		emit.Emit("submit:done", d)
		return items
	}

	eng := engines.Get("bing", s.mgr)
	defer eng.Close()

	// Pre-mark invalid URLs so they never reach IndexNow under host="".
	invalid := map[int]string{}
	byHost := map[string][]int{}
	for i, it := range items {
		if !engines.ValidSubmitURL(it.URL) {
			items[i].Status = Failed
			items[i].Reason = "URL 无效（需 http(s)://域名/路径）"
			invalid[i] = items[i].Reason
			continue
		}
		h := engines.NormalizeHost(it.URL)
		byHost[h] = append(byHost[h], i)
	}
	for idx, reason := range invalid {
		emit.Emit("submit:item", ItemUpdate{Index: idx, Status: Failed, Reason: reason})
	}

	hosts := make([]string, 0, len(byHost))
	for h := range byHost {
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)

	total := len(items)
	done := len(invalid)
	var firstErr string

	for _, host := range hosts {
		idxs := byHost[host]
		batches := (len(idxs) + maxBatch - 1) / maxBatch
		for b := 0; b < batches; b++ {
			select {
			case <-stop:
				d := count(items)
				d.Error = firstErr
				emit.Emit("submit:done", d)
				return items
			case <-ctx.Done():
				d := count(items)
				if firstErr == "" {
					firstErr = "任务已取消"
				}
				d.Error = firstErr
				emit.Emit("submit:done", d)
				return items
			default:
			}
			lo := b * maxBatch
			hi := lo + maxBatch
			if hi > len(idxs) {
				hi = len(idxs)
			}
			chunk := idxs[lo:hi]
			chunkURLs := make([]string, len(chunk))
			for i, idx := range chunk {
				chunkURLs[i] = items[idx].URL
			}
			emit.Emit("submit:progress", Progress{
				Host: host, Done: done + 1, Total: total,
				Batch: b + 1, Batches: batches,
			})

			st := Failed
			reason := ""
			if bs, ok := eng.(engines.BatchSubmitter); ok {
				r := bs.SubmitBatch(ctx, host, apiKey, chunkURLs)
				if r.Ok {
					st = Success
				} else {
					reason = humanReason(r.Reason, r.Detail)
					if firstErr == "" {
						firstErr = host + ": " + reason
					}
				}
			} else if result, err := eng.SubmitURLs(ctx, chunkURLs, apiKey); err == nil {
				if ok, ok2 := result[engines.NormalizeHost(chunkURLs[0])]; ok2 && ok {
					st = Success
				} else {
					reason = "提交失败"
					if firstErr == "" {
						firstErr = host + ": 提交失败"
					}
				}
			} else {
				reason = err.Error()
				if firstErr == "" {
					firstErr = host + ": " + reason
				}
			}
			for _, idx := range chunk {
				items[idx].Status = st
				items[idx].Reason = reason
				emit.Emit("submit:item", ItemUpdate{Index: idx, Status: st, Reason: reason})
			}
			done += len(chunk)
		}
		select {
		case <-stop:
		case <-ctx.Done():
		case <-time.After(500 * time.Millisecond):
		}
	}
	d := count(items)
	d.Error = firstErr
	emit.Emit("submit:done", d)
	return items
}

// humanReason maps an engine reason code + detail to a display string.
func humanReason(reason, detail string) string {
	detail = strings.TrimSpace(detail)
	short := detail
	if len(short) > 120 {
		short = short[:120] + "…"
	}
	switch reason {
	case "ok":
		return ""
	case "bad_key":
		if short != "" {
			return "Key 无效（403）: " + short
		}
		return "Key 无效（403），请检查设置页 Key 与站根 key.txt"
	case "mismatch":
		return "host 与 urlList 不一致（422），请按域名分别提交"
	case "bad_request":
		if short != "" {
			return "请求无效（400）: " + short
		}
		return "请求无效（400）"
	case "rate_limited":
		return "触发限流（429），已按 Retry-After 重试仍失败，稍后重试"
	case "server_error":
		if short != "" {
			return "服务端错误: " + short
		}
		return "服务端错误，稍后重试"
	case "network_error":
		if short != "" {
			return "网络错误: " + short
		}
		return "网络错误"
	case "cancelled":
		return "任务已取消"
	default:
		if reason == "" {
			reason = "提交失败"
		}
		if short != "" {
			return reason + ": " + short
		}
		return reason
	}
}

func groupByHost(items []Item) map[string][]int {
	out := map[string][]int{}
	for i, it := range items {
		h := engines.NormalizeHost(it.URL)
		out[h] = append(out[h], i)
	}
	return out
}

func hostOf(raw string) string {
	return engines.NormalizeHost(raw)
}

func urlsOf(items []Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.URL
	}
	return out
}

func count(items []Item) Done {
	var d Done
	for _, it := range items {
		switch it.Status {
		case Success:
			d.Success++
		case Failed:
			d.Failed++
		case Pending:
			d.Pending++
		}
	}
	return d
}

func trim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\n' || s[0] == '\r' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
