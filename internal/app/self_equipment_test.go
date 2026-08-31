package app

import (
	"encoding/binary"
	"testing"
	"time"

	"blonymonitorv2/internal/packet"
)

func TestSelfEquipmentDurabilityFlow(t *testing.T) {
	a := NewApp()

	public := make([]byte, 80)
	binary.LittleEndian.PutUint32(public[0:], 10)
	binary.LittleEndian.PutUint32(public[4:], 41160)
	extended := make([]byte, 144)
	binary.LittleEndian.PutUint32(extended[16:], 7120)
	binary.LittleEndian.PutUint32(extended[20:], 22000)

	inventory := &packet.GamePacket{
		At: time.UnixMilli(1000),
		Msg: packet.Message{
			packet.NewMessageElemByte(1),
			packet.NewMessageElemLong(4503599638666428),
			packet.NewMessageElemByte(2),
			packet.NewMessageElemString("Flandre"),
			packet.NewMessageElemString(""),
			packet.NewMessageElemString(""),
			packet.NewMessageElemInt(10001),
			packet.NewMessageElemLong(22518919189329483),
			packet.NewMessageElemByte(2),
			packet.NewMessageElemBin(public),
			packet.NewMessageElemBin(extended),
			packet.NewMessageElemString("CTSPEED:4:750;OWNER:s:Flandre;"),
		},
	}
	a.handleSelfInventoryPacket(inventory)

	state := a.GetSelfEquipmentState()
	if !state.Ready || state.Character != "Flandre" {
		t.Fatalf("inventory not ready: %#v", state)
	}
	var axe *SelfEquipmentSlot
	for i := range state.Slots {
		if state.Slots[i].PocketType == 10 {
			axe = &state.Slots[i]
			break
		}
	}
	if axe == nil || axe.ItemID != 41160 || axe.DurabilityValue == nil || *axe.DurabilityValue != 7.12 {
		t.Fatalf("axe slot mismatch: %#v", axe)
	}
	if axe.DurabilityMax == nil || *axe.DurabilityMax != 22 {
		t.Fatalf("axe max mismatch: %#v", axe)
	}

	a.handleItemDurabilityUpdatePacket(&packet.GamePacket{
		At: time.UnixMilli(2000),
		Msg: packet.Message{
			packet.NewMessageElemLong(22518919189329483),
			packet.NewMessageElemInt(7040),
		},
	})
	state = a.GetSelfEquipmentState()
	for i := range state.Slots {
		if state.Slots[i].PocketType != 10 {
			continue
		}
		slot := state.Slots[i]
		if slot.DurabilityValue == nil || *slot.DurabilityValue != 7.04 {
			t.Fatalf("updated value = %#v", slot.DurabilityValue)
		}
		if slot.DurabilityDelta == nil || *slot.DurabilityDelta > -0.079 || *slot.DurabilityDelta < -0.081 {
			t.Fatalf("delta = %#v", slot.DurabilityDelta)
		}
		if len(slot.DurabilityHistory) != 2 {
			t.Fatalf("history len = %d", len(slot.DurabilityHistory))
		}
		return
	}
	t.Fatal("axe slot missing after update")
}

