package app

import (
	"blonymonitorv2/db"
	"sort"
	"strconv"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// addDamage 添加伤害记录
func (a *App) addDamage(attackerId, targetId uint64, skillId uint16, damage float32, isCritical bool) {
	now := time.Now().Unix()
	attackerIdStr := strconv.FormatUint(attackerId, 10)
	targetIdStr := strconv.FormatUint(targetId, 10)
	damageFloat := float64(damage)

	// 用于锁外操作的变量
	var shouldTriggerAttackerTimer bool
	var shouldTriggerTargetTimer bool
	var record DamageRecord
	var pendingAdjusted []DamageRecord

	// 在锁内完成所有数据更新
	a.mu.Lock()

	attackerName := a.getEntityNameUnsafe(attackerIdStr)
	targetName := a.getEntityNameUnsafe(targetIdStr)
	effectiveDamage, overflowDamage, adjusted := a.adjustDamageAfterKnownDeathUnsafe(targetIdStr, damageFloat)
	a.damageSeq++

	record = DamageRecord{
		Seq:            a.damageSeq,
		AttackerID:     attackerIdStr,
		AttackerName:   attackerName,
		TargetID:       targetIdStr,
		TargetName:     targetName,
		SkillID:        int(skillId),
		Damage:         effectiveDamage,
		RawDamage:      damageFloat,
		OverflowDamage: overflowDamage,
		Adjusted:       adjusted,
		IsCritical:     isCritical,
		At:             now,
	}

	a.damages = append(a.damages, record)
	a.targetDamages[targetIdStr] = append(a.targetDamages[targetIdStr], record)

	// 保留最近 2000 条记录（仅用于图表和最近记录展示）
	if len(a.damages) > 2000 {
		a.damages = a.damages[len(a.damages)-2000:]
	}

	// 更新聚合统计（只统计PC造成的伤害）
	entity := a.entities[attackerIdStr]
	if entity != nil && entity.IsPC {
		// 更新攻击者统计
		attackerStat := a.attackerStats[attackerIdStr]
		if attackerStat == nil {
			attackerStat = &attackerAggStats{firstHit: now}
			a.attackerStats[attackerIdStr] = attackerStat
		}
		attackerStat.total += effectiveDamage
		attackerStat.hits++
		if isCritical {
			attackerStat.crits++
		}
		attackerStat.lastHit = now

		// 更新技能统计
		if a.skillStats[attackerIdStr] == nil {
			a.skillStats[attackerIdStr] = make(map[int]*skillAggStats)
		}
		skillStat := a.skillStats[attackerIdStr][int(skillId)]
		if skillStat == nil {
			skillStat = &skillAggStats{min: damageFloat, max: damageFloat}
			a.skillStats[attackerIdStr][int(skillId)] = skillStat
		}
		skillStat.total += effectiveDamage
		skillStat.hits++
		if isCritical {
			skillStat.crits++
			if skillStat.crits == 1 || effectiveDamage < skillStat.critMin {
				skillStat.critMin = effectiveDamage
			}
			if effectiveDamage > skillStat.critMax {
				skillStat.critMax = effectiveDamage
			}
		}
		if effectiveDamage < skillStat.min {
			skillStat.min = effectiveDamage
		}
		if effectiveDamage > skillStat.max {
			skillStat.max = effectiveDamage
		}

		// 更新总伤害
		a.totalDamage += effectiveDamage

		// 更新图表聚合数据（每30秒一个桶，保留2小时）
		// 使用角色名称作为 key，这样同一玩家的不同实体ID会被合并
		attackerName := a.getEntityNameUnsafe(attackerIdStr)
		a.updateChartAggData(attackerName, now, effectiveDamage)

		// 更新按目标分组的图表聚合数据（用于怪物跟踪模式）
		a.updateTargetChartAggData(targetIdStr, attackerName, now, effectiveDamage)

		// 标记需要触发计时器（锁外执行）
		shouldTriggerAttackerTimer = true
	}

	// 更新受到伤害聚合统计（所有伤害都统计，不限于PC）
	// 先获取攻击者和目标的信息（用于缓存）
	attackerRaceID := a.getEntityRaceIDUnsafe(attackerIdStr)
	attackerIsPC := attackerRaceID >= 0 && isPC(attackerRaceID)
	attackerRaceName := ""
	if attackerRaceID >= 0 && !attackerIsPC {
		attackerRaceName = a.getRaceNameUnsafe(attackerRaceID)
	}
	attackerName = a.getEntityNameUnsafe(attackerIdStr)

	targetRaceID := a.getEntityRaceIDUnsafe(targetIdStr)
	targetIsPC := targetRaceID >= 0 && isPC(targetRaceID)
	targetRaceName := ""
	if targetRaceID >= 0 && !targetIsPC {
		targetRaceName = a.getRaceNameUnsafe(targetRaceID)
	}
	targetName = a.getEntityNameUnsafe(targetIdStr)

	targetStat := a.takenStats[targetIdStr]
	if targetStat == nil {
		targetStat = &targetAggStats{
			total:     0,
			attackers: make(map[string]*takenAggStats),
			firstHit:  now,
			name:      targetName,
			raceId:    targetRaceID,
			isPC:      targetIsPC,
		}
		a.takenStats[targetIdStr] = targetStat
	}
	targetStat.total += effectiveDamage
	targetStat.lastHit = now

	attackerTakenStat := targetStat.attackers[attackerIdStr]
	if attackerTakenStat == nil {
		attackerTakenStat = &takenAggStats{
			skills:   make(map[int]*takenSkillAggStats),
			firstHit: now,
			name:     attackerName,
			raceId:   attackerRaceID,
			isPC:     attackerIsPC,
		}
		targetStat.attackers[attackerIdStr] = attackerTakenStat
	}
	attackerTakenStat.total += effectiveDamage
	attackerTakenStat.hits++
	attackerTakenStat.lastHit = now
	if isCritical {
		attackerTakenStat.crits++
	}

	// 更新技能统计
	takenSkillStat := attackerTakenStat.skills[int(skillId)]
	if takenSkillStat == nil {
		takenSkillStat = &takenSkillAggStats{min: damageFloat, max: damageFloat}
		attackerTakenStat.skills[int(skillId)] = takenSkillStat
	}
	takenSkillStat.total += effectiveDamage
	takenSkillStat.hits++
	if isCritical {
		takenSkillStat.crits++
		if takenSkillStat.crits == 1 || effectiveDamage < takenSkillStat.critMin {
			takenSkillStat.critMin = effectiveDamage
		}
		if effectiveDamage > takenSkillStat.critMax {
			takenSkillStat.critMax = effectiveDamage
		}
	}
	if effectiveDamage < takenSkillStat.min {
		takenSkillStat.min = effectiveDamage
	}
	if effectiveDamage > takenSkillStat.max {
		takenSkillStat.max = effectiveDamage
	}
	takenSkillStat.records = append(takenSkillStat.records, SkillHitRecord{
		Seq:            record.Seq,
		Damage:         effectiveDamage,
		RawDamage:      damageFloat,
		OverflowDamage: overflowDamage,
		Adjusted:       adjusted,
		IsCritical:     isCritical,
		Timestamp:      now,
	})

	// 标记需要触发目标计时器（锁外执行）
	shouldTriggerTargetTimer = true

	// 记录伤害事件日志
	a.eventLogs = append(a.eventLogs, EventLog{
		Seq:            record.Seq,
		Type:           "damage",
		At:             now,
		EntityID:       attackerIdStr,
		EntityName:     attackerName,
		RaceID:         attackerRaceID,
		RaceName:       attackerRaceName,
		IsPC:           attackerIsPC,
		TargetID:       targetIdStr,
		TargetName:     targetName,
		TargetRaceID:   targetRaceID,
		TargetRaceName: targetRaceName,
		TargetIsPC:     targetIsPC,
		SkillID:        int(skillId),
		SkillName:      a.getSkillNameUnsafe(int(skillId)),
		Damage:         effectiveDamage,
		RawDamage:      damageFloat,
		OverflowDamage: overflowDamage,
		Adjusted:       adjusted,
		IsCritical:     isCritical,
	})

	// 保留最近 500 条日志
	if len(a.eventLogs) > 500 {
		a.eventLogs = a.eventLogs[len(a.eventLogs)-500:]
	}

	// 释放锁
	pendingAdjusted = a.resolvePendingBossHPDamageWindowUnsafe(targetIdStr, a.damageSeq)
	a.mu.Unlock()

	// 锁外操作：触发计时器和发送事件
	if shouldTriggerAttackerTimer {
		a.attackerTimerMgr.OnAttack(attackerIdStr)
	}
	if shouldTriggerTargetTimer {
		a.targetTimerMgr.OnHit(targetIdStr)
	}

	// 发送事件通知前端更新（移到锁外）
	runtime.EventsEmit(a.ctx, "damage", record)
	for _, adjustedRecord := range pendingAdjusted {
		runtime.EventsEmit(a.ctx, "damage", adjustedRecord)
	}
}

// GetDamageByAttacker 获取按攻击者分组的伤害统计
func (a *App) GetDamageByAttacker() []DamageStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]DamageStats, 0)
	now := time.Now().Unix()
	for id, stats := range a.attackerStats {
		dps := 0.0
		// 计算状态：根据最后攻击时间判断
		status := "idle"
		isActive := now-stats.lastHit < 8
		if isActive {
			status = "active"
		}

		// DPS 计算：active 时使用当前时间（实时刷新），idle 时使用 lastHit（固定值）
		if isActive && now > stats.firstHit {
			dps = stats.total / float64(now-stats.firstHit)
		} else if stats.lastHit > stats.firstHit {
			dps = stats.total / float64(stats.lastHit-stats.firstHit)
		}

		percent := 0.0
		if a.totalDamage > 0 {
			percent = (stats.total / a.totalDamage) * 100
		}

		result = append(result, DamageStats{
			ID:          id,
			Name:        a.getEntityNameUnsafe(id),
			TotalDamage: stats.total,
			DPS:         dps,
			Percent:     percent,
			HitCount:    stats.hits,
			CritCount:   stats.crits,
			Status:      status,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalDamage > result[j].TotalDamage
	})

	return result
}

