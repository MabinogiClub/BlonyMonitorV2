package app

import (
	"fmt"
	"testing"
)

func TestKeepRetainedTargetIDsKeepsHighHPAndRecentTrash(t *testing.T) {
	ranks := []targetRetentionRank{
		{ID: "old-boss", MaxHP: 1_800_000_000, Appeared: 10, LastHit: 20},
		{ID: "old-phantom", MaxHP: 50_000_000, Appeared: 11, LastHit: 21},
		{ID: "old-trash", MaxHP: 5_000_000, Appeared: 12, LastHit: 22},
		{ID: "new-trash", MaxHP: 2_000_000, Appeared: 800, LastHit: 900},
	}

	keep := keepRetainedTargetIDs(ranks)
	if _, ok := keep["old-boss"]; !ok {
		t.Fatal("20M+ boss should be kept")
	}
	if _, ok := keep["old-phantom"]; !ok {
		t.Fatal("20M+ phantom should be kept")
	}
	if _, ok := keep["new-trash"]; !ok {
		t.Fatal("recent low-HP target should stay on the time stack")
	}
	if _, ok := keep["old-trash"]; !ok {
		t.Fatal("low-HP stack is under the cap, so older trash stays")
	}
}

func TestKeepRetainedTargetIDsStacksLowHPByRecency(t *testing.T) {
	ranks := make([]targetRetentionRank, 0, 52)
	ranks = append(ranks, targetRetentionRank{
		ID: "old-boss", MaxHP: 60_000_000, Appeared: 1, LastHit: 2,
	})
	for i := 1; i <= 51; i++ {
		ranks = append(ranks, targetRetentionRank{
			ID:       fmt.Sprintf("trash-%d", i),
			MaxHP:    5_000_000,
			Appeared: int64(i),
			LastHit:  int64(i),
		})
	}

	keep := keepRetainedTargetIDs(ranks)
	if len(keep) != 51 {
		t.Fatalf("keep count = %d, want 51 (1 boss + 50 trash)", len(keep))
	}
	if _, ok := keep["old-boss"]; !ok {
		t.Fatal("old 20M+ target should not be evicted by the trash stack")
	}
	if _, ok := keep["trash-1"]; ok {
		t.Fatal("oldest low-HP target should fall off the time stack")
	}
	if _, ok := keep["trash-2"]; !ok {
		t.Fatal("newest 50 low-HP targets should remain")
	}
	if _, ok := keep["trash-51"]; !ok {
		t.Fatal("most recent low-HP target should remain")
	}
}

func TestKeepRetainedRawTargets(t *testing.T) {
	targets := []interface{}{
		map[string]interface{}{
			"targetId":   "old-boss",
			"appearedAt": float64(10),
			"bossHP":     map[string]interface{}{"maxHp": float64(1_800_000_000)},
		},
		map[string]interface{}{
			"targetId":   "old-trash",
			"appearedAt": float64(11),
			"bossHP":     map[string]interface{}{"maxHp": float64(5_000_000)},
		},
	}
	for i := 1; i <= 50; i++ {
		targets = append(targets, map[string]interface{}{
			"targetId":   fmt.Sprintf("trash-%d", i),
			"appearedAt": float64(100 + i),
			"bossHP":     map[string]interface{}{"maxHp": float64(2_000_000)},
		})
	}

	filtered := keepRetainedRawTargets(targets)
	seen := make(map[string]bool, len(filtered))
	for _, raw := range filtered {
		target := raw.(map[string]interface{})
		seen[target["targetId"].(string)] = true
	}
	if !seen["old-boss"] {
		t.Fatal("20M+ raw target should be kept")
	}
	if seen["old-trash"] {
		t.Fatal("old low-HP raw target should fall off the time stack")
	}
	if !seen["trash-50"] || !seen["trash-1"] {
		t.Fatal("newest 50 low-HP raw targets should remain")
	}
}

func addRetentionTarget(a *App, id string, maxHP float64, appeared, lastHit, seq int64) {
	a.takenStats[id] = &targetAggStats{
		name:     "mob-" + id,
		total:    maxHP,
		firstHit: appeared,
		lastHit:  lastHit,
		attackers: map[string]*takenAggStats{
			"player": {
				name:     "Player",
				isPC:     true,
				firstHit: appeared,
				lastHit:  lastHit,
				skills: map[int]*takenSkillAggStats{
					1: {records: []SkillHitRecord{{
						Seq:       seq,
						Timestamp: lastHit,
						Damage:    1,
					}}},
				},
			},
		},
	}
	a.bossHP[id] = &BossHPInfo{EntityID: id, MaxHP: maxHP}
	a.bossHPHistory[id] = []BossHPRecord{{
		EntityID:    id,
		MaxHP:       maxHP,
		CurrentHP:   maxHP,
		HpTimestamp: lastHit,
		DamageSeq:   seq,
	}}
}

func TestRetainedTakenTargetIDsKeepsBossesAndRecentTrash(t *testing.T) {
	a := NewApp()
	a.mu.Lock()
	defer a.mu.Unlock()
	addRetentionTarget(a, "old-boss", 1_800_000_000, 10, 20, 1)
	addRetentionTarget(a, "old-trash", 5_000_000, 11, 21, 2)
	for i := 1; i <= 50; i++ {
		addRetentionTarget(a, fmt.Sprintf("trash-%d", i), 2_000_000, int64(100+i), int64(100+i), int64(10+i))
	}

	keep := a.retainedTakenTargetIDsUnsafe(0, true)
	if _, ok := keep["old-boss"]; !ok {
		t.Fatal("20M+ live target should be kept")
	}
	if _, ok := keep["old-trash"]; ok {
		t.Fatal("old low-HP live target should fall off the time stack")
	}
	if _, ok := keep["trash-50"]; !ok {
		t.Fatal("newest low-HP live target should remain")
	}
	if len(keep) != 51 {
		t.Fatalf("keep count = %d, want 51", len(keep))
	}
}
