package app

import (
	"sort"
	"strconv"
	"time"

	"blonymonitorv2/db"
	"blonymonitorv2/internal/packet"
)

// 自己装备栏（不含衣装）中需要观察的 pocket。
var selfEquipmentPockets = map[uint32]string{
	5:  "衣服",
	6:  "手部",
	7:  "脚部",
	8:  "头部",
	9:  "长袍/翅膀",
	10: "武器I·主手",
	11: "武器II·主手",
	12: "武器I·副手",
	13: "武器II·副手",
	14: "武器I·副手",
	16: "左侧首饰",
	17: "右侧首饰",
}

type SelfEquipmentSlot struct {
	PocketType        uint32             `json:"pocketType"`
	SlotLabel         string             `json:"slotLabel"`
	ItemID            uint32             `json:"itemId,omitempty"`
	InstanceID        uint64             `json:"instanceId,omitempty"`
	Name              string             `json:"name,omitempty"`
	DurabilityValue   *float32           `json:"durabilityValue,omitempty"`
	DurabilityMax     *float32           `json:"durabilityMax,omitempty"`
	DurabilityRaw     *uint32            `json:"durabilityRaw,omitempty"`
	PreviousValue     *float32           `json:"previousValue,omitempty"`
	DurabilityDelta   *float32           `json:"durabilityDelta,omitempty"`
	UpdatedAt         int64              `json:"updatedAt,omitempty"`
	DurabilityHistory []DurabilitySample `json:"durabilityHistory,omitempty"`
}

type SelfEquipmentState struct {
	Ready      bool                `json:"ready"`
	Character  string              `json:"character,omitempty"`
	CapturedAt int64               `json:"capturedAt,omitempty"`
	Slots      []SelfEquipmentSlot `json:"slots"`
}

