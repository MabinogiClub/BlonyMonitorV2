package app

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"blonymonitorv2/db"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const saveFileExtension = ".json.gz"

// SaveFileData 战斗历史保存文件根结构。
type SaveFileData struct {
	Targets []targetExport `json:"targets"`
}

type skillExport struct {
	SkillDamageStats
	HitRecords []SkillHitRecord `json:"hitRecords,omitempty"`
}

type attackerExport struct {
	AttackerWithSkills
	SkillsDetail []skillExport `json:"skillsDetail,omitempty"`
}

type targetExport struct {
	TargetID    string           `json:"targetId"`
	TargetName  string           `json:"targetName"`
	TotalDamage float64          `json:"totalDamage"`
	DPS         float64          `json:"dps"`
	Duration    int64            `json:"duration"`
	Attackers   []attackerExport `json:"attackers"`
	CleanedAt   int64            `json:"cleanedAt"`
	AppearedAt  int64            `json:"appearedAt"`
	DeathTime   int64            `json:"deathTime,omitempty"`
	BossHP      *BossHPExport    `json:"bossHP,omitempty"`
}

// BossHPExport Boss HP 时间线导出。
type BossHPExport struct {
	EntityID string         `json:"entityId"`
	RaceID   int            `json:"raceId"`
	MaxHP    float64        `json:"maxHp"`
	History  []BossHPRecord `json:"history"`
}

func (a *App) saveDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return "save"
	}
	return filepath.Join(filepath.Dir(exePath), "save")
}

func cloneSortedSkillHitRecords(records []SkillHitRecord) []SkillHitRecord {
	result := make([]SkillHitRecord, len(records))
	copy(result, records)
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Timestamp != result[j].Timestamp {
			return result[i].Timestamp < result[j].Timestamp
		}
		iSeq := result[i].Seq
		jSeq := result[j].Seq
		if iSeq <= 0 {
			iSeq = math.MaxInt64
		}
		if jSeq <= 0 {
			jSeq = math.MaxInt64
		}
		return iSeq < jSeq
	})
	return result
}

func cloneSortedBossHPRecords(records []BossHPRecord) []BossHPRecord {
	result := make([]BossHPRecord, len(records))
	copy(result, records)
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].HpTimestamp < result[j].HpTimestamp
	})
	return result
}

