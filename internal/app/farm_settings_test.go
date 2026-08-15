package app

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestFarmSettingsDefaults(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())

	got := loadFarmSettings()
	want := defaultFarmSettings()
	if got != want {
		t.Fatalf("default settings = %+v, want %+v", got, want)
	}
	if got.Enabled {
		t.Fatal("farm monitor must be disabled by default")
	}
	if !got.ReadyNotificationEnabled || !got.SpecialNotificationEnabled {
		t.Fatal("notification switches must be enabled by default")
	}
}

func TestFarmManagerPersistsSettings(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	mgr := NewFarmManager(ctx, nil, nil, nil)
	mgr.SetEnabled(true)
	mgr.SetReadyNotificationEnabled(false)
	mgr.SetSpecialNotificationEnabled(false)
	cancel()

	reloadedCtx, reloadedCancel := context.WithCancel(context.Background())
	defer reloadedCancel()
	reloaded := NewFarmManager(reloadedCtx, nil, nil, nil)
	state := reloaded.State(time.Now())
	if !state.Enabled || state.ReadyNotificationEnabled || state.SpecialNotificationEnabled {
		t.Fatalf("reloaded settings = enabled:%v ready:%v special:%v", state.Enabled, state.ReadyNotificationEnabled, state.SpecialNotificationEnabled)
	}
}

func TestFarmSettingsMissingFieldsKeepNotificationDefaults(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	if err := os.WriteFile(farmSettingsPath(), []byte(`{"enabled":true}`), 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	got := loadFarmSettings()
	if !got.Enabled || !got.ReadyNotificationEnabled || !got.SpecialNotificationEnabled {
		t.Fatalf("settings with missing fields = %+v", got)
	}
}
