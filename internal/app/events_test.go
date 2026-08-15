package app

import (
	"testing"

	"blonymonitorv2/internal/packet"
)

func TestRestorePersistentConditionsIgnoresPetCharacterData(t *testing.T) {
	a := NewApp()
	a.selfId = "4503599638666428"
	a.selfName = "Flandre"

	a.restorePersistentConditions(&packet.CharacterDataPacket{
		Id:     4504699154711619,
		Name:   "Pet",
		RaceId: 490432,
		Conditions: []*packet.CharacterConditionPacket{
			{
				Id:       4504699154711619,
				IsEnable: true,
				EntityCharacterCondition: packet.EntityCharacterCondition{
					CCId:     63,
					Duration: 1800,
				},
			},
		},
	})

	if a.selfId != "4503599638666428" || a.selfName != "Flandre" {
		t.Fatalf("pet replaced self character: id=%q name=%q", a.selfId, a.selfName)
	}
	if len(a.eventLogs) != 0 {
		t.Fatalf("pet conditions were restored: event count = %d", len(a.eventLogs))
	}
}
