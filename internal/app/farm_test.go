package app

import (
	"context"
	"fmt"
	"testing"
	"time"

	"blonymonitorv2/internal/packet"
)

func farmRawMilliseconds(wall time.Time) int64 {
	naiveUTC := time.Date(wall.Year(), wall.Month(), wall.Day(), wall.Hour(), wall.Minute(), wall.Second(), wall.Nanosecond(), time.UTC)
	return naiveUTC.UnixMilli() + dotNetUnixEpochMilliseconds
}

func farmCropPacket(at time.Time, entityID uint64, phase, xmlState string) *packet.GamePacket {
	return &packet.GamePacket{
		At: at,
		Op: opcodeFarmCropState,
		Id: entityID,
		Msg: packet.Message{
			packet.NewMessageElemString(phase),
			packet.NewMessageElemLong(0),
			packet.NewMessageElemByte(1),
			packet.NewMessageElemString(xmlState),
		},
	}
}

func TestFarmResourcePackets(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr := NewFarmManager(ctx, nil, nil, nil)
	now := time.Now()

	mgr.HandlePacket(&packet.GamePacket{
		At: now,
		Op: opcodeFarmSummary,
		Msg: packet.Message{
			packet.NewMessageElemLong(123),
			packet.NewMessageElemInt(7),
			packet.NewMessageElemInt(22),
		},
	})
	mgr.HandlePacket(&packet.GamePacket{
		At: now,
		Op: opcodeFarmEnergy,
		Msg: packet.Message{
			packet.NewMessageElemLong(123),
			packet.NewMessageElemInt(11),
		},
	})

	state := mgr.State(now)
	if !state.FertilityKnown || state.Fertility != 22 {
		t.Fatalf("fertility = %d, known=%v", state.Fertility, state.FertilityKnown)
	}
	if !state.EnergyKnown || state.Energy != 11 {
		t.Fatalf("energy = %d, known=%v", state.Energy, state.EnergyKnown)
	}
}

func TestFarmCropUpdatesAccelerationAndHarvest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr := NewFarmManager(ctx, nil, nil, nil)
	location := time.FixedZone("CST", 8*60*60)
	serverStart := time.Date(2026, 8, 14, 19, 22, 57, 0, location)
	packetAt := serverStart.Add(7 * time.Second)
	startRaw := farmRawMilliseconds(serverStart)

	xmlState := fmt.Sprintf(`<xml onwer="123" itemid="5041232" support="20" lmtime="%d" supportIndex="3" Special="false" starttime="%d" Fertility="true" fieldprop="456"/>`, startRaw, startRaw)
	mgr.HandlePacket(farmCropPacket(packetAt, 456, "seed", xmlState))

	state := mgr.State(packetAt)
	plot := state.Plots[0]
	if !plot.Planted || plot.CropName != "黑莓" || plot.Support != 20 || !plot.Fertility {
		t.Fatalf("unexpected plot: %#v", plot)
	}
	if plot.TotalSeconds != 144 {
		t.Fatalf("fertile blackberry duration = %d, want 144", plot.TotalSeconds)
	}
	wantReadyAt := packetAt.Add(144 * time.Second).UnixMilli()
	if plot.ReadyAt != wantReadyAt {
		t.Fatalf("readyAt = %d, want %d", plot.ReadyAt, wantReadyAt)
	}

	careXML := fmt.Sprintf(`<xml onwer="123" itemid="5041232" support="40" lmtime="%d" supportIndex="4" Special="false" starttime="%d" Fertility="true" fieldprop="456"/>`, startRaw+20_000, startRaw)
	mgr.HandlePacket(farmCropPacket(packetAt.Add(20*time.Second), 456, "bud", careXML))
	plot = mgr.State(packetAt.Add(20 * time.Second)).Plots[0]
	if plot.Support != 40 || plot.Quality != "advanced" {
		t.Fatalf("care update = support %d quality %q", plot.Support, plot.Quality)
	}

	acceleratedRaw := startRaw - 60_000
	acceleratedXML := fmt.Sprintf(`<xml onwer="123" itemid="5041232" support="40" lmtime="%d" supportIndex="0" Special="false" starttime="%d" Fertility="true" fieldprop="456"/>`, startRaw+20_000, acceleratedRaw)
	previousReadyAt := plot.ReadyAt
	mgr.HandlePacket(farmCropPacket(packetAt.Add(25*time.Second), 456, "bud", acceleratedXML))
	plot = mgr.State(packetAt.Add(25 * time.Second)).Plots[0]
	if plot.ReadyAt != previousReadyAt-60_000 {
		t.Fatalf("accelerated readyAt = %d, want %d", plot.ReadyAt, previousReadyAt-60_000)
	}

	harvestXML := fmt.Sprintf(`<xml onwer="123" itemid="0" support="0" lmtime="%d" supportIndex="0" Special="false" starttime="%d" Fertility="false" fieldprop="0"/>`, startRaw+30_000, startRaw+30_000)
	mgr.HandlePacket(farmCropPacket(packetAt.Add(30*time.Second), 456, "seed", harvestXML))
	if mgr.State(packetAt.Add(30 * time.Second)).Plots[0].Planted {
		t.Fatal("harvested crop was not removed")
	}
}

