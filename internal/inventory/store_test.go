package inventory

import (
	"path/filepath"
	"testing"
)

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

func TestBankTabsAreReplacedIndependently(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	tabA := Snapshot{HolderID: "account:test", HolderType: "account", HolderName: "账号共享", Container: "bank", SubLocation: "角色1", CapturedAt: 100}
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
