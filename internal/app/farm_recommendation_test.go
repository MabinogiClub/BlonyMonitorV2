package app

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"blonymonitorv2/internal/packet"
)

func fullRecommendationFarmState() FarmState {
	plots := make([]FarmPlotState, len(farmSlotDefinitions))
	for index, definition := range farmSlotDefinitions {
		plots[index] = FarmPlotState{
			Index:    index,
			Kind:     definition.Kind,
			Label:    definition.Label,
			Unlocked: true,
			Quality:  "empty",
		}
	}
	return FarmState{Synced: true, Plots: plots}
}

func syncedRecommendationManager(goal string, orders ...FarmDeliveryState) *farmRecommendationManager {
	return &farmRecommendationManager{data: FarmRecommendationData{
		Version:            2,
		Goal:               goal,
		UseNormalMaterials: true,
		OrdersSynced:       true,
		InventorySynced:    true,
		RecipesSynced:      true,
		Orders:             orders,
		Inventory:          make(map[string]int),
		UnlockedRecipes: []uint32{
			5_041_004, 5_041_005, 5_041_006, 5_041_007,
			5_041_009, 5_041_010, 5_041_011, 5_041_012,
			5_041_014, 5_041_015, 5_041_016, 5_041_017,
			5_041_019, 5_041_020, 5_041_021, 5_041_022,
		},
	}}
}

func TestFarmRecommendationFiltersExhaustedAndRanksActualKeys(t *testing.T) {
	mgr := syncedRecommendationManager(farmGoalKeys,
		FarmDeliveryState{DBKey: 5, RemainingDeliveries: 0, MaximumDeliveries: 3, KeyReward: 1},
		FarmDeliveryState{DBKey: 5, RemainingDeliveries: 1, MaximumDeliveries: 3, KeyReward: 1},
		FarmDeliveryState{DBKey: 26, RemainingDeliveries: 2, MaximumDeliveries: 5, KeyReward: 2},
		FarmDeliveryState{DBKey: 1, RemainingDeliveries: 10, MaximumDeliveries: 10, CoinReward: 70},
	)

	got := mgr.state(fullRecommendationFarmState()).Recommendations
	if len(got) != 2 {
		t.Fatalf("recommendations = %d, want 2: %#v", len(got), got)
	}
	if got[0].DBKey != 26 || got[0].KeyReward != 2 {
		t.Fatalf("top recommendation = %#v, want order 26 with 2 keys", got[0])
	}
	if got[1].DBKey != 5 {
		t.Fatalf("second recommendation = %#v, want order 5", got[1])
	}
}

func TestFarmRecommendationAssumesRecipesUnlocked(t *testing.T) {
	mgr := syncedRecommendationManager(farmGoalKeys,
		FarmDeliveryState{DBKey: 29, RemainingDeliveries: 1, MaximumDeliveries: 5, KeyReward: 3},
	)
	mgr.data.UnlockedRecipes = nil

	got := mgr.state(fullRecommendationFarmState()).Recommendations
	if len(got) != 1 || !got[0].Eligible {
		t.Fatalf("recipe recommendation = %#v, want one eligible order", got)
	}
	if len(got[0].MissingRecipes) != 0 {
		t.Fatalf("missing recipes = %#v, want none", got[0].MissingRecipes)
	}
}

func TestFarmRecommendationAcceptsStoredProductsWithoutRecipe(t *testing.T) {
	mgr := syncedRecommendationManager(farmGoalKeys,
		FarmDeliveryState{DBKey: 29, RemainingDeliveries: 1, MaximumDeliveries: 5, KeyReward: 3},
	)
	mgr.data.UnlockedRecipes = nil
	mgr.data.Inventory = map[string]int{
		"5041007": 2,
		"5041012": 2,
		"5041016": 2,
	}

	got := mgr.state(fullRecommendationFarmState()).Recommendations
	if len(got) != 1 || !got[0].Eligible {
		t.Fatalf("stored products recommendation = %#v, want eligible", got)
	}
	if got[0].EstimatedSeconds != farmDeliveryCatalog[29].DeliverySeconds {
		t.Fatalf("estimated seconds = %d, want delivery only", got[0].EstimatedSeconds)
	}
}

func TestFarmRecommendationCoinGoalUsesObservedReward(t *testing.T) {
	mgr := syncedRecommendationManager(farmGoalCoins,
		FarmDeliveryState{DBKey: 5, RemainingDeliveries: 1, MaximumDeliveries: 3, CoinReward: 250, KeyReward: 1},
		FarmDeliveryState{DBKey: 26, RemainingDeliveries: 1, MaximumDeliveries: 5, CoinReward: 200, KeyReward: 2},
	)

	got := mgr.state(fullRecommendationFarmState()).Recommendations
	if len(got) != 2 || got[0].DBKey != 5 {
		t.Fatalf("coin recommendations = %#v, want order 5 first", got)
	}
}

