package packet

import (
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestParseItemDetailPacket(t *testing.T) {
	publicData := make([]byte, 80)
	binary.LittleEndian.PutUint32(publicData[0:], 5)
	binary.LittleEndian.PutUint32(publicData[4:], 2230031)
	binary.LittleEndian.PutUint32(publicData[8:], 0x112233)
	binary.LittleEndian.PutUint16(publicData[36:], 1)
	extendedData := make([]byte, 144)
	binary.LittleEndian.PutUint32(extendedData[0:], 53)
	binary.LittleEndian.PutUint32(extendedData[16:], 19120)
	binary.LittleEndian.PutUint32(extendedData[20:], 20000)
	binary.LittleEndian.PutUint32(extendedData[40:], 9)
	binary.LittleEndian.PutUint32(extendedData[44:], 3)
	msg := Message{
		NewMessageElemLong(99),
		NewMessageElemByte(2),
		NewMessageElemBin(publicData),
		NewMessageElemBin(extendedData),
		NewMessageElemString("QUAL:4:100;IMRC:4:4;IMRBV:f:8.8;MTWR:1:3;UPR1:s:20013,321,1,1000,1;OWNER:s:黑羊黯蚀;MDEF:f:9;MPROT:f:7;IMSIUV180:4:6;"),
		NewMessageElemByte(1),
		NewMessageElemBin(sampleOptionBlock(7, 15, 25, 2002002)),
	}

	details := ParseItemDetailPacket(msg)
	if len(details) != 1 {
		t.Fatalf("ParseItemDetailPacket() returned %d details, want 1", len(details))
	}
	detail := details[0]
	if detail.InstanceID != 99 || detail.Item.ItemId != 2230031 {
		t.Fatalf("unexpected identity: %#v", detail)
	}
	if detail.DurabilityMax == nil || *detail.DurabilityMax != 20 || detail.DurabilityValue < 19 || detail.DurabilityValue > 20 {
		t.Fatalf("durability not parsed from points: value=%v max=%v", detail.DurabilityValue, detail.DurabilityMax)
	}
	if detail.Defense == nil || *detail.Defense != 9 || detail.Protection == nil || *detail.Protection != 3 {
		t.Fatalf("defense/protection missing: %#v", detail)
	}
	if detail.OwnerName != "黑羊黯蚀" || len(detail.EgoEntries) != 1 || len(detail.ReforgeEntries) != 1 {
		t.Fatalf("attributes were not classified: %#v", detail)
	}
	if len(detail.Options) != 1 || len(detail.EnchantEntries) == 0 || detail.Options[0].OptionID != 2002002 {
		t.Fatalf("options were not parsed: %#v", detail)
	}
}

func TestParseItemDetailPacketVariantWithMidStringAndOverflow(t *testing.T) {
	publicData := make([]byte, 80)
	binary.LittleEndian.PutUint32(publicData[0:], 8)
	binary.LittleEndian.PutUint32(publicData[4:], 2210005)
	extendedData := make([]byte, 144)
	binary.LittleEndian.PutUint32(extendedData[16:], 20000)
	binary.LittleEndian.PutUint32(extendedData[20:], 20000)
	msg := Message{
		NewMessageElemLong(4503599627431010),
		NewMessageElemLong(22518913258883849),
		NewMessageElemByte(2),
		NewMessageElemBin(publicData),
		NewMessageElemString(""),
		NewMessageElemBin(extendedData),
		NewMessageElemString("OWNER:s:Veyr;MDEF:f:14;MPROT:f:11;IMRBV:f:16.8;"),
		NewMessageElemString("ATTMIN:2:40;"),
		NewMessageElemByte(0),
	}

	details := ParseItemDetailPacket(msg)
	if len(details) != 1 {
		t.Fatalf("ParseItemDetailPacket() returned %d details, want 1", len(details))
	}
	detail := details[0]
	if detail.InstanceID != 22518913258883849 || detail.Item.ItemId != 2210005 {
		t.Fatalf("unexpected identity: %#v", detail)
	}
	if detail.OwnerName != "Veyr" || detail.AttackMin == nil || *detail.AttackMin != 40 {
		t.Fatalf("overflow attributes were not merged: %#v", detail)
	}
	if detail.DurabilityMax == nil || *detail.DurabilityMax != 20 || detail.DurabilityValue != 20 {
		t.Fatalf("durability max missing: %#v", detail)
	}
}

func TestParseItemDetailPacketAcceptsR143PublicBlock(t *testing.T) {
	// Captured from 0x5bd4 after R143: public item blob grew from 80 to 84 bytes.
	publicData, err := hex.DecodeString("08000000d6b82100ff330000ffffffffffffffff000000002179b3002798f700ebe2fd0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000ffff")
	if err != nil {
		t.Fatal(err)
	}
	extendedData := make([]byte, 144)
	binary.LittleEndian.PutUint32(extendedData[16:], 20000)
	binary.LittleEndian.PutUint32(extendedData[20:], 20000)
	details := ParseItemDetailPacket(Message{
		NewMessageElemLong(99),
		NewMessageElemByte(2),
		NewMessageElemBin(publicData),
		NewMessageElemBin(extendedData),
		NewMessageElemString("OWNER:s:Flandre;"),
	})
	if len(details) != 1 || details[0].Item.ItemId != 2210006 || details[0].Item.PocketType != 8 {
		t.Fatalf("details = %#v", details)
	}
}

func sampleOptionBlock(applyType uint32, statID, statValue int16, optionID uint32) []byte {
	data := make([]byte, 40)
	binary.LittleEndian.PutUint32(data[0:], applyType)
	binary.LittleEndian.PutUint16(data[12:], uint16(statID))
	binary.LittleEndian.PutUint16(data[14:], uint16(statValue))
	binary.LittleEndian.PutUint32(data[36:], optionID)
	return data
}