// GetDamageBySkill 获取按技能分组的伤害统计
func (a *App) GetDamageBySkill() []AttackerWithSkills {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]AttackerWithSkills, 0)
	now := time.Now().Unix()
	for attackerId, skillMap := range a.skillStats {
		attackerStat := a.attackerStats[attackerId]
		if attackerStat == nil {
			continue
		}

		dps := 0.0
		// 计算状态：根据最后攻击时间判断
		status := "idle"
		isActive := now-attackerStat.lastHit < 8
		if isActive {
			status = "active"
		}

		// DPS 计算：active 时使用当前时间（实时刷新），idle 时使用 lastHit（固定值）
		if isActive && now > attackerStat.firstHit {
			dps = attackerStat.total / float64(now-attackerStat.firstHit)
		} else if attackerStat.lastHit > attackerStat.firstHit {
			dps = attackerStat.total / float64(attackerStat.lastHit-attackerStat.firstHit)
		}

		percent := 0.0
		if a.totalDamage > 0 {
			percent = (attackerStat.total / a.totalDamage) * 100
		}

		skills := make([]SkillDamageStats, 0)
		for skillId, skillStat := range skillMap {
			skillPercent := 0.0
			if attackerStat.total > 0 {
				skillPercent = (skillStat.total / attackerStat.total) * 100
			}
			avgDamage := 0.0
			if skillStat.hits > 0 {
				avgDamage = skillStat.total / float64(skillStat.hits)
			}

			skills = append(skills, SkillDamageStats{
				SkillID:       skillId,
				SkillName:     a.getSkillNameUnsafe(skillId),
				TotalDamage:   skillStat.total,
				Percent:       skillPercent,
				HitCount:      skillStat.hits,
				CritCount:     skillStat.crits,
				AvgDamage:     avgDamage,
				MinDamage:     skillStat.min,
				MaxDamage:     skillStat.max,
				CritMinDamage: skillStat.critMin,
				CritMaxDamage: skillStat.critMax,
			})
		}

		sort.Slice(skills, func(i, j int) bool {
			return skills[i].TotalDamage > skills[j].TotalDamage
		})

		result = append(result, AttackerWithSkills{
			ID:          attackerId,
			Name:        a.getEntityNameUnsafe(attackerId),
			TotalDamage: attackerStat.total,
			DPS:         dps,
			Percent:     percent,
			Skills:      skills,
			Status:      status,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalDamage > result[j].TotalDamage
	})

	return result
}

