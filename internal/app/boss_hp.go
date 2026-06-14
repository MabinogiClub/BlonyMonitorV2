package app

import (
	"math"
	"strconv"
	"time"

	"blonymonitorv2/internal/packet"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	bossHPLockBandPercent        = 0.2
	bossHPLockStepPercent        = 5.0
	bossHPLockConfirmTicks       = 20
	bossDamageAdjustEpsilon      = 1.0
	bossHPLockMinMaxHP           = 100000.0
	bossHPLockMinOverflow        = 1000.0
	bossHPLockMinOverflowPercent = 0.01
)

type bossDamageOverflowAdjustResult struct {
	Records       []DamageRecord
	Overflow      float64
	Trigger       DamageRecord
	TriggerSeq    int64
	TriggerFound  bool
	LockThreshold float64
}

func (a *App) handleEntityProperty(pkt *packet.GamePacket) {
	if pkt == nil || len(pkt.Msg) < 4 {
		return
	}

	props := parseEntityProperties(pkt.Msg)
	currentHP, hasCurrent := props[28]
	if !hasCurrent {
		return
	}

	maxHP := 0.0
	if v, ok := props[29]; ok {
		maxHP = v
	}
	if v, ok := props[30]; ok && v > maxHP {
		maxHP = v
	}
	if maxHP <= 0 {
		return
	}

	if currentHP < 0 {
		currentHP = 0
	}
	if currentHP > maxHP {
		currentHP = maxHP
	}

	id := strconv.FormatUint(pkt.Id, 10)
	now := time.Now().UnixMilli() / 10

	a.mu.Lock()
	raceID := a.getEntityRaceIDUnsafe(id)
	if raceID >= 0 && isPC(raceID) {
		a.mu.Unlock()
		return
	}

	name := a.getEntityNameUnsafe(id)
	percent := (currentHP / maxHP) * 100
	prevHP := currentHP
	prevPercent := percent
	currentDamageSeq := a.damageSeq
	prevDamageSeq := currentDamageSeq
	hasPrev := false
	if prev := a.bossHP[id]; prev != nil {
		prevHP = prev.CurrentHP
		prevPercent = prev.Percent
		prevDamageSeq = prev.DamageSeq
		hasPrev = true
	}

	a.bossHP[id] = &BossHPInfo{
		EntityID:  id,
		Name:      name,
		RaceID:    raceID,
		CurrentHP: currentHP,
		MaxHP:     maxHP,
		Percent:   percent,
		UpdatedAt: now,
		DamageSeq: currentDamageSeq,
	}

	a.bossHPHistory[id] = append(a.bossHPHistory[id], BossHPRecord{
		EntityID:    id,
		RaceID:      raceID,
		CurrentHP:   currentHP,
		MaxHP:       maxHP,
		Percent:     percent,
		HpTimestamp: now,
		DamageSeq:   currentDamageSeq,
	})

	var adjusted []DamageRecord
	var overflowResult bossDamageOverflowAdjustResult
	consumeDamageWindow := !hasPrev
	if hasPrev {
		hpDelta := prevHP - currentHP
		lockThreshold := getPotentialBossHPLockThreshold(currentHP, percent, prevPercent)
		if lockThreshold > 0 {
			a.setBossHPLockCandidateUnsafe(id, lockThreshold, now)
		}
		adjustThreshold := lockThreshold
		markLockTrigger := lockThreshold > 0
		isPlatformContinuation := false
		if hpDelta <= bossDamageAdjustEpsilon && currentDamageSeq > prevDamageSeq {
			if activeThreshold := a.getActiveBossHPLockThresholdUnsafe(id, currentHP, percent); activeThreshold > 0 {
				adjustThreshold = activeThreshold
				markLockTrigger = false
				isPlatformContinuation = true
			} else if candidateThreshold := a.getConfirmedBossHPLockCandidateUnsafe(id, currentHP, percent, now); candidateThreshold > 0 {
				a.activateBossHPLockPlatformUnsafe(id, candidateThreshold)
				adjustThreshold = candidateThreshold
				markLockTrigger = false
				isPlatformContinuation = true
			}
		}
		if shouldAdjustBossDamageForHPChange(prevHP, currentHP, adjustThreshold, isPlatformContinuation) {
			if hpDelta > bossDamageAdjustEpsilon && currentDamageSeq <= prevDamageSeq {
				a.setPendingBossHPDamageWindowUnsafe(id, BossHPPendingDamageWindow{
					FromSeq:         prevDamageSeq,
					HPDelta:         hpDelta,
					LockThreshold:   adjustThreshold,
					MarkLockTrigger: markLockTrigger,
					MaxHP:           maxHP,
					CurrentHP:       currentHP,
					CurrentPercent:  percent,
					PrevHP:          prevHP,
					PrevPercent:     prevPercent,
					Timestamp:       now,
				})
				consumeDamageWindow = false
			} else {
				overflowResult = a.adjustBossDamageOverflowUnsafe(id, prevDamageSeq, currentDamageSeq, hpDelta, adjustThreshold, markLockTrigger, maxHP)
				adjusted = append(adjusted, overflowResult.Records...)
				consumeDamageWindow = true
				delete(a.bossHPPending, id)
			}
		} else if hpDelta < -bossDamageAdjustEpsilon {
			consumeDamageWindow = true
			delete(a.bossHPPending, id)
		} else if hpDelta <= bossDamageAdjustEpsilon && currentDamageSeq > prevDamageSeq && !isPlatformContinuation {
			consumeDamageWindow = true
			delete(a.bossHPPending, id)
		}
		if currentHP <= 0 {
			adjusted = append(adjusted, a.capDeadTargetDamageToMaxHPUnsafe(id)...)
			consumeDamageWindow = true
		}
	}
	if !consumeDamageWindow {
		a.bossHP[id].DamageSeq = prevDamageSeq
	}
	lockThreshold, locked := a.markBossHPLockUnsafe(id, name, raceID, currentHP, maxHP, percent, prevHP, prevPercent, now, overflowResult)
	if locked {
		history := a.bossHPHistory[id]
		if len(history) > 0 {
			history[len(history)-1].Threshold = lockThreshold
			history[len(history)-1].Locked = true
			a.bossHPHistory[id] = history
		}
	}
	a.clearInactiveBossHPLockPlatformUnsafe(id, currentHP, percent)
	a.mu.Unlock()

	for _, record := range adjusted {
		runtime.EventsEmit(a.ctx, "damage", record)
	}
}

