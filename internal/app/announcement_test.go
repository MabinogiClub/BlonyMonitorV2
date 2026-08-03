package app

import (
	"crypto/hmac"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"blonymonitorv2/internal/config"
)

func TestDecodeAnnouncementsAndSelectLatest(t *testing.T) {
	response := []byte(`{"announcements":[{"timestamp":10,"title":"旧","html":"<p>old</p>"},{"timestamp":20,"title":"新","html":"<p>new</p>"}]}`)
	items, err := decodeAnnouncements(response)
	if err != nil || len(items) != 2 {
		t.Fatalf("unexpected decode: %v, %#v", err, items)
	}
}

func TestGetLatestAnnouncement(t *testing.T) {
	origEndpoint, origSecret := config.AnnouncementEndpoint, config.UploadSecret
	defer func() {
		config.AnnouncementEndpoint = origEndpoint
		config.UploadSecret = origSecret
	}()
	config.UploadSecret = "test-secret"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Player-ID") != "announcement" {
			t.Fatalf("unexpected announcement player ID: %q", r.Header.Get("X-Player-ID"))
		}
		body, _ := io.ReadAll(r.Body)
		timestamp, err := strconv.ParseInt(r.Header.Get("X-Timestamp"), 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		expected := "HMAC-SHA256 " + signRankingConsent(config.UploadSecret, timestamp, r.Header.Get("X-Nonce"), http.MethodGet, "announcement", body)
		if !hmac.Equal([]byte(expected), []byte(r.Header.Get("Authorization"))) {
			t.Fatal("announcement request signature mismatch")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"announcements":[{"timestamp":100,"title":"old","html":"<p>old</p>"},{"timestamp":200,"title":"new","html":"<strong>new</strong>"}]}`))
	}))
	defer server.Close()
	config.AnnouncementEndpoint = server.URL

	announcement, err := NewApp().GetLatestAnnouncement()
	if err != nil {
		t.Fatal(err)
	}
	if !announcement.Available || !announcement.Found || announcement.Timestamp != 200 || announcement.Title != "new" {
		t.Fatalf("unexpected latest announcement: %+v", announcement)
	}
}

func TestDecodeSingleAnnouncement(t *testing.T) {
	items, err := decodeAnnouncements([]byte(`{"timestamp":1,"html":"<p>one</p>"}`))
	if err != nil || len(items) != 1 {
		t.Fatalf("unexpected single announcement: %v %#v", err, items)
	}
	encoded, _ := json.Marshal(items[0])
	if len(encoded) == 0 {
		t.Fatal("expected announcement to marshal")
	}
}
