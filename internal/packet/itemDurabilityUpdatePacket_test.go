package packet

import "testing"

func TestParseItemDurabilityUpdatePacket(t *testing.T) {
	msg := Message{
		NewMessageElemLong(22518919189329483),
		NewMessageElemInt(7040),
	}
	update, ok := ParseItemDurabilityUpdatePacket(msg)
	if !ok {
		t.Fatal("ParseItemDurabilityUpdatePacket() = false")
	}
	if update.InstanceID != 22518919189329483 || update.DurabilityRaw != 7040 {
		t.Fatalf("unexpected update: %#v", update)
	}
	if update.DurabilityValue != 7.04 {
		t.Fatalf("DurabilityValue = %v, want 7.04", update.DurabilityValue)
	}
}