func TestFarmLinkedEntityHarvestAndReplant(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr := NewFarmManager(ctx, nil, nil, nil)
	now := time.Now()
	oldRaw := farmRawMilliseconds(now.Add(-32 * time.Minute))
	const fieldEntity = uint64(5001)
	const cropEntity = uint64(6001)

	mgr.HandlePacket(&packet.GamePacket{
		At: now,
		Op: opcodeFarmSnapshot,
		Msg: packet.Message{
			packet.NewMessageElemString(`<xml level="7" fertility="0" ownerName="Tester"/>`),
			packet.NewMessageElemString(fmt.Sprintf(`<xml onwer="123" fieldprop="%d" itemid="5041234" support="45" lmtime="%d" supportIndex="0" Special="false" starttime="%d" Fertility="false"/>`, fieldEntity, oldRaw, oldRaw)),
		},
	})
	if plot := mgr.State(now).Plots[0]; !plot.Planted || !plot.Ready {
		t.Fatalf("snapshot crop was not loaded as ready: %#v", plot)
	}
	// Relationship packets arrive without itemid and connect the crop entity
	// used by incremental updates to the field entity stored in snapshots.
	mgr.HandlePacket(farmCropPacket(now, cropEntity, "seed", `<xml onwer="123" fieldprop="5001"/>`))

	harvest := fmt.Sprintf(`<xml onwer="123" itemid="0" support="0" lmtime="%d" supportIndex="0" Special="false" starttime="%d" linkprop="0" fieldprop="0" Fertility="false"/>`, oldRaw, oldRaw)
	mgr.HandlePacket(farmCropPacket(now.Add(time.Second), cropEntity, "empty", harvest))
	if mgr.State(now.Add(time.Second)).Plots[0].Planted {
		t.Fatal("linked harvest did not clear the snapshot crop")
	}

	newRaw := farmRawMilliseconds(now.Add(2 * time.Second))
	replant := fmt.Sprintf(`<xml onwer="123" fieldprop="%d" itemid="5041234" support="0" lmtime="%d" supportIndex="0" Special="false" starttime="%d" Fertility="false"/>`, fieldEntity, newRaw, newRaw)
	mgr.HandlePacket(farmCropPacket(now.Add(2*time.Second), cropEntity, "seed", replant))
	plot := mgr.State(now.Add(2 * time.Second)).Plots[0]
	if !plot.Planted || plot.Ready || plot.Support != 0 {
		t.Fatalf("replanted crop did not replace the harvested state: %#v", plot)
	}
}