func TestFarmPlantingSuggestionsUseSharedInventoryAcrossTopThreeOrders(t *testing.T) {
	mgr := syncedRecommendationManager(farmGoalKeys,
		FarmDeliveryState{DBKey: 7, RemainingDeliveries: 3, MaximumDeliveries: 3, KeyReward: 1},
		FarmDeliveryState{DBKey: 8, RemainingDeliveries: 3, MaximumDeliveries: 3, KeyReward: 1},
		FarmDeliveryState{DBKey: 9, RemainingDeliveries: 3, MaximumDeliveries: 3, KeyReward: 1},
	)
	mgr.data.Inventory = map[string]int{
		"5040996": 18,
		"5040997": 10,
		"5040998": 9,
	}

	state := mgr.state(fullRecommendationFarmState())
	if state.PlantingReferenceCount != 3 || len(state.PlantingSuggestions) != 1 {
		t.Fatalf("planting plan = count:%d crops:%#v, want only the shortage across three orders", state.PlantingReferenceCount, state.PlantingSuggestions)
	}
	if crop := state.PlantingSuggestions[0]; crop.ItemID != 5_040_997 || crop.Count != 2 || crop.PlotKind != "field" {
		t.Fatalf("planting suggestion = %#v, want two okra", crop)
	}
	sufficientByOrder := make(map[int]bool)
	for _, recommendation := range state.Recommendations {
		sufficientByOrder[recommendation.DBKey] = recommendation.MaterialSufficient
	}
	if !sufficientByOrder[7] || sufficientByOrder[8] || !sufficientByOrder[9] {
		t.Fatalf("individual recommendations do not reflect their own inventory: %#v", sufficientByOrder)
	}
}

func TestFarmPlantingSuggestionsDoNotPlantStoredRecipeMaterials(t *testing.T) {
	mgr := syncedRecommendationManager(farmGoalKeys,
		FarmDeliveryState{DBKey: 5, RemainingDeliveries: 3, MaximumDeliveries: 3, KeyReward: 3},
	)
	mgr.data.Inventory = map[string]int{
		"5040996": 2,
		"5040998": 100,
		"5040999": 2,
		"5041002": 2,
	}

	state := mgr.state(fullRecommendationFarmState())
	if state.PlantingReferenceCount != 1 || len(state.PlantingSuggestions) != 0 {
		t.Fatalf("planting plan = count:%d crops:%#v, want no planting with stored recipe materials", state.PlantingReferenceCount, state.PlantingSuggestions)
	}
	wantProductions := map[uint32]struct {
		count   int
		factory string
	}{
		5_041_003: {count: 2, factory: "abundant"},
		5_041_013: {count: 2, factory: "shining"},
	}
	if len(state.PlantingProductions) != len(wantProductions) {
		t.Fatalf("production suggestions = %#v", state.PlantingProductions)
	}
	for _, production := range state.PlantingProductions {
		want := wantProductions[production.ItemID]
		if production.Count != want.count || production.Factory != want.factory {
			t.Fatalf("production suggestion = %#v, want %#v", production, wantProductions)
		}
	}
}

func TestFarmRecommendationDoesNotMixCropQualities(t *testing.T) {
	mgr := syncedRecommendationManager(farmGoalCoins,
		FarmDeliveryState{DBKey: 9, RemainingDeliveries: 1, MaximumDeliveries: 10, CoinReward: 180},
	)
	mgr.data.UseAdvancedMaterials = true
	mgr.data.Inventory = map[string]int{
		"5040998": 5,
		"5041031": 4,
	}

	got := mgr.state(fullRecommendationFarmState()).Recommendations
	if len(got) != 1 || len(got[0].SuggestedCrops) != 1 {
		t.Fatalf("recommendation = %#v, want one crop shortage", got)
	}
	if crop := got[0].SuggestedCrops[0]; crop.ItemID != 5_040_998 || crop.Count != 4 || crop.PlotKind != "field" {
		t.Fatalf("suggested crop = %#v, want four normal jasmine", crop)
	}
	if got[0].EstimatedSeconds != farmCropDurations[5_040_998]+farmDeliveryCatalog[9].DeliverySeconds {
		t.Fatalf("estimated seconds = %d", got[0].EstimatedSeconds)
	}
}

