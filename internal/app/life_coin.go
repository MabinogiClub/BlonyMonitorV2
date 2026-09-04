package app

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"blonymonitorv2/internal/packet"
)

const lifeAssociationCurrency = "life_association_coin"

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
		if err := a.farmStatsStore.SyncCurrencyBalance(character, lifeAssociationCurrency, total, at); err != nil {
			logger.Printf("[LifeCoin] sync balance failed: %v", err)
		}
		return
	}

	// 0x59ee updates one currency stack. Once a snapshot established the
	// aggregate balance, apply the stack delta and let the store classify it.
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
		if err := a.farmStatsStore.SyncCurrencyBalance(character, lifeAssociationCurrency, balance, at); err != nil {
			logger.Printf("[LifeCoin] record stack delta failed: %v", err)
		}
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
			if err := a.farmStatsStore.SyncCurrencyBalance(character, lifeAssociationCurrency, balance, at); err != nil {
				logger.Printf("[LifeCoin] record removed stack failed: %v", err)
			}
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
		if err := a.farmStatsStore.SyncCurrencyBalance(character, lifeAssociationCurrency, value, at); err != nil {
			logger.Printf("[LifeCoin] text balance sync failed: %v", err)
		}
	}
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
