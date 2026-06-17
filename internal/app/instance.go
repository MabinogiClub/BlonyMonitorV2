package app

import (
	"strconv"
	"time"

	"blonymonitorv2/internal/packet"
)

func findQMMDGN(params string) int {
	token := "QMMDGN:8:"
	for i := 0; i <= len(params)-len(token); i++ {
		if params[i:i+len(token)] == token {
			return i
		}
	}
	return -1
}

func (a *App) handleChineseName(pkt *packet.GamePacket) {
	chineseName, ok := parseDungeonChineseNamePacket(pkt)
	if !ok {
		return
	}

	nowMillis := time.Now().UnixMilli()
	targetEntityId := strconv.FormatUint(pkt.Id, 10)

	a.mu.Lock()
	a.rememberDungeonNameCandidateUnsafe(chineseName, targetEntityId, nowMillis)
	if a.selfId != "" && targetEntityId != a.selfId {
		a.mu.Unlock()
		logger.Printf("[ChineseName] 忽略非自身目标的地下城中文名候选: target=%s self=%s name=%s\n",
			targetEntityId, a.selfId, chineseName)
		return
	}

	if a.currentDungeon == nil {
		a.mu.Unlock()
		logger.Printf("[ChineseName] 记录提前到达的地下城中文名候选: %s\n", chineseName)
		return
	}
	if a.dungeonChineseNameReceived {
		a.mu.Unlock()
		logger.Printf("[ChineseName] 已收到过地下城中文名，忽略重复包: %s\n", chineseName)
		return
	}
	if !a.inDungeonNameAcceptWindowUnsafe(nowMillis) {
		a.mu.Unlock()
		logger.Printf("[ChineseName] 忽略超过进入窗口的地下城中文名候选: %s\n", chineseName)
		return
	}
	if !dungeonNameHasKeyword(chineseName) {
		a.mu.Unlock()
		logger.Printf("[ChineseName] 记录无地下城关键字的后到候选，等待兜底选择: %s\n", chineseName)
		return
	}

	logger.Printf("[ChineseName] 更新地下城中文名: %s\n", chineseName)
	currentMap, applied := a.applyDungeonChineseNameUnsafe(chineseName)
	a.mu.Unlock()

	if applied {
		a.setCurrentMap(currentMap)
	}
}

func parseDungeonChineseNamePacket(pkt *packet.GamePacket) (string, bool) {
	if len(pkt.Msg) != 2 {
		return "", false
	}
	if pkt.Msg[0].Type() != packet.MessageElemTypeByte || pkt.Msg[1].Type() != packet.MessageElemTypeString {
		return "", false
	}
	if pkt.Msg[0].Data().(uint8) != 3 {
		return "", false
	}

	chineseName := pkt.Msg[1].Data().(string)
	if chineseName == "" {
		return "", false
	}
	return chineseName, true
}

