package config

import (
	"encoding/base64"
	"strings"
)

// 以下变量仅供 CI 通过 -ldflags 注入，勿在源码中填写真实密钥。
var (
	uploadSecretInjectB64      = ""
	uploadEndpointInject       = ""
	uploadEndpointInjectB64    = ""
	uploadDungeonKeywordInject = ""
	uploadEnabledInject        = ""
)

func applyBuildInjectOverrides() {
	if uploadSecretInjectB64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(uploadSecretInjectB64)
		if err == nil {
			if secret := strings.TrimSpace(string(decoded)); secret != "" && secret != UploadSecretPlaceholder {
				UploadSecret = secret
			}
		}
	}
	if uploadEndpointInjectB64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(uploadEndpointInjectB64)
		if err == nil {
			if endpoint := strings.TrimSpace(string(decoded)); endpoint != "" {
				UploadEndpoint = endpoint
			}
		}
	} else if endpoint := strings.TrimSpace(uploadEndpointInject); endpoint != "" {
		UploadEndpoint = endpoint
	}
	if keyword := strings.TrimSpace(uploadDungeonKeywordInject); keyword != "" {
		UploadDungeonKeyword = keyword
	}
	if enabled := strings.TrimSpace(uploadEnabledInject); enabled != "" {
		UploadEnabled = enabled == "1" || strings.EqualFold(enabled, "true")
	}
}
