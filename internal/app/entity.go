package app

import (
	"sort"
	"strconv"
	"strings"
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
	eventAt := nowCentiseconds()
	conditions := make([]uint32, 0, len(entity.CharacterConditionMap))
	for conditionID := range entity.CharacterConditionMap {
		conditions = append(conditions, conditionID)
	}
	sort.Slice(conditions, func(i, j int) bool { return conditions[i] < conditions[j] })

	a.creatureLib[idStr] = entity.Name
	raceIdCache[idStr] = raceId

	existingEntity, exists := a.entities[idStr]
	isPc := isPC(raceId)
	ownerID := entity.OwnerId
	if ownerID == 0 && existingEntity != nil {
		ownerID = existingEntity.OwnerID
	}
	equipments := equipmentInfoFromPacket(entity.EquipItemMap)
	a.mergeCachedEquipmentDetailsUnsafe(idStr, equipments)
	if len(equipments) == 0 && existingEntity != nil {
		equipments = existingEntity.Equipments
	}

	a.entities[idStr] = &EntityInfo{
		ID:              idStr,
		Name:            entity.Name,
		RaceID:          raceId,
		IsPC:            isPc,
		Conditions:      conditions,
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
		OwnerID:         ownerID,
		HP:              entity.HP,
		MaxHP:           entity.MaxHP,
		MP:              entity.MP,
		MaxMP:           entity.MaxMP,
		Stamina:         entity.Stamina,
		MaxStamina:      entity.MaxStamina,
		Equipments:      equipments,
		AddedAt:         now,
	}
	a.mergeCachedEquipmentDetailsUnsafe(idStr, a.entities[idStr].Equipments)
	a.absorbSyntheticEquipmentEntityUnsafe(a.entities[idStr])
	for _, conditionID := range conditions {
		a.ensureAppearedStatusUnsafe(entity, conditionID, eventAt)
	}

	if !exists {
		a.eventLogs = append(a.eventLogs, EventLog{
			Type:       "appear",
			At:         eventAt,
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

func (a *App) mergeCachedEquipmentDetailsUnsafe(entityID string, equipments []EquipmentInfo) {
	cached := a.equipmentDetails[entityID]
	if entity, ok := a.entities[entityID]; ok && entity.Name != "" {
		if byName := a.equipmentDetails[entity.Name]; len(byName) > 0 {
			if cached == nil {
				cached = byName
			} else {
				merged := make(map[uint32]EquipmentInfo, len(cached)+len(byName))
				for itemID, detail := range cached {
					merged[itemID] = detail
				}
				for itemID, detail := range byName {
					merged[itemID] = detail
				}
				cached = merged
			}
		}
	}
	if len(cached) == 0 {
		return
	}
	for index := range equipments {
		if detail, ok := cached[equipments[index].ItemID]; ok {
			applyEquipmentDetail(&equipments[index], detail)
		}
	}
}

func equipmentInfoFromPacket(items map[uint32]*packet.EntityItem) []EquipmentInfo {
	result := make([]EquipmentInfo, 0, len(items))
	for _, item := range items {
		if item == nil || item.ItemId == 0 {
			continue
		}
		result = append(result, EquipmentInfo{
			PocketType: item.PocketType,
			ItemID:     item.ItemId,
			Color1:     item.Color1,
			Color2:     item.Color2,
			Color3:     item.Color3,
			Color4:     item.Color4,
			Color5:     item.Color5,
			Color6:     item.Color6,
			Color7:     item.Color7,
			Amount:     item.Amount,
		})
		if info := db.GetItemInfo(int(item.ItemId)); info != nil {
			result[len(result)-1].Name = info.DisplayName()
			result[len(result)-1].Description = info.Description
			result[len(result)-1].Icon = info.Icon
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].PocketType < result[j].PocketType })
	return result
}

func (a *App) handleItemDetailPacket(pkt *packet.GamePacket) {
	if pkt == nil {
		return
	}
	details := packet.ParseItemDetailPacket(pkt.Msg)
	if len(details) == 0 {
		return
	}

	capturedAt := pkt.At.UnixMilli()
	if capturedAt <= 0 {
		capturedAt = time.Now().UnixMilli()
	}
	packetEntityID := strconv.FormatUint(pkt.Id, 10)

	a.mu.Lock()
	defer a.mu.Unlock()
	for _, detail := range details {
		if detail.Item == nil || detail.Item.ItemId == 0 {
			continue
		}
		historyKey := packetEntityID
		if historyKey == "0" {
			historyKey = detail.OwnerName
		}
		history := a.equipmentHistory[historyKey][detail.Item.ItemId]
		if detail.OwnerName != "" && len(history) == 0 {
			history = a.equipmentHistory[detail.OwnerName][detail.Item.ItemId]
		}
		if a.equipmentHistory[historyKey] == nil {
			a.equipmentHistory[historyKey] = make(map[uint32][]DurabilitySample)
		}
		if len(history) == 0 || history[len(history)-1].Raw != detail.DurabilityRaw {
			history = append(history, DurabilitySample{At: capturedAt, Value: detail.DurabilityValue, Raw: detail.DurabilityRaw})
			if len(history) > 32 {
				history = history[len(history)-32:]
			}
		}

		info := equipmentInfoFromDetail(detail, history, capturedAt)
		if a.equipmentDetails[historyKey] == nil {
			a.equipmentDetails[historyKey] = make(map[uint32]EquipmentInfo)
		}
		a.equipmentHistory[historyKey][detail.Item.ItemId] = history
		a.equipmentDetails[historyKey][detail.Item.ItemId] = info
		if detail.OwnerName != "" {
			if a.equipmentHistory[detail.OwnerName] == nil {
				a.equipmentHistory[detail.OwnerName] = make(map[uint32][]DurabilitySample)
			}
			if a.equipmentDetails[detail.OwnerName] == nil {
				a.equipmentDetails[detail.OwnerName] = make(map[uint32]EquipmentInfo)
			}
			a.equipmentHistory[detail.OwnerName][detail.Item.ItemId] = history
			a.equipmentDetails[detail.OwnerName][detail.Item.ItemId] = info
		}

		matched := false
		for entityID, entity := range a.entities {
			if entityID != packetEntityID && (detail.OwnerName == "" || entity.Name != detail.OwnerName) {
				continue
			}
			matched = true
			mergeEquipmentDetail(entity, info)
			if entityID != historyKey {
				if a.equipmentDetails[entityID] == nil {
					a.equipmentDetails[entityID] = make(map[uint32]EquipmentInfo)
				}
				if a.equipmentHistory[entityID] == nil {
					a.equipmentHistory[entityID] = make(map[uint32][]DurabilitySample)
				}
				a.equipmentDetails[entityID][detail.Item.ItemId] = info
				a.equipmentHistory[entityID][detail.Item.ItemId] = history
			}
		}
		if !matched {
			owner := a.ensureEquipmentOwnerEntityUnsafe(detail.OwnerName, packetEntityID, capturedAt)
			mergeEquipmentDetail(owner, info)
			if owner.ID != historyKey {
				if a.equipmentDetails[owner.ID] == nil {
					a.equipmentDetails[owner.ID] = make(map[uint32]EquipmentInfo)
				}
				if a.equipmentHistory[owner.ID] == nil {
					a.equipmentHistory[owner.ID] = make(map[uint32][]DurabilitySample)
				}
				a.equipmentDetails[owner.ID][detail.Item.ItemId] = info
				a.equipmentHistory[owner.ID][detail.Item.ItemId] = history
			}
		}
	}
}

func (a *App) ensureEquipmentOwnerEntityUnsafe(ownerName, packetEntityID string, capturedAt int64) *EntityInfo {
	if ownerName != "" {
		for _, entity := range a.entities {
			if entity != nil && entity.IsPC && entity.Name == ownerName {
				if capturedAt > entity.AddedAt {
					entity.AddedAt = capturedAt
				}
				return entity
			}
		}
	}
	if packetEntityID != "" && packetEntityID != "0" {
		if entity, ok := a.entities[packetEntityID]; ok && entity != nil {
			if ownerName != "" && entity.Name == "" {
				entity.Name = ownerName
			}
			entity.IsPC = true
			if capturedAt > entity.AddedAt {
				entity.AddedAt = capturedAt
			}
			return entity
		}
	}

	id := packetEntityID
	name := ownerName
	if ownerName != "" {
		id = "equip:" + ownerName
	}
	if id == "" || id == "0" {
		id = "equip:unknown"
	}
	if name == "" {
		name = id
	}
	entity := &EntityInfo{
		ID:         id,
		Name:       name,
		IsPC:       true,
		AddedAt:    capturedAt,
		Equipments: make([]EquipmentInfo, 0),
	}
	a.entities[id] = entity
	return entity
}

func (a *App) absorbSyntheticEquipmentEntityUnsafe(entity *EntityInfo) {
	if entity == nil || entity.Name == "" {
		return
	}
	syntheticID := "equip:" + entity.Name
	if syntheticID == entity.ID {
		return
	}
	synthetic, ok := a.entities[syntheticID]
	if !ok || synthetic == nil {
		return
	}
	for _, item := range synthetic.Equipments {
		mergeEquipmentDetail(entity, item)
	}
	if cached := a.equipmentDetails[syntheticID]; len(cached) > 0 {
		if a.equipmentDetails[entity.ID] == nil {
			a.equipmentDetails[entity.ID] = make(map[uint32]EquipmentInfo)
		}
		for itemID, detail := range cached {
			a.equipmentDetails[entity.ID][itemID] = detail
		}
	}
	if history := a.equipmentHistory[syntheticID]; len(history) > 0 {
		if a.equipmentHistory[entity.ID] == nil {
			a.equipmentHistory[entity.ID] = make(map[uint32][]DurabilitySample)
		}
		for itemID, samples := range history {
			a.equipmentHistory[entity.ID][itemID] = samples
		}
	}
	delete(a.entities, syntheticID)
}

func equipmentInfoFromDetail(detail packet.ItemDetail, history []DurabilitySample, capturedAt int64) EquipmentInfo {
	item := detail.Item
	info := EquipmentInfo{
		PocketType:        item.PocketType,
		ItemID:            item.ItemId,
		InstanceID:        detail.InstanceID,
		Color1:            item.Color1,
		Color2:            item.Color2,
		Color3:            item.Color3,
		Color4:            item.Color4,
		Color5:            item.Color5,
		Color6:            item.Color6,
		Color7:            item.Color7,
		Amount:            item.Amount,
		DetailAvailable:   true,
		DurabilityValue:   &detail.DurabilityValue,
		DurabilityMax:     detail.DurabilityMax,
		DurabilityRaw:     &detail.DurabilityRaw,
		DetailCapturedAt:  capturedAt,
		Quality:           detail.Quality,
		RepairCount:       detail.RepairCount,
		RepairBaseValue:   detail.RepairBaseValue,
		MaxWear:           detail.MaxWear,
		MakerName:         detail.MakerName,
		Defense:           detail.Defense,
		Protection:        detail.Protection,
		MagicDefense:      detail.MagicDefense,
		MagicProtection:   detail.MagicProtection,
		AttackMin:         detail.AttackMin,
		UpgradeEntries:    append([]string(nil), detail.UpgradeEntries...),
		SpecialEntries:    append([]string(nil), detail.SpecialEntries...),
		StatEntries:       append([]string(nil), detail.StatEntries...),
		EnchantEntries:    append([]string(nil), detail.EnchantEntries...),
		ReforgeEntries:    append([]string(nil), detail.ReforgeEntries...),
		EgoEntries:        append([]string(nil), detail.EgoEntries...),
		RawAttributes:     detail.Attributes,
		DurabilityHistory: append([]DurabilitySample(nil), history...),
	}
	if len(history) > 1 {
		previous := history[len(history)-2].Value
		delta := detail.DurabilityValue - previous
		info.PreviousDurability = &previous
		info.DurabilityDelta = &delta
	}
	if dbInfo := db.GetItemInfo(int(item.ItemId)); dbInfo != nil {
		info.Name = dbInfo.DisplayName()
		info.Description = dbInfo.Description
		info.Icon = dbInfo.Icon
	}
	for index, entry := range info.EgoEntries {
		info.EgoEntries[index] = enrichEgoEntry(entry)
	}
	for index, entry := range info.ReforgeEntries {
		info.ReforgeEntries[index] = enrichReforgeEntry(entry)
	}
	return info
}

func mergeEquipmentDetail(entity *EntityInfo, detail EquipmentInfo) {
	if entity == nil {
		return
	}
	for index := range entity.Equipments {
		item := &entity.Equipments[index]
		sameInstance := detail.InstanceID != 0 && item.InstanceID == detail.InstanceID
		sameSlotItem := item.ItemID == detail.ItemID && (item.PocketType == detail.PocketType || item.PocketType == 0)
		if !sameInstance && !sameSlotItem {
			continue
		}
		applyEquipmentDetail(item, detail)
		return
	}
	entity.Equipments = append(entity.Equipments, detail)
	sort.Slice(entity.Equipments, func(i, j int) bool {
		return entity.Equipments[i].PocketType < entity.Equipments[j].PocketType
	})
}

func applyEquipmentDetail(item *EquipmentInfo, detail EquipmentInfo) {
	item.InstanceID = detail.InstanceID
	if detail.PocketType != 0 {
		item.PocketType = detail.PocketType
	}
	item.DetailAvailable = detail.DetailAvailable
	item.DurabilityValue = detail.DurabilityValue
	item.DurabilityMax = detail.DurabilityMax
	item.DurabilityRaw = detail.DurabilityRaw
	item.PreviousDurability = detail.PreviousDurability
	item.DurabilityDelta = detail.DurabilityDelta
	item.DurabilityHistory = append([]DurabilitySample(nil), detail.DurabilityHistory...)
	item.DetailCapturedAt = detail.DetailCapturedAt
	item.Quality = detail.Quality
	item.RepairCount = detail.RepairCount
	item.RepairBaseValue = detail.RepairBaseValue
	item.MaxWear = detail.MaxWear
	item.MakerName = detail.MakerName
	item.Defense = detail.Defense
	item.Protection = detail.Protection
	item.MagicDefense = detail.MagicDefense
	item.MagicProtection = detail.MagicProtection
	item.AttackMin = detail.AttackMin
	item.UpgradeEntries = append([]string(nil), detail.UpgradeEntries...)
	item.SpecialEntries = append([]string(nil), detail.SpecialEntries...)
	item.StatEntries = append([]string(nil), detail.StatEntries...)
	item.EnchantEntries = append([]string(nil), detail.EnchantEntries...)
	item.ReforgeEntries = append([]string(nil), detail.ReforgeEntries...)
	item.EgoEntries = append([]string(nil), detail.EgoEntries...)
	item.RawAttributes = detail.RawAttributes
	if detail.Name != "" {
		item.Name = detail.Name
	}
	if detail.Description != "" {
		item.Description = detail.Description
	}
	if detail.Icon != "" {
		item.Icon = detail.Icon
	}
	if detail.Color1 != 0 || detail.Color2 != 0 || detail.Color3 != 0 {
		item.Color1 = detail.Color1
		item.Color2 = detail.Color2
		item.Color3 = detail.Color3
		item.Color4 = detail.Color4
		item.Color5 = detail.Color5
		item.Color6 = detail.Color6
		item.Color7 = detail.Color7
	}
	if detail.Amount != 0 {
		item.Amount = detail.Amount
	}
}

func enrichEgoEntry(entry string) string {
	const marker = "材料ID "
	index := strings.Index(entry, marker)
	if index < 0 {
		return entry
	}
	rest := entry[index+len(marker):]
	idPart := rest
	if cut := strings.IndexAny(rest, " ·,"); cut >= 0 {
		idPart = rest[:cut]
	}
	itemID, err := strconv.Atoi(idPart)
	if err != nil {
		return entry
	}
	info := db.GetItemInfo(itemID)
	if info == nil {
		return entry
	}
	name := info.DisplayName()
	if name == "" {
		return entry
	}
	return strings.Replace(entry, marker+idPart, marker+idPart+"("+name+")", 1)
}

func enrichReforgeEntry(entry string) string {
	const marker = "技能ID "
	index := strings.Index(entry, marker)
	if index < 0 {
		return entry
	}
	rest := entry[index+len(marker):]
	idPart := rest
	if cut := strings.IndexAny(rest, " ·,"); cut >= 0 {
		idPart = rest[:cut]
	}
	skillID, err := strconv.Atoi(idPart)
	if err != nil || skillID <= 0 {
		return entry
	}
	skillName, found := skillNameCache.lookup(skillID)
	if !found {
		return entry
	}
	return strings.Replace(entry, marker+idPart, marker+idPart+"("+skillName+")", 1)
}

func (a *App) getRaceNameUnsafe(raceId int) string {
	return raceNameCache.name(raceId)
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
	if id == a.selfId && a.selfName != "" {
		return a.selfName
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
			Equipments:      append([]EquipmentInfo(nil), entity.Equipments...),
			LastSeenAt:      entity.AddedAt,
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
