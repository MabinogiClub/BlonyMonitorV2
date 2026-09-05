package app

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"blonymonitorv2/internal/packet"
)

const lifeAssociationCurrency = "life_association_coin"

const lifeCoinSettleDelay = 500 * time.Millisecond

type lifeCoinPendingSync struct {
	balance    int64
	at         time.Time
	generation uint64
	timer      *time.Timer
}

// 5300264 is the item class used by the server for 生活协会硬币. Pocket 65
// also contains unrelated currency items (for example seals), so pocket type
// alone must not be used when rebuilding an authoritative total.
const lifeAssociationCoinItemID uint32 = 5300264

var lifeCoinBalanceRE = regexp.MustCompile(`生活协会硬币[^/]{0,80}/\s*([0-9][0-9,]*)`)

func lifeCoinCharacterKey(id uint64) string {
	if id == 0 {
		return ""
	}
	return "character:" + strconv.FormatUint(id, 10)
}

func (a *App) handleLifeCoinPacket(pkt *packet.GamePacket) {
	if pkt == nil || a.farmStatsStore == nil {
		return
	}
	// 0x5209 is wrapped in a synthetic entity ID in some login/channel
	// responses. Parse the character header first and always use its ID for
	// the currency owner; the outer packet ID is not a character ID.
	var characterData *packet.CharacterDataPacket
	if pkt.Op == opcodeCharacterData {
		parsed, err := packet.ParseCharacterDataPacket(pkt)
		if err != nil || parsed == nil || parsed.Id == 0 || !isPC(int(parsed.RaceId)) {
			return
		}
		characterData = parsed
	}
	character := lifeCoinCharacterKey(characterDataID(characterData))
	if character == "" {
		character = lifeCoinCharacterKey(pkt.Id)
	}
	if character == "" {
		a.mu.RLock()
		character = lifeCoinCharacterKeyString(a.selfId)
		a.mu.RUnlock()
	}
	if character == "" {
		return
	}
	at := pkt.At
	if at.IsZero() {
		at = time.Now()
	}

	// Character snapshots contain every currency-pocket stack and are the
	// authoritative total, which also repairs changes missed while capture was off.
	if pkt.Op == opcodeCharacterData {
		total := int64(0)
		stacks := make(map[uint64]int64)
		for _, record := range packet.ParseInventoryItemRecords(pkt.Msg) {
			if record.Item == nil || record.Item.PocketType != inventoryCurrencyPocket || record.Item.ItemId != lifeAssociationCoinItemID {
				continue
			}
			amount := int64(record.Item.Amount)
			total += amount
			if record.InstanceID != 0 {
				stacks[record.InstanceID] = amount
			}
		}
		a.mu.Lock()
		a.lifeCoinStacks[character] = stacks
		a.lifeCoinBalances[character] = total
		a.lifeCoinBalanceKnown[character] = true
		a.mu.Unlock()
		if err := a.syncLifeCoinBalanceNow(character, total, at); err != nil {
			logger.Printf("[LifeCoin] sync balance failed: %v", err)
		}
		return
	}

	// 0x59ee updates one currency stack. Once a snapshot established the
	// aggregate balance, update the in-memory total and settle the net change
	// only after the server's burst of stack rebalancing packets is complete.
	if pkt.Op == opcodeItemStackUpdate && len(pkt.Msg) >= 3 &&
		pkt.Msg[0].Type() == packet.MessageElemTypeLong && pkt.Msg[1].Type() == packet.MessageElemTypeShort &&
		pkt.Msg[2].Type() == packet.MessageElemTypeInt && pkt.Msg[2].Data().(uint32) == inventoryCurrencyPocket {
		instance := pkt.Msg[0].Data().(uint64)
		amount := int64(pkt.Msg[1].Data().(uint16))
		a.mu.Lock()
		stacks := a.lifeCoinStacks[character]
		previous, known := stacks[instance]
		// 0x59ee does not contain an item ID. Before a full character
		// snapshot identifies the instance, a pocket-65 update could belong to
		// any other currency and must not become a provisional life-coin total.
		if !known || !a.lifeCoinBalanceKnown[character] {
			a.mu.Unlock()
			return
		}
		stacks[instance] = amount
		balance := a.lifeCoinBalances[character]
		balance += amount - previous
		a.lifeCoinBalances[character] = balance
		a.mu.Unlock()
		a.scheduleLifeCoinBalanceSync(character, balance, at)
		return
	}
	if pkt.Op == opcodeItemRemoveUpdate && len(pkt.Msg) >= 1 && pkt.Msg[0].Type() == packet.MessageElemTypeLong {
		instance := pkt.Msg[0].Data().(uint64)
		a.mu.Lock()
		stacks := a.lifeCoinStacks[character]
		amount, known := stacks[instance]
		if known {
			delete(stacks, instance)
		}
		balanceKnown := a.lifeCoinBalanceKnown[character]
		balance := a.lifeCoinBalances[character]
		if known && balanceKnown {
			balance -= amount
			if balance < 0 {
				balance = 0
			}
			a.lifeCoinBalances[character] = balance
		}
		a.mu.Unlock()
		if known && balanceKnown {
			a.scheduleLifeCoinBalanceSync(character, balance, at)
		}
		return
	}

	// Some versions put the current total in an NPC response instead of an
	// inventory snapshot. Treat only the explicit "cost / current" form as sync.
	for _, elem := range pkt.Msg {
		if elem == nil || elem.Type() != packet.MessageElemTypeString {
			continue
		}
		text, _ := elem.Data().(string)
		match := lifeCoinBalanceRE.FindStringSubmatch(text)
		if len(match) != 2 {
			continue
		}
		value, err := strconv.ParseInt(strings.ReplaceAll(match[1], ",", ""), 10, 64)
		if err != nil {
			continue
		}
		a.mu.Lock()
		a.lifeCoinBalances[character] = value
		a.lifeCoinBalanceKnown[character] = true
		a.mu.Unlock()
		if err := a.syncLifeCoinBalanceNow(character, value, at); err != nil {
			logger.Printf("[LifeCoin] text balance sync failed: %v", err)
		}
	}
}

