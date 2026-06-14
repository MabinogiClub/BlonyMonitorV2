package app

import "sort"

// GetPlayerTimeline 获取指定玩家的时间轴
func (a *App) GetPlayerTimeline(playerId string) PlayerTimeline {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// 收集与该玩家相关的所有事件
	playerEvents := make([]EventLog, 0)
	var startTime, endTime int64

	for _, log := range a.eventLogs {
		// 判断事件是否与该玩家相关
		isRelated := false

		switch log.Type {
		case "damage":
			// 玩家造成伤害或受到伤害
			if log.EntityID == playerId || log.TargetID == playerId {
				isRelated = true
			}
		case "appear":
			// 玩家出现
			if log.EntityID == playerId {
				isRelated = true
			}
		case "condition":
			// 玩家的状态变化
			if log.EntityID == playerId {
				isRelated = true
			}
		case "finish":
			// 玩家击杀或被击杀
			if log.EntityID == playerId || log.AttackerID == playerId {
				isRelated = true
			}
		}

		if isRelated {
			playerEvents = append(playerEvents, log)
			// 更新时间范围
			if startTime == 0 || log.At < startTime {
				startTime = log.At
			}
			if log.At > endTime {
				endTime = log.At
			}
		}
	}

	// 按时间排序（从早到晚）
	sort.Slice(playerEvents, func(i, j int) bool {
		return playerEvents[i].At < playerEvents[j].At
	})

	// 获取玩家名称
	playerName := a.getEntityNameUnsafe(playerId)

	return PlayerTimeline{
		PlayerID:   playerId,
		PlayerName: playerName,
		StartTime:  startTime,
		EndTime:    endTime,
		Events:     playerEvents,
	}
}
