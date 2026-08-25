package packet

import "testing"

func TestParseItemPocketMovePacketSingle(t *testing.T) {
	msg := Message{
		NewMessageElemLong(22518915767165306),
		NewMessageElemInt(10),
		NewMessageElemInt(1),
		NewMessageElemByte(2),
		NewMessageElemByte(0),
		NewMessageElemByte(0),
	}
	moves := ParseItemPocketMovePacket(msg)
	if len(moves) != 1 {
		t.Fatalf("len=%d", len(moves))
	}
	if moves[0].InstanceID != 22518915767165306 || moves[0].FromPocket != 10 || moves[0].ToPocket != 1 {
		t.Fatalf("move=%#v", moves[0])
	}
}

func TestParseItemPocketMovePacketSwap(t *testing.T) {
	msg := Message{
		NewMessageElemLong(1001),
		NewMessageElemInt(1),
		NewMessageElemInt(10),
		NewMessageElemByte(2),
		NewMessageElemByte(0),
		NewMessageElemByte(0),
		NewMessageElemLong(1002),
		NewMessageElemInt(10),
		NewMessageElemInt(1),
		NewMessageElemByte(2),
		NewMessageElemByte(0),
		NewMessageElemByte(0),
	}
	moves := ParseItemPocketMovePacket(msg)
	if len(moves) != 2 {
		t.Fatalf("len=%d want 2", len(moves))
	}
	if moves[0].ToPocket != 10 || moves[1].ToPocket != 1 {
		t.Fatalf("moves=%#v", moves)
	}
}
