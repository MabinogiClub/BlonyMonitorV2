package app

import (
	"path/filepath"
	"testing"
	"time"

	"blonymonitorv2/internal/farmstats"
)

func TestFarmUsageSeparatesAccountsAndCountsEnergyDecreases(t *testing.T) {
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

	a.accountID = ""
	a.selfId = "202"
	a.selfName = "角色二"
	a.rememberFarmAccount("second", "202")
	a.recordFarmPlant(202, "红梨树", 5000, time.Now())
	a.recordFarmEnergy(202, 60, time.Now())
	a.recordFarmEnergy(202, 58, time.Now())

	first, err := store.Usage("account:first")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Usage("account:second")
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