// GetDamageTaken 获取受到伤害统计
func (a *App) GetDamageTaken() []TargetDamageStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// 使用聚合统计数据（不受2000条记录限制）
	result := make([]TargetDamageStats, 0)
	now := time.Now().Unix()
	for targetId, targetStat := range a.takenStats {
		attackers := make([]AttackerWithSkills, 0)

		for attackerId, stats := range targetStat.attackers {
			percent := 0.0
			if targetStat.total > 0 {
				percent = (stats.total / targetStat.total) * 100
			}

			// 计算攻击者状态
			attackerIsActive := now-stats.lastHit < 8

			// 计算该攻击者对该目标的DPS
			// 根据攻击者状态和目标状态决定使用哪个时间点
			var endTime int64
			if attackerIsActive {
				// 攻击者 active：如果目标已死亡使用死亡时间，否则使用当前时间
				endTime = now
				if targetStat.deathTime > 0 {
					endTime = targetStat.deathTime
				}
			} else {
				// 攻击者 idle：使用攻击者最后攻击时间（固定值）
				endTime = stats.lastHit
			}

			attackerDuration := endTime - targetStat.firstHit
			if attackerDuration < 1 {
				attackerDuration = 1
			}
			attackerDps := stats.total / float64(attackerDuration)

			// 构建技能统计列表
			skills := make([]SkillDamageStats, 0)
			for skillId, skillStat := range stats.skills {
				skillPercent := 0.0
				if stats.total > 0 {
					skillPercent = (skillStat.total / stats.total) * 100
				}
				avgDamage := 0.0
				if skillStat.hits > 0 {
					avgDamage = skillStat.total / float64(skillStat.hits)
				}

				skills = append(skills, SkillDamageStats{
					SkillID:       skillId,
					SkillName:     a.getSkillNameUnsafe(skillId),
					TotalDamage:   skillStat.total,
					Percent:       skillPercent,
					HitCount:      skillStat.hits,
					CritCount:     skillStat.crits,
					AvgDamage:     avgDamage,
					MinDamage:     skillStat.min,
					MaxDamage:     skillStat.max,
					CritMinDamage: skillStat.critMin,
					CritMaxDamage: skillStat.critMax,
				})
			}

			// 按伤害排序技能
			sort.Slice(skills, func(i, j int) bool {
				// 主排序：按伤害降序
				if skills[i].TotalDamage != skills[j].TotalDamage {
					return skills[i].TotalDamage > skills[j].TotalDamage
				}
				// 次排序：按技能ID排序（保证稳定性）
				return skills[i].SkillID < skills[j].SkillID
			})

			// 计算该攻击者的状态
			attackerStatus := "idle"
			if now-stats.lastHit < 8 {
				attackerStatus = "active"
			}

			attackers = append(attackers, AttackerWithSkills{
				ID:          attackerId,
				Name:        formatDisplayName(attackerId, stats.name, stats.raceId, stats.isPC),
				TotalDamage: stats.total,
				DPS:         attackerDps,
				Percent:     percent,
				Skills:      skills,
				Status:      attackerStatus,
			})
		}

		sort.Slice(attackers, func(i, j int) bool {
			// 主排序：按伤害降序
			if attackers[i].TotalDamage != attackers[j].TotalDamage {
				return attackers[i].TotalDamage > attackers[j].TotalDamage
			}
			// 次排序：按ID排序（保证稳定性）
			return attackers[i].ID < attackers[j].ID
		})

		// 计算状态：根据死亡时间和最后受击时间判断
		status := "idle"
		targetIsActive := false
		if targetStat.deathTime > 0 {
			status = "dead"
		} else if now-targetStat.lastHit < 8 {
			status = "active"
			targetIsActive = true
		}

		// 计算DPS和存活时间
		// 根据目标状态决定使用哪个时间点
		var endTime int64
		if status == "dead" {
			// 已死亡：使用死亡时间（固定值）
			endTime = targetStat.deathTime
		} else if targetIsActive {
			// active：使用当前时间（实时刷新）
			endTime = now
		} else {
			// idle：使用最后受击时间（固定值）
			endTime = targetStat.lastHit
		}

		duration := endTime - targetStat.firstHit
		if duration < 1 {
			duration = 1 // 至少1秒，避免除零
		}
		dps := targetStat.total / float64(duration)

		result = append(result, TargetDamageStats{
			ID:          targetId,
			Name:        formatDisplayName(targetId, targetStat.name, targetStat.raceId, targetStat.isPC),
			TotalDamage: targetStat.total,
			DPS:         dps,
			Duration:    duration,
			Attackers:   attackers,
			Status:      status,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		// 主排序：按伤害降序
		if result[i].TotalDamage != result[j].TotalDamage {
			return result[i].TotalDamage > result[j].TotalDamage
		}
		// 次排序：按ID排序（保证稳定性）
		return result[i].ID < result[j].ID
	})

	return result
}

