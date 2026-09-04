package farmstats

import (
	"path/filepath"
	"testing"
	"time"
)

func TestUsageAggregatesByDayAndAccount(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "farm_stats.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	dayOne := time.Date(2026, 9, 3, 23, 59, 0, 0, time.FixedZone("CST", 8*60*60))
	dayTwo := dayOne.Add(2 * time.Minute)
	for _, at := range []time.Time{dayOne, dayOne, dayTwo} {
		if err := store.RecordPlant("account:a", "黑莓", 1200, at); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.RecordPlant("account:a", "红梨树", 5000, dayTwo); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordEnergy("account:a", 7, dayOne); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordEnergy("account:a", 3, dayTwo); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordPlant("account:b", "黑莓", 1200, dayOne); err != nil {
		t.Fatal(err)
	}
	u, err := store.Usage("account:a")
	if err != nil {
		t.Fatal(err)
	}
	if u.CoinsSpent != 8600 || u.EnergySpent != 10 || u.CropCounts["黑莓"] != 3 || u.CropCounts["红梨树"] != 1 {
		t.Fatalf("unexpected usage: %#v", u)
	}
	other, err := store.Usage("account:b")
	if err != nil {
		t.Fatal(err)
	}
	if other.CoinsSpent != 1200 || other.CropCounts["黑莓"] != 1 {
		t.Fatalf("accounts not isolated: %#v", other)
	}
}

func TestMergeCombinesProvisionalRows(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "farm_stats.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now()
	if err := store.RecordPlant("character:123", "秋葵", 1200, now); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordPlant("account:test", "秋葵", 1200, now); err != nil {
		t.Fatal(err)
	}
	if err := store.Merge("character:123", "account:test"); err != nil {
		t.Fatal(err)
	}
	u, err := store.Usage("account:test")
	if err != nil {
		t.Fatal(err)
	}
	if u.CoinsSpent != 2400 || u.CropCounts["秋葵"] != 2 {
		t.Fatalf("merged usage: %#v", u)
	}
	old, err := store.Usage("character:123")
	if err != nil {
		t.Fatal(err)
	}
	if old.CoinsSpent != 0 {
		t.Fatalf("provisional rows remain: %#v", old)
	}
}

func TestUsageDateRollsOverAtSevenCST(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	cases := []struct {
		name string
		at   time.Time
		want string
	}{
		{"before reset", time.Date(2026, 9, 4, 6, 59, 59, 0, location), "2026-09-03"},
		{"at reset", time.Date(2026, 9, 4, 7, 0, 0, 0, location), "2026-09-04"},
		{"after reset", time.Date(2026, 9, 4, 12, 0, 0, 0, location), "2026-09-04"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := usageDate(tc.at); got != tc.want {
				t.Fatalf("usageDate(%s) = %q, want %q", tc.at, got, tc.want)
			}
		})
	}
}

func TestUsageHistoryIncludesEmptyDaysAndUsesFarmDay(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "farm_stats.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	location := time.FixedZone("CST", 8*60*60)
	// 06:30 on Sep 4 belongs to Sep 3; 07:00 starts Sep 4.
	if err := store.RecordPlant("account:a", "黑莓", 1200, time.Date(2026, 9, 4, 6, 30, 0, 0, location)); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordPlant("account:a", "秋葵", 1200, time.Date(2026, 9, 4, 7, 0, 0, 0, location)); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordEnergy("account:a", 5, time.Date(2026, 9, 3, 8, 0, 0, 0, location)); err != nil {
		t.Fatal(err)
	}
	history, err := store.UsageHistory("account:a", time.Date(2026, 9, 4, 8, 0, 0, 0, location), 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 7 || history[0].Date != "2026-08-29" || history[6].Date != "2026-09-04" {
		t.Fatalf("unexpected history dates: %#v", history)
	}
	// Verify the final two farm days (Sep 3 and Sep 4) and an empty day.
	byDate := map[string]DailyUsage{}
	for _, day := range history {
		byDate[day.Date] = day
	}
	if byDate["2026-09-03"].CropCounts["黑莓"] != 1 || byDate["2026-09-03"].EnergySpent != 5 {
		t.Fatalf("Sep 3 aggregation incorrect: %#v", byDate["2026-09-03"])
	}
	if byDate["2026-09-04"].CropCounts["秋葵"] != 1 || byDate["2026-09-04"].CoinsSpent != 1200 {
		t.Fatalf("Sep 4 aggregation incorrect: %#v", byDate["2026-09-04"])
	}
	if len(byDate["2026-08-30"].CropCounts) != 0 {
		t.Fatalf("empty day should have empty crop counts: %#v", byDate["2026-08-30"])
	}
}
