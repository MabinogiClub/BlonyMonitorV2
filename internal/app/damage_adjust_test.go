package app

import (
	"math/rand"
	"reflect"
	"testing"
)

// ---------------------------------------------------------------------------
// seqWindowRange / scanBossDamageWindow
// ---------------------------------------------------------------------------

func TestSeqWindowRange(t *testing.T) {
	records := []DamageRecord{
		{Seq: 1, Damage: 10},
		{Seq: 3, Damage: 0},
		{Seq: 5, Damage: 20},
		{Seq: 7, Damage: 30},
	}
	cases := []struct {
		name    string
		fromSeq int64
		toSeq   int64
		lo, hi  int
	}{
		{"excludes fromSeq includes toSeq", 1, 5, 1, 3},
		{"empty window", 2, 2, 1, 1},
		{"below all", -10, 0, 0, 0},
		{"above all", 8, 100, 4, 4},
		{"whole range", 0, 7, 0, 4},
		{"no positive records window", 2, 3, 1, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lo, hi := seqWindowRange(records, tc.fromSeq, tc.toSeq)
			if lo != tc.lo || hi != tc.hi {
				t.Fatalf("seqWindowRange(%d,%d) = (%d,%d), want (%d,%d)", tc.fromSeq, tc.toSeq, lo, hi, tc.lo, tc.hi)
			}
		})
	}
}

func TestSeqWindowRangeMatchesLinearFilter(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for iter := 0; iter < 2000; iter++ {
		n := rng.Intn(200)
		records := make([]DamageRecord, n)
		seq := int64(0)
		for i := range records {
			seq += int64(1 + rng.Intn(5))
			records[i] = DamageRecord{Seq: seq, Damage: float64(1 + rng.Intn(100))}
		}
		fromSeq := int64(rng.Intn(int(seq + 3)))
		toSeq := fromSeq
		if fromSeq < seq {
			toSeq = fromSeq + int64(rng.Intn(int(seq-fromSeq+2)))
		}

		lo, hi := seqWindowRange(records, fromSeq, toSeq)
		// 线性参考：收集所有满足 fromSeq < Seq <= toSeq 的下标
		var want []int
		for i, r := range records {
			if r.Seq > fromSeq && r.Seq <= toSeq {
				want = append(want, i)
			}
		}
		if len(want) == 0 {
			if lo != hi {
				t.Fatalf("iter %d: expected empty range, got [%d,%d)", iter, lo, hi)
			}
			continue
		}
		if lo != want[0] || hi != want[len(want)-1]+1 {
			t.Fatalf("iter %d: range mismatch: got [%d,%d), want [%d,%d)", iter, lo, hi, want[0], want[len(want)-1]+1)
		}
	}
}

func TestScanBossDamageWindow(t *testing.T) {
	records := []DamageRecord{
		{Seq: 1, Damage: 0},   // 跳过
		{Seq: 2, Damage: 100}, // 累计 100
		{Seq: 3, Damage: 200}, // 累计 300
		{Seq: 4, Damage: -5},  // 跳过
		{Seq: 5, Damage: 100}, // 累计 400
	}
	// limit = hpDelta + tolerance = 250 + 1 = 251，首次超过在 Seq 3
	scan := scanBossDamageWindow(records, 0, len(records), 250, 0.5)
	if scan.totalDamage != 400 {
		t.Fatalf("totalDamage = %v, want 400", scan.totalDamage)
	}
	if scan.hitCount != 3 {
		t.Fatalf("hitCount = %d, want 3", scan.hitCount)
	}
	if scan.triggerIndex != 2 {
		t.Fatalf("triggerIndex = %d, want 2", scan.triggerIndex)
	}

	// 无记录超过阈值：triggerIndex 落到最后一个正伤害记录
	scan = scanBossDamageWindow(records, 0, len(records), 1000, 1)
	if scan.triggerIndex != 4 {
		t.Fatalf("fallback triggerIndex = %d, want 4", scan.triggerIndex)
	}

	// 无正伤害记录
	scan = scanBossDamageWindow([]DamageRecord{{Seq: 1, Damage: 0}}, 0, 1, 100, 1)
	if scan.hitCount != 0 || scan.triggerIndex != -1 {
		t.Fatalf("empty scan = %+v", scan)
	}
}

