package app

import "math"

func (a *App) adjustBossDamageOverflowUnsafe(targetIdStr string, fromSeq, toSeq int64, hpDelta float64, lockThreshold float64, markLockTrigger bool, maxHP float64) bossDamageOverflowAdjustResult {
	result := bossDamageOverflowAdjustResult{LockThreshold: lockThreshold}
	if hpDelta < 0 || toSeq <= fromSeq {
		return result
	}
	records := a.targetDamages[targetIdStr]
	if len(records) == 0 {
		return result
	}

	indexes := make([]int, 0)
	totalDamage := 0.0
	for i := range records {
		r := records[i]
		if r.Seq <= fromSeq || r.Seq > toSeq || r.Damage <= 0 {
			continue
		}
		indexes = append(indexes, i)
		totalDamage += r.Damage
	}
	if len(indexes) == 0 {
		return result
	}

	tolerance := bossDamageSyncTolerance(maxHP)
	overflow := totalDamage - hpDelta
	if overflow <= tolerance {
		return result
	}
	result.Overflow = overflow

	triggerIndex := findBossLockTriggerIndex(records, indexes, hpDelta, tolerance)
	canMarkLockTrigger := markLockTrigger && lockThreshold > 0 && isMeaningfulBossLockOverflow(overflow, maxHP)
	if triggerIndex >= 0 && canMarkLockTrigger {
		trigger := &a.targetDamages[targetIdStr][triggerIndex]
		trigger.LockTriggered = true
		trigger.LockThreshold = lockThreshold
		result.TriggerFound = true
		result.Trigger = *trigger
		result.TriggerSeq = trigger.Seq
	}
	return result
}

func (a *App) adjustBossDamageOverflowOnClone(records []DamageRecord, fromSeq, toSeq int64, hpDelta float64, lockThreshold float64, markLockTrigger bool, maxHP float64) {
	if hpDelta < 0 || toSeq <= fromSeq || len(records) == 0 {
		return
	}

	indexes := make([]int, 0)
	totalDamage := 0.0
	for i := range records {
		r := records[i]
		if r.Seq <= fromSeq || r.Seq > toSeq || r.Damage <= 0 {
			continue
		}
		indexes = append(indexes, i)
		totalDamage += r.Damage
	}
	if len(indexes) == 0 {
		return
	}

	tolerance := bossDamageSyncTolerance(maxHP)
	overflow := totalDamage - hpDelta
	if overflow <= tolerance {
		return
	}

	triggerIndex := findBossLockTriggerIndex(records, indexes, hpDelta, tolerance)
	canMarkLockTrigger := markLockTrigger && lockThreshold > 0 && isMeaningfulBossLockOverflow(overflow, maxHP)
	canAttachLockThreshold := lockThreshold > 0 && (!markLockTrigger || canMarkLockTrigger)
	if triggerIndex >= 0 && canMarkLockTrigger {
		records[triggerIndex].LockTriggered = true
		records[triggerIndex].LockThreshold = lockThreshold
	}

	for i := len(indexes) - 1; i >= 0 && overflow > bossDamageAdjustEpsilon; i-- {
		if canMarkLockTrigger && triggerIndex >= 0 && indexes[i] < triggerIndex {
			break
		}
		record := &records[indexes[i]]
		cut := record.Damage
		if cut > overflow {
			cut = overflow
		}
		if cut <= 0 {
			continue
		}
		if canAttachLockThreshold && !record.LockTriggered {
			record.LockThreshold = lockThreshold
		}
		cutDamageRecordInPlace(record, cut)
		overflow -= cut
	}
}

func bossDamageSyncTolerance(maxHP float64) float64 {
	if maxHP <= 0 {
		return bossDamageAdjustEpsilon
	}

	hp32 := float32(maxHP)
	if hp32 <= 0 || math.IsInf(float64(hp32), 0) || math.IsNaN(float64(hp32)) {
		return bossDamageAdjustEpsilon
	}

	next := math.Nextafter32(hp32, float32(math.Inf(1)))
	step := float64(next - hp32)
	if step < bossDamageAdjustEpsilon {
		return bossDamageAdjustEpsilon
	}
	return step
}

func findBossLockTriggerIndex(records []DamageRecord, indexes []int, hpDelta float64, tolerance float64) int {
	if len(indexes) == 0 {
		return -1
	}
	if tolerance < bossDamageAdjustEpsilon {
		tolerance = bossDamageAdjustEpsilon
	}

	total := 0.0
	for i := 0; i < len(indexes); i++ {
		idx := indexes[i]
		total += records[idx].Damage
		if total > hpDelta+tolerance {
			return idx
		}
	}
	return indexes[len(indexes)-1]
}