func (a *App) scheduleLifeCoinBalanceSync(character string, balance int64, at time.Time) {
	a.mu.Lock()
	if a.lifeCoinPending == nil {
		a.lifeCoinPending = make(map[string]*lifeCoinPendingSync)
	}
	if pending := a.lifeCoinPending[character]; pending != nil && pending.timer != nil {
		pending.timer.Stop()
	}
	a.lifeCoinPendingGeneration++
	generation := a.lifeCoinPendingGeneration
	pending := &lifeCoinPendingSync{balance: balance, at: at, generation: generation}
	pending.timer = time.AfterFunc(lifeCoinSettleDelay, func() {
		a.flushPendingLifeCoinBalance(character, generation)
	})
	a.lifeCoinPending[character] = pending
	a.mu.Unlock()
}

func (a *App) flushPendingLifeCoinBalance(character string, generation uint64) {
	a.lifeCoinSyncMu.Lock()
	defer a.lifeCoinSyncMu.Unlock()

	a.mu.Lock()
	pending := a.lifeCoinPending[character]
	if pending == nil || generation != 0 && pending.generation != generation {
		a.mu.Unlock()
		return
	}
	if pending.timer != nil {
		pending.timer.Stop()
	}
	delete(a.lifeCoinPending, character)
	store := a.farmStatsStore
	a.mu.Unlock()

	if store != nil {
		if err := store.SyncCurrencyBalance(character, lifeAssociationCurrency, pending.balance, pending.at); err != nil {
			logger.Printf("[LifeCoin] settle stack balance failed: %v", err)
		}
	}
}

func (a *App) flushAllPendingLifeCoinBalances() {
	a.mu.RLock()
	characters := make([]string, 0, len(a.lifeCoinPending))
	for character := range a.lifeCoinPending {
		characters = append(characters, character)
	}
	a.mu.RUnlock()
	for _, character := range characters {
		a.flushPendingLifeCoinBalance(character, 0)
	}
}

func (a *App) syncLifeCoinBalanceNow(character string, balance int64, at time.Time) error {
	a.lifeCoinSyncMu.Lock()
	defer a.lifeCoinSyncMu.Unlock()

	a.mu.Lock()
	if pending := a.lifeCoinPending[character]; pending != nil {
		if pending.timer != nil {
			pending.timer.Stop()
		}
		delete(a.lifeCoinPending, character)
	}
	store := a.farmStatsStore
	a.mu.Unlock()
	if store == nil {
		return nil
	}
	return store.SyncCurrencyBalance(character, lifeAssociationCurrency, balance, at)
}

func characterDataID(character *packet.CharacterDataPacket) uint64 {
	if character == nil {
		return 0
	}
	return character.Id
}

func lifeCoinCharacterKeyString(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	return "character:" + id
}