// updateChartAggData 更新图表聚合数据
// 按角色名称聚合（而非实体ID），这样同一个玩家在不同情况下（如进塔）会合并统计
// 注意：调用此方法前必须持有写锁
func (a *App) updateChartAggData(attackerName string, timestamp int64, damage float64) {
	// 使用角色名称作为 key（而非实体ID）
	// 这样同一个玩家即使有多个实体ID也会被合并
	chartData := a.chartAggData[attackerName]
	if chartData == nil {
		chartData = &chartAttackerData{
			buckets: make(map[int64]float64),
			times:   make([]int64, 0),
		}
		a.chartAggData[attackerName] = chartData
	}

	// 计算时间桶（对齐到30秒）
	bucket := timestamp - (timestamp % ChartBucketSeconds)

	// 如果是新的时间桶，添加到时间列表
	if _, exists := chartData.buckets[bucket]; !exists {
		chartData.times = append(chartData.times, bucket)
	}

	// 累加伤害到对应时间桶
	chartData.buckets[bucket] += damage

	// 清理过期数据（超过2小时的）
	cutoff := timestamp - ChartMaxDurationSecs
	newTimes := make([]int64, 0, len(chartData.times))
	for _, t := range chartData.times {
		if t >= cutoff {
			newTimes = append(newTimes, t)
		} else {
			delete(chartData.buckets, t)
		}
	}
	chartData.times = newTimes
}

