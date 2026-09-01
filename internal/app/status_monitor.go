package app

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"blonymonitorv2/internal/packet"
)

var monitoredStatusOrder = []uint32{515, 516, 914, 915, 680, 192, 193, 681, 194, 1225, 63, 1121, 1150, 1023, 1033, 1035, 1036, 1037, battlefieldShockStatusID}

var targetMonitoredStatusOrder = []uint32{1164, 1165, 1166, 211, 392, 504, 1012, 1004, 912, 913, 803, 464, 465, 1063, 1138, 1026, 10001, 10002}

var monitoredStatusNames = map[uint32]string{
	515:                      "状态支援",
	516:                      "觉醒",
	914:                      "喵咪的恩赐（物理）",
	915:                      "喵咪的恩赐（魔法）",
	680:                      "战争序曲",
	192:                      "活跃进行曲",
	193:                      "行进曲",
	681:                      "忍耐之歌",
	194:                      "丰收之歌",
	1225:                     "超燃咚咚",
	63:                       "攻击力增加",
	1121:                     "魔法攻击强化",
	1150:                     "炼金术伤害增加",
	1023:                     "月亮",
	1033:                     "星星",
	1035:                     "日",
	1036:                     "月",
	1037:                     "月蚀",
	battlefieldShockStatusID: "战场的震慑",
}

var targetMonitoredStatusNames = map[uint32]string{
	1164:  "防御/保护减少",
	1165:  "魔法防御/魔法保护减少",
	1166:  "增加受到的伤害",
	211:   "骨龙 恶魔的疾走",
	392:   "闪电风暴",
	504:   "雷霆咆哮",
	1012:  "防御，保护减少",
	1004:  "锐利之眼",
	912:   "喵咪的奔袭",
	913:   "喵咪的奔袭",
	803:   "崩坏波动",
	464:   "结冰状态",
	465:   "结冰状态",
	1063:  "倒塌的屏障",
	1138:  "三叶草印记",
	1026:  "倒吊者",
	10001: "减少物理防御和保护的魔法阵",
	10002: "减少魔法防御和保护魔法阵",
}

var statusStrengthFields = map[uint32]string{
	680:                      "MCMBAMAX",
	192:                      "LSMA",
	193:                      "SPDPC",
	battlefieldShockStatusID: battlefieldShockStrengthField,
}

var statusIconIDs = map[uint32]uint32{
	battlefieldShockStatusID: battlefieldShockIconID,
}

var channelPersistentStatusIDs = map[uint32]struct{}{
	63:   {},
	1121: {},
	1150: {},
}

func isChannelPersistentStatus(conditionID uint32) bool {
	_, ok := channelPersistentStatusIDs[conditionID]
	return ok
}

type BuffDetailValue struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type BuffCoverageSegment struct {
	StartedAt     int64                      `json:"startedAt"`
	EndedAt       int64                      `json:"endedAt"`
	StartOffset   float64                    `json:"startOffset"`
	EndOffset     float64                    `json:"endOffset"`
	ActiveSeconds float64                    `json:"activeSeconds"`
	Strength      *float64                   `json:"strength,omitempty"`
	RawDetail     string                     `json:"rawDetail,omitempty"`
	Details       map[string]BuffDetailValue `json:"details,omitempty"`
}

type BuffCoverage struct {
	ConditionID     uint32                `json:"conditionId"`
	ConditionName   string                `json:"conditionName"`
	IconID          uint32                `json:"iconId,omitempty"`
	ActiveSeconds   float64               `json:"activeSeconds"`
	CoveragePercent float64               `json:"coveragePercent"`
	StrengthField   string                `json:"strengthField,omitempty"`
	AverageStrength *float64              `json:"averageStrength,omitempty"`
	MinStrength     *float64              `json:"minStrength,omitempty"`
	MaxStrength     *float64              `json:"maxStrength,omitempty"`
	Segments        []BuffCoverageSegment `json:"segments,omitempty"`
}

type PlayerBuffCoverage struct {
	PlayerID      string         `json:"playerId"`
	PlayerName    string         `json:"playerName"`
	IsSelf        bool           `json:"isSelf"`
	BattleSeconds float64        `json:"battleSeconds,omitempty"`
	Buffs         []BuffCoverage `json:"buffs"`
}

type statusIntervalKey struct {
	entityID    string
	conditionID uint32
}

type statusInterval struct {
	EntityID    string
	ConditionID uint32
	StartedAt   int64
	EndedAt     int64
	RawDetail   string
	Strength    *float64
}

type buffParticipant struct {
	name   string
	isSelf bool
}

