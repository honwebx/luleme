// Package checker ports app/views/url_checker.py query logic.
package checker

import (
	"context"
	"time"

	"luleme/internal/browser"
	"luleme/internal/engines"
)

type Status string

const (
	Pending    Status = "pending"
	Indexed    Status = "indexed"
	NotIndexed Status = "not_indexed"
	Error      Status = "error"
)

type Item struct {
	URL    string `json:"url"`
	Status Status `json:"status"`
}

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

func (s *Service) Stop() {
	if s.stop != nil {
		select {
		case <-s.stop:
		default:
			close(s.stop)
		}
	}
}

type Start struct {
	URLs []string `json:"urls"`
}
type Progress struct {
	Index int    `json:"index"`
	Total int    `json:"total"`
	URL   string `json:"url"`
}
type ItemUpdate struct {
	Index  int    `json:"index"`
	Status Status `json:"status"`
}
type Done struct {
	Indexed    int `json:"indexed"`
	NotIndexed int `json:"notIndexed"`
}

// Check dedupes urls, emits the ordered list, then checks each pending URL
// with a 1s spacing (mirrors url_checker.py's asyncio.sleep(1)).
func (s *Service) Check(ctx context.Context, urls []string, emit Emitter) []Item {
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
	s.stop = make(chan struct{})

	emit.Emit("check:start", Start{URLs: urlsOf(items)})

	eng := engines.Get("bing", s.mgr)
	defer eng.Close()

	pending := pendingIndices(items)
	for i, idx := range pending {
		select {
		case <-s.stop:
			emit.Emit("check:done", count(items))
			return items
		case <-ctx.Done():
			emit.Emit("check:done", count(items))
			return items
		default:
		}
		url := items[idx].URL
		emit.Emit("check:progress", Progress{Index: i + 1, Total: len(pending), URL: url})

		st := Error
		if res, err := eng.CheckIndexed(ctx, url); err == nil && res != nil {
			if *res {
				st = Indexed
			} else {
				st = NotIndexed
			}
		}
		items[idx].Status = st
		emit.Emit("check:item", ItemUpdate{Index: idx, Status: st})

		select {
		case <-s.stop:
		case <-ctx.Done():
		case <-time.After(1 * time.Second):
		}
	}
	emit.Emit("check:done", count(items))
	return items
}

func urlsOf(items []Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.URL
	}
	return out
}

func pendingIndices(items []Item) []int {
	var out []int
	for i, it := range items {
		if it.Status == Pending {
			out = append(out, i)
		}
	}
	return out
}

func count(items []Item) Done {
	var d Done
	for _, it := range items {
		switch it.Status {
		case Indexed:
			d.Indexed++
		case NotIndexed:
			d.NotIndexed++
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
