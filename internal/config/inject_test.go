package config

import (
	"encoding/base64"
	"testing"
)

func TestApplyBuildInjectOverrides(t *testing.T) {
	origSecret := UploadSecret
	origEndpoint := UploadEndpoint
	origKeyword := UploadDungeonKeyword
	origEnabled := UploadEnabled
	defer func() {
		UploadSecret = origSecret
		UploadEndpoint = origEndpoint
		UploadDungeonKeyword = origKeyword
		UploadEnabled = origEnabled
		uploadSecretInjectB64 = ""
		uploadEndpointInject = ""
		uploadEndpointInjectB64 = ""
		uploadDungeonKeywordInject = ""
		uploadEnabledInject = ""
	}()

	uploadSecretInjectB64 = base64.StdEncoding.EncodeToString([]byte("injected-secret"))
	uploadEndpointInjectB64 = base64.StdEncoding.EncodeToString([]byte("http://inject.example/push"))
	uploadDungeonKeywordInject = "测试副本"
	uploadEnabledInject = "false"

	applyBuildInjectOverrides()

	if UploadSecret != "injected-secret" {
		t.Fatalf("unexpected secret: %q", UploadSecret)
	}
	if UploadEndpoint != "http://inject.example/push" {
		t.Fatalf("unexpected endpoint: %q", UploadEndpoint)
	}
	if UploadDungeonKeyword != "测试副本" {
		t.Fatalf("unexpected keyword: %q", UploadDungeonKeyword)
	}
	if UploadEnabled {
		t.Fatal("expected upload disabled from inject")
	}
}
