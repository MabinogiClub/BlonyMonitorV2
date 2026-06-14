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
	canAttachLockThreshold := lockThreshold > 0 && (!markLockTrigger || canMarkLockTrigger)
	if triggerIndex >= 0 && canMarkLockTrigger {
		trigger := &a.targetDamages[targetIdStr][triggerIndex]
		trigger.LockTriggered = true
		trigger.LockThreshold = lockThreshold
		result.TriggerFound = true
		a.updateMirroredDamageRecordUnsafe(*trigger, trigger.Damage)
		a.updateSkillHitRecordUnsafe(*trigger, trigger.Damage)
		a.updateDamageEventLogUnsafe(*trigger, trigger.Damage)
	}

	adjusted := make([]DamageRecord, 0)
	for i := len(indexes) - 1; i >= 0 && overflow > bossDamageAdjustEpsilon; i-- {
		if canMarkLockTrigger && triggerIndex >= 0 && indexes[i] < triggerIndex {
			break
		}
		record := &a.targetDamages[targetIdStr][indexes[i]]
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
		a.applyDamageOverflowCutUnsafe(record, cut)
		overflow -= cut
		adjusted = append(adjusted, *record)
	}

	if triggerIndex >= 0 {
		result.Trigger = a.targetDamages[targetIdStr][triggerIndex]
		result.TriggerSeq = result.Trigger.Seq
	}
	result.Records = adjusted
	return result
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

func (a *App) applyDamageOverflowCutUnsafe(record *DamageRecord, cut float64) {
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

	a.updateMirroredDamageRecordUnsafe(*record, oldDamage)
	a.updateSkillHitRecordUnsafe(*record, oldDamage)
	a.updateDamageEventLogUnsafe(*record, oldDamage)
	a.subtractDamageAggregatesUnsafe(*record, cut)
}

func (a *App) updateMirroredDamageRecordUnsafe(adjusted DamageRecord, oldDamage float64) {
	for i := len(a.damages) - 1; i >= 0; i-- {
		r := &a.damages[i]
		if adjusted.Seq > 0 && r.Seq != adjusted.Seq {
			continue
		}
		if adjusted.Seq == 0 && (r.At != adjusted.At || r.AttackerID != adjusted.AttackerID || r.TargetID != adjusted.TargetID || r.SkillID != adjusted.SkillID || r.IsCritical != adjusted.IsCritical || math.Abs(r.Damage-oldDamage) > bossDamageAdjustEpsilon) {
			continue
		}
		*r = adjusted
		return
	}
}

func (a *App) updateSkillHitRecordUnsafe(adjusted DamageRecord, oldDamage float64) {
	targetStat := a.takenStats[adjusted.TargetID]
	if targetStat == nil {
		return
	}
	attackerStat := targetStat.attackers[adjusted.AttackerID]
	if attackerStat == nil {
		return
	}
	skillStat := attackerStat.skills[adjusted.SkillID]
	if skillStat == nil {
		return
	}
	for i := len(skillStat.records) - 1; i >= 0; i-- {
		r := &skillStat.records[i]
		if adjusted.Seq > 0 && r.Seq != adjusted.Seq {
			continue
		}
		if adjusted.Seq == 0 && (r.Timestamp != adjusted.At || r.IsCritical != adjusted.IsCritical || math.Abs(r.Damage-oldDamage) > bossDamageAdjustEpsilon) {
			continue
		}
		r.Damage = adjusted.Damage
		r.RawDamage = adjusted.RawDamage
		r.OverflowDamage = adjusted.OverflowDamage
		r.Adjusted = adjusted.Adjusted
		r.LockTriggered = adjusted.LockTriggered
		r.LockThreshold = adjusted.LockThreshold
		return
	}
}

func (a *App) updateDamageEventLogUnsafe(adjusted DamageRecord, oldDamage float64) {
	for i := len(a.eventLogs) - 1; i >= 0; i-- {
		log := &a.eventLogs[i]
		if log.Type != "damage" {
			continue
		}
		if adjusted.Seq > 0 && log.Seq != adjusted.Seq {
			continue
		}
		if adjusted.Seq == 0 && (log.At != adjusted.At || log.EntityID != adjusted.AttackerID || log.TargetID != adjusted.TargetID || log.SkillID != adjusted.SkillID || log.IsCritical != adjusted.IsCritical || math.Abs(log.Damage-oldDamage) > bossDamageAdjustEpsilon) {
			continue
		}
		log.Damage = adjusted.Damage
		log.RawDamage = adjusted.RawDamage
		log.OverflowDamage = adjusted.OverflowDamage
		log.Adjusted = adjusted.Adjusted
		log.LockTriggered = adjusted.LockTriggered
		log.LockThreshold = adjusted.LockThreshold
		return
	}
}

