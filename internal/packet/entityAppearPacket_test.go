package packet

import "testing"

const testSummonOwnerID uint64 = 4_503_599_631_398_952

func shiftedSummonOwnerSignature(ownerID uint64) Message {
	return Message{
		NewMessageElemByte(1),
		NewMessageElemByte(1),
		NewMessageElemFloat(120),
		NewMessageElemByte(1),
		NewMessageElemLong(0),
		NewMessageElemFloat(1),
		NewMessageElemInt(0),
		NewMessageElemLong(ownerID),
		NewMessageElemByte(11),
		NewMessageElemByte(0),
		NewMessageElemFloat(1.79),
	}
}

func TestParseEntityOwnerIDFromShiftedSummonLayout(t *testing.T) {
	msg := Message{
		NewMessageElemInt(0),
		NewMessageElemInt(0),
		NewMessageElemInt(0),
		NewMessageElemInt(0),
	}
	msg = append(msg, shiftedSummonOwnerSignature(testSummonOwnerID)...)

	ownerID, ok := parseEntityOwnerID(msg)
	if !ok || ownerID != testSummonOwnerID {
		t.Fatalf("parseEntityOwnerID() = (%d, %v), want (%d, true)", ownerID, ok, testSummonOwnerID)
	}
}

func TestParseEntityOwnerIDIgnoresObservedPetLayout(t *testing.T) {
	msg := Message{
		NewMessageElemByte(9),
		NewMessageElemByte(0),
		NewMessageElemLong(0),
		NewMessageElemByte(1),
		NewMessageElemLong(0),
		NewMessageElemFloat(1),
		NewMessageElemInt(0),
		NewMessageElemLong(testSummonOwnerID),
		NewMessageElemByte(2),
		NewMessageElemByte(0),
		NewMessageElemFloat(0.42),
	}

	if ownerID, ok := parseEntityOwnerID(msg); ok {
		t.Fatalf("parseEntityOwnerID() = (%d, true), want no owner", ownerID)
	}
}

func TestParseEntityAppearPacketFallsBackForShiftedSummon(t *testing.T) {
	msg := Message{
		NewMessageElemLong(9_001),
		NewMessageElemByte(5),
		NewMessageElemString("Puppet"),
		NewMessageElemInt(0),
		NewMessageElemInt(0),
		NewMessageElemInt(990_104),
	}
	msg = append(msg, shiftedSummonOwnerSignature(testSummonOwnerID)...)

	entity, err := ParseEntityAppearPacket(msg)
	if err != nil {
		t.Fatalf("ParseEntityAppearPacket() error = %v", err)
	}
	if entity == nil || entity.Id != 9_001 || entity.OwnerId != testSummonOwnerID {
		t.Fatalf("ParseEntityAppearPacket() = %#v", entity)
	}
}

func TestParseEntityAppearPacketKeepsShiftedEntityWithoutOwner(t *testing.T) {
	msg := Message{
		NewMessageElemLong(9_001),
		NewMessageElemByte(5),
		NewMessageElemString("Puppet"),
		NewMessageElemInt(0),
		NewMessageElemInt(0),
		NewMessageElemInt(990_104),
	}
	msg = append(msg, shiftedSummonOwnerSignature(0)...)

	entity, err := ParseEntityAppearPacket(msg)
	if err != nil {
		t.Fatalf("ParseEntityAppearPacket() error = %v", err)
	}
	if entity == nil || entity.Id != 9_001 || entity.OwnerId != 0 {
		t.Fatalf("ParseEntityAppearPacket() = %#v", entity)
	}
}
