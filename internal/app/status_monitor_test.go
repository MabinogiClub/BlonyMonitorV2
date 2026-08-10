package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"blonymonitorv2/internal/packet"
)

func TestBossStatusMonitorIncludesAdditionalBuffs(t *testing.T) {
	want := map[uint32]string{
		516: "觉醒",
		914: "喵咪的恩赐（物理）",
		915: "喵咪的恩赐（魔法）",
	}
	for conditionID, name := range want {
		if !isMonitoredStatus(conditionID) {
			t.Fatalf("condition %d is not monitored", conditionID)
		}
		if got := monitoredStatusNames[conditionID]; got != name {
			t.Fatalf("condition %d name = %q, want %q", conditionID, got, name)
		}
	}
}

func TestAdditionalBossStatusesAreNotBuffNotifications(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	mgr := NewBuffTimerManager(context.Background(), "")
	for _, conditionID := range []uint32{516, 914, 915} {
		if _, exists := mgr.targetBuffs[conditionID]; exists {
			t.Fatalf("condition %d must not be added to buff notifications", conditionID)
		}
	}
}

func findBuffCoverage(t *testing.T, player PlayerBuffCoverage, conditionID uint32) BuffCoverage {
	t.Helper()
	for _, buff := range player.Buffs {
		if buff.ConditionID == conditionID {
			return buff
		}
	}
	t.Fatalf("condition %d missing from coverage", conditionID)
	return BuffCoverage{}
}

func TestBuildBuffCoverageTracksStrengthSegments(t *testing.T) {
	a := NewApp()
	a.selfId = "1"
	a.selfName = "Self"
	a.entities["1"] = &EntityInfo{ID: "1", Name: "Self", IsPC: true}
	a.entities["2"] = &EntityInfo{ID: "2", Name: "Teammate", IsPC: true}
	a.mu.Lock()
	a.recordStatusConditionUnsafe(1, 680, true, "MCMBAMAX:f:50;SBT:8:1;", map[string]packet.ConditionDetailValue{
		"MCMBAMAX": {Type: "f", Value: "50"},
	}, 100)
	a.recordStatusConditionUnsafe(1, 680, true, "MCMBAMAX:f:60;SBT:8:1;", map[string]packet.ConditionDetailValue{
		"MCMBAMAX": {Type: "f", Value: "60"},
	}, 300)
	a.recordStatusConditionUnsafe(1, 680, false, "", nil, 500)
	a.recordStatusConditionUnsafe(2, 192, true, "LSMA:f:48.718998;SBT:8:1;", map[string]packet.ConditionDetailValue{
		"LSMA": {Type: "f", Value: "48.718998"},
	}, 50)
	coverage := a.buildBuffCoverageUnsafe(100, 600, map[string]buffParticipant{
		"1": {name: "Self", isSelf: true},
		"2": {name: "Teammate"},
	})
	a.mu.Unlock()

	if len(coverage) != 2 || len(coverage[0].Buffs) != len(monitoredStatusOrder) {
		t.Fatalf("unexpected coverage shape: %+v", coverage)
	}
	war := findBuffCoverage(t, coverage[0], 680)
	if war.CoveragePercent != 80 || war.ActiveSeconds != 4 || len(war.Segments) != 2 {
		t.Fatalf("unexpected war coverage: %+v", war)
	}
	if war.AverageStrength == nil || *war.AverageStrength != 55 {
		t.Fatalf("unexpected average strength: %+v", war.AverageStrength)
	}
	activeMarch := findBuffCoverage(t, coverage[1], 192)
	if coverage[1].PlayerName != "Teammate" || activeMarch.CoveragePercent != 100 || activeMarch.ActiveSeconds != 5 {
		t.Fatalf("unexpected teammate coverage: %+v", coverage[1])
	}
	if activeMarch.AverageStrength == nil || *activeMarch.AverageStrength != 48.718998 {
		t.Fatalf("unexpected teammate strength: %+v", activeMarch.AverageStrength)
	}
	encoded, err := json.Marshal(coverage)
	if err != nil {
		t.Fatalf("marshal coverage: %v", err)
	}
	if !strings.Contains(string(encoded), "MCMBAMAX:f:50") || !strings.Contains(string(encoded), `"details"`) {
		t.Fatalf("raw buff details were not preserved: %s", encoded)
	}
}

func TestBuildBuffCoverageExcludesNonParticipants(t *testing.T) {
	a := NewApp()
	a.entities["2"] = &EntityInfo{ID: "2", Name: "Nearby Player", IsPC: true}

	a.mu.Lock()
	a.recordStatusConditionUnsafe(2, 680, true, "MCMBAMAX:f:50;", map[string]packet.ConditionDetailValue{
		"MCMBAMAX": {Type: "f", Value: "50"},
	}, 100)
	coverage := a.buildBuffCoverageUnsafe(100, 600, map[string]buffParticipant{
		"1": {name: "Boss Attacker", isSelf: true},
	})
	a.mu.Unlock()

	if len(coverage) != 1 || coverage[0].PlayerID != "1" {
		t.Fatalf("expected only boss participants, got %+v", coverage)
	}
}

func TestFinishEventEndsPlayerStatuses(t *testing.T) {
	a := NewApp()
	a.selfId = "1"
	a.entities["1"] = &EntityInfo{ID: "1", Name: "Self", RaceID: 10001, IsPC: true}

	a.mu.Lock()
	a.recordStatusConditionUnsafe(1, 680, true, "MCMBAMAX:f:50;", map[string]packet.ConditionDetailValue{
		"MCMBAMAX": {Type: "f", Value: "50"},
	}, 100)
	a.mu.Unlock()

	a.addFinishEvent(1, 2)

	a.mu.RLock()
	defer a.mu.RUnlock()
	if len(a.activeStatusIntervals) != 0 {
		t.Fatalf("expected death to end all active statuses, got %d", len(a.activeStatusIntervals))
	}
	if len(a.statusIntervals) != 1 || a.statusIntervals[0].EndedAt == 0 {
		t.Fatalf("expected closed status interval, got %+v", a.statusIntervals)
	}
}