func (a *App) subtractDamageAggregatesUnsafe(record DamageRecord, cut float64) {
	if cut <= 0 {
		return
	}
	if attacker := a.attackerStats[record.AttackerID]; attacker != nil {
		attacker.total -= cut
		if attacker.total < 0 {
			attacker.total = 0
		}
	}
	if skillMap := a.skillStats[record.AttackerID]; skillMap != nil {
		if skill := skillMap[record.SkillID]; skill != nil {
			skill.total -= cut
			if skill.total < 0 {
				skill.total = 0
			}
			a.recalculateGlobalSkillExtremesUnsafe(record.AttackerID, record.SkillID, skill)
		}
	}
	a.totalDamage -= cut
	if a.totalDamage < 0 {
		a.totalDamage = 0
	}
	if target := a.takenStats[record.TargetID]; target != nil {
		target.total -= cut
		if target.total < 0 {
			target.total = 0
		}
		if attacker := target.attackers[record.AttackerID]; attacker != nil {
			attacker.total -= cut
			if attacker.total < 0 {
				attacker.total = 0
			}
			if skill := attacker.skills[record.SkillID]; skill != nil {
				skill.total -= cut
				if skill.total < 0 {
					skill.total = 0
				}
				a.recalculateTakenSkillExtremesUnsafe(skill)
			}
		}
	}
	// 图表数据不反向扣减；排行榜与战报使用修正后的聚合与明细。
}

func (a *App) recalculateGlobalSkillExtremesUnsafe(attackerID string, skillID int, stat *skillAggStats) {
	stat.min = 0
	stat.max = 0
	stat.critMin = 0
	stat.critMax = 0
	nonCritCount := 0
	critCount := 0
	for _, r := range a.damages {
		if r.AttackerID != attackerID || r.SkillID != skillID {
			continue
		}
		if r.IsCritical {
			critCount++
			if critCount == 1 || r.Damage < stat.critMin {
				stat.critMin = r.Damage
			}
			if r.Damage > stat.critMax {
				stat.critMax = r.Damage
			}
		} else {
			nonCritCount++
			if nonCritCount == 1 || r.Damage < stat.min {
				stat.min = r.Damage
			}
			if r.Damage > stat.max {
				stat.max = r.Damage
			}
		}
	}
}

func (a *App) recalculateTakenSkillExtremesUnsafe(stat *takenSkillAggStats) {
	stat.min = 0
	stat.max = 0
	stat.critMin = 0
	stat.critMax = 0
	nonCritCount := 0
	critCount := 0
	for _, r := range stat.records {
		if r.IsCritical {
			critCount++
			if critCount == 1 || r.Damage < stat.critMin {
				stat.critMin = r.Damage
			}
			if r.Damage > stat.critMax {
				stat.critMax = r.Damage
			}
		} else {
			nonCritCount++
			if nonCritCount == 1 || r.Damage < stat.min {
				stat.min = r.Damage
			}
			if r.Damage > stat.max {
				stat.max = r.Damage
			}
		}
	}
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

func (a *App) capDeadTargetDamageToMaxHPUnsafe(targetIdStr string) []DamageRecord {
	targetStat := a.takenStats[targetIdStr]
	if targetStat == nil {
		return nil
	}
	maxHP := 0.0
	if hp := a.bossHP[targetIdStr]; hp != nil && hp.MaxHP > 0 {
		maxHP = hp.MaxHP
	} else if history := a.bossHPHistory[targetIdStr]; len(history) > 0 {
		maxHP = history[len(history)-1].MaxHP
	}
	if maxHP <= 0 || targetStat.total <= maxHP+bossDamageAdjustEpsilon {
		return nil
	}
	overflow := targetStat.total - maxHP
	records := a.targetDamages[targetIdStr]
	adjusted := make([]DamageRecord, 0)
	for i := len(records) - 1; i >= 0 && overflow > bossDamageAdjustEpsilon; i-- {
		record := &a.targetDamages[targetIdStr][i]
		if record.Damage <= 0 {
			continue
		}
		cut := record.Damage
		if cut > overflow {
			cut = overflow
		}
		a.applyDamageOverflowCutUnsafe(record, cut)
		overflow -= cut
		adjusted = append(adjusted, *record)
	}
	return adjusted
}

func (a *App) capAllDeadTargetDamageToMaxHPUnsafe() []DamageRecord {
	adjusted := make([]DamageRecord, 0)
	for targetId := range a.takenStats {
		adjusted = append(adjusted, a.capDeadTargetDamageToMaxHPUnsafe(targetId)...)
	}
	return adjusted
}
