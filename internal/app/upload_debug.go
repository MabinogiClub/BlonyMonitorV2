package app

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// UploadDebugInfo 最近一次战斗上传状态（供调试面板展示）。
type UploadDebugInfo struct {
	Status     string `json:"status"`
	Dungeon    string `json:"dungeon"`
	FileName   string `json:"fileName"`
	HTTPStatus int    `json:"httpStatus"`
	Response   string `json:"response"`
	Message    string `json:"message"`
	UpdatedAt  string `json:"updatedAt"`
}

var (
	uploadDebugMu sync.RWMutex
	uploadDebug   = UploadDebugInfo{
		Status:  "idle",
		Message: "暂无上传记录",
	}
)

func getUploadDebugInfo() UploadDebugInfo {
	uploadDebugMu.RLock()
	defer uploadDebugMu.RUnlock()
	return uploadDebug
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
