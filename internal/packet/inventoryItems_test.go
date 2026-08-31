package packet

import (
	"encoding/binary"
	"testing"
)

func TestParseInventoryItemRecords(t *testing.T) {
	publicData := make([]byte, 80)
	binary.LittleEndian.PutUint32(publicData[0:], 61)
	binary.LittleEndian.PutUint32(publicData[4:], 85692)
	binary.LittleEndian.PutUint16(publicData[36:], 4)
	records := ParseInventoryItemRecords(Message{
		NewMessageElemLong(123),
		NewMessageElemByte(2),
		NewMessageElemBin(publicData),
	})
	if len(records) != 1 || records[0].InstanceID != 123 ||
		records[0].Item.ItemId != 85692 || records[0].Item.Amount != 4 {
		t.Fatalf("records = %#v", records)
	}
}
