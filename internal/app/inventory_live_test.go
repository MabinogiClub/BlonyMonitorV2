package app

import (
	"path/filepath"
	"testing"
	"time"

	"blonymonitorv2/internal/inventory"
	"blonymonitorv2/internal/packet"
)

func newInventoryLiveTestApp(t *testing.T) *App {
	t.Helper()
	store, err := inventory.Open(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	a := NewApp()
	a.inventoryStore = store
	t.Cleanup(func() { a.closeInventoryUpdates(); store.Close() })
	return a
}

func TestInventoryLiveUpdatesKnownInstancesAndPreservesOtherCharacters(t *testing.T) {
	a := newInventoryLiveTestApp(t)
	at := time.UnixMilli(1000)
	for _, id := range []uint64{101, 202} {
		a.handleInventoryCharacterPacket(&packet.GamePacket{At: at, Msg: inventoryTestRecord(id, 2, 60022, 10)},
			&packet.CharacterDataPacket{Id: id, Name: "同名角色", RaceId: 10001})
	}
	// The instance's captured owner is used even while the displayed self ID
	// is stale. Unknown and older updates cannot change another character.
	a.selfId = "101"
	a.handleInventoryDelta(&packet.GamePacket{At: at.Add(time.Second), Op: opcodeItemStackUpdate,
		Msg: packet.Message{packet.NewMessageElemLong(202), packet.NewMessageElemShort(7), packet.NewMessageElemInt(2)}})
	a.handleInventoryDelta(&packet.GamePacket{At: at, Op: opcodeItemStackUpdate,
		Msg: packet.Message{packet.NewMessageElemLong(202), packet.NewMessageElemShort(99), packet.NewMessageElemInt(2)}})
	a.handleInventoryDelta(&packet.GamePacket{At: at.Add(time.Second), Op: opcodeItemStackUpdate,
		Msg: packet.Message{packet.NewMessageElemLong(999), packet.NewMessageElemShort(99), packet.NewMessageElemInt(2)}})
	a.flushInventoryUpdates()
	state := a.GetInventoryState()
	amounts := make(map[string]int)
	for _, row := range state.Rows {
		amounts[row.HolderID] = row.Amount
	}
	if amounts["101"] != 10 || amounts["202"] != 7 || len(state.Rows) != 2 {
		t.Fatalf("rows = %#v", state.Rows)
	}

	a.handleInventoryDelta(&packet.GamePacket{At: at.Add(2 * time.Second), Op: opcodeItemPocketMove,
		Msg: packet.Message{packet.NewMessageElemLong(202), packet.NewMessageElemInt(2), packet.NewMessageElemInt(61)}})
	a.flushInventoryUpdates()
	for _, row := range a.GetInventoryState().Rows {
		if row.HolderID == "202" && (row.Container != inventoryContainerGacha || row.Amount != 7) {
			t.Fatalf("moved row = %#v", row)
		}
	}
	a.handleInventoryDelta(&packet.GamePacket{At: at.Add(3 * time.Second), Op: opcodeItemRemoveUpdate, Msg: packet.Message{packet.NewMessageElemLong(202)}})
	a.flushInventoryUpdates()
	if rows := a.GetInventoryState().Rows; len(rows) != 1 || rows[0].HolderID != "101" {
		t.Fatalf("removed rows = %#v", rows)
	}
}

func TestInventoryLiveTracksInfoUpdatesAndEmptyMaterialSacs(t *testing.T) {
	a := newInventoryLiveTestApp(t)
	at := time.UnixMilli(1000)
	a.handleInventoryCharacterPacket(&packet.GamePacket{At: at, Msg: inventoryTestRecord(1, 2, 5110021, 12)},
		&packet.CharacterDataPacket{Id: 101, Name: "角色", RaceId: 10001})
	a.handleInventoryDelta(&packet.GamePacket{At: at.Add(time.Second), Op: opcodeItemInfoUpdate, Msg: inventoryTestRecord(1, 2, 5110021, 3)})
	a.flushInventoryUpdates()
	if rows := a.GetInventoryState().Rows; len(rows) != 1 || rows[0].Amount != 3 {
		t.Fatalf("info rows = %#v", rows)
	}
	a.handleInventoryDelta(&packet.GamePacket{At: at.Add(2 * time.Second), Op: opcodeItemStackUpdate,
		Msg: packet.Message{packet.NewMessageElemLong(1), packet.NewMessageElemShort(0), packet.NewMessageElemInt(2)}})
	a.flushInventoryUpdates()
	if rows := a.GetInventoryState().Rows; len(rows) != 1 || rows[0].Amount != 0 {
		t.Fatalf("empty sac = %#v", rows)
	}
	// A subsequent channel/login snapshot is authoritative, including an
	// empty sac. Pending changes are settled before it is replaced.
	a.handleInventoryDelta(&packet.GamePacket{At: at.Add(3 * time.Second), Op: opcodeItemStackUpdate,
		Msg: packet.Message{packet.NewMessageElemLong(1), packet.NewMessageElemShort(4), packet.NewMessageElemInt(2)}})
	a.handleInventoryCharacterPacket(&packet.GamePacket{At: at.Add(4 * time.Second), Msg: inventoryTestRecord(1, 2, 5110021, 0)},
		&packet.CharacterDataPacket{Id: 101, Name: "角色", RaceId: 10001})
	if rows := a.GetInventoryState().Rows; len(rows) != 1 || rows[0].Amount != 0 {
		t.Fatalf("resynced sac = %#v", rows)
	}
}

func TestInventoryLiveAcceptsTransformedItemRecord(t *testing.T) {
	a := newInventoryLiveTestApp(t)
	at := time.UnixMilli(1000)
	a.handleInventoryCharacterPacket(&packet.GamePacket{At: at, Msg: inventoryTestRecord(1, 104, 4001473, 1)},
		&packet.CharacterDataPacket{Id: 101, Name: "角色", RaceId: 10001})
	a.handleInventoryDelta(&packet.GamePacket{Id: 101, At: at.Add(time.Second), Op: opcodeItemMoveUpdate,
		Msg: inventoryTestRecord(1, 104, 5000138, 1)})
	a.flushInventoryUpdates()
	if rows := a.GetInventoryState().Rows; len(rows) != 1 || rows[0].ItemID != 5000138 {
		t.Fatalf("transformed item rows = %#v", rows)
	}
}

func TestInventoryLiveBatchesBurstAndSkipsUnchangedItems(t *testing.T) {
	a := newInventoryLiveTestApp(t)
	at := time.UnixMilli(1000)
	a.handleInventoryCharacterPacket(&packet.GamePacket{At: at, Msg: inventoryTestRecord(1, 2, 60022, 10)},
		&packet.CharacterDataPacket{Id: 101, Name: "角色", RaceId: 10001})
	for _, amount := range []uint16{10, 9, 8, 7} {
		a.handleInventoryDelta(&packet.GamePacket{At: at.Add(time.Second), Op: opcodeItemStackUpdate,
			Msg: packet.Message{packet.NewMessageElemLong(1), packet.NewMessageElemShort(amount), packet.NewMessageElemInt(2)}})
	}
	a.inventoryLive.mu.Lock()
	pendingCount := len(a.inventoryLive.pending)
	a.inventoryLive.mu.Unlock()
	if pendingCount != 1 {
		t.Fatalf("pending count = %d", pendingCount)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		rows := a.GetInventoryState().Rows
		if len(rows) == 1 && rows[0].Amount == 7 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("batch did not settle: %#v", rows)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestInventoryLiveRejectsDifferentRecipientsAndDelayedUpdates(t *testing.T) {
	a := newInventoryLiveTestApp(t)
	at := time.UnixMilli(1000)
	a.handleInventoryCharacterPacket(&packet.GamePacket{At: at, Msg: inventoryTestRecord(1, 2, 60022, 10)},
		&packet.CharacterDataPacket{Id: 101, Name: "角色", RaceId: 10001})
	stack := func(holder uint64, delta time.Duration, amount uint16) {
		a.handleInventoryDelta(&packet.GamePacket{Id: holder, At: at.Add(delta), Op: opcodeItemStackUpdate,
			Msg: packet.Message{packet.NewMessageElemLong(1), packet.NewMessageElemShort(amount), packet.NewMessageElemInt(2)}})
	}
	// An unchanged newer update still advances the watermark. A later-arriving
	// old packet and a packet addressed to a pet cannot overwrite this owner.
	stack(101, 2*time.Second, 10)
	stack(101, time.Second, 9)
	stack(202, 3*time.Second, 8)
	a.flushInventoryUpdates()
	if counts := a.GetCharacterBackpackCounts("101"); counts[60022] != 10 {
		t.Fatalf("counts = %#v", counts)
	}
	a.handleInventoryDelta(&packet.GamePacket{Id: 101, At: at.Add(4 * time.Second), Op: opcodeItemRemoveUpdate,
		Msg: packet.Message{packet.NewMessageElemLong(1)}})
	stack(101, 5*time.Second, 7)
	a.flushInventoryUpdates()
	if counts := a.GetCharacterBackpackCounts("101"); len(counts) != 0 {
		t.Fatalf("removed item was restored: %#v", counts)
	}
}

func TestInventoryLiveTracksNewInstancesTransfersAndSpecialStorage(t *testing.T) {
	a := newInventoryLiveTestApp(t)
	at := time.UnixMilli(1000)
	a.handleInventoryCharacterPacket(&packet.GamePacket{At: at, Msg: inventoryTestRecord(1, 104, 60022, 2)},
		&packet.CharacterDataPacket{Id: 101, Name: "角色", RaceId: 10001})
	a.handleInventoryCharacterPacket(&packet.GamePacket{At: at, Msg: inventoryTestRecord(2, 104, 60022, 3)},
		&packet.CharacterDataPacket{Id: 202, Name: "宠物", RaceId: 10002})

	// Pocket 88 is the Moonlight Island store, not the trade backpack. Pocket
	// 307 is the farm store. Both stay out of barter material totals.
	for pocket, want := range map[uint32]string{88: inventoryContainerMoonlight, farmStoragePocketType: inventoryContainerFarm} {
		a.handleInventoryDelta(&packet.GamePacket{Id: 101, At: at.Add(time.Second), Op: opcodeItemStackUpdate,
			Msg: packet.Message{packet.NewMessageElemLong(1), packet.NewMessageElemShort(2), packet.NewMessageElemInt(pocket)}})
		a.flushInventoryUpdates()
		found := false
		for _, row := range a.GetInventoryState().Rows {
			if row.HolderID == "101" && row.ItemID == 60022 && row.Container == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("pocket %d did not map to %s: %#v", pocket, want, a.GetInventoryState().Rows)
		}
		if counts := a.GetCharacterBackpackCounts("101"); len(counts) != 0 {
			t.Fatalf("pocket %d leaked into backpack counts: %#v", pocket, counts)
		}
	}

	// A split creates an instance absent from the login snapshot; its full
	// item record is enough to add it under the established packet recipient.
	a.handleInventoryDelta(&packet.GamePacket{Id: 101, At: at.Add(2 * time.Second), Op: opcodeItemMoveUpdate,
		Msg: inventoryTestRecord(3, 104, 60022, 4)})
	a.flushInventoryUpdates()
	if counts := a.GetCharacterBackpackCounts("101"); counts[60022] != 4 {
		t.Fatalf("split instance counts = %#v", counts)
	}

	// Pet removal followed by the character's public item record transfers the
	// same instance without leaving a stale pet copy.
	a.handleInventoryDelta(&packet.GamePacket{Id: 202, At: at.Add(3 * time.Second), Op: opcodeItemRemoveUpdate,
		Msg: packet.Message{packet.NewMessageElemLong(2)}})
	a.handleInventoryDelta(&packet.GamePacket{Id: 101, At: at.Add(4 * time.Second), Op: opcodeItemMoveUpdate,
		Msg: inventoryTestRecord(2, 104, 60022, 3)})
	a.flushInventoryUpdates()
	petCount, characterCount := 0, 0
	for _, row := range a.GetInventoryState().Rows {
		if row.ItemID != 60022 {
			continue
		}
		if row.HolderID == "202" && row.Container == inventoryContainerBackpack {
			petCount += row.Amount
		} else if row.HolderID == "101" && row.Container == inventoryContainerBackpack {
			characterCount += row.Amount
		}
	}
	if petCount != 0 || characterCount != 7 {
		t.Fatalf("pet=%d character=%d rows=%#v", petCount, characterCount, a.GetInventoryState().Rows)
	}
}
