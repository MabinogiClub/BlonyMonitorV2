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

// handleChineseName 处理中文名称数据包（地下城中文名晚到）
func (a *App) handleChineseName(pkt *packet.GamePacket) {
	if len(pkt.Msg) < 2 {
		return
	}
	if pkt.Msg[0].Type() != packet.MessageElemTypeByte {
		return
	}
	if pkt.Msg[0].Data().(uint8) != 3 {
		return
	}
	if pkt.Msg[1].Type() != packet.MessageElemTypeString {
		return
	}

	chineseName := pkt.Msg[1].Data().(string)
	if chineseName == "" {
		return
	}

	a.mu.RLock()
	inDungeon := a.currentDungeon != nil
	a.mu.RUnlock()
	if !inDungeon {
		return
	}

	a.mu.Lock()
	if a.dungeonChineseNameReceived {
		a.mu.Unlock()
		return
	}
	a.dungeonChineseNameReceived = true
	a.dungeonSaveName = chineseName
	dungeonID := int(a.currentDungeon.DungeonID)
	a.mu.Unlock()

	logger.Printf("[ChineseName] 更新地下城中文名: %s\n", chineseName)
	a.setCurrentMap(&CurrentMapInfo{
		MapID:     dungeonID,
		MapName:   chineseName,
		LocalName: "地下城",
	})
}

// handleInstanceName 处理副本名称数据包
func (a *App) handleInstanceName(pkt *packet.GamePacket) {
	if len(pkt.Msg) < 7 {
		return
	}
	if pkt.Msg[6].Type() != packet.MessageElemTypeString {
		return
	}

	instanceName := pkt.Msg[6].Data().(string)
	if instanceName == "" {
		return
	}

	var instanceID uint64
	if len(pkt.Msg) > 34 && pkt.Msg[34].Type() == packet.MessageElemTypeString {
		params := pkt.Msg[34].Data().(string)
		if idx := findQMMDGN(params); idx != -1 {
			start := idx + len("QMMDGN:8:")
			end := start
			for end < len(params) && params[end] != ';' {
				end++
			}
			if end > start {
				if id, err := strconv.ParseUint(params[start:end], 10, 64); err == nil {
					instanceID = id
				}
			}
		}
	}

	var refreshMap *CurrentMapInfo

	a.mu.Lock()
	if a.instanceNameReceived && a.currentInstance != nil {
		if a.currentInstance.InstanceName == instanceName {
			if a.currentInstance.InstanceID == 0 && instanceID != 0 {
				a.currentInstance.InstanceID = instanceID
			}
			if a.instanceSaveName == "" {
				a.instanceSaveName = instanceName
			}
			if a.currentMap != nil && isRandomInstanceMapID(a.currentMap.MapID) {
				if a.currentInstance.MapID == 0 {
					a.currentInstance.MapID = uint32(a.currentMap.MapID)
				}
				if a.instanceEnterMapID == 0 {
					a.instanceEnterMapID = a.currentMap.MapID
				}
				if a.currentMap.MapName != instanceName || a.currentMap.LocalName != "副本" {
					refreshMap = &CurrentMapInfo{
						MapID:     a.currentMap.MapID,
						MapName:   instanceName,
						LocalName: "副本",
					}
				}
			}
			a.mu.Unlock()
			if refreshMap != nil {
				logger.Printf("[Instance] 副本名称晚到，刷新当前地图显示: %s\n", instanceName)
				a.setCurrentMap(refreshMap)
			}
			return
		}
		a.mu.Unlock()
		return
	}

	a.instanceNameReceived = true
	a.currentInstance = &InstanceInfo{
		InstanceID:   instanceID,
		InstanceName: instanceName,
		MapID:        0,
		EnteredAt:    time.Now().UnixMilli() / 10,
	}
	a.instanceSaveName = instanceName
	if a.currentMap != nil && isRandomInstanceMapID(a.currentMap.MapID) {
		a.currentInstance.MapID = uint32(a.currentMap.MapID)
		if a.instanceEnterMapID == 0 {
			a.instanceEnterMapID = a.currentMap.MapID
		}
		refreshMap = &CurrentMapInfo{
			MapID:     a.currentMap.MapID,
			MapName:   instanceName,
			LocalName: "副本",
		}
	}
	a.mu.Unlock()

	logger.Printf("[Instance] 保存副本名称: %s (ID: %d)\n", instanceName, instanceID)
	if refreshMap != nil {
		a.setCurrentMap(refreshMap)
	}
}
