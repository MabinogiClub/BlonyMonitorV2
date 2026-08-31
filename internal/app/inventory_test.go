package app

import (
	"encoding/binary"
	"path/filepath"
	"testing"

	"blonymonitorv2/internal/inventory"
	"blonymonitorv2/internal/packet"
)

func inventoryTestRecord(instanceID uint64, pocket, itemID uint32, amount uint16) packet.Message {
	publicData := make([]byte, 80)
	binary.LittleEndian.PutUint32(publicData[0:], pocket)
	binary.LittleEndian.PutUint32(publicData[4:], itemID)
	binary.LittleEndian.PutUint16(publicData[36:], amount)
	return packet.Message{
		packet.NewMessageElemLong(instanceID),
		packet.NewMessageElemByte(2),
		packet.NewMessageElemBin(publicData),
	}
}

func TestCharacterSnapshotSplitsGachaFromBackpack(t *testing.T) {
	store, err := inventory.Open(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	app := NewApp()
	app.inventoryStore = store

	message := append(inventoryTestRecord(1, 23, 101, 2), inventoryTestRecord(2, 61, 102, 4)...)
	app.handleInventoryCharacterPacket(&packet.GamePacket{Msg: message}, &packet.CharacterDataPacket{
		Id: 1, Name: "角色1", RaceId: 10001,
	})
	state := app.GetInventoryState()
	if len(state.Rows) != 2 {
		t.Fatalf("rows = %#v", state.Rows)
	}
	containers := map[uint32]string{}
	for _, row := range state.Rows {
		containers[row.ItemID] = row.Container
	}
	if containers[101] != inventoryContainerBackpack || containers[102] != inventoryContainerGacha {
		t.Fatalf("containers = %#v", containers)
	}
}

func TestBankSnapshotUsesAccountAndTab(t *testing.T) {
	store, err := inventory.Open(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	app := NewApp()
	app.inventoryStore = store

	message := packet.Message{
		packet.NewMessageElemByte(1),
		packet.NewMessageElemByte(0),
		packet.NewMessageElemLong(0),
		packet.NewMessageElemByte(0),
		packet.NewMessageElemString("account-name"),
		packet.NewMessageElemString("DunbartonBank"),
		packet.NewMessageElemString("敦巴伦银行"),
		packet.NewMessageElemLong(0),
		packet.NewMessageElemInt(1),
		packet.NewMessageElemString("角色1"),
	}
	message = append(message, inventoryTestRecord(3, 2, 103, 1)...)
	app.handleInventoryBankPacket(&packet.GamePacket{Msg: message})
	state := app.GetInventoryState()
	if len(state.Rows) != 1 || state.Rows[0].HolderType != "account" ||
		state.Rows[0].SubLocation != "角色1" || state.Rows[0].Container != inventoryContainerBank {
		t.Fatalf("rows = %#v", state.Rows)
	}
}

func TestIncompletePetSnapshotDoesNotReplaceStoredBackpack(t *testing.T) {
	store, err := inventory.Open(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	app := NewApp()
	app.inventoryStore = store

	message := inventoryTestRecord(1, 2, 201, 5)
	app.handleInventoryCharacterPacket(&packet.GamePacket{Msg: message}, &packet.CharacterDataPacket{
		Id: 10, Name: "完整宠物", RaceId: 490292,
	})
	app.handleInventoryCharacterPacket(&packet.GamePacket{}, &packet.CharacterDataPacket{
		Id: 10,
	})

	state := app.GetInventoryState()
	if len(state.Rows) != 1 || state.Rows[0].ItemID != 201 || state.Rows[0].Amount != 5 {
		t.Fatalf("partial snapshot replaced stored backpack: %#v", state.Rows)
	}
}

func TestIncompleteFriendPetAppearanceDoesNotChangeOwner(t *testing.T) {
	store, err := inventory.Open(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	app := NewApp()
	app.inventoryStore = store

	if err := store.UpdatePetOwner("10", "已有宠物", "1", "角色1", 100); err != nil {
		t.Fatal(err)
	}
	app.handleInventoryPetOwner(&packet.EntityInfo{
		Id: 10, OwnerId: 2, RaceId: 490292,
	}, 200)

	state := app.GetInventoryState()
	if len(state.Holders) != 1 || state.Holders[0].OwnerCharacterID != "1" ||
		state.Holders[0].OwnerCharacterName != "角色1" {
		t.Fatalf("friend summon changed owner: %#v", state.Holders)
	}
}
