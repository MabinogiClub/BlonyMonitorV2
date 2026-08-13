package app

import (
	"testing"

	"blonymonitorv2/internal/packet"
)

func delayedDamagePacket(effectType uint32, skillID uint16, damage uint32) *packet.GamePacket {
	return &packet.GamePacket{
		Op: opcodeEffectDelayed,
		Id: testTargetID,
		Msg: packet.Message{
			packet.NewMessageElemInt(0),
			packet.NewMessageElemInt(effectType),
			packet.NewMessageElemInt(damage),
			packet.NewMessageElemByte(0),
			packet.NewMessageElemInt(9),
			packet.NewMessageElemLong(testPlayerID),
			packet.NewMessageElemShort(skillID),
		},
	}
}

func hydraDamagePacket(effectType uint32, damage uint32) *packet.GamePacket {
	return &packet.GamePacket{
		Op: opcodeEffectDamage,
		Id: testTargetID,
		Msg: packet.Message{
			packet.NewMessageElemInt(effectType),
			packet.NewMessageElemByte(2),
			packet.NewMessageElemInt(damage),
			packet.NewMessageElemInt(9),
			packet.NewMessageElemLong(testPlayerID),
			packet.NewMessageElemShort(35_024),
			packet.NewMessageElemByte(0),
		},
	}
}

func TestProcessPacketRecordsCurrentHydraDamage(t *testing.T) {
	a := newSummonDamageTestApp(t, true)
	a.processPacket(hydraDamagePacket(effectDamageTypeHydra, 2_179))

	if len(a.damages) != 1 {
		t.Fatalf("recorded %d hydra damage events, want 1", len(a.damages))
	}
	record := a.damages[0]
	if record.SkillID != 35_024 || record.RawDamage != 2_179 {
		t.Fatalf("hydra record = %+v", record)
	}
}

func TestProcessPacketRejectsPreviousHydraEffectType(t *testing.T) {
	a := newSummonDamageTestApp(t, true)
	a.processPacket(hydraDamagePacket(353, 2_179))

	if len(a.damages) != 0 {
		t.Fatalf("previous hydra effect type unexpectedly recorded damage: %+v", a.damages)
	}
}

func TestProcessPacketRecordsCurrentDelayedEffectDamage(t *testing.T) {
	a := newSummonDamageTestApp(t, true)
	tests := []struct {
		name    string
		skillID uint16
		damage  uint32
	}{
		{name: "continuity attack", skillID: 58_009, damage: 374_939},
		{name: "blast", skillID: 58_100, damage: 126_971},
		{name: "flare", skillID: 58_101, damage: 419_496},
	}

	for _, test := range tests {
		a.processPacket(delayedDamagePacket(effectDelayedDamageType, test.skillID, test.damage))
	}

	if len(a.damages) != len(tests) {
		t.Fatalf("recorded %d delayed damage events, want %d", len(a.damages), len(tests))
	}
	for i, test := range tests {
		record := a.damages[i]
		if record.SkillID != int(test.skillID) || record.RawDamage != float64(test.damage) {
			t.Fatalf("%s record = %+v", test.name, record)
		}
	}
}

func TestProcessPacketRejectsPreviousDelayedEffectType(t *testing.T) {
	a := newSummonDamageTestApp(t, true)
	a.processPacket(delayedDamagePacket(318, 58_100, 126_971))

	if len(a.damages) != 0 {
		t.Fatalf("previous delayed effect type unexpectedly recorded damage: %+v", a.damages)
	}
}