func TestFarmRecommendationSerializesCraftsInSameFactory(t *testing.T) {
	mgr := syncedRecommendationManager(farmGoalKeys,
		FarmDeliveryState{DBKey: 26, RemainingDeliveries: 1, MaximumDeliveries: 5, KeyReward: 2},
	)
	mgr.data.Inventory = map[string]int{
		"5040996": 3,
		"5040997": 1,
		"5040998": 5,
		"5041001": 1,
	}

	got := mgr.state(fullRecommendationFarmState()).Recommendations
	if len(got) != 1 {
		t.Fatalf("recommendation = %#v", got)
	}
	want := int64(2*60 + 300 + 300)
	if got[0].EstimatedSeconds != want {
		t.Fatalf("estimated seconds = %d, want %d with abundant factory serialized", got[0].EstimatedSeconds, want)
	}
	if !got[0].RequiresCrafting || got[0].RequiresPlanting {
		t.Fatalf("work tags = crafting:%v planting:%v", got[0].RequiresCrafting, got[0].RequiresPlanting)
	}
}

func TestFarmRecommendationWaitsForPlantingBeforeCrafting(t *testing.T) {
	mgr := syncedRecommendationManager(farmGoalKeys,
		FarmDeliveryState{DBKey: 5, RemainingDeliveries: 1, MaximumDeliveries: 3, KeyReward: 1},
	)

	got := mgr.state(fullRecommendationFarmState()).Recommendations
	if len(got) != 1 {
		t.Fatalf("recommendation = %#v", got)
	}
	// Quartz needs two serial harvests from its single plot (36 minutes). The
	// abundant and shining factories then work in parallel for two minutes.
	want := int64(36*60 + 2*60 + 5*60)
	if got[0].EstimatedSeconds != want {
		t.Fatalf("estimated seconds = %d, want %d with planting before crafting", got[0].EstimatedSeconds, want)
	}
	if !got[0].RequiresPlanting || !got[0].RequiresCrafting {
		t.Fatalf("work tags = planting:%v crafting:%v", got[0].RequiresPlanting, got[0].RequiresCrafting)
	}
}

func TestFarmRecommendationSchedulesTwelveFarmPlotsInParallel(t *testing.T) {
	farm := fullRecommendationFarmState()
	needs := map[uint32]int{
		5_040_996: 6,
		5_040_999: 2,
		5_041_000: 2,
		5_041_001: 1,
		5_041_002: 1,
	}

	seconds, _, warnings := estimateFarmGrowth(needs, farm)
	if len(warnings) != 0 {
		t.Fatalf("growth warnings = %#v", warnings)
	}
	if seconds != farmCropDurations[5_041_000] {
		t.Fatalf("growth seconds = %d, want six fields and six special plots in parallel", seconds)
	}
}

func TestFarmRecommendationShowsDeliveringAndCompletedOrders(t *testing.T) {
	now := time.Now()
	mgr := syncedRecommendationManager(farmGoalKeys,
		FarmDeliveryState{
			DBKey:               5,
			StartedAt:           farmGameClockMilliseconds(now.Add(-time.Minute)),
			RemainingDeliveries: 2,
			MaximumDeliveries:   3,
			KeyReward:           1,
		},
		FarmDeliveryState{
			DBKey:               26,
			StartedAt:           farmGameClockMilliseconds(now.Add(-10 * time.Minute)),
			RemainingDeliveries: 4,
			MaximumDeliveries:   5,
			KeyReward:           2,
		},
	)

	got := mgr.state(fullRecommendationFarmState()).Recommendations
	if len(got) != 2 {
		t.Fatalf("active deliveries = %#v", got)
	}
	statuses := map[int]string{}
	for _, order := range got {
		statuses[order.DBKey] = order.DeliveryStatus
		if order.Eligible || order.TargetPerHour <= 0 {
			t.Fatalf("active delivery did not retain a ranking rate: %#v", order)
		}
	}
	if statuses[5] != "delivering" || statuses[26] != "completed" {
		t.Fatalf("delivery statuses = %#v", statuses)
	}
}

func TestFarmRecommendationKeepsRankWhenDeliveryStarts(t *testing.T) {
	now := time.Now()
	mgr := syncedRecommendationManager(farmGoalKeys,
		FarmDeliveryState{DBKey: 7, RemainingDeliveries: 3, MaximumDeliveries: 3, KeyReward: 3},
		FarmDeliveryState{DBKey: 9, RemainingDeliveries: 3, MaximumDeliveries: 3, KeyReward: 1},
		FarmDeliveryState{DBKey: 27, RemainingDeliveries: 5, MaximumDeliveries: 5, KeyReward: 3},
	)
	mgr.data.Inventory = map[string]int{
		"5040996": 18,
		"5040998": 9,
		"5041006": 2,
		"5041022": 2,
		"5041012": 2,
	}

	before := mgr.state(fullRecommendationFarmState()).Recommendations
	if len(before) != 3 || before[0].DBKey != 7 || before[0].Rank != 1 {
		t.Fatalf("initial recommendations = %#v, want order 7 at rank 1", before)
	}
	for index := range mgr.data.Orders {
		if mgr.data.Orders[index].DBKey == 7 {
			mgr.data.Orders[index].StartedAt = farmGameClockMilliseconds(now)
		}
	}
	mgr.data.Inventory = map[string]int{}

	after := mgr.state(fullRecommendationFarmState()).Recommendations
	if len(after) != 3 || after[0].DBKey != 7 || after[0].Rank != 1 || after[0].DeliveryStatus != "delivering" {
		t.Fatalf("delivery recommendations = %#v, want active order to keep rank 1", after)
	}
}