func TestFarmIncompleteCropStatePreservesFertility(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr := NewFarmManager(ctx, nil, nil, nil)
	now := time.Now()
	raw := farmRawMilliseconds(now)

	complete := fmt.Sprintf(`<xml onwer="123" itemid="5040991" support="0" lmtime="%d" supportIndex="0" Special="false" starttime="%d" Fertility="true" fieldprop="789"/>`, raw, raw)
	mgr.HandlePacket(farmCropPacket(now, 789, "seed", complete))
	incomplete := fmt.Sprintf(`<xml onwer="123" itemid="5040991" support="9" lmtime="%d" supportIndex="1" Special="false" starttime="%d" fieldprop="789"/>`, raw, raw)
	mgr.HandlePacket(farmCropPacket(now, 789, "seed", incomplete))

	plot := mgr.State(now).Plots[0]
	if !plot.Fertility || plot.Support != 9 {
		t.Fatalf("incomplete update lost state: %#v", plot)
	}
}

func TestFarmSnapshotRebuildsCropsAndFertility(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr := NewFarmManager(ctx, nil, nil, nil)
	now := time.Now()
	raw := farmRawMilliseconds(now.Add(-time.Minute))

	mgr.HandlePacket(&packet.GamePacket{
		At: now,
		Op: opcodeFarmSnapshot,
		Msg: packet.Message{
			packet.NewMessageElemString(`<xml level="7" fertility="20" ownerName="Tester"/>`),
			packet.NewMessageElemString(fmt.Sprintf(`<xml onwer="123" fieldprop="1001" itemid="5040992" support="45" lmtime="%d" supportIndex="0" Special="true" starttime="%d" Fertility="false"/>`, raw, raw)),
			packet.NewMessageElemString(fmt.Sprintf(`<xml onwer="123" fieldprop="1002" itemid="5040993" support="5" lmtime="%d" supportIndex="0" Special="false" starttime="%d" Fertility="true"/>`, raw, raw)),
		},
	})

	state := mgr.State(now)
	if !state.Synced || state.Fertility != 20 {
		t.Fatalf("snapshot state = %#v", state)
	}
	if !state.Plots[6].Planted || state.Plots[6].CropName != "红梨树" || !state.Plots[6].Special {
		t.Fatalf("red pear slot = %#v", state.Plots[6])
	}
	if !state.Plots[7].Planted || state.Plots[7].CropName != "橡胶树" || !state.Plots[7].Fertility {
		t.Fatalf("rubber slot = %#v", state.Plots[7])
	}
}

func TestFarmSnapshotKeepsNewerLiveCareState(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr := NewFarmManager(ctx, nil, nil, nil)
	now := time.Now()
	startRaw := farmRawMilliseconds(now.Add(-time.Minute))
	fieldEntityID := uint64(1001)
	cropEntityID := uint64(2001)

	liveXML := fmt.Sprintf(`<xml onwer="123" fieldprop="%d" itemid="5040989" support="85" lmtime="%d" supportIndex="5" Special="true" starttime="%d" Fertility="false"/>`, fieldEntityID, startRaw+20_000, startRaw)
	mgr.HandlePacket(farmCropPacket(now, cropEntityID, "bud", liveXML))

	staleSnapshotXML := fmt.Sprintf(`<xml onwer="123" fieldprop="%d" itemid="5040989" support="45" lmtime="%d" supportIndex="0" Special="true" starttime="%d" Fertility="false"/>`, fieldEntityID, startRaw, startRaw)
	mgr.HandlePacket(&packet.GamePacket{
		At: now.Add(time.Second),
		Op: opcodeFarmSnapshot,
		Msg: packet.Message{
			packet.NewMessageElemString(`<xml level="12" fertility="7" ownerName="Tester"/>`),
			packet.NewMessageElemInt(1),
			packet.NewMessageElemInt(0),
			packet.NewMessageElemString(staleSnapshotXML),
		},
	})

	state := mgr.State(now.Add(time.Second))
	plot := state.Plots[0]
	if plot.Support != 85 || plot.Phase != "bud" || !plot.Special {
		t.Fatalf("stale snapshot overwrote live care state: %#v", plot)
	}
	if !state.FertilityKnown || state.Fertility != 7 {
		t.Fatalf("snapshot fertility was not applied: known=%v value=%d", state.FertilityKnown, state.Fertility)
	}
}

