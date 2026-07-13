package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"blonymonitorv2/internal/config"
)

// uploadLog 始终输出到控制台，便于 wails dev 调试上传。
var uploadLog = log.New(os.Stdout, "[Upload] ", log.LstdFlags)

const battleUploadTimeout = 15 * time.Second

func shouldUploadBattle(saveName string) bool {
	return uploadBlockReason(saveName) == ""
}

func uploadBlockReason(saveName string) string {
	enabled, endpoint, keyword := getUploadFilterConfig()
	if !enabled {
		return "推送已关闭"
	}
	if strings.TrimSpace(endpoint) == "" {
		return "未配置上传地址"
	}
	if !isUploadSecretConfigured() {
		return "未配置上传密钥"
	}
	if strings.TrimSpace(keyword) == "" {
		return "未配置副本关键字"
	}
	if !saveNameMatchesUploadKeyword(saveName, keyword) {
		return fmt.Sprintf("副本名 %q 不匹配关键字 %q", saveName, keyword)
	}
	return ""
}

func saveNameMatchesUploadKeyword(saveName, keyword string) bool {
	for _, part := range strings.Split(keyword, ",") {
		if kw := strings.TrimSpace(part); kw != "" && strings.Contains(saveName, kw) {
			return true
		}
	}
	return false
}

func minUploadTargetMaxHP(saveName string) int64 {
	if strings.Contains(saveName, "佩斯皮亚德") {
		return config.MinUploadTargetMaxHPPeisiaad
	}
	return config.MinUploadTargetMaxHP
}

func filterSaveDataForUpload(data SaveFileData, saveName string) SaveFileData {
	minHP := minUploadTargetMaxHP(saveName)
	filtered := make([]targetExport, 0, len(data.Targets))
	for _, target := range data.Targets {
		if target.BossHP == nil || target.BossHP.MaxHP < float64(minHP) {
			continue
		}
		filtered = append(filtered, target)
	}
	return SaveFileData{Targets: filtered}
}

// scheduleBattleUpload 在战斗记录保存后异步上传。调用方必须已持有 a.mu。
func (a *App) scheduleBattleUpload(saveData SaveFileData, filePath, saveName string) {
	fileName := filepath.Base(filePath)

	if reason := uploadBlockReason(saveName); reason != "" {
		uploadLog.Printf("跳过上传：%s (dungeon=%s file=%s)\n", reason, saveName, fileName)
		recordUploadSkipped(saveName, fileName, reason)
		return
	}

	playerID := a.selfId
	playerName := a.selfName

	if playerID == "" {
		uploadLog.Printf("跳过上传：未识别到玩家 ID (dungeon=%s file=%s)\n", saveName, fileName)
		recordUploadSkipped(saveName, fileName, "未识别到玩家 ID")
		return
	}

	uploadData := filterSaveDataForUpload(saveData, saveName)
	if len(uploadData.Targets) == 0 {
		uploadLog.Printf("跳过上传：无符合血量条件的目标 (dungeon=%s file=%s)\n", saveName, fileName)
		recordUploadSkipped(saveName, fileName, "无符合血量条件的目标")
		return
	}

	gzData, err := marshalSaveJSON(uploadData)
	if err != nil {
		uploadLog.Printf("序列化失败 (dungeon=%s file=%s): %v\n", saveName, fileName, err)
		recordUploadSkipped(saveName, fileName, fmt.Sprintf("序列化失败: %v", err))
		return
	}

	endpoint := strings.TrimSpace(config.UploadEndpoint)
	dungeonName := saveName
	targetCount := len(uploadData.Targets)
	payloadBytes := len(gzData)

	recordUploadUploading(dungeonName, fileName)
	uploadLog.Printf(
		"开始上传: dungeon=%s file=%s player=%s(%s) targets=%d payload=%d bytes endpoint=%s\n",
		dungeonName, fileName, playerName, playerID, targetCount, payloadBytes, endpoint,
	)

	go func() {
		if err := postBattleUpload(endpoint, playerID, playerName, dungeonName, fileName, gzData, targetCount); err != nil {
			uploadLog.Printf("上传失败: dungeon=%s file=%s err=%v\n", dungeonName, fileName, err)
		}
	}()
}

