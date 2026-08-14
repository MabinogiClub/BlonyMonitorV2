package app

import "testing"

func TestPendingDetailResetKeepsCumulativeDamageAndStartsWithNewHit(t *testing.T) {
	a := newSummonDamageTestApp(t, true)
	playerID := entityID(testPlayerID)
	targetID := entityID(testTargetID)
	const skillID = 59_167

	a.addDamage(testPlayerID, testTargetID, skillID, 100, false)
	a.currentMap = &CurrentMapInfo{MapID: 10_001}
	a.damageSeqAtLastAutoSave = a.damageSeq
	if !a.cleanupAndSaveTakenStats(100, "outside", detailResetDeferred) {
		t.Fatal("leaving transition unexpectedly failed")
	}
	if !a.detailResetPending || len(a.takenStats) != 1 {
		t.Fatalf("leaving transition did not retain visible detail: pending=%v targets=%d", a.detailResetPending, len(a.takenStats))
	}

	a.addDamage(testPlayerID, testTargetID, skillID, 250, true)

	if a.detailResetPending {
		t.Fatal("pending detail reset was not consumed by the first new hit")
	}
	if got := a.cumulativeAttackerStats[playerID]; got == nil || got.total != 100 || got.hits != 1 {
		t.Fatalf("cumulative attacker stats = %#v", got)
	}
	current := a.takenStats[targetID].attackers[playerID].skills[skillID]
	if current == nil || len(current.records) != 1 || current.records[0].Damage != 250 {
		t.Fatalf("current detailed skill records = %#v", current)
	}
	if a.damageSeq != 1 || a.damageSeqAtLastAutoSave != 0 {
		t.Fatalf("new segment sequence state = (%d, %d)", a.damageSeq, a.damageSeqAtLastAutoSave)
	}

	attackers := a.GetDamageByAttacker()
	if len(attackers) != 1 || attackers[0].TotalDamage != 350 || attackers[0].HitCount != 2 || attackers[0].CritCount != 1 {
		t.Fatalf("combined attacker stats = %#v", attackers)
	}
	skills := a.GetDamageBySkill()
	if len(skills) != 1 || len(skills[0].Skills) != 1 {
		t.Fatalf("combined skill stats = %#v", skills)
	}
	gotSkill := skills[0].Skills[0]
	if gotSkill.TotalDamage != 350 || gotSkill.HitCount != 2 || gotSkill.CritCount != 1 ||
		gotSkill.MinDamage != 100 || gotSkill.CritMinDamage != 250 {
		t.Fatalf("combined skill = %#v", gotSkill)
	}
}

func TestEntryResetReleasesDetailButKeepsDamageSummary(t *testing.T) {
	a := newSummonDamageTestApp(t, true)
	playerID := entityID(testPlayerID)
	a.addDamage(testPlayerID, testTargetID, 59_167, 1234, false)
	a.damageSeqAtLastAutoSave = a.damageSeq

	if !a.cleanupAndSaveTakenStats(10_001, "dungeon", detailResetImmediately) {
		t.Fatal("entry transition unexpectedly failed")
	}

	if len(a.takenStats) != 0 || len(a.targetDamages) != 0 || len(a.damages) != 0 {
		t.Fatalf("detailed state was retained: targets=%d targetRecords=%d recent=%d", len(a.takenStats), len(a.targetDamages), len(a.damages))
	}
	if got := a.cumulativeAttackerStats[playerID]; got == nil || got.total != 1234 {
		t.Fatalf("cumulative attacker stats = %#v", got)
	}
	if got := a.GetDamageByAttacker(); len(got) != 1 || got[0].TotalDamage != 1234 {
		t.Fatalf("damage view after detail reset = %#v", got)
	}

	a.mu.Lock()
	a.clearDamageStateUnsafe()
	a.mu.Unlock()
	if len(a.cumulativeAttackerStats) != 0 || len(a.chartAggData) != 0 {
		t.Fatalf("manual clear retained cumulative state: attackers=%d charts=%d", len(a.cumulativeAttackerStats), len(a.chartAggData))
	}
}