func isMonitoredStatus(conditionID uint32) bool {
	if _, ok := monitoredStatusNames[conditionID]; ok {
		return true
	}
	_, ok := targetMonitoredStatusNames[conditionID]
	return ok
}

func extractStatusStrength(conditionID uint32, details map[string]packet.ConditionDetailValue) *float64 {
	field := statusStrengthFields[conditionID]
	if field == "" {
		return nil
	}
	detail, ok := details[field]
	if !ok {
		return nil
	}
	value, err := strconv.ParseFloat(detail.Value, 64)
	if err != nil {
		return nil
	}
	return &value
}

func (a *App) recordStatusConditionUnsafe(entityID uint64, conditionID uint32, enabled bool, rawDetail string, details map[string]packet.ConditionDetailValue, at int64) {
	if !isMonitoredStatus(conditionID) {
		return
	}
	if a.activeStatusIntervals == nil {
		a.activeStatusIntervals = make(map[statusIntervalKey]*statusInterval)
	}
	if a.statusIntervals == nil {
		a.statusIntervals = make([]*statusInterval, 0)
	}
	entityIDString := strconv.FormatUint(entityID, 10)
	key := statusIntervalKey{entityID: entityIDString, conditionID: conditionID}
	if active := a.activeStatusIntervals[key]; active != nil {
		active.EndedAt = at
		delete(a.activeStatusIntervals, key)
	}
	if !enabled {
		return
	}

	strength := extractStatusStrength(conditionID, details)
	interval := &statusInterval{
		EntityID:    entityIDString,
		ConditionID: conditionID,
		StartedAt:   at,
		RawDetail:   rawDetail,
		Strength:    strength,
	}
	a.statusIntervals = append(a.statusIntervals, interval)
	a.activeStatusIntervals[key] = interval
}

func (a *App) ensureAppearedStatusUnsafe(entity *packet.EntityInfo, conditionID uint32, at int64) {
	if entity == nil || !isMonitoredStatus(conditionID) {
		return
	}
	if a.activeStatusIntervals == nil {
		a.activeStatusIntervals = make(map[statusIntervalKey]*statusInterval)
	}
	if a.statusIntervals == nil {
		a.statusIntervals = make([]*statusInterval, 0)
	}
	entityID := strconv.FormatUint(entity.Id, 10)
	key := statusIntervalKey{entityID: entityID, conditionID: conditionID}
	if active := a.activeStatusIntervals[key]; active != nil {
		return
	}
	interval := &statusInterval{
		EntityID:    entityID,
		ConditionID: conditionID,
		StartedAt:   at,
	}
	a.statusIntervals = append(a.statusIntervals, interval)
	a.activeStatusIntervals[key] = interval
}

func (a *App) endStatusIntervalsForEntity(entityID uint64, at int64) {
	entityIDString := strconv.FormatUint(entityID, 10)
	a.mu.Lock()
	defer a.mu.Unlock()
	a.endStatusIntervalsForEntityUnsafe(entityIDString, at)
}

func (a *App) endStatusIntervalsForEntityUnsafe(entityID string, at int64) {
	for key, interval := range a.activeStatusIntervals {
		if key.entityID != entityID {
			continue
		}
		interval.EndedAt = at
		delete(a.activeStatusIntervals, key)
	}
	delete(a.battlefieldShockStates, entityID)
}

func (a *App) expireTransientStatusesForChannelChangeUnsafe(at int64) {
	for key, interval := range a.activeStatusIntervals {
		if isChannelPersistentStatus(key.conditionID) {
			continue
		}
		interval.EndedAt = at
		delete(a.activeStatusIntervals, key)
	}

	activeConditions := make(map[statusIntervalKey]EventLog)
	for _, event := range a.eventLogs {
		if event.Type != "condition" {
			continue
		}
		key := statusIntervalKey{entityID: event.EntityID, conditionID: event.ConditionID}
		if event.IsEnable {
			activeConditions[key] = event
		} else {
			delete(activeConditions, key)
		}
	}
	for key, event := range activeConditions {
		if isChannelPersistentStatus(key.conditionID) {
			continue
		}
		event.At = at
		event.IsEnable = false
		event.AttackerID = ""
		event.AttackerName = ""
		a.eventLogs = append(a.eventLogs, event)
	}
	if len(a.eventLogs) > 500 {
		a.eventLogs = a.eventLogs[len(a.eventLogs)-500:]
	}

	for _, entity := range a.entities {
		kept := entity.Conditions[:0]
		for _, conditionID := range entity.Conditions {
			if isChannelPersistentStatus(conditionID) {
				kept = append(kept, conditionID)
			}
		}
		entity.Conditions = kept
	}

	a.musicPerformances = make(map[uint64]*musicPerformance)
	a.battlefieldShockStates = make(map[string]*battlefieldShockState)
}

