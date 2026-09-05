package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const displaySettingsFileName = "blonymonitor-display-settings.json"

type displaySettings struct {
	MaskSelfName bool `json:"maskSelfName"`
}

func displaySettingsPath() string {
	return filepath.Join(ensureUserDataDirectory(), displaySettingsFileName)
}

func loadDisplaySettings() displaySettings {
	data, err := os.ReadFile(displaySettingsPath())
	if err != nil {
		return displaySettings{}
	}

	var settings displaySettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return displaySettings{}
	}
	return settings
}

func saveDisplaySettings(settings displaySettings) error {
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	return os.WriteFile(displaySettingsPath(), data, 0o644)
}

// SetMaskSelfName controls whether the main character name is masked in the status bar.
func (a *App) SetMaskSelfName(enabled bool) error {
	if err := saveDisplaySettings(displaySettings{MaskSelfName: enabled}); err != nil {
		return fmt.Errorf("save display settings: %w", err)
	}
	a.mu.Lock()
	a.maskSelfName = enabled
	a.mu.Unlock()
	return nil
}

// GetMaskSelfName returns whether the main character name is masked.
func (a *App) GetMaskSelfName() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.maskSelfName
}
