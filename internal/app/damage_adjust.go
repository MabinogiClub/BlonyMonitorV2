package app

import (
	"math"
	"sort"
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

	// 在读锁内构建基础映射并只拷贝会参与重放的目标（写时复制），重放本身放到锁外。
	a.mu.RLock()
	exportDamage := buildExportDamageMap(a.targetDamages)
	adjustRecords := cloneAdjustableTargetDamages(a.targetDamages, a.bossHPHistory)
	bossHPHistory := cloneBossHPHistoryForTargets(a.bossHPHistory, adjustRecords)
	bossHPMax := snapshotBossHPMaxUnsafe(a.bossHP, a.bossHPHistory)
	a.mu.RUnlock()

	applyExportDamageAdjustments(adjustRecords, bossHPHistory, bossHPMax, exportDamage)

	a.exportDamageMu.Lock()
	a.exportDamageCache = exportDamage
	a.exportDamageDirty = false
	a.exportDamageBuiltAt = time.Now()
	a.exportDamageMu.Unlock()
	return exportDamage
}

// buildExportDamageMap 汇总所有目标的 seq→伤害原始映射。
func buildExportDamageMap(targetDamages map[string][]DamageRecord) map[int64]float64 {
	exportDamage := make(map[int64]float64)
	for _, records := range targetDamages {
		for _, r := range records {
			exportDamage[r.Seq] = r.Damage
		}
	}
	return exportDamage
}

// cloneAdjustableTargetDamages 只拷贝会参与 HP 时间线重放的目标（有伤害记录且 HP 历史
// 至少两条）。其余目标在重放中只读不写，无需拷贝，避免每次导出全量深拷贝。
func cloneAdjustableTargetDamages(targetDamages map[string][]DamageRecord, bossHPHistory map[string][]BossHPRecord) map[string][]DamageRecord {
	adjustRecords := make(map[string][]DamageRecord)
	for id, records := range targetDamages {
		if len(records) == 0 || len(bossHPHistory[id]) < 2 {
			continue
		}
		copied := make([]DamageRecord, len(records))
		copy(copied, records)
		adjustRecords[id] = copied
	}
	return adjustRecords
}