func TestFarmRecommendationMarksLowRollForRefresh(t *testing.T) {
	mgr := syncedRecommendationManager(farmGoalKeys,
		FarmDeliveryState{DBKey: 27, RemainingDeliveries: 5, MaximumDeliveries: 5, KeyReward: 1},
	)
	got := mgr.state(fullRecommendationFarmState()).Recommendations
	if len(got) != 1 || !got[0].RefreshRecommended {
		t.Fatalf("recommendation = %#v, want refresh suggestion", got)
	}
	if got[0].KeyRewardMin != 1 || got[0].KeyRewardMax != 3 {
		t.Fatalf("key range = %d-%d", got[0].KeyRewardMin, got[0].KeyRewardMax)
	}
}

func TestFarmRecommendationWaitsForAllRequiredSources(t *testing.T) {
	mgr := syncedRecommendationManager(farmGoalKeys,
		FarmDeliveryState{DBKey: 5, RemainingDeliveries: 1, MaximumDeliveries: 3, KeyReward: 1},
	)
	mgr.data.InventorySynced = false

	state := mgr.state(fullRecommendationFarmState())
	if state.DataReady || len(state.Recommendations) != 0 {
		t.Fatalf("incomplete state = %#v, want no recommendations", state)
	}
	if state.StatusMessage != "请进入一次塔汀农场以初始化储存库" {
		t.Fatalf("status = %q", state.StatusMessage)
	}
}

func TestFarmRecommendationDoesNotRequireRecipeSync(t *testing.T) {
	mgr := syncedRecommendationManager(farmGoalKeys,
		FarmDeliveryState{DBKey: 5, RemainingDeliveries: 1, MaximumDeliveries: 3, KeyReward: 1},
	)
	mgr.data.RecipesSynced = false

	state := mgr.state(fullRecommendationFarmState())
	if !state.DataReady || len(state.Recommendations) != 1 {
		t.Fatalf("state = %#v, want recommendation without recipe sync", state)
	}
}

func farmRecommendationPacketMessage(includeMissionProgress bool) packet.Message {
	message := packet.Message{
		packet.NewMessageElemString("5041003;5041008;5041013;5041018;"),
		packet.NewMessageElemInt(1),
		packet.NewMessageElemInt(5),
		packet.NewMessageElemLong(0),
		packet.NewMessageElemInt(2),
		packet.NewMessageElemInt(1),
		packet.NewMessageElemInt(5_300_264),
		packet.NewMessageElemInt(138),
		packet.NewMessageElemInt(2),
		packet.NewMessageElemInt(5_041_043),
		packet.NewMessageElemInt(1),
		packet.NewMessageElemInt(5_041_046),
		packet.NewMessageElemInt(2),
	}
	if includeMissionProgress {
		message = append(message,
			packet.NewMessageElemInt(1),
			packet.NewMessageElemInt(1),
			packet.NewMessageElemInt(1),
			packet.NewMessageElemByte(1),
		)
	}
	return message
}

func farmRecommendationStartedDeliveryMessage(startedAt uint64, remaining uint32) packet.Message {
	message := farmRecommendationPacketMessage(false)
	message[3] = packet.NewMessageElemLong(startedAt)
	message[4] = packet.NewMessageElemInt(remaining)
	return message
}

func farmStorageCharacterData(items ...struct {
	itemID uint32
	amount uint16
}) packet.Message {
	message := packet.Message{
		packet.NewMessageElemByte(1),
		packet.NewMessageElemLong(4503599638666428),
		packet.NewMessageElemByte(2),
		packet.NewMessageElemString("Flandre"),
		packet.NewMessageElemString(""),
		packet.NewMessageElemString(""),
		packet.NewMessageElemInt(10001),
	}
	for _, value := range items {
		data := make([]byte, 80)
		binary.LittleEndian.PutUint32(data[0:], farmStoragePocketType)
		binary.LittleEndian.PutUint32(data[4:], value.itemID)
		binary.LittleEndian.PutUint16(data[36:], value.amount)
		message = append(message, packet.NewMessageElemBin(data))
	}
	return message
}

