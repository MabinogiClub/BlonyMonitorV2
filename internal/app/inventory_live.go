package app

import (
	"strconv"
	"sync"
	"time"

	"blonymonitorv2/internal/inventory"
	"blonymonitorv2/internal/packet"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Only instances established by a complete snapshot while this app is running
// are tracked. Unknown or delayed packets cannot create a guessed owner.
type inventoryLiveState struct {
	mu      sync.Mutex
	items   map[uint64]inventory.ItemChange
	pending map[uint64]inventory.ItemChange
	timer   *time.Timer
	closed  bool
}

// R147 ItemDB cloth/silk sac IDs. Their amount represents contents, including
// zero for an empty sac, rather than a count of sac objects.
func isTextileMaterialSac(id uint32) bool {
	return id >= 2048 && id <= 2055 || id >= 2118 && id <= 2125 ||
		id >= 5110018 && id <= 5110025 || id >= 5110035 && id <= 5110042
}

func inventoryContainerForPocket(pocket uint32) string {
	switch pocket {
	case 61:
		return inventoryContainerGacha
	case inventoryCurrencyPocket:
		return inventoryContainerCurrency
	default:
		return inventoryContainerBackpack
	}
}

func (a *App) trackInventorySnapshotsLocked(snapshots []inventory.Snapshot) {
	state := &a.inventoryLive
	if state.items == nil {
		state.items = make(map[uint64]inventory.ItemChange)
	}
	for _, snapshot := range snapshots {
		if snapshot.HolderType == "account" || snapshot.SubLocation != "" {
			continue
		}
		if snapshot.Container != inventoryContainerBackpack && snapshot.Container != inventoryContainerGacha && snapshot.Container != inventoryContainerCurrency {
			continue
		}
		for id, item := range state.items {
			if item.HolderID == snapshot.HolderID && item.Container == snapshot.Container {
				delete(state.items, id)
			}
		}
		for _, item := range snapshot.Items {
			if item.InstanceID == 0 {
				continue
			}
			state.items[item.InstanceID] = inventory.ItemChange{HolderID: snapshot.HolderID, Container: snapshot.Container, Item: item, UpdatedAt: snapshot.CapturedAt}
		}
	}
}

func (a *App) handleInventoryDelta(pkt *packet.GamePacket) {
	if pkt == nil || a.inventoryStore == nil {
		return
	}
	state := &a.inventoryLive
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.closed || len(state.items) == 0 {
		return
	}
	at := inventoryCapturedAt(pkt)
	queue := func(id uint64, update func(*inventory.ItemChange)) {
		item, known := state.items[id]
		if !known || at < item.UpdatedAt || item.Removed {
			return
		}
		// Delta packet IDs identify the receiving holder. A different holder
		// requires a new snapshot; never carry an old owner across a transfer.
		if pkt.Id != 0 && strconv.FormatUint(pkt.Id, 10) != item.HolderID {
			return
		}
		previous := item
		update(&item)
		if item.Item == previous.Item && item.Removed == previous.Removed && item.Container == previous.Container {
			item.UpdatedAt = at
			state.items[id] = item
			return
		}
		item.UpdatedAt = at
		state.items[id] = item
		if state.pending == nil {
			state.pending = make(map[uint64]inventory.ItemChange)
		}
		state.pending[id] = item
	}
	switch pkt.Op {
	case opcodeItemStackUpdate:
		if len(pkt.Msg) < 3 || pkt.Msg[0].Type() != packet.MessageElemTypeLong || pkt.Msg[1].Type() != packet.MessageElemTypeShort || pkt.Msg[2].Type() != packet.MessageElemTypeInt {
			return
		}
		queue(pkt.Msg[0].Data().(uint64), func(item *inventory.ItemChange) {
			item.Item.Amount = int(pkt.Msg[1].Data().(uint16))
			item.Item.PocketType = pkt.Msg[2].Data().(uint32)
			item.Container = inventoryContainerForPocket(item.Item.PocketType)
			// Material sacs use zero to mean empty; the sac itself still exists.
		})
	case opcodeItemRemoveUpdate:
		if len(pkt.Msg) == 0 || pkt.Msg[0].Type() != packet.MessageElemTypeLong {
			return
		}
		queue(pkt.Msg[0].Data().(uint64), func(item *inventory.ItemChange) { item.Removed = true })
	case opcodeItemPocketMove, opcodeItemPocketSwap:
		for _, move := range packet.ParseItemPocketMovePacket(pkt.Msg) {
			queue(move.InstanceID, func(item *inventory.ItemChange) {
				item.Item.PocketType = move.ToPocket
				item.Container = inventoryContainerForPocket(move.ToPocket)
			})
		}
	case opcodeItemInfoUpdate, opcodeItemMoveUpdate:
		for _, record := range packet.ParseInventoryItemRecords(pkt.Msg) {
			queue(record.InstanceID, func(item *inventory.ItemChange) {
				if record.Item.ItemId != item.Item.ItemID {
					return
				}
				item.Item.Amount = int(record.Item.Amount)
				if isTextileMaterialSac(record.Item.ItemId) {
					item.Item.Amount = int(record.RawAmount)
				}
				item.Item.PocketType = record.Item.PocketType
				item.Container = inventoryContainerForPocket(record.Item.PocketType)
			})
		}
	}
	// A fixed 300 ms window coalesces stack split/merge bursts and bounds UI
	// updates without writing the complete backpack on every packet.
	if len(state.pending) > 0 && state.timer == nil {
		state.timer = time.AfterFunc(300*time.Millisecond, func() {
			state.mu.Lock()
			state.timer = nil
			changed := a.flushInventoryUpdatesLocked()
			state.mu.Unlock()
			if changed && a.ctx != nil {
				runtime.EventsEmit(a.ctx, "inventory-updated")
			}
		})
	}
}

func (a *App) flushInventoryUpdatesLocked() bool {
	state := &a.inventoryLive
	if len(state.pending) == 0 {
		return false
	}
	changes := make([]inventory.ItemChange, 0, len(state.pending))
	for _, change := range state.pending {
		changes = append(changes, change)
	}
	if err := a.inventoryStore.ApplyItemChanges(changes); err != nil {
		logger.Printf("[Inventory] update instances failed: %v", err)
		return false
	}
	for id, change := range state.pending {
		if change.Removed {
			delete(state.items, id)
		}
	}
	state.pending = nil
	return true
}

func (a *App) flushInventoryUpdates() {
	a.inventoryLive.mu.Lock()
	defer a.inventoryLive.mu.Unlock()
	a.flushInventoryUpdatesLocked()
}

func (a *App) closeInventoryUpdates() {
	a.inventoryLive.mu.Lock()
	defer a.inventoryLive.mu.Unlock()
	a.inventoryLive.closed = true
	if a.inventoryLive.timer != nil {
		a.inventoryLive.timer.Stop()
	}
	a.flushInventoryUpdatesLocked()
}
