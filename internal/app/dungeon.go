package app

import (
	"fmt"
	"strconv"
	"time"

	"blonymonitorv2/db"
	"blonymonitorv2/internal/packet"
)

// ParseDungeonInfoPacket ??????????
func ParseDungeonInfoPacket(pkt *packet.GamePacket) (*DungeonInfo, error) {
	msg := pkt.Msg
	if len(msg) < 13 {
		return nil, fmt.Errorf("????????: %d", len(msg))
	}

	if msg[1].Type() != packet.MessageElemTypeLong {
		return nil, fmt.Errorf("??ID????: %d", msg[1].Type())
	}
	instanceID := msg[1].Data().(uint64)

	if msg[3].Type() != packet.MessageElemTypeString {
		return nil, fmt.Errorf("?????????: %d", msg[3].Type())
	}
	dungeonName := msg[3].Data().(string)

	if msg[4].Type() != packet.MessageElemTypeInt {
		return nil, fmt.Errorf("???ID????: %d", msg[4].Type())
	}
	dungeonID := msg[4].Data().(uint32)

	if msg[5].Type() != packet.MessageElemTypeInt {
		return nil, fmt.Errorf("??????: %d", msg[5].Type())
	}
	seed := msg[5].Data().(uint32)

	if msg[7].Type() != packet.MessageElemTypeInt {
		return nil, fmt.Errorf("??????: %d", msg[7].Type())
	}
	difficulty := msg[7].Data().(uint32)

	if len(msg) < 13 || msg[12].Type() != packet.MessageElemTypeInt {
		return nil, fmt.Errorf("??????")
	}
	floorCount := msg[12].Data().(uint32)

	layoutSize := int(floorCount * 2)
	floorLayout := make([]uint8, 0, layoutSize)
	for i := 13; i < 13+layoutSize && i < len(msg); i++ {
		if msg[i].Type() == packet.MessageElemTypeByte {
			floorLayout = append(floorLayout, msg[i].Data().(uint8))
		}
	}

	return &DungeonInfo{
		InstanceID:  instanceID,
		DungeonName: dungeonName,
		DungeonID:   dungeonID,
		Seed:        seed,
		Difficulty:  difficulty,
		FloorCount:  floorCount,
		FloorLayout: floorLayout,
		EnteredAt:   time.Now().Unix(),
	}, nil
}

// onDungeonEnter ????????
func (a *App) onDungeonEnter(pkt *packet.GamePacket, info *DungeonInfo) {
	var playerIdFromPacket uint64
	if len(pkt.Msg) > 0 && pkt.Msg[0].Type() == packet.MessageElemTypeLong {
		playerIdFromPacket = pkt.Msg[0].Data().(uint64)
	}

	dungeonLocalName := info.DungeonName
	dungeonInfo := db.NewDungeonDB(info.DungeonName)
	if dungeonInfo.LocalName != "" {
		dungeonLocalName = dungeonInfo.LocalName
	}

	now := time.Now().UnixMilli()
	playerIdStr := strconv.FormatUint(playerIdFromPacket, 10)

	a.mu.Lock()
	a.currentDungeon = info

	selfIdChanged := false
	if a.selfId == "" && playerIdFromPacket != 0 {
		a.selfId = playerIdStr
		selfIdChanged = true
	}

	newEntities := make(map[string]*EntityInfo)
	selfIdToKeep := a.selfId
	if selfIdToKeep == "" && playerIdFromPacket != 0 {
		selfIdToKeep = playerIdStr
	}
	if selfIdToKeep != "" {
		if selfEntity, ok := a.entities[selfIdToKeep]; ok {
			newEntities[selfIdToKeep] = selfEntity
		}
	}
	a.entities = newEntities
	a.lastMapChangeAt = now
	selfId := a.selfId
	a.mu.Unlock()

	if selfIdChanged && selfId != "" && a.buffTimerMgr != nil {
		a.buffTimerMgr.SetSelfId(selfId)
	}

	currentMap := &CurrentMapInfo{
		MapID:     int(info.DungeonID),
		MapName:   dungeonLocalName,
		LocalName: "???",
	}

	a.setCurrentMap(currentMap)
}

// onDungeonComplete ????????
func (a *App) onDungeonComplete() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.currentDungeon == nil {
		return
	}

	a.currentDungeon.CompletedAt = time.Now().Unix()
	a.currentDungeon.IsCompleted = true

	logger.Printf("[Dungeon] ?????: %s\n", a.currentDungeon.DungeonName)
	a.currentDungeon = nil
}

// GetCurrentDungeon ?????????
func (a *App) GetCurrentDungeon() *DungeonInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.currentDungeon
}