func TestFarmRecommendationSyncsObservedSnapshot(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	mgr := &farmRecommendationManager{data: defaultFarmRecommendationData()}
	now := time.UnixMilli(1_700_000_000_000)
	mgr.recordPacket(&packet.GamePacket{Op: opcodeFarmSnapshot, At: now, Msg: farmRecommendationPacketMessage(true)})

	if !mgr.data.OrdersSynced || mgr.data.InventorySynced || !mgr.data.RecipesSynced {
		t.Fatalf("sync flags = orders:%v inventory:%v recipes:%v", mgr.data.OrdersSynced, mgr.data.InventorySynced, mgr.data.RecipesSynced)
	}
	if len(mgr.data.Orders) != 1 {
		t.Fatalf("orders = %#v, want one", mgr.data.Orders)
	}
	order := mgr.data.Orders[0]
	if order.DBKey != 5 || order.RemainingDeliveries != 2 || order.MaximumDeliveries != 3 || order.CoinReward != 138 || order.KeyReward != 1 {
		t.Fatalf("parsed order = %#v", order)
	}
	if order.MaterialRewards["储存库升级用涂料"] != 2 {
		t.Fatalf("material rewards = %#v", order.MaterialRewards)
	}
	if mgr.data.SyncedAt != now.UnixMilli() {
		t.Fatalf("syncedAt = %d, want %d", mgr.data.SyncedAt, now.UnixMilli())
	}
}

func TestFarmRecommendationExposesStorageLevelCapacityAndItems(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	mgr := syncedRecommendationManager(farmGoalKeys)
	mgr.data.StorageLevel = 5
	mgr.data.Inventory = map[string]int{
		"5040996": 18,
		"5041029": 2,
		"5041036": 4,
		"5040997": 5,
		"5041030": 6,
		"5041013": 1,
		"5041044": 3,
	}

	state := mgr.state(fullRecommendationFarmState())
	if state.StorageLevel != 5 || state.StorageCapacity != 13_000 || state.StorageUsed != 39 {
		t.Fatalf("storage summary = level:%d used:%d capacity:%d", state.StorageLevel, state.StorageUsed, state.StorageCapacity)
	}
	if len(state.StorageItems) != 7 {
		t.Fatalf("storage items = %#v", state.StorageItems)
	}
	if state.StorageItems[0].Name != "黑莓" || state.StorageItems[0].Quality != "normal" || state.StorageItems[0].Icon != "item_seasonalfarming_06_s" {
		t.Fatalf("first storage item = %#v", state.StorageItems[0])
	}
	wantNames := []string{"黑莓", "高级黑莓", "最高级黑莓", "秋葵", "高级秋葵", "红月耳环", "储存库升级用砖块"}
	for index, wantName := range wantNames {
		if state.StorageItems[index].Name != wantName {
			t.Fatalf("storage item %d = %q, want %q; all items = %#v", index, state.StorageItems[index].Name, wantName, state.StorageItems)
		}
	}
}

func TestFarmRecommendationReadsStorageLevelFromSnapshot(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	mgr := &farmRecommendationManager{data: defaultFarmRecommendationData()}
	message := append(packet.Message{
		packet.NewMessageElemString(`<xml level="12" StorageLevel="5"/>`),
	}, farmRecommendationPacketMessage(false)...)
	mgr.recordPacket(&packet.GamePacket{Op: opcodeFarmSnapshot, At: time.Now(), Msg: message})

	if mgr.data.StorageLevel != 5 {
		t.Fatalf("storage level = %d, want 5", mgr.data.StorageLevel)
	}
}

func TestFarmRecommendationSyncsStorageFromCharacterData(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	mgr := &farmRecommendationManager{data: defaultFarmRecommendationData()}
	now := time.UnixMilli(1_700_000_000_000)
	mgr.recordPacket(&packet.GamePacket{Op: opcodeCharacterData, At: now, Msg: farmStorageCharacterData(
		struct {
			itemID uint32
			amount uint16
		}{5_041_013, 1},
		struct {
			itemID uint32
			amount uint16
		}{5_041_022, 2},
	)})

	if !mgr.data.InventorySynced {
		t.Fatal("storage was not synchronized from character data")
	}
	if mgr.data.Inventory["5041013"] != 1 || mgr.data.Inventory["5041022"] != 2 {
		t.Fatalf("inventory = %#v", mgr.data.Inventory)
	}
	if _, exists := mgr.data.Inventory["5041020"]; exists {
		t.Fatalf("zero-count item should be absent: %#v", mgr.data.Inventory)
	}
}

