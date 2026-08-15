package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	defaultSoundVolume    = 100
	soundSettingsFileName = "blonymonitor-sound-settings.json"
)

type soundSettings struct {
	Volume int `json:"volume"`
}

func soundSettingsPath() string {
	return filepath.Join(analysisLogDirectory(), soundSettingsFileName)
}

func normalizeSoundVolume(volume int) int {
	if volume < 0 {
		return 0
	}
	if volume > 100 {
		return 100
	}
	return volume
}

func loadSoundVolume() int {
	data, err := os.ReadFile(soundSettingsPath())
	if err != nil {
		return defaultSoundVolume
	}
	var settings soundSettings
	if err := json.Unmarshal(data, &settings); err != nil || settings.Volume < 0 || settings.Volume > 100 {
		return defaultSoundVolume
	}
	return settings.Volume
}

func saveSoundVolume(volume int) error {
	data, err := json.Marshal(soundSettings{Volume: volume})
	if err != nil {
		return err
	}
	return os.WriteFile(soundSettingsPath(), data, 0o644)
}

// SetSoundVolume sets the volume for Buff and farm reminder sounds.
func (a *App) SetSoundVolume(volume int) error {
	volume = normalizeSoundVolume(volume)
	if err := saveSoundVolume(volume); err != nil {
		return fmt.Errorf("save sound volume: %w", err)
	}
	a.mu.Lock()
	a.soundVolume = volume
	a.mu.Unlock()
	return nil
}

// GetSoundVolume returns the volume for Buff and farm reminder sounds.
func (a *App) GetSoundVolume() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.soundVolume
}

// PlaySoundPreview plays a real reminder sound at the current volume.
func (a *App) PlaySoundPreview() {
	playFarmSound("农作物成熟.wav", a.GetSoundVolume())
}
