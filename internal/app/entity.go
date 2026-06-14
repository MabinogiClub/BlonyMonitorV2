package app

import (
	"sort"
	"strconv"
	"time"

	"blonymonitorv2/db"
	"blonymonitorv2/internal/packet"
)

// addEntity 添加实体（完整信息）
func (a *App) addEntity(entity *packet.EntityInfo) {
	if entity == nil {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	idStr := strconv.FormatUint(entity.Id, 10)
	raceId := int(entity.RaceId)
	now := time.Now().UnixMilli()

	// 添加到生物库（持久映射）
	a.creatureLib[idStr] = entity.Name

	// 添加到种族ID缓存
	raceIdCache[idStr] = raceId

	// 检查是否已存在，避免重复记录
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

	// 只记录新出现的实体
	if !exists {
		now := time.Now().Unix()
		a.eventLogs = append(a.eventLogs, EventLog{
			Type:       "appear",
			At:         now,
			EntityID:   idStr,
			EntityName: entity.Name,
			RaceID:     raceId,
			RaceName:   a.getRaceNameUnsafe(raceId),
			IsPC:       isPc,
		})

		// 保留最近 500 条日志
		if len(a.eventLogs) > 500 {
			a.eventLogs = a.eventLogs[len(a.eventLogs)-500:]
		}
	}
}

// getRaceNameUnsafe 获取种族名称（需要在锁内调用）
func (a *App) getRaceNameUnsafe(raceId int) string {
	raceInfo := db.NewRace(raceId)
	return raceInfo.GetName()
}

// raceIdCache 种族ID缓存（避免重复遍历事件日志）
// 注意：此缓存在 App 锁内使用，不需要额外同步
var raceIdCache = make(map[string]int)

// getEntityRaceIDUnsafe 获取实体的种族ID（需要在锁内调用）
// 返回 -1 表示未找到
// 优化：使用缓存避免遍历事件日志
func (a *App) getEntityRaceIDUnsafe(id string) int {
	// 首先检查当前实体映射（最快）
	if e, ok := a.entities[id]; ok {
		return e.RaceID
	}

	// 检查种族ID缓存
	if raceId, ok := raceIdCache[id]; ok {
		return raceId
	}

	// 不再遍历事件日志，返回 -1
	// 如果实体曾经出现过，应该在 entities 或缓存中
	return -1
}

// getEntityNameUnsafe 获取实体名称（需要在锁内调用）
// 优化：优先使用缓存，避免遍历事件日志
func (a *App) getEntityNameUnsafe(id string) string {
	// 首先检查当前实体映射（最快）
	if e, ok := a.entities[id]; ok {
		return e.Name
	}

	// 然后检查生物库（持久映射，第二快）
	if name, ok := a.creatureLib[id]; ok {
		return name
	}

	// 不再遍历事件日志，直接返回ID后缀
	// 事件日志遍历在高频调用时会造成性能问题
	// 生物库已经在 addEntity 时填充，应该能覆盖大部分情况
	if len(id) > 6 {
		return id[len(id)-6:]
	}
	return id
}

// GetAllPCEntities 获取所有PC实体 (供前端调用)
func (a *App) GetAllPCEntities() []PCEntityInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// 从事件日志汇总每个实体的状态
	entityConditions := make(map[string]map[uint32]bool) // entityId -> conditionId -> exists

	for _, log := range a.eventLogs {
		if log.Type == "condition" {
			if entityConditions[log.EntityID] == nil {
				entityConditions[log.EntityID] = make(map[uint32]bool)
			}
			if log.IsEnable {
				entityConditions[log.EntityID][log.ConditionID] = true
			} else {
				delete(entityConditions[log.EntityID], log.ConditionID)
			}
		}
	}

	result := make([]PCEntityInfo, 0)
	for _, entity := range a.entities {
		if entity.IsPC {
			// 使用从事件日志汇总的状态
			conditions := make([]uint32, 0)

			if condMap, ok := entityConditions[entity.ID]; ok {
				for condId := range condMap {
					conditions = append(conditions, condId)
				}
			}

			// 按状态ID排序，保持稳定顺序
			sort.Slice(conditions, func(i, j int) bool {
				return conditions[i] < conditions[j]
			})

			// 生成状态名称列表
			conditionNames := make([]string, 0, len(conditions))
			for _, condId := range conditions {
				conditionNames = append(conditionNames, a.getConditionNameUnsafe(condId))
			}

			// 判断是否为玩家自身
			isSelf := entity.ID == a.selfId

			result = append(result, PCEntityInfo{
				ID:              entity.ID,
				Name:            entity.Name,
				RaceID:          entity.RaceID,
				IsSelf:          isSelf,
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
	}

	// 排序：玩家自身置顶，其他按名字排序
	sort.Slice(result, func(i, j int) bool {
		// 玩家自身优先
		if result[i].IsSelf != result[j].IsSelf {
			return result[i].IsSelf
		}
		// 其他按名字排序
		return result[i].Name < result[j].Name
	})

	return result
}

// GetAllCreatures 获取所有生物（从事件日志中提取）
func (a *App) GetAllCreatures() []CreatureInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// 计算地图切换的时间阈值（转换为秒，因为eventLogs.At是秒级）
	// 保留地图切换前500ms内出现的生物（玩家自身可能在地图切换前加载）
	mapChangeThreshold := int64(0)
	if a.lastMapChangeAt > 0 {
		mapChangeThreshold = (a.lastMapChangeAt - 500) / 1000 // 毫秒转秒，减去500ms
	}

	// 从事件日志中收集所有出现过的生物
	creatures := make(map[string]*CreatureInfo)
	deadSet := make(map[string]bool) // 记录已死亡的生物

	for _, log := range a.eventLogs {
		switch log.Type {
		case "appear":
			// 过滤掉地图切换前出现的生物（但保留500ms内的）
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
			// 只记录地图切换后的死亡事件
			if mapChangeThreshold > 0 && log.At < mapChangeThreshold {
				continue
			}
			deadSet[log.EntityID] = true
		}
	}

	// 更新存活状态
	for id := range deadSet {
		if creature, ok := creatures[id]; ok {
			creature.IsAlive = false
		}
	}

	// 转换为切片并排序
	result := make([]CreatureInfo, 0, len(creatures))
	for _, creature := range creatures {
		result = append(result, *creature)
	}

	// 排序：玩家在最顶部（按名字排序），怪物在下面（按ID排序）
	sort.Slice(result, func(i, j int) bool {
		// 玩家优先
		if result[i].IsPC != result[j].IsPC {
			return result[i].IsPC // PC 排在前面
		}
		// 同类型内部排序
		if result[i].IsPC {
			// 玩家按名字排序
			return result[i].Name < result[j].Name
		}
		// 怪物按ID排序
		return result[i].ID < result[j].ID
	})

	return result
}
