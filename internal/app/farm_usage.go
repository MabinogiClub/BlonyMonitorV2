package app

import (
	"strconv"
	"strings"
	"time"
)

func (a *App) rememberFarmAccount(accountID, characterID string) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return
	}
	accountKey := "account:" + accountID
	a.mu.Lock()
	a.accountID = accountKey
	if a.characterAccounts == nil {
		a.characterAccounts = make(map[string]string)
	}
	if characterID != "" {
		a.characterAccounts[characterID] = accountKey
	}
	a.mu.Unlock()
}

// farmCharacterForOwner returns the stable character-scoped key used by farm
// statistics. Account IDs are intentionally not used here because one account
// can contain multiple characters whose usage must remain isolated.
func (a *App) farmCharacterForOwner(ownerID uint64) string {
	if ownerID != 0 {
		return "character:" + strconv.FormatUint(ownerID, 10)
	}
	a.mu.RLock()
	selfID := a.selfId
	a.mu.RUnlock()
	if selfID == "" {
		return ""
	}
	return "character:" + selfID
}

func (a *App) recordFarmPlant(ownerID uint64, cropName string, coinCost int64, at time.Time) {
	if a.farmStatsStore == nil {
		return
	}
	character := a.farmCharacterForOwner(ownerID)
	if err := a.farmStatsStore.RecordPlant(character, cropName, coinCost, at); err != nil {
		logger.Printf("[Farm] record planting usage failed: %v", err)
	}
}

func (a *App) recordFarmEnergy(ownerID uint64, energy int, at time.Time) {
	if a.farmStatsStore == nil {
		return
	}
	character := a.farmCharacterForOwner(ownerID)
	if character == "" {
		return
	}
	a.mu.Lock()
	if a.farmEnergyByCharacter == nil {
		a.farmEnergyByCharacter = make(map[string]int)
	}
	previous, known := a.farmEnergyByCharacter[character]
	a.farmEnergyByCharacter[character] = energy
	a.mu.Unlock()
	if !known || energy >= previous {
		return
	}
	if err := a.farmStatsStore.RecordEnergy(character, previous-energy, at); err != nil {
		logger.Printf("[Farm] record energy usage failed: %v", err)
	}
}

func (a *App) decorateFarmState(state FarmState) FarmState {
	state.CropCounts = map[string]int64{}
	state.TodayCropCounts = map[string]int64{}
	state.YesterdayCropCounts = map[string]int64{}
	state.DailyTrend = []FarmDailyUsage{}
	character := a.farmCharacterForOwner(0)
	state.AccountID = character
	if a.farmStatsStore == nil || character == "" {
		return state
	}
	usage, err := a.farmStatsStore.Usage(character)
	if err != nil {
		logger.Printf("[Farm] read usage failed: %v", err)
		return state
	}
	state.CoinsSpent = usage.CoinsSpent
	state.EnergySpent = usage.EnergySpent
	state.CropCounts = usage.CropCounts
	if history, err := a.farmStatsStore.UsageHistory(character, time.Now(), 30); err != nil {
		logger.Printf("[Farm] read usage history failed: %v", err)
	} else {
		if len(history) > 0 {
			state.TodayCropCounts = history[len(history)-1].CropCounts
		}
		if len(history) > 1 {
			state.YesterdayCropCounts = history[len(history)-2].CropCounts
		}
		state.DailyTrend = make([]FarmDailyUsage, len(history))
		for i, daily := range history {
			state.DailyTrend[i] = FarmDailyUsage{
				Date: daily.Date, CoinsSpent: daily.CoinsSpent,
				EnergySpent: daily.EnergySpent, CropCounts: daily.CropCounts,
			}
		}
	}
	if life, err := a.farmStatsStore.CurrencyUsage(character, lifeAssociationCurrency); err != nil {
		logger.Printf("[LifeCoin] read usage failed: %v", err)
	} else {
		state.LifeCoinsAdded = life.Added
		state.LifeCoinsSpent = life.Spent
		state.LifeCoinsBalance = life.Balance
		state.LifeCoinsKnown = life.BalanceKnown
	}
	if lifeHistory, err := a.farmStatsStore.CurrencyUsageHistory(character, lifeAssociationCurrency, time.Now(), 30); err == nil {
		if len(state.DailyTrend) == 0 {
			state.DailyTrend = make([]FarmDailyUsage, len(lifeHistory))
			for i, day := range lifeHistory {
				state.DailyTrend[i] = FarmDailyUsage{Date: day.Date, LifeCoinsAdded: day.Added, LifeCoinsSpent: day.Spent, LifeCoinsBalance: day.Balance, CropCounts: map[string]int64{}}
			}
		} else {
			byDate := make(map[string]int, len(state.DailyTrend))
			for i, day := range state.DailyTrend {
				byDate[day.Date] = i
			}
			for _, day := range lifeHistory {
				if i, ok := byDate[day.Date]; ok {
					state.DailyTrend[i].LifeCoinsAdded = day.Added
					state.DailyTrend[i].LifeCoinsSpent = day.Spent
					state.DailyTrend[i].LifeCoinsBalance = day.Balance
				}
			}
		}
	} else {
		logger.Printf("[LifeCoin] read usage history failed: %v", err)
	}
	return state
}
