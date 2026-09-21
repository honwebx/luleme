package main

import (
	"context"
	"log"
	"strings"
	"sync"

	"luleme/internal/browser"
	"luleme/internal/checker"
	"luleme/internal/crawler"
	"luleme/internal/config"
	"luleme/internal/settings"
	"luleme/internal/submitter"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx          context.Context
	browserMgr   *browser.Manager
	crawlerSvc   *crawler.Service
	crawling     bool
	lastItems    []crawler.Item
	lastValid    []string
	checkerSvc   *checker.Service
	checking     bool
	lastChecked  []checker.Item
	submitterSvc *submitter.Service
	mu           sync.Mutex
	submitting   bool
	submitCancel context.CancelFunc
	lastSubmitted []submitter.Item
}

func NewApp() *App {
	mgr := browser.NewManager()
	return &App{
		browserMgr:   mgr,
		crawlerSvc:   crawler.New(mgr),
		checkerSvc:   checker.New(mgr),
		submitterSvc: submitter.New(mgr),
	}
}

func (a *App) OnStartup(ctx context.Context) {
	a.ctx = ctx
}

// Version returns the app version.
func (a *App) Version() string {
	return "1.0.0"
}

// BrowserMode returns a human label describing the current browser/HTTP mode.
func (a *App) BrowserMode() string {
	return a.browserMgr.Brand()
}

// GetApiKey returns the stored IndexNow API key (empty if none).
func (a *App) GetApiKey() string {
	s, err := settings.Load()
	if err != nil {
		log.Printf("load settings: %v", err)
		return ""
	}
	return strings.TrimSpace(s.BingAPIKey)
}

// validAPIKey reports whether key looks like an IndexNow key (8-128 chars,
// letters/digits/hyphen/underscore; Bing issues UUID-style keys).
func validAPIKey(key string) bool {
	if len(key) < 8 || len(key) > 128 {
		return false
	}
	for _, r := range key {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

// SaveApiKey persists the IndexNow API key. Returns an error message on failure.
func (a *App) SaveApiKey(key string) string {
	key = strings.TrimSpace(key)
	if key != "" && !validAPIKey(key) {
		return "API Key 格式无效（需 8-128 位字母/数字/-/下划线）"
	}
	if err := settings.Save(settings.Settings{BingAPIKey: key}); err != nil {
		return err.Error()
	}
	return ""
}

// DataDir returns the absolute path to the app data directory (useful for diagnostics).
func (a *App) DataDir() string {
	dir, err := config.DataDir()
	if err != nil {
		return ""
	}
	return dir
}

// ---- Crawler ----

// wailsEmitter adapts the wails event emitter to crawler.Emitter.
type wailsEmitter struct{ ctx context.Context }

func (w wailsEmitter) Emit(event string, data any) {
	wailsruntime.EventsEmit(w.ctx, event, data)
}

// StartCrawl launches a crawl goroutine. Returns "" on success or an error msg.
func (a *App) StartCrawl(rawURL string) string {
	if a.crawling {
		return "已有爬取任务在进行中"
	}
	if rawURL == "" {
		return "请输入网站URL"
	}
	a.crawling = true
	go func() {
		defer func() { a.crawling = false }()
		items, valid, err := a.crawlerSvc.Crawl(a.ctx, rawURL, wailsEmitter{a.ctx})
		a.lastItems = items
		a.lastValid = valid
		if err != nil {
			wailsruntime.EventsEmit(a.ctx, "crawl:error", err.Error())
		}
	}()
	return ""
}

// StopCrawl halts an in-progress crawl.
func (a *App) StopCrawl() {
	a.crawlerSvc.Stop()
}

// ExportCrawl opens a save dialog and writes crawl results to xlsx.
// Returns "" on success or an error message.
func (a *App) ExportCrawl() string {
	if len(a.lastItems) == 0 {
		return "没有可导出的结果"
	}
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		DefaultFilename: "crawl_results.xlsx",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Excel (*.xlsx)", Pattern: "*.xlsx"},
		},
	})
	if err != nil || path == "" {
		return ""
	}
	if err := crawler.ExportXlsx(a.lastItems, path); err != nil {
		return err.Error()
	}
	return ""
}

