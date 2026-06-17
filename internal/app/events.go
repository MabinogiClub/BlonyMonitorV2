package app

import "strconv"

// addConditionEvent ????????
func (a *App) addConditionEvent(entityId uint64, conditionId uint32, isEnable bool, attackerId uint64, disableAt int64, duration int64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := nowCentiseconds()
	entityIdStr := strconv.FormatUint(entityId, 10)
	attackerIdStr := ""
	if attackerId != 0 {
		attackerIdStr = strconv.FormatUint(attackerId, 10)
	}

	if entity, ok := a.entities[entityIdStr]; ok {
		if isEnable {
			exists := false
			for _, cid := range entity.Conditions {
				if cid == conditionId {
					exists = true
					break
				}
			}
			if !exists {
				entity.Conditions = append(entity.Conditions, conditionId)
			}

			if a.buffTimerMgr != nil && duration > 0 {
				a.buffTimerMgr.StartTimer(conditionId, entityId, entity.Name, duration)
			}
		} else {
			for i, cid := range entity.Conditions {
				if cid == conditionId {
					entity.Conditions = append(entity.Conditions[:i], entity.Conditions[i+1:]...)
					break
				}
			}

			if a.buffTimerMgr != nil {
				a.buffTimerMgr.StopTimer(entityId, conditionId)
			}
		}
	}

	entityRaceID := a.getEntityRaceIDUnsafe(entityIdStr)
	entityIsPC := entityRaceID >= 0 && isPC(entityRaceID)
	entityRaceName := ""
	if entityRaceID >= 0 && !entityIsPC {
		entityRaceName = a.getRaceNameUnsafe(entityRaceID)
	}
	a.eventLogs = append(a.eventLogs, EventLog{
		Type:          "condition",
		At:            now,
		EntityID:      entityIdStr,
		EntityName:    a.getEntityNameUnsafe(entityIdStr),
		RaceID:        entityRaceID,
		RaceName:      entityRaceName,
		IsPC:          entityIsPC,
		ConditionID:   conditionId,
		ConditionName: a.getConditionNameUnsafe(conditionId),
		IsEnable:      isEnable,
		AttackerID:    attackerIdStr,
		AttackerName:  a.getEntityNameUnsafe(attackerIdStr),
	})

	if len(a.eventLogs) > 500 {
		a.eventLogs = a.eventLogs[len(a.eventLogs)-500:]
	}
}

// addFinishEvent ??????
func (a *App) addFinishEvent(targetId uint64, attackerId uint64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := nowCentiseconds()
	targetIdStr := strconv.FormatUint(targetId, 10)
	attackerIdStr := ""
	if attackerId != 0 {
		attackerIdStr = strconv.FormatUint(attackerId, 10)
	}

	if targetStat, ok := a.takenStats[targetIdStr]; ok {
		targetStat.deathTime = now
		if a.targetTimerMgr != nil {
			a.targetTimerMgr.OnDeath(targetIdStr)
		}
	}

	targetRaceID := a.getEntityRaceIDUnsafe(targetIdStr)
	targetIsPC := targetRaceID >= 0 && isPC(targetRaceID)
	targetRaceName := ""
	if targetRaceID >= 0 && !targetIsPC {
		targetRaceName = a.getRaceNameUnsafe(targetRaceID)
	}
	a.eventLogs = append(a.eventLogs, EventLog{
		Type:         "finish",
		At:           now,
		EntityID:     targetIdStr,
		EntityName:   a.getEntityNameUnsafe(targetIdStr),
		RaceID:       targetRaceID,
		RaceName:     targetRaceName,
		IsPC:         targetIsPC,
		AttackerID:   attackerIdStr,
		AttackerName: a.getEntityNameUnsafe(attackerIdStr),
	})

	if len(a.eventLogs) > 500 {
		a.eventLogs = a.eventLogs[len(a.eventLogs)-500:]
	}
}

// GetEventLogs ?????? (?????)
func (a *App) GetEventLogs(limit int, filter string) []EventLog {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if limit <= 0 || limit > len(a.eventLogs) {
		limit = len(a.eventLogs)
	}

	result := make([]EventLog, 0, limit)
	for i := len(a.eventLogs) - 1; i >= 0 && len(result) < limit; i-- {
		log := a.eventLogs[i]
		if filter != "" && filter != "all" && log.Type != filter {
			continue
		}
		result = append(result, log)
	}

	return result
}
