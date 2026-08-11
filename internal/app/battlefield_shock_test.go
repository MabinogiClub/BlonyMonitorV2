package app

import (
	"testing"
	"time"

	"blonymonitorv2/internal/packet"
)

func TestParseBattlefieldShockNotice(t *testing.T) {
	pkt := &packet.GamePacket{Msg: packet.Message{
		packet.NewMessageElemByte(4),
		packet.NewMessageElemString("温雯演奏的“战场的震慑”响彻在我军战场上。\n最大攻击力增加35%。\n最小攻击力增加35%。\n"),
	}}
	notice, ok := parseBattlefieldShockNotice(pkt)
	if !ok {
		t.Fatal("battlefield shock notice was not recognized")
	}
	if notice.Performer != "温雯" || notice.Strength != 35 {
		t.Fatalf("unexpected notice: %+v", notice)
	}

	pkt.Msg[0] = packet.NewMessageElemByte(8)
	if _, ok := parseBattlefieldShockNotice(pkt); ok {
		t.Fatal("non-effect message must not be recognized")
	}
}

func TestParseMusicPerformancePackets(t *testing.T) {
	msg := make(packet.Message, 33)
	for i := range msg {
		msg[i] = packet.NewMessageElemByte(0)
	}
	msg[0] = packet.NewMessageElemLong(123)
	msg[1] = packet.NewMessageElemInt(musicPerformanceHeader)
	msg[5] = packet.NewMessageElemString("single")
	msg[11] = packet.NewMessageElemString("7110;score")
	msg[27] = packet.NewMessageElemLong(456)
	msg[32] = packet.NewMessageElemInt(60_000)
	pkt := &packet.GamePacket{At: time.UnixMilli(10_000), Msg: msg}

	performance, ok := parseMusicPerformanceStart(pkt)
	if !ok || performance.InstanceID != 123 || performance.ActorID != 456 || performance.StartedAt != 1000 {
		t.Fatalf("unexpected performance: %+v, ok=%v", performance, ok)
	}
	stopID, ok := parseMusicPerformanceStop(&packet.GamePacket{Msg: packet.Message{packet.NewMessageElemLong(123)}})
	if !ok || stopID != 123 {
		t.Fatalf("unexpected stop: id=%d ok=%v", stopID, ok)
	}
}

func TestBattlefieldShockLeaseFeedsBossCoverage(t *testing.T) {
	a := NewApp()
	a.selfId = "1"
	a.entities["1"] = &EntityInfo{ID: "1", Name: "Self", IsPC: true}
	a.entities["2"] = &EntityInfo{ID: "2", Name: "温雯", IsPC: true}
	a.musicPerformances[123] = &musicPerformance{InstanceID: 123, ActorID: 2, StartedAt: 0}

	a.mu.Lock()
	state, _ := a.activateBattlefieldShockUnsafe(1, battlefieldShockNotice{
		Performer: "温雯",
		Strength:  35,
		Raw:       "震慑首次生效",
	}, 1000)
	if state.PerformanceID != 123 || state.ExpiresAt != 3000 {
		t.Fatalf("unexpected initial state: %+v", state)
	}
	if next := a.advanceBattlefieldShockUnsafe("1", state.Generation, 2000); next != 3000 || state.ExpiresAt != 4000 {
		t.Fatalf("unexpected refreshed state: %+v, next=%d", state, next)
	}
	delete(a.musicPerformances, 123)
	if next := a.advanceBattlefieldShockUnsafe("1", state.Generation, 2500); next != 4000 {
		t.Fatalf("stopped performance must retain lease, next=%d", next)
	}
	if next := a.advanceBattlefieldShockUnsafe("1", state.Generation, 4000); next != 0 {
		t.Fatalf("expired lease must stop, next=%d", next)
	}
	coverage := a.buildBuffCoverageUnsafe(1000, 5000, map[string]buffParticipant{
		"1": {name: "Self", isSelf: true},
	})
	a.mu.Unlock()

	if len(coverage) != 1 {
		t.Fatalf("unexpected coverage: %+v", coverage)
	}
	shock := findBuffCoverage(t, coverage[0], battlefieldShockStatusID)
	if shock.IconID != battlefieldShockIconID {
		t.Fatalf("battlefield shock icon = %d, want %d", shock.IconID, battlefieldShockIconID)
	}
	if shock.ActiveSeconds != 30 || shock.CoveragePercent != 75 || len(shock.Segments) != 1 {
		t.Fatalf("unexpected battlefield shock coverage: %+v", shock)
	}
	if shock.AverageStrength == nil || *shock.AverageStrength != 35 {
		t.Fatalf("unexpected battlefield shock strength: %+v", shock.AverageStrength)
	}
}

func TestBattlefieldShockLeaseMatchesObservedLogs(t *testing.T) {
	tests := []struct {
		name         string
		stopAt       int64
		actualExpiry int64
	}{
		{name: "first performance", stopAt: 4991, actualExpiry: 6026},
		{name: "second performance", stopAt: 4265, actualExpiry: 6031},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewApp()
			a.entities["1"] = &EntityInfo{ID: "1", Name: "Self", IsPC: true}
			a.entities["2"] = &EntityInfo{ID: "2", Name: "温雯", IsPC: true}
			a.musicPerformances[123] = &musicPerformance{InstanceID: 123, ActorID: 2, StartedAt: -1000}
			state, next := a.activateBattlefieldShockUnsafe(1, battlefieldShockNotice{
				Performer: "温雯",
				Strength:  35,
			}, 0)
			for next < tt.stopAt {
				next = a.advanceBattlefieldShockUnsafe("1", state.Generation, next)
			}
			delete(a.musicPerformances, 123)
			predictedExpiry := a.advanceBattlefieldShockUnsafe("1", state.Generation, next)
			delta := predictedExpiry - tt.actualExpiry
			if delta < 0 {
				delta = -delta
			}
			if delta > 35 {
				t.Fatalf("predicted expiry=%d, actual=%d, delta=%.2fs", predictedExpiry, tt.actualExpiry, float64(delta)/100)
			}
		})
	}
}