func (a *App) buildTargetExport() []targetExport {
	if len(a.takenStats) == 0 {
		return nil
	}

	now := time.Now().Unix()
	result := make([]targetExport, 0, len(a.takenStats))

	for id, stat := range a.takenStats {
		attackers := make([]attackerExport, 0)
		for attackerID, attackerStat := range stat.attackers {
			percent := 0.0
			if stat.total > 0 {
				percent = (attackerStat.total / stat.total) * 100
			}

			endTime := stat.lastHit
			if stat.deathTime > 0 {
				endTime = stat.deathTime
			}
			duration := endTime - stat.firstHit
			if duration < 1 {
				duration = 1
			}
			dps := attackerStat.total / float64(duration)

			skills := make([]skillExport, 0)
			for skillID, skillStat := range attackerStat.skills {
				skillPercent := 0.0
				if attackerStat.total > 0 {
					skillPercent = (skillStat.total / attackerStat.total) * 100
				}
				avgDamage := 0.0
				if skillStat.hits > 0 {
					avgDamage = skillStat.total / float64(skillStat.hits)
				}

				skills = append(skills, skillExport{
					SkillDamageStats: SkillDamageStats{
						SkillID:       skillID,
						SkillName:     a.getSkillNameUnsafe(skillID),
						TotalDamage:   skillStat.total,
						Percent:       skillPercent,
						HitCount:      skillStat.hits,
						CritCount:     skillStat.crits,
						AvgDamage:     avgDamage,
						MinDamage:     skillStat.min,
						MaxDamage:     skillStat.max,
						CritMinDamage: skillStat.critMin,
						CritMaxDamage: skillStat.critMax,
					},
					HitRecords: cloneSortedSkillHitRecords(skillStat.records),
				})
			}

			sort.Slice(skills, func(i, j int) bool {
				if skills[i].TotalDamage != skills[j].TotalDamage {
					return skills[i].TotalDamage > skills[j].TotalDamage
				}
				return skills[i].SkillID < skills[j].SkillID
			})

			skillStatsOnly := make([]SkillDamageStats, len(skills))
			for i, s := range skills {
				skillStatsOnly[i] = s.SkillDamageStats
			}

			attackers = append(attackers, attackerExport{
				AttackerWithSkills: AttackerWithSkills{
					ID:          attackerID,
					Name:        formatDisplayName(attackerID, attackerStat.name, attackerStat.raceId, attackerStat.isPC),
					TotalDamage: attackerStat.total,
					DPS:         dps,
					Percent:     percent,
					IsPC:        attackerStat.isPC,
					Skills:      skillStatsOnly,
				},
				SkillsDetail: skills,
			})
		}

		sort.Slice(attackers, func(i, j int) bool {
			if attackers[i].TotalDamage != attackers[j].TotalDamage {
				return attackers[i].TotalDamage > attackers[j].TotalDamage
			}
			return attackers[i].ID < attackers[j].ID
		})

		endTime := stat.lastHit
		if stat.deathTime > 0 {
			endTime = stat.deathTime
		}
		duration := endTime - stat.firstHit
		if duration < 1 {
			duration = 1
		}
		targetDps := stat.total / float64(duration)

		var targetBossHP *BossHPExport
		if records, ok := a.bossHPHistory[id]; ok && len(records) > 0 {
			historyCopy := cloneSortedBossHPRecords(records)
			maxHp := 0.0
			raceID := 0
			if info := a.bossHP[id]; info != nil {
				maxHp = info.MaxHP
				raceID = info.RaceID
			} else if len(historyCopy) > 0 {
				maxHp = historyCopy[len(historyCopy)-1].MaxHP
				raceID = historyCopy[len(historyCopy)-1].RaceID
			}
			if maxHp > 0 {
				targetBossHP = &BossHPExport{
					EntityID: id,
					RaceID:   raceID,
					MaxHP:    maxHp,
					History:  historyCopy,
				}
			}
		}

		result = append(result, targetExport{
			TargetID:    id,
			TargetName:  formatDisplayName(id, stat.name, stat.raceId, stat.isPC),
			TotalDamage: stat.total,
			DPS:         targetDps,
			Duration:    duration,
			Attackers:   attackers,
			CleanedAt:   now,
			AppearedAt:  stat.firstHit,
			DeathTime:   stat.deathTime,
			BossHP:      targetBossHP,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].TotalDamage != result[j].TotalDamage {
			return result[i].TotalDamage > result[j].TotalDamage
		}
		return result[i].TargetID < result[j].TargetID
	})

	return result
}

func (a *App) buildSaveFileData() SaveFileData {
	a.capAllDeadTargetDamageToMaxHPUnsafe()
	return SaveFileData{
		Targets: a.buildTargetExport(),
	}
}

