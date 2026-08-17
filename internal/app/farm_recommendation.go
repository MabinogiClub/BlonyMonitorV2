package app

import (
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"blonymonitorv2/internal/packet"
)

const (
	farmRecommendationDataFileName    = "blonymonitor-farm-recommendation.json"
	farmRecommendationCaptureFileName = "blonymonitor-farm-recommendation-capture.jsonl"
	farmRecommendationCaptureMaxBytes = int64(16 * 1024 * 1024)
	farmRecommendationCaptureDuration = 3 * time.Minute
	farmGoalKeys                      = "keys"
	farmGoalCoins                     = "coins"
)

type farmCatalogItem struct {
	ItemID uint32
	Count  int
}

type farmRecipeDefinition struct {
	Name         string
	CraftSeconds int64
	Factory      string
	Materials    []farmCatalogItem
}

type farmDeliveryDefinition struct {
	DBKey           int
	Name            string
	DeliverySeconds int64
	RequiredItems   []farmCatalogItem
}

// FarmDeliveryState is the server-authoritative part of one currently visible order.
// It is persisted locally so recommendations survive application restarts.
type FarmDeliveryState struct {
	DBKey               int            `json:"dbKey"`
	StartedAt           int64          `json:"startedAt,omitempty"`
	RemainingDeliveries int            `json:"remainingDeliveries"`
	MaximumDeliveries   int            `json:"maximumDeliveries"`
	CoinReward          int            `json:"coinReward"`
	KeyReward           int            `json:"keyReward"`
	MaterialRewards     map[string]int `json:"materialRewards,omitempty"`
}

// FarmRecommendationData contains only data observed from the local game session.
type FarmRecommendationData struct {
	Version              int                 `json:"version"`
	CharacterID          string              `json:"characterId,omitempty"`
	Goal                 string              `json:"goal"`
	UseNormalMaterials   bool                `json:"useNormalMaterials"`
	UseAdvancedMaterials bool                `json:"useAdvancedMaterials"`
	UseHighestMaterials  bool                `json:"useHighestMaterials"`
	SyncedAt             int64               `json:"syncedAt,omitempty"`
	OrdersSynced         bool                `json:"ordersSynced"`
	InventorySynced      bool                `json:"inventorySynced"`
	RecipesSynced        bool                `json:"recipesSynced"`
	StorageLevel         int                 `json:"storageLevel,omitempty"`
	Orders               []FarmDeliveryState `json:"orders,omitempty"`
	Inventory            map[string]int      `json:"inventory,omitempty"`
	UnlockedRecipes      []uint32            `json:"unlockedRecipes,omitempty"`
}

type FarmRecommendationAmount struct {
	ItemID   uint32 `json:"itemId"`
	Name     string `json:"name"`
	Count    int    `json:"count"`
	PlotKind string `json:"plotKind,omitempty"`
	Factory  string `json:"factory,omitempty"`
}

type FarmStorageItem struct {
	ItemID   uint32 `json:"itemId"`
	Name     string `json:"name"`
	Count    int    `json:"count"`
	Quality  string `json:"quality"`
	Category string `json:"category"`
	Icon     string `json:"icon"`
}

type FarmOrderRecommendation struct {
	Rank                 int                        `json:"rank"`
	DBKey                int                        `json:"dbKey"`
	Name                 string                     `json:"name"`
	RemainingDeliveries  int                        `json:"remainingDeliveries"`
	MaximumDeliveries    int                        `json:"maximumDeliveries"`
	CoinReward           int                        `json:"coinReward"`
	CoinRewardMin        int                        `json:"coinRewardMin"`
	CoinRewardMax        int                        `json:"coinRewardMax"`
	KeyReward            int                        `json:"keyReward"`
	KeyRewardMin         int                        `json:"keyRewardMin"`
	KeyRewardMax         int                        `json:"keyRewardMax"`
	MaterialRewards      map[string]int             `json:"materialRewards,omitempty"`
	MaterialRewardMin    int                        `json:"materialRewardMin"`
	MaterialRewardMax    int                        `json:"materialRewardMax"`
	RefreshRecommended   bool                       `json:"refreshRecommended"`
	MaterialSufficient   bool                       `json:"materialSufficient"`
	RequiresCrafting     bool                       `json:"requiresCrafting"`
	RequiresPlanting     bool                       `json:"requiresPlanting"`
	DeliveryStatus       string                     `json:"deliveryStatus,omitempty"`
	DeliveryRemaining    int64                      `json:"deliveryRemainingSeconds,omitempty"`
	EstimatedSeconds     int64                      `json:"estimatedSeconds"`
	DeliverySeconds      int64                      `json:"deliverySeconds"`
	TargetPerHour        float64                    `json:"targetPerHour"`
	Eligible             bool                       `json:"eligible"`
	Warnings             []string                   `json:"warnings,omitempty"`
	MissingRecipes       []string                   `json:"missingRecipes,omitempty"`
	SuggestedCrops       []FarmRecommendationAmount `json:"suggestedCrops,omitempty"`
	SuggestedProductions []FarmRecommendationAmount `json:"suggestedProductions,omitempty"`
}

type FarmRecommendationState struct {
	Goal                   string                     `json:"goal"`
	UseNormalMaterials     bool                       `json:"useNormalMaterials"`
	UseAdvancedMaterials   bool                       `json:"useAdvancedMaterials"`
	UseHighestMaterials    bool                       `json:"useHighestMaterials"`
	DataReady              bool                       `json:"dataReady"`
	OrdersSynced           bool                       `json:"ordersSynced"`
	InventorySynced        bool                       `json:"inventorySynced"`
	RecipesSynced          bool                       `json:"recipesSynced"`
	SyncedAt               int64                      `json:"syncedAt,omitempty"`
	StorageLevel           int                        `json:"storageLevel"`
	StorageUsed            int                        `json:"storageUsed"`
	StorageCapacity        int                        `json:"storageCapacity"`
	StorageItems           []FarmStorageItem          `json:"storageItems"`
	CaptureEnabled         bool                       `json:"captureEnabled"`
	CaptureFile            string                     `json:"captureFile,omitempty"`
	StatusMessage          string                     `json:"statusMessage"`
	PlantingSuggestions    []FarmRecommendationAmount `json:"plantingSuggestions"`
	PlantingProductions    []FarmRecommendationAmount `json:"plantingProductions"`
	PlantingReferenceCount int                        `json:"plantingReferenceCount"`
	Recommendations        []FarmOrderRecommendation  `json:"recommendations"`
}

type farmRecommendationManager struct {
	mu                   sync.Mutex
	data                 FarmRecommendationData
	captureEnabled       bool
	captureStartedAt     time.Time
	captureBytes         int64
	captureFile          *os.File
	pendingStorageItemID uint32
	pendingStorageItemAt time.Time
	seenCraftStarts      map[string]bool
	lastRanks            map[int]int
	deliveryRanks        map[int]int
}

func farmRecommendationDataPath() string {
	return filepath.Join(analysisLogDirectory(), farmRecommendationDataFileName)
}

func farmRecommendationCapturePath() string {
	return filepath.Join(analysisLogDirectory(), farmRecommendationCaptureFileName)
}

func defaultFarmRecommendationData() FarmRecommendationData {
	return FarmRecommendationData{
		Version:            3,
		Goal:               farmGoalKeys,
		UseNormalMaterials: true,
		Inventory:          make(map[string]int),
	}
}

func newFarmRecommendationManager() *farmRecommendationManager {
	m := &farmRecommendationManager{data: defaultFarmRecommendationData()}
	data, err := os.ReadFile(farmRecommendationDataPath())
	if err == nil {
		var saved FarmRecommendationData
		if json.Unmarshal(data, &saved) == nil {
			m.data = saved
			m.data.Version = 3
			if m.data.Goal != farmGoalCoins && m.data.Goal != farmGoalKeys {
				m.data.Goal = farmGoalKeys
			}
			if !m.data.UseNormalMaterials && !m.data.UseAdvancedMaterials && !m.data.UseHighestMaterials {
				m.data.UseNormalMaterials = true
			}
			if m.data.Inventory == nil {
				m.data.Inventory = make(map[string]int)
			}
		}
	}
	return m
}

func resetFarmRecommendationDataForCharacter(data FarmRecommendationData, characterID string) FarmRecommendationData {
	reset := defaultFarmRecommendationData()
	reset.CharacterID = characterID
	reset.Goal = data.Goal
	reset.UseNormalMaterials = data.UseNormalMaterials
	reset.UseAdvancedMaterials = data.UseAdvancedMaterials
	reset.UseHighestMaterials = data.UseHighestMaterials
	return reset
}

