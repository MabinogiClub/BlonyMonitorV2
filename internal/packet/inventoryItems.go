package packet

// InventoryItemRecord is the stable portion shared by character, bank,
// dressing-room and cash-shop inventory snapshots.
type InventoryItemRecord struct {
	InstanceID uint64
	Item       *EntityItem
}

// ParseInventoryItemRecords intentionally reads only the public 80-byte item
// block. Optional extended fields vary between containers and client versions.
func ParseInventoryItemRecords(msg Message) []InventoryItemRecord {
	records := make([]InventoryItemRecord, 0)
	seen := make(map[uint64]struct{})
	for index := 1; index < len(msg); index++ {
		if msg[index].Type() != MessageElemTypeBin ||
			msg[index-1].Type() != MessageElemTypeByte ||
			msg[index-1].Data().(uint8) != 2 {
			continue
		}
		publicData := msg[index].Data().([]byte)
		if len(publicData) != 80 {
			continue
		}
		item, err := EntityItemReader(publicData)
		if err != nil || item.ItemId == 0 {
			continue
		}
		var instanceID uint64
		if index >= 2 && msg[index-2].Type() == MessageElemTypeLong {
			instanceID = msg[index-2].Data().(uint64)
		}
		if instanceID != 0 {
			if _, exists := seen[instanceID]; exists {
				continue
			}
			seen[instanceID] = struct{}{}
		}
		records = append(records, InventoryItemRecord{InstanceID: instanceID, Item: item})
	}
	return records
}