func isMeaningfulBossLockOverflow(overflow, maxHP float64) bool {
	if maxHP < bossHPLockMinMaxHP {
		return false
	}
	if overflow < bossHPLockMinOverflow {
		return false
	}
	return (overflow/maxHP)*100 >= bossHPLockMinOverflowPercent
}

func (a *App) setPendingBossHPDamageWindowUnsafe(id string, window BossHPPendingDamageWindow) {
	if window.HPDelta <= bossDamageAdjustEpsilon {
		delete(a.bossHPPending, id)
		return
	}
	if pending := a.bossHPPending[id]; pending != nil && pending.FromSeq == window.FromSeq {
		pending.HPDelta += window.HPDelta
		if window.LockThreshold > 0 {
			pending.LockThreshold = window.LockThreshold
			pending.MarkLockTrigger = window.MarkLockTrigger
		}
		pending.MaxHP = window.MaxHP
		pending.CurrentHP = window.CurrentHP
		pending.CurrentPercent = window.CurrentPercent
		pending.Timestamp = window.Timestamp
		return
	}
	a.bossHPPending[id] = &window
}

func (a *App) resolvePendingBossHPDamageWindowUnsafe(id string, toSeq int64) []DamageRecord {
	pending := a.bossHPPending[id]
	if pending == nil || toSeq <= pending.FromSeq {
		return nil
	}

	records := a.targetDamages[id]
	totalDamage := 0.0
	for i := range records {
		r := records[i]
		if r.Seq <= pending.FromSeq || r.Seq > toSeq || r.Damage <= 0 {
			continue
		}
		totalDamage += r.Damage
	}

	tolerance := bossDamageSyncTolerance(pending.MaxHP)
	if totalDamage <= bossDamageAdjustEpsilon {
		return nil
	}
	if totalDamage < pending.HPDelta-tolerance {
		pending.LastAttemptSeq = toSeq
		pending.LastAttemptDamage = totalDamage
		return nil
	}

	overflowResult := a.adjustBossDamageOverflowUnsafe(
		id,
		pending.FromSeq,
		toSeq,
		pending.HPDelta,
		pending.LockThreshold,
		pending.MarkLockTrigger,
		pending.MaxHP,
	)
	lockThreshold, locked := a.markBossHPLockUnsafe(
		id,
		a.getEntityNameUnsafe(id),
		a.getEntityRaceIDUnsafe(id),
		pending.CurrentHP,
		pending.MaxHP,
		pending.CurrentPercent,
		pending.PrevHP,
		pending.PrevPercent,
		pending.Timestamp,
		overflowResult,
	)
	if locked {
		history := a.bossHPHistory[id]
		if len(history) > 0 {
			history[len(history)-1].Threshold = lockThreshold
			history[len(history)-1].Locked = true
			a.bossHPHistory[id] = history
		}
	}
	a.consumeBossHPDamageSeqUnsafe(id, pending.CurrentHP, toSeq)
	delete(a.bossHPPending, id)
	return overflowResult.Records
}

func (a *App) flushPendingBossHPDamageWindowsUnsafe() []DamageRecord {
	if len(a.bossHPPending) == 0 {
		return nil
	}
	adjusted := make([]DamageRecord, 0)
	for id := range a.bossHPPending {
		adjusted = append(adjusted, a.resolvePendingBossHPDamageWindowUnsafe(id, a.damageSeq)...)
	}
	return adjusted
}

func (a *App) consumeBossHPDamageSeqUnsafe(id string, currentHP float64, seq int64) {
	if seq <= 0 {
		return
	}
	if hp := a.bossHP[id]; hp != nil && hp.CurrentHP == currentHP && hp.DamageSeq < seq {
		hp.DamageSeq = seq
	}
	history := a.bossHPHistory[id]
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].CurrentHP != currentHP {
			continue
		}
		if history[i].DamageSeq < seq {
			history[i].DamageSeq = seq
		}
		a.bossHPHistory[id] = history
		return
	}
}

func cutDamageRecordInPlace(record *DamageRecord, cut float64) {
	oldDamage := record.Damage
	if cut > oldDamage {
		cut = oldDamage
	}
	record.Damage -= cut
	if record.Damage < 0 {
		record.Damage = 0
	}
	if record.RawDamage <= 0 {
		record.RawDamage = oldDamage
	}
	record.OverflowDamage += cut
	record.Adjusted = true
}

func cloneTargetDamagesMap(src map[string][]DamageRecord) map[string][]DamageRecord {
	dst := make(map[string][]DamageRecord, len(src))
	for id, records := range src {
		cloned := make([]DamageRecord, len(records))
		copy(cloned, records)
		dst[id] = cloned
	}
	return dst
}

