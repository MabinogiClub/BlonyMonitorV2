package app

import (
	"sort"
	"strconv"
)

const (
	// minAlwaysRetainedTargetMaxHP keeps boss-tier targets (佩隆, 幻影, 结界, …)
	// even after wipes. Trash below this threshold uses a recency stack.
	minAlwaysRetainedTargetMaxHP = 20_000_000
	maxStackedLowHPTargets       = 50
)

type targetRetentionRank struct {
	ID       string
	MaxHP    float64
	Appeared int64
	LastHit  int64
}

func (rank targetRetentionRank) recency() int64 {
	if rank.LastHit > rank.Appeared {
		return rank.LastHit
	}
	return rank.Appeared
}

func keepRetainedTargetIDs(ranks []targetRetentionRank) map[string]struct{} {
	high := make([]targetRetentionRank, 0, len(ranks))
	low := make([]targetRetentionRank, 0, len(ranks))
	for _, rank := range ranks {
		if rank.ID == "" {
			continue
		}
		if rank.MaxHP >= minAlwaysRetainedTargetMaxHP {
			high = append(high, rank)
			continue
		}
		low = append(low, rank)
	}

	if len(low) > maxStackedLowHPTargets {
		sort.SliceStable(low, func(i, j int) bool {
			if left, right := low[i].recency(), low[j].recency(); left != right {
				return left > right
			}
			return low[i].ID < low[j].ID
		})
		low = low[:maxStackedLowHPTargets]
	}

	ids := make(map[string]struct{}, len(high)+len(low))
	for _, rank := range high {
		ids[rank.ID] = struct{}{}
	}
	for _, rank := range low {
		ids[rank.ID] = struct{}{}
	}
	return ids
}

func (a *App) targetMaxHPUnsafe(id string) float64 {
	if info := a.bossHP[id]; info != nil && info.MaxHP > 0 {
		return info.MaxHP
	}
	history := a.bossHPHistory[id]
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].MaxHP > 0 {
			return history[i].MaxHP
		}
	}
	return 0
}

func targetHasHitsSince(stat *targetAggStats, sinceSeq int64) bool {
	if stat == nil {
		return false
	}
	for _, attacker := range stat.attackers {
		if attacker == nil {
			continue
		}
		for _, skill := range attacker.skills {
			if skill == nil {
				continue
			}
			for _, record := range skill.records {
				if sinceSeq <= 0 || record.Seq > sinceSeq {
					return true
				}
			}
		}
	}
	return false
}

func (a *App) retainedTakenTargetIDsUnsafe(sinceSeq int64, requireHits bool) map[string]struct{} {
	ranks := make([]targetRetentionRank, 0, len(a.takenStats))
	for id, stat := range a.takenStats {
		if requireHits && !targetHasHitsSince(stat, sinceSeq) {
			continue
		}
		appeared := int64(0)
		lastHit := int64(0)
		if stat != nil {
			appeared = stat.firstHit
			lastHit = stat.lastHit
		}
		ranks = append(ranks, targetRetentionRank{
			ID:       id,
			MaxHP:    a.targetMaxHPUnsafe(id),
			Appeared: appeared,
			LastHit:  lastHit,
		})
	}
	return keepRetainedTargetIDs(ranks)
}

func jsonNumberAsFloat(value interface{}) float64 {
	switch number := value.(type) {
	case float64:
		return number
	case float32:
		return float64(number)
	case int:
		return float64(number)
	case int64:
		return float64(number)
	case uint64:
		return float64(number)
	}
	return 0
}

func keepRetainedRawTargets(targets []interface{}) []interface{} {
	if len(targets) == 0 {
		return targets
	}
	ranks := make([]targetRetentionRank, len(targets))
	for index, raw := range targets {
		id := "idx:" + strconv.Itoa(index)
		maxHP := 0.0
		appeared := int64(0)
		lastHit := int64(0)
		if target, ok := raw.(map[string]interface{}); ok {
			if value, ok := target["targetId"].(string); ok && value != "" {
				id = value
			} else if value, ok := target["id"].(string); ok && value != "" {
				id = value
			}
			if hp, ok := target["bossHP"].(map[string]interface{}); ok {
				maxHP = jsonNumberAsFloat(hp["maxHp"])
			}
			appeared = int64(jsonNumberAsFloat(target["appearedAt"]))
			if deathTime := int64(jsonNumberAsFloat(target["deathTime"])); deathTime > lastHit {
				lastHit = deathTime
			}
			if cleanedAt := int64(jsonNumberAsFloat(target["cleanedAt"])); cleanedAt > lastHit {
				lastHit = cleanedAt
			}
		}
		ranks[index] = targetRetentionRank{ID: id, MaxHP: maxHP, Appeared: appeared, LastHit: lastHit}
	}
	keep := keepRetainedTargetIDs(ranks)
	if len(keep) == len(targets) {
		return targets
	}
	filtered := make([]interface{}, 0, len(keep))
	for index, raw := range targets {
		if _, ok := keep[ranks[index].ID]; ok {
			filtered = append(filtered, raw)
		}
	}
	return filtered
}
