package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"blonymonitorv2/internal/config"
)

const announcementTimeout = 10 * time.Second

// ServerAnnouncement 是服务端公告的展示数据。HTML 由前端白名单清洗后渲染。
type ServerAnnouncement struct {
	Available bool   `json:"available"`
	Found     bool   `json:"found"`
	Timestamp int64  `json:"timestamp"`
	Title     string `json:"title"`
	HTML      string `json:"html"`
}

type announcementListResponse struct {
	Announcements []ServerAnnouncement `json:"announcements"`
}

func announcementConfigured() bool {
	return strings.TrimSpace(config.AnnouncementEndpoint) != "" && isUploadSecretConfigured()
}

// GetLatestAnnouncement 每次应用启动时读取服务器公告，并只返回时间戳最大的公告。
func (a *App) GetLatestAnnouncement() (ServerAnnouncement, error) {
	state := ServerAnnouncement{Available: announcementConfigured()}
	if !state.Available {
		setServerInteraction("announcement", "disabled", "公告服务未配置")
		return state, nil
	}
	setServerInteraction("announcement", "checking", "检查中")

	nonce, err := newUploadNonce()
	if err != nil {
		setServerInteraction("announcement", "error", "请求失败")
		return state, fmt.Errorf("create announcement request nonce: %w", err)
	}
	timestamp := time.Now().Unix()
	endpoint := strings.TrimSpace(config.AnnouncementEndpoint)
	body := []byte{}
	req, err := http.NewRequest(http.MethodGet, endpoint, bytes.NewReader(body))
	if err != nil {
		setServerInteraction("announcement", "error", "请求失败")
		return state, fmt.Errorf("create announcement request: %w", sanitizeUploadError(err, endpoint))
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "HMAC-SHA256 "+signRankingConsent(config.UploadSecret, timestamp, nonce, http.MethodGet, "announcement", body))
	req.Header.Set("X-Timestamp", fmt.Sprintf("%d", timestamp))
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Player-ID", "announcement")

	resp, err := (&http.Client{Timeout: announcementTimeout}).Do(req)
	if err != nil {
		setServerInteraction("announcement", "error", "请求失败")
		return state, fmt.Errorf("announcement service request failed: %w", sanitizeUploadError(err, endpoint))
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 128*1024))
	if err != nil {
		setServerInteraction("announcement", "error", "读取失败")
		return state, fmt.Errorf("read announcement service response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		setServerInteraction("announcement", "error", "服务返回错误")
		return state, fmt.Errorf("announcement service returned %s", resp.Status)
	}

	announcements, err := decodeAnnouncements(responseBody)
	if err != nil {
		setServerInteraction("announcement", "error", "响应格式错误")
		return state, fmt.Errorf("decode announcement service response: %w", err)
	}
	for _, candidate := range announcements {
		if !candidate.Found && candidate.Timestamp == 0 && strings.TrimSpace(candidate.HTML) == "" {
			continue
		}
		if !state.Found || candidate.Timestamp > state.Timestamp {
			state = candidate
			state.Available = true
			state.Found = true
		}
	}
	if state.Found {
		setServerInteraction("announcement", "success", "已获取最新公告")
	} else {
		setServerInteraction("announcement", "success", "没有新公告")
	}
	return state, nil
}

func decodeAnnouncements(data []byte) ([]ServerAnnouncement, error) {
	var list []ServerAnnouncement
	if err := json.Unmarshal(data, &list); err == nil {
		return list, nil
	}
	var envelope announcementListResponse
	if err := json.Unmarshal(data, &envelope); err == nil && envelope.Announcements != nil {
		return envelope.Announcements, nil
	}
	var single ServerAnnouncement
	if err := json.Unmarshal(data, &single); err != nil {
		return nil, err
	}
	return []ServerAnnouncement{single}, nil
}