func (a *App) resetStatusHistoryUnsafe(at int64) {
	continued := make([]*statusInterval, 0, len(a.activeStatusIntervals))
	active := make(map[statusIntervalKey]*statusInterval, len(a.activeStatusIntervals))
	for key, current := range a.activeStatusIntervals {
		current.EndedAt = at
		next := &statusInterval{
			EntityID:    current.EntityID,
			ConditionID: current.ConditionID,
			StartedAt:   at,
			RawDetail:   current.RawDetail,
			Strength:    current.Strength,
		}
		continued = append(continued, next)
		active[key] = next
	}
	a.statusIntervals = continued
	a.activeStatusIntervals = active
}

func splitTargetBuffCoverage(coverage []PlayerBuffCoverage, targetID string) ([]PlayerBuffCoverage, *PlayerBuffCoverage) {
	for index, entry := range coverage {
		if entry.PlayerID != targetID {
			continue
		}
		remaining := make([]PlayerBuffCoverage, 0, len(coverage)-1)
		remaining = append(remaining, coverage[:index]...)
		remaining = append(remaining, coverage[index+1:]...)
		return remaining, &entry
	}
	return coverage, nil
}

var targetBuffCoverageNames = map[string]struct{}{
	"枯木之佩塔克":          {},
	"布隆塔纳斯":           {},
	"雷内恩的米耶尔":         {},
	"雷内恩的米耶尔:悔恨":      {},
	"如凯莱赫般坚固的木头人":     {},
	"如伊鲁夏般坚固的木头人":     {},
	"如塔赫杜因盖尔塔般坚固的木头人": {},
	"如女妖王般坚固的木头人":     {},
	"如浣熊般弱的木头人":       {},
}

func shouldMonitorTargetBuffCoverage(targetName string) bool {
	name := strings.TrimSpace(targetName)
	if _, ok := targetBuffCoverageNames[name]; ok {
		return true
	}

	// History names may include the display suffix (for example
	// "雷内恩的米耶尔 #6240" or "雷内恩的米耶尔(506240)"). Match only
	// those explicit suffix boundaries so similarly prefixed monsters stay out.
	for _, marker := range []string{" #", "("} {
		if index := strings.Index(name, marker); index > 0 {
			_, ok := targetBuffCoverageNames[strings.TrimSpace(name[:index])]
			if ok {
				return true
			}
		}
	}
	return false
}

func (a *App) buildBuffCoverageForTargetUnsafe(start, end int64, targetID, targetName string, participants map[string]buffParticipant, aliases ...string) ([]PlayerBuffCoverage, *PlayerBuffCoverage) {
	playerCoverage := a.buildBuffCoverageUnsafe(start, end, participants)
	monitoredTarget := shouldMonitorTargetBuffCoverage(targetName)
	if !monitoredTarget {
		for _, alias := range aliases {
			if shouldMonitorTargetBuffCoverage(alias) {
				monitoredTarget = true
				break
			}
		}
	}
	if !monitoredTarget {
		return playerCoverage, nil
	}
	targetParticipants := make(map[string]buffParticipant, len(participants)+1)
	for id, participant := range participants {
		targetParticipants[id] = participant
	}
	targetParticipants[targetID] = buffParticipant{name: targetName}
	coverage := a.buildBuffCoverageWithStatusesUnsafe(start, end, targetParticipants, targetMonitoredStatusOrder, targetMonitoredStatusNames, nil)
	var targetCoverage *PlayerBuffCoverage
	_, targetCoverage = splitTargetBuffCoverage(coverage, targetID)
	return playerCoverage, targetCoverage
}
func (a *App) buildBuffCoverageUnsafe(start, end int64, participants map[string]buffParticipant) []PlayerBuffCoverage {
	return a.buildBuffCoverageWithStatusesUnsafe(start, end, participants, monitoredStatusOrder, monitoredStatusNames, statusStrengthFields)
}

