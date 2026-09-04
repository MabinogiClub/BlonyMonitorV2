package app

import (
	"path/filepath"
	"testing"
	"time"

	"blonymonitorv2/internal/farmstats"
)

func TestFarmUsageSeparatesCharactersAndCountsEnergyDecreases(t *testing.T) {
	store, err := farmstats.Open(filepath.Join(t.TempDir(), "farm_stats.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	a := NewApp()
	a.farmStatsStore = store
	a.selfId = "101"
	a.selfName = "角色一"
	a.rememberFarmAccount("first", "101")
	a.recordFarmPlant(101, "黑莓", 1200, time.Now())
	a.recordFarmEnergy(101, 80, time.Now())
	a.recordFarmEnergy(101, 74, time.Now())
	a.recordFarmEnergy(101, 90, time.Now())

	a.selfId = "202"
	a.selfName = "角色二"
	a.rememberFarmAccount("first", "202")
	a.recordFarmPlant(202, "红梨树", 5000, time.Now())
	a.recordFarmEnergy(202, 60, time.Now())
	a.recordFarmEnergy(202, 58, time.Now())

	first, err := store.Usage("character:101")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Usage("character:202")
	if err != nil {
		t.Fatal(err)
	}
	if first.CoinsSpent != 1200 || first.EnergySpent != 6 {
		t.Fatalf("first account usage = %#v", first)
	}
	if second.CoinsSpent != 5000 || second.EnergySpent != 2 {
		t.Fatalf("second account usage = %#v", second)
	}
}

func TestFarmUsageDoesNotMergeCharactersUnderSameAccount(t *testing.T) {
	store, err := farmstats.Open(filepath.Join(t.TempDir(), "farm_stats.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	a := NewApp()
	a.farmStatsStore = store
	a.accountID = "account:main"
	a.selfId = "101"
	a.rememberFarmAccount("main", "101")
	a.recordFarmPlant(101, "蜘蛛古木", 5000, time.Now())
	a.recordFarmPlant(202, "茉莉", 1200, time.Now())
	characterOne, err := store.Usage("character:101")
	if err != nil {
		t.Fatal(err)
	}
	characterTwo, err := store.Usage("character:202")
	if err != nil {
		t.Fatal(err)
	}
	if characterOne.CropCounts["蜘蛛古木"] != 1 || characterOne.CoinsSpent != 5000 {
		t.Fatalf("character one usage = %#v", characterOne)
	}
	if characterTwo.CropCounts["茉莉"] != 1 || characterTwo.CoinsSpent != 1200 {
		t.Fatalf("character two usage = %#v", characterTwo)
	}
	accountUsage, err := store.Usage("account:main")
	if err != nil {
		t.Fatal(err)
	}
	if accountUsage.CoinsSpent != 0 {
		t.Fatalf("character usage unexpectedly merged into account: %#v", accountUsage)
	}
}
