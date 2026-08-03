package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"blonymonitorv2/internal/config"
)

func TestSignRankingConsentCoversMethodPlayerAndBody(t *testing.T) {
	base := signRankingConsent("test-secret", 1700000000, "nonce-1", http.MethodPut, "12345", []byte(`{"mode":"public"}`))
	if base == signRankingConsent("test-secret", 1700000000, "nonce-1", http.MethodGet, "12345", []byte(`{"mode":"public"}`)) {
		t.Fatal("expected method to affect signature")
	}
	if base == signRankingConsent("test-secret", 1700000000, "nonce-1", http.MethodPut, "99999", []byte(`{"mode":"public"}`)) {
		t.Fatal("expected player ID to affect signature")
	}
	if base == signRankingConsent("test-secret", 1700000000, "nonce-1", http.MethodPut, "12345", []byte(`{"mode":"none"}`)) {
		t.Fatal("expected body to affect signature")
	}
}

func TestGetRankingParticipationWithoutPlayer(t *testing.T) {
	origEndpoint, origSecret := config.RankingConsentEndpoint, config.UploadSecret
	defer func() {
		config.RankingConsentEndpoint, config.UploadSecret = origEndpoint, origSecret
	}()
	config.RankingConsentEndpoint = "http://example.com/ranking-consent"
	config.UploadSecret = "test-secret"

	state, err := NewApp().GetRankingParticipation()
	if err != nil {
		t.Fatal(err)
	}
	if !state.Available || state.PlayerReady || state.Mode != RankingModeNone || state.Participating {
		t.Fatalf("unexpected state: %+v", state)
	}
}

func TestSetRankingParticipation(t *testing.T) {
	origEndpoint, origSecret := config.RankingConsentEndpoint, config.UploadSecret
	defer func() {
		config.RankingConsentEndpoint, config.UploadSecret = origEndpoint, origSecret
	}()
	config.UploadSecret = "test-secret"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("X-Player-ID") != "12345" {
			t.Errorf("unexpected player ID: %q", r.Header.Get("X-Player-ID"))
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		timestamp, err := strconv.ParseInt(r.Header.Get("X-Timestamp"), 10, 64)
		if err != nil {
			t.Errorf("invalid timestamp: %v", err)
		}
		expected := "HMAC-SHA256 " + signRankingConsent(
			config.UploadSecret,
			timestamp,
			r.Header.Get("X-Nonce"),
			r.Method,
			r.Header.Get("X-Player-ID"),
			body,
		)
		if r.Header.Get("Authorization") != expected {
			t.Errorf("unexpected signature")
		}
		var update rankingConsentUpdate
		if err := json.Unmarshal(body, &update); err != nil {
			t.Fatal(err)
		}
		if update.Mode != RankingModePublic || update.PlayerName != "Alice" {
			t.Errorf("unexpected update: %+v", update)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"mode":"public","updatedAt":"2026-07-21T12:00:00Z"}`))
	}))
	defer server.Close()
	config.RankingConsentEndpoint = server.URL

	a := NewApp()
	a.selfId = "12345"
	a.selfName = "Alice"
	state, err := a.SetRankingParticipation(RankingModePublic)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Available || !state.PlayerReady || state.Mode != RankingModePublic || !state.Participating || state.PlayerName != "Alice" {
		t.Fatalf("unexpected state: %+v", state)
	}
}

func TestNormalizeRankingMode(t *testing.T) {
	legacyTrue := true
	if got := normalizeRankingMode("", &legacyTrue); got != RankingModePublic {
		t.Fatalf("legacy participating=true should map to public, got %q", got)
	}
	if got := normalizeRankingMode("anonymous", nil); got != RankingModeAnonymous {
		t.Fatalf("expected anonymous, got %q", got)
	}
	if got := normalizeRankingMode("unknown", nil); got != RankingModeNone {
		t.Fatalf("unknown mode should default to none, got %q", got)
	}
}
