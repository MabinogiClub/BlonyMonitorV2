package app

import (
	"strings"
	"time"
)

const (
	dungeonNameCandidateTTLMillis   int64 = 3000
	dungeonNameFallbackWaitMillis         = 1000
	recentDungeonNameCandidateLimit       = 10
)

type dungeonNameCandidate struct {
	Name     string
	EntityID string
	At       int64
}

func (a *App) clearRecentDungeonNameCandidatesUnsafe() {
	a.recentDungeonNameCandidates = nil
}

func (a *App) rememberDungeonNameCandidateUnsafe(name, entityID string, nowMillis int64) {
	a.recentDungeonNameCandidates = append(a.recentDungeonNameCandidates, dungeonNameCandidate{
		Name:     name,
		EntityID: entityID,
		At:       nowMillis,
	})
	if len(a.recentDungeonNameCandidates) > recentDungeonNameCandidateLimit {
		a.recentDungeonNameCandidates = a.recentDungeonNameCandidates[len(a.recentDungeonNameCandidates)-recentDungeonNameCandidateLimit:]
	}
}

func dungeonNameHasKeyword(name string) bool {
	return strings.Contains(name, "地下城")
}

func (a *App) inDungeonNameAcceptWindowUnsafe(nowMillis int64) bool {
	if a.currentDungeon == nil {
		return false
	}
	if a.currentDungeon.EnteredAt == 0 {
		return true
	}

	enteredMillis := a.currentDungeon.EnteredAt * 10
	return nowMillis-enteredMillis <= dungeonNameCandidateTTLMillis
}

func (a *App) bestRecentDungeonNameCandidateUnsafe(nowMillis int64) (string, bool) {
	if name, ok := a.bestRecentDungeonNameCandidateWithPreferenceUnsafe(nowMillis, true); ok {
		return name, true
	}
	return a.bestRecentDungeonNameCandidateWithPreferenceUnsafe(nowMillis, false)
}

func (a *App) bestRecentDungeonNameCandidateWithPreferenceUnsafe(nowMillis int64, requireKeyword bool) (string, bool) {
	for i := len(a.recentDungeonNameCandidates) - 1; i >= 0; i-- {
		candidate := a.recentDungeonNameCandidates[i]
		if nowMillis-candidate.At > dungeonNameCandidateTTLMillis {
			continue
		}
		if a.selfId != "" && candidate.EntityID != a.selfId {
			continue
		}
		if requireKeyword && !dungeonNameHasKeyword(candidate.Name) {
			continue
		}
		return candidate.Name, true
	}
	return "", false
}

func (a *App) applyDungeonChineseNameUnsafe(name string) (*CurrentMapInfo, bool) {
	if a.currentDungeon == nil || a.dungeonChineseNameReceived {
		return nil, false
	}

	a.dungeonChineseNameReceived = true
	a.dungeonLocalName = name
	a.dungeonSaveName = name

	return &CurrentMapInfo{
		MapID:     int(a.currentDungeon.DungeonID),
		MapName:   name,
		LocalName: "地下城",
	}, true
}

func (a *App) scheduleDungeonNameFallback(instanceID uint64, enteredAt int64) {
	go func() {
		time.Sleep(time.Duration(dungeonNameFallbackWaitMillis) * time.Millisecond)

		nowMillis := time.Now().UnixMilli()
		var currentMap *CurrentMapInfo

		a.mu.Lock()
		if a.currentDungeon == nil ||
			a.currentDungeon.InstanceID != instanceID ||
			a.currentDungeon.EnteredAt != enteredAt ||
			a.dungeonChineseNameReceived {
			a.mu.Unlock()
			return
		}

		if name, ok := a.bestRecentDungeonNameCandidateUnsafe(nowMillis); ok {
			if mapInfo, applied := a.applyDungeonChineseNameUnsafe(name); applied {
				currentMap = mapInfo
				logger.Printf("[Dungeon] 使用提前到达的地下城中文名兜底: %s\n", name)
			}
		}
		a.mu.Unlock()

		if currentMap != nil {
			a.setCurrentMap(currentMap)
		}
	}()
}
