package packet

import (
	"testing"
	"time"
)

func TestParseCharacterDataPacketRestoresPersistentConditions(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	packetAt := time.Date(2026, 8, 15, 11, 4, 10, 78_806_900, location)
	message := Message{
		NewMessageElemByte(1),
		NewMessageElemLong(4503599638666428),
		NewMessageElemByte(2),
		NewMessageElemString("Flandre"),
		NewMessageElemString(""),
		NewMessageElemString(""),
		NewMessageElemInt(10001),
		NewMessageElemByte(0),
		NewMessageElemInt(3),
		NewMessageElemInt(63),
		NewMessageElemLong(63922391486217),
		NewMessageElemString("SBT:8:63922391486217;"),
		NewMessageElemLong(0),
		NewMessageElemString(""),
		NewMessageElemString(""),
		NewMessageElemInt(1121),
		NewMessageElemLong(63922390427117),
		NewMessageElemString("DATA:f:0.2;ITEM:4:5000083;DURA:4:1800000;SBT:8:63922390427117;"),
		NewMessageElemLong(0),
		NewMessageElemString(""),
		NewMessageElemString(""),
		NewMessageElemInt(1150),
		NewMessageElemLong(63922390434771),
		NewMessageElemString("DATA:f:0.2;ITEM:4:5000306;DURA:4:1800000;SBT:8:63922390434771;"),
		NewMessageElemLong(0),
		NewMessageElemString(""),
		NewMessageElemString(""),
	}

	parsed, err := ParseCharacterDataPacket(&GamePacket{At: packetAt, Msg: message})
	if err != nil {
		t.Fatalf("parse character data: %v", err)
	}
	if parsed.Id != 4503599638666428 || parsed.Name != "Flandre" || parsed.RaceId != 10001 {
		t.Fatalf("unexpected character: id=%d name=%q race=%d", parsed.Id, parsed.Name, parsed.RaceId)
	}
	if len(parsed.Conditions) != 3 {
		t.Fatalf("condition count = %d, want 3", len(parsed.Conditions))
	}

	wantIDs := []uint32{63, 1121, 1150}
	wantDurations := []int64{2837, 1778, 1785}
	for index, condition := range parsed.Conditions {
		if condition.CCId != wantIDs[index] {
			t.Errorf("condition %d id = %d, want %d", index, condition.CCId, wantIDs[index])
		}
		if condition.Duration != wantDurations[index] {
			t.Errorf("condition %d duration = %d, want remaining %d", condition.CCId, condition.Duration, wantDurations[index])
		}
	}
}
