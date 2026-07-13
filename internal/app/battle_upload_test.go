package app

import (
	"encoding/base64"
	"testing"

	"blonymonitorv2/internal/config"
)

func TestFilterSaveDataForUpload(t *testing.T) {
	data := SaveFileData{
		Targets: []targetExport{
			{TargetID: "1", BossHP: &BossHPExport{MaxHP: 199_999_999}},
			{TargetID: "2", BossHP: &BossHPExport{MaxHP: 200_000_000}},
			{TargetID: "3", BossHP: &BossHPExport{MaxHP: 500_000_000}},
			{TargetID: "4"},
		},
	}

	filtered := filterSaveDataForUpload(data, "布里列赫")
	if len(filtered.Targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(filtered.Targets))
	}
	if filtered.Targets[0].TargetID != "2" || filtered.Targets[1].TargetID != "3" {
		t.Fatalf("unexpected target ids: %s, %s", filtered.Targets[0].TargetID, filtered.Targets[1].TargetID)
	}
}

func TestShouldUploadBattle(t *testing.T) {
	origEndpoint := config.UploadEndpoint
	origKeyword := config.UploadDungeonKeyword
	origSecret := config.UploadSecret
	origEnabled := config.UploadEnabled
	defer func() {
		config.UploadEndpoint = origEndpoint
		config.UploadDungeonKeyword = origKeyword
		config.UploadSecret = origSecret
		config.UploadEnabled = origEnabled
	}()

	config.UploadEndpoint = "http://example.com/upload"
	config.UploadDungeonKeyword = "布里列赫,佩斯皮亚德"
	config.UploadSecret = "test-secret"
	config.UploadEnabled = true

	if !shouldUploadBattle("布里列赫") {
		t.Fatal("expected upload for matching dungeon")
	}
	if shouldUploadBattle("其他副本") {
		t.Fatal("expected skip for non-matching dungeon")
	}
	if !shouldUploadBattle("2026-07-12_15-04-05_布里列赫") {
		t.Fatal("expected upload when save name contains keyword")
	}
	if !shouldUploadBattle("2026-07-13_22-21-48_佩斯皮亚德") {
		t.Fatal("expected upload when save name contains secondary keyword")
	}
	if !shouldUploadBattle("佩斯皮亚德") {
		t.Fatal("expected upload for comma-separated keyword list")
	}

	config.UploadEndpoint = ""
	if shouldUploadBattle("布里列赫") {
		t.Fatal("expected skip when endpoint empty")
	}

	config.UploadEndpoint = "http://example.com/upload"
	config.UploadSecret = config.UploadSecretPlaceholder
	if shouldUploadBattle("布里列赫") {
		t.Fatal("expected skip when secret is placeholder")
	}

	config.UploadEnabled = false
	config.UploadSecret = "test-secret"
	if shouldUploadBattle("布里列赫") {
		t.Fatal("expected skip when upload disabled")
	}
}

func TestSignBattleUpload(t *testing.T) {
	data := []byte("gzip-battle-data")
	sig := signBattleUpload("test-secret", 1700000000, "nonce-1", "12345", data)
	if sig == "" {
		t.Fatal("expected non-empty signature")
	}
	if sig != signBattleUpload("test-secret", 1700000000, "nonce-1", "12345", data) {
		t.Fatal("signature should be deterministic")
	}
	if sig == signBattleUpload("test-secret", 1700000000, "nonce-1", "12345", []byte("other")) {
		t.Fatal("signature should differ for different payload")
	}
	if !verifyBattleUploadSignature("test-secret", 1700000000, "nonce-1", "12345", data, sig) {
		t.Fatal("signature verification failed")
	}
	if verifyBattleUploadSignature("wrong-secret", 1700000000, "nonce-1", "12345", data, sig) {
		t.Fatal("signature verification should fail for wrong secret")
	}
}

func TestUploadBlockReason(t *testing.T) {
	origEndpoint := config.UploadEndpoint
	origKeyword := config.UploadDungeonKeyword
	origSecret := config.UploadSecret
	origEnabled := config.UploadEnabled
	defer func() {
		config.UploadEndpoint = origEndpoint
		config.UploadDungeonKeyword = origKeyword
		config.UploadSecret = origSecret
		config.UploadEnabled = origEnabled
	}()

	config.UploadEndpoint = "http://example.com/upload"
	config.UploadDungeonKeyword = "佩斯皮亚德"
	config.UploadSecret = "test-secret"
	config.UploadEnabled = true

	if reason := uploadBlockReason("战斗记录"); reason == "" {
		t.Fatal("expected block reason for generic save name")
	}
	if reason := uploadBlockReason("佩斯皮亚德"); reason != "" {
		t.Fatalf("expected no block reason, got %q", reason)
	}

	config.UploadEndpoint = ""
	if reason := uploadBlockReason("佩斯皮亚德"); reason != "未配置上传地址" {
		t.Fatalf("unexpected reason: %q", reason)
	}
}

func TestUploadSecretKeyBase64(t *testing.T) {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i)
	}
	encoded := base64.StdEncoding.EncodeToString(raw)

	if got := uploadSecretKey(encoded); string(got) != string(raw) {
		t.Fatalf("expected decoded key bytes, got len %d", len(got))
	}
	if got := uploadSecretKey("plain-text-secret"); string(got) != "plain-text-secret" {
		t.Fatalf("expected plain secret bytes, got %q", got)
	}
}
