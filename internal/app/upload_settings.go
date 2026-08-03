package app

import (
	"strings"
	"sync"

	"blonymonitorv2/internal/config"
)

// UploadSettings 战斗数据推送配置（供前端展示与修改）。
type UploadSettings struct {
	Enabled        bool   `json:"enabled"`
	Endpoint       string `json:"endpoint"` // 始终留空，保留字段仅用于兼容旧前端。
	EndpointReady  bool   `json:"endpointReady"`
	DungeonKeyword string `json:"dungeonKeyword"`
	SecretReady    bool   `json:"secretReady"`
}

var uploadConfigMu sync.RWMutex

// GetUploadSettings 获取战斗数据推送配置。
func (a *App) GetUploadSettings() UploadSettings {
	uploadConfigMu.RLock()
	defer uploadConfigMu.RUnlock()

	return UploadSettings{
		Enabled:        config.UploadEnabled,
		EndpointReady:  strings.TrimSpace(config.UploadEndpoint) != "",
		DungeonKeyword: config.UploadDungeonKeyword,
		SecretReady:    isUploadSecretConfigured(),
	}
}

// SetUploadSettings 更新战斗数据推送开关（endpoint/密钥等由配置文件注入）。
func (a *App) SetUploadSettings(settings UploadSettings) {
	uploadConfigMu.Lock()
	defer uploadConfigMu.Unlock()

	config.UploadEnabled = settings.Enabled
}

func isUploadEnabled() bool {
	uploadConfigMu.RLock()
	defer uploadConfigMu.RUnlock()
	return config.UploadEnabled
}

func getUploadFilterConfig() (enabled bool, endpoint, keyword string) {
	uploadConfigMu.RLock()
	defer uploadConfigMu.RUnlock()
	return config.UploadEnabled, strings.TrimSpace(config.UploadEndpoint), strings.TrimSpace(config.UploadDungeonKeyword)
}
