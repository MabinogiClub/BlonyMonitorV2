package app

import (
	"testing"

	"blonymonitorv2/internal/packet"
)

func instanceNameMessage(name string, paramsIndex int, params string, size int) packet.Message {
	msg := make(packet.Message, size)
	for i := range msg {
		msg[i] = packet.NewMessageElemByte(0)
	}
	msg[5] = packet.NewMessageElemInt(717000)
	msg[6] = packet.NewMessageElemString(name)
	msg[paramsIndex] = packet.NewMessageElemString(params)
	return msg
}

func TestParseInstanceNamePacketLegacyQMMDGNIndex(t *testing.T) {
	pkt := &packet.GamePacket{Msg: instanceNameMessage(
		"布里列赫", 34,
		"QMTS:b:true;QMMDGN:8:157625986957967549;QVER:1:0;",
		35,
	)}
	name, id, ok := parseInstanceNamePacket(pkt)
	if !ok || name != "布里列赫" || id != 157625986957967549 {
		t.Fatalf("got name=%q id=%d ok=%v", name, id, ok)
	}
}

func TestParseInstanceNamePacketR143QMMDGNMovedToIndex36(t *testing.T) {
	pkt := &packet.GamePacket{Msg: instanceNameMessage(
		"布里列赫", 36,
		"QMTS:b:true;QMBEXP:f:1;QMMDGN:8:157625986957967554;QMMDFG:1:1;",
		50,
	)}
	name, id, ok := parseInstanceNamePacket(pkt)
	if !ok || name != "布里列赫" || id != 157625986957967554 {
		t.Fatalf("got name=%q id=%d ok=%v", name, id, ok)
	}
}

func TestParseInstanceNamePacketIgnoresQuestWithoutInstanceID(t *testing.T) {
	pkt := &packet.GamePacket{Msg: instanceNameMessage(
		"布里列赫", 36,
		"QMTS:b:true;QMBEXP:f:1;QVER:1:0;",
		50,
	)}
	if _, _, ok := parseInstanceNamePacket(pkt); ok {
		t.Fatal("quest packet without QMMDGN should be ignored")
	}
}