func (m *farmRecommendationManager) close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.captureFile != nil {
		_ = m.captureFile.Close()
		m.captureFile = nil
	}
	m.captureEnabled = false
}

func (m *farmRecommendationManager) saveLocked() error {
	data, err := json.MarshalIndent(m.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(farmRecommendationDataPath(), data, 0o644)
}

func (m *farmRecommendationManager) setGoal(goal string) error {
	if goal != farmGoalKeys && goal != farmGoalCoins {
		return fmt.Errorf("unknown farm recommendation goal %q", goal)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data.Goal = goal
	return m.saveLocked()
}

func (m *farmRecommendationManager) setMaterialQualities(normal, advanced, highest bool) error {
	if !normal && !advanced && !highest {
		return fmt.Errorf("at least one farm material quality must be enabled")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data.UseNormalMaterials = normal
	m.data.UseAdvancedMaterials = advanced
	m.data.UseHighestMaterials = highest
	return m.saveLocked()
}

func (m *farmRecommendationManager) setCaptureEnabled(enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !enabled {
		m.captureEnabled = false
		if m.captureFile != nil {
			err := m.captureFile.Close()
			m.captureFile = nil
			return err
		}
		return nil
	}
	if m.captureEnabled {
		return nil
	}
	file, err := os.OpenFile(farmRecommendationCapturePath(), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	goal := m.data.Goal
	m.data = defaultFarmRecommendationData()
	m.data.Goal = goal
	m.captureFile = file
	m.captureEnabled = true
	m.captureStartedAt = time.Now()
	m.captureBytes = 0
	marker, _ := json.Marshal(map[string]any{
		"type": "session",
		"at":   m.captureStartedAt.UnixMilli(),
	})
	marker = append(marker, '\n')
	n, err := m.captureFile.Write(marker)
	m.captureBytes += int64(n)
	if err != nil {
		_ = m.captureFile.Close()
		m.captureFile = nil
		m.captureEnabled = false
		return err
	}
	return m.saveLocked()
}

type farmPacketCaptureElement struct {
	Type  uint8 `json:"type"`
	Value any   `json:"value"`
}

type farmPacketCaptureRecord struct {
	Type     string                     `json:"type"`
	At       int64                      `json:"at"`
	Opcode   string                     `json:"opcode"`
	EntityID string                     `json:"entityId"`
	Elements []farmPacketCaptureElement `json:"elements"`
}

func farmCaptureValue(elem packet.IMessageElem) any {
	if elem.Type() == packet.MessageElemTypeBin {
		if value, ok := elem.Data().([]byte); ok {
			return base64.StdEncoding.EncodeToString(value)
		}
	}
	return elem.Data()
}

func skipFarmRecommendationCaptureOpcode(op uint32) bool {
	switch op {
	case opcodeEntityAppear, opcodeEntitiesAppear, opcodeEntityProperty, opcodeEntityRemove,
		opcodeCombatAction, opcodeEffectDamage, opcodeEffectDelayed, opcodeConditionUpdate,
		opcodeSetFinisher, opcodePerformanceStart, opcodePerformanceStop:
		return true
	default:
		return false
	}
}

func farmMessageUint(message packet.Message, index int) (uint32, bool) {
	if index < 0 || index >= len(message) || message[index].Type() != packet.MessageElemTypeInt {
		return 0, false
	}
	return message[index].Data().(uint32), true
}

func farmMessageLong(message packet.Message, index int) (uint64, bool) {
	if index < 0 || index >= len(message) || message[index].Type() != packet.MessageElemTypeLong {
		return 0, false
	}
	return message[index].Data().(uint64), true
}

func parseFarmRecipeList(value string) ([]uint32, bool) {
	parts := strings.Split(value, ";")
	recipes := make([]uint32, 0, len(parts))
	hasKnownRecipe := false
	for _, part := range parts {
		if part == "" {
			continue
		}
		parsed, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			return nil, false
		}
		itemID := uint32(parsed)
		if itemID < 5_041_003 || itemID > 5_042_000 {
			return nil, false
		}
		_, known := farmRecipes[itemID]
		hasKnownRecipe = hasKnownRecipe || known
		recipes = append(recipes, itemID)
	}
	return recipes, hasKnownRecipe
}

func findFarmRecipeList(message packet.Message) (int, []uint32, bool) {
	for index, elem := range message {
		if elem.Type() != packet.MessageElemTypeString {
			continue
		}
		if recipes, ok := parseFarmRecipeList(elem.Data().(string)); ok {
			return index, recipes, true
		}
	}
	return -1, nil, false
}

func addFarmObservedReward(order *FarmDeliveryState, itemID uint32, count int) {
	if count <= 0 {
		return
	}
	switch itemID {
	case 5_300_264:
		order.CoinReward += count
	case 5_041_043:
		order.KeyReward += count
	default:
		name := farmRewardMaterialNames[itemID]
		if name == "" {
			name = strconv.FormatUint(uint64(itemID), 10)
		}
		if order.MaterialRewards == nil {
			order.MaterialRewards = make(map[string]int)
		}
		order.MaterialRewards[name] += count
	}
}

func parseFarmRewardGroups(message packet.Message, index *int, order *FarmDeliveryState) bool {
	groupCount, ok := farmMessageUint(message, *index)
	if !ok || groupCount > 16 {
		return false
	}
	*index++
	for range groupCount {
		itemID, itemOK := farmMessageUint(message, *index)
		count, countOK := farmMessageUint(message, *index+1)
		if !itemOK || !countOK {
			return false
		}
		addFarmObservedReward(order, itemID, int(count))
		*index += 2
	}
	return true
}

func parseFarmDeliveryPayload(message packet.Message, recipeIndex int) ([]FarmDeliveryState, int, bool) {
	index := recipeIndex + 1
	orderCount, ok := farmMessageUint(message, index)
	if !ok || orderCount > 100 {
		return nil, index, false
	}
	index++
	orders := make([]FarmDeliveryState, 0, orderCount)
	for range orderCount {
		if index+3 >= len(message) || message[index+1].Type() != packet.MessageElemTypeLong {
			return nil, index, false
		}
		dbKey, dbKeyOK := farmMessageUint(message, index)
		startedAt, startedAtOK := farmMessageLong(message, index+1)
		remaining, remainingOK := farmMessageUint(message, index+2)
		if !dbKeyOK || !startedAtOK || !remainingOK || dbKey == 0 || dbKey > 10_000 {
			return nil, index, false
		}
		order := FarmDeliveryState{
			DBKey:               int(dbKey),
			StartedAt:           int64(startedAt),
			RemainingDeliveries: int(remaining),
			MaximumDeliveries:   farmDeliveryMaximums[int(dbKey)],
		}
		index += 3 // DBKey, next refresh timestamp, remaining deliveries.
		if !parseFarmRewardGroups(message, &index, &order) || !parseFarmRewardGroups(message, &index, &order) {
			return nil, index, false
		}
		orders = append(orders, order)
	}
	return orders, index, true
}

func parseFarmStorageCharacterData(message packet.Message) (map[string]int, bool) {
	if len(message) < 4 || message[1].Type() != packet.MessageElemTypeLong || message[3].Type() != packet.MessageElemTypeString {
		return nil, false
	}

	inventory := make(map[string]int)
	for _, elem := range message {
		if elem.Type() != packet.MessageElemTypeBin {
			continue
		}
		item, err := packet.EntityItemReader(elem.Data().([]byte))
		if err != nil || item.PocketType != farmStoragePocketType || !farmStorageItemIDSet[item.ItemId] {
			continue
		}
		inventory[strconv.FormatUint(uint64(item.ItemId), 10)] += int(item.Amount)
	}
	return inventory, true
}

func parseFarmStorageLevel(message packet.Message) (int, bool) {
	for _, elem := range message {
		if elem.Type() != packet.MessageElemTypeString {
			continue
		}
		var state struct {
			StorageLevel int `xml:"StorageLevel,attr"`
		}
		if xml.Unmarshal([]byte(elem.Data().(string)), &state) == nil && farmStorageCapacities[state.StorageLevel] > 0 {
			return state.StorageLevel, true
		}
	}
	return 0, false
}

type farmCraftStart struct {
	OutputItemID uint32 `xml:"CMSFPI,attr"`
	OutputAmount int    `xml:"CMSFPA,attr"`
	StartedAt    string `xml:"CMSFPT,attr"`
	MaterialIDs  string `xml:"CMSFPMT,attr"`
}

func parseFarmItemIDs(value string) ([]uint32, bool) {
	parts := strings.Split(value, ";")
	result := make([]uint32, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		itemID, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			return nil, false
		}
		result = append(result, uint32(itemID))
	}
	return result, len(result) > 0
}

func parseFarmCraftStart(message packet.Message) (farmCraftStart, []farmCatalogItem, bool) {
	for _, elem := range message {
		if elem.Type() != packet.MessageElemTypeString {
			continue
		}
		var state farmCraftStart
		if xml.Unmarshal([]byte(elem.Data().(string)), &state) != nil || state.OutputItemID == 0 || state.OutputAmount <= 0 || state.StartedAt == "0" || state.StartedAt == "" {
			continue
		}
		recipe, known := farmRecipes[state.OutputItemID]
		materialIDs, parsed := parseFarmItemIDs(state.MaterialIDs)
		if !known || !parsed || len(materialIDs) != len(recipe.Materials) {
			continue
		}
		countsByBaseID := make(map[uint32]int, len(recipe.Materials))
		for _, material := range recipe.Materials {
			countsByBaseID[canonicalFarmCropID(material.ItemID)] = material.Count
		}
		consumed := make([]farmCatalogItem, 0, len(materialIDs))
		for _, itemID := range materialIDs {
			count, found := countsByBaseID[canonicalFarmCropID(itemID)]
			if !found {
				return farmCraftStart{}, nil, false
			}
			consumed = append(consumed, farmCatalogItem{ItemID: itemID, Count: count * state.OutputAmount})
		}
		return state, consumed, true
	}
	return farmCraftStart{}, nil, false
}

func parseFarmItemNotice(message packet.Message) (uint32, bool) {
	if len(message) < 1 || message[0].Type() != packet.MessageElemTypeString {
		return 0, false
	}
	var notice struct {
		ItemID uint32 `xml:"classid,attr"`
	}
	if xml.Unmarshal([]byte(message[0].Data().(string)), &notice) != nil || !farmStorageItemIDSet[notice.ItemID] {
		return 0, false
	}
	return notice.ItemID, true
}

func parseFarmStorageNoticeCount(message packet.Message) (int, bool) {
	count, ok := farmMessageUint(message, 0)
	if !ok || count == 0 || count > 10_000 {
		return 0, false
	}
	return int(count), true
}

func updateInventoryFromFarmDeliveries(inventory map[string]int, previous, current []FarmDeliveryState, qualityIndexes []int) map[string]int {
	previousByOrder := make(map[int]FarmDeliveryState, len(previous))
	for _, order := range previous {
		previousByOrder[order.DBKey] = order
	}
	remainingInventory := exactFarmInventory(inventory)
	for _, order := range current {
		oldOrder, found := previousByOrder[order.DBKey]
		if !found {
			continue
		}
		if oldOrder.StartedAt != 0 && order.StartedAt == 0 && order.RemainingDeliveries < oldOrder.RemainingDeliveries {
			for name, count := range oldOrder.MaterialRewards {
				if itemID := farmRewardMaterialIDs[name]; itemID != 0 {
					remainingInventory[itemID] += count
				}
			}
			continue
		}
		if oldOrder.StartedAt != 0 || order.StartedAt == 0 {
			continue
		}
		definition, known := farmDeliveryCatalog[order.DBKey]
		if !known {
			continue
		}
		for _, required := range definition.RequiredItems {
			if _, crop := farmCropNames[canonicalFarmCropID(required.ItemID)]; crop {
				reserveFarmCrop(remainingInventory, required.ItemID, required.Count, qualityIndexes)
				continue
			}
			remainingInventory[required.ItemID] -= min(remainingInventory[required.ItemID], required.Count)
		}
	}
	result := make(map[string]int, len(remainingInventory))
	for itemID, count := range remainingInventory {
		if count > 0 {
			result[strconv.FormatUint(uint64(itemID), 10)] = count
		}
	}
	return result
}

func (m *farmRecommendationManager) saveInventoryLocked(at time.Time) {
	m.data.SyncedAt = at.UnixMilli()
	if err := m.saveLocked(); err != nil {
		logger.Printf("[FarmRecommendation] 保存同步数据失败: %v", err)
	}
}

func (m *farmRecommendationManager) syncInventoryDelta(pkt *packet.GamePacket) bool {
	switch pkt.Op {
	case opcodeItemNotice:
		itemID, ok := parseFarmItemNotice(pkt.Msg)
		if !ok {
			m.mu.Lock()
			m.pendingStorageItemID = 0
			m.pendingStorageItemAt = time.Time{}
			m.mu.Unlock()
			return false
		}
		m.mu.Lock()
		m.pendingStorageItemID = itemID
		m.pendingStorageItemAt = pkt.At
		m.mu.Unlock()
		return true
	case opcodeFarmStorageNotice:
		count, ok := parseFarmStorageNoticeCount(pkt.Msg)
		if !ok {
			return false
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		if !m.data.InventorySynced || m.pendingStorageItemID == 0 || pkt.At.Sub(m.pendingStorageItemAt) < 0 || pkt.At.Sub(m.pendingStorageItemAt) > 3*time.Second {
			m.pendingStorageItemID = 0
			m.pendingStorageItemAt = time.Time{}
			return true
		}
		key := strconv.FormatUint(uint64(m.pendingStorageItemID), 10)
		m.data.Inventory[key] += count
		m.pendingStorageItemID = 0
		m.pendingStorageItemAt = time.Time{}
		m.saveInventoryLocked(pkt.At)
		return true
	case opcodeFarmCropState:
		state, consumed, ok := parseFarmCraftStart(pkt.Msg)
		if !ok {
			return false
		}
		key := fmt.Sprintf("%d:%d:%s", pkt.Id, state.OutputItemID, state.StartedAt)
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.seenCraftStarts == nil {
			m.seenCraftStarts = make(map[string]bool)
		}
		if m.seenCraftStarts[key] {
			return true
		}
		m.seenCraftStarts[key] = true
		if !m.data.InventorySynced {
			return true
		}
		inventory := exactFarmInventory(m.data.Inventory)
		for _, material := range consumed {
			inventory[material.ItemID] -= min(inventory[material.ItemID], material.Count)
		}
		m.data.Inventory = farmInventoryStrings(inventory)
		m.saveInventoryLocked(pkt.At)
		return true
	default:
		return false
	}
}

func farmInventoryStrings(inventory map[uint32]int) map[string]int {
	result := make(map[string]int, len(inventory))
	for itemID, count := range inventory {
		if count > 0 {
			result[strconv.FormatUint(uint64(itemID), 10)] = count
		}
	}
	return result
}

func (m *farmRecommendationManager) syncPacket(pkt *packet.GamePacket) {
	if pkt.Op == opcodeCharacterData {
		character, err := packet.ParseCharacterDataPacket(pkt)
		if err != nil || character.Id == 0 || !isPC(int(character.RaceId)) {
			return
		}
		inventory, ok := parseFarmStorageCharacterData(pkt.Msg)
		if !ok {
			return
		}
		characterID := strconv.FormatUint(character.Id, 10)
		m.mu.Lock()
		if m.data.CharacterID != "" && m.data.CharacterID != characterID {
			m.data = resetFarmRecommendationDataForCharacter(m.data, characterID)
			m.lastRanks = nil
			m.deliveryRanks = nil
		} else {
			m.data.CharacterID = characterID
		}
		m.data.Inventory = inventory
		m.data.InventorySynced = true
		m.pendingStorageItemID = 0
		m.pendingStorageItemAt = time.Time{}
		m.saveInventoryLocked(pkt.At)
		m.mu.Unlock()
		return
	}
	if m.syncInventoryDelta(pkt) {
		return
	}
	if pkt.Op != opcodeFarmSnapshot && pkt.Op != opcodeFarmSummary && pkt.Op != opcodeFarmDelivery {
		return
	}
	recipeIndex, recipes, ok := findFarmRecipeList(pkt.Msg)
	if !ok {
		return
	}

	var orders []FarmDeliveryState
	ordersSynced := false
	storageLevel, storageLevelSynced := parseFarmStorageLevel(pkt.Msg)
	if pkt.Op == opcodeFarmSnapshot || pkt.Op == opcodeFarmDelivery {
		orders, _, ordersSynced = parseFarmDeliveryPayload(pkt.Msg, recipeIndex)
		if !ordersSynced {
			return
		}
	}

	m.mu.Lock()
	m.data.UnlockedRecipes = recipes
	m.data.RecipesSynced = true
	if storageLevelSynced {
		m.data.StorageLevel = storageLevel
	}
	if ordersSynced {
		if pkt.Op == opcodeFarmDelivery && m.data.InventorySynced {
			m.data.Inventory = updateInventoryFromFarmDeliveries(m.data.Inventory, m.data.Orders, orders, enabledFarmQualityIndexes(m.data))
			m.pendingStorageItemID = 0
			m.pendingStorageItemAt = time.Time{}
		}
		m.data.Orders = orders
		m.data.OrdersSynced = true
	}
	m.data.SyncedAt = pkt.At.UnixMilli()
	if err := m.saveLocked(); err != nil {
		logger.Printf("[FarmRecommendation] 保存同步数据失败: %v", err)
	}
	m.mu.Unlock()
}

func (m *farmRecommendationManager) recordPacket(pkt *packet.GamePacket) {
	if pkt == nil || pkt.Msg == nil {
		return
	}
	m.syncPacket(pkt)
	if skipFarmRecommendationCaptureOpcode(pkt.Op) {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.captureEnabled || m.captureFile == nil {
		return
	}
	if time.Since(m.captureStartedAt) > farmRecommendationCaptureDuration || m.captureBytes >= farmRecommendationCaptureMaxBytes {
		m.captureEnabled = false
		_ = m.captureFile.Close()
		m.captureFile = nil
		return
	}
	record := farmPacketCaptureRecord{
		Type:     "packet",
		At:       pkt.At.UnixMilli(),
		Opcode:   fmt.Sprintf("0x%x", pkt.Op),
		EntityID: strconv.FormatUint(pkt.Id, 10),
		Elements: make([]farmPacketCaptureElement, 0, len(pkt.Msg)),
	}
	for _, elem := range pkt.Msg {
		record.Elements = append(record.Elements, farmPacketCaptureElement{Type: uint8(elem.Type()), Value: farmCaptureValue(elem)})
	}
	line, err := json.Marshal(record)
	if err != nil {
		return
	}
	line = append(line, '\n')
	n, err := m.captureFile.Write(line)
	if err == nil {
		m.captureBytes += int64(n)
	}
}

var farmCropNames = map[uint32]string{
	5_040_996: "黑莓",
	5_040_997: "秋葵",
	5_040_998: "茉莉",
	5_040_999: "红梨",
	5_041_000: "橡胶",
	5_041_001: "魔法蜘蛛丝",
	5_041_002: "石英",
}

var farmStorageCapacities = map[int]int{
	1: 1_000,
	2: 3_000,
	3: 6_000,
	4: 9_000,
	5: 13_000,
	6: 17_000,
	7: 21_000,
}

var farmCropIconNames = map[uint32]string{
	5_040_996: "item_seasonalfarming_06_s",
	5_040_997: "item_seasonalfarming_05_s",
	5_040_998: "item_seasonalfarming_07_s",
	5_040_999: "item_seasonalfarming_08_s",
	5_041_000: "item_seasonalfarming_09_s",
	5_041_001: "item_seasonalfarming_04_s",
	5_041_002: "item_seasonalfarming_10_s",
	5_041_029: "item_seasonalfarming_06_h",
	5_041_030: "item_seasonalfarming_05_h",
	5_041_031: "item_seasonalfarming_07_h",
	5_041_032: "item_seasonalfarming_08_h",
	5_041_033: "item_seasonalfarming_09_h",
	5_041_034: "item_seasonalfarming_04_h",
	5_041_035: "item_seasonalfarming_10_h",
	5_041_036: "item_seasonalfarming_06_p",
	5_041_037: "item_seasonalfarming_05_p",
	5_041_038: "item_seasonalfarming_07_p",
	5_041_039: "item_seasonalfarming_08_p",
	5_041_040: "item_seasonalfarming_09_p",
	5_041_041: "item_seasonalfarming_04_p",
	5_041_042: "item_seasonalfarming_10_p",
}

func farmStorageItemQuality(itemID uint32) string {
	variants := farmCropQualityVariants[canonicalFarmCropID(itemID)]
	for index, variant := range variants {
		if variant != itemID {
			continue
		}
		switch index {
		case 1:
			return "advanced"
		case 2:
			return "highest"
		default:
			return "normal"
		}
	}
	return "none"
}

func farmStorageItemCategory(itemID uint32) string {
	if _, crop := farmQualityCropIDs[itemID]; crop {
		return "crop"
	}
	if _, product := farmRecipes[itemID]; product {
		return "product"
	}
	return "upgrade"
}

func farmStorageItemIcon(itemID uint32) string {
	if icon := farmCropIconNames[itemID]; icon != "" {
		return icon
	}
	if itemID >= 5_041_003 && itemID <= 5_041_022 {
		return fmt.Sprintf("item_seasonalfarming_%02d", 15+itemID-5_041_003)
	}
	switch itemID {
	case 5_041_044:
		return "item_brick"
	case 5_041_045:
		return "item_metalplate"
	case 5_041_046:
		return "item_paint"
	case 5_041_047:
		return "item_ice_piece"
	default:
		return ""
	}
}

var farmCropDurations = map[uint32]int64{
	5_040_996: 12 * 60,
	5_040_997: 18 * 60,
	5_040_998: 31*60 + 30,
	5_040_999: 21*60 + 30,
	5_041_000: 24 * 60,
	5_041_001: 16*60 + 30,
	5_041_002: 18 * 60,
}

var farmQualityCropIDs = map[uint32]uint32{
	5_040_996: 5_040_996, 5_041_029: 5_040_996, 5_041_036: 5_040_996,
	5_040_997: 5_040_997, 5_041_030: 5_040_997, 5_041_037: 5_040_997,
	5_040_998: 5_040_998, 5_041_031: 5_040_998, 5_041_038: 5_040_998,
	5_040_999: 5_040_999, 5_041_032: 5_040_999, 5_041_039: 5_040_999,
	5_041_000: 5_041_000, 5_041_033: 5_041_000, 5_041_040: 5_041_000,
	5_041_001: 5_041_001, 5_041_034: 5_041_001, 5_041_041: 5_041_001,
	5_041_002: 5_041_002, 5_041_035: 5_041_002, 5_041_042: 5_041_002,
}

var farmCropQualityVariants = map[uint32][]uint32{
	5_040_996: {5_040_996, 5_041_029, 5_041_036},
	5_040_997: {5_040_997, 5_041_030, 5_041_037},
	5_040_998: {5_040_998, 5_041_031, 5_041_038},
	5_040_999: {5_040_999, 5_041_032, 5_041_039},
	5_041_000: {5_041_000, 5_041_033, 5_041_040},
	5_041_001: {5_041_001, 5_041_034, 5_041_041},
	5_041_002: {5_041_002, 5_041_035, 5_041_042},
}

type farmOrderRewardRange struct {
	CoinMin, CoinMax         int
	KeyMin, KeyMax           int
	MaterialMin, MaterialMax int
}

var farmOrderRewardRanges = func() map[int]farmOrderRewardRange {
	ranges := make(map[int]farmOrderRewardRange, 30)
	assign := func(keys []int, value farmOrderRewardRange) {
		for _, key := range keys {
			ranges[key] = value
		}
	}
	assign([]int{1, 2, 3, 4}, farmOrderRewardRange{CoinMin: 50, CoinMax: 70, MaterialMin: 1, MaterialMax: 1})
	assign([]int{5, 6}, farmOrderRewardRange{CoinMin: 100, CoinMax: 250, KeyMin: 1, KeyMax: 1})
	assign([]int{7, 8, 9}, farmOrderRewardRange{CoinMin: 150, CoinMax: 180, MaterialMin: 1, MaterialMax: 1})
	assign([]int{10, 11, 12, 13}, farmOrderRewardRange{CoinMin: 150, CoinMax: 180, KeyMin: 1, KeyMax: 1})
	assign([]int{14, 15, 16, 17, 18, 19, 20}, farmOrderRewardRange{CoinMin: 200, CoinMax: 410, KeyMin: 1, KeyMax: 1, MaterialMin: 1, MaterialMax: 1})
	assign([]int{21, 22, 23, 24, 25, 26}, farmOrderRewardRange{CoinMin: 200, CoinMax: 750, KeyMin: 1, KeyMax: 2, MaterialMin: 1, MaterialMax: 2})
	assign([]int{27, 28, 29, 30}, farmOrderRewardRange{CoinMin: 300, CoinMax: 1000, KeyMin: 1, KeyMax: 3, MaterialMin: 1, MaterialMax: 3})
	return ranges
}()

var farmRecipes = map[uint32]farmRecipeDefinition{
	5_041_003: {Name: "黑莓汁", CraftSeconds: 60, Factory: "abundant", Materials: []farmCatalogItem{{5_040_998, 1}, {5_040_996, 1}}},
	5_041_004: {Name: "甜蜜蛋糕", CraftSeconds: 120, Factory: "abundant", Materials: []farmCatalogItem{{5_040_996, 1}, {5_040_999, 1}}},
	5_041_005: {Name: "红梨果酱", CraftSeconds: 180, Factory: "abundant", Materials: []farmCatalogItem{{5_040_997, 1}, {5_040_999, 1}}},
	5_041_006: {Name: "星星色拉", CraftSeconds: 240, Factory: "abundant", Materials: []farmCatalogItem{{5_040_999, 1}, {5_040_996, 1}, {5_040_997, 2}}},
	5_041_007: {Name: "茉莉香水", CraftSeconds: 300, Factory: "abundant", Materials: []farmCatalogItem{{5_040_997, 1}, {5_040_996, 1}, {5_040_998, 2}}},
	5_041_008: {Name: "紫色布料", CraftSeconds: 60, Factory: "gentle", Materials: []farmCatalogItem{{5_041_001, 1}, {5_040_996, 1}}},
	5_041_009: {Name: "花纹连衣裙", CraftSeconds: 120, Factory: "gentle", Materials: []farmCatalogItem{{5_040_998, 1}, {5_041_001, 1}}},
	5_041_010: {Name: "防水布料", CraftSeconds: 180, Factory: "gentle", Materials: []farmCatalogItem{{5_041_001, 1}, {5_041_000, 1}}},
	5_041_011: {Name: "强化纤维", CraftSeconds: 240, Factory: "gentle", Materials: []farmCatalogItem{{5_040_997, 1}, {5_041_000, 1}, {5_041_001, 2}}},
	5_041_012: {Name: "晚礼服", CraftSeconds: 300, Factory: "gentle", Materials: []farmCatalogItem{{5_040_998, 1}, {5_041_002, 1}, {5_041_001, 2}}},
	5_041_013: {Name: "红月耳环", CraftSeconds: 60, Factory: "shining", Materials: []farmCatalogItem{{5_041_002, 1}, {5_040_999, 1}}},
	5_041_014: {Name: "纯净花漾发夹", CraftSeconds: 120, Factory: "shining", Materials: []farmCatalogItem{{5_041_002, 1}, {5_040_998, 1}}},
	5_041_015: {Name: "石英粉", CraftSeconds: 180, Factory: "shining", Materials: []farmCatalogItem{{5_041_001, 1}, {5_041_002, 1}}},
	5_041_016: {Name: "午夜珍珠油漆", CraftSeconds: 240, Factory: "shining", Materials: []farmCatalogItem{{5_040_996, 1}, {5_041_000, 1}, {5_041_002, 2}}},
	5_041_017: {Name: "装饰用水晶剑", CraftSeconds: 300, Factory: "shining", Materials: []farmCatalogItem{{5_041_000, 1}, {5_040_997, 1}, {5_041_002, 2}}},
	5_041_018: {Name: "强力粘合剂", CraftSeconds: 60, Factory: "delicate", Materials: []farmCatalogItem{{5_041_000, 1}, {5_041_001, 1}}},
	5_041_019: {Name: "天然橡胶", CraftSeconds: 120, Factory: "delicate", Materials: []farmCatalogItem{{5_040_997, 1}, {5_041_000, 1}}},
	5_041_020: {Name: "金盏花工艺箱", CraftSeconds: 180, Factory: "delicate", Materials: []farmCatalogItem{{5_040_998, 1}, {5_041_000, 1}}},
	5_041_021: {Name: "黄昏鲁特琴", CraftSeconds: 240, Factory: "delicate", Materials: []farmCatalogItem{{5_040_996, 1}, {5_040_998, 1}, {5_040_999, 2}}},
	5_041_022: {Name: "黎明之弓", CraftSeconds: 300, Factory: "delicate", Materials: []farmCatalogItem{{5_041_002, 1}, {5_040_997, 1}, {5_040_999, 2}}},
}

var farmAlwaysUnlockedRecipes = map[uint32]bool{
	5_041_003: true,
	5_041_008: true,
	5_041_013: true,
	5_041_018: true,
}

var farmDeliveryMaximums = map[int]int{
	1: 10, 2: 10, 3: 10, 4: 10,
	5: 3, 6: 3,
	7: 10, 8: 10, 9: 10,
	10: 7, 11: 7, 12: 7, 13: 7, 14: 7, 15: 7, 16: 7, 17: 7, 18: 7, 19: 7, 20: 7,
	21: 5, 22: 5, 23: 5, 24: 5, 25: 5, 26: 5, 27: 5, 28: 5, 29: 5, 30: 5,
}

const farmStoragePocketType uint32 = 307

// These are the items allowed in pocket 307, matching Storage/Sort in SeasonalFarming.xml.
var farmStorageItemIDs = []uint32{
	5_040_996, 5_041_029, 5_041_036,
	5_040_997, 5_041_030, 5_041_037,
	5_040_998, 5_041_031, 5_041_038,
	5_040_999, 5_041_032, 5_041_039,
	5_041_000, 5_041_033, 5_041_040,
	5_041_001, 5_041_034, 5_041_041,
	5_041_002, 5_041_035, 5_041_042,
	5_041_003, 5_041_004, 5_041_005, 5_041_006, 5_041_007,
	5_041_008, 5_041_009, 5_041_010, 5_041_011, 5_041_012,
	5_041_013, 5_041_014, 5_041_015, 5_041_016, 5_041_017,
	5_041_018, 5_041_019, 5_041_020, 5_041_021, 5_041_022,
	5_041_044, 5_041_045, 5_041_046, 5_041_047,
}

var farmStorageItemIDSet = func() map[uint32]bool {
	result := make(map[uint32]bool, len(farmStorageItemIDs))
	for _, itemID := range farmStorageItemIDs {
		result[itemID] = true
	}
	return result
}()

var farmRewardMaterialNames = map[uint32]string{
	5_041_044: "储存库升级用砖块",
	5_041_045: "储存库升级用铁板",
	5_041_046: "储存库升级用涂料",
	5_041_047: "储存库升级用玻璃",
}

var farmRewardMaterialIDs = func() map[string]uint32 {
	result := make(map[string]uint32, len(farmRewardMaterialNames))
	for itemID, name := range farmRewardMaterialNames {
		result[name] = itemID
	}
	return result
}()

func farmItems(values ...int) []farmCatalogItem {
	items := make([]farmCatalogItem, 0, len(values)/2)
	for index := 0; index+1 < len(values); index += 2 {
		items = append(items, farmCatalogItem{ItemID: uint32(values[index]), Count: values[index+1]})
	}
	return items
}

var farmDeliveryCatalog = map[int]farmDeliveryDefinition{
	1:  {1, "敦巴伦学校老师的订单", 180, farmItems(5040996, 7, 5040998, 5)},
	2:  {2, "龙遗迹考古学家的订单", 180, farmItems(5040997, 4, 5041001, 2)},
	3:  {3, "班格铁匠的订单", 180, farmItems(5040999, 2, 5041002, 2)},
	4:  {4, "迪尔科内尔居民的订单", 180, farmItems(5041000, 3, 5040996, 6)},
	5:  {5, "塔汀画家的订单", 300, farmItems(5041003, 2, 5041013, 2)},
	6:  {6, "考利芙峡谷旅行向导的订单", 300, farmItems(5041008, 2, 5041018, 2)},
	7:  {7, "黑莓集中订单", 180, farmItems(5040996, 18)},
	8:  {8, "秋葵集中订单", 180, farmItems(5040997, 12)},
	9:  {9, "茉莉集中订单", 180, farmItems(5040998, 9)},
	10: {10, "红梨集中订单", 300, farmItems(5040999, 6)},
	11: {11, "橡胶集中订单", 300, farmItems(5041000, 6)},
	12: {12, "魔法蜘蛛丝集中订单", 300, farmItems(5041001, 3)},
	13: {13, "石英集中订单", 300, farmItems(5041002, 3)},
	14: {14, "杜加德走廊木匠的订单", 300, farmItems(5040996, 1, 5041008, 2, 5041005, 2)},
	15: {15, "斯利比矿工的订单", 300, farmItems(5040997, 1, 5041018, 1, 5041010, 2)},
	16: {16, "列扎尔酿造厂管理员的订单", 300, farmItems(5040998, 2, 5041013, 2, 5041004, 1)},
	17: {17, "塔汀古怪炼金术士的订单", 300, farmItems(5040999, 1, 5041003, 1, 5041015, 2)},
	18: {18, "凯安港口船员的订单", 300, farmItems(5041000, 2, 5041019, 1, 5041006, 1)},
	19: {19, "仙魔商区店员的订单", 300, farmItems(5041001, 2, 5041009, 1, 5041020, 1)},
	20: {20, "班格钟表匠人的订单", 300, farmItems(5041002, 1, 5041014, 1, 5041011, 1)},
	21: {21, "艾明马恰室内装饰专家的订单", 300, farmItems(5041008, 1, 5041016, 2, 5041010, 1)},
	22: {22, "吟游诗人营地流浪者的订单", 300, farmItems(5041014, 2, 5041021, 1, 5041015, 1)},
	23: {23, "敦巴伦居民的订单", 300, farmItems(5041013, 1, 5041022, 1, 5041020, 1)},
	24: {24, "奥斯纳赛看山人的订单", 300, farmItems(5041004, 1, 5041012, 2, 5041005, 2)},
	25: {25, "迪尔科内尔行商的订单", 300, farmItems(5041018, 2, 5041017, 1, 5041019, 2)},
	26: {26, "卡普港口服饰设计师的订单", 300, farmItems(5041003, 2, 5041007, 1, 5041009, 1)},
	27: {27, "凯安港口贸易事务员的订单", 600, farmItems(5041006, 2, 5041022, 2, 5041012, 2)},
	28: {28, "阿布内尔商区店员的订单", 600, farmItems(5041011, 2, 5041017, 2, 5041021, 2)},
	29: {29, "塔拉“大佬”的订单", 600, farmItems(5041012, 2, 5041016, 2, 5041007, 2)},
	30: {30, "拉赫王城侍从的订单", 600, farmItems(5041007, 2, 5041017, 2, 5041022, 2)},
}

func canonicalFarmCropID(itemID uint32) uint32 {
	if canonical, ok := farmQualityCropIDs[itemID]; ok {
		return canonical
	}
	return itemID
}

func farmItemName(itemID uint32) string {
	if name := farmCropNames[canonicalFarmCropID(itemID)]; name != "" {
		return name
	}
	if recipe, ok := farmRecipes[itemID]; ok {
		return recipe.Name
	}
	return strconv.FormatUint(uint64(itemID), 10)
}

func farmRecommendationItemName(itemID uint32) string {
	baseID := canonicalFarmCropID(itemID)
	name := farmCropNames[baseID]
	if name == "" || baseID == itemID {
		return farmItemName(itemID)
	}
	variants := farmCropQualityVariants[baseID]
	if len(variants) == 3 {
		switch itemID {
		case variants[1]:
			return "高级" + name
		case variants[2]:
			return "最高级" + name
		}
	}
	return name
}

func exactFarmInventory(raw map[string]int) map[uint32]int {
	result := make(map[uint32]int)
	for key, count := range raw {
		itemID, err := strconv.ParseUint(key, 10, 32)
		if err == nil && count > 0 {
			result[uint32(itemID)] += count
		}
	}
	return result
}

func normalizedFarmInventory(raw map[string]int) map[uint32]int {
	result := make(map[uint32]int)
	for key, count := range raw {
		itemID, err := strconv.ParseUint(key, 10, 32)
		if err != nil || count <= 0 {
			continue
		}
		result[canonicalFarmCropID(uint32(itemID))] += count
	}
	return result
}

func consumeFarmItem(inventory map[uint32]int, itemID uint32, count int) int {
	itemID = canonicalFarmCropID(itemID)
	available := inventory[itemID]
	used := min(available, count)
	inventory[itemID] -= used
	return count - used
}

func farmPlotCropID(plot FarmPlotState) uint32 {
	var baseID uint32
	switch plot.ItemID {
	case 5_040_989, 5_041_232:
		baseID = 5_040_996
	case 5_040_990, 5_041_233:
		baseID = 5_040_997
	case 5_040_991, 5_041_234:
		baseID = 5_040_998
	case 5_040_992, 5_041_235:
		baseID = 5_040_999
	case 5_040_993, 5_041_236:
		baseID = 5_041_000
	case 5_040_994, 5_041_237:
		baseID = 5_041_001
	case 5_040_995, 5_041_238:
		baseID = 5_041_002
	default:
		baseID = canonicalFarmCropID(plot.ItemID)
	}
	variants := farmCropQualityVariants[baseID]
	if len(variants) != 3 {
		return baseID
	}
	switch plot.Quality {
	case "advanced":
		return variants[1]
	case "highest":
		return variants[2]
	default:
		return variants[0]
	}
}

func enabledFarmQualityIndexes(data FarmRecommendationData) []int {
	indexes := make([]int, 0, 3)
	if data.UseNormalMaterials {
		indexes = append(indexes, 0)
	}
	if data.UseAdvancedMaterials {
		indexes = append(indexes, 1)
	}
	if data.UseHighestMaterials {
		indexes = append(indexes, 2)
	}
	return indexes
}

func reserveFarmCrop(inventory map[uint32]int, baseID uint32, count int, qualityIndexes []int) (uint32, int) {
	variants := farmCropQualityVariants[canonicalFarmCropID(baseID)]
	if len(variants) != 3 || len(qualityIndexes) == 0 {
		return baseID, count
	}
	selected := variants[qualityIndexes[0]]
	for _, qualityIndex := range qualityIndexes {
		candidate := variants[qualityIndex]
		if inventory[candidate] >= count {
			selected = candidate
			break
		}
		if inventory[candidate] > inventory[selected] {
			selected = candidate
		}
	}
	used := min(inventory[selected], count)
	inventory[selected] -= used
	return selected, count - used
}

func farmCropKind(itemID uint32) string {
	switch canonicalFarmCropID(itemID) {
	case 5_040_996, 5_040_997, 5_040_998:
		return "field"
	case 5_040_999:
		return "redPear"
	case 5_041_000:
		return "rubber"
	case 5_041_001:
		return "spider"
	case 5_041_002:
		return "quartz"
	default:
		return ""
	}
}

func scheduleFarmJobs(loads []int64, jobs []int64) (int64, bool) {
	if len(jobs) == 0 {
		return 0, true
	}
	if len(loads) == 0 {
		return 0, false
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i] > jobs[j] })
	var completion int64
	for _, job := range jobs {
		index := 0
		for candidate := 1; candidate < len(loads); candidate++ {
			if loads[candidate] < loads[index] {
				index = candidate
			}
		}
		loads[index] += job
		completion = max(completion, loads[index])
	}
	return completion, true
}

func estimateFarmGrowth(needs map[uint32]int, farm FarmState) (int64, map[uint32]int, []string) {
	remaining := make(map[uint32]int, len(needs))
	for itemID, count := range needs {
		remaining[itemID] = count
	}
	loadsByKind := make(map[string][]int64)
	var completion int64
	for _, plot := range farm.Plots {
		if !plot.Unlocked {
			continue
		}
		load := plot.RemainingSeconds
		if plot.Ready {
			load = 0
		}
		loadsByKind[plot.Kind] = append(loadsByKind[plot.Kind], load)
		cropID := farmPlotCropID(plot)
		if plot.Planted && remaining[cropID] > 0 {
			remaining[cropID]--
			completion = max(completion, load)
		}
	}

	warnings := make([]string, 0)
	jobsByKind := make(map[string][]int64)
	for itemID, count := range remaining {
		if count <= 0 {
			continue
		}
		baseID := canonicalFarmCropID(itemID)
		kind := farmCropKind(baseID)
		if kind == "" {
			continue
		}
		for range count {
			jobsByKind[kind] = append(jobsByKind[kind], farmCropDurations[baseID])
		}
	}
	for kind, jobs := range jobsByKind {
		itemCompletion, ok := scheduleFarmJobs(append([]int64(nil), loadsByKind[kind]...), jobs)
		if ok {
			completion = max(completion, itemCompletion)
			continue
		}
		if kind == "field" {
			warnings = append(warnings, "没有可用的耕地")
		} else {
			warnings = append(warnings, fmt.Sprintf("%s采集设施尚未解锁", kind))
		}
	}
	return completion, remaining, warnings
}

func sortedFarmAmounts(values map[uint32]int) []FarmRecommendationAmount {
	result := make([]FarmRecommendationAmount, 0, len(values))
	for itemID, count := range values {
		if count <= 0 {
			continue
		}
		result = append(result, FarmRecommendationAmount{
			ItemID:   itemID,
			Name:     farmRecommendationItemName(itemID),
			Count:    count,
			PlotKind: farmCropKind(itemID),
			Factory:  farmRecipes[itemID].Factory,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ItemID < result[j].ItemID })
	return result
}

func shouldRefreshFarmReward(current, minimum, maximum, remainingDeliveries int) bool {
	if remainingDeliveries <= 1 || current <= 0 || minimum >= maximum {
		return false
	}
	expectedAfterRefresh := float64(remainingDeliveries-1) * float64(minimum+maximum) / 2
	currentTotal := float64(remainingDeliveries * current)
	return expectedAfterRefresh > currentTotal
}

func farmGameClockMilliseconds(now time.Time) int64 {
	_, offsetSeconds := now.Zone()
	return now.UnixMilli() + dotNetUnixEpochMilliseconds + int64(offsetSeconds)*1000
}

func farmDeliveryProgress(startedAt, deliverySeconds int64, now time.Time) (string, int64) {
	if startedAt <= 0 {
		return "", 0
	}
	remainingMilliseconds := startedAt + deliverySeconds*1000 - farmGameClockMilliseconds(now)
	if remainingMilliseconds <= 0 {
		return "completed", 0
	}
	return "delivering", (remainingMilliseconds + 999) / 1000
}

func (m *farmRecommendationManager) recommendLocked(farm FarmState) []FarmOrderRecommendation {
	if !m.data.OrdersSynced || !m.data.InventorySynced {
		return []FarmOrderRecommendation{}
	}
	qualityIndexes := enabledFarmQualityIndexes(m.data)
	results := make([]FarmOrderRecommendation, 0, len(m.data.Orders))
	for _, current := range m.data.Orders {
		if current.RemainingDeliveries <= 0 {
			continue
		}
		definition, known := farmDeliveryCatalog[current.DBKey]
		if !known {
			continue
		}
		rewardRange := farmOrderRewardRanges[current.DBKey]
		target := current.KeyReward
		if m.data.Goal == farmGoalCoins {
			target = current.CoinReward
		}
		if current.StartedAt != 0 {
			status, remaining := farmDeliveryProgress(current.StartedAt, definition.DeliverySeconds, time.Now())
			rate := float64(0)
			if target > 0 && definition.DeliverySeconds > 0 {
				rate = math.Round(float64(target)*3600/float64(definition.DeliverySeconds)*1000) / 1000
			}
			frozenRank := m.deliveryRanks[current.DBKey]
			if frozenRank == 0 {
				frozenRank = m.lastRanks[current.DBKey]
			}
			results = append(results, FarmOrderRecommendation{
				Rank:                frozenRank,
				DBKey:               current.DBKey,
				Name:                definition.Name,
				RemainingDeliveries: current.RemainingDeliveries,
				MaximumDeliveries:   current.MaximumDeliveries,
				CoinReward:          current.CoinReward,
				CoinRewardMin:       rewardRange.CoinMin,
				CoinRewardMax:       rewardRange.CoinMax,
				KeyReward:           current.KeyReward,
				KeyRewardMin:        rewardRange.KeyMin,
				KeyRewardMax:        rewardRange.KeyMax,
				MaterialRewards:     current.MaterialRewards,
				MaterialRewardMin:   rewardRange.MaterialMin,
				MaterialRewardMax:   rewardRange.MaterialMax,
				DeliveryStatus:      status,
				DeliveryRemaining:   remaining,
				EstimatedSeconds:    remaining,
				DeliverySeconds:     definition.DeliverySeconds,
				TargetPerHour:       rate,
			})
			continue
		}
		if target <= 0 {
			continue
		}

		inventory := exactFarmInventory(m.data.Inventory)
		cropNeeds := make(map[uint32][]int)
		productionNeeds := make(map[uint32]int)
		craftByFactory := make(map[string]int64)
		for _, required := range definition.RequiredItems {
			if _, crop := farmCropNames[canonicalFarmCropID(required.ItemID)]; crop {
				baseID := canonicalFarmCropID(required.ItemID)
				cropNeeds[baseID] = append(cropNeeds[baseID], required.Count)
				continue
			}
			available := inventory[required.ItemID]
			used := min(available, required.Count)
			inventory[required.ItemID] -= used
			remaining := required.Count - used
			if remaining == 0 {
				continue
			}
			recipe, hasRecipe := farmRecipes[required.ItemID]
			if !hasRecipe {
				continue
			}
			productionNeeds[required.ItemID] += remaining
			craftByFactory[recipe.Factory] += recipe.CraftSeconds * int64(remaining)
			for range remaining {
				for _, material := range recipe.Materials {
					baseID := canonicalFarmCropID(material.ItemID)
					cropNeeds[baseID] = append(cropNeeds[baseID], material.Count)
				}
			}
		}
		plantNeeds := make(map[uint32]int)
		for itemID, groups := range cropNeeds {
			sort.Slice(groups, func(i, j int) bool { return groups[i] > groups[j] })
			for _, count := range groups {
				selectedID, remaining := reserveFarmCrop(inventory, itemID, count, qualityIndexes)
				if remaining > 0 {
					plantNeeds[selectedID] += remaining
				}
			}
		}

		growSeconds, plantNeeds, warnings := estimateFarmGrowth(plantNeeds, farm)
		var craftSeconds int64
		for _, duration := range craftByFactory {
			craftSeconds = max(craftSeconds, duration)
		}
		eligible := true
		for _, warning := range warnings {
			if warning == "没有可用的耕地" || strings.HasSuffix(warning, "尚未解锁") {
				eligible = false
			}
		}
		estimated := growSeconds + craftSeconds + definition.DeliverySeconds
		if estimated <= 0 {
			estimated = definition.DeliverySeconds
		}
		rate := float64(target) * 3600 / float64(estimated)
		if !eligible {
			rate = 0
		}
		refreshRecommended := shouldRefreshFarmReward(current.KeyReward, rewardRange.KeyMin, rewardRange.KeyMax, current.RemainingDeliveries)
		if m.data.Goal == farmGoalCoins {
			refreshRecommended = shouldRefreshFarmReward(current.CoinReward, rewardRange.CoinMin, rewardRange.CoinMax, current.RemainingDeliveries)
		}
		requiresCrafting := len(productionNeeds) > 0
		requiresPlanting := len(plantNeeds) > 0
		results = append(results, FarmOrderRecommendation{
			DBKey:                current.DBKey,
			Name:                 definition.Name,
			RemainingDeliveries:  current.RemainingDeliveries,
			MaximumDeliveries:    current.MaximumDeliveries,
			CoinReward:           current.CoinReward,
			CoinRewardMin:        rewardRange.CoinMin,
			CoinRewardMax:        rewardRange.CoinMax,
			KeyReward:            current.KeyReward,
			KeyRewardMin:         rewardRange.KeyMin,
			KeyRewardMax:         rewardRange.KeyMax,
			MaterialRewards:      current.MaterialRewards,
			MaterialRewardMin:    rewardRange.MaterialMin,
			MaterialRewardMax:    rewardRange.MaterialMax,
			RefreshRecommended:   refreshRecommended,
			MaterialSufficient:   !requiresCrafting && !requiresPlanting,
			RequiresCrafting:     requiresCrafting,
			RequiresPlanting:     requiresPlanting,
			EstimatedSeconds:     estimated,
			DeliverySeconds:      definition.DeliverySeconds,
			TargetPerHour:        math.Round(rate*1000) / 1000,
			Eligible:             eligible,
			Warnings:             warnings,
			SuggestedCrops:       sortedFarmAmounts(plantNeeds),
			SuggestedProductions: sortedFarmAmounts(productionNeeds),
		})
	}
	frozenDeliveries := make([]FarmOrderRecommendation, 0, 1)
	sortable := make([]FarmOrderRecommendation, 0, len(results))
	for _, recommendation := range results {
		if recommendation.DeliveryStatus != "" && recommendation.Rank > 0 {
			frozenDeliveries = append(frozenDeliveries, recommendation)
			continue
		}
		sortable = append(sortable, recommendation)
	}
	sort.SliceStable(sortable, func(i, j int) bool {
		leftRankable := sortable[i].Eligible || sortable[i].DeliveryStatus != ""
		rightRankable := sortable[j].Eligible || sortable[j].DeliveryStatus != ""
		if leftRankable != rightRankable {
			return leftRankable
		}
		if sortable[i].TargetPerHour != sortable[j].TargetPerHour {
			return sortable[i].TargetPerHour > sortable[j].TargetPerHour
		}
		if sortable[i].EstimatedSeconds != sortable[j].EstimatedSeconds {
			return sortable[i].EstimatedSeconds < sortable[j].EstimatedSeconds
		}
		return sortable[i].DBKey < sortable[j].DBKey
	})
	sort.SliceStable(frozenDeliveries, func(i, j int) bool { return frozenDeliveries[i].Rank < frozenDeliveries[j].Rank })
	results = sortable
	for _, delivery := range frozenDeliveries {
		position := min(max(delivery.Rank-1, 0), len(results))
		results = append(results, FarmOrderRecommendation{})
		copy(results[position+1:], results[position:])
		results[position] = delivery
	}
	if m.lastRanks == nil {
		m.lastRanks = make(map[int]int)
	}
	if m.deliveryRanks == nil {
		m.deliveryRanks = make(map[int]int)
	}
	activeDeliveries := make(map[int]bool)
	presentOrders := make(map[int]bool, len(results))
	for index := range results {
		results[index].Rank = index + 1
		presentOrders[results[index].DBKey] = true
		m.lastRanks[results[index].DBKey] = results[index].Rank
		if results[index].DeliveryStatus != "" {
			activeDeliveries[results[index].DBKey] = true
			if m.deliveryRanks[results[index].DBKey] == 0 {
				m.deliveryRanks[results[index].DBKey] = results[index].Rank
			}
		}
	}
	for dbKey := range m.lastRanks {
		if !presentOrders[dbKey] {
			delete(m.lastRanks, dbKey)
		}
	}
	for dbKey := range m.deliveryRanks {
		if !activeDeliveries[dbKey] {
			delete(m.deliveryRanks, dbKey)
		}
	}
	return results
}

const farmPlantingSuggestionOrderLimit = 3

func farmPlantingSuggestions(
	recommendations []FarmOrderRecommendation,
	rawInventory map[string]int,
	qualityIndexes []int,
) ([]FarmRecommendationAmount, []FarmRecommendationAmount, int) {
	inventory := exactFarmInventory(rawInventory)
	cropNeeds := make(map[uint32][]int)
	productionNeeds := make(map[uint32]int)
	referenceCount := 0
	for _, recommendation := range recommendations {
		if referenceCount >= farmPlantingSuggestionOrderLimit {
			break
		}
		if !recommendation.Eligible || recommendation.DeliveryStatus != "" {
			continue
		}
		definition, known := farmDeliveryCatalog[recommendation.DBKey]
		if !known {
			continue
		}
		referenceCount++
		for _, required := range definition.RequiredItems {
			if _, crop := farmCropNames[canonicalFarmCropID(required.ItemID)]; crop {
				baseID := canonicalFarmCropID(required.ItemID)
				cropNeeds[baseID] = append(cropNeeds[baseID], required.Count)
				continue
			}
			available := inventory[required.ItemID]
			used := min(available, required.Count)
			inventory[required.ItemID] -= used
			remaining := required.Count - used
			if remaining == 0 {
				continue
			}
			recipe, hasRecipe := farmRecipes[required.ItemID]
			if !hasRecipe {
				continue
			}
			productionNeeds[required.ItemID] += remaining
			for range remaining {
				for _, material := range recipe.Materials {
					baseID := canonicalFarmCropID(material.ItemID)
					cropNeeds[baseID] = append(cropNeeds[baseID], material.Count)
				}
			}
		}
	}
	plantNeeds := make(map[uint32]int)
	for itemID, groups := range cropNeeds {
		sort.Slice(groups, func(i, j int) bool { return groups[i] > groups[j] })
		for _, count := range groups {
			selectedID, remaining := reserveFarmCrop(inventory, itemID, count, qualityIndexes)
			if remaining > 0 {
				plantNeeds[selectedID] += remaining
			}
		}
	}
	return sortedFarmAmounts(plantNeeds), sortedFarmAmounts(productionNeeds), referenceCount
}

func compactStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}

func (m *farmRecommendationManager) storageItemsLocked() ([]FarmStorageItem, int) {
	items := make([]FarmStorageItem, 0, len(m.data.Inventory))
	used := 0
	for key, count := range m.data.Inventory {
		itemID, err := strconv.ParseUint(key, 10, 32)
		if err != nil || count <= 0 || !farmStorageItemIDSet[uint32(itemID)] {
			continue
		}
		id := uint32(itemID)
		name := farmRecommendationItemName(id)
		if rewardName := farmRewardMaterialNames[id]; rewardName != "" {
			name = rewardName
		}
		used += count
		items = append(items, FarmStorageItem{
			ItemID:   id,
			Name:     name,
			Count:    count,
			Quality:  farmStorageItemQuality(id),
			Category: farmStorageItemCategory(id),
			Icon:     farmStorageItemIcon(id),
		})
	}
	categoryOrder := map[string]int{"crop": 0, "product": 1, "upgrade": 2}
	qualityOrder := map[string]int{"normal": 0, "advanced": 1, "highest": 2, "none": 3}
	sort.Slice(items, func(i, j int) bool {
		leftCategory := categoryOrder[items[i].Category]
		rightCategory := categoryOrder[items[j].Category]
		if leftCategory != rightCategory {
			return leftCategory < rightCategory
		}
		if items[i].Category == "crop" {
			leftBaseID := canonicalFarmCropID(items[i].ItemID)
			rightBaseID := canonicalFarmCropID(items[j].ItemID)
			if leftBaseID != rightBaseID {
				return leftBaseID < rightBaseID
			}
			if items[i].Quality != items[j].Quality {
				return qualityOrder[items[i].Quality] < qualityOrder[items[j].Quality]
			}
		}
		return items[i].ItemID < items[j].ItemID
	})
	return items, used
}

func (m *farmRecommendationManager) state(farm FarmState) FarmRecommendationState {
	m.mu.Lock()
	defer m.mu.Unlock()
	storageItems, storageUsed := m.storageItemsLocked()
	dataReady := m.data.OrdersSynced && m.data.InventorySynced
	status := "请进入塔汀农场并打开农场界面以初始化推荐数据"
	if m.data.OrdersSynced && !m.data.InventorySynced {
		status = "请进入一次塔汀农场以初始化储存库"
	} else if !m.data.OrdersSynced && m.data.InventorySynced {
		status = "请打开游戏内塔汀农场界面以刷新交货清单"
	} else if dataReady {
		status = "已根据当前订单和储存库生成推荐"
	}
	recommendations := m.recommendLocked(farm)
	plantingSuggestions, plantingProductions, plantingReferenceCount := farmPlantingSuggestions(
		recommendations,
		m.data.Inventory,
		enabledFarmQualityIndexes(m.data),
	)
	return FarmRecommendationState{
		Goal:                   m.data.Goal,
		UseNormalMaterials:     m.data.UseNormalMaterials,
		UseAdvancedMaterials:   m.data.UseAdvancedMaterials,
		UseHighestMaterials:    m.data.UseHighestMaterials,
		DataReady:              dataReady,
		OrdersSynced:           m.data.OrdersSynced,
		InventorySynced:        m.data.InventorySynced,
		RecipesSynced:          m.data.RecipesSynced,
		SyncedAt:               m.data.SyncedAt,
		StorageLevel:           m.data.StorageLevel,
		StorageUsed:            storageUsed,
		StorageCapacity:        farmStorageCapacities[m.data.StorageLevel],
		StorageItems:           storageItems,
		CaptureEnabled:         m.captureEnabled,
		CaptureFile:            farmRecommendationCapturePath(),
		StatusMessage:          status,
		PlantingSuggestions:    plantingSuggestions,
		PlantingProductions:    plantingProductions,
		PlantingReferenceCount: plantingReferenceCount,
		Recommendations:        recommendations,
	}
}