func (a *App) markBossHPLockUnsafe(id, name string, raceID int, currentHP, maxHP, percent, prevHP, prevPercent float64, now int64, overflowResult bossDamageOverflowAdjustResult) (float64, bool) {
	if !overflowResult.TriggerFound || overflowResult.LockThreshold <= 0 {
		return 0, false
	}
	state := a.getBossHPWatchStateUnsafe(id)
	threshold := overflowResult.LockThreshold
	if state.Emitted[threshold] {
		return 0, false
	}
	trigger := overflowResult.Trigger
	a.eventLogs = append(a.eventLogs, EventLog{
		Seq:            trigger.Seq,
		Type:           "boss_hp",
		At:             now,
		EntityID:       id,
		EntityName:     name,
		RaceID:         raceID,
		RaceName:       a.getRaceNameUnsafe(raceID),
		AttackerID:     trigger.AttackerID,
		AttackerName:   trigger.AttackerName,
		SkillID:        trigger.SkillID,
		SkillName:      a.getSkillNameUnsafe(trigger.SkillID),
		Damage:         trigger.Damage,
		RawDamage:      trigger.RawDamage,
		OverflowDamage: trigger.OverflowDamage,
		Adjusted:       trigger.Adjusted,
		LockTriggered:  true,
		LockThreshold:  threshold,
		CurrentHP:      currentHP,
		MaxHP:          maxHP,
		Percent:        percent,
		PrevHP:         prevHP,
		PrevPercent:    prevPercent,
		Threshold:      threshold,
		Locked:         true,
	})
	if len(a.eventLogs) > 500 {
		a.eventLogs = a.eventLogs[len(a.eventLogs)-500:]
	}
	state.Emitted[threshold] = true
	return threshold, true
}

func (a *App) getBossHPWatchStateUnsafe(id string) *BossHPWatchState {
	state := a.bossHPWatch[id]
	if state == nil {
		state = &BossHPWatchState{Emitted: make(map[float64]bool)}
		a.bossHPWatch[id] = state
	}
	if state.Emitted == nil {
		state.Emitted = make(map[float64]bool)
	}
	return state
}

func (a *App) setBossHPLockCandidateUnsafe(id string, threshold float64, now int64) {
	state := a.getBossHPWatchStateUnsafe(id)
	if state.ActiveThreshold == threshold || state.CandidateThreshold == threshold {
		return
	}
	state.CandidateThreshold = threshold
	state.CandidateAt = now
}

