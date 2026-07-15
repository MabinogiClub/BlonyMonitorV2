package app

import (
	"math"
	"time"
)

const exportDamageCacheMinInterval = 400 * time.Millisecond

func (a *App) markExportDamageDirty() {
	a.exportDamageMu.Lock()
	a.exportDamageDirty = true
	a.exportDamageMu.Unlock()
}

func (a *App) invalidateExportDamageCache() {
	a.exportDamageMu.Lock()
	a.exportDamageCache = nil
	a.exportDamageDirty = true
	a.exportDamageMu.Unlock()
}

// getExportDamage 返回 Boss HP 溢出修正后的 seq→伤害映射；带缓存，避免每次 API 全量重放。
func (a *App) getExportDamage() map[int64]float64 {
	a.exportDamageMu.Lock()
	now := time.Now()
	if a.exportDamageCache != nil && !a.exportDamageDirty && now.Sub(a.exportDamageBuiltAt) < exportDamageCacheMinInterval {
		cache := a.exportDamageCache
		a.exportDamageMu.Unlock()
		return cache
	}
	a.exportDamageMu.Unlock()

	a.mu.RLock()
	targetDamages := cloneTargetDamagesMap(a.targetDamages)
	bossHPHistory := cloneBossHPHistoryMap(a.bossHPHistory)
	bossHPMax := snapshotBossHPMaxUnsafe(a.bossHP, a.bossHPHistory)
	a.mu.RUnlock()

	cache := computeExportDamageOnSnapshot(targetDamages, bossHPHistory, bossHPMax)

	a.exportDamageMu.Lock()
	a.exportDamageCache = cache
	a.exportDamageDirty = false
	a.exportDamageBuiltAt = time.Now()
	a.exportDamageMu.Unlock()
	return cache
}

func cloneBossHPHistoryMap(src map[string][]BossHPRecord) map[string][]BossHPRecord {
	dst := make(map[string][]BossHPRecord, len(src))
	for id, records := range src {
		copied := make([]BossHPRecord, len(records))
		copy(copied, records)
		dst[id] = copied
	}
	return dst
}

func snapshotBossHPMaxUnsafe(bossHP map[string]*BossHPInfo, bossHPHistory map[string][]BossHPRecord) map[string]float64 {
	maxMap := make(map[string]float64, len(bossHP)+len(bossHPHistory))
	for id, hp := range bossHP {
		if hp != nil && hp.MaxHP > 0 {
			maxMap[id] = hp.MaxHP
		}
	}
	for id, history := range bossHPHistory {
		if maxMap[id] > 0 || len(history) == 0 {
			continue
		}
		maxMap[id] = history[len(history)-1].MaxHP
	}
	return maxMap
}

func effectiveRecordDamage(r SkillHitRecord, exportDamage map[int64]float64) float64 {
	if r.Seq > 0 {
		if dmg, ok := exportDamage[r.Seq]; ok {
			return dmg
		}
	}
	return r.Damage
}

func aggregateHitRecordsFast(records []SkillHitRecord, exportDamage map[int64]float64) (total float64, hits, crits int, min, max, critMin, critMax float64, firstHit, lastHit int64) {
	normalHits := 0
	for i, r := range records {
		dmg := effectiveRecordDamage(r, exportDamage)
		total += dmg
		hits++
		if i == 0 {
			firstHit, lastHit = r.Timestamp, r.Timestamp
		} else {
			if r.Timestamp < firstHit {
				firstHit = r.Timestamp
			}
			if r.Timestamp > lastHit {
				lastHit = r.Timestamp
			}
		}
		if r.IsCritical {
			crits++
			if crits == 1 || dmg < critMin {
				critMin = dmg
			}
			if dmg > critMax {
				critMax = dmg
			}
		} else {
			normalHits++
			if normalHits == 1 || dmg < min {
				min = dmg
			}
			if dmg > max {
				max = dmg
			}
		}
	}
	return total, hits, crits, min, max, critMin, critMax, firstHit, lastHit
}

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

func adjustBossDamageOverflowOnClone(records []DamageRecord, fromSeq, toSeq int64, hpDelta float64, lockThreshold float64, markLockTrigger bool, maxHP float64) {
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

// computeExportDamageOnSnapshot 在数据快照上重放 Boss HP 时间线并修正伤害。
func computeExportDamageOnSnapshot(cloned map[string][]DamageRecord, bossHPHistory map[string][]BossHPRecord, bossHPMax map[string]float64) map[int64]float64 {
	for targetID, history := range bossHPHistory {
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
			adjustBossDamageOverflowOnClone(
				records,
				fromSeq,
				toSeq,
				hpDelta,
				lockThreshold,
				lockThreshold > 0,
				curr.MaxHP,
			)
		}
		maxHP := bossHPMax[targetID]
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

// computeExportDamageBySeqUnsafe 在持锁上下文内计算导出伤害（用于历史保存）。
func (a *App) computeExportDamageBySeqUnsafe() map[int64]float64 {
	return computeExportDamageOnSnapshot(
		cloneTargetDamagesMap(a.targetDamages),
		a.bossHPHistory,
		snapshotBossHPMaxUnsafe(a.bossHP, a.bossHPHistory),
	)
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
	return aggregateHitRecordsFast(records, exportDamage)
}

func (a *App) buildSkillDamageStatsFromRecordsUnsafe(skillID int, records []SkillHitRecord, exportDamage map[int64]float64, parentTotal float64) SkillDamageStats {
	total, hits, crits, min, max, critMin, critMax, _, _ := aggregateHitRecordsWithExport(records, exportDamage)
	avgDamage := 0.0
	if hits > 0 {
		avgDamage = total / float64(hits)
	}
	percent := 0.0
	if parentTotal > 0 {
		percent = (total / parentTotal) * 100
	}
	return SkillDamageStats{
		SkillID:       skillID,
		SkillName:     a.getSkillNameUnsafe(skillID),
		TotalDamage:   total,
		Percent:       percent,
		HitCount:      hits,
		CritCount:     crits,
		AvgDamage:     avgDamage,
		MinDamage:     min,
		MaxDamage:     max,
		CritMinDamage: critMin,
		CritMaxDamage: critMax,
	}
}

func (a *App) aggregateAttackerHitRecordsUnsafe(attackerID string, exportDamage map[int64]float64) (total float64, hits, crits int) {
	for _, targetStat := range a.takenStats {
		att := targetStat.attackers[attackerID]
		if att == nil {
			continue
		}
		for _, skillStat := range att.skills {
			t, h, c, _, _, _, _, _, _ := aggregateHitRecordsWithExport(skillStat.records, exportDamage)
			total += t
			hits += h
			crits += c
		}
	}
	return total, hits, crits
}

func (a *App) aggregateTargetHitRecordsUnsafe(targetID string, exportDamage map[int64]float64) float64 {
	targetStat := a.takenStats[targetID]
	if targetStat == nil {
		return 0
	}
	total := 0.0
	for _, att := range targetStat.attackers {
		for _, skillStat := range att.skills {
			t, _, _, _, _, _, _, _, _ := aggregateHitRecordsWithExport(skillStat.records, exportDamage)
			total += t
		}
	}
	return total
}

func (a *App) aggregateAdjustedPCDamageUnsafe(exportDamage map[int64]float64) float64 {
	total := 0.0
	for _, targetStat := range a.takenStats {
		for _, att := range targetStat.attackers {
			if !att.isPC {
				continue
			}
			for _, skillStat := range att.skills {
				t, _, _, _, _, _, _, _, _ := aggregateHitRecordsWithExport(skillStat.records, exportDamage)
				total += t
			}
		}
	}
	return total
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
