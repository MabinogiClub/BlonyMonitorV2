package app

import (
	"strconv"
	"testing"

	"blonymonitorv2/db"
	"blonymonitorv2/internal/packet"

	"github.com/jmoiron/sqlx"
)

const (
	testPlayerID uint64 = 4_503_599_631_398_952
	testSummonID uint64 = 4_503_599_631_398_953
	testTargetID uint64 = 4_503_599_631_398_954
)

func entityID(id uint64) string {
	return strconv.FormatUint(id, 10)
}

func newSummonDamageTestApp(t *testing.T, ownerKnown bool) *App {
	t.Helper()
	testDB, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if _, err := testDB.Exec(`CREATE TABLE SkillInfo (
		SkillID INTEGER, SkillEngName TEXT, SkillLocalName TEXT, SkillDesc TEXT, ImageData TEXT
	)`); err != nil {
		t.Fatalf("create SkillInfo table: %v", err)
	}
	previousDB := db.DB
	db.DB = testDB
	t.Cleanup(func() {
		db.DB = previousDB
		_ = testDB.Close()
	})

	a := NewApp()
	if ownerKnown {
		a.entities[entityID(testPlayerID)] = &EntityInfo{
			ID: entityID(testPlayerID), Name: "Player", RaceID: 1, IsPC: true,
		}
	}
	a.entities[entityID(testSummonID)] = &EntityInfo{
		ID: entityID(testSummonID), Name: "Puppet", RaceID: -1, OwnerID: testPlayerID,
	}
	a.entities[entityID(testTargetID)] = &EntityInfo{
		ID: entityID(testTargetID), Name: "Target", RaceID: 1, IsPC: true,
	}
	return a
}

func TestAddDamageCreditsSummonToKnownPCOwner(t *testing.T) {
	a := newSummonDamageTestApp(t, true)
	a.addDamage(testSummonID, testTargetID, 59_167, 1234, true)

	playerID := entityID(testPlayerID)
	summonID := entityID(testSummonID)
	targetID := entityID(testTargetID)
	if len(a.damages) != 1 || a.damages[0].AttackerID != playerID || a.damages[0].AttackerName != "Player" {
		t.Fatalf("damage record = %#v", a.damages)
	}
	if a.attackerStats[playerID] == nil || a.attackerStats[playerID].total != 1234 {
		t.Fatalf("player aggregate = %#v", a.attackerStats[playerID])
	}
	if _, ok := a.attackerStats[summonID]; ok {
		t.Fatalf("summon unexpectedly has a global aggregate")
	}
	if a.takenStats[targetID].attackers[playerID] == nil || a.takenStats[targetID].attackers[summonID] != nil {
		t.Fatalf("target attackers = %#v", a.takenStats[targetID].attackers)
	}
}

func TestAddDamageKeepsSummonSeparateForUnknownNonLocalOwner(t *testing.T) {
	a := newSummonDamageTestApp(t, false)
	a.addDamage(testSummonID, testTargetID, 59_167, 1234, false)

	summonID := entityID(testSummonID)
	targetID := entityID(testTargetID)
	if a.damages[0].AttackerID != summonID || a.damages[0].AttackerName != "Puppet" {
		t.Fatalf("damage record = %#v", a.damages[0])
	}
	if a.takenStats[targetID].attackers[summonID] == nil {
		t.Fatalf("summon missing from target attackers")
	}
}

func TestAddDamageKeepsSummonSeparateForKnownNonPCOwner(t *testing.T) {
	a := newSummonDamageTestApp(t, false)
	a.entities[entityID(testPlayerID)] = &EntityInfo{
		ID: entityID(testPlayerID), Name: "NPC Owner", RaceID: 42,
	}
	a.addDamage(testSummonID, testTargetID, 59_167, 1234, false)

	summonID := entityID(testSummonID)
	targetID := entityID(testTargetID)
	if a.damages[0].AttackerID != summonID || a.damages[0].AttackerName != "Puppet" {
		t.Fatalf("damage record = %#v", a.damages[0])
	}
	if a.takenStats[targetID].attackers[summonID] == nil {
		t.Fatalf("summon missing from target attackers")
	}
}

func TestAddDamageCreditsLocalOwnerWithoutOwnerEntity(t *testing.T) {
	a := newSummonDamageTestApp(t, false)
	a.selfId = entityID(testPlayerID)
	a.selfName = "Player"
	a.addDamage(testSummonID, testTargetID, 59_167, 1234, false)

	playerID := entityID(testPlayerID)
	if a.damages[0].AttackerID != playerID || a.damages[0].AttackerName != "Player" {
		t.Fatalf("damage record = %#v", a.damages[0])
	}
	if a.attackerStats[playerID] == nil || a.attackerStats[playerID].total != 1234 {
		t.Fatalf("player aggregate = %#v", a.attackerStats[playerID])
	}
	attacker := a.takenStats[entityID(testTargetID)].attackers[playerID]
	if attacker == nil || attacker.name != "Player" {
		t.Fatalf("target attacker = %#v", attacker)
	}
	if len(a.eventLogs) == 0 || a.eventLogs[len(a.eventLogs)-1].EntityName != "Player" {
		t.Fatalf("last event = %#v", a.eventLogs)
	}
	if a.chartAggData["Player"] == nil {
		t.Fatalf("chart data is not keyed by player name: %#v", a.chartAggData)
	}
	if got := a.GetDamageByAttacker(); len(got) != 1 || got[0].Name != "Player" {
		t.Fatalf("GetDamageByAttacker() = %#v", got)
	}
	if got := a.GetDamageBySkill(); len(got) != 1 || got[0].Name != "Player" {
		t.Fatalf("GetDamageBySkill() = %#v", got)
	}
}

func TestAddEntityFillsOwnerWithoutClearingIt(t *testing.T) {
	a := NewApp()
	summonID := entityID(testSummonID)
	a.entities[summonID] = &EntityInfo{ID: summonID, Name: "Puppet", RaceID: 990_104}

	a.addEntity(&packet.EntityInfo{Id: testSummonID, Name: "Puppet", RaceId: 990_104, OwnerId: testPlayerID})
	if a.entities[summonID].OwnerID != testPlayerID {
		t.Fatalf("owner after non-zero update = %d", a.entities[summonID].OwnerID)
	}
	a.addEntity(&packet.EntityInfo{Id: testSummonID, Name: "Puppet", RaceId: 990_104})
	if a.entities[summonID].OwnerID != testPlayerID {
		t.Fatalf("owner after zero update = %d", a.entities[summonID].OwnerID)
	}
}
