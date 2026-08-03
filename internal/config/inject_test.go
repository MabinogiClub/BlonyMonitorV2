package config

import (
	"encoding/base64"
	"testing"
)

func TestApplyBuildInjectOverrides(t *testing.T) {
	origSecret := UploadSecret
	origEndpoint := UploadEndpoint
	origRankingConsentEndpoint := RankingConsentEndpoint
	origAnnouncementEndpoint := AnnouncementEndpoint
	origKeyword := UploadDungeonKeyword
	origEnabled := UploadEnabled
	defer func() {
		UploadSecret = origSecret
		UploadEndpoint = origEndpoint
		RankingConsentEndpoint = origRankingConsentEndpoint
		AnnouncementEndpoint = origAnnouncementEndpoint
		UploadDungeonKeyword = origKeyword
		UploadEnabled = origEnabled
		uploadSecretInjectB64 = ""
		uploadEndpointInject = ""
		uploadEndpointInjectB64 = ""
		rankingConsentEndpointInjectB64 = ""
		announcementEndpointInjectB64 = ""
		uploadDungeonKeywordInject = ""
		uploadEnabledInject = ""
	}()

	uploadSecretInjectB64 = base64.StdEncoding.EncodeToString([]byte("injected-secret"))
	uploadEndpointInjectB64 = base64.StdEncoding.EncodeToString([]byte("http://inject.example/push"))
	rankingConsentEndpointInjectB64 = base64.StdEncoding.EncodeToString([]byte("http://inject.example/ranking-consent"))
	announcementEndpointInjectB64 = base64.StdEncoding.EncodeToString([]byte("http://inject.example/announcement"))
	uploadDungeonKeywordInject = "测试副本"
	uploadEnabledInject = "false"

	applyBuildInjectOverrides()

	if UploadSecret != "injected-secret" {
		t.Fatalf("unexpected secret: %q", UploadSecret)
	}
	if UploadEndpoint != "http://inject.example/push" {
		t.Fatalf("unexpected endpoint: %q", UploadEndpoint)
	}
	if RankingConsentEndpoint != "http://inject.example/ranking-consent" {
		t.Fatalf("unexpected ranking consent endpoint: %q", RankingConsentEndpoint)
	}
	if AnnouncementEndpoint != "http://inject.example/announcement" {
		t.Fatalf("unexpected announcement endpoint: %q", AnnouncementEndpoint)
	}
	if UploadDungeonKeyword != "测试副本" {
		t.Fatalf("unexpected keyword: %q", UploadDungeonKeyword)
	}
	if UploadEnabled {
		t.Fatal("expected upload disabled from inject")
	}
}