func (a *App) handleSelfInventoryPacket(pkt *packet.GamePacket) {
	if pkt == nil {
		return
	}
	parsed, err := packet.ParseCharacterDataPacket(pkt)
	if err != nil || parsed == nil || parsed.Id == 0 || !isPC(int(parsed.RaceId)) {
		// 宠物/召唤物也会发 0x5209，不能覆盖玩家装备快照。
		return
	}
	characterID := strconv.FormatUint(parsed.Id, 10)

	a.mu.RLock()
	knownSelf := a.selfId
	a.mu.RUnlock()
	if knownSelf != "" && knownSelf != characterID {
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
	characterName := parsed.Name

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.selfEquipmentItemCache == nil {
		a.selfEquipmentItemCache = make(map[uint64]SelfEquipmentSlot)
	}

	byPocket := make(map[uint32]SelfEquipmentSlot, len(selfEquipmentPockets))
	instanceIndex := make(map[uint64]uint32, len(selfEquipmentPockets))
	for _, detail := range details {
		if detail.Item == nil {
			continue
		}
		pocket := detail.Item.PocketType
		label := selfEquipmentPockets[pocket]
		slot := selfEquipmentSlotFromDetail(detail, label, capturedAt)
		if detail.InstanceID != 0 {
			a.selfEquipmentItemCache[detail.InstanceID] = slot
		}
		if _, watched := selfEquipmentPockets[pocket]; !watched {
			continue
		}
		byPocket[pocket] = slot
		if detail.InstanceID != 0 {
			instanceIndex[detail.InstanceID] = pocket
		}
	}

	a.selfEquipmentSlots = byPocket
	a.selfEquipmentByInstance = instanceIndex
	a.selfEquipmentCapturedAt = capturedAt
	if characterName != "" {
		a.selfEquipmentCharacter = characterName
	} else if a.selfName != "" {
		a.selfEquipmentCharacter = a.selfName
	}
}

// handleSelfEquipmentItemUpdatePacket 处理 0x59e0 / 0x5bd4：
// 换装、战术栏切武器时，物品会带着新 pocket 与精确耐久推送过来。
func (a *App) handleSelfEquipmentItemUpdatePacket(pkt *packet.GamePacket) {
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

	a.mu.Lock()
	defer a.mu.Unlock()
	for _, detail := range details {
		a.applySelfEquipmentDetailLocked(detail, capturedAt)
	}
}

func (a *App) applySelfEquipmentDetailLocked(detail packet.ItemDetail, capturedAt int64) {
	if detail.Item == nil || detail.InstanceID == 0 {
		return
	}
	if a.selfEquipmentItemCache == nil {
		a.selfEquipmentItemCache = make(map[uint64]SelfEquipmentSlot)
	}

	newPocket := detail.Item.PocketType
	label := selfEquipmentPockets[newPocket]
	slot := selfEquipmentSlotFromDetail(detail, label, capturedAt)
	a.selfEquipmentItemCache[detail.InstanceID] = slot
	a.applySelfEquipmentPocketChangeLocked(detail.InstanceID, newPocket, capturedAt)
}

// handleSelfEquipmentPocketMovePacket 处理手动拖拽换装：0x59de / 0x59df。
func (a *App) handleSelfEquipmentPocketMovePacket(pkt *packet.GamePacket) {
	if pkt == nil {
		return
	}
	moves := packet.ParseItemPocketMovePacket(pkt.Msg)
	if len(moves) == 0 {
		return
	}
	capturedAt := pkt.At.UnixMilli()
	if capturedAt <= 0 {
		capturedAt = time.Now().UnixMilli()
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	for _, move := range moves {
		a.applySelfEquipmentPocketChangeLocked(move.InstanceID, move.ToPocket, capturedAt)
	}
}

func (a *App) applySelfEquipmentPocketChangeLocked(instanceID uint64, newPocket uint32, capturedAt int64) {
	if instanceID == 0 {
		return
	}
	if a.selfEquipmentItemCache == nil {
		a.selfEquipmentItemCache = make(map[uint64]SelfEquipmentSlot)
	}

	oldPocket, hadOld := a.selfEquipmentByInstance[instanceID]
	if !hadOld {
		if slot, ok := a.selfEquipmentSlots[newPocket]; ok && slot.InstanceID == instanceID {
			oldPocket, hadOld = newPocket, true
		}
	}

	if hadOld && oldPocket != newPocket {
		if slot, ok := a.selfEquipmentSlots[oldPocket]; ok && slot.InstanceID == instanceID {
			delete(a.selfEquipmentSlots, oldPocket)
		}
		delete(a.selfEquipmentByInstance, instanceID)
	}

	cached, hasCache := a.selfEquipmentItemCache[instanceID]
	if !hasCache {
		// 尚无缓存时，若只是离开观察栏，上面已清空；进入观察栏则等后续详情包。
		return
	}
	cached.PocketType = newPocket
	if label, ok := selfEquipmentPockets[newPocket]; ok {
		cached.SlotLabel = label
	}
	cached.UpdatedAt = capturedAt
	a.selfEquipmentItemCache[instanceID] = cached

	label, watched := selfEquipmentPockets[newPocket]
	if !watched {
		return
	}

	if existing, ok := a.selfEquipmentSlots[newPocket]; ok && existing.InstanceID != 0 && existing.InstanceID != instanceID {
		if mapped, ok := a.selfEquipmentByInstance[existing.InstanceID]; ok && mapped == newPocket {
			delete(a.selfEquipmentByInstance, existing.InstanceID)
		}
	}

	slot := cached
	slot.SlotLabel = label
	if hadOld && oldPocket == newPocket {
		if prev, ok := a.selfEquipmentSlots[newPocket]; ok && prev.InstanceID == instanceID {
			slot.DurabilityHistory = append([]DurabilitySample(nil), prev.DurabilityHistory...)
			slot.PreviousValue = prev.DurabilityValue
			if prev.DurabilityValue != nil && slot.DurabilityValue != nil {
				delta := *slot.DurabilityValue - *prev.DurabilityValue
				if delta != 0 {
					slot.DurabilityDelta = &delta
				} else {
					slot.DurabilityDelta = nil
				}
			}
			if slot.DurabilityRaw != nil {
				raw := *slot.DurabilityRaw
				if len(slot.DurabilityHistory) == 0 || slot.DurabilityHistory[len(slot.DurabilityHistory)-1].Raw != raw {
					slot.DurabilityHistory = append(slot.DurabilityHistory, DurabilitySample{
						At:    capturedAt,
						Value: *slot.DurabilityValue,
						Raw:   raw,
					})
					if len(slot.DurabilityHistory) > 64 {
						slot.DurabilityHistory = slot.DurabilityHistory[len(slot.DurabilityHistory)-64:]
					}
				}
			}
		}
	}
	a.selfEquipmentSlots[newPocket] = slot
	a.selfEquipmentByInstance[instanceID] = newPocket
	if capturedAt > a.selfEquipmentCapturedAt {
		a.selfEquipmentCapturedAt = capturedAt
	}
}

func (a *App) handleItemDurabilityUpdatePacket(pkt *packet.GamePacket) {
	if pkt == nil {
		return
	}
	update, ok := packet.ParseItemDurabilityUpdatePacket(pkt.Msg)
	if !ok || update.InstanceID == 0 {
		return
	}
	capturedAt := pkt.At.UnixMilli()
	if capturedAt <= 0 {
		capturedAt = time.Now().UnixMilli()
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	pocket, ok := a.selfEquipmentByInstance[update.InstanceID]
	if !ok {
		return
	}
	slot, ok := a.selfEquipmentSlots[pocket]
	if !ok {
		return
	}

	previous := slot.DurabilityValue
	raw := update.DurabilityRaw
	value := update.DurabilityValue
	slot.PreviousValue = previous
	slot.DurabilityRaw = &raw
	slot.DurabilityValue = &value
	slot.UpdatedAt = capturedAt
	if previous != nil {
		delta := value - *previous
		slot.DurabilityDelta = &delta
	} else {
		slot.DurabilityDelta = nil
	}
	if len(slot.DurabilityHistory) == 0 || slot.DurabilityHistory[len(slot.DurabilityHistory)-1].Raw != raw {
		slot.DurabilityHistory = append(slot.DurabilityHistory, DurabilitySample{
			At:    capturedAt,
			Value: value,
			Raw:   raw,
		})
		if len(slot.DurabilityHistory) > 64 {
			slot.DurabilityHistory = slot.DurabilityHistory[len(slot.DurabilityHistory)-64:]
		}
	}
	a.selfEquipmentSlots[pocket] = slot
}

func selfEquipmentSlotFromDetail(detail packet.ItemDetail, label string, capturedAt int64) SelfEquipmentSlot {
	item := detail.Item
	raw := detail.DurabilityRaw
	value := detail.DurabilityValue
	slot := SelfEquipmentSlot{
		PocketType:      item.PocketType,
		SlotLabel:       label,
		ItemID:          item.ItemId,
		InstanceID:      detail.InstanceID,
		DurabilityValue: &value,
		DurabilityMax:   detail.DurabilityMax,
		DurabilityRaw:   &raw,
		UpdatedAt:       capturedAt,
		DurabilityHistory: []DurabilitySample{{
			At:    capturedAt,
			Value: value,
			Raw:   raw,
		}},
	}
	if info := db.GetItemInfo(int(item.ItemId)); info != nil {
		slot.Name = info.DisplayName()
	}
	if slot.Name == "" {
		slot.Name = "物品 #" + strconv.FormatUint(uint64(item.ItemId), 10)
	}
	return slot
}

func (a *App) GetSelfEquipmentState() SelfEquipmentState {
	a.mu.RLock()
	defer a.mu.RUnlock()

	slots := make([]SelfEquipmentSlot, 0, len(selfEquipmentPockets))
	for pocket, label := range selfEquipmentPockets {
		if slot, ok := a.selfEquipmentSlots[pocket]; ok {
			cp := slot
			cp.DurabilityHistory = append([]DurabilitySample(nil), slot.DurabilityHistory...)
			slots = append(slots, cp)
			continue
		}
		slots = append(slots, SelfEquipmentSlot{PocketType: pocket, SlotLabel: label})
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i].PocketType < slots[j].PocketType })

	character := a.selfEquipmentCharacter
	if character == "" {
		character = a.selfName
	}
	return SelfEquipmentState{
		Ready:      len(a.selfEquipmentSlots) > 0,
		Character:  character,
		CapturedAt: a.selfEquipmentCapturedAt,
		Slots:      slots,
	}
}
