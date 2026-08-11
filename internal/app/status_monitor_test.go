package app

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"blonymonitorv2/internal/packet"
)

func TestBossStatusMonitorIncludesAdditionalBuffs(t *testing.T) {
	want := map[uint32]string{
		63:                       "攻击力增加",
		516:                      "觉醒",
		914:                      "喵咪的恩赐（物理）",
		915:                      "喵咪的恩赐（魔法）",
		1023:                     "月亮",
		1033:                     "星星",
		1121:                     "魔法攻击强化",
		1150:                     "炼金术伤害增加",
		battlefieldShockStatusID: "战场的震慑",
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
	for _, conditionID := range []uint32{516, 914, 915, 1023, 1033, battlefieldShockStatusID} {
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

func TestChannelChangeExpiresTransientStatusesAndTimers(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	a := NewApp()
	a.selfId = "1"
	a.entities["1"] = &EntityInfo{
		ID:         "1",
		Name:       "Self",
		RaceID:     10001,
		IsPC:       true,
		Conditions: []uint32{63, 680, 999},
	}
	a.eventLogs = []EventLog{
		{Type: "condition", At: 100, EntityID: "1", EntityName: "Self", IsPC: true, ConditionID: 63, ConditionName: "攻击力增加", IsEnable: true},
		{Type: "condition", At: 100, EntityID: "1", EntityName: "Self", IsPC: true, ConditionID: 680, ConditionName: "战争序曲", IsEnable: true},
		{Type: "condition", At: 100, EntityID: "1", EntityName: "Self", IsPC: true, ConditionID: 999, ConditionName: "测试状态", IsEnable: true},
	}
	a.recordStatusConditionUnsafe(1, 63, true, "", nil, 100)
	a.recordStatusConditionUnsafe(1, 680, true, "", nil, 100)
	a.musicPerformances[1] = &musicPerformance{InstanceID: 1, ActorID: 1, StartedAt: 100}
	a.battlefieldShockStates["1"] = &battlefieldShockState{TargetID: 1}

	a.buffTimerMgr = NewBuffTimerManager(context.Background(), "1")
	a.buffTimerMgr.StartTimer(63, 1, "Self", 120)
	a.buffTimerMgr.StartTimer(680, 1, "Self", 120)

	if _, _, changed := a.observeServerConnection("211.147.76.31", 11020, false, 150); changed {
		t.Fatal("initial server observation must not be treated as a channel change")
	}
	channelName, previousChannelName, changed := a.observeServerConnection("211.147.76.32", 11020, true, 200)
	if !changed || previousChannelName != "[伊鲁夏 频道1]" || channelName != "[伊鲁夏 频道2]" {
		t.Fatalf("unexpected channel transition: changed=%v previous=%q current=%q", changed, previousChannelName, channelName)
	}

	if len(a.activeStatusIntervals) != 1 || a.activeStatusIntervals[statusIntervalKey{entityID: "1", conditionID: 63}] == nil {
		t.Fatalf("only the potion status should remain active: %+v", a.activeStatusIntervals)
	}
	war := a.statusIntervals[1]
	if war.ConditionID != 680 || war.EndedAt != 200 {
		t.Fatalf("war status was not ended at the channel change: %+v", war)
	}
	if got := a.entities["1"].Conditions; !slices.Equal(got, []uint32{63}) {
		t.Fatalf("entity conditions = %v, want [63]", got)
	}
	if len(a.musicPerformances) != 0 || len(a.battlefieldShockStates) != 0 {
		t.Fatalf("music-derived state was not cleared: performances=%v shock=%v", a.musicPerformances, a.battlefieldShockStates)
	}

	activeConditions := make([]uint32, 0, 1)
	conditionState := make(map[uint32]bool)
	for _, event := range a.eventLogs {
		if event.EntityID != "1" || event.Type != "condition" {
			continue
		}
		conditionState[event.ConditionID] = event.IsEnable
	}
	for conditionID, active := range conditionState {
		if active {
			activeConditions = append(activeConditions, conditionID)
		}
	}
	slices.Sort(activeConditions)
	if !slices.Equal(activeConditions, []uint32{63}) {
		t.Fatalf("displayed player condition events were not reset: %v", activeConditions)
	}
	timers := a.buffTimerMgr.GetActiveTimersInfo()
	if len(timers) != 1 || timers[0].CCId != 63 {
		t.Fatalf("only the potion timer should remain active: %+v", timers)
	}
}

func TestPotionStatusesPersistAcrossChannelChanges(t *testing.T) {
	for _, conditionID := range []uint32{63, 1121, 1150} {
		if !isChannelPersistentStatus(conditionID) {
			t.Errorf("condition %d must persist across channel changes", conditionID)
		}
	}
	if isChannelPersistentStatus(680) {
		t.Fatal("music buffs must expire on a channel change")
	}
}

func TestConnectionChangeSignalWorksBehindSharedProxyEndpoint(t *testing.T) {
	a := NewApp()
	if _, _, changed := a.observeServerConnection("127.0.0.1", 11020, false, 100); changed {
		t.Fatal("initial proxy connection must not be treated as a channel change")
	}
	if _, _, changed := a.observeServerConnection("127.0.0.1", 11020, true, 200); !changed {
		t.Fatal("TCP connection change must be honored even when the proxy endpoint is unchanged")
	}
}
