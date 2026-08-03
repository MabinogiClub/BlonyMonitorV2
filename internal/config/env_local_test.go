package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseEnvFile(t *testing.T) {
	content := `
# comment
BLONY_UPLOAD_SECRET=local-secret
export BLONY_UPLOAD_ENDPOINT="http://example.com/push"
BLONY_UPLOAD_ENABLED=true
BLONY_RANKING_CONSENT_ENDPOINT=http://example.com/ranking-consent
BLONY_ANNOUNCEMENT_ENDPOINT=http://example.com/announcement
`
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	values := parseEnvFile(file)
	if values["BLONY_UPLOAD_SECRET"] != "local-secret" {
		t.Fatalf("unexpected secret: %q", values["BLONY_UPLOAD_SECRET"])
	}
	if values["BLONY_UPLOAD_ENDPOINT"] != "http://example.com/push" {
		t.Fatalf("unexpected endpoint: %q", values["BLONY_UPLOAD_ENDPOINT"])
	}
	if values["BLONY_RANKING_CONSENT_ENDPOINT"] != "http://example.com/ranking-consent" {
		t.Fatalf("unexpected ranking consent endpoint: %q", values["BLONY_RANKING_CONSENT_ENDPOINT"])
	}
	if values["BLONY_ANNOUNCEMENT_ENDPOINT"] != "http://example.com/announcement" {
		t.Fatalf("unexpected announcement endpoint: %q", values["BLONY_ANNOUNCEMENT_ENDPOINT"])
	}
}

func TestLoadLocalEnvFile(t *testing.T) {
	origSecret := UploadSecret
	origEndpoint := UploadEndpoint
	origRankingConsentEndpoint := RankingConsentEndpoint
	origAnnouncementEndpoint := AnnouncementEndpoint
	defer func() {
		UploadSecret = origSecret
		UploadEndpoint = origEndpoint
		RankingConsentEndpoint = origRankingConsentEndpoint
		AnnouncementEndpoint = origAnnouncementEndpoint
	}()

	dir := t.TempDir()
	content := "BLONY_UPLOAD_SECRET=env-file-secret\nBLONY_UPLOAD_ENDPOINT=http://env.example/push\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("MABI_WORK_DIR", dir)
	loadLocalEnvFile()

	if UploadSecret != "env-file-secret" {
		t.Fatalf("unexpected secret: %q", UploadSecret)
	}
	if UploadEndpoint != "http://env.example/push" {
		t.Fatalf("unexpected endpoint: %q", UploadEndpoint)
	}
}

func TestApplyUploadEnvOverrides(t *testing.T) {
	origSecret := UploadSecret
	origEndpoint := UploadEndpoint
	origEnabled := UploadEnabled
	origKeyword := UploadDungeonKeyword
	defer func() {
		UploadSecret = origSecret
		UploadEndpoint = origEndpoint
		UploadEnabled = origEnabled
		UploadDungeonKeyword = origKeyword
		_ = os.Unsetenv("BLONY_UPLOAD_SECRET")
		_ = os.Unsetenv("BLONY_UPLOAD_ENDPOINT")
		_ = os.Unsetenv("BLONY_UPLOAD_ENABLED")
		_ = os.Unsetenv("BLONY_RANKING_CONSENT_ENDPOINT")
		_ = os.Unsetenv("BLONY_ANNOUNCEMENT_ENDPOINT")
		_ = os.Unsetenv("BLONY_UPLOAD_DUNGEON_KEYWORD")
	}()

	UploadSecret = ""
	_ = os.Setenv("BLONY_UPLOAD_SECRET", "env-secret")
	_ = os.Setenv("BLONY_UPLOAD_ENDPOINT", "http://example.com/push")
	_ = os.Setenv("BLONY_UPLOAD_ENABLED", "false")
	_ = os.Setenv("BLONY_RANKING_CONSENT_ENDPOINT", "http://example.com/ranking-consent")
	_ = os.Setenv("BLONY_ANNOUNCEMENT_ENDPOINT", "http://example.com/announcement")
	_ = os.Setenv("BLONY_UPLOAD_DUNGEON_KEYWORD", "测试副本")
	applyUploadEnvOverrides()

	if UploadSecret != "env-secret" {
		t.Fatalf("unexpected secret: %q", UploadSecret)
	}
	if UploadEndpoint != "http://example.com/push" {
		t.Fatalf("unexpected endpoint: %q", UploadEndpoint)
	}
	if UploadEnabled {
		t.Fatal("expected upload disabled from env")
	}
	if UploadDungeonKeyword != "测试副本" {
		t.Fatalf("unexpected keyword: %q", UploadDungeonKeyword)
	}
	if RankingConsentEndpoint != "http://example.com/ranking-consent" {
		t.Fatalf("unexpected ranking consent endpoint: %q", RankingConsentEndpoint)
	}
	if AnnouncementEndpoint != "http://example.com/announcement" {
		t.Fatalf("unexpected announcement endpoint: %q", AnnouncementEndpoint)
	}
}

func TestEnvOverridesDotEnv(t *testing.T) {
	origSecret := UploadSecret
	defer func() {
		UploadSecret = origSecret
		_ = os.Unsetenv("BLONY_UPLOAD_SECRET")
	}()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("BLONY_UPLOAD_SECRET=from-dotenv\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("MABI_WORK_DIR", dir)
	_ = os.Setenv("BLONY_UPLOAD_SECRET", "from-process-env")

	UploadSecret = ""
	loadLocalEnvFile()
	if UploadSecret != "from-dotenv" {
		t.Fatalf("expected dotenv value first, got %q", UploadSecret)
	}
	applyUploadEnvOverrides()
	if UploadSecret != "from-process-env" {
		t.Fatalf("expected process env to win, got %q", UploadSecret)
	}
}

func TestResolveLocalEnvPathWalksUpFromBuildBin(t *testing.T) {
	root := t.TempDir()
	buildBin := filepath.Join(root, "build", "bin")
	if err := os.MkdirAll(buildBin, 0o755); err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(root, ".env")
	if err := os.WriteFile(envPath, []byte("BLONY_UPLOAD_SECRET=found\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	path, err := resolveLocalEnvPathFrom(buildBin, buildBin, "")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(path) != filepath.Clean(envPath) {
		t.Fatalf("expected %s, got %s", envPath, path)
	}
}

func TestResolveLocalEnvPathPrefersWorkDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("x=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	path, err := resolveLocalEnvPathFrom("", "", dir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(path) != filepath.Clean(filepath.Join(dir, ".env")) {
		t.Fatalf("unexpected path: %s", path)
	}
}
