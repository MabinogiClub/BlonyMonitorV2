package app

import (
	"math"
	"strconv"
	"strings"
	"time"

	"blonymonitorv2/internal/packet"
)

const (
	battlefieldShockStatusID      uint32 = 0xffff0001
	battlefieldShockIconID        uint32 = 1160
	battlefieldShockStrengthField        = "MAX_DAMAGE_PERCENT"
	battlefieldShockRefresh              = 10 * time.Second
	battlefieldShockDuration             = 20 * time.Second
	battlefieldShockMatchMinDelay        = 8 * time.Second
	battlefieldShockMatchMaxDelay        = 13 * time.Second
	musicPerformanceHeader        uint32 = 45528
)

type musicPerformance struct {
	InstanceID uint64
	ActorID    uint64
	StartedAt  int64
}

type battlefieldShockState struct {
	TargetID             uint64
	PerformanceID        uint64
	PerformanceStartedAt int64
	NextRefreshAt        int64
	ExpiresAt            int64
	Generation           uint64
}

type battlefieldShockNotice struct {
	Performer string
	Strength  float64
	Raw       string
}

func durationCentiseconds(duration time.Duration) int64 {
	return duration.Milliseconds() / 10
}

func packetCentiseconds(pkt *packet.GamePacket) int64 {
	if pkt != nil && !pkt.At.IsZero() {
		return pkt.At.UnixMilli() / 10
	}
	return nowCentiseconds()
}

func parseMusicPerformanceStart(pkt *packet.GamePacket) (*musicPerformance, bool) {
	if pkt == nil || len(pkt.Msg) != 33 {
		return nil, false
	}
	if pkt.Msg[0].Type() != packet.MessageElemTypeLong ||
		pkt.Msg[1].Type() != packet.MessageElemTypeInt ||
		pkt.Msg[5].Type() != packet.MessageElemTypeString ||
		pkt.Msg[11].Type() != packet.MessageElemTypeString ||
		pkt.Msg[27].Type() != packet.MessageElemTypeLong ||
		pkt.Msg[32].Type() != packet.MessageElemTypeInt {
		return nil, false
	}
	if pkt.Msg[1].Data().(uint32) != musicPerformanceHeader || pkt.Msg[5].Data().(string) != "single" {
		return nil, false
	}
	instanceID := pkt.Msg[0].Data().(uint64)
	actorID := pkt.Msg[27].Data().(uint64)
	if instanceID == 0 || actorID == 0 || pkt.Msg[11].Data().(string) == "" {
		return nil, false
	}
	return &musicPerformance{
		InstanceID: instanceID,
		ActorID:    actorID,
		StartedAt:  packetCentiseconds(pkt),
	}, true
}

func parseMusicPerformanceStop(pkt *packet.GamePacket) (uint64, bool) {
	if pkt == nil || len(pkt.Msg) != 1 || pkt.Msg[0].Type() != packet.MessageElemTypeLong {
		return 0, false
	}
	instanceID := pkt.Msg[0].Data().(uint64)
	return instanceID, instanceID != 0
}

func parseBattlefieldShockNotice(pkt *packet.GamePacket) (battlefieldShockNotice, bool) {
	if pkt == nil || len(pkt.Msg) < 2 ||
		pkt.Msg[0].Type() != packet.MessageElemTypeByte ||
		pkt.Msg[1].Type() != packet.MessageElemTypeString ||
		pkt.Msg[0].Data().(uint8) != 4 {
		return battlefieldShockNotice{}, false
	}

	text := pkt.Msg[1].Data().(string)
	const performanceMarker = "演奏的“战场的震慑”"
	markerAt := strings.Index(text, performanceMarker)
	if markerAt <= 0 {
		return battlefieldShockNotice{}, false
	}

	strength := 35.0
	const strengthPrefix = "最大攻击力增加"
	if start := strings.Index(text, strengthPrefix); start >= 0 {
		start += len(strengthPrefix)
		if end := strings.Index(text[start:], "%"); end > 0 {
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(text[start:start+end]), 64); err == nil {
				strength = parsed
			}
		}
	}

	return battlefieldShockNotice{
		Performer: strings.TrimSpace(text[:markerAt]),
		Strength:  strength,
		Raw:       text,
	}, true
}

func (a *App) handleMusicPerformanceStart(pkt *packet.GamePacket) {
	performance, ok := parseMusicPerformanceStart(pkt)
	if !ok {
		return
	}
	a.mu.Lock()
	if a.musicPerformances == nil {
		a.musicPerformances = make(map[uint64]*musicPerformance)
	}
	a.musicPerformances[performance.InstanceID] = performance
	a.mu.Unlock()
}

func (a *App) handleMusicPerformanceStop(pkt *packet.GamePacket) {
	instanceID, ok := parseMusicPerformanceStop(pkt)
	if !ok {
		return
	}
	a.mu.Lock()
	delete(a.musicPerformances, instanceID)
	a.mu.Unlock()
}