func TestFarmRecommendationFullStorageSyncReplacesLocalMirror(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	mgr := &farmRecommendationManager{data: defaultFarmRecommendationData()}
	mgr.data.Inventory = map[string]int{"5041001": 9, "5041002": 4}
	mgr.data.InventorySynced = true
	mgr.recordPacket(&packet.GamePacket{Op: opcodeCharacterData, At: time.Now(), Msg: farmStorageCharacterData(
		struct {
			itemID uint32
			amount uint16
		}{5_041_001, 2},
	)})

	if mgr.data.Inventory["5041001"] != 2 || len(mgr.data.Inventory) != 1 {
		t.Fatalf("full storage sync did not replace mirror: %#v", mgr.data.Inventory)
	}
}

func TestFarmRecommendationIgnoresPetCharacterData(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	mgr := syncedRecommendationManager(farmGoalKeys,
		FarmDeliveryState{DBKey: 12, RemainingDeliveries: 7, MaximumDeliveries: 7, KeyReward: 1},
	)
	mgr.data.CharacterID = "4503599638666428"
	mgr.data.Inventory = map[string]int{"5041001": 9}
	petMessage := farmStorageCharacterData(struct {
		itemID uint32
		amount uint16
	}{5_041_002, 4})
	petMessage[1] = packet.NewMessageElemLong(4504699155172732)
	petMessage[3] = packet.NewMessageElemString("沙县小吃喵")
	petMessage[6] = packet.NewMessageElemInt(490508)

	mgr.recordPacket(&packet.GamePacket{Op: opcodeCharacterData, At: time.Now(), Msg: petMessage})

	if mgr.data.CharacterID != "4503599638666428" || mgr.data.Inventory["5041001"] != 9 || len(mgr.data.Inventory) != 1 {
		t.Fatalf("pet character data changed player storage: %#v", mgr.data)
	}
	if !mgr.data.OrdersSynced || len(mgr.data.Orders) != 1 || mgr.data.Orders[0].DBKey != 12 {
		t.Fatalf("pet character data cleared player orders: %#v", mgr.data.Orders)
	}
}

func TestFarmRecommendationRestoresLocalMirrorAfterRestart(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MABI_WORK_DIR", dir)
	saved := defaultFarmRecommendationData()
	saved.CharacterID = "4503599638666428"
	saved.InventorySynced = true
	saved.OrdersSynced = true
	saved.RecipesSynced = true
	saved.StorageLevel = 5
	saved.Inventory = map[string]int{"5041001": 7}
	saved.Orders = []FarmDeliveryState{{DBKey: 12, RemainingDeliveries: 7, MaximumDeliveries: 7, KeyReward: 1}}
	data, err := json.Marshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, farmRecommendationDataFileName), data, 0o644); err != nil {
		t.Fatal(err)
	}

	mgr := newFarmRecommendationManager()
	if mgr.data.CharacterID != saved.CharacterID || !mgr.data.InventorySynced || mgr.data.Inventory["5041001"] != 7 {
		t.Fatalf("restored local mirror = %#v", mgr.data)
	}
	if len(mgr.data.Orders) != 1 || mgr.data.Orders[0].DBKey != 12 || mgr.data.StorageLevel != 5 {
		t.Fatalf("restored recommendation data = %#v", mgr.data)
	}
}

func TestFarmRecommendationClearsPreviousCharacterOrdersOnStorageSync(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	mgr := syncedRecommendationManager(farmGoalKeys,
		FarmDeliveryState{DBKey: 12, RemainingDeliveries: 7, MaximumDeliveries: 7, KeyReward: 1},
	)
	mgr.data.CharacterID = "100"
	mgr.data.Inventory = map[string]int{"5041001": 9}
	message := farmStorageCharacterData(struct {
		itemID uint32
		amount uint16
	}{5_041_002, 4})
	message[1] = packet.NewMessageElemLong(200)
	mgr.recordPacket(&packet.GamePacket{Op: opcodeCharacterData, At: time.Now(), Msg: message})

	if mgr.data.CharacterID != "200" || mgr.data.OrdersSynced || len(mgr.data.Orders) != 0 {
		t.Fatalf("previous character orders were retained: %#v", mgr.data)
	}
	if !mgr.data.InventorySynced || mgr.data.Inventory["5041002"] != 4 || len(mgr.data.Inventory) != 1 {
		t.Fatalf("new character storage was not synchronized: %#v", mgr.data.Inventory)
	}
}

