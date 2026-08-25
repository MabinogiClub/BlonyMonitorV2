package packet

// ItemDurabilityUpdate 是 0x5bd5：按物品实例推送的当前耐久点数。
type ItemDurabilityUpdate struct {
	InstanceID      uint64
	DurabilityRaw   uint32
	DurabilityValue float32
}

// ParseItemDurabilityUpdatePacket 解析耐久增量包。
func ParseItemDurabilityUpdatePacket(msg Message) (*ItemDurabilityUpdate, bool) {
	if len(msg) < 2 {
		return nil, false
	}
	if msg[0].Type() != MessageElemTypeLong || msg[1].Type() != MessageElemTypeInt {
		return nil, false
	}
	raw := msg[1].Data().(uint32)
	return &ItemDurabilityUpdate{
		InstanceID:      msg[0].Data().(uint64),
		DurabilityRaw:   raw,
		DurabilityValue: float32(raw) / 1000,
	}, true
}