// ---------------------------------------------------------------------------
// adjustBossDamageOverflowOnClone
// ---------------------------------------------------------------------------

func cloneRecords(records []DamageRecord) []DamageRecord {
	if records == nil {
		return nil
	}
	copied := make([]DamageRecord, len(records))
	copy(copied, records)
	return copied
}

func TestAdjustBossDamageOverflowOnCloneBasicCut(t *testing.T) {
	// seq 6..10 共 500，hpDelta 300 → 溢出 200，从尾部砍
	records := make([]DamageRecord, 10)
	for i := range records {
		records[i] = DamageRecord{Seq: int64(i + 1), Damage: 100}
	}
	adjustBossDamageOverflowOnClone(records, 5, 10, 300, 0, false, 1e9)

	for i := 0; i < 5; i++ { // seq 1..5（含 fromSeq=5）不动
		if records[i].Damage != 100 || records[i].Adjusted {
			t.Fatalf("record %d should be untouched, got %+v", i+1, records[i])
		}
	}
	for i := 5; i < 8; i++ { // seq 6..8 保留
		if records[i].Damage != 100 || records[i].Adjusted {
			t.Fatalf("record %d should keep 100, got %+v", i+1, records[i])
		}
	}
	for i := 8; i < 10; i++ { // seq 9..10 被砍光
		if records[i].Damage != 0 || !records[i].Adjusted || records[i].OverflowDamage != 100 {
			t.Fatalf("record %d should be cut to 0, got %+v", i+1, records[i])
		}
	}
}

func TestAdjustBossDamageOverflowOnCloneToleranceNoCut(t *testing.T) {
	records := []DamageRecord{
		{Seq: 1, Damage: 100},
		{Seq: 2, Damage: 100},
		{Seq: 3, Damage: 100.5},
	}
	// maxHP<=0 时 tolerance = epsilon(1.0)；溢出 0.5 <= 1.0 → 不砍
	adjustBossDamageOverflowOnClone(records, 0, 3, 300, 0, false, 0)
	for i := range records {
		if records[i].Adjusted || records[i].Damage <= 0 {
			t.Fatalf("record %d should be untouched, got %+v", i, records[i])
		}
	}
}

func TestAdjustBossDamageOverflowOnCloneLockTrigger(t *testing.T) {
	// maxHP=200000：tolerance 落到 epsilon(1.0)；overflow 2000 有意义
	records := []DamageRecord{
		{Seq: 1, Damage: 500},
		{Seq: 2, Damage: 500},
		{Seq: 3, Damage: 500},
		{Seq: 4, Damage: 500},
		{Seq: 5, Damage: 500},
		{Seq: 6, Damage: 500},
	}
	adjustBossDamageOverflowOnClone(records, 0, 6, 1000, 50, true, 200000)

	// 触发点：累计首次超过 1000+1 → Seq 3；触发记录被标记
	if !records[2].LockTriggered || records[2].LockThreshold != 50 {
		t.Fatalf("record 3 should be lock trigger, got %+v", records[2])
	}
	// 触发点之前的记录（Seq 1..2）完全不动
	for i := 0; i < 2; i++ {
		if records[i].Adjusted || records[i].LockThreshold != 0 || records[i].Damage != 500 {
			t.Fatalf("record %d should be untouched, got %+v", i+1, records[i])
		}
	}
	// 溢出 2000 需要从尾部砍 4 条（Seq 6,5,4 和触发记录 Seq 3）：
	// 削减循环只在越过触发点（i < triggerIndex）时才停止，触发记录本身允许被砍。
	for i := 2; i < 6; i++ {
		if records[i].Damage != 0 || !records[i].Adjusted || records[i].LockThreshold != 50 || records[i].OverflowDamage != 500 {
			t.Fatalf("record %d should be cut with threshold, got %+v", i+1, records[i])
		}
	}
}