func TestFarmRecommendationAddsObservedStorageDeposit(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	mgr := &farmRecommendationManager{data: defaultFarmRecommendationData()}
	mgr.data.Inventory = map[string]int{"5041001": 3}
	mgr.data.InventorySynced = true
	now := time.Now()
	mgr.recordPacket(&packet.GamePacket{Op: opcodeItemNotice, At: now, Msg: packet.Message{
		packet.NewMessageElemString("<xml type='item' classid='5041001' />"),
		packet.NewMessageElemInt(1500),
	}})
	mgr.recordPacket(&packet.GamePacket{Op: opcodeFarmStorageNotice, At: now.Add(time.Millisecond), Msg: packet.Message{
		packet.NewMessageElemInt(2),
		packet.NewMessageElemInt(1),
		packet.NewMessageElemString("已将2个塔汀农场普通魔法蜘蛛丝存入储存库。"),
	}})

	if mgr.data.Inventory["5041001"] != 5 {
		t.Fatalf("inventory after deposit = %#v", mgr.data.Inventory)
	}
}

func TestFarmRecommendationAddsCollectedCraftProduct(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	mgr := &farmRecommendationManager{data: defaultFarmRecommendationData()}
	mgr.data.InventorySynced = true
	now := time.Now()
	mgr.recordPacket(&packet.GamePacket{Op: opcodeItemNotice, At: now, Msg: packet.Message{
		packet.NewMessageElemString("<xml type='item' classid='5041006' />"),
		packet.NewMessageElemInt(1500),
	}})
	mgr.recordPacket(&packet.GamePacket{Op: opcodeFarmStorageNotice, At: now.Add(time.Millisecond), Msg: packet.Message{
		packet.NewMessageElemInt(2),
		packet.NewMessageElemInt(1),
		packet.NewMessageElemString("已将2个塔汀农场星星色拉存入储存库。"),
	}})

	if mgr.data.Inventory["5041006"] != 2 {
		t.Fatalf("inventory after collecting craft product = %#v", mgr.data.Inventory)
	}
}

func TestFarmRecommendationRecalculatesFromSharedStorageMirror(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	mgr := syncedRecommendationManager(farmGoalKeys,
		FarmDeliveryState{DBKey: 12, RemainingDeliveries: 7, MaximumDeliveries: 7, KeyReward: 1},
	)
	mgr.data.Inventory = map[string]int{"5041001": 2}
	before := mgr.state(fullRecommendationFarmState()).Recommendations
	if len(before) != 1 || !before[0].RequiresPlanting || before[0].MaterialSufficient {
		t.Fatalf("recommendation before deposit = %#v", before)
	}

	now := time.Now()
	mgr.recordPacket(&packet.GamePacket{Op: opcodeItemNotice, At: now, Msg: packet.Message{
		packet.NewMessageElemString("<xml type='item' classid='5041001' />"),
		packet.NewMessageElemInt(1500),
	}})
	mgr.recordPacket(&packet.GamePacket{Op: opcodeFarmStorageNotice, At: now.Add(time.Millisecond), Msg: packet.Message{
		packet.NewMessageElemInt(1),
		packet.NewMessageElemInt(1),
		packet.NewMessageElemString("已将1个塔汀农场普通魔法蜘蛛丝存入储存库。"),
	}})

	after := mgr.state(fullRecommendationFarmState()).Recommendations
	if len(after) != 1 || !after[0].MaterialSufficient || after[0].RequiresPlanting || after[0].RequiresCrafting {
		t.Fatalf("recommendation after deposit = %#v", after)
	}
}

func TestFarmRecommendationConsumesExactCraftMaterialsOnce(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	mgr := &farmRecommendationManager{data: defaultFarmRecommendationData()}
	mgr.data.Inventory = map[string]int{
		"5041029": 10,
		"5041030": 10,
		"5041032": 10,
		"5040996": 7,
	}
	mgr.data.InventorySynced = true
	message := packet.Message{
		packet.NewMessageElemString("single"),
		packet.NewMessageElemString(`<xml onwer="4503599638666428" CMSFPI="5041006" CMSFPA="2" CMSFPRA="0" CMSFPT="63922563425408" CMSFPMT="5041029;5041030;5041032;"/>`),
	}
	pkt := &packet.GamePacket{Id: 45468061394272275, Op: opcodeFarmCropState, At: time.Now(), Msg: message}
	mgr.recordPacket(pkt)
	mgr.recordPacket(pkt)

	if mgr.data.Inventory["5041029"] != 8 || mgr.data.Inventory["5041030"] != 6 || mgr.data.Inventory["5041032"] != 8 {
		t.Fatalf("inventory after craft = %#v", mgr.data.Inventory)
	}
	if mgr.data.Inventory["5040996"] != 7 {
		t.Fatalf("craft consumed a different quality tier: %#v", mgr.data.Inventory)
	}
}

func TestFarmRecommendationIncrementKeepsSnapshotInventory(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	mgr := &farmRecommendationManager{data: defaultFarmRecommendationData()}
	mgr.data.Inventory = map[string]int{"5040996": 9}
	mgr.data.InventorySynced = true
	mgr.recordPacket(&packet.GamePacket{Op: opcodeFarmDelivery, At: time.Now(), Msg: farmRecommendationPacketMessage(false)})

	if !mgr.data.OrdersSynced || mgr.data.Inventory["5040996"] != 9 {
		t.Fatalf("increment sync lost inventory: %#v", mgr.data)
	}
}