func TestFarmSnapshotUsesOfficialFieldSlotOrder(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr := NewFarmManager(ctx, nil, nil, nil)
	now := time.Now()
	raw := farmRawMilliseconds(now)
	serverSlots := []int{1, 2, 3, 7, 8, 9}
	message := packet.Message{packet.NewMessageElemString(`<xml level="7" fertility="0" ownerName="Tester"/>`)}

	for _, slotID := range serverSlots {
		cropXML := fmt.Sprintf(`<xml onwer="123" fieldprop="%d" itemid="5041234" support="%d" lmtime="%d" supportIndex="0" Special="false" starttime="%d" Fertility="false"/>`, 7000+slotID, slotID, raw+int64(slotID), raw+int64(slotID))
		message = append(message,
			packet.NewMessageElemInt(uint32(slotID)),
			packet.NewMessageElemInt(0),
			packet.NewMessageElemString(cropXML),
		)
	}
	mgr.HandlePacket(&packet.GamePacket{At: now, Op: opcodeFarmSnapshot, Msg: message})

	state := mgr.State(now)
	for index, serverSlotID := range serverSlots {
		plot := state.Plots[index]
		if !plot.Planted || plot.Support != serverSlotID {
			t.Errorf("field grid index %d = %#v, want server slot %d", index, plot, serverSlotID)
		}
	}
}

func TestFarmPlantingIntoEmptySnapshotFieldKeepsOfficialSlot(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr := NewFarmManager(ctx, nil, nil, nil)
	now := time.Now()
	raw := farmRawMilliseconds(now)

	occupiedFields := map[int]bool{1: true, 2: true, 3: true, 7: false, 8: true, 9: true}
	entities := make([]uint64, 0, 24)
	baseEntities := make(map[int]uint64)
	nextEntityID := uint64(9002)
	for slotID := 1; slotID <= 12; slotID++ {
		baseEntities[slotID] = nextEntityID
		entities = append(entities, nextEntityID)
		nextEntityID++
		if farmServerSlotKind(slotID) == "field" && occupiedFields[slotID] {
			entities = append(entities, nextEntityID)
			nextEntityID++
		}
	}
	for offset := uint64(0); offset < 6; offset++ {
		entities = append(entities, nextEntityID+offset)
	}

	itemBySlot := map[int]uint32{
		1: 5041232, 2: 5041233, 3: 5041234,
		4: 5041235, 5: 5041235, 6: 5041237,
		8: 5041232, 9: 5041233, 10: 5041236, 11: 5041236, 12: 5041238,
	}
	makeSnapshot := func(emptySlot int) packet.Message {
		snapshot := packet.Message{packet.NewMessageElemString(`<xml level="12" fertility="80" ownerName="Tester"/>`)}
		for slotID := 1; slotID <= 12; slotID++ {
			rawState := ""
			if itemID := itemBySlot[slotID]; itemID != 0 && slotID != emptySlot {
				rawState = fmt.Sprintf(`<xml onwer="123" fieldprop="%d" itemid="%d" support="0" lmtime="%d" supportIndex="0" Special="false" starttime="%d" Fertility="true"/>`, baseEntities[slotID]+10000, itemID, raw, raw+int64(slotID))
			}
			snapshot = append(snapshot, packet.NewMessageElemInt(uint32(slotID)), packet.NewMessageElemInt(0), packet.NewMessageElemString(rawState))
		}
		return snapshot
	}
	snapshot := makeSnapshot(0)
	mgr.HandlePacket(&packet.GamePacket{At: now, Op: opcodeFarmSnapshot, Msg: snapshot})

	// In real captures the persisted snapshot arrives first and can still contain
	// entity IDs from the previous farm instance. The current entity list follows.
	summary := packet.Message{
		packet.NewMessageElemLong(123),
		packet.NewMessageElemInt(12),
		packet.NewMessageElemInt(80),
		packet.NewMessageElemInt(uint32(len(entities))),
	}
	for _, entityID := range entities {
		summary = append(summary, packet.NewMessageElemLong(entityID))
	}
	mgr.HandlePacket(&packet.GamePacket{At: now.Add(time.Second), Op: opcodeFarmSummary, Msg: summary})
	mgr.mu.RLock()
	inferredSlot := mgr.entitySlots[baseEntities[9]]
	mgr.mu.RUnlock()
	if inferredSlot != 9 {
		t.Fatalf("current field entity slot = %d, want 9", inferredSlot)
	}

	mgr.HandlePacket(farmCropPacket(now.Add(2*time.Second), baseEntities[9], "single", `<xml onwer="123" linkprop="0"/>`))
	if plot := mgr.State(now.Add(2 * time.Second)).Plots[5]; plot.Planted {
		t.Fatalf("live harvest did not clear stale snapshot slot 9: %#v", plot)
	}
	mgr.HandlePacket(&packet.GamePacket{At: now.Add(2500 * time.Millisecond), Op: opcodeFarmSnapshot, Msg: makeSnapshot(9)})
	mgr.mu.RLock()
	inferredSlot = mgr.entitySlots[baseEntities[9]]
	mgr.mu.RUnlock()
	if inferredSlot != 9 {
		t.Fatalf("later snapshot shifted current field entity to slot %d", inferredSlot)
	}

	const cropEntityID = uint64(9999)
	fieldEntityID := baseEntities[7]
	mgr.HandlePacket(farmCropPacket(now.Add(3*time.Second), fieldEntityID, "single", fmt.Sprintf(`<xml onwer="123" linkprop="%d"/>`, cropEntityID)))
	planted := fmt.Sprintf(`<xml onwer="123" fieldprop="%d" itemid="5041234" support="0" lmtime="%d" supportIndex="0" Special="false" starttime="%d" Fertility="true"/>`, fieldEntityID, raw+1000, raw+1000)
	mgr.HandlePacket(farmCropPacket(now.Add(3*time.Second), cropEntityID, "seed", planted))

	state := mgr.State(now.Add(3 * time.Second))
	if plot := state.Plots[3]; !plot.Planted || plot.CropName != "茉莉" {
		t.Fatalf("server slot 7 planting was placed at %#v", plot)
	}
	if plot := state.Plots[0]; !plot.Planted || plot.CropName != "黑莓" {
		t.Fatalf("existing server slot 1 crop moved after planting: %#v", plot)
	}
}

