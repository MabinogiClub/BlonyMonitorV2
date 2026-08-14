package app

type detailResetMode uint8

const (
	detailResetNone detailResetMode = iota
	detailResetDeferred
	detailResetImmediately
)

// foldCurrentDamageIntoCumulativeUnsafe finalizes the current detailed segment
// using the same Boss overflow corrections as history export, then retains only
// the compact values needed by the multi-battle damage views.
func (a *App) foldCurrentDamageIntoCumulativeUnsafe() {
	if len(a.takenStats) == 0 {
		return
	}

	exportDamage := a.computeExportDamageBySeqUnsafe()
	if a.cumulativeAttackerStats == nil {
		a.cumulativeAttackerStats = make(map[string]*cumulativeAttackerAggStats)
	}
	for attackerID := range a.attackerStats {
		var name string
		var total float64
		var hits, crits int
		var firstHit, lastHit int64
		hasHits := false

		cumulative := a.cumulativeAttackerStats[attackerID]
		hadCumulative := cumulative != nil
		if cumulative == nil {
			cumulative = &cumulativeAttackerAggStats{skills: make(map[int]*skillAggStats)}
			a.cumulativeAttackerStats[attackerID] = cumulative
		}

		for _, targetStat := range a.takenStats {
			attacker := targetStat.attackers[attackerID]
			if attacker == nil || !attacker.isPC {
				continue
			}
			if attacker.name != "" {
				name = attacker.name
			}
			for skillID, skill := range attacker.skills {
				skillTotal, skillHits, skillCrits, min, max, critMin, critMax, skillFirst, skillLast :=
					aggregateHitRecordsWithExport(skill.records, exportDamage)
				if skillHits == 0 {
					continue
				}

				total += skillTotal
				hits += skillHits
				crits += skillCrits
				if !hasHits {
					firstHit, lastHit = skillFirst, skillLast
					hasHits = true
				} else {
					if skillFirst < firstHit {
						firstHit = skillFirst
					}
					if skillLast > lastHit {
						lastHit = skillLast
					}
				}

				merged := cumulative.skills[skillID]
				if merged == nil {
					merged = &skillAggStats{}
					cumulative.skills[skillID] = merged
				}
				mergeFinalizedSkillAgg(merged, skillTotal, skillHits, skillCrits, min, max, critMin, critMax)
			}
		}

		if !hasHits {
			if !hadCumulative {
				delete(a.cumulativeAttackerStats, attackerID)
			}
			continue
		}
		if name == "" {
			name = a.getEntityNameUnsafe(attackerID)
		}
		if name != "" {
			cumulative.name = name
		}
		cumulative.total += total
		cumulative.hits += hits
		cumulative.crits += crits
		cumulative.combatDuration += durationSeconds(firstHit, lastHit)
		if cumulative.firstHit == 0 || firstHit < cumulative.firstHit {
			cumulative.firstHit = firstHit
		}
		if lastHit > cumulative.lastHit {
			cumulative.lastHit = lastHit
		}
	}
}

func mergeFinalizedSkillAgg(dst *skillAggStats, total float64, hits, crits int, min, max, critMin, critMax float64) {
	existingNormalHits := dst.hits - dst.crits
	newNormalHits := hits - crits
	if newNormalHits > 0 {
		if existingNormalHits == 0 || min < dst.min {
			dst.min = min
		}
		if max > dst.max {
			dst.max = max
		}
	}
	if crits > 0 {
		if dst.crits == 0 || critMin < dst.critMin {
			dst.critMin = critMin
		}
		if critMax > dst.critMax {
			dst.critMax = critMax
		}
	}
	dst.total += total
	dst.hits += hits
	dst.crits += crits
}

func (a *App) markDetailResetPendingUnsafe() {
	if len(a.takenStats) > 0 {
		a.detailResetPending = true
	}
}

func (a *App) resetPendingDetailSegmentUnsafe(reason string) bool {
	if !a.detailResetPending {
		return false
	}
	a.foldCurrentDamageIntoCumulativeUnsafe()
	a.clearDetailDamageStateUnsafe()
	a.detailResetPending = false
	logger.Printf("[DamageSegment] reset detailed battle data: %s\n", reason)
	return true
}

func (a *App) applyDetailResetModeUnsafe(mode detailResetMode, reason string) {
	switch mode {
	case detailResetDeferred:
		a.markDetailResetPendingUnsafe()
	case detailResetImmediately:
		a.markDetailResetPendingUnsafe()
		a.resetPendingDetailSegmentUnsafe(reason)
	}
}

func mergedSkillDamageStats(skillID int, cumulative *skillAggStats, currentRecords []SkillHitRecord, exportDamage map[int64]float64, parentTotal float64, skillName string) SkillDamageStats {
	merged := skillAggStats{}
	if cumulative != nil {
		merged = *cumulative
	}
	currentTotal, hits, crits, min, max, critMin, critMax, _, _ := aggregateHitRecordsWithExport(currentRecords, exportDamage)
	mergeFinalizedSkillAgg(&merged, currentTotal, hits, crits, min, max, critMin, critMax)

	percent := 0.0
	if parentTotal > 0 {
		percent = merged.total / parentTotal * 100
	}
	average := 0.0
	if merged.hits > 0 {
		average = merged.total / float64(merged.hits)
	}
	return SkillDamageStats{
		SkillID:       skillID,
		SkillName:     skillName,
		TotalDamage:   merged.total,
		Percent:       percent,
		HitCount:      merged.hits,
		CritCount:     merged.crits,
		AvgDamage:     average,
		MinDamage:     merged.min,
		MaxDamage:     merged.max,
		CritMinDamage: merged.critMin,
		CritMaxDamage: merged.critMax,
	}
}
