package app

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"blonymonitorv2/internal/config"
)

func isUploadSecretConfigured() bool {
	secret := strings.TrimSpace(config.UploadSecret)
	if secret == "" || secret == config.UploadSecretPlaceholder {
		return false
	}
	return true
}

func newUploadNonce() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func hashUploadPayload(gzData []byte) string {
	sum := sha256.Sum256(gzData)
	return hex.EncodeToString(sum[:])
}

// uploadSecretKey 返回 HMAC 密钥字节。若 secret 为标准 Base64 且解码后长度 ≥16，
// 则使用解码结果（与服务端 dpsPusher 一致）；否则使用 UTF-8 明文。
func uploadSecretKey(secret string) []byte {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil
	}
	decoded, err := base64.StdEncoding.DecodeString(secret)
	if err == nil && len(decoded) >= 16 && base64.StdEncoding.EncodeToString(decoded) == secret {
		return decoded
	}
	return []byte(secret)
}

func signBattleUpload(secret string, timestamp int64, nonce, playerID string, gzData []byte) string {
	payload := fmt.Sprintf("%d\n%s\n%s\n%s", timestamp, nonce, playerID, hashUploadPayload(gzData))
	mac := hmac.New(sha256.New, uploadSecretKey(secret))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func verifyBattleUploadSignature(secret string, timestamp int64, nonce, playerID string, gzData []byte, signature string) bool {
	expected := signBattleUpload(secret, timestamp, nonce, playerID, gzData)
	return hmac.Equal([]byte(expected), []byte(strings.TrimSpace(signature)))
}