func capTargetDamageRecordsToMaxHP(records []DamageRecord, maxHP float64) {
	if maxHP <= 0 || len(records) == 0 {
		return
	}
	total := 0.0
	for _, r := range records {
		total += r.Damage
	}
	if total <= maxHP+bossDamageAdjustEpsilon {
		return
	}
	overflow := total - maxHP
	for i := len(records) - 1; i >= 0 && overflow > bossDamageAdjustEpsilon; i-- {
		if records[i].Damage <= 0 {
			continue
		}
		cut := records[i].Damage
		if cut > overflow {
			cut = overflow
		}
		cutDamageRecordInPlace(&records[i], cut)
		overflow -= cut
	}
}

// computeExportDamageBySeqUnsafe 在历史导出时重放 Boss HP 时间线并修正伤害，不影响实时统计。
func (a *App) computeExportDamageBySeqUnsafe() map[int64]float64 {
	cloned := cloneTargetDamagesMap(a.targetDamages)
	for targetID, history := range a.bossHPHistory {
		records := cloned[targetID]
		if len(records) == 0 || len(history) < 2 {
			continue
		}
		for i := 1; i < len(history); i++ {
			prev := history[i-1]
			curr := history[i]
			hpDelta := prev.CurrentHP - curr.CurrentHP
			fromSeq := prev.DamageSeq
			toSeq := curr.DamageSeq
			if hpDelta <= bossDamageAdjustEpsilon || toSeq <= fromSeq {
				continue
			}
			lockThreshold := getPotentialBossHPLockThreshold(curr.CurrentHP, curr.Percent, prev.Percent)
			a.adjustBossDamageOverflowOnClone(
				records,
				fromSeq,
				toSeq,
				hpDelta,
				lockThreshold,
				lockThreshold > 0,
				curr.MaxHP,
			)
		}
		maxHP := 0.0
		if hp := a.bossHP[targetID]; hp != nil && hp.MaxHP > 0 {
			maxHP = hp.MaxHP
		} else if len(history) > 0 {
			maxHP = history[len(history)-1].MaxHP
		}
		last := history[len(history)-1]
		if last.CurrentHP <= bossDamageAdjustEpsilon && maxHP > 0 {
			capTargetDamageRecordsToMaxHP(records, maxHP)
		}
		cloned[targetID] = records
	}
	exportDamage := make(map[int64]float64)
	for _, records := range cloned {
		for _, r := range records {
			exportDamage[r.Seq] = r.Damage
		}
	}
	return exportDamage
}

func applyExportDamageToHitRecords(records []SkillHitRecord, exportDamage map[int64]float64) []SkillHitRecord {
	if len(exportDamage) == 0 {
		return records
	}
	adjusted := make([]SkillHitRecord, len(records))
	for i, r := range records {
		adjusted[i] = r
		if r.Seq > 0 {
			if dmg, ok := exportDamage[r.Seq]; ok {
				adjusted[i].Damage = dmg
				if dmg < r.Damage {
					adjusted[i].OverflowDamage += r.Damage - dmg
					adjusted[i].Adjusted = true
				}
			}
		}
	}
	return adjusted
}

func aggregateHitRecordsWithExport(records []SkillHitRecord, exportDamage map[int64]float64) (total float64, hits, crits int, min, max, critMin, critMax float64, firstHit, lastHit int64) {
	adjusted := applyExportDamageToHitRecords(records, exportDamage)
	return aggregateHitRecords(adjusted)
}

func (a *App) adjustDamageAfterKnownDeathUnsafe(targetIdStr string, damageFloat float64) (float64, float64, bool) {
	hp := a.bossHP[targetIdStr]
	if hp == nil || hp.MaxHP <= 0 || hp.CurrentHP > 0 {
		return damageFloat, 0, false
	}
	alreadyCounted := 0.0
	if targetStat := a.takenStats[targetIdStr]; targetStat != nil {
		alreadyCounted = targetStat.total
	}
	remaining := hp.MaxHP - alreadyCounted
	if remaining < 0 {
		remaining = 0
	}
	effectiveDamage := damageFloat
	if effectiveDamage > remaining {
		effectiveDamage = remaining
	}
	if effectiveDamage < 0 {
		effectiveDamage = 0
	}
	overflowDamage := damageFloat - effectiveDamage
	if overflowDamage < 0 {
		overflowDamage = 0
	}
	return effectiveDamage, overflowDamage, overflowDamage > bossDamageAdjustEpsilon
}
