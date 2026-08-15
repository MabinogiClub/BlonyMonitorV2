package app

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const farmSettingsFileName = "blonymonitor-farm-settings.json"

type farmSettings struct {
	Enabled                    bool `json:"enabled"`
	ReadyNotificationEnabled   bool `json:"readyNotificationEnabled"`
	SpecialNotificationEnabled bool `json:"specialNotificationEnabled"`
}

func defaultFarmSettings() farmSettings {
	return farmSettings{
		ReadyNotificationEnabled:   true,
		SpecialNotificationEnabled: true,
	}
}

func farmSettingsPath() string {
	return filepath.Join(analysisLogDirectory(), farmSettingsFileName)
}

func loadFarmSettings() farmSettings {
	settings := defaultFarmSettings()
	data, err := os.ReadFile(farmSettingsPath())
	if err != nil || json.Unmarshal(data, &settings) != nil {
		return defaultFarmSettings()
	}
	return settings
}

func saveFarmSettings(settings farmSettings) error {
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	return os.WriteFile(farmSettingsPath(), data, 0o644)
}
