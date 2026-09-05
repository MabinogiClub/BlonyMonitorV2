package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDisplaySettingsDefaultToUnmasked(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())

	if settings := loadDisplaySettings(); settings.MaskSelfName {
		t.Fatal("maskSelfName must default to false")
	}
}

func TestDisplaySettingsRoundTrip(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())

	app := NewApp()
	if err := app.SetMaskSelfName(true); err != nil {
		t.Fatalf("set mask self name: %v", err)
	}
	if !app.GetMaskSelfName() {
		t.Fatal("app maskSelfName was not updated")
	}
	if got := loadDisplaySettings(); !got.MaskSelfName {
		t.Fatal("saved maskSelfName was not enabled")
	}

	data, err := os.ReadFile(displaySettingsPath())
	if err != nil {
		t.Fatalf("read display settings: %v", err)
	}
	if string(data) != `{"maskSelfName":true}` {
		t.Fatalf("display settings JSON = %s", data)
	}
}

func TestDisplaySettingsPathUsesUserDataDirectory(t *testing.T) {
	workDir := t.TempDir()
	t.Setenv("MABI_WORK_DIR", workDir)

	want := filepath.Join(workDir, "blonymonitor-display-settings.json")
	if got := displaySettingsPath(); got != want {
		t.Fatalf("display settings path = %q, want %q", got, want)
	}
}