func TestSelfEquipmentSwapViaMoveUpdate(t *testing.T) {
	a := NewApp()

	seed := func(pocket uint32, itemID uint32, instanceID uint64, durPts, maxPts uint32) packet.Message {
		public := make([]byte, 80)
		binary.LittleEndian.PutUint32(public[0:], pocket)
		binary.LittleEndian.PutUint32(public[4:], itemID)
		extended := make([]byte, 144)
		binary.LittleEndian.PutUint32(extended[16:], durPts)
		binary.LittleEndian.PutUint32(extended[20:], maxPts)
		return packet.Message{
			packet.NewMessageElemLong(instanceID),
			packet.NewMessageElemByte(2),
			packet.NewMessageElemBin(public),
			packet.NewMessageElemBin(extended),
			packet.NewMessageElemString("OWNER:s:Flandre;"),
			packet.NewMessageElemString(""),
		}
	}

	// 初始：武器I主手为斧
	a.handleSelfInventoryPacket(&packet.GamePacket{
		At: time.UnixMilli(1000),
		Msg: packet.Message{
			packet.NewMessageElemByte(1),
			packet.NewMessageElemLong(4503599638666428),
			packet.NewMessageElemByte(2),
			packet.NewMessageElemString("Flandre"),
			packet.NewMessageElemString(""),
			packet.NewMessageElemString(""),
			packet.NewMessageElemInt(10001),
			packet.NewMessageElemLong(1001),
			packet.NewMessageElemByte(2),
			packet.NewMessageElemBin(func() []byte {
				b := make([]byte, 80)
				binary.LittleEndian.PutUint32(b[0:], 10)
				binary.LittleEndian.PutUint32(b[4:], 41160)
				return b
			}()),
			packet.NewMessageElemBin(func() []byte {
				b := make([]byte, 144)
				binary.LittleEndian.PutUint32(b[16:], 7120)
				binary.LittleEndian.PutUint32(b[20:], 22000)
				return b
			}()),
			packet.NewMessageElemString("OWNER:s:Flandre;"),
		},
	})

	// 手动/战术换装：旧武器移入背包，新武器进入主手
	a.handleSelfEquipmentItemUpdatePacket(&packet.GamePacket{
		At:  time.UnixMilli(2000),
		Msg: seed(10, 1250026, 2002, 31541, 32000),
	})
	a.handleSelfEquipmentItemUpdatePacket(&packet.GamePacket{
		At:  time.UnixMilli(2001),
		Msg: seed(2104, 41160, 1001, 7120, 22000),
	})

	state := a.GetSelfEquipmentState()
	var main *SelfEquipmentSlot
	for i := range state.Slots {
		if state.Slots[i].PocketType == 10 {
			main = &state.Slots[i]
			break
		}
	}
	if main == nil || main.ItemID != 1250026 || main.InstanceID != 2002 {
		t.Fatalf("main hand after swap = %#v", main)
	}
	if main.DurabilityValue == nil || *main.DurabilityValue != 31.541 {
		t.Fatalf("main dur = %#v", main.DurabilityValue)
	}

	// 战术栏再切回：新武器离位，旧斧回到主手
	a.handleSelfEquipmentItemUpdatePacket(&packet.GamePacket{
		At:  time.UnixMilli(3000),
		Msg: seed(2041, 1250026, 2002, 31541, 32000),
	})
	a.handleSelfEquipmentItemUpdatePacket(&packet.GamePacket{
		At:  time.UnixMilli(3001),
		Msg: seed(10, 41160, 1001, 7040, 22000),
	})

	state = a.GetSelfEquipmentState()
	main = nil
	for i := range state.Slots {
		if state.Slots[i].PocketType == 10 {
			main = &state.Slots[i]
			break
		}
	}
	if main == nil || main.ItemID != 41160 || main.InstanceID != 1001 {
		t.Fatalf("main hand after tactical return = %#v", main)
	}
	if main.DurabilityValue == nil || *main.DurabilityValue != 7.04 {
		t.Fatalf("returned axe dur = %#v", main.DurabilityValue)
	}
}

func TestSelfEquipmentIgnoresPetInventory(t *testing.T) {
	a := NewApp()
	a.selfId = "4503599638666428"
	a.selfName = "Flandre"

	public := make([]byte, 80)
	binary.LittleEndian.PutUint32(public[0:], 10)
	binary.LittleEndian.PutUint32(public[4:], 41160)
	extended := make([]byte, 144)
	binary.LittleEndian.PutUint32(extended[16:], 7120)
	binary.LittleEndian.PutUint32(extended[20:], 22000)

	a.handleSelfInventoryPacket(&packet.GamePacket{
		At: time.UnixMilli(1000),
		Msg: packet.Message{
			packet.NewMessageElemByte(1),
			packet.NewMessageElemLong(4503599638666428),
			packet.NewMessageElemByte(2),
			packet.NewMessageElemString("Flandre"),
			packet.NewMessageElemString(""),
			packet.NewMessageElemString(""),
			packet.NewMessageElemInt(10001),
			packet.NewMessageElemLong(22518919189329483),
			packet.NewMessageElemByte(2),
			packet.NewMessageElemBin(public),
			packet.NewMessageElemBin(extended),
			packet.NewMessageElemString("OWNER:s:Flandre;"),
		},
	})
	if !a.GetSelfEquipmentState().Ready {
		t.Fatal("player inventory should be ready")
	}

	// 宠物 0x5209：无观察中的装备栏，但仍含可解析物品块，旧逻辑会清空面板。
	petPublic := make([]byte, 80)
	binary.LittleEndian.PutUint32(petPublic[0:], 1) // 背包
	binary.LittleEndian.PutUint32(petPublic[4:], 1001)
	petExtended := make([]byte, 144)
	a.handleSelfInventoryPacket(&packet.GamePacket{
		At: time.UnixMilli(2000),
		Msg: packet.Message{
			packet.NewMessageElemByte(1),
			packet.NewMessageElemLong(4504699154742358),
			packet.NewMessageElemByte(2),
			packet.NewMessageElemString("Pet"),
			packet.NewMessageElemString(""),
			packet.NewMessageElemString(""),
			packet.NewMessageElemInt(490432),
			packet.NewMessageElemLong(999),
			packet.NewMessageElemByte(2),
			packet.NewMessageElemBin(petPublic),
			packet.NewMessageElemBin(petExtended),
			packet.NewMessageElemString("OWNER:s:Pet;"),
		},
	})

	state := a.GetSelfEquipmentState()
	if !state.Ready || state.Character != "Flandre" {
		t.Fatalf("pet wiped equipment state: %#v", state)
	}
	var axe *SelfEquipmentSlot
	for i := range state.Slots {
		if state.Slots[i].PocketType == 10 {
			axe = &state.Slots[i]
			break
		}
	}
	if axe == nil || axe.ItemID != 41160 {
		t.Fatalf("axe missing after pet inventory: %#v", axe)
	}
}

