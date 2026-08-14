package app

import (
	"testing"
	"time"
)

func TestDPSRefreshSettingsDefaultAndPersistence(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	if got := loadDPSRefreshSettings(); got != defaultDPSRefreshSettings() {
		t.Fatalf("default settings = %+v", got)
	}

	a := NewApp()
	a.dpsUpdateThrottler = NewDPSUpdateThrottler(a, 100*time.Millisecond)
	want := DPSRefreshSettings{BackendIntervalMS: 1000, FrontendIntervalMS: 500}
	got, err := a.SetDPSRefreshSettings(want)
	if err != nil {
		t.Fatalf("SetDPSRefreshSettings() error = %v", err)
	}
	if got != want || a.GetDPSRefreshSettings() != want {
		t.Fatalf("saved settings = %+v, want %+v", got, want)
	}
	if interval := time.Duration(a.dpsUpdateThrottler.minInterval.Load()); interval != time.Second {
		t.Fatalf("backend interval = %v, want 1s", interval)
	}
	if loaded := loadDPSRefreshSettings(); loaded != want {
		t.Fatalf("persisted settings = %+v, want %+v", loaded, want)
	}
}

func TestDPSRefreshSettingsRejectsOutOfRangeValues(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	a := NewApp()
	before := a.GetDPSRefreshSettings()
	if _, err := a.SetDPSRefreshSettings(DPSRefreshSettings{
		BackendIntervalMS:  10,
		FrontendIntervalMS: 200,
	}); err == nil {
		t.Fatal("expected an error for an interval below the supported range")
	}
	if after := a.GetDPSRefreshSettings(); after != before {
		t.Fatalf("invalid settings changed state: before=%+v after=%+v", before, after)
	}
}
