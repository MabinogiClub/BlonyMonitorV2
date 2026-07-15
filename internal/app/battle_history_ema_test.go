package app

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestAggregateHitRecordsSeparatesNormalAndCriticalRanges(t *testing.T) {
	records := []SkillHitRecord{
		{Damage: 100, Timestamp: 100},
		{Damage: 250, IsCritical: true, Timestamp: 110},
		{Damage: 150, Timestamp: 120},
		{Damage: 300, IsCritical: true, Timestamp: 130},
	}

	total, hits, crits, min, max, critMin, critMax, _, _ := aggregateHitRecordsWithExport(records, nil)
	if total != 800 || hits != 4 || crits != 2 {
		t.Fatalf("aggregate totals = (%v, %d, %d)", total, hits, crits)
	}
	if min != 100 || max != 150 || critMin != 250 || critMax != 300 {
		t.Fatalf("damage ranges = normal %v~%v, critical %v~%v", min, max, critMin, critMax)
	}

	_, _, _, min, max, critMin, critMax, _, _ = aggregateHitRecordsWithExport([]SkillHitRecord{
		{Damage: 200, IsCritical: true, Timestamp: 100},
		{Damage: 300, IsCritical: true, Timestamp: 110},
	}, nil)
	if min != 0 || max != 0 || critMin != 200 || critMax != 300 {
		t.Fatalf("critical-only ranges = normal %v~%v, critical %v~%v", min, max, critMin, critMax)
	}
}

func TestLiveSkillAggregatesSeparateNormalAndCriticalRanges(t *testing.T) {
	a := newSummonDamageTestApp(t, true)
	a.addDamage(testPlayerID, testTargetID, 59_167, 100, false)
	a.addDamage(testPlayerID, testTargetID, 59_167, 250, true)

	playerID := entityID(testPlayerID)
	targetID := entityID(testTargetID)
	skill := a.skillStats[playerID][59_167]
	if skill == nil || skill.min != 100 || skill.max != 100 || skill.critMin != 250 || skill.critMax != 250 {
		t.Fatalf("player skill ranges = %#v", skill)
	}
	takenSkill := a.takenStats[targetID].attackers[playerID].skills[59_167]
	if takenSkill == nil || takenSkill.min != 100 || takenSkill.max != 100 || takenSkill.critMin != 250 || takenSkill.critMax != 250 {
		t.Fatalf("target skill ranges = %#v", takenSkill)
	}

	stats := a.GetDamageBySkill()
	if len(stats) != 1 || len(stats[0].Skills) != 1 {
		t.Fatalf("GetDamageBySkill() = %#v", stats)
	}
	got := stats[0].Skills[0]
	if got.MinDamage != 100 || got.MaxDamage != 100 || got.CritMinDamage != 250 || got.CritMaxDamage != 250 {
		t.Fatalf("exported skill ranges = %#v", got)
	}
}

func TestExportDurationSecondsMatchesEMA(t *testing.T) {
	tests := []struct {
		name       string
		start, end int64
		want       int64
	}{
		{name: "same tick", start: 100, end: 100, want: 1},
		{name: "sub-second", start: 100, end: 199, want: 1},
		{name: "fraction truncates", start: 100, end: 299, want: 1},
		{name: "whole seconds", start: 100, end: 300, want: 2},
		{name: "negative", start: 300, end: 100, want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exportDurationSeconds(tt.start, tt.end); got != tt.want {
				t.Fatalf("exportDurationSeconds(%d, %d) = %d, want %d", tt.start, tt.end, got, tt.want)
			}
		})
	}
}

func TestAttackerExportJSONMatchesEMASchema(t *testing.T) {
	data, err := json.Marshal(attackerExport{
		ID: "1", Name: "NPC", IsPC: false, LastHit: 200, AppearedAt: 100,
		Skills: []SkillDamageStats{},
	})
	if err != nil {
		t.Fatalf("marshal attacker export: %v", err)
	}

	var fields map[string]interface{}
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatalf("unmarshal attacker export: %v", err)
	}
	if isPC, ok := fields["isPC"]; !ok || isPC != false {
		t.Fatalf("isPC field = %#v, present = %v", isPC, ok)
	}
	if fields["lastHit"] != float64(200) || fields["appearedAt"] != float64(100) {
		t.Fatalf("attacker timing fields = %#v", fields)
	}
	if _, exists := fields["status"]; exists {
		t.Fatalf("unexpected status field in EMA-compatible export: %s", data)
	}
}

func TestTargetExportWritesEMAIntegerDurationAndReadsLegacyFraction(t *testing.T) {
	data, err := json.Marshal(targetExport{Duration: float64(exportDurationSeconds(100, 199))})
	if err != nil {
		t.Fatalf("marshal target export: %v", err)
	}
	if !bytes.Contains(data, []byte(`"duration":1`)) || bytes.Contains(data, []byte(`"duration":1.0`)) {
		t.Fatalf("new target duration is not EMA-compatible: %s", data)
	}

	var legacy targetExport
	if err := json.Unmarshal([]byte(`{"duration":12.34}`), &legacy); err != nil {
		t.Fatalf("unmarshal legacy fractional duration: %v", err)
	}
	if legacy.Duration != 12.34 {
		t.Fatalf("legacy duration = %v, want 12.34", legacy.Duration)
	}
}

func TestBuildSaveFileDataUsesEMASchemaAndRanges(t *testing.T) {
	a := newSummonDamageTestApp(t, false)
	a.addDamage(testSummonID, testTargetID, 59_167, 100, false)
	a.addDamage(testSummonID, testTargetID, 59_167, 250, true)

	saveData := a.buildSaveFileDataSince(0)
	if len(saveData.Targets) != 1 || len(saveData.Targets[0].Attackers) != 1 {
		t.Fatalf("save data = %#v", saveData)
	}
	target := saveData.Targets[0]
	attacker := target.Attackers[0]
	if target.Duration != 1 {
		t.Fatalf("target duration = %v, want 1", target.Duration)
	}
	if attacker.IsPC || attacker.AppearedAt <= 0 || attacker.LastHit < attacker.AppearedAt {
		t.Fatalf("attacker identity/timing = %#v", attacker)
	}
	if len(attacker.Skills) != 1 {
		t.Fatalf("attacker skills = %#v", attacker.Skills)
	}
	skill := attacker.Skills[0]
	if skill.MinDamage != 100 || skill.MaxDamage != 100 || skill.CritMinDamage != 250 || skill.CritMaxDamage != 250 {
		t.Fatalf("saved skill ranges = %#v", skill)
	}

	data, err := json.Marshal(saveData)
	if err != nil {
		t.Fatalf("marshal save data: %v", err)
	}
	if !bytes.Contains(data, []byte(`"isPC":false`)) || bytes.Contains(data, []byte(`"status"`)) {
		t.Fatalf("save JSON is not EMA-compatible: %s", data)
	}
}