func TestFarmRecommendationIncrementConsumesMaterialsWhenDeliveryStarts(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	mgr := &farmRecommendationManager{data: defaultFarmRecommendationData()}
	mgr.data.Inventory = map[string]int{"5041003": 5, "5041013": 4}
	mgr.data.InventorySynced = true
	mgr.data.Orders = []FarmDeliveryState{{DBKey: 5, RemainingDeliveries: 3}}
	mgr.data.OrdersSynced = true
	mgr.recordPacket(&packet.GamePacket{Op: opcodeFarmDelivery, At: time.Now(), Msg: farmRecommendationStartedDeliveryMessage(63922563445638, 3)})

	if mgr.data.Inventory["5041003"] != 3 || mgr.data.Inventory["5041013"] != 2 {
		t.Fatalf("inventory after delivery = %#v", mgr.data.Inventory)
	}
	mgr.recordPacket(&packet.GamePacket{Op: opcodeFarmDelivery, At: time.Now(), Msg: farmRecommendationStartedDeliveryMessage(63922563445638, 2)})
	if mgr.data.Inventory["5041003"] != 3 || mgr.data.Inventory["5041013"] != 2 {
		t.Fatalf("delivery completion consumed materials twice: %#v", mgr.data.Inventory)
	}
}

func TestFarmRecommendationAddsDeliveryMaterialRewardOnce(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	mgr := &farmRecommendationManager{data: defaultFarmRecommendationData()}
	mgr.data.Inventory = map[string]int{"5041046": 3}
	mgr.data.InventorySynced = true
	mgr.data.Orders = []FarmDeliveryState{{
		DBKey:               5,
		StartedAt:           63_922_563_447_473,
		RemainingDeliveries: 3,
		MaterialRewards:     map[string]int{"储存库升级用涂料": 2},
	}}
	mgr.data.OrdersSynced = true
	completion := &packet.GamePacket{
		Op:  opcodeFarmDelivery,
		At:  time.Now(),
		Msg: farmRecommendationStartedDeliveryMessage(0, 2),
	}
	mgr.recordPacket(completion)

	if mgr.data.Inventory["5041046"] != 5 {
		t.Fatalf("inventory after delivery reward = %#v", mgr.data.Inventory)
	}
	mgr.recordPacket(completion)
	if mgr.data.Inventory["5041046"] != 5 {
		t.Fatalf("delivery reward was added twice: %#v", mgr.data.Inventory)
	}
}

func TestFarmRecommendationDoesNotImportAnotherPlayersCapture(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MABI_WORK_DIR", dir)
	message := farmRecommendationPacketMessage(true)
	record := farmPacketCaptureRecord{
		Type:     "packet",
		At:       1_700_000_000_000,
		Opcode:   "0x21394",
		EntityID: "1",
		Elements: make([]farmPacketCaptureElement, 0, len(message)),
	}
	for _, elem := range message {
		record.Elements = append(record.Elements, farmPacketCaptureElement{Type: uint8(elem.Type()), Value: farmCaptureValue(elem)})
	}
	line, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	line = append(line, '\n')
	if err := os.WriteFile(filepath.Join(dir, farmRecommendationCaptureFileName), line, 0o644); err != nil {
		t.Fatal(err)
	}

	mgr := newFarmRecommendationManager()
	if mgr.data.OrdersSynced || mgr.data.InventorySynced || mgr.data.RecipesSynced || len(mgr.data.Orders) != 0 {
		t.Fatalf("manager imported stale capture data: %#v", mgr.data)
	}
}

func TestFarmRecommendationUpdateClearsPreviousPlayerData(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MABI_WORK_DIR", dir)
	capturePath := filepath.Join(dir, farmRecommendationCaptureFileName)
	if err := os.WriteFile(capturePath, []byte("old-player-data\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mgr := syncedRecommendationManager(farmGoalCoins, FarmDeliveryState{DBKey: 24, RemainingDeliveries: 5, KeyReward: 2})
	if err := mgr.setCaptureEnabled(true); err != nil {
		t.Fatal(err)
	}
	defer mgr.close()

	if mgr.data.Goal != farmGoalCoins || mgr.data.OrdersSynced || mgr.data.InventorySynced || mgr.data.RecipesSynced || len(mgr.data.Orders) != 0 {
		t.Fatalf("data after player update = %#v", mgr.data)
	}
	capture, err := os.ReadFile(capturePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(capture) == "old-player-data\n" {
		t.Fatal("capture file was not replaced")
	}
}
