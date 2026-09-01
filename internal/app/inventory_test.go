package app

import (
	"encoding/binary"
	"path/filepath"
	"testing"

	"blonymonitorv2/db"
	"blonymonitorv2/internal/inventory"
	"blonymonitorv2/internal/packet"

	"github.com/jmoiron/sqlx"
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

func TestCharacterSnapshotSplitsSpecialContainersFromBackpack(t *testing.T) {
	store, err := inventory.Open(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	app := NewApp()
	app.inventoryStore = store

	message := append(inventoryTestRecord(1, 23, 101, 2), inventoryTestRecord(2, 61, 102, 4)...)
	message = append(message, inventoryTestRecord(3, inventoryCurrencyPocket, 103, 6)...)
	app.handleInventoryCharacterPacket(&packet.GamePacket{Msg: message}, &packet.CharacterDataPacket{
		Id: 1, Name: "角色1", RaceId: 10001,
	})
	state := app.GetInventoryState()
	if len(state.Rows) != 3 {
		t.Fatalf("rows = %#v", state.Rows)
	}
	containers := map[uint32]string{}
	for _, row := range state.Rows {
		containers[row.ItemID] = row.Container
	}
	if containers[101] != inventoryContainerBackpack || containers[102] != inventoryContainerGacha ||
		containers[103] != inventoryContainerCurrency {
		t.Fatalf("containers = %#v", containers)
	}
}

func TestCurrencyPocketInExistingBackpackSnapshotIsReclassified(t *testing.T) {
	store, err := inventory.Open(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.ReplaceSnapshots([]inventory.Snapshot{{
		HolderID: "1", HolderType: "character", HolderName: "角色1", Container: inventoryContainerBackpack,
		Items: []inventory.Item{{ItemID: 75891, Name: "贝特林高级印章", Amount: 1, PocketType: inventoryCurrencyPocket}},
	}}); err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	app.inventoryStore = store

	state := app.GetInventoryState()
	if len(state.Rows) != 1 || state.Rows[0].Container != inventoryContainerCurrency {
		t.Fatalf("rows = %#v", state.Rows)
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
	app.selfId = "character-1"
	app.selfName = "角色1"

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
		state.Rows[0].ScannedByID != "character-1" || state.Rows[0].ScannedByName != "角色1" ||
		state.Rows[0].SubLocation != "角色1" || state.Rows[0].Container != inventoryContainerBank {
		t.Fatalf("rows = %#v", state.Rows)
	}
}

func TestDressingRoomSnapshotUsesCurrentCharacter(t *testing.T) {
	store, err := inventory.Open(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	app := NewApp()
	app.inventoryStore = store
	app.selfId = "character-2"
	app.selfName = "角色2"

	message := packet.Message{
		packet.NewMessageElemByte(1),
		packet.NewMessageElemString("account-name"),
	}
	message = append(message, inventoryTestRecord(4, 2, 104, 1)...)
	app.handleInventoryDressingRoomPacket(&packet.GamePacket{Msg: message})
	state := app.GetInventoryState()
	if len(state.Rows) != 1 || state.Rows[0].ScannedByID != "character-2" ||
		state.Rows[0].ScannedByName != "角色2" ||
		state.Rows[0].Container != inventoryContainerDressing {
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

func TestInventoryItemNameKeepsCachedFallbackForUnknownItem(t *testing.T) {
	if got := inventoryItemName(^uint32(0), "cached name"); got != "cached name" {
		t.Fatalf("inventoryItemName() = %q, want cached name", got)
	}
}

func TestInventoryDisplayNameIncludesEnchantPrefixesAndScrollName(t *testing.T) {
	testDB, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := testDB.Exec(`CREATE TABLE OptionSet (ID INTEGER, LocalName TEXT);
CREATE TABLE ItemDB (ID INTEGER, Text_Name0 TEXT, Text_Name1 TEXT, Text_Desc1 TEXT, File_InvImage TEXT);
INSERT INTO OptionSet VALUES (21132, '老练的'), (31690, '兔猿人'), (21638, '安乐');
INSERT INTO ItemDB VALUES
  (2110113, 'Fresh Baristar Hat', '清新的咖啡师帽子', '', ''),
  (2210006, 'Bliain Ender''s Intense Circlet for Women', '布里安恩德斯激情锁环(女款)', '', '');`); err != nil {
		t.Fatal(err)
	}
	previousDB := db.DB
	db.DB = testDB
	t.Cleanup(func() {
		db.DB = previousDB
		_ = testDB.Close()
	})

	extended := make([]byte, 144)
	binary.LittleEndian.PutUint16(extended[60:], 21132)
	binary.LittleEndian.PutUint16(extended[62:], 31690)
	if got := inventoryDisplayName(2210006, "布里安恩德斯激情锁环(女款)", extended, ""); got != "老练的 兔猿人 布里安恩德斯激情锁环(女款)" {
		t.Fatalf("equipment name = %q", got)
	}
	if got := inventoryDisplayName(62059, "专用魔法释放卷轴", nil, "ENPFIX:4:21638;"); got != "专用魔法释放卷轴 - 安乐" {
		t.Fatalf("scroll name = %q", got)
	}
	if got := inventoryStateItemName(2110113, "Fresh Baristar Hat"); got != "清新的咖啡师帽子" {
		t.Fatalf("old English name = %q", got)
	}
	if got := inventoryStateItemName(2110113, "安乐 清新的咖啡师帽子"); got != "安乐 清新的咖啡师帽子" {
		t.Fatalf("instance name = %q", got)
	}
	if got := inventoryEnchantName(2210006, "老练的 兔猿人 布里安恩德斯激情锁环(女款)"); got != "老练的 兔猿人" {
		t.Fatalf("enchant name = %q", got)
	}
	if got := inventoryEnchantName(62059, "专用魔法释放卷轴 - 安乐"); got != "" {
		t.Fatalf("scroll enchant name = %q", got)
	}
}
