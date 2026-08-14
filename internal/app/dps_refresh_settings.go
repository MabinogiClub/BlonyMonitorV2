package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	defaultBackendRefreshIntervalMS  int64 = 100
	defaultFrontendRefreshIntervalMS int64 = 200
	minRefreshIntervalMS             int64 = 50
	maxRefreshIntervalMS             int64 = 15_000
	dpsRefreshSettingsFileName             = "blonymonitor-refresh-settings.json"
)

type DPSRefreshSettings struct {
	BackendIntervalMS  int64 `json:"backendIntervalMs"`
	FrontendIntervalMS int64 `json:"frontendIntervalMs"`
}

func defaultDPSRefreshSettings() DPSRefreshSettings {
	return DPSRefreshSettings{
		BackendIntervalMS:  defaultBackendRefreshIntervalMS,
		FrontendIntervalMS: defaultFrontendRefreshIntervalMS,
	}
}

func dpsRefreshSettingsPath() string {
	return filepath.Join(analysisLogDirectory(), dpsRefreshSettingsFileName)
}

func validateDPSRefreshSettings(settings DPSRefreshSettings) error {
	if settings.BackendIntervalMS < minRefreshIntervalMS || settings.BackendIntervalMS > maxRefreshIntervalMS {
		return fmt.Errorf("backend refresh interval must be between %d and %d ms", minRefreshIntervalMS, maxRefreshIntervalMS)
	}
	if settings.FrontendIntervalMS < minRefreshIntervalMS || settings.FrontendIntervalMS > maxRefreshIntervalMS {
		return fmt.Errorf("frontend refresh interval must be between %d and %d ms", minRefreshIntervalMS, maxRefreshIntervalMS)
	}
	return nil
}

func loadDPSRefreshSettings() DPSRefreshSettings {
	settings := defaultDPSRefreshSettings()
	data, err := os.ReadFile(dpsRefreshSettingsPath())
	if err != nil || json.Unmarshal(data, &settings) != nil || validateDPSRefreshSettings(settings) != nil {
		return defaultDPSRefreshSettings()
	}
	return settings
}

func saveDPSRefreshSettings(settings DPSRefreshSettings) error {
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	return os.WriteFile(dpsRefreshSettingsPath(), data, 0o644)
}

func (a *App) GetDPSRefreshSettings() DPSRefreshSettings {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.dpsRefreshSettings
}

func (a *App) SetDPSRefreshSettings(settings DPSRefreshSettings) (DPSRefreshSettings, error) {
	if err := validateDPSRefreshSettings(settings); err != nil {
		return a.GetDPSRefreshSettings(), err
	}
	if err := saveDPSRefreshSettings(settings); err != nil {
		return a.GetDPSRefreshSettings(), err
	}

	a.mu.Lock()
	a.dpsRefreshSettings = settings
	throttler := a.dpsUpdateThrottler
	a.mu.Unlock()
	if throttler != nil {
		throttler.SetMinInterval(time.Duration(settings.BackendIntervalMS) * time.Millisecond)
		if a.ctx != nil {
			throttler.RequestUpdate()
		}
	}
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "dps-refresh-settings-changed", settings)
	}
	return settings, nil
}