// GetChartData 获取图表数据
// 从聚合数据中读取，每30秒一个数据点，最多显示2小时（240个点）
// 只返回已完成的时间桶，排除当前正在进行的时间桶，避免数据跳动
// 数据按角色名称聚合，同一玩家的不同实体ID会被合并
func (a *App) GetChartData() []ChartSeries {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// 计算当前正在进行的时间桶（需要排除）
	now := time.Now().Unix()
	currentBucket := now - (now % ChartBucketSeconds)

	result := make([]ChartSeries, 0)

	for attackerName, chartData := range a.chartAggData {
		if chartData == nil || len(chartData.times) == 0 {
			continue
		}

		// 收集并排序时间点
		times := make([]int64, len(chartData.times))
		copy(times, chartData.times)
		sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })

		// 计算每30秒的DPS（伤害/30秒 = 每秒伤害）
		// 只包含已完成的时间桶（排除当前正在进行的）
		data := make([]ChartDataPoint, 0, len(times))
		for _, t := range times {
			// 跳过当前正在进行的时间桶
			if t >= currentBucket {
				continue
			}
			damage := chartData.buckets[t]
			// 每30秒的伤害量除以30得到每秒伤害（DPS）
			dps := damage / float64(ChartBucketSeconds)
			data = append(data, ChartDataPoint{
				Time:   t * 1000, // 转换为毫秒给前端
				Damage: dps,
			})
		}

		// 只有有数据的才添加到结果
		// 现在 key 就是角色名称，ID 和 Name 都使用它
		if len(data) > 0 {
			result = append(result, ChartSeries{
				ID:   attackerName,
				Name: attackerName,
				Data: data,
			})
		}
	}

	// 按平均DPS排序（计算所有时间段的平均DPS）
	sort.Slice(result, func(i, j int) bool {
		iAvg := 0.0
		jAvg := 0.0
		if len(result[i].Data) > 0 {
			sum := 0.0
			for _, point := range result[i].Data {
				sum += point.Damage
			}
			iAvg = sum / float64(len(result[i].Data))
		}
		if len(result[j].Data) > 0 {
			sum := 0.0
			for _, point := range result[j].Data {
				sum += point.Damage
			}
			jAvg = sum / float64(len(result[j].Data))
		}
		return iAvg > jAvg
	})

	return result
}

