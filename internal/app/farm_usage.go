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
	provisionalKey := ""
	if characterID != "" {
		provisionalKey = "character:" + characterID
	}
	a.mu.Lock()
	a.accountID = accountKey
	if a.characterAccounts == nil {
		a.characterAccounts = make(map[string]string)
	}
	if characterID != "" {
		a.characterAccounts[characterID] = accountKey
		if energy, exists := a.farmEnergyByAccount[provisionalKey]; exists {
			if _, accountKnown := a.farmEnergyByAccount[accountKey]; !accountKnown {
				a.farmEnergyByAccount[accountKey] = energy
			}
			delete(a.farmEnergyByAccount, provisionalKey)
		}
	}
	a.mu.Unlock()
	if a.farmStatsStore != nil && provisionalKey != "" {
		if err := a.farmStatsStore.Merge(provisionalKey, accountKey); err != nil {
			logger.Printf("[Farm] merge provisional usage failed: %v", err)
		}
	}
}

func (a *App) farmAccountForOwner(ownerID uint64) string {
	owner := strconv.FormatUint(ownerID, 10)
	a.mu.RLock()
	if ownerID != 0 {
		if account := a.characterAccounts[owner]; account != "" {
			a.mu.RUnlock()
			return account
		}
	}
	if a.accountID != "" && (ownerID == 0 || owner == a.selfId) {
		account := a.accountID
		a.mu.RUnlock()
		return account
	}
	a.mu.RUnlock()
	if ownerID != 0 {
		return "character:" + owner
	}
	a.mu.RLock()
	selfID := a.selfId
	a.mu.RUnlock()
	if selfID != "" {
		return "character:" + selfID
	}
	return ""
}

func (a *App) recordFarmPlant(ownerID uint64, cropName string, coinCost int64, at time.Time) {
	if a.farmStatsStore == nil {
		return
	}
	account := a.farmAccountForOwner(ownerID)
	if err := a.farmStatsStore.RecordPlant(account, cropName, coinCost, at); err != nil {
		logger.Printf("[Farm] record planting usage failed: %v", err)
	}
}

func (a *App) recordFarmEnergy(ownerID uint64, energy int, at time.Time) {
	if a.farmStatsStore == nil {
		return
	}
	account := a.farmAccountForOwner(ownerID)
	if account == "" {
		return
	}
	a.mu.Lock()
	previous, known := a.farmEnergyByAccount[account]
	a.farmEnergyByAccount[account] = energy
	a.mu.Unlock()
	if !known || energy >= previous {
		return
	}
	if err := a.farmStatsStore.RecordEnergy(account, previous-energy, at); err != nil {
		logger.Printf("[Farm] record energy usage failed: %v", err)
	}
}

func (a *App) decorateFarmState(state FarmState) FarmState {
	state.CropCounts = map[string]int64{}
	state.TodayCropCounts = map[string]int64{}
	state.YesterdayCropCounts = map[string]int64{}
	state.DailyTrend = []FarmDailyUsage{}
	account := a.farmAccountForOwner(0)
	state.AccountID = account
	if a.farmStatsStore == nil || account == "" {
		return state
	}
	usage, err := a.farmStatsStore.Usage(account)
	if err != nil {
		logger.Printf("[Farm] read usage failed: %v", err)
		return state
	}
	state.AccountID = usage.AccountID
	state.CoinsSpent = usage.CoinsSpent
	state.EnergySpent = usage.EnergySpent
	state.CropCounts = usage.CropCounts
	if history, err := a.farmStatsStore.UsageHistory(account, time.Now(), 30); err != nil {
		logger.Printf("[Farm] read usage history failed: %v", err)
	} else {
		if len(history) > 0 {
			state.TodayCropCounts = history[len(history)-1].CropCounts
		}
		if len(history) > 1 {
			state.YesterdayCropCounts = history[len(history)-2].CropCounts
		}
		if len(history) > 0 {
			state.TodayCoinsSpent = history[len(history)-1].CoinsSpent
			state.TodayEnergySpent = history[len(history)-1].EnergySpent
		}
		if len(history) > 1 {
			state.YesterdayCoinsSpent = history[len(history)-2].CoinsSpent
			state.YesterdayEnergySpent = history[len(history)-2].EnergySpent
		}
		state.DailyTrend = make([]FarmDailyUsage, len(history))
		for i, daily := range history {
			state.DailyTrend[i] = FarmDailyUsage{
				Date: daily.Date, CoinsSpent: daily.CoinsSpent,
				EnergySpent: daily.EnergySpent, CropCounts: daily.CropCounts,
			}
		}
	}
	return state
}