// cloneBossHPHistoryForTargets 只克隆 targetIDs 对应的 HP 历史。
func cloneBossHPHistoryForTargets(src map[string][]BossHPRecord, targetIDs map[string][]DamageRecord) map[string][]BossHPRecord {
	dst := make(map[string][]BossHPRecord, len(targetIDs))
	for id := range targetIDs {
		records, ok := src[id]
		if !ok {
			continue
		}
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

	lo, hi := seqWindowRange(records, fromSeq, toSeq)
	if lo >= hi {
		return result
	}

	tolerance := bossDamageSyncTolerance(maxHP)
	scan := scanBossDamageWindow(records, lo, hi, hpDelta, tolerance)
	if scan.hitCount == 0 {
		return result
	}
	overflow := scan.totalDamage - hpDelta
	if overflow <= tolerance {
		return result
	}
	result.Overflow = overflow

	canMarkLockTrigger := markLockTrigger && lockThreshold > 0 && isMeaningfulBossLockOverflow(overflow, maxHP)
	if scan.triggerIndex >= 0 && canMarkLockTrigger {
		trigger := &a.targetDamages[targetIdStr][scan.triggerIndex]
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

	lo, hi := seqWindowRange(records, fromSeq, toSeq)
	if lo >= hi {
		return
	}

	tolerance := bossDamageSyncTolerance(maxHP)
	scan := scanBossDamageWindow(records, lo, hi, hpDelta, tolerance)
	if scan.hitCount == 0 {
		return
	}
	overflow := scan.totalDamage - hpDelta
	if overflow <= tolerance {
		return
	}

	canMarkLockTrigger := markLockTrigger && lockThreshold > 0 && isMeaningfulBossLockOverflow(overflow, maxHP)
	canAttachLockThreshold := lockThreshold > 0 && (!markLockTrigger || canMarkLockTrigger)
	applyBossDamageOverflowCuts(records, lo, hi, overflow, scan.triggerIndex, lockThreshold, canMarkLockTrigger, canAttachLockThreshold)
}

// seqWindowRange 用二分查找定位 Seq ∈ (fromSeq, toSeq] 的记录区间 [lo, hi)。
// records 必须按 Seq 升序排列：targetDamages 只按全局递增的 damageSeq 追加，
// 从不重排，因此该前提始终成立。
func seqWindowRange(records []DamageRecord, fromSeq, toSeq int64) (lo, hi int) {
	lo = sort.Search(len(records), func(i int) bool { return records[i].Seq > fromSeq })
	hi = sort.Search(len(records), func(i int) bool { return records[i].Seq > toSeq })
	return lo, hi
}

type bossDamageWindowScan struct {
	totalDamage  float64 // 窗口内正伤害之和
	hitCount     int     // 窗口内正伤害记录数
	triggerIndex int     // 累计伤害首次超过 hpDelta+tolerance 的记录下标；未超过时为最后一个正伤害记录
}

// scanBossDamageWindow 单趟扫描窗口区间，同时完成伤害求和与锁血触发点定位，
// 替代原先“建索引求和 + 再扫一遍找触发点”的两次遍历。
func scanBossDamageWindow(records []DamageRecord, lo, hi int, hpDelta, tolerance float64) bossDamageWindowScan {
	scan := bossDamageWindowScan{triggerIndex: -1}
	if tolerance < bossDamageAdjustEpsilon {
		tolerance = bossDamageAdjustEpsilon
	}
	limit := hpDelta + tolerance
	triggerFound := false
	for i := lo; i < hi; i++ {
		damage := records[i].Damage
		if damage <= 0 {
			continue
		}
		scan.totalDamage += damage
		scan.hitCount++
		if !triggerFound {
			if scan.totalDamage > limit {
				scan.triggerIndex = i
				triggerFound = true
			} else {
				scan.triggerIndex = i
			}
		}
	}
	return scan
}

// applyBossDamageOverflowCuts 从窗口尾部向前削减溢出伤害；必要时先标记锁血触发记录。
func applyBossDamageOverflowCuts(records []DamageRecord, lo, hi int, overflow float64, triggerIndex int, lockThreshold float64, canMarkLockTrigger, canAttachLockThreshold bool) {
	if triggerIndex >= 0 && canMarkLockTrigger {
		records[triggerIndex].LockTriggered = true
		records[triggerIndex].LockThreshold = lockThreshold
	}
	for i := hi - 1; i >= lo && overflow > bossDamageAdjustEpsilon; i-- {
		if canMarkLockTrigger && triggerIndex >= 0 && i < triggerIndex {
			break
		}
		record := &records[i]
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
	lo, hi := seqWindowRange(records, pending.FromSeq, toSeq)
	totalDamage := 0.0
	for i := lo; i < hi; i++ {
		if records[i].Damage > 0 {
			totalDamage += records[i].Damage
		}
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

// computeExportDamageOnSnapshot 在目标伤害快照上重放 Boss HP 时间线并修正伤害。
// targetDamages/bossHPHistory 在调用期间必须保持一致快照（调用方负责加锁或提前克隆）。
// 只有会参与修正的目标才会拷贝记录（写时复制），其余目标仅读取。
func computeExportDamageOnSnapshot(targetDamages map[string][]DamageRecord, bossHPHistory map[string][]BossHPRecord, bossHPMax map[string]float64) map[int64]float64 {
	exportDamage := buildExportDamageMap(targetDamages)
	adjustRecords := cloneAdjustableTargetDamages(targetDamages, bossHPHistory)
	applyExportDamageAdjustments(adjustRecords, bossHPHistory, bossHPMax, exportDamage)
	return exportDamage
}

// applyExportDamageAdjustments 在拷贝出的目标记录上重放 Boss HP 时间线，
// 并把修正后的 seq→伤害写回 exportDamage。
func applyExportDamageAdjustments(adjustRecords map[string][]DamageRecord, bossHPHistory map[string][]BossHPRecord, bossHPMax map[string]float64, exportDamage map[int64]float64) {
	for targetID, records := range adjustRecords {
		history := bossHPHistory[targetID]
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
		for _, r := range records {
			exportDamage[r.Seq] = r.Damage
		}
	}
}

// computeExportDamageBySeqUnsafe 在持锁上下文内计算导出伤害（用于历史保存）。
func (a *App) computeExportDamageBySeqUnsafe() map[int64]float64 {
	return computeExportDamageOnSnapshot(
		a.targetDamages,
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
