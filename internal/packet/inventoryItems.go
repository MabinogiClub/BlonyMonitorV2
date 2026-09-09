package packet

const (
	publicItemBlockSizeLegacy = 80
	// R143 appends 4 trailing bytes (observed as 00 00 FF FF) to the public
	// item blob. Pocket, item ID, colors and stack count stay at the same
	// offsets as the previous 80-byte layout.
	publicItemBlockSizeR143 = 84
)

// IsPublicItemBlock reports whether data is a character/inventory public item
// blob. Older captures are 80 bytes; current China clients send 84 bytes.
func IsPublicItemBlock(data []byte) bool {
	switch len(data) {
	case publicItemBlockSizeLegacy, publicItemBlockSizeR143:
		return true
	default:
		return false
	}
}

// InventoryItemRecord is the stable portion shared by character, bank,
// dressing-room and cash-shop inventory snapshots.
type InventoryItemRecord struct {
	InstanceID   uint64
	Item         *EntityItem
	ExtendedData []byte
	// Attributes is the optional instance attribute string following the
	// public item block (for example ENPFIX/ENSFIX enchant IDs).
	Attributes string
}

// ParseInventoryItemRecords intentionally reads only the public item block.
// Optional extended fields vary between containers and client versions.
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
		if !IsPublicItemBlock(publicData) {
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
		var extendedData []byte
		if index+1 < len(msg) && msg[index+1].Type() == MessageElemTypeBin {
			if data, ok := msg[index+1].Data().([]byte); ok && len(data) == 144 {
				extendedData = append([]byte(nil), data...)
			}
		}
		attributes := ""
		// Inventory snapshots normally carry bin[144] followed by one or more
		// attribute strings. Keep this optional so bank/shop variants remain
		// compatible with older layouts.
		for next := index + 1; next < len(msg) && next <= index+4; next++ {
			if msg[next].Type() != MessageElemTypeString {
				continue
			}
			text, ok := msg[next].Data().(string)
			if ok {
				attributes += text
			}
		}
		records = append(records, InventoryItemRecord{
			InstanceID:   instanceID,
			Item:         item,
			ExtendedData: extendedData,
			Attributes:   attributes,
		})
	}
	return records
}
