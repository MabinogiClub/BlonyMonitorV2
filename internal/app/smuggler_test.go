package app

import (
	"testing"
	"time"

	"blonymonitorv2/internal/packet"
)

func smugglerTestPacket(at time.Time, text string) *packet.GamePacket {
	return &packet.GamePacket{At: at, Op: opcodeChineseName, Msg: packet.Message{
		packet.NewMessageElemByte(8), packet.NewMessageElemString(text),
	}}
}

func TestSmugglerNoticeMessages(t *testing.T) {
	tests := []struct {
		text, kind, area, item string
	}{
		{"在4分钟后地区的布拉格平原西北方处，出现了走私贩子. 今天的贸易物品为大理石。", "forecast", "布拉格平原西北方", "大理石"},
		{"在斯利比 岩石地带东侧地区出现了走私贩子。今天的贸易物品为热气结晶。", "appear", "斯利比 岩石地带东侧", "热气结晶"},
		{"布拉格平原西北方地区的走私贩子 4分钟后将会消失。", "disappear_forecast", "布拉格平原西北方", ""},
		{"在塔汀南侧地区出现了走私商人。今天的贸易物品为茶几", "appear", "塔汀南侧", "茶几"},
	}
	for _, test := range tests {
		t.Run(test.kind+test.item, func(t *testing.T) {
			got := parseSmugglerNotice(test.text)
			if got == nil || got.Type != test.kind || got.Area != test.area || got.Item != test.item {
				t.Fatalf("unexpected notice: %+v", got)
			}
		})
	}
	for _, text := range []string{"", "地下城名称", "获得了热气结晶", "走私贩子", "在塔汀地区出现了活动商人。今天的贸易物品为茶几。"} {
		if parseSmugglerNotice(text) != nil {
			t.Fatalf("unrelated announcement accepted: %q", text)
		}
	}
}

func TestSmugglerNoticeMergeAndExpiry(t *testing.T) {
	a := NewApp()
	start := time.Now().Add(-15 * time.Minute)
	forecast := smugglerTestPacket(start, "在4分钟后地区的布拉格平原西北方处，出现了走私贩子. 今天的贸易物品为大理石。")
	if !a.handleSmugglerNotice(forecast) {
		t.Fatal("forecast not recognized")
	}
	ending := smugglerTestPacket(start.Add(12*time.Minute), "布拉格平原西北方地区的走私贩子 4分钟后将会消失。")
	a.handleSmugglerNotice(ending)
	got := a.GetSmugglerNotice()
	if got.Item != "大理石" || got.Type != "disappear_forecast" || got.ReceivedAt != ending.At.UnixMilli() {
		t.Fatalf("same-cycle item not preserved: %+v", got)
	}
	// An older or duplicate packet must not overwrite the most recent phase.
	a.handleSmugglerNotice(forecast)
	if a.GetSmugglerNotice().Type != "disappear_forecast" {
		t.Fatal("out-of-order packet overwrote ending")
	}
	// The following cycle's ending alone must not inherit yesterday's item.
	ending.At = ending.At.Add(36 * time.Minute)
	a.handleSmugglerNotice(ending)
	if a.GetSmugglerNotice().Item != "" {
		t.Fatal("previous cycle's item leaked")
	}
	copy := a.GetSmugglerNotice()
	copy.Area = "mutated"
	if a.GetSmugglerNotice().Area == "mutated" {
		t.Fatal("getter returned mutable internal state")
	}
	a.observeServerConnection("127.0.0.1", 11020, true, 0)
	if a.GetSmugglerNotice() != nil {
		t.Fatal("connection change retained another channel's notice")
	}
}

func TestSmugglerNoticeDispatch(t *testing.T) {
	a := NewApp()
	pkt := smugglerTestPacket(time.Now(), "在布拉格平原西北方地区出现了走私贩子。今天的贸易物品为大理石。")
	a.processPacket(pkt)
	if got := a.GetSmugglerNotice(); got == nil || got.Item != "大理石" {
		t.Fatalf("announcement did not reach smuggler state: %+v", got)
	}
	if len(a.recentDungeonNameCandidates) != 0 {
		t.Fatal("smuggler announcement leaked into dungeon names")
	}
	pkt.Op = opcodeMapChange
	if a.handleSmugglerNotice(pkt) {
		t.Fatal("non-announcement opcode accepted")
	}
}
