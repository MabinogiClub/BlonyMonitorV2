package app

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"blonymonitorv2/db"
	"blonymonitorv2/internal/inventory"
	"blonymonitorv2/internal/packet"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	inventoryContainerBackpack = "backpack"
	inventoryContainerGacha    = "gacha"
	inventoryContainerBank     = "bank"
	inventoryContainerDressing = "dressing"
	inventoryContainerCash     = "cash"
)

func inventoryCapturedAt(pkt *packet.GamePacket) int64 {
	if pkt != nil && !pkt.At.IsZero() {
		return pkt.At.UnixMilli()
	}
	return time.Now().UnixMilli()
}

func inventoryItems(records []packet.InventoryItemRecord) []inventory.Item {
	items := make([]inventory.Item, 0, len(records))
	names := make(map[uint32]string)
	for _, record := range records {
		if record.Item == nil || record.Item.ItemId == 0 {
			continue
		}
		name, known := names[record.Item.ItemId]
		if !known {
			if info := db.GetItemInfo(int(record.Item.ItemId)); info != nil {
				name = info.DisplayName()
			}
			if name == "" {
				name = "物品 #" + strconv.FormatUint(uint64(record.Item.ItemId), 10)
			}
			names[record.Item.ItemId] = name
		}
		amount := int(record.Item.Amount)
		if amount <= 0 {
			amount = 1
		}
		items = append(items, inventory.Item{
			InstanceID: record.InstanceID,
			ItemID:     record.Item.ItemId,
			Name:       name,
			Amount:     amount,
			PocketType: record.Item.PocketType,
		})
	}
	return items
}

func (a *App) replaceInventorySnapshots(snapshots ...inventory.Snapshot) {
	if a.inventoryStore == nil || len(snapshots) == 0 {
		return
	}
	if err := a.inventoryStore.ReplaceSnapshots(snapshots); err != nil {
		logger.Printf("[Inventory] replace snapshot failed: %v", err)
		return
	}
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "inventory-updated")
	}
}

func (a *App) handleInventoryCharacterPacket(pkt *packet.GamePacket, character *packet.CharacterDataPacket) {
	// A partial friend-pet announcement must never replace a stored snapshot.
	// A real 0x5209 login snapshot always includes ID, name and race.
	if pkt == nil || character == nil || character.Id == 0 ||
		strings.TrimSpace(character.Name) == "" || character.RaceId == 0 {
		return
	}
	holderType := "pet"
	if isPC(int(character.RaceId)) {
		holderType = "character"
	}
	backpack := make([]packet.InventoryItemRecord, 0)
	gacha := make([]packet.InventoryItemRecord, 0)
	for _, record := range packet.ParseInventoryItemRecords(pkt.Msg) {
		if record.Item != nil && record.Item.PocketType == 61 {
			gacha = append(gacha, record)
		} else {
			backpack = append(backpack, record)
		}
	}
	base := inventory.Snapshot{
		HolderID:   strconv.FormatUint(character.Id, 10),
		HolderType: holderType,
		HolderName: character.Name,
		CapturedAt: inventoryCapturedAt(pkt),
	}
	backpackSnapshot := base
	backpackSnapshot.Container = inventoryContainerBackpack
	backpackSnapshot.Items = inventoryItems(backpack)
	gachaSnapshot := base
	gachaSnapshot.Container = inventoryContainerGacha
	gachaSnapshot.Items = inventoryItems(gacha)
	a.replaceInventorySnapshots(backpackSnapshot, gachaSnapshot)
}

func stringMessageValue(msg packet.Message, index int) string {
	if index < 0 || index >= len(msg) || msg[index].Type() != packet.MessageElemTypeString {
		return ""
	}
	return strings.TrimSpace(msg[index].Data().(string))
}

func (a *App) handleInventoryBankPacket(pkt *packet.GamePacket) {
	if pkt == nil {
		return
	}
	account := stringMessageValue(pkt.Msg, 4)
	tab := stringMessageValue(pkt.Msg, 9)
	if account == "" || tab == "" {
		return
	}
	a.replaceInventorySnapshots(inventory.Snapshot{
		HolderID:    "account:" + account,
		HolderType:  "account",
		HolderName:  "账号共享",
		Container:   inventoryContainerBank,
		SubLocation: tab,
		CapturedAt:  inventoryCapturedAt(pkt),
		Items:       inventoryItems(packet.ParseInventoryItemRecords(pkt.Msg)),
	})
}

func (a *App) handleInventoryDressingRoomPacket(pkt *packet.GamePacket) {
	if pkt == nil {
		return
	}
	account := stringMessageValue(pkt.Msg, 1)
	if account == "" {
		return
	}
	a.replaceInventorySnapshots(inventory.Snapshot{
		HolderID:   "account:" + account,
		HolderType: "account",
		HolderName: "账号共享",
		Container:  inventoryContainerDressing,
		CapturedAt: inventoryCapturedAt(pkt),
		Items:      inventoryItems(packet.ParseInventoryItemRecords(pkt.Msg)),
	})
}

func (a *App) holderIdentity(id string) (string, string) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if entity := a.entities[id]; entity != nil {
		holderType := "pet"
		if entity.IsPC {
			holderType = "character"
		}
		return holderType, entity.Name
	}
	if id == a.selfId {
		return "character", a.selfName
	}
	return "character", ""
}

func (a *App) handleInventoryCashPacket(pkt *packet.GamePacket) {
	if pkt == nil || pkt.Id == 0 {
		return
	}
	holderID := strconv.FormatUint(pkt.Id, 10)
	holderType, holderName := a.holderIdentity(holderID)
	a.replaceInventorySnapshots(inventory.Snapshot{
		HolderID:   holderID,
		HolderType: holderType,
		HolderName: holderName,
		Container:  inventoryContainerCash,
		CapturedAt: inventoryCapturedAt(pkt),
		Items:      inventoryItems(packet.ParseInventoryItemRecords(pkt.Msg)),
	})
}

func (a *App) handleInventoryPetOwner(entity *packet.EntityInfo, updatedAt int64) {
	// Friend summons (0x906f) contain only an ID. Ownership is updated only
	// from a complete primary-pet 0x520c entity.
	if a.inventoryStore == nil || entity == nil || entity.Id == 0 || entity.OwnerId == 0 ||
		strings.TrimSpace(entity.Name) == "" || entity.RaceId == 0 || isPC(int(entity.RaceId)) {
		return
	}
	ownerID := strconv.FormatUint(entity.OwnerId, 10)
	_, ownerName := a.holderIdentity(ownerID)
	if err := a.inventoryStore.UpdatePetOwner(strconv.FormatUint(entity.Id, 10), entity.Name, ownerID, ownerName, updatedAt); err != nil {
		logger.Printf("[Inventory] update pet owner failed: %v", err)
		return
	}
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "inventory-updated")
	}
}

func (a *App) GetInventoryState() inventory.State {
	if a.inventoryStore == nil {
		return inventory.State{Rows: []inventory.Row{}, Holders: []inventory.Holder{}}
	}
	state, err := a.inventoryStore.State()
	if err != nil {
		logger.Printf("[Inventory] read state failed: %v", err)
		return inventory.State{Rows: []inventory.Row{}, Holders: []inventory.Holder{}}
	}
	return state
}

func (a *App) GetInventoryDatabasePath() string {
	state := a.GetInventoryState()
	return fmt.Sprint(state.DatabasePath)
}
