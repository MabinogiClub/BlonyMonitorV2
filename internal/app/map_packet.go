package app

import (
	"fmt"
	"strconv"
	"time"

	"blonymonitorv2/db"
	"blonymonitorv2/internal/packet"
)

func (a *App) handleMapChange(pkt *packet.GamePacket) {
	if len(pkt.Msg) < 2 || pkt.Msg[1].Type() != packet.MessageElemTypeInt {
		return
	}

	mapID := int(pkt.Msg[1].Data().(uint32))
	targetEntityId := strconv.FormatUint(pkt.Id, 10)

	a.mu.RLock()
	inDungeon := a.currentDungeon != nil
	inInstance := a.currentInstance != nil
	currentSelfId := a.selfId
	a.mu.RUnlock()

	if currentSelfId != "" && targetEntityId != currentSelfId {
		return
	}

	if inDungeon && mapID >= 10000 && mapID < 20000 {
		a.identifySelfFromMapPacket(targetEntityId, true)
		return
	}

	now := time.Now().UnixMilli()

	if inInstance && isInstanceMapID(mapID) {
		a.mu.RLock()
		existingMapID := a.currentInstance.MapID
		a.mu.RUnlock()
		if existingMapID != 0 && uint32(mapID) == existingMapID {
			logger.Printf("[Instance] 检测到副本内部切换 (%d)，忽略\n", mapID)
			return
		}
	}

	a.mu.RLock()
	oldMapName := ""
	if a.currentMap != nil {
		oldMapName = a.currentMap.LocalName
	}
	leavingDungeon := a.currentDungeon != nil && (mapID < 10000 || mapID >= 20000)
	leavingInstance := a.currentInstance != nil && a.currentInstance.MapID != 0 && !isInstanceMapID(mapID)
	a.mu.RUnlock()

	a.cleanupAndSaveTakenStats(mapID, oldMapName)

	if leavingDungeon || leavingInstance {
		time.Sleep(100 * time.Millisecond)
	}

	a.mu.Lock()
	if a.currentDungeon != nil && (mapID < 10000 || mapID >= 20000) {
		logger.Printf("[Map] 离开地下城: %s\n", a.currentDungeon.DungeonName)
		a.currentDungeon = nil
		a.dungeonLocalName = ""
		a.dungeonSaveName = ""
		a.dungeonChineseNameReceived = false
		a.clearRecentDungeonNameCandidatesUnsafe()
	}

	if a.currentInstance != nil && a.currentInstance.MapID != 0 && !isInstanceMapID(mapID) {
		logger.Printf("[Map] 离开副本: %s\n", a.currentInstance.InstanceName)
		a.resetInstanceStateUnsafe()
	}
	if a.currentInstance != nil && a.currentInstance.MapID == 0 && !isInstanceMapID(mapID) {
		logger.Printf("[Instance] 清除未绑定地图的副本名称: %s\n", a.currentInstance.InstanceName)
		a.resetInstanceStateUnsafe()
	}

	threshold := now - 500
	newEntities := make(map[string]*EntityInfo)
	for id, entity := range a.entities {
		if entity.AddedAt >= threshold {
			newEntities[id] = entity
		}
	}
	a.entities = newEntities
	a.lastMapChangeAt = now

	if a.selfId == "" {
		if targetEntityId != "" && targetEntityId != "0" {
			if entity, ok := newEntities[targetEntityId]; ok && entity.IsPC {
				a.selfId = entity.ID
				a.selfName = entity.Name
				logger.Printf("[Map] 识别玩家自身: %s (ID: %s)\n", entity.Name, entity.ID)
			}
		}
		if a.selfId == "" {
			for _, entity := range newEntities {
				if entity.IsPC {
					a.selfId = entity.ID
					a.selfName = entity.Name
					logger.Printf("[Map] 识别玩家自身(备用): %s (ID: %s)\n", entity.Name, entity.ID)
					break
				}
			}
		}
	}
	selfId := a.selfId
	selfName := a.selfName
	a.mu.Unlock()

	if selfId != "" {
		a.setSelfInfo(selfId, selfName)
	}

	mapInfo := db.NewMinimapInfo_FieldMapInfoList(mapID)
	mapName := mapInfo.MapName
	localName := mapInfo.MapLocalName

	if mapName == "" {
		mapName = fmt.Sprintf("地图 #%d", mapID)
	}
	if localName == "" {
		localName = "未知区域"
	}
	if isInstanceMapID(mapID) {
		mapName, localName = a.resolveInstanceMapDisplay(mapID)
	}

	currentMap := &CurrentMapInfo{
		MapID:     mapID,
		MapName:   mapName,
		LocalName: localName,
	}

	logger.Printf("[Map] 地图切换: %s - %s (ID: %d)\n", localName, mapName, mapID)
	a.setCurrentMap(currentMap)
}

func (a *App) identifySelfFromMapPacket(targetEntityId string, inDungeon bool) {
	a.mu.RLock()
	hasSelf := a.selfId != ""
	a.mu.RUnlock()
	if hasSelf {
		return
	}

	var selfId, selfName string
	a.mu.Lock()
	if a.selfId == "" && targetEntityId != "" && targetEntityId != "0" {
		if entity, ok := a.entities[targetEntityId]; ok && entity.IsPC {
			a.selfId = entity.ID
			a.selfName = entity.Name
			if inDungeon {
				logger.Printf("[Map] 识别玩家自身(地下城): %s (ID: %s)\n", entity.Name, entity.ID)
			} else {
				logger.Printf("[Map] 识别玩家自身: %s (ID: %s)\n", entity.Name, entity.ID)
			}
		}
	}
	if a.selfId == "" {
		for _, entity := range a.entities {
			if entity.IsPC {
				a.selfId = entity.ID
				a.selfName = entity.Name
				if inDungeon {
					logger.Printf("[Map] 识别玩家自身(备用-地下城): %s (ID: %s)\n", entity.Name, entity.ID)
				} else {
					logger.Printf("[Map] 识别玩家自身(备用): %s (ID: %s)\n", entity.Name, entity.ID)
				}
				break
			}
		}
	}
	selfId = a.selfId
	selfName = a.selfName
	a.mu.Unlock()

	if selfId != "" {
		a.setSelfInfo(selfId, selfName)
	}
}