func TestFarmPartialUnlockMapsHarvestAndPlanting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr := NewFarmManager(ctx, nil, nil, nil)
	now := time.Now()
	raw := farmRawMilliseconds(now)

	// This is the exact slot shape observed at farm level 8: slots 1, 2 and 10
	// are absent, while fields 3, 7 and 8 already have crop child entities.
	presentSlots := []int{3, 4, 5, 6, 7, 8, 9, 11, 12}
	occupiedFields := map[int]bool{3: true, 7: true, 8: true, 9: false}
	baseEntities := make(map[int]uint64)
	entities := make([]uint64, 0, 18)
	nextEntityID := uint64(417)
	for _, slotID := range presentSlots {
		baseEntities[slotID] = nextEntityID
		entities = append(entities, nextEntityID)
		nextEntityID++
		if farmServerSlotKind(slotID) == "field" && occupiedFields[slotID] {
			entities = append(entities, nextEntityID)
			nextEntityID++
		}
	}
	for offset := uint64(0); offset < 6; offset++ {
		entities = append(entities, nextEntityID+offset)
	}

	itemBySlot := map[int]uint32{
		3: 5040990, 6: 5041237, 7: 5041234, 8: 5041234,
	}
	snapshot := packet.Message{packet.NewMessageElemString(`<xml level="8" fertility="4" ownerName="Tester"/>`)}
	for _, slotID := range presentSlots {
		rawState := ""
		if itemID := itemBySlot[slotID]; itemID != 0 {
			rawState = fmt.Sprintf(`<xml onwer="123" fieldprop="%d" itemid="%d" support="0" lmtime="%d" supportIndex="0" Special="false" starttime="%d" Fertility="true"/>`, baseEntities[slotID]+10000, itemID, raw, raw+int64(slotID))
		}
		snapshot = append(snapshot, packet.NewMessageElemInt(uint32(slotID)), packet.NewMessageElemInt(0), packet.NewMessageElemString(rawState))
	}
	mgr.HandlePacket(&packet.GamePacket{At: now, Op: opcodeFarmSnapshot, Msg: snapshot})

	summary := packet.Message{
		packet.NewMessageElemLong(123),
		packet.NewMessageElemInt(8),
		packet.NewMessageElemInt(4),
		packet.NewMessageElemInt(uint32(len(entities))),
	}
	for _, entityID := range entities {
		summary = append(summary, packet.NewMessageElemLong(entityID))
	}
	mgr.HandlePacket(&packet.GamePacket{At: now.Add(time.Second), Op: opcodeFarmSummary, Msg: summary})

	if got := mgr.entitySlots[baseEntities[7]]; got != 7 {
		t.Fatalf("farm 4 entity slot = %d, want server slot 7", got)
	}
	if got := mgr.entitySlots[baseEntities[9]]; got != 9 {
		t.Fatalf("farm 6 entity slot = %d, want server slot 9", got)
	}

	// Harvest farm 4 (server slot 7), then reuse its child entity to plant a
	// blackberry in farm 6 (server slot 9), matching the reported capture.
	mgr.HandlePacket(farmCropPacket(now.Add(2*time.Second), baseEntities[7], "single", `<xml onwer="123" linkprop="0"/>`))
	if plot := mgr.State(now.Add(2 * time.Second)).Plots[3]; plot.Planted {
		t.Fatalf("farm 4 was not cleared after harvest: %#v", plot)
	}

	const reusedCropEntityID = uint64(423)
	mgr.HandlePacket(farmCropPacket(now.Add(3*time.Second), baseEntities[9], "single", fmt.Sprintf(`<xml onwer="123" linkprop="%d"/>`, reusedCropEntityID)))
	planted := fmt.Sprintf(`<xml onwer="123" fieldprop="%d" itemid="5041232" support="0" lmtime="%d" supportIndex="0" Special="false" starttime="%d" Fertility="true"/>`, baseEntities[9], raw+3000, raw+3000)
	mgr.HandlePacket(farmCropPacket(now.Add(3*time.Second), reusedCropEntityID, "seed", planted))

	state := mgr.State(now.Add(3 * time.Second))
	if plot := state.Plots[5]; !plot.Planted || plot.CropName != "黑莓" {
		t.Fatalf("farm 6 planting was placed at %#v", plot)
	}
	if plot := state.Plots[0]; plot.Planted {
		t.Fatalf("farm 6 planting incorrectly fell back to farm 1: %#v", plot)
	}
	if got := mgr.entitySlots[baseEntities[7]]; got != 7 {
		t.Fatalf("reused crop changed farm 4 base slot to %d", got)
	}
	if got := mgr.entitySlots[reusedCropEntityID]; got != 9 {
		t.Fatalf("reused crop entity slot = %d, want 9", got)
	}

	// A delayed duplicate clear for the old field must not delete the crop now
	// attached to farm 6.
	mgr.HandlePacket(farmCropPacket(now.Add(4*time.Second), baseEntities[7], "single", `<xml onwer="123" linkprop="0"/>`))
	if plot := mgr.State(now.Add(4 * time.Second)).Plots[5]; !plot.Planted || plot.CropName != "黑莓" {
		t.Fatalf("old field clear deleted reused crop from farm 6: %#v", plot)
	}
}

