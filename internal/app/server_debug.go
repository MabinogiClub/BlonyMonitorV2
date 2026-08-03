package app

import (
	"strings"
	"sync"
	"time"
)

// ServiceInteractionDebug 只记录服务交互状态，不保存地址、密钥或完整响应。
type ServiceInteractionDebug struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	UpdatedAt string `json:"updatedAt"`
}

type serverInteractionDebug struct {
	Announcement ServiceInteractionDebug
	Ranking      ServiceInteractionDebug
}

var (
	serverInteractionMu sync.RWMutex
	serverInteraction   = serverInteractionDebug{
		Announcement: ServiceInteractionDebug{Status: "idle", Message: "未检查"},
		Ranking:      ServiceInteractionDebug{Status: "idle", Message: "未检查"},
	}
)

func setServerInteraction(service, status, message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "-"
	}
	if len(message) > 256 {
		message = message[:256] + "..."
	}
	info := ServiceInteractionDebug{
		Status:    status,
		Message:   message,
		UpdatedAt: time.Now().Format("15:04:05"),
	}

	serverInteractionMu.Lock()
	switch service {
	case "announcement":
		serverInteraction.Announcement = info
	case "ranking":
		serverInteraction.Ranking = info
	}
	serverInteractionMu.Unlock()
}

func getServerInteractionDebug() (announcement, ranking ServiceInteractionDebug) {
	serverInteractionMu.RLock()
	defer serverInteractionMu.RUnlock()
	return serverInteraction.Announcement, serverInteraction.Ranking
}