func (a *App) buildBuffCoverageWithStatusesUnsafe(start, end int64, participants map[string]buffParticipant, statusOrder []uint32, statusNames map[uint32]string, strengthFields map[uint32]string) []PlayerBuffCoverage {
	if a.analysisLoggingEnabled() {
		coverageStarted := time.Now()
		defer func() { a.recordBuffCoverage(time.Since(coverageStarted)) }()
	}
	if end <= start {
		return nil
	}
	players := make(map[string]buffParticipant, len(participants))
	for id, participant := range participants {
		players[id] = participant
	}

	type buffAggregate struct {
		seconds          float64
		strengthSeconds  float64
		weightedStrength float64
		minStrength      *float64
		maxStrength      *float64
		segments         []BuffCoverageSegment
	}
	aggregates := make(map[string]map[uint32]*buffAggregate)
	for _, interval := range a.statusIntervals {
		intervalEnd := interval.EndedAt
		if intervalEnd == 0 || intervalEnd > end {
			intervalEnd = end
		}
		intervalStart := interval.StartedAt
		if intervalStart < start {
			intervalStart = start
		}
		if intervalStart >= intervalEnd {
			continue
		}
		_, knownParticipant := players[interval.EntityID]
		if !knownParticipant {
			continue
		}
		if aggregates[interval.EntityID] == nil {
			aggregates[interval.EntityID] = make(map[uint32]*buffAggregate)
		}
		aggregate := aggregates[interval.EntityID][interval.ConditionID]
		if aggregate == nil {
			aggregate = &buffAggregate{}
			aggregates[interval.EntityID][interval.ConditionID] = aggregate
		}
		seconds := float64(intervalEnd-intervalStart) / float64(timePrecisionScale)
		aggregate.seconds += seconds
		segment := BuffCoverageSegment{
			StartedAt:     intervalStart,
			EndedAt:       intervalEnd,
			StartOffset:   float64(intervalStart-start) / float64(timePrecisionScale),
			EndOffset:     float64(intervalEnd-start) / float64(timePrecisionScale),
			ActiveSeconds: seconds,
			Strength:      interval.Strength,
			RawDetail:     interval.RawDetail,
		}
		aggregate.segments = append(aggregate.segments, segment)
		if interval.Strength != nil {
			value := *interval.Strength
			aggregate.strengthSeconds += seconds
			aggregate.weightedStrength += value * seconds
			if aggregate.minStrength == nil || value < *aggregate.minStrength {
				minValue := value
				aggregate.minStrength = &minValue
			}
			if aggregate.maxStrength == nil || value > *aggregate.maxStrength {
				maxValue := value
				aggregate.maxStrength = &maxValue
			}
		}
	}
	// 战场的震慑是团队范围效果，但游戏只向当前角色发送状态包。
	// 本角色存在该状态时，将其覆盖区间共享给本场已确认的队友。
	if a.selfId != "" {
		sharedShock := aggregates[a.selfId][battlefieldShockStatusID]
		if sharedShock != nil {
			for playerID := range players {
				if playerID == a.selfId {
					continue
				}
				if aggregates[playerID] == nil {
					aggregates[playerID] = make(map[uint32]*buffAggregate)
				}
				if aggregates[playerID][battlefieldShockStatusID] == nil {
					aggregates[playerID][battlefieldShockStatusID] = sharedShock
				}
			}
		}
	}

	battleSeconds := float64(end-start) / float64(timePrecisionScale)
	result := make([]PlayerBuffCoverage, 0, len(players))
	for playerID, participant := range players {
		buffs := make([]BuffCoverage, 0, len(statusOrder))
		for _, conditionID := range statusOrder {
			coverage := BuffCoverage{
				ConditionID:   conditionID,
				ConditionName: statusNames[conditionID],
				IconID:        statusIconIDs[conditionID],
				StrengthField: strengthFields[conditionID],
			}
			if aggregate := aggregates[playerID][conditionID]; aggregate != nil {
				coverage.ActiveSeconds = aggregate.seconds
				coverage.CoveragePercent = aggregate.seconds / battleSeconds * 100
				if coverage.CoveragePercent > 100 {
					coverage.CoveragePercent = 100
				}
				coverage.MinStrength = aggregate.minStrength
				coverage.MaxStrength = aggregate.maxStrength
				coverage.Segments = aggregate.segments
				if aggregate.strengthSeconds > 0 {
					average := aggregate.weightedStrength / aggregate.strengthSeconds
					coverage.AverageStrength = &average
				}
			}
			buffs = append(buffs, coverage)
		}
		name := participant.name
		if name == "" {
			name = a.getEntityNameUnsafe(playerID)
		}
		result = append(result, PlayerBuffCoverage{
			PlayerID:      playerID,
			PlayerName:    name,
			IsSelf:        participant.isSelf || playerID == a.selfId,
			BattleSeconds: battleSeconds,
			Buffs:         buffs,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsSelf != result[j].IsSelf {
			return result[i].IsSelf
		}
		if result[i].PlayerName != result[j].PlayerName {
			return result[i].PlayerName < result[j].PlayerName
		}
		return result[i].PlayerID < result[j].PlayerID
	})
	return result
}
