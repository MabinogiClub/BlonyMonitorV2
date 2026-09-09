package farmstats

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSyncCurrencyBalanceFirstRecordIsBaselineOnly(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "farm_stats.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	at := time.Date(2026, 9, 4, 12, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	if err := store.SyncCurrencyBalance("character:101", "life_association_coin", 179895, at); err != nil {
		t.Fatal(err)
	}
	first, err := store.CurrencyUsage("character:101", "life_association_coin")
	if err != nil {
		t.Fatal(err)
	}
	if !first.BalanceKnown || first.Balance != 179895 || first.Added != 0 || first.Spent != 0 {
		t.Fatalf("first sync must establish only a baseline: %#v", first)
	}

	if err := store.SyncCurrencyBalance("character:101", "life_association_coin", 180295, at.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := store.SyncCurrencyBalance("character:101", "life_association_coin", 178595, at.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	usage, err := store.CurrencyUsage("character:101", "life_association_coin")
	if err != nil {
		t.Fatal(err)
	}
	if usage.Added != 400 || usage.Spent != 1700 || usage.Balance != 178595 {
		t.Fatalf("subsequent changes were not classified correctly: %#v", usage)
	}
}

func TestSyncCurrencyBalanceDropsOrphanedFirstSyncDeltas(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "farm_stats.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	at := time.Date(2026, 9, 4, 12, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	if _, err := store.db.Exec(`INSERT INTO currency_usage_daily(character_id, usage_date, currency, added, spent, balance, balance_known, updated_at)
VALUES(?, ?, ?, 178274, 178274, 0, 1, ?)`, "character:101", usageDate(at), "life_association_coin", at.UnixMilli()); err != nil {
		t.Fatal(err)
	}
	if err := store.SyncCurrencyBalance("character:101", "life_association_coin", 179895, at.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	usage, err := store.CurrencyUsage("character:101", "life_association_coin")
	if err != nil {
		t.Fatal(err)
	}
	if usage.Added != 0 || usage.Spent != 0 || usage.Balance != 179895 {
		t.Fatalf("orphaned first-sync deltas were retained: %#v", usage)
	}
}

func TestSyncCurrencyBalanceRepairsFullWalletBounce(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "farm_stats.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	at := time.Date(2026, 9, 9, 16, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	if err := store.SyncCurrencyBalance("character:101", "life_association_coin", 126396, at); err != nil {
		t.Fatal(err)
	}
	if err := store.SyncCurrencyBalance("character:101", "life_association_coin", 0, at.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := store.SyncCurrencyBalance("character:101", "life_association_coin", 125796, at.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	usage, err := store.CurrencyUsage("character:101", "life_association_coin")
	if err != nil {
		t.Fatal(err)
	}
	if usage.Added != 0 || usage.Spent != 600 || usage.Balance != 125796 {
		t.Fatalf("full-wallet bounce was not collapsed: %#v", usage)
	}
	history, err := store.CurrencyUsageHistory("character:101", "life_association_coin", at, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].Added != 0 || history[0].Spent != 600 || history[0].Balance != 125796 {
		t.Fatalf("daily bounce was not collapsed: %#v", history)
	}
}