func marshalSaveJSON(v interface{}) ([]byte, error) {
	jsonData, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(jsonData); err != nil {
		_ = gz.Close()
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeSaveFile(filePath string, v interface{}) (string, error) {
	jsonData, err := marshalSaveJSON(v)
	if err != nil {
		return filePath, err
	}

	finalPath := uniqueSaveFilePath(filePath)
	return finalPath, writeFileAtomic(finalPath, jsonData)
}

func uniqueSaveFilePath(filePath string) string {
	stem := filePath
	ext := filepath.Ext(filePath)
	if strings.HasSuffix(filePath, saveFileExtension) {
		ext = saveFileExtension
		stem = strings.TrimSuffix(filePath, saveFileExtension)
	} else if ext != "" {
		stem = strings.TrimSuffix(filePath, ext)
	}

	candidate := filePath
	for i := 2; ; i++ {
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		} else if err != nil {
			return candidate
		}
		candidate = fmt.Sprintf("%s_%d%s", stem, i, ext)
	}
}

func writeFileAtomic(filePath string, data []byte) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(filePath)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	keepTemp := false
	defer func() {
		if !keepTemp {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, filePath); err != nil {
		return err
	}
	keepTemp = true
	return nil
}

func countBossHPTargets(targets []targetExport) int {
	count := 0
	for _, t := range targets {
		if t.BossHP != nil {
			count++
		}
	}
	return count
}

func readSaveFile(filePath string) ([]byte, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	if filepath.Ext(filePath) != ".gz" {
		return data, nil
	}

	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	return io.ReadAll(gz)
}

func (a *App) resolveSaveName(mapName string) string {
	if a.currentDungeon != nil {
		if a.dungeonSaveName != "" {
			return a.dungeonSaveName
		}
		dungeonLocalName := a.currentDungeon.DungeonName
		dungeonInfo := db.NewDungeonDB(a.currentDungeon.DungeonName)
		if dungeonInfo.LocalName != "" {
			dungeonLocalName = dungeonInfo.LocalName
		}
		return dungeonLocalName
	}
	if a.currentInstance != nil && a.currentInstance.InstanceName != "" {
		if a.instanceSaveName != "" {
			return a.instanceSaveName
		}
		return a.currentInstance.InstanceName
	}
	if a.currentMap != nil {
		if a.currentMap.LocalName != "" && a.currentMap.LocalName != "???" && a.currentMap.LocalName != "副本" {
			return a.currentMap.LocalName
		}
		if a.currentMap.MapName != "" && a.currentMap.MapName != "副本" {
			return a.currentMap.MapName
		}
	}
	if a.instanceSaveName != "" {
		return a.instanceSaveName
	}
	if mapName != "" && mapName != "???" && mapName != "副本" {
		return mapName
	}
	return "战斗记录"
}

func (a *App) saveTakenStatsLocked(saveName string) (string, int, int, error) {
	saveData := a.buildSaveFileData()
	if len(saveData.Targets) == 0 {
		return "", 0, 0, nil
	}

	timeStr := time.Now().Format("2006-01-02_15-04-05")
	fileName := fmt.Sprintf("%s_%s%s", timeStr, saveName, saveFileExtension)
	filePath := filepath.Join(a.saveDir(), fileName)

	finalPath, err := writeSaveFile(filePath, saveData)
	if err != nil {
		return "", 0, 0, err
	}
	a.emitHistorySaved(finalPath)
	return finalPath, len(saveData.Targets), countBossHPTargets(saveData.Targets), nil
}

func (a *App) emitHistorySaved(filePath string) {
	if a.ctx == nil || filePath == "" {
		return
	}
	runtime.EventsEmit(a.ctx, "history-saved", map[string]string{
		"path": filePath,
		"file": filepath.Base(filePath),
	})
}

// GetSaveDir 返回战斗记录保存目录（供前端展示与调试）。
func (a *App) GetSaveDir() string {
	return a.saveDir()
}

func (a *App) clearDamageStateUnsafe() {
	a.takenStats = make(map[string]*targetAggStats)
	a.targetDamages = make(map[string][]DamageRecord)
	a.attackerStats = make(map[string]*attackerAggStats)
	a.skillStats = make(map[string]map[int]*skillAggStats)
	a.totalDamage = 0
	a.damages = make([]DamageRecord, 0)
	a.damageSeq = 0
	a.chartAggData = make(map[string]*chartAttackerData)
	a.targetChartAggData = make(map[string]map[string]*chartAttackerData)
	a.damageSeqAtLastAutoSave = 0
	a.clearAllBossHPUnsafe()
}

// cleanupAndSaveTakenStats 场景/副本切换时保存战斗记录，但不清空内存中的统计数据。
func (a *App) cleanupAndSaveTakenStats(mapID int, mapName string) {
	a.mu.RLock()
	oldMapID := 0
	if a.currentMap != nil {
		oldMapID = a.currentMap.MapID
	}
	currentDungeon := a.currentDungeon
	currentInstance := a.currentInstance
	instanceSaveName := a.instanceSaveName
	damageSeq := a.damageSeq
	damageSeqAtLastAutoSave := a.damageSeqAtLastAutoSave
	a.mu.RUnlock()

	if isRandomInstanceMapID(oldMapID) && isRandomInstanceMapID(mapID) {
		logger.Printf("[Cleanup] 副本内切换 (%d -> %d)，保留数据\n", oldMapID, mapID)
		return
	}

	if damageSeq == damageSeqAtLastAutoSave {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.takenStats) == 0 {
		return
	}

	saveName := a.resolveSaveName(mapName)
	if saveName == "战斗记录" && currentDungeon == nil && currentInstance == nil {
		if isRandomInstanceMapID(oldMapID) && !isRandomInstanceMapID(mapID) && oldMapID > 0 {
			mapInfo := db.NewMinimapInfo_FieldMapInfoList(oldMapID)
			if mapInfo.MapLocalName != "" {
				saveName = mapInfo.MapLocalName
			}
		}
		if saveName == "战斗记录" && instanceSaveName != "" {
			saveName = instanceSaveName
		}
	}

	finalPath, targetCount, bossHPCount, err := a.saveTakenStatsLocked(saveName)
	if err != nil {
		logger.Printf("[Cleanup] 保存失败: %v\n", err)
		return
	}
	if finalPath == "" {
		return
	}

	a.damageSeqAtLastAutoSave = a.damageSeq
	logger.Printf("[Cleanup] 地图切换保存 %d 个目标, %d 个bossHP -> %s（保留当前统计）\n", targetCount, bossHPCount, finalPath)
}

// ClearAndSave 保存并清空数据（供前端调用）。
func (a *App) ClearAndSave() {
	a.mu.Lock()

	if len(a.takenStats) == 0 {
		a.eventLogs = make([]EventLog, 0)
		a.clearDamageStateUnsafe()
		a.mu.Unlock()
		runtime.EventsEmit(a.ctx, "clear")
		return
	}

	saveName := a.resolveSaveName("手动保存")
	finalPath, targetCount, bossHPCount, err := a.saveTakenStatsLocked(saveName)
	if err != nil {
		logger.Printf("[ClearAndSave] 保存失败: %v\n", err)
		a.mu.Unlock()
		return
	}

	if finalPath != "" {
		logger.Printf("[ClearAndSave] 保存 %d 个目标, %d 个bossHP -> %s\n", targetCount, bossHPCount, finalPath)
	}
	a.eventLogs = make([]EventLog, 0)
	a.clearDamageStateUnsafe()
	a.mu.Unlock()

	runtime.EventsEmit(a.ctx, "clear")
}

func (a *App) shutdownSaveData() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.takenStats) == 0 || a.damageSeq == a.damageSeqAtLastAutoSave {
		return
	}

	saveName := a.resolveSaveName("退出保存")
	finalPath, targetCount, bossHPCount, err := a.saveTakenStatsLocked(saveName)
	if err != nil {
		logger.Printf("[Shutdown] 保存失败: %v\n", err)
		return
	}
	if finalPath == "" {
		return
	}

	a.damageSeqAtLastAutoSave = a.damageSeq
	logger.Printf("[Shutdown] 保存 %d 个目标, %d 个bossHP -> %s\n", targetCount, bossHPCount, finalPath)
}

