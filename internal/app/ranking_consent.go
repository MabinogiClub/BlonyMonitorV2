package app

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"blonymonitorv2/internal/config"
)

const rankingConsentTimeout = 10 * time.Second

const (
	RankingModeNone      = "none"
	RankingModeAnonymous = "anonymous"
	RankingModePublic    = "public"
)

// RankingParticipation 是服务器保存的当前角色公开排行参与状态。
type RankingParticipation struct {
	Available   bool   `json:"available"`
	PlayerReady bool   `json:"playerReady"`
	PlayerName  string `json:"playerName"`
	Mode        string `json:"mode"`
	// Participating 保留给旧前端；匿名和公开排行都会返回 true。
	Participating bool   `json:"participating"`
	UpdatedAt     string `json:"updatedAt"`
}

type rankingConsentUpdate struct {
	Mode          string `json:"mode"`
	PlayerName    string `json:"playerName,omitempty"`
	ClientVersion string `json:"clientVersion"`
}

type rankingConsentResponse struct {
	Mode          string `json:"mode"`
	Participating *bool  `json:"participating,omitempty"`
	UpdatedAt     string `json:"updatedAt"`
}

func normalizeRankingMode(mode string, legacyParticipating *bool) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case RankingModeAnonymous:
		return RankingModeAnonymous
	case RankingModePublic:
		return RankingModePublic
	case RankingModeNone:
		return RankingModeNone
	}
	if legacyParticipating != nil && *legacyParticipating {
		return RankingModePublic
	}
	return RankingModeNone
}

func validRankingMode(mode string) bool {
	switch mode {
	case RankingModeNone, RankingModeAnonymous, RankingModePublic:
		return true
	default:
		return false
	}
}

func rankingConsentConfigured() bool {
	return strings.TrimSpace(config.RankingConsentEndpoint) != "" && isUploadSecretConfigured()
}

func (a *App) currentPlayerIdentity() (string, string) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.selfId, a.selfName
}

// GetRankingParticipation 从服务器读取当前角色的排行参与状态。
func (a *App) GetRankingParticipation() (RankingParticipation, error) {
	playerID, playerName := a.currentPlayerIdentity()
	state := RankingParticipation{
		Available:   rankingConsentConfigured(),
		PlayerReady: playerID != "",
		PlayerName:  playerName,
		Mode:        RankingModeNone,
	}
	if !state.Available || !state.PlayerReady {
		if !state.Available {
			setServerInteraction("ranking", "disabled", "排行服务未配置")
		} else {
			setServerInteraction("ranking", "idle", "尚未识别角色")
		}
		return state, nil
	}
	setServerInteraction("ranking", "checking", "同步中")

	response, err := requestRankingConsent(http.MethodGet, playerID, playerName, nil)
	if err != nil {
		setServerInteraction("ranking", "error", "同步失败")
		return state, err
	}
	state.Mode = normalizeRankingMode(response.Mode, response.Participating)
	state.Participating = state.Mode != RankingModeNone
	state.UpdatedAt = response.UpdatedAt
	setServerInteraction("ranking", "success", "同步成功")
	return state, nil
}

// SetRankingParticipation 将当前角色的排行参与选择保存到服务器。
func (a *App) SetRankingParticipation(mode string) (RankingParticipation, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	playerID, playerName := a.currentPlayerIdentity()
	state := RankingParticipation{
		Available:     rankingConsentConfigured(),
		PlayerReady:   playerID != "",
		PlayerName:    playerName,
		Mode:          mode,
		Participating: mode != RankingModeNone,
	}
	if !validRankingMode(mode) {
		setServerInteraction("ranking", "error", "无效的排行模式")
		return state, fmt.Errorf("invalid ranking mode")
	}
	if !state.Available {
		setServerInteraction("ranking", "disabled", "排行服务未配置")
		return state, fmt.Errorf("ranking consent service is not configured")
	}
	if !state.PlayerReady {
		setServerInteraction("ranking", "idle", "尚未识别角色")
		return state, fmt.Errorf("current player is not identified")
	}
	setServerInteraction("ranking", "saving", "保存中")

	body, err := json.Marshal(rankingConsentUpdate{
		Mode:          mode,
		PlayerName:    playerName,
		ClientVersion: config.ClientVersion,
	})
	if err != nil {
		setServerInteraction("ranking", "error", "请求数据失败")
		return state, fmt.Errorf("encode ranking participation: %w", err)
	}
	response, err := requestRankingConsent(http.MethodPut, playerID, playerName, body)
	if err != nil {
		setServerInteraction("ranking", "error", "保存失败")
		return state, err
	}
	state.Mode = normalizeRankingMode(response.Mode, response.Participating)
	state.Participating = state.Mode != RankingModeNone
	state.UpdatedAt = response.UpdatedAt
	setServerInteraction("ranking", "success", "保存成功")
	return state, nil
}

func requestRankingConsent(method, playerID, playerName string, body []byte) (rankingConsentResponse, error) {
	var result rankingConsentResponse
	endpoint := strings.TrimSpace(config.RankingConsentEndpoint)
	secret := strings.TrimSpace(config.UploadSecret)

	nonce, err := newUploadNonce()
	if err != nil {
		return result, fmt.Errorf("create ranking request nonce: %w", err)
	}
	timestamp := time.Now().Unix()
	signature := signRankingConsent(secret, timestamp, nonce, method, playerID, body)

	req, err := http.NewRequest(method, endpoint, bytes.NewReader(body))
	if err != nil {
		return result, fmt.Errorf("create ranking request: %w", sanitizeUploadError(err, endpoint))
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "HMAC-SHA256 "+signature)
	req.Header.Set("X-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Player-ID", playerID)
	if playerName != "" {
		req.Header.Set("X-Player-Name", playerName)
	}

	client := &http.Client{Timeout: rankingConsentTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return result, fmt.Errorf("ranking service request failed: %w", sanitizeUploadError(err, endpoint))
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return result, fmt.Errorf("read ranking service response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return result, fmt.Errorf("ranking service returned %s", resp.Status)
	}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return result, fmt.Errorf("decode ranking service response: %w", err)
	}
	return result, nil
}

func hashRankingConsentBody(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func signRankingConsent(secret string, timestamp int64, nonce, method, playerID string, body []byte) string {
	payload := fmt.Sprintf("%d\n%s\n%s\n%s\n%s", timestamp, nonce, strings.ToUpper(method), playerID, hashRankingConsentBody(body))
	mac := hmac.New(sha256.New, uploadSecretKey(secret))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
