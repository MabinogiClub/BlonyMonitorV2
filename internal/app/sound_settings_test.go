package app

import "testing"

func TestSoundVolumeDefaultsAndPersistence(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	if got := loadSoundVolume(); got != defaultSoundVolume {
		t.Fatalf("default sound volume = %d, want %d", got, defaultSoundVolume)
	}

	a := NewApp()
	if err := a.SetSoundVolume(42); err != nil {
		t.Fatalf("set sound volume: %v", err)
	}
	if got := a.GetSoundVolume(); got != 42 {
		t.Fatalf("current sound volume = %d, want 42", got)
	}
	if got := loadSoundVolume(); got != 42 {
		t.Fatalf("saved sound volume = %d, want 42", got)
	}
}

func TestSoundVolumeIsClamped(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	a := NewApp()
	if err := a.SetSoundVolume(120); err != nil {
		t.Fatalf("set sound volume above range: %v", err)
	}
	if got := a.GetSoundVolume(); got != 100 {
		t.Fatalf("clamped high sound volume = %d, want 100", got)
	}
	if err := a.SetSoundVolume(-20); err != nil {
		t.Fatalf("set sound volume below range: %v", err)
	}
	if got := a.GetSoundVolume(); got != 0 {
		t.Fatalf("clamped low sound volume = %d, want 0", got)
	}
}
