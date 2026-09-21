package engines

import "luleme/internal/browser"

// Get is the engine factory (currently ignores name and returns BingEngine,
// matching the original get_engine behaviour; name kept for future extensibility).
func Get(name string, mgr *browser.Manager) Engine {
	_ = name
	return NewBingEngine(mgr)
}
