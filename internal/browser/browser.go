// Package browser wraps chromedp with the same surface as the original
// core/browser.py (Create / GetPageSource / Quit), plus runtime browser
// detection that prefers Chrome, then Edge, then degrades to HTTP-only.
package browser

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/chromedp/chromedp"
)

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
	"(KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// Detected describes a usable browser on the host.
type Detected struct {
	ExecPath string
	Brand    string
	OK       bool
}

type cand struct{ path, name, brand string }

// Detect looks for Chrome first, then Edge. Returns OK=false when none found.
func Detect() Detected {
	for _, c := range candidates() {
		if c.path != "" {
			if info, err := os.Stat(c.path); err == nil && !info.IsDir() {
				return Detected{ExecPath: c.path, Brand: c.brand, OK: true}
			}
		}
		if c.name != "" {
			if p, err := exec.LookPath(c.name); err == nil {
				return Detected{ExecPath: p, Brand: c.brand, OK: true}
			}
		}
	}
	return Detected{OK: false, Brand: "仅 HTTP 模式"}
}

func candidates() []cand {
	switch runtime.GOOS {
	case "windows":
		progFiles := os.Getenv("ProgramFiles")
		progFiles86 := os.Getenv("ProgramFiles(x86)")
		var out []cand
		// Chrome first
		for _, base := range []string{progFiles, progFiles86} {
			if base == "" {
				continue
			}
			out = append(out, cand{path: filepath.Join(base, "Google", "Chrome", "Application", "chrome.exe"), brand: "Chrome"})
		}
		// Edge
		for _, base := range []string{progFiles86, progFiles} {
			if base == "" {
				continue
			}
			out = append(out, cand{path: filepath.Join(base, "Microsoft", "Edge", "Application", "msedge.exe"), brand: "Edge"})
		}
		return out
	case "darwin":
		return []cand{
			{path: "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", brand: "Chrome"},
			{path: "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge", brand: "Edge"},
			{name: "chromium", brand: "Chromium"},
		}
	default: // linux & others
		return []cand{
			{name: "google-chrome", brand: "Chrome"},
			{name: "google-chrome-stable", brand: "Chrome"},
			{name: "chromium", brand: "Chromium"},
			{name: "chromium-browser", brand: "Chromium"},
			{name: "microsoft-edge", brand: "Edge"},
			{name: "microsoft-edge-stable", brand: "Edge"},
			{name: "msedge", brand: "Edge"},
		}
	}
}

// Session is a single browser instance lifecycle.
type Session struct {
	ctx     context.Context
	cancel  context.CancelFunc
	brand   string
	started bool
}

// Manager owns browser detection results.
type Manager struct {
	detected Detected
}

func NewManager() *Manager {
	return &Manager{detected: Detect()}
}

// Available reports whether a usable browser was found.
func (m *Manager) Available() bool { return m.detected.OK }

// Brand returns the detected browser brand, or the HTTP-only label.
func (m *Manager) Brand() string {
	if m.detected.OK {
		return m.detected.Brand
	}
	return "仅 HTTP 模式"
}

// Start launches a headless browser session. Returns an error (not a crash)
// when no browser is available, so callers can fall back to HTTP.
func (m *Manager) Start() (*Session, error) {
	if !m.detected.OK {
		return nil, errors.New("no usable browser installed")
	}
	opts := append([]chromedp.ExecAllocatorOption{},
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("headless", "new"),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("blink-settings", "imagesEnabled=false"),
		chromedp.UserAgent(userAgent),
		chromedp.ExecPath(m.detected.ExecPath),
	)
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancel2 := chromedp.NewContext(allocCtx)
	// Force the browser to actually start (chromedp lazily starts on first Run).
	if err := chromedp.Run(ctx); err != nil {
		cancel2()
		cancel()
		return nil, err
	}
	return &Session{ctx: ctx, cancel: func() { cancel2(); cancel() }, brand: m.detected.Brand, started: true}, nil
}

// GetPageSource navigates to url and returns the rendered HTML (mirrors
// async_get_page_source). A 15s default is used when timeout <= 0.
func (s *Session) GetPageSource(url string, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	ctx, cancel := context.WithTimeout(s.ctx, timeout)
	defer cancel()
	var html string
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.OuterHTML("html", &html),
	)
	if err != nil {
		return "", err
	}
	return html, nil
}

// Close quits the browser (mirrors async_quit_driver).
func (s *Session) Close() {
	if s.cancel != nil {
		s.cancel()
	}
	s.started = false
}

// Brand returns the session browser brand.
func (s *Session) Brand() string { return s.brand }

// Started reports whether the session is live.
func (s *Session) Started() bool { return s.started }
