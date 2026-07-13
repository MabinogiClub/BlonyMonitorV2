package app

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// UploadDebugInfo 最近一次战斗上传状态（供调试面板展示）。
type UploadDebugInfo struct {
	Status            string `json:"status"`
	Dungeon           string `json:"dungeon"`
	FileName          string `json:"fileName"`
	HTTPStatus        int    `json:"httpStatus"`
	Response          string `json:"response"`
	Message           string `json:"message"`
	UpdatedAt         string `json:"updatedAt"`
	LastSaveName      string `json:"lastSaveName"`
	LastSaveFile      string `json:"lastSaveFile"`
	LastSaveAt        string `json:"lastSaveAt"`
	ConfigEnabled     bool   `json:"configEnabled"`
	ConfigSecretReady bool   `json:"configSecretReady"`
	ConfigHasEndpoint bool   `json:"configHasEndpoint"`
	ConfigKeyword     string `json:"configKeyword"`
}

var (
	uploadDebugMu sync.RWMutex
	uploadDebug   = UploadDebugInfo{
		Status:  "idle",
		Message: "暂无上传记录",
	}
	lastBattleSaveName string
	lastBattleSaveFile string
	lastBattleSaveAt   string
)

func getUploadDebugInfo() UploadDebugInfo {
	uploadDebugMu.RLock()
	info := uploadDebug
	info.LastSaveName = lastBattleSaveName
	info.LastSaveFile = lastBattleSaveFile
	info.LastSaveAt = lastBattleSaveAt
	uploadDebugMu.RUnlock()

	enabled, endpoint, keyword := getUploadFilterConfig()
	info.ConfigEnabled = enabled
	info.ConfigSecretReady = isUploadSecretConfigured()
	info.ConfigHasEndpoint = strings.TrimSpace(endpoint) != ""
	info.ConfigKeyword = keyword
	return info
}

func setUploadDebug(info UploadDebugInfo) {
	info.UpdatedAt = time.Now().Format("15:04:05")
	uploadDebugMu.Lock()
	uploadDebug = info
	uploadDebugMu.Unlock()
}

func recordUploadUploading(dungeon, fileName string) {
	setUploadDebug(UploadDebugInfo{
		Status:   "uploading",
		Dungeon:  dungeon,
		FileName: fileName,
		Message:  "正在上传...",
	})
}

func recordUploadSuccess(dungeon, fileName string, statusCode int, response string) {
	setUploadDebug(UploadDebugInfo{
		Status:     "success",
		Dungeon:    dungeon,
		FileName:   fileName,
		HTTPStatus: statusCode,
		Response:   truncateUploadDebugText(response, 512),
		Message:    fmt.Sprintf("上传成功 HTTP %d", statusCode),
	})
}

func recordUploadError(dungeon, fileName string, statusCode int, response string, err error) {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	if statusCode > 0 && msg == "" {
		msg = fmt.Sprintf("HTTP %d", statusCode)
	}
	setUploadDebug(UploadDebugInfo{
		Status:     "error",
		Dungeon:    dungeon,
		FileName:   fileName,
		HTTPStatus: statusCode,
		Response:   truncateUploadDebugText(response, 512),
		Message:    msg,
	})
}

func recordLastBattleSave(saveName, fileName string) {
	uploadDebugMu.Lock()
	lastBattleSaveName = saveName
	lastBattleSaveFile = fileName
	lastBattleSaveAt = time.Now().Format("15:04:05")
	uploadDebugMu.Unlock()
}

func recordUploadSkipped(dungeon, fileName, reason string) {
	setUploadDebug(UploadDebugInfo{
		Status:   "skipped",
		Dungeon:  dungeon,
		FileName: fileName,
		Message:  reason,
	})
}

func truncateUploadDebugText(text string, maxLen int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "-"
	}
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}
