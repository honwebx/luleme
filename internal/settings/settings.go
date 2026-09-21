package settings

import (
	"encoding/json"
	"os"

	"luleme/internal/config"
)

type Settings struct {
	BingAPIKey string `json:"bing_api_key"`
}

// Load reads settings.json. Returns zero-value Settings when absent or invalid.
func Load() (Settings, error) {
	path, err := config.SettingsFile()
	if err != nil {
		return Settings{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Settings{}, nil
		}
		return Settings{}, err
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return Settings{}, err
	}
	return s, nil
}

// Save writes settings.json atomically within the app data directory.
func Save(s Settings) error {
	path, err := config.SettingsFile()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