func TestSelfEquipmentManualDragUsesPocketMove(t *testing.T) {
	a := NewApp()

	publicA := make([]byte, 80)
	binary.LittleEndian.PutUint32(publicA[0:], 10)
	binary.LittleEndian.PutUint32(publicA[4:], 1250026)
	extendedA := make([]byte, 144)
	binary.LittleEndian.PutUint32(extendedA[16:], 31541)
	binary.LittleEndian.PutUint32(extendedA[20:], 32000)

	publicB := make([]byte, 80)
	binary.LittleEndian.PutUint32(publicB[0:], 1) // 背包
	binary.LittleEndian.PutUint32(publicB[4:], 41481)
	extendedB := make([]byte, 144)
	binary.LittleEndian.PutUint32(extendedB[16:], 35218)
	binary.LittleEndian.PutUint32(extendedB[20:], 36000)

	a.handleSelfInventoryPacket(&packet.GamePacket{
		At: time.UnixMilli(1000),
		Msg: packet.Message{
			packet.NewMessageElemByte(1),
			packet.NewMessageElemLong(4503599638666428),
			packet.NewMessageElemByte(2),
			packet.NewMessageElemString("Flandre"),
			packet.NewMessageElemString(""),
			packet.NewMessageElemString(""),
			packet.NewMessageElemInt(10001),
			packet.NewMessageElemLong(1001),
			packet.NewMessageElemByte(2),
			packet.NewMessageElemBin(publicA),
			packet.NewMessageElemBin(extendedA),
			packet.NewMessageElemString("OWNER:s:Flandre;"),
			packet.NewMessageElemLong(2002),
			packet.NewMessageElemByte(2),
			packet.NewMessageElemBin(publicB),
			packet.NewMessageElemBin(extendedB),
			packet.NewMessageElemString("OWNER:s:Flandre;"),
		},
	})

	// 手动拖拽互换：背包剑上主手，原主手回背包（无 0x59e0）
	a.handleSelfEquipmentPocketMovePacket(&packet.GamePacket{
		At: time.UnixMilli(2000),
		Msg: packet.Message{
			packet.NewMessageElemLong(2002),
			packet.NewMessageElemInt(1),
			packet.NewMessageElemInt(10),
			packet.NewMessageElemByte(2),
			packet.NewMessageElemByte(0),
			packet.NewMessageElemByte(0),
			packet.NewMessageElemLong(1001),
			packet.NewMessageElemInt(10),
			packet.NewMessageElemInt(1),
			packet.NewMessageElemByte(2),
			packet.NewMessageElemByte(0),
			packet.NewMessageElemByte(0),
		},
	})

	state := a.GetSelfEquipmentState()
	var main *SelfEquipmentSlot
	for i := range state.Slots {
		if state.Slots[i].PocketType == 10 {
			main = &state.Slots[i]
			break
		}
	}
	if main == nil || main.ItemID != 41481 || main.InstanceID != 2002 {
		t.Fatalf("manual drag main hand = %#v", main)
	}
	if main.DurabilityValue == nil || *main.DurabilityValue != 35.218 {
		t.Fatalf("manual drag dur = %#v", main.DurabilityValue)
	}
}

func TestSelfEquipmentIncludesWeaponOneAmmoPocket(t *testing.T) {
	a := NewApp()

	public := make([]byte, 80)
	binary.LittleEndian.PutUint32(public[0:], 14)
	binary.LittleEndian.PutUint32(public[4:], 1420001)
	extended := make([]byte, 144)
	binary.LittleEndian.PutUint32(extended[16:], 10000)
	binary.LittleEndian.PutUint32(extended[20:], 10000)

	a.handleSelfInventoryPacket(&packet.GamePacket{
		At: time.UnixMilli(1000),
		Msg: packet.Message{
			packet.NewMessageElemByte(1),
			packet.NewMessageElemLong(4503599638666428),
			packet.NewMessageElemByte(2),
			packet.NewMessageElemString("Flandre"),
			packet.NewMessageElemString(""),
			packet.NewMessageElemString(""),
			packet.NewMessageElemInt(10001),
			packet.NewMessageElemLong(22518906351513680),
			packet.NewMessageElemByte(2),
			packet.NewMessageElemBin(public),
			packet.NewMessageElemBin(extended),
			packet.NewMessageElemString(""),
		},
	})

	state := a.GetSelfEquipmentState()
	for i := range state.Slots {
		slot := state.Slots[i]
		if slot.PocketType != 14 {
			continue
		}
		if slot.ItemID != 1420001 || slot.SlotLabel != "武器I·副手" {
			t.Fatalf("weapon I ammo slot = %#v", slot)
		}
		if slot.DurabilityValue == nil || *slot.DurabilityValue != 10 {
			t.Fatalf("weapon I ammo durability = %#v", slot.DurabilityValue)
		}
		return
	}
	t.Fatal("weapon I ammo pocket missing")
}
