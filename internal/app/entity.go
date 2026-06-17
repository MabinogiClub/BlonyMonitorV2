package app

import (
	"sort"
	"strconv"
	"time"

	"blonymonitorv2/db"
	"blonymonitorv2/internal/packet"
)

func (a *App) addEntity(entity *packet.EntityInfo) {
	if entity == nil {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	idStr := strconv.FormatUint(entity.Id, 10)
	raceId := int(entity.RaceId)
	now := time.Now().UnixMilli()

	a.creatureLib[idStr] = entity.Name
	raceIdCache[idStr] = raceId

	_, exists := a.entities[idStr]
	isPc := isPC(raceId)

	a.entities[idStr] = &EntityInfo{
		ID:              idStr,
		Name:            entity.Name,
		RaceID:          raceId,
		IsPC:            isPc,
		Conditions:      make([]uint32, 0),
		SkinColor:       entity.SkinColor,
		EyeType:         entity.EyeType,
		LeftEyeColor:    entity.LeftEyeColor,
		RightEyeColor:   entity.RightEyeColor,
		MouthType:       entity.MouthType,
		Height:          entity.Height,
		Weight:          entity.Weight,
		Upper:           entity.Upper,
		Lower:           entity.Lower,
		CombatPower:     entity.CombatPower,
		TitleID:         entity.TitleId,
		SubTitleID:      entity.SubTitleId,
		StyleTitleID:    entity.StyleTitleId,
		StyleSubTitleID: entity.StyleSubTitleId,
		GuildName:       entity.GuildName,
		OwnerID:         entity.OwnerId,
		HP:              entity.HP,
		MaxHP:           entity.MaxHP,
		MP:              entity.MP,
		MaxMP:           entity.MaxMP,
		Stamina:         entity.Stamina,
		MaxStamina:      entity.MaxStamina,
		AddedAt:         now,
	}

	if !exists {
		a.eventLogs = append(a.eventLogs, EventLog{
			Type:       "appear",
			At:         nowCentiseconds(),
			EntityID:   idStr,
			EntityName: entity.Name,
			RaceID:     raceId,
			RaceName:   a.getRaceNameUnsafe(raceId),
			IsPC:       isPc,
		})

		if len(a.eventLogs) > 500 {
			a.eventLogs = a.eventLogs[len(a.eventLogs)-500:]
		}
	}
}

func (a *App) getRaceNameUnsafe(raceId int) string {
	raceInfo := db.NewRace(raceId)
	return raceInfo.GetName()
}

var raceIdCache = make(map[string]int)

func (a *App) getEntityRaceIDUnsafe(id string) int {
	if e, ok := a.entities[id]; ok {
		return e.RaceID
	}
	if raceId, ok := raceIdCache[id]; ok {
		return raceId
	}
	return -1
}

func (a *App) getEntityNameUnsafe(id string) string {
	if e, ok := a.entities[id]; ok {
		return e.Name
	}
	if name, ok := a.creatureLib[id]; ok {
		return name
	}
	if len(id) > 6 {
		return id[len(id)-6:]
	}
	return id
}

func (a *App) GetAllPCEntities() []PCEntityInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()

	entityConditions := make(map[string]map[uint32]bool)
	for _, log := range a.eventLogs {
		if log.Type != "condition" {
			continue
		}
		if entityConditions[log.EntityID] == nil {
			entityConditions[log.EntityID] = make(map[uint32]bool)
		}
		if log.IsEnable {
			entityConditions[log.EntityID][log.ConditionID] = true
		} else {
			delete(entityConditions[log.EntityID], log.ConditionID)
		}
	}

	result := make([]PCEntityInfo, 0)
	for _, entity := range a.entities {
		if !entity.IsPC {
			continue
		}

		conditions := make([]uint32, 0)
		if condMap, ok := entityConditions[entity.ID]; ok {
			for condId := range condMap {
				conditions = append(conditions, condId)
			}
		}
		sort.Slice(conditions, func(i, j int) bool {
			return conditions[i] < conditions[j]
		})

		conditionNames := make([]string, 0, len(conditions))
		for _, condId := range conditions {
			conditionNames = append(conditionNames, a.getConditionNameUnsafe(condId))
		}

		result = append(result, PCEntityInfo{
			ID:              entity.ID,
			Name:            entity.Name,
			RaceID:          entity.RaceID,
			IsSelf:          entity.ID == a.selfId,
			Conditions:      conditions,
			ConditionNames:  conditionNames,
			SkinColor:       entity.SkinColor,
			EyeType:         entity.EyeType,
			LeftEyeColor:    entity.LeftEyeColor,
			RightEyeColor:   entity.RightEyeColor,
			MouthType:       entity.MouthType,
			Height:          entity.Height,
			Weight:          entity.Weight,
			Upper:           entity.Upper,
			Lower:           entity.Lower,
			CombatPower:     entity.CombatPower,
			TitleID:         entity.TitleID,
			SubTitleID:      entity.SubTitleID,
			StyleTitleID:    entity.StyleTitleID,
			StyleSubTitleID: entity.StyleSubTitleID,
			GuildName:       entity.GuildName,
			OwnerID:         entity.OwnerID,
			HP:              entity.HP,
			MaxHP:           entity.MaxHP,
			MP:              entity.MP,
			MaxMP:           entity.MaxMP,
			Stamina:         entity.Stamina,
			MaxStamina:      entity.MaxStamina,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].IsSelf != result[j].IsSelf {
			return result[i].IsSelf
		}
		return result[i].Name < result[j].Name
	})

	return result
}

func (a *App) GetAllCreatures() []CreatureInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()

	mapChangeThreshold := int64(0)
	if a.lastMapChangeAt > 0 {
		mapChangeThreshold = (a.lastMapChangeAt / 10) - 50
	}

	creatures := make(map[string]*CreatureInfo)
	deadSet := make(map[string]bool)

	for _, log := range a.eventLogs {
		switch log.Type {
		case "appear":
			if mapChangeThreshold > 0 && log.At < mapChangeThreshold {
				continue
			}
			if _, exists := creatures[log.EntityID]; !exists {
				creatures[log.EntityID] = &CreatureInfo{
					ID:         log.EntityID,
					Name:       log.EntityName,
					RaceID:     log.RaceID,
					RaceName:   log.RaceName,
					IsPC:       log.IsPC,
					IsAlive:    true,
					AppearedAt: log.At,
				}
			}
		case "finish":
			if mapChangeThreshold > 0 && log.At < mapChangeThreshold {
				continue
			}
			deadSet[log.EntityID] = true
		}
	}

	for id := range deadSet {
		if creature, ok := creatures[id]; ok {
			creature.IsAlive = false
		}
	}

	result := make([]CreatureInfo, 0, len(creatures))
	for _, creature := range creatures {
		result = append(result, *creature)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].IsPC != result[j].IsPC {
			return result[i].IsPC
		}
		if result[i].IsPC {
			return result[i].Name < result[j].Name
		}
		return result[i].ID < result[j].ID
	})

	return result
}