func (a *App) handleInstanceName(pkt *packet.GamePacket) {
	logger.Printf("[Instance Debug] 开始解析副本名称包 - Elements: %d\n", len(pkt.Msg))

	instanceName, instanceID, ok := parseInstanceNamePacket(pkt)
	logger.Printf("[Instance Debug] 提取到副本名称: '%s'\n", instanceName)
	if !ok {
		logger.Printf("[Instance Debug] 忽略非副本实例 0x8ca0 包\n")
		return
	}

	targetEntityId := strconv.FormatUint(pkt.Id, 10)
	a.mu.RLock()
	currentSelfId := a.selfId
	a.mu.RUnlock()
	if currentSelfId != "" && targetEntityId != currentSelfId {
		logger.Printf("[Instance Debug] 忽略非自身目标的副本名候选: target=%s self=%s name=%s\n",
			targetEntityId, currentSelfId, instanceName)
		return
	}

	var refreshMap *CurrentMapInfo

	a.mu.Lock()
	now := nowCentiseconds()
	targetMapID, inInstanceMap, waitingForInstanceName := a.instanceNameTargetMapIDUnsafe(now)
	if a.instanceNameReceived && a.currentInstance != nil {
		sameInstance := a.currentInstance.InstanceID != 0 && a.currentInstance.InstanceID == instanceID
		sameName := a.currentInstance.InstanceName == instanceName
		isPendingBeforeMap := a.currentInstance.MapID == 0 && !inInstanceMap
		isNewWaitedInstance := (waitingForInstanceName || isPendingBeforeMap) && !sameInstance && !sameName
		if isNewWaitedInstance {
			logger.Printf("[Instance] 收到新的副本实例，覆盖待绑定副本名 old=%s new=%s (ID: %d)\n",
				a.currentInstance.InstanceName, instanceName, instanceID)
		}
		if !isNewWaitedInstance && (sameInstance || sameName) {
			if a.currentInstance.InstanceID == 0 && instanceID != 0 {
				a.currentInstance.InstanceID = instanceID
				logger.Printf("[Instance] 更新副本实例ID: %s (ID: %d)\n", instanceName, instanceID)
			}
			if a.instanceSaveName == "" {
				a.instanceSaveName = instanceName
			}
			if targetMapID != 0 {
				if a.currentInstance.MapID == 0 {
					a.currentInstance.MapID = uint32(targetMapID)
				}
				if a.instanceEnterMapID == 0 {
					a.instanceEnterMapID = targetMapID
				}
				if a.currentMap != nil && (a.currentMap.MapName != instanceName || a.currentMap.LocalName != defaultInstanceDisplayName) {
					refreshMap = &CurrentMapInfo{
						MapID:     targetMapID,
						MapName:   instanceName,
						LocalName: defaultInstanceDisplayName,
					}
				}
			}
			a.clearInstanceNameWaitUnsafe()
			a.mu.Unlock()
			if refreshMap != nil {
				logger.Printf("[Instance] 副本名称晚到，刷新当前地图显示: %s\n", instanceName)
				a.setCurrentMap(refreshMap)
			} else {
				logger.Printf("[Instance Debug] 已收到过副本名称，忽略重复包: %s\n", instanceName)
			}
			return
		}

		if !isNewWaitedInstance {
			logger.Printf("[Instance Debug] 已收到过副本名称，忽略不同副本名包 old=%s new=%s\n",
				a.currentInstance.InstanceName, instanceName)
			a.mu.Unlock()
			return
		}
	}

	a.instanceNameReceived = true
	a.currentInstance = &InstanceInfo{
		InstanceID:   instanceID,
		InstanceName: instanceName,
		MapID:        0,
		EnteredAt:    now,
	}
	a.instanceSaveName = instanceName
	if targetMapID != 0 {
		a.currentInstance.MapID = uint32(targetMapID)
		if a.instanceEnterMapID == 0 {
			a.instanceEnterMapID = targetMapID
		}
		if a.currentMap != nil {
			refreshMap = &CurrentMapInfo{
				MapID:     targetMapID,
				MapName:   instanceName,
				LocalName: defaultInstanceDisplayName,
			}
		}
	}
	a.clearInstanceNameWaitUnsafe()
	a.mu.Unlock()

	logger.Printf("[Instance] 保存副本名称: %s (ID: %d)\n", instanceName, instanceID)
	if refreshMap != nil {
		logger.Printf("[Instance] 副本名称晚到，刷新当前地图显示: %s\n", instanceName)
		a.setCurrentMap(refreshMap)
	}
}

func parseInstanceNamePacket(pkt *packet.GamePacket) (string, uint64, bool) {
	if len(pkt.Msg) <= 34 {
		return "", 0, false
	}
	if pkt.Msg[5].Type() != packet.MessageElemTypeInt ||
		pkt.Msg[6].Type() != packet.MessageElemTypeString ||
		pkt.Msg[34].Type() != packet.MessageElemTypeString {
		return "", 0, false
	}

	instanceName := pkt.Msg[6].Data().(string)
	if instanceName == "" {
		return "", 0, false
	}

	instanceID := parseQMMDGNInstanceID(pkt.Msg)
	if instanceID == 0 {
		return instanceName, 0, false
	}
	return instanceName, instanceID, true
}

func parseQMMDGNInstanceID(msg packet.Message) uint64 {
	if len(msg) <= 34 || msg[34].Type() != packet.MessageElemTypeString {
		return 0
	}

	params := msg[34].Data().(string)
	idx := findQMMDGN(params)
	if idx == -1 {
		return 0
	}

	start := idx + len("QMMDGN:8:")
	end := start
	for end < len(params) && params[end] != ';' {
		end++
	}
	if end <= start {
		return 0
	}

	id, err := strconv.ParseUint(params[start:end], 10, 64)
	if err != nil {
		return 0
	}
	return id
}
