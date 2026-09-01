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

func TestParseInventoryItemRecordsKeepsInstanceAttributes(t *testing.T) {
	publicData := make([]byte, 80)
	binary.LittleEndian.PutUint32(publicData[4:], 62059)
	extendedData := make([]byte, 144)
	binary.LittleEndian.PutUint16(extendedData[60:], 21608)
	binary.LittleEndian.PutUint16(extendedData[62:], 31696)
	records := ParseInventoryItemRecords(Message{
		NewMessageElemLong(456),
		NewMessageElemByte(2),
		NewMessageElemBin(publicData),
		NewMessageElemBin(extendedData),
		NewMessageElemString("ENPFIX:4:21638;"),
		NewMessageElemString("LSOW:s:Flandre;"),
	})
	if len(records) != 1 || records[0].Attributes != "ENPFIX:4:21638;LSOW:s:Flandre;" {
		t.Fatalf("records = %#v", records)
	}
	if len(records[0].ExtendedData) != 144 || binary.LittleEndian.Uint16(records[0].ExtendedData[60:]) != 21608 {
		t.Fatalf("extended data = %x", records[0].ExtendedData)
	}
}
