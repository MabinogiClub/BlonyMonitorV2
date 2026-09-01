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

func TestRestorePersistentConditionsDoesNotReplaceSelfWithOtherPC(t *testing.T) {
	a := NewApp()
	a.selfId = "4503599638666428"
	a.selfName = "Flandre"
	otherID := "4767482432295257"
	a.entities[otherID] = &EntityInfo{ID: otherID, Name: "Other", IsPC: true}

	a.restorePersistentConditions(&packet.CharacterDataPacket{
		Id:     4767482432295257,
		Name:   "Other",
		RaceId: 10001,
	})

	if a.selfId != "4503599638666428" || a.selfName != "Flandre" {
		t.Fatalf("other PC replaced self character: id=%q name=%q", a.selfId, a.selfName)
	}
}

func TestRestorePersistentConditionsIgnoresPetWithPCRaceData(t *testing.T) {
	a := NewApp()
	petID := "4767482432295257"
	ownerID := uint64(4503599638666428)
	a.entities[petID] = &EntityInfo{
		ID:      petID,
		Name:    "Pet",
		IsPC:    true,
		OwnerID: ownerID,
	}

	a.restorePersistentConditions(&packet.CharacterDataPacket{
		Id:     4767482432295257,
		Name:   "Pet",
		RaceId: 10001,
	})

	if a.selfId != "" {
		t.Fatalf("pet packet established self character: id=%q", a.selfId)
	}
}

func TestIsPCAcceptsAllPlayableRaces(t *testing.T) {
	playableRaces := []int{8001, 8002, 9001, 9002, 10001, 10002}
	for _, raceID := range playableRaces {
		if !isPC(raceID) {
			t.Errorf("playable race %d was rejected", raceID)
		}
	}
}