func TestFarmIgnoresUnrelatedSnapshot(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr := NewFarmManager(ctx, nil, nil, nil)
	now := time.Now()
	raw := farmRawMilliseconds(now)

	farmSnapshot := &packet.GamePacket{
		At: now,
		Op: opcodeFarmSnapshot,
		Msg: packet.Message{
			packet.NewMessageElemString(`<xml level="7" fertility="20" ownerName="Tester"/>`),
			packet.NewMessageElemString(fmt.Sprintf(`<xml onwer="123" fieldprop="1001" itemid="5040989" support="18" lmtime="%d" supportIndex="0" Special="false" starttime="%d" Fertility="true"/>`, raw, raw)),
		},
	}
	mgr.HandlePacket(farmSnapshot)
	if !mgr.State(now).Plots[0].Planted {
		t.Fatal("farm snapshot did not load the crop")
	}

	mgr.HandlePacket(&packet.GamePacket{
		At: now.Add(time.Second),
		Op: opcodeFarmSnapshot,
		Msg: packet.Message{
			packet.NewMessageElemString(`<xml inventory="1" owner="123"/>`),
			packet.NewMessageElemString(`<xml itemid="999" count="10"/>`),
		},
	})
	state := mgr.State(now.Add(time.Second))
	if !state.Plots[0].Planted || state.Plots[0].CropName != "黑莓" {
		t.Fatalf("unrelated snapshot cleared farm crops: %#v", state.Plots[0])
	}
	if state.Fertility != 20 {
		t.Fatalf("unrelated snapshot changed fertility to %d", state.Fertility)
	}
}