func postBattleUpload(endpoint, playerID, playerName, dungeonName, fileName string, gzData []byte, targetCount int) error {
	secret := strings.TrimSpace(config.UploadSecret)
	if secret == "" {
		return fmt.Errorf("upload secret not configured")
	}

	nonce, err := newUploadNonce()
	if err != nil {
		recordUploadError(dungeonName, fileName, 0, "", err)
		return err
	}
	timestamp := time.Now().Unix()
	signature := signBattleUpload(secret, timestamp, nonce, playerID, gzData)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	_ = writer.WriteField("playerId", playerID)
	if playerName != "" {
		_ = writer.WriteField("playerName", playerName)
	}
	_ = writer.WriteField("dungeonName", dungeonName)
	_ = writer.WriteField("fileName", fileName)
	_ = writer.WriteField("clientVersion", config.ClientVersion)
	_ = writer.WriteField("contentSha256", hashUploadPayload(gzData))

	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		recordUploadError(dungeonName, fileName, 0, "", err)
		return err
	}
	if _, err := io.Copy(part, bytes.NewReader(gzData)); err != nil {
		recordUploadError(dungeonName, fileName, 0, "", err)
		return err
	}
	if err := writer.Close(); err != nil {
		recordUploadError(dungeonName, fileName, 0, "", err)
		return err
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, &body)
	if err != nil {
		recordUploadError(dungeonName, fileName, 0, "", err)
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "HMAC-SHA256 "+signature)
	req.Header.Set("X-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-Nonce", nonce)

	client := &http.Client{Timeout: battleUploadTimeout}
	resp, err := client.Do(req)
	if err != nil {
		recordUploadError(dungeonName, fileName, 0, "", err)
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	bodyText := strings.TrimSpace(string(respBody))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		uploadErr := fmt.Errorf("server returned %s", resp.Status)
		if bodyText != "" {
			uploadErr = fmt.Errorf("server returned %s: %s", resp.Status, bodyText)
		}
		recordUploadError(dungeonName, fileName, resp.StatusCode, bodyText, uploadErr)
		return uploadErr
	}

	recordUploadSuccess(dungeonName, fileName, resp.StatusCode, bodyText)
	uploadLog.Printf(
		"上传成功: dungeon=%s file=%s status=%d targets=%d payload=%d bytes response=%s\n",
		dungeonName, fileName, resp.StatusCode, targetCount, len(gzData), bodyText,
	)
	return nil
}

func saveNameFromFileName(fileName string) string {
	stem := strings.TrimSuffix(fileName, saveFileExtension)
	parts := strings.SplitN(stem, "_", 3)
	if len(parts) == 3 {
		return parts[2]
	}
	return stem
}

func pickUploaderFromSaveData(data SaveFileData) (string, string) {
	type playerDamage struct {
		id   string
		name string
		dmg  float64
	}
	best := playerDamage{}
	for _, target := range data.Targets {
		for _, attacker := range target.Attackers {
			if !attacker.IsPC || attacker.ID == "" {
				continue
			}
			if attacker.TotalDamage > best.dmg {
				best = playerDamage{id: attacker.ID, name: attacker.Name, dmg: attacker.TotalDamage}
			}
		}
	}
	return best.id, best.name
}

// UploadSaveFile uploads one saved battle file using current upload config.
func UploadSaveFile(filePath, playerID, playerName string) error {
	if !isUploadSecretConfigured() {
		return fmt.Errorf("upload secret not configured")
	}
	endpoint := strings.TrimSpace(config.UploadEndpoint)
	if endpoint == "" {
		return fmt.Errorf("upload endpoint not configured")
	}
	if !config.UploadEnabled {
		return fmt.Errorf("upload is disabled")
	}

	raw, err := readSaveFile(filePath)
	if err != nil {
		return err
	}

	var saveData SaveFileData
	if err := json.Unmarshal(raw, &saveData); err != nil {
		return fmt.Errorf("parse save file: %w", err)
	}

	fileName := filepath.Base(filePath)
	saveName := saveNameFromFileName(fileName)
	if !shouldUploadBattle(saveName) {
		return fmt.Errorf("save name %q does not match upload keyword", saveName)
	}

	if playerID == "" {
		playerID, playerName = pickUploaderFromSaveData(saveData)
	}
	if playerID == "" {
		return fmt.Errorf("player id not found in save file")
	}

	uploadData := filterSaveDataForUpload(saveData, saveName)
	if len(uploadData.Targets) == 0 {
		return fmt.Errorf("no upload-eligible targets in %q", saveName)
	}

	gzData, err := marshalSaveJSON(uploadData)
	if err != nil {
		return err
	}

	uploadLog.Printf("CLI 上传: file=%s targets=%d payload=%d bytes\n", fileName, len(uploadData.Targets), len(gzData))
	return postBattleUpload(endpoint, playerID, playerName, saveName, fileName, gzData, len(uploadData.Targets))
}
