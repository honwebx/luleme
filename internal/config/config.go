package config

import (
	"os"
	"path/filepath"
)

const AppName = "录了么"

const IndexNowEndpoint = "https://api.indexnow.org/IndexNow"

// DataDir returns the per-platform data directory for this app.
// os.UserConfigDir already returns:
//   - Windows: %AppData% (AppData\Roaming)
//   - macOS:   ~/Library/Application Support
//   - Linux:   $XDG_CONFIG_HOME or ~/.config
// which matches the original config.py behaviour exactly.
func DataDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, AppName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// SettingsFile returns the absolute path to settings.json.
func SettingsFile() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}