func (a *App) getActiveBossHPLockThresholdUnsafe(id string, currentHP, percent float64) float64 {
	state := a.bossHPWatch[id]
	if state == nil || state.ActiveThreshold <= 0 || currentHP <= 0 {
		return 0
	}
	if math.Abs(percent-state.ActiveThreshold) > bossHPLockBandPercent {
		return 0
	}
	return state.ActiveThreshold
}

func (a *App) getConfirmedBossHPLockCandidateUnsafe(id string, currentHP, percent float64, now int64) float64 {
	state := a.bossHPWatch[id]
	if state == nil || state.CandidateThreshold <= 0 || currentHP <= 0 {
		return 0
	}
	if math.Abs(percent-state.CandidateThreshold) > bossHPLockBandPercent {
		return 0
	}
	if now-state.CandidateAt < bossHPLockConfirmTicks {
		return 0
	}
	return state.CandidateThreshold
}

func (a *App) activateBossHPLockPlatformUnsafe(id string, threshold float64) {
	state := a.getBossHPWatchStateUnsafe(id)
	state.ActiveThreshold = threshold
	state.CandidateThreshold = 0
	state.CandidateAt = 0
}

func (a *App) clearInactiveBossHPLockPlatformUnsafe(id string, currentHP, percent float64) {
	state := a.bossHPWatch[id]
	if state == nil {
		return
	}
	if state.ActiveThreshold > 0 && !isBossHPWithinLockBand(currentHP, percent, state.ActiveThreshold) {
		state.ActiveThreshold = 0
	}
	if state.CandidateThreshold > 0 && !isBossHPWithinLockBand(currentHP, percent, state.CandidateThreshold) {
		state.CandidateThreshold = 0
		state.CandidateAt = 0
	}
}

func isBossHPWithinLockBand(currentHP, percent, threshold float64) bool {
	if currentHP <= 0 {
		return false
	}
	return percent >= threshold-bossHPLockBandPercent && percent <= threshold+bossHPLockBandPercent
}

func getPotentialBossHPLockThreshold(currentHP, percent, prevPercent float64) float64 {
	threshold, ok := getBossHPLockThreshold(percent)
	if !ok || currentHP <= 0 || prevPercent <= threshold+bossHPLockBandPercent || percent >= prevPercent {
		return 0
	}
	return threshold
}

func getBossHPLockThreshold(percent float64) (float64, bool) {
	threshold := math.Round(percent/bossHPLockStepPercent) * bossHPLockStepPercent
	if threshold <= 0 || threshold >= 100 {
		return 0, false
	}
	if math.Abs(percent-threshold) > bossHPLockBandPercent {
		return 0, false
	}
	return threshold, true
}

func shouldAdjustBossDamageForHPChange(prevHP, currentHP, lockThreshold float64, platformContinuation bool) bool {
	hpDelta := prevHP - currentHP
	if hpDelta < -bossDamageAdjustEpsilon {
		return false
	}
	if hpDelta > bossDamageAdjustEpsilon {
		return true
	}
	return currentHP > bossDamageAdjustEpsilon && lockThreshold > 0 && platformContinuation
}

func parseEntityProperties(msg packet.Message) map[uint32]float64 {
	props := make(map[uint32]float64)
	if len(msg) < 4 {
		return props
	}
	start := 0
	if msg[0].Type() == packet.MessageElemTypeByte && msg[1].Type() == packet.MessageElemTypeInt {
		start = 2
	}
	for i := start; i+1 < len(msg); i += 2 {
		if msg[i].Type() != packet.MessageElemTypeInt || msg[i+1].Type() != packet.MessageElemTypeFloat {
			continue
		}
		key := msg[i].Data().(uint32)
		value := float64(msg[i+1].Data().(float32))
		props[key] = value
	}
	return props
}

func (a *App) clearBossHP(entityID string) {
	a.mu.Lock()
	delete(a.bossHP, entityID)
	delete(a.bossHPWatch, entityID)
	delete(a.bossHPPending, entityID)
	a.mu.Unlock()
}

func (a *App) clearAllBossHPUnsafe() {
	a.bossHP = make(map[string]*BossHPInfo)
	a.bossHPHistory = make(map[string][]BossHPRecord)
	a.bossHPWatch = make(map[string]*BossHPWatchState)
	a.bossHPPending = make(map[string]*BossHPPendingDamageWindow)
}

func (a *App) GetBossHP() []*BossHPInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]*BossHPInfo, 0, len(a.bossHP))
	for _, hp := range a.bossHP {
		if hp == nil || hp.MaxHP <= 0 {
			continue
		}
		copyHP := *hp
		result = append(result, &copyHP)
	}
	return result
}