func TestAdjustBossDamageOverflowOnCloneNoOps(t *testing.T) {
	cases := []struct {
		name    string
		records []DamageRecord
		fromSeq int64
		toSeq   int64
		hpDelta float64
	}{
		{"empty records", nil, 0, 10, 100},
		{"negative hpDelta", []DamageRecord{{Seq: 1, Damage: 100}}, 0, 1, -10},
		{"toSeq not after fromSeq", []DamageRecord{{Seq: 1, Damage: 100}}, 5, 5, 100},
		{"window beyond records", []DamageRecord{{Seq: 1, Damage: 100}}, 10, 20, 100},
		{"only non-positive damage", []DamageRecord{{Seq: 1, Damage: 0}, {Seq: 2, Damage: -3}}, 0, 2, 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			original := cloneRecords(tc.records)
			adjustBossDamageOverflowOnClone(tc.records, tc.fromSeq, tc.toSeq, tc.hpDelta, 0, false, 1e9)
			if !reflect.DeepEqual(tc.records, original) {
				t.Fatalf("records modified: %+v -> %+v", original, tc.records)
			}
		})
	}
}

func TestAdjustBossDamageOverflowOnCloneMatchesReference(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for iter := 0; iter < 3000; iter++ {
		n := rng.Intn(200)
		records := make([]DamageRecord, n)
		seq := int64(0)
		for i := range records {
			seq += int64(1 + rng.Intn(4))
			records[i] = DamageRecord{Seq: seq}
			switch rng.Intn(10) {
			case 0:
				records[i].Damage = 0
			case 1:
				records[i].Damage = float64(rng.Intn(60))
			default:
				records[i].Damage = float64(1 + rng.Intn(2000))
			}
		}
		fromSeq := int64(rng.Intn(int(seq + 3)))
		toSeq := fromSeq
		if fromSeq < seq {
			toSeq = fromSeq + int64(rng.Intn(int(seq-fromSeq+2)))
		}
		hpDelta := float64(rng.Intn(50000))
		maxHP := []float64{0, 100000, 200000, 1e9}[rng.Intn(4)]
		lockThreshold := []float64{0, 50}[rng.Intn(2)]
		markLockTrigger := rng.Intn(2) == 1

		got := cloneRecords(records)
		want := cloneRecords(records)
		adjustBossDamageOverflowOnClone(got, fromSeq, toSeq, hpDelta, lockThreshold, markLockTrigger, maxHP)
		referenceAdjustBossDamageOverflowOnClone(want, fromSeq, toSeq, hpDelta, lockThreshold, markLockTrigger, maxHP)

		if !reflect.DeepEqual(got, want) {
			t.Fatalf("iter %d mismatch\ninput=%+v\nfrom=%d to=%d hpDelta=%v maxHP=%v lock=%v mark=%v\ngot =%+v\nwant=%+v",
				iter, records, fromSeq, toSeq, hpDelta, maxHP, lockThreshold, markLockTrigger, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// computeExportDamageOnSnapshot
// ---------------------------------------------------------------------------

func TestComputeExportDamageOnSnapshotEndToEnd(t *testing.T) {
	targetDamages := map[string][]DamageRecord{
		"boss": {
			{Seq: 1, Damage: 100},
			{Seq: 2, Damage: 100},
			{Seq: 3, Damage: 100},
			{Seq: 4, Damage: 100},
		},
		"mob": { // 无 HP 历史：原始伤害原样导出
			{Seq: 5, Damage: 40},
			{Seq: 6, Damage: 40},
		},
	}
	bossHPHistory := map[string][]BossHPRecord{
		"boss": {
			{DamageSeq: 0, CurrentHP: 1000, MaxHP: 1000, Percent: 100},
			{DamageSeq: 4, CurrentHP: 700, MaxHP: 1000, Percent: 70}, // hpDelta 300，窗口 (0,4] 总 400 → 溢出 100
		},
	}
	bossHPMax := map[string]float64{"boss": 1000}

	original := map[string][]DamageRecord{
		"boss": cloneRecords(targetDamages["boss"]),
		"mob":  cloneRecords(targetDamages["mob"]),
	}

	exportDamage := computeExportDamageOnSnapshot(targetDamages, bossHPHistory, bossHPMax)

	if !reflect.DeepEqual(targetDamages, original) {
		t.Fatalf("computeExportDamageOnSnapshot mutated its input")
	}
	// 尾部 Seq 4 被砍 100 → 0
	want := map[int64]float64{1: 100, 2: 100, 3: 100, 4: 0, 5: 40, 6: 40}
	if !reflect.DeepEqual(exportDamage, want) {
		t.Fatalf("exportDamage = %v, want %v", exportDamage, want)
	}
}

func TestComputeExportDamageOnSnapshotDeathCap(t *testing.T) {
	targetDamages := map[string][]DamageRecord{
		"boss": {
			{Seq: 1, Damage: 400},
			{Seq: 2, Damage: 400},
			{Seq: 3, Damage: 400},
		},
	}
	bossHPHistory := map[string][]BossHPRecord{
		"boss": {
			{DamageSeq: 0, CurrentHP: 1000, MaxHP: 1000, Percent: 100},
			{DamageSeq: 3, CurrentHP: 0, MaxHP: 1000, Percent: 0}, // 死亡 → 总伤害封顶到 maxHP
		},
	}
	bossHPMax := map[string]float64{"boss": 1000}

	exportDamage := computeExportDamageOnSnapshot(targetDamages, bossHPHistory, bossHPMax)
	// 1200 封顶到 1000：从尾部砍 200（Seq 3 砍 200 → 200）
	want := map[int64]float64{1: 400, 2: 400, 3: 200}
	if !reflect.DeepEqual(exportDamage, want) {
		t.Fatalf("exportDamage = %v, want %v", exportDamage, want)
	}
}

func TestComputeExportDamageOnSnapshotShortHistoryUntouched(t *testing.T) {
	targetDamages := map[string][]DamageRecord{
		"boss": {{Seq: 1, Damage: 123}},
	}
	// 历史不足两条：不做任何修正
	bossHPHistory := map[string][]BossHPRecord{
		"boss": {{DamageSeq: 1, CurrentHP: 500, MaxHP: 1000, Percent: 50}},
	}
	bossHPMax := map[string]float64{"boss": 1000}

	exportDamage := computeExportDamageOnSnapshot(targetDamages, bossHPHistory, bossHPMax)
	if exportDamage[1] != 123 {
		t.Fatalf("exportDamage[1] = %v, want 123", exportDamage[1])
	}
}

func TestComputeExportDamageOnSnapshotMatchesReference(t *testing.T) {
	rng := rand.New(rand.NewSource(2024))
	for iter := 0; iter < 500; iter++ {
		targetDamages := make(map[string][]DamageRecord)
		bossHPHistory := make(map[string][]BossHPRecord)
		bossHPMax := make(map[string]float64)

		// 与生产一致：damageSeq 全局递增，跨目标唯一
		globalSeq := int64(0)
		for _, id := range []string{"a", "b", "c", "d"} {
			n := rng.Intn(80)
			records := make([]DamageRecord, n)
			for i := range records {
				globalSeq += int64(1 + rng.Intn(3))
				records[i] = DamageRecord{Seq: globalSeq}
				if rng.Intn(8) == 0 {
					records[i].Damage = 0
				} else {
					records[i].Damage = float64(1 + rng.Intn(1500))
				}
			}
			if len(records) > 0 {
				targetDamages[id] = records
			}

			// 随机历史（含 DamageSeq 回退与死亡结尾）
			h := rng.Intn(30)
			history := make([]BossHPRecord, 0, h)
			currentHP := float64(1000 + rng.Intn(5000))
			maxHP := currentHP
			damageSeq := int64(0)
			for j := 0; j < h; j++ {
				drop := float64(rng.Intn(int(currentHP) + 1))
				currentHP -= drop
				damageSeq += int64(rng.Intn(int(globalSeq) + 1))
				if rng.Intn(5) == 0 && j > 0 {
					damageSeq = history[j-1].DamageSeq // 模拟 seq 回退
				}
				percent := 0.0
				if maxHP > 0 {
					percent = currentHP / maxHP * 100
				}
				history = append(history, BossHPRecord{
					DamageSeq: damageSeq,
					CurrentHP: currentHP,
					MaxHP:     maxHP,
					Percent:   percent,
				})
			}
			if len(history) >= 2 {
				bossHPHistory[id] = history
			}
			bossHPMax[id] = maxHP
		}

		got := computeExportDamageOnSnapshot(targetDamages, bossHPHistory, bossHPMax)
		want := referenceComputeExportDamageOnSnapshot(cloneTargetDamagesForTest(targetDamages), bossHPHistory, bossHPMax)

		if !reflect.DeepEqual(got, want) {
			t.Fatalf("iter %d mismatch\nhistory=%+v\ngot =%v\nwant=%v", iter, bossHPHistory, got, want)
		}
	}
}

func cloneTargetDamagesForTest(src map[string][]DamageRecord) map[string][]DamageRecord {
	dst := make(map[string][]DamageRecord, len(src))
	for id, records := range src {
		copied := make([]DamageRecord, len(records))
		copy(copied, records)
		dst[id] = copied
	}
	return dst
}

func TestComputeExportDamageBySeqUnsafe(t *testing.T) {
	a := &App{
		targetDamages: map[string][]DamageRecord{
			"boss": {
				{Seq: 1, Damage: 100},
				{Seq: 2, Damage: 100},
			},
		},
		bossHPHistory: map[string][]BossHPRecord{
			"boss": {
				{DamageSeq: 0, CurrentHP: 500, MaxHP: 500, Percent: 100},
				{DamageSeq: 2, CurrentHP: 350, MaxHP: 500, Percent: 70}, // hpDelta 150，窗口总 200 → 溢出 50
			},
		},
		bossHP: map[string]*BossHPInfo{"boss": {MaxHP: 500}},
	}
	exportDamage := a.computeExportDamageBySeqUnsafe()
	// Seq 2 被砍 50 → 50
	if exportDamage[1] != 100 || exportDamage[2] != 50 {
		t.Fatalf("exportDamage = %v", exportDamage)
	}
	// 原记录不被改动
	if a.targetDamages["boss"][1].Damage != 100 {
		t.Fatalf("live records mutated: %+v", a.targetDamages["boss"])
	}
}

// ---------------------------------------------------------------------------
// bossHPHistory 上限
// ---------------------------------------------------------------------------

func TestCapBossHPHistoryUnsafe(t *testing.T) {
	a := &App{bossHPHistory: map[string][]BossHPRecord{}}
	const id = "boss"
	for i := 0; i < bossHPHistoryMaxEntries+100; i++ {
		a.bossHPHistory[id] = append(a.bossHPHistory[id], BossHPRecord{HpTimestamp: int64(i)})
	}
	a.capBossHPHistoryUnsafe(id)
	history := a.bossHPHistory[id]
	if len(history) != bossHPHistoryMaxEntries {
		t.Fatalf("len = %d, want %d", len(history), bossHPHistoryMaxEntries)
	}
	if history[0].HpTimestamp != 100 || history[len(history)-1].HpTimestamp != bossHPHistoryMaxEntries+99 {
		t.Fatalf("trimmed window wrong: first=%d last=%d", history[0].HpTimestamp, history[len(history)-1].HpTimestamp)
	}

	// 不足上限时不裁剪
	a.bossHPHistory[id] = a.bossHPHistory[id][:10]
	a.capBossHPHistoryUnsafe(id)
	if len(a.bossHPHistory[id]) != 10 {
		t.Fatalf("short history should be kept, len = %d", len(a.bossHPHistory[id]))
	}
}

// ---------------------------------------------------------------------------
// 旧线性实现参考（用于等价性测试，勿在生产代码中调用）
// ---------------------------------------------------------------------------

func referenceAdjustBossDamageOverflowOnClone(records []DamageRecord, fromSeq, toSeq int64, hpDelta float64, lockThreshold float64, markLockTrigger bool, maxHP float64) {
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

	triggerIndex := referenceFindBossLockTriggerIndex(records, indexes, hpDelta, tolerance)
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

func referenceFindBossLockTriggerIndex(records []DamageRecord, indexes []int, hpDelta float64, tolerance float64) int {
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

func referenceComputeExportDamageOnSnapshot(cloned map[string][]DamageRecord, bossHPHistory map[string][]BossHPRecord, bossHPMax map[string]float64) map[int64]float64 {
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
			referenceAdjustBossDamageOverflowOnClone(
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

// ---------------------------------------------------------------------------
// Benchmark
// ---------------------------------------------------------------------------

func BenchmarkAdjustBossDamageOverflowOnClone(b *testing.B) {
	const total = 10000
	records := make([]DamageRecord, total)
	for i := range records {
		records[i] = DamageRecord{Seq: int64(i + 1), Damage: 100}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 每次用小块窗口，衡量二分定位 + 单趟扫描的开销
		window := cloneRecords(records)
		adjustBossDamageOverflowOnClone(window, 5000, 5100, 500, 0, false, 1e9)
	}
}