func TestCumulativeDamagePreservesTeamShareAcrossSegments(t *testing.T) {
	a := newSummonDamageTestApp(t, true)
	const teammateID uint64 = 2002
	teammateIDString := entityID(teammateID)
	a.entities[teammateIDString] = &EntityInfo{
		ID: teammateIDString, Name: "Teammate", RaceID: 10_001, IsPC: true,
	}

	a.addDamage(testPlayerID, testTargetID, 59_167, 100, false)
	a.addDamage(teammateID, testTargetID, 59_168, 300, true)
	a.damageSeqAtLastAutoSave = a.damageSeq
	if !a.cleanupAndSaveTakenStats(10_001, "dungeon", detailResetImmediately) {
		t.Fatal("first segment transition unexpectedly failed")
	}
	a.addDamage(testPlayerID, testTargetID, 59_167, 100, false)

	byID := make(map[string]DamageStats)
	for _, stats := range a.GetDamageByAttacker() {
		byID[stats.ID] = stats
	}
	player := byID[entityID(testPlayerID)]
	teammate := byID[teammateIDString]
	if player.TotalDamage != 200 || player.HitCount != 2 || player.Percent != 40 {
		t.Fatalf("player cumulative share = %#v", player)
	}
	if teammate.TotalDamage != 300 || teammate.HitCount != 1 || teammate.CritCount != 1 || teammate.Percent != 60 {
		t.Fatalf("teammate cumulative share = %#v", teammate)
	}
}

func TestRandomInstanceInternalMapChangeDoesNotSplitDetail(t *testing.T) {
	a := newSummonDamageTestApp(t, true)
	a.addDamage(testPlayerID, testTargetID, 59_167, 100, false)
	a.currentMap = &CurrentMapInfo{MapID: instanceMapIDMin}

	if !a.cleanupAndSaveTakenStats(instanceMapIDMin+1, "instance", detailResetNone) {
		t.Fatal("internal instance transition unexpectedly failed")
	}
	if len(a.takenStats) != 1 || a.detailResetPending || len(a.cumulativeAttackerStats) != 0 {
		t.Fatalf("internal transition split detail: targets=%d pending=%v cumulative=%d", len(a.takenStats), a.detailResetPending, len(a.cumulativeAttackerStats))
	}
}

func TestDetailResetPreservesActiveMusicBuffStateAndStrength(t *testing.T) {
	a := newSummonDamageTestApp(t, true)
	a.addDamage(testPlayerID, testTargetID, 59_167, 100, false)

	strength := 42.5
	key := statusIntervalKey{entityID: entityID(testPlayerID), conditionID: 680}
	active := &statusInterval{
		EntityID:    key.entityID,
		ConditionID: key.conditionID,
		StartedAt:   nowCentiseconds() - 100,
		RawDetail:   "music buff",
		Strength:    &strength,
	}
	a.statusIntervals = []*statusInterval{active}
	a.activeStatusIntervals[key] = active
	a.musicPerformances[99] = &musicPerformance{InstanceID: 99, ActorID: testPlayerID, StartedAt: active.StartedAt}
	a.battlefieldShockStates[key.entityID] = &battlefieldShockState{TargetID: testPlayerID, PerformanceID: 99}
	a.damageSeqAtLastAutoSave = a.damageSeq

	if !a.cleanupAndSaveTakenStats(10_001, "dungeon", detailResetImmediately) {
		t.Fatal("music buff entry transition unexpectedly failed")
	}

	continued := a.activeStatusIntervals[key]
	if continued == nil || continued == active || continued.Strength == nil || *continued.Strength != strength {
		t.Fatalf("active music buff was not continued with strength: %#v", continued)
	}
	if continued.RawDetail != active.RawDetail || continued.StartedAt < active.StartedAt {
		t.Fatalf("continued music buff metadata = %#v", continued)
	}
	if a.musicPerformances[99] == nil || a.battlefieldShockStates[key.entityID] == nil {
		t.Fatalf("music-derived state was cleared: performances=%v shock=%v", a.musicPerformances, a.battlefieldShockStates)
	}
}