// ---- Checker ----

// StartCheck launches the index-checking loop for the given URLs.
func (a *App) StartCheck(urls []string) string {
	if a.checking {
		return "已有查询任务在进行中"
	}
	if len(urls) == 0 {
		return "请先添加待查询URL"
	}
	a.checking = true
	go func() {
		defer func() { a.checking = false }()
		items := a.checkerSvc.Check(a.ctx, urls, wailsEmitter{a.ctx})
		a.lastChecked = items
	}()
	return ""
}

// StopCheck halts an in-progress check.
func (a *App) StopCheck() {
	a.checkerSvc.Stop()
}

// ImportCheckUrls opens a file dialog and imports URLs from txt/xlsx.
// Returns the imported URL list (empty on cancel).
func (a *App) ImportCheckUrls() []string {
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "导入URL",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "URL 文件 (*.txt;*.xlsx)", Pattern: "*.txt;*.xlsx"},
		},
	})
	if err != nil || path == "" {
		return nil
	}
	urls, err := checker.ImportUrls(path)
	if err != nil {
		wailsruntime.EventsEmit(a.ctx, "check:error", err.Error())
		return nil
	}
	return urls
}

// ExportCheck opens a save dialog and writes check results to xlsx.
func (a *App) ExportCheck() string {
	if len(a.lastChecked) == 0 {
		return "没有可导出的结果"
	}
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		DefaultFilename: "bing_check_results.xlsx",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Excel (*.xlsx)", Pattern: "*.xlsx"},
		},
	})
	if err != nil || path == "" {
		return ""
	}
	if err := checker.ExportXlsx(a.lastChecked, path); err != nil {
		return err.Error()
	}
	return ""
}

// ---- Submitter ----

// StartSubmit launches the IndexNow submission loop for the given URLs.
func (a *App) StartSubmit(urls []string) string {
	a.mu.Lock()
	if a.submitting {
		a.mu.Unlock()
		return "已有提交任务在进行中"
	}
	apiKey := a.GetApiKey()
	if apiKey == "" {
		a.mu.Unlock()
		return "请先在设置中配置 Bing API Key"
	}
	if len(urls) == 0 {
		a.mu.Unlock()
		return "没有待提交的URL"
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.submitCancel = cancel
	a.submitting = true
	a.mu.Unlock()
	go func() {
		defer func() {
			a.mu.Lock()
			a.submitting = false
			a.submitCancel = nil
			a.mu.Unlock()
		}()
		items := a.submitterSvc.Submit(ctx, urls, apiKey, wailsEmitter{a.ctx})
		a.mu.Lock()
		a.lastSubmitted = items
		a.mu.Unlock()
	}()
	return ""
}

// StopSubmit halts an in-progress submission.
func (a *App) StopSubmit() {
	a.mu.Lock()
	cancel := a.submitCancel
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	a.submitterSvc.Stop()
}

// ExportSubmit opens a save dialog and writes submit results to xlsx.
func (a *App) ExportSubmit() string {
	a.mu.Lock()
	items := a.lastSubmitted
	a.mu.Unlock()
	if len(items) == 0 {
		return "没有可导出的结果"
	}
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		DefaultFilename: "indexnow_submit_results.xlsx",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Excel (*.xlsx)", Pattern: "*.xlsx"},
		},
	})
	if err != nil || path == "" {
		return ""
	}
	if err := submitter.ExportXlsx(items, path); err != nil {
		return err.Error()
	}
	return ""
}

// ImportSubmitUrls opens a file dialog and imports URLs from txt/xlsx.
func (a *App) ImportSubmitUrls() []string {
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "导入URL",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "URL 文件 (*.txt;*.xlsx)", Pattern: "*.txt;*.xlsx"},
		},
	})
	if err != nil || path == "" {
		return nil
	}
	urls, err := submitter.ImportUrls(path)
	if err != nil {
		wailsruntime.EventsEmit(a.ctx, "submit:error", err.Error())
		return nil
	}
	return urls
}
