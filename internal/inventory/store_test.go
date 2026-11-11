package inventory

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestOpenMigratesExistingInventoryDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inventory.db")
	database, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(`CREATE TABLE snapshots (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		holder_id TEXT NOT NULL,
		container TEXT NOT NULL,
		sub_location TEXT NOT NULL DEFAULT '',
		updated_at INTEGER NOT NULL,
		UNIQUE(holder_id, container, sub_location)
	)`)
	if err != nil {
		database.Close()
		t.Fatal(err)
	}
	database.Close()

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open() migration failed: %v", err)
	}
	defer store.Close()
	columns := map[string]bool{}
	rows, err := store.db.Query("PRAGMA table_info(snapshots)")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		columns[name] = true
	}
	if !columns["scanned_by_id"] || !columns["scanned_by_name"] {
		t.Fatalf("missing migrated columns: %#v", columns)
	}
}

func TestReplaceSnapshotsAndPetOwner(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	base := Snapshot{HolderID: "pet-1", HolderType: "pet", HolderName: "鼠鼠窝啊", Container: "backpack", CapturedAt: 100}
	base.Items = []Item{{InstanceID: 1, ItemID: 500, Name: "测试道具", Amount: 3}}
	if err := store.ReplaceSnapshots([]Snapshot{base}); err != nil {
		t.Fatal(err)
	}
	base.CapturedAt = 200
	base.Items = []Item{{InstanceID: 2, ItemID: 500, Name: "测试道具", Amount: 1}}
	if err := store.ReplaceSnapshots([]Snapshot{base}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdatePetOwner("pet-1", "鼠鼠窝啊", "char-2", "角色2", 210); err != nil {
		t.Fatal(err)
	}

	state, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Rows) != 1 || state.Rows[0].Amount != 1 || state.Rows[0].OwnerCharacterID != "char-2" {
		t.Fatalf("unexpected state: %#v", state)
	}
}

func TestCharacterBackpackCountsExcludesOtherHoldersAndContainers(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	snapshots := []Snapshot{
		{HolderID: "1", HolderType: "character", HolderName: "同名", Container: "backpack", Items: []Item{
			{InstanceID: 1, ItemID: 60022, Amount: 4, PocketType: 2},
			{InstanceID: 2, ItemID: 60022, Amount: 3, PocketType: 23},
			{InstanceID: 3, ItemID: 5110021, Amount: 712, PocketType: 23},
			{InstanceID: 4, ItemID: 5110038, Amount: 0, PocketType: 23},
			{InstanceID: 5, ItemID: 60022, Amount: 100, PocketType: 65},
		}},
		{HolderID: "1", HolderType: "character", HolderName: "同名", Container: "gacha", Items: []Item{{InstanceID: 6, ItemID: 60022, Amount: 100}}},
		{HolderID: "2", HolderType: "character", HolderName: "同名", Container: "backpack", Items: []Item{{InstanceID: 7, ItemID: 60022, Amount: 100}}},
		{HolderID: "3", HolderType: "pet", HolderName: "同名", Container: "backpack", Items: []Item{{InstanceID: 8, ItemID: 60022, Amount: 100}}},
		{HolderID: "account:a", HolderType: "account", Container: "bank", Items: []Item{{InstanceID: 9, ItemID: 60022, Amount: 100}}},
	}
	if err := store.ReplaceSnapshots(snapshots); err != nil {
		t.Fatal(err)
	}
	counts, err := store.CharacterBackpackCounts("1")
	if err != nil {
		t.Fatal(err)
	}
	if len(counts) != 3 || counts[60022] != 7 || counts[5110021] != 712 || counts[5110038] != 0 {
		t.Fatalf("counts = %#v", counts)
	}
	counts, err = store.CharacterBackpackCounts("3")
	if err != nil || len(counts) != 0 {
		t.Fatalf("pet counts = %#v, error = %v", counts, err)
	}
}

func TestStateMergesSameItemAcrossPockets(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	snapshot := Snapshot{
		HolderID: "character-1", HolderType: "character", HolderName: "角色1",
		Container: "backpack", CapturedAt: 100,
		Items: []Item{
			{InstanceID: 1, ItemID: 63296, Name: "世界频道号角", Amount: 10, PocketType: 123},
			{InstanceID: 2, ItemID: 63296, Name: "世界频道号角", Amount: 4, PocketType: 123},
			{InstanceID: 3, ItemID: 63296, Name: "世界频道号角", Amount: 10, PocketType: 126},
		},
	}
	if err := store.ReplaceSnapshots([]Snapshot{snapshot}); err != nil {
		t.Fatal(err)
	}

	state, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Rows) != 1 || state.Rows[0].Amount != 24 || state.Rows[0].PocketType != 0 {
		t.Fatalf("unexpected merged state: %#v", state.Rows)
	}
}

func TestBankTabsAreReplacedIndependently(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	tabA := Snapshot{HolderID: "account:test", HolderType: "account", HolderName: "账号共享", Container: "bank", SubLocation: "角色1", ScannedByID: "character-1", ScannedByName: "当前角色", CapturedAt: 100}
	tabA.Items = []Item{{InstanceID: 1, ItemID: 101, Name: "物品A", Amount: 2}}
	tabB := Snapshot{HolderID: "account:test", HolderType: "account", HolderName: "账号共享", Container: "bank", SubLocation: "角色2", CapturedAt: 100}
	tabB.Items = []Item{{InstanceID: 2, ItemID: 102, Name: "物品B", Amount: 3}}
	if err := store.ReplaceSnapshots([]Snapshot{tabA, tabB}); err != nil {
		t.Fatal(err)
	}

	tabA.CapturedAt = 200
	tabA.Items = nil
	if err := store.ReplaceSnapshots([]Snapshot{tabA}); err != nil {
		t.Fatal(err)
	}
	state, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Rows) != 1 || state.Rows[0].ItemID != 102 || state.Rows[0].SubLocation != "角色2" {
		t.Fatalf("unexpected bank state: %#v", state.Rows)
	}
}

func TestStateDoesNotMergeSameItemAcrossBankTabs(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	tabA := Snapshot{
		HolderID: "account:test", HolderType: "account", HolderName: "账号共享",
		Container: "bank", SubLocation: "角色1", CapturedAt: 100,
		Items: []Item{{InstanceID: 1, ItemID: 63296, Name: "世界频道号角", Amount: 14, PocketType: 2}},
	}
	tabB := Snapshot{
		HolderID: "account:test", HolderType: "account", HolderName: "账号共享",
		Container: "bank", SubLocation: "角色2", CapturedAt: 100,
		Items: []Item{{InstanceID: 2, ItemID: 63296, Name: "世界频道号角", Amount: 10, PocketType: 11}},
	}
	if err := store.ReplaceSnapshots([]Snapshot{tabA, tabB}); err != nil {
		t.Fatal(err)
	}

	state, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Rows) != 2 || state.Rows[0].SubLocation == state.Rows[1].SubLocation {
		t.Fatalf("bank tabs were merged: %#v", state.Rows)
	}
}

func TestDefaultPathCanBeOverridden(t *testing.T) {
	expected := filepath.Join(t.TempDir(), "private", "inventory.db")
	t.Setenv("BLONY_INVENTORY_DB", expected)
	actual, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if actual != filepath.Clean(expected) {
		t.Fatalf("DefaultPath() = %q, want %q", actual, expected)
	}
}