// GetCleanedTargetsList 获取所有保存的战斗记录文件列表。
func (a *App) GetCleanedTargetsList() []string {
	saveDir := a.saveDir()
	files := make([]string, 0)

	entries, err := os.ReadDir(saveDir)
	if err != nil {
		return files
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".json") || strings.HasSuffix(name, saveFileExtension) {
			files = append(files, name)
		}
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i] > files[j]
	})

	return files
}

// ReadCleanedTargetFileFull 读取战斗记录文件（完整格式）。
func (a *App) ReadCleanedTargetFileFull(fileName string) interface{} {
	filePath := filepath.Join(a.saveDir(), fileName)

	data, err := readSaveFile(filePath)
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("读取文件失败: %v", err),
		}
	}

	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("解析JSON失败: %v", err),
		}
	}

	if obj, ok := raw.(map[string]interface{}); ok {
		if targets, exists := obj["targets"]; exists {
			targetsArr, ok := targets.([]interface{})
			if !ok {
				return obj
			}

			if oldBossHP, hasOldBossHP := obj["bossHP"]; hasOldBossHP {
				bossHPArr, ok := oldBossHP.([]interface{})
				if ok {
					bossHPMap := make(map[string]interface{})
					for _, bh := range bossHPArr {
						if bhObj, ok := bh.(map[string]interface{}); ok {
							if entityID, ok := bhObj["entityId"].(string); ok {
								bossHPMap[entityID] = bh
							}
						}
					}
					for i, t := range targetsArr {
						if tObj, ok := t.(map[string]interface{}); ok {
							if targetID, ok := tObj["targetId"].(string); ok {
								if hp, found := bossHPMap[targetID]; found {
									tObj["bossHP"] = hp
									targetsArr[i] = tObj
								}
							}
						}
					}
				}
				delete(obj, "bossHP")
			}

			return obj
		}
		return map[string]interface{}{
			"targets": raw,
		}
	}

	return raw
}