func (a *App) findShockPerformanceUnsafe(performer string, at int64) *musicPerformance {
	minDelay := durationCentiseconds(battlefieldShockMatchMinDelay)
	maxDelay := durationCentiseconds(battlefieldShockMatchMaxDelay)
	wantDelay := durationCentiseconds(battlefieldShockRefresh)

	var namedMatch *musicPerformance
	var onlyCandidate *musicPerformance
	candidateCount := 0
	bestDistance := int64(math.MaxInt64)
	for _, performance := range a.musicPerformances {
		delay := at - performance.StartedAt
		if delay < minDelay || delay > maxDelay {
			continue
		}
		candidateCount++
		onlyCandidate = performance
		if performer == "" || a.getEntityNameUnsafe(strconv.FormatUint(performance.ActorID, 10)) != performer {
			continue
		}
		distance := delay - wantDelay
		if distance < 0 {
			distance = -distance
		}
		if distance < bestDistance {
			bestDistance = distance
			namedMatch = performance
		}
	}
	if namedMatch != nil {
		return namedMatch
	}
	if candidateCount == 1 {
		return onlyCandidate
	}
	return nil
}

func (a *App) activateBattlefieldShockUnsafe(targetID uint64, notice battlefieldShockNotice, at int64) (*battlefieldShockState, int64) {
	if a.battlefieldShockStates == nil {
		a.battlefieldShockStates = make(map[string]*battlefieldShockState)
	}
	a.battlefieldShockGeneration++
	state := &battlefieldShockState{
		TargetID:      targetID,
		NextRefreshAt: at + durationCentiseconds(battlefieldShockRefresh),
		ExpiresAt:     at + durationCentiseconds(battlefieldShockDuration),
		Generation:    a.battlefieldShockGeneration,
	}
	if performance := a.findShockPerformanceUnsafe(notice.Performer, at); performance != nil {
		state.PerformanceID = performance.InstanceID
		state.PerformanceStartedAt = performance.StartedAt
	}

	details := map[string]packet.ConditionDetailValue{
		battlefieldShockStrengthField: {Type: "f", Value: strconv.FormatFloat(notice.Strength, 'f', -1, 64)},
	}
	a.recordStatusConditionUnsafe(targetID, battlefieldShockStatusID, true, notice.Raw, details, at)
	a.battlefieldShockStates[strconv.FormatUint(targetID, 10)] = state
	return state, state.NextRefreshAt
}

func (a *App) isShockPerformanceActiveUnsafe(state *battlefieldShockState) bool {
	if state == nil || state.PerformanceID == 0 {
		return false
	}
	performance := a.musicPerformances[state.PerformanceID]
	return performance != nil && performance.StartedAt == state.PerformanceStartedAt
}

func (a *App) advanceBattlefieldShockUnsafe(targetID string, generation uint64, at int64) int64 {
	state := a.battlefieldShockStates[targetID]
	if state == nil || state.Generation != generation {
		return 0
	}

	if a.isShockPerformanceActiveUnsafe(state) {
		refresh := durationCentiseconds(battlefieldShockRefresh)
		duration := durationCentiseconds(battlefieldShockDuration)
		for state.NextRefreshAt <= at {
			state.ExpiresAt = state.NextRefreshAt + duration
			state.NextRefreshAt += refresh
		}
		return state.NextRefreshAt
	}
	if at < state.ExpiresAt {
		return state.ExpiresAt
	}

	a.recordStatusConditionUnsafe(state.TargetID, battlefieldShockStatusID, false, "", nil, state.ExpiresAt)
	delete(a.battlefieldShockStates, targetID)
	return 0
}

func (a *App) scheduleBattlefieldShockCheck(targetID string, generation uint64, checkAt int64) {
	if checkAt == 0 {
		return
	}
	delay := time.Until(time.UnixMilli(checkAt * 10))
	if delay < 0 {
		delay = 0
	}
	time.AfterFunc(delay, func() {
		a.mu.Lock()
		nextCheck := a.advanceBattlefieldShockUnsafe(targetID, generation, nowCentiseconds())
		a.mu.Unlock()
		if nextCheck != 0 {
			a.scheduleBattlefieldShockCheck(targetID, generation, nextCheck)
		}
	})
}

func (a *App) handleBattlefieldShockNotice(pkt *packet.GamePacket) bool {
	notice, ok := parseBattlefieldShockNotice(pkt)
	if !ok {
		return false
	}
	at := packetCentiseconds(pkt)
	targetIDString := strconv.FormatUint(pkt.Id, 10)
	a.mu.Lock()
	state, nextCheck := a.activateBattlefieldShockUnsafe(pkt.Id, notice, at)
	a.mu.Unlock()
	a.scheduleBattlefieldShockCheck(targetIDString, state.Generation, nextCheck)
	return true
}