// updateTargetChartAggData 更新按目标分组的图表聚合数据
// 用于怪物跟踪模式下显示针对特定怪物的DPS曲线
// 注意：调用此方法前必须持有写锁
func (a *App) updateTargetChartAggData(targetId string, attackerName string, timestamp int64, damage float64) {
	targetChart := a.targetChartAggData[targetId]
	if targetChart == nil {
		targetChart = make(map[string]*chartAttackerData)
		a.targetChartAggData[targetId] = targetChart
	}

	chartData := targetChart[attackerName]
	if chartData == nil {
		chartData = &chartAttackerData{
			buckets: make(map[int64]float64),
			times:   make([]int64, 0),
		}
		targetChart[attackerName] = chartData
	}

	// 计算时间桶（对齐到30秒）
	bucket := timestamp - (timestamp % ChartBucketSeconds)

	// 如果是新的时间桶，添加到时间列表
	if _, exists := chartData.buckets[bucket]; !exists {
		chartData.times = append(chartData.times, bucket)
	}

	// 累加伤害到对应时间桶
	chartData.buckets[bucket] += damage

	// 清理过期数据（超过2小时的）
	cutoff := timestamp - ChartMaxDurationSecs
	newTimes := make([]int64, 0, len(chartData.times))
	for _, t := range chartData.times {
		if t >= cutoff {
			newTimes = append(newTimes, t)
		} else {
			delete(chartData.buckets, t)
		}
	}
	chartData.times = newTimes
}

// GetChartDataForTarget 获取针对特定目标的图表数据
// 与 GetChartData 类似，但只返回对指定目标造成伤害的DPS曲线
func (a *App) GetChartDataForTarget(targetId string) []ChartSeries {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// 计算当前正在进行的时间桶（需要排除）
	now := time.Now().Unix()
	currentBucket := now - (now % ChartBucketSeconds)

	result := make([]ChartSeries, 0)

	targetChart := a.targetChartAggData[targetId]
	if targetChart == nil {
		return result
	}

	for attackerName, chartData := range targetChart {
		if chartData == nil || len(chartData.times) == 0 {
			continue
		}

		// 收集并排序时间点
		times := make([]int64, len(chartData.times))
		copy(times, chartData.times)
		sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })

		// 计算每30秒的DPS（伤害/30秒 = 每秒伤害）
		// 只包含已完成的时间桶（排除当前正在进行的）
		data := make([]ChartDataPoint, 0, len(times))
		for _, t := range times {
			if t >= currentBucket {
				continue
			}
			damage := chartData.buckets[t]
			dps := damage / float64(ChartBucketSeconds)
			data = append(data, ChartDataPoint{
				Time:   t * 1000, // 转换为毫秒给前端
				Damage: dps,
			})
		}

		if len(data) > 0 {
			result = append(result, ChartSeries{
				ID:   attackerName,
				Name: attackerName,
				Data: data,
			})
		}
	}

	// 按平均DPS排序
	sort.Slice(result, func(i, j int) bool {
		iAvg := 0.0
		jAvg := 0.0
		if len(result[i].Data) > 0 {
			sum := 0.0
			for _, point := range result[i].Data {
				sum += point.Damage
			}
			iAvg = sum / float64(len(result[i].Data))
		}
		if len(result[j].Data) > 0 {
			sum := 0.0
			for _, point := range result[j].Data {
				sum += point.Damage
			}
			jAvg = sum / float64(len(result[j].Data))
		}
		return iAvg > jAvg
	})

	return result
}

// HasActivePlayer 检查是否有活跃玩家（最近8秒内有攻击）
// 用于前端判断是否需要更新图表
func (a *App) HasActivePlayer() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	now := time.Now().Unix()
	for _, stats := range a.attackerStats {
		if now-stats.lastHit < 8 {
			return true
		}
	}
	return false
}

// GetRecentDamages 获取最近伤害记录
func (a *App) GetRecentDamages(limit int) []DamageRecord {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if len(a.damages) <= limit {
		result := make([]DamageRecord, len(a.damages))
		copy(result, a.damages)
		return result
	}
	result := make([]DamageRecord, limit)
	copy(result, a.damages[len(a.damages)-limit:])
	return result
}

// -.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.
// 获取ID后缀用于区分同种族的不同个体
func getIdSuffix(id string) string {
	if len(id) > 4 {
		return id[len(id)-4:]
	}
	return id
}

// 根据缓存信息生成显示名称
// PC: 使用名字
// 怪物: 使用种族名 + ID后缀（从数据库查询）
func formatDisplayName(id, name string, raceId int, isPC bool) string {
	if isPC && name != "" {
		return name
	} else {
		// 非PC优先从数据库获取种族名
		race_info := db.NewRace(raceId)
		raceName := race_info.GetName()
		if raceName != "" {
			return raceName + " #" + getIdSuffix(id)
		}
		if name != "" {
			return name + " #" + getIdSuffix(id)
		}
	}

	// 最后返回ID的后6位
	if len(id) > 6 {
		return id[len(id)-6:]
	}
	return id
}
