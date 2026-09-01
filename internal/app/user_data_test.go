package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUserDataDirectoryHonorsWorkDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MABI_WORK_DIR", dir)

	if got := userDataDirectory(); got != filepath.Clean(dir) {
		t.Fatalf("userDataDirectory() = %q, want %q", got, filepath.Clean(dir))
	}
	if got := (&App{}).saveDir(); got != filepath.Join(dir, "save") {
		t.Fatalf("saveDir() = %q", got)
	}
	if got := buffOrderFilePath(); got != filepath.Join(dir, "buff_order.json") {
		t.Fatalf("buffOrderFilePath() = %q", got)
	}
}

func TestMigrateLegacyUserDataCopiesFilesAndSaveDirectory(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	legacySettings := []byte(`{"enabled":true}`)
	if err := os.WriteFile(filepath.Join(source, farmSettingsFileName), legacySettings, 0o644); err != nil {
		t.Fatal(err)
	}
	legacySave := filepath.Join(source, "save", "2026-09-01_test.json.gz")
	if err := os.MkdirAll(filepath.Dir(legacySave), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacySave, []byte("battle"), 0o644); err != nil {
		t.Fatal(err)
	}

	migrateLegacyUserDataFrom(source, target)

	assertFileContents(t, filepath.Join(target, farmSettingsFileName), legacySettings)
	assertFileContents(t, filepath.Join(target, "save", filepath.Base(legacySave)), []byte("battle"))
}

func TestMigrateLegacyUserDataDoesNotOverwriteExistingFiles(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	name := farmRecommendationDataFileName
	if err := os.WriteFile(filepath.Join(source, name), []byte("legacy"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, name), []byte("current"), 0o644); err != nil {
		t.Fatal(err)
	}

	migrateLegacyUserDataFrom(source, target)

	assertFileContents(t, filepath.Join(target, name), []byte("current"))
	assertFileContents(t, filepath.Join(source, name), []byte("legacy"))
}

func assertFileContents(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	if string(got) != string(want) {
		t.Fatalf("contents of %q = %q, want %q", path, got, want)
	}
}
