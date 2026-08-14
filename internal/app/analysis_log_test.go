package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalysisLogIsOptInAndCanBeStopped(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MABI_WORK_DIR", dir)
	a := NewApp()
	a.ctx = context.Background()

	if a.GetAnalysisLogEnabled() {
		t.Fatal("analysis logging must be disabled by default")
	}
	if err := a.SetAnalysisLogEnabled(true); err != nil {
		t.Fatalf("enable analysis logging: %v", err)
	}
	if !a.GetAnalysisLogEnabled() {
		t.Fatal("analysis logging did not enable")
	}
	a.writeAnalysisSnapshot(a.analysisLog)
	path := a.GetAnalysisLogPath()
	if path != filepath.Join(dir, analysisLogFileName) {
		t.Fatalf("analysis path = %q", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read analysis log: %v", err)
	}
	if !strings.Contains(string(data), "SESSION_START") {
		t.Fatalf("analysis log has no session marker: %s", data)
	}
	if !strings.Contains(string(data), "RUNTIME") || !strings.Contains(string(data), "INTERVAL") {
		t.Fatalf("analysis log has no performance snapshot: %s", data)
	}
	if err := a.SetAnalysisLogEnabled(false); err != nil {
		t.Fatalf("disable analysis logging: %v", err)
	}
	if a.GetAnalysisLogEnabled() {
		t.Fatal("analysis logging did not disable")
	}
	settings, err := os.ReadFile(analysisLogSettingsPath())
	if err != nil {
		t.Fatalf("read analysis settings: %v", err)
	}
	if !strings.Contains(string(settings), `"enabled":false`) {
		t.Fatalf("analysis settings were not persisted as disabled: %s", settings)
	}
}