func TestFarmQualityThresholds(t *testing.T) {
	cases := map[int]string{0: "normal", 39: "normal", 40: "advanced", 69: "advanced", 70: "highest", 113: "highest"}
	for support, want := range cases {
		if got := qualityForSupport(support); got != want {
			t.Errorf("qualityForSupport(%d) = %q, want %q", support, got, want)
		}
	}
}

func TestFarmSpecialNotificationOnceAndSwitch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	now := time.Now()
	raw := farmRawMilliseconds(now)
	notifications := 0

	mgr := NewFarmManager(ctx, nil, nil, func(plot FarmPlotState) {
		notifications++
		if !plot.Special || plot.CropName != "黑莓" {
			t.Errorf("unexpected special plot: %#v", plot)
		}
	})
	mgr.SetEnabled(true)

	special := fmt.Sprintf(`<xml onwer="123" itemid="5041232" support="0" lmtime="%d" supportIndex="0" Special="true" starttime="%d" Fertility="false" fieldprop="1001"/>`, raw, raw)
	mgr.HandlePacket(farmCropPacket(now, 1001, "seed", special))
	mgr.HandlePacket(farmCropPacket(now.Add(time.Second), 1001, "bud", special))
	if notifications != 1 {
		t.Fatalf("special notifications = %d, want 1", notifications)
	}

	mgr.SetSpecialNotificationEnabled(false)
	secondRaw := raw + 2_000
	secondSpecial := fmt.Sprintf(`<xml onwer="123" itemid="5041233" support="0" lmtime="%d" supportIndex="0" Special="true" starttime="%d" Fertility="false" fieldprop="1002"/>`, secondRaw, secondRaw)
	mgr.HandlePacket(farmCropPacket(now.Add(2*time.Second), 1002, "seed", secondSpecial))
	if notifications != 1 {
		t.Fatalf("disabled special notification fired, count = %d", notifications)
	}
	if mgr.State(now).SpecialNotificationEnabled {
		t.Fatal("special notification switch was not disabled")
	}
}

func TestFarmReadyNotificationDebouncesForThirtySeconds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	now := time.Now()
	raw := farmRawMilliseconds(now.Add(-13 * time.Minute))
	notifications := 0

	mgr := NewFarmManager(ctx, nil, func(FarmPlotState) {
		notifications++
	}, nil)
	cancel()
	mgr.SetEnabled(true)

	mature := fmt.Sprintf(`<xml onwer="123" itemid="5041232" support="40" lmtime="%d" supportIndex="0" Special="false" starttime="%d" Fertility="false" fieldprop="2001"/>`, raw, raw)
	mgr.HandlePacket(farmCropPacket(now, 2001, "completed", mature))
	if notifications != 1 {
		t.Fatalf("ready notifications = %d, want 1", notifications)
	}

	mgr.mu.Lock()
	mgr.crops[2001].Notified = false
	mgr.mu.Unlock()
	mgr.notifyReady(now.Add(10 * time.Second))
	if notifications != 1 {
		t.Fatalf("notification repeated inside debounce window, count = %d", notifications)
	}

	mgr.mu.Lock()
	mgr.crops[2001].Notified = false
	mgr.mu.Unlock()
	mgr.notifyReady(now.Add(31 * time.Second))
	if notifications != 2 {
		t.Fatalf("notification did not fire after debounce window, count = %d", notifications)
	}
}
