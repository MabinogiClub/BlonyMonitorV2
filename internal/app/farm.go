package app

import (
	"context"
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
	dotNetUnixEpochMilliseconds = int64(62_135_596_800_000)
	farmResourceMaximum         = 100
	farmReadyDebounce           = 30 * time.Second
)

type farmCropDefinition struct {
	Name            string
	Kind            string
	DurationSeconds int64
}

var farmCropDefinitions = map[uint32]farmCropDefinition{
	5_040_989: {Name: "黑莓", Kind: "field", DurationSeconds: 12 * 60},
	5_041_232: {Name: "黑莓", Kind: "field", DurationSeconds: 12 * 60},
	5_040_990: {Name: "秋葵", Kind: "field", DurationSeconds: 18 * 60},
	5_041_233: {Name: "秋葵", Kind: "field", DurationSeconds: 18 * 60},
	5_040_991: {Name: "茉莉", Kind: "field", DurationSeconds: 31*60 + 30},
	5_041_234: {Name: "茉莉", Kind: "field", DurationSeconds: 31*60 + 30},
	5_040_992: {Name: "红梨树", Kind: "redPear", DurationSeconds: 21*60 + 30},
	5_041_235: {Name: "红梨树", Kind: "redPear", DurationSeconds: 21*60 + 30},
	5_040_993: {Name: "橡胶树", Kind: "rubber", DurationSeconds: 24 * 60},
	5_041_236: {Name: "橡胶树", Kind: "rubber", DurationSeconds: 24 * 60},
	5_040_994: {Name: "蜘蛛古木", Kind: "spider", DurationSeconds: 16*60 + 30},
	5_041_237: {Name: "蜘蛛古木", Kind: "spider", DurationSeconds: 16*60 + 30},
	5_040_995: {Name: "石英矿脉", Kind: "quartz", DurationSeconds: 18 * 60},
	5_041_238: {Name: "石英矿脉", Kind: "quartz", DurationSeconds: 18 * 60},
}

type farmSlotDefinition struct {
	ServerSlotID int
	Kind         string
	Label        string
}

var farmSlotDefinitions = []farmSlotDefinition{
	{ServerSlotID: 1, Kind: "field", Label: "农田 1"},
	{ServerSlotID: 2, Kind: "field", Label: "农田 2"},
	{ServerSlotID: 3, Kind: "field", Label: "农田 3"},
	{ServerSlotID: 7, Kind: "field", Label: "农田 4"},
	{ServerSlotID: 8, Kind: "field", Label: "农田 5"},
	{ServerSlotID: 9, Kind: "field", Label: "农田 6"},
	{ServerSlotID: 4, Kind: "redPear", Label: "红梨木 1"},
	{ServerSlotID: 10, Kind: "rubber", Label: "橡胶树 1"},
	{ServerSlotID: 6, Kind: "spider", Label: "蜘蛛古木"},
	{ServerSlotID: 5, Kind: "redPear", Label: "红梨木 2"},
	{ServerSlotID: 11, Kind: "rubber", Label: "橡胶树 2"},
	{ServerSlotID: 12, Kind: "quartz", Label: "石英矿脉"},
}

type farmXMLState struct {
	XMLName    xml.Name `xml:"xml"`
	Level      string   `xml:"level,attr"`
	Owner      string   `xml:"onwer,attr"`
	ItemID     string   `xml:"itemid,attr"`
	Support    string   `xml:"support,attr"`
	LastTime   string   `xml:"lmtime,attr"`
	SupportIdx string   `xml:"supportIndex,attr"`
	Special    string   `xml:"Special,attr"`
	StartTime  string   `xml:"starttime,attr"`
	Fertility  string   `xml:"Fertility,attr"`
	FieldProp  string   `xml:"fieldprop,attr"`
	LinkProp   string   `xml:"linkprop,attr"`
	FarmFert   string   `xml:"fertility,attr"`
}

type farmCropRecord struct {
	EntityID        uint64
	SlotID          int
	OwnerID         uint64
	ItemID          uint32
	Phase           string
	Support         int
	SupportIdx      int
	Special         bool
	Fertility       bool
	StartRawMS      int64
	LastRawMS       int64
	Notified        bool
	SpecialNotified bool
	LastUpdated     time.Time
}

// FarmPlotState is the UI-facing state of one fixed farm slot.
type FarmPlotState struct {
	Index            int     `json:"index"`
	Kind             string  `json:"kind"`
	Label            string  `json:"label"`
	EntityID         string  `json:"entityId,omitempty"`
	Planted          bool    `json:"planted"`
	ItemID           uint32  `json:"itemId,omitempty"`
	CropName         string  `json:"cropName,omitempty"`
	Phase            string  `json:"phase,omitempty"`
	Support          int     `json:"support"`
	Quality          string  `json:"quality"`
	Special          bool    `json:"special"`
	Fertility        bool    `json:"fertility"`
	StartedAt        int64   `json:"startedAt,omitempty"`
	ReadyAt          int64   `json:"readyAt,omitempty"`
	TotalSeconds     int64   `json:"totalSeconds,omitempty"`
	RemainingSeconds int64   `json:"remainingSeconds,omitempty"`
	Progress         float64 `json:"progress"`
	Ready            bool    `json:"ready"`
}

// FarmState contains all data needed by the farm tab.
type FarmState struct {
	Enabled                    bool            `json:"enabled"`
	ReadyNotificationEnabled   bool            `json:"readyNotificationEnabled"`
	SpecialNotificationEnabled bool            `json:"specialNotificationEnabled"`
	Fertility                  int             `json:"fertility"`
	FertilityMax               int             `json:"fertilityMax"`
	FertilityKnown             bool            `json:"fertilityKnown"`
	Energy                     int             `json:"energy"`
	EnergyMax                  int             `json:"energyMax"`
	EnergyKnown                bool            `json:"energyKnown"`
	Synced                     bool            `json:"synced"`
	UpdatedAt                  int64           `json:"updatedAt"`
	Plots                      []FarmPlotState `json:"plots"`
}

type FarmManager struct {
	mu                         sync.RWMutex
	enabled                    bool
	readyNotificationEnabled   bool
	specialNotificationEnabled bool
	fertility                  int
	energy                     int
	fertilityKnown             bool
	energyKnown                bool
	synced                     bool
	updatedAt                  time.Time
	crops                      map[uint64]*farmCropRecord
	entityLinks                map[uint64]map[uint64]struct{}
	entitySlots                map[uint64]int
	summaryEntityIDs           []uint64
	summarySlotsInferred       bool
	snapshotSlotsPresent       map[int]bool
	snapshotFieldOccupied      map[int]bool
	clockOffset                time.Duration
	clockSamples               int
	readyNotifiedAt            map[string]time.Time
	onState                    func(FarmState)
	onReady                    func(FarmPlotState)
	onSpecial                  func(FarmPlotState)
}

func NewFarmManager(ctx context.Context, onState func(FarmState), onReady func(FarmPlotState), onSpecial func(FarmPlotState)) *FarmManager {
	m := &FarmManager{
		crops:                      make(map[uint64]*farmCropRecord),
		entityLinks:                make(map[uint64]map[uint64]struct{}),
		entitySlots:                make(map[uint64]int),
		snapshotSlotsPresent:       make(map[int]bool),
		snapshotFieldOccupied:      make(map[int]bool),
		readyNotificationEnabled:   true,
		specialNotificationEnabled: true,
		readyNotifiedAt:            make(map[string]time.Time),
		onState:                    onState,
		onReady:                    onReady,
		onSpecial:                  onSpecial,
	}
	go m.run(ctx)
	return m
}

func (m *FarmManager) run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			m.notifyReady(now)
		}
	}
}

func (m *FarmManager) SetEnabled(enabled bool) {
	m.mu.Lock()
	m.enabled = enabled
	m.updatedAt = time.Now()
	m.mu.Unlock()
	m.emitState()
	if enabled {
		m.notifyReady(time.Now())
	}
}

func (m *FarmManager) SetReadyNotificationEnabled(enabled bool) {
	m.mu.Lock()
	m.readyNotificationEnabled = enabled
	m.updatedAt = time.Now()
	m.mu.Unlock()
	m.emitState()
	if enabled {
		m.notifyReady(time.Now())
	}
}

func (m *FarmManager) SetSpecialNotificationEnabled(enabled bool) {
	m.mu.Lock()
	m.specialNotificationEnabled = enabled
	m.updatedAt = time.Now()
	m.mu.Unlock()
	m.emitState()
}

func (m *FarmManager) HandlePacket(pkt *packet.GamePacket) {
	if pkt == nil || pkt.Msg == nil {
		return
	}

	changed := false
	specialDetected := false
	switch pkt.Op {
	case opcodeFarmCropState:
		changed, specialDetected = m.handleCropState(pkt)
	case opcodeFarmEntityList:
		changed = m.handleFarmEntityList(pkt)
	case opcodeFarmSummary:
		changed = m.handleFarmSummary(pkt)
	case opcodeFarmSnapshot:
		changed = m.handleFarmSnapshot(pkt)
	case opcodeFarmEnergy:
		changed = m.handleFarmEnergy(pkt)
	}
	if changed {
		m.emitState()
		m.notifyReady(pkt.At)
	}
	if specialDetected {
		m.notifySpecial(pkt.Id, pkt.At)
	}
}

func (m *FarmManager) handleFarmEntityList(pkt *packet.GamePacket) bool {
	entityIDs := farmSummaryEntityIDs(pkt.Msg)
	if len(entityIDs) == 0 {
		return false
	}
	m.mu.Lock()
	m.rememberSummaryEntitiesLocked(entityIDs)
	m.inferSnapshotEntitySlotsLocked(m.snapshotSlotsPresent, m.snapshotFieldOccupied, m.crops)
	m.synced = true
	m.updatedAt = pkt.At
	m.mu.Unlock()
	return true
}

func (m *FarmManager) handleFarmSummary(pkt *packet.GamePacket) bool {
	if len(pkt.Msg) < 3 || pkt.Msg[2].Type() != packet.MessageElemTypeInt {
		return false
	}
	fertility := int(pkt.Msg[2].Data().(uint32))
	entityIDs := farmSummaryEntityIDs(pkt.Msg)
	m.mu.Lock()
	changed := !m.fertilityKnown || m.fertility != fertility
	m.fertility = fertility
	m.fertilityKnown = true
	m.synced = true
	m.updatedAt = pkt.At
	m.rememberSummaryEntitiesLocked(entityIDs)
	m.inferSnapshotEntitySlotsLocked(m.snapshotSlotsPresent, m.snapshotFieldOccupied, m.crops)
	m.mu.Unlock()
	return changed
}

func farmSummaryEntityIDs(message packet.Message) []uint64 {
	var result []uint64
	for index, elem := range message {
		if elem.Type() != packet.MessageElemTypeInt {
			continue
		}
		count := int(elem.Data().(uint32))
		if count < len(farmSlotDefinitions) || count > 64 || index+count >= len(message) {
			continue
		}
		candidate := make([]uint64, count)
		valid := true
		for offset := 0; offset < count; offset++ {
			entity := message[index+1+offset]
			if entity.Type() != packet.MessageElemTypeLong {
				valid = false
				break
			}
			candidate[offset] = entity.Data().(uint64)
		}
		if valid && len(candidate) > len(result) {
			result = candidate
		}
	}
	return result
}

func (m *FarmManager) rememberSummaryEntitiesLocked(entityIDs []uint64) {
	if len(entityIDs) == 0 {
		return
	}
	if len(m.summaryEntityIDs) != 0 {
		known := make(map[uint64]struct{}, len(m.summaryEntityIDs))
		for _, entityID := range m.summaryEntityIDs {
			known[entityID] = struct{}{}
		}
		for _, entityID := range entityIDs {
			if _, exists := known[entityID]; exists {
				return
			}
		}
	}
	m.summaryEntityIDs = append(m.summaryEntityIDs[:0], entityIDs...)
	m.summarySlotsInferred = false
}

func (m *FarmManager) handleFarmEnergy(pkt *packet.GamePacket) bool {
	if len(pkt.Msg) < 2 || pkt.Msg[1].Type() != packet.MessageElemTypeInt {
		return false
	}
	energy := int(pkt.Msg[1].Data().(uint32))
	m.mu.Lock()
	changed := !m.energyKnown || m.energy != energy
	m.energy = energy
	m.energyKnown = true
	m.updatedAt = pkt.At
	m.mu.Unlock()
	return changed
}

func (m *FarmManager) handleFarmSnapshot(pkt *packet.GamePacket) bool {
	newCrops := make(map[uint64]*farmCropRecord)
	slotsPresent := make(map[int]bool)
	fieldOccupied := make(map[int]bool)
	fertility := -1

	for index, elem := range pkt.Msg {
		if elem.Type() != packet.MessageElemTypeString {
			continue
		}
		raw := strings.TrimSpace(elem.Data().(string))
		slotID := farmSnapshotSlotID(pkt.Msg, index)
		if slotID != 0 {
			slotsPresent[slotID] = true
		}
		if slotID != 0 && farmServerSlotKind(slotID) == "field" {
			if _, exists := fieldOccupied[slotID]; !exists {
				fieldOccupied[slotID] = false
			}
		}
		if !strings.HasPrefix(raw, "<xml") {
			continue
		}
		attrs, ok := parseFarmXML(raw)
		if !ok {
			continue
		}
		if attrs.FarmFert != "" {
			if value, err := strconv.Atoi(attrs.FarmFert); err == nil {
				fertility = value
			}
		}
		record, ok := cropRecordFromXML(attrs, 0, "growing", pkt.At)
		if !ok || record.ItemID == 0 {
			continue
		}
		record.SlotID = slotID
		if farmServerSlotKind(slotID) == "field" {
			fieldOccupied[slotID] = true
		}
		if record.EntityID == 0 {
			record.EntityID = uint64(record.StartRawMS)
		}
		newCrops[record.EntityID] = record
	}
	// 0x21394 is also used by non-farm snapshots. Only the farm variant has
	// the farm metadata XML with a lowercase fertility attribute.
	if fertility < 0 {
		return false
	}

	m.mu.Lock()
	m.snapshotSlotsPresent = make(map[int]bool, len(slotsPresent))
	for slotID := range slotsPresent {
		m.snapshotSlotsPresent[slotID] = true
	}
	m.snapshotFieldOccupied = make(map[int]bool, len(fieldOccupied))
	for slotID, occupied := range fieldOccupied {
		m.snapshotFieldOccupied[slotID] = occupied
	}
	m.inferSnapshotEntitySlotsLocked(slotsPresent, fieldOccupied, newCrops)
	for id, record := range newCrops {
		if old := m.findSameCropLocked(record); old != nil {
			record.Notified = old.Notified
			record.SpecialNotified = old.SpecialNotified
			if record.SlotID == 0 {
				record.SlotID = old.SlotID
			}
		}
		if record.SlotID != 0 {
			m.setEntitySlotLocked(id, record.SlotID)
		}
		newCrops[id] = record
	}
	m.crops = newCrops
	m.fertility = fertility
	m.fertilityKnown = true
	m.synced = true
	m.updatedAt = pkt.At
	m.mu.Unlock()
	return true
}

func farmServerSlotKind(slotID int) string {
	for _, definition := range farmSlotDefinitions {
		if definition.ServerSlotID == slotID {
			return definition.Kind
		}
	}
	return ""
}

func (m *FarmManager) inferSnapshotEntitySlotsLocked(slotsPresent map[int]bool, fieldOccupied map[int]bool, crops map[uint64]*farmCropRecord) {
	if m.summarySlotsInferred || len(slotsPresent) == 0 {
		return
	}

	serverSlotIDs := make([]int, 0, len(slotsPresent))
	windowLength := len(slotsPresent)
	for slotID := 1; slotID <= len(farmSlotDefinitions); slotID++ {
		if !slotsPresent[slotID] {
			continue
		}
		serverSlotIDs = append(serverSlotIDs, slotID)
		if farmServerSlotKind(slotID) == "field" && fieldOccupied[slotID] {
			windowLength++
		}
	}
	anchors := make(map[int]uint64)
	for _, crop := range crops {
		if crop.SlotID != 0 && crop.EntityID != 0 {
			anchors[crop.SlotID] = crop.EntityID
		}
	}
	if windowLength > len(m.summaryEntityIDs) {
		return
	}

	entities := m.summaryEntityIDs[:windowLength]
	for index := 1; index < len(entities); index++ {
		if entities[index] != entities[index-1]+1 {
			return
		}
	}

	inferred := make(map[int][]uint64, len(serverSlotIDs))
	position := 0
	for _, slotID := range serverSlotIDs {
		baseEntityID := entities[position]
		position++
		inferred[slotID] = []uint64{baseEntityID}
		if anchor := anchors[slotID]; anchor != 0 && containsFarmEntityID(m.summaryEntityIDs, anchor) && anchor != baseEntityID {
			return
		}
		if farmServerSlotKind(slotID) == "field" && fieldOccupied[slotID] {
			inferred[slotID] = append(inferred[slotID], entities[position])
			position++
		}
	}
	for slotID, entityIDs := range inferred {
		m.setEntitySlotLocked(entityIDs[0], slotID)
		if len(entityIDs) == 2 {
			m.bindFieldCropLocked(entityIDs[0], entityIDs[1], slotID)
		}
	}
	m.summarySlotsInferred = true
}

func containsFarmEntityID(entityIDs []uint64, target uint64) bool {
	for _, entityID := range entityIDs {
		if entityID == target {
			return true
		}
	}
	return false
}

func farmSnapshotSlotID(message packet.Message, xmlIndex int) int {
	if xmlIndex < 2 || message[xmlIndex-2].Type() != packet.MessageElemTypeInt {
		return 0
	}
	slotID := int(message[xmlIndex-2].Data().(uint32))
	if slotID < 1 || slotID > 12 {
		return 0
	}
	return slotID
}

func (m *FarmManager) handleCropState(pkt *packet.GamePacket) (bool, bool) {
	if len(pkt.Msg) < 4 || pkt.Msg[0].Type() != packet.MessageElemTypeString || pkt.Msg[3].Type() != packet.MessageElemTypeString {
		return false, false
	}
	phase := strings.ToLower(strings.TrimSpace(pkt.Msg[0].Data().(string)))
	attrs, ok := parseFarmXML(pkt.Msg[3].Data().(string))
	if !ok {
		return false, false
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	fieldEntityID := parseUint64Fallback(attrs.FieldProp)
	linkEntityID := parseUint64Fallback(attrs.LinkProp)
	if fieldEntityID != 0 && fieldEntityID != pkt.Id {
		m.bindFieldCropLocked(fieldEntityID, pkt.Id, m.entitySlots[fieldEntityID])
	} else if linkEntityID != 0 && linkEntityID != pkt.Id {
		m.bindFieldCropLocked(pkt.Id, linkEntityID, m.entitySlots[pkt.Id])
	}
	if attrs.ItemID == "" {
		if phase == "single" && attrs.LinkProp == "0" {
			deleted := m.deleteLinkedCropsLocked(pkt.Id)
			m.clearEntityLinksLocked(pkt.Id)
			if deleted {
				m.updatedAt = pkt.At
				return true, false
			}
		}
		return false, false
	}

	itemID, err := strconv.ParseUint(attrs.ItemID, 10, 32)
	if err != nil {
		return false, false
	}

	if itemID == 0 {
		deleted := m.deleteLinkedCropsLocked(pkt.Id)
		m.clearEntityLinksLocked(pkt.Id)
		if deleted {
			m.updatedAt = pkt.At
			return true, false
		}
		return false, false
	}

	record, valid := cropRecordFromXML(attrs, pkt.Id, phase, pkt.At)
	if !valid {
		return false, false
	}
	record.SlotID = m.entitySlotLocked(pkt.Id)

	old := m.findLinkedCropLocked(pkt.Id)
	if old == nil {
		old = m.findSameCropLocked(record)
	}
	if old != nil && old.EntityID != pkt.Id {
		delete(m.crops, old.EntityID)
	}
	if old == nil || record.LastRawMS != old.LastRawMS {
		m.calibrateClockLocked(pkt.At, record.LastRawMS)
	}
	if old != nil {
		if record.SlotID == 0 {
			record.SlotID = old.SlotID
		}
		if attrs.Fertility == "" {
			record.Fertility = old.Fertility
		}
		if attrs.Special == "" {
			record.Special = old.Special
		}
		if record.StartRawMS == old.StartRawMS {
			record.Notified = old.Notified
		}
		record.SpecialNotified = old.SpecialNotified
	}
	if record.SlotID != 0 {
		m.setEntitySlotLocked(pkt.Id, record.SlotID)
	}
	specialDetected := record.Special && !record.SpecialNotified && (old == nil || !old.Special)
	m.crops[pkt.Id] = record
	m.synced = true
	m.updatedAt = pkt.At
	return true, specialDetected
}

func (m *FarmManager) bindFieldCropLocked(fieldEntityID, cropEntityID uint64, slotID int) {
	if fieldEntityID == 0 || cropEntityID == 0 || fieldEntityID == cropEntityID {
		return
	}
	if slotID == 0 {
		slotID = m.entitySlots[fieldEntityID]
	}
	if slotID == 0 {
		slotID = m.entitySlots[cropEntityID]
	}

	// Crop entity IDs are reused after harvest. Keep only the current pair so
	// an old field cannot clear a crop that has since moved to another field.
	m.clearEntityLinksLocked(fieldEntityID)
	m.clearEntityLinksLocked(cropEntityID)
	m.entityLinks[fieldEntityID] = map[uint64]struct{}{cropEntityID: {}}
	m.entityLinks[cropEntityID] = map[uint64]struct{}{fieldEntityID: {}}
	if slotID != 0 {
		m.entitySlots[fieldEntityID] = slotID
		m.entitySlots[cropEntityID] = slotID
	}
}

func (m *FarmManager) clearEntityLinksLocked(entityID uint64) {
	for linked := range m.entityLinks[entityID] {
		delete(m.entityLinks[linked], entityID)
		if len(m.entityLinks[linked]) == 0 {
			delete(m.entityLinks, linked)
		}
	}
	delete(m.entityLinks, entityID)
}

func (m *FarmManager) linkedEntityIDsLocked(entityID uint64) []uint64 {
	if entityID == 0 {
		return nil
	}
	result := make([]uint64, 0, 1+len(m.entityLinks[entityID]))
	result = append(result, entityID)
	for linked := range m.entityLinks[entityID] {
		result = append(result, linked)
	}
	return result
}

func (m *FarmManager) findLinkedCropLocked(entityID uint64) *farmCropRecord {
	for _, linked := range m.linkedEntityIDsLocked(entityID) {
		if crop := m.crops[linked]; crop != nil {
			return crop
		}
	}
	return nil
}

func (m *FarmManager) entitySlotLocked(entityID uint64) int {
	if slotID := m.entitySlots[entityID]; slotID != 0 {
		return slotID
	}
	for linked := range m.entityLinks[entityID] {
		if slotID := m.entitySlots[linked]; slotID != 0 {
			return slotID
		}
	}
	return 0
}

func (m *FarmManager) setEntitySlotLocked(entityID uint64, slotID int) {
	if entityID == 0 || slotID < 1 || slotID > 12 {
		return
	}
	m.entitySlots[entityID] = slotID
}

func (m *FarmManager) deleteLinkedCropsLocked(entityID uint64) bool {
	deleted := false
	slotID := m.entitySlotLocked(entityID)
	for _, linked := range m.linkedEntityIDsLocked(entityID) {
		if _, exists := m.crops[linked]; !exists {
			continue
		}
		delete(m.crops, linked)
		deleted = true
	}
	if slotID != 0 {
		for cropEntityID, crop := range m.crops {
			if crop.SlotID != slotID {
				continue
			}
			delete(m.crops, cropEntityID)
			deleted = true
		}
	}
	return deleted
}

func parseFarmXML(raw string) (farmXMLState, bool) {
	var attrs farmXMLState
	if err := xml.Unmarshal([]byte(raw), &attrs); err != nil {
		return farmXMLState{}, false
	}
	return attrs, true
}

func cropRecordFromXML(attrs farmXMLState, packetID uint64, phase string, at time.Time) (*farmCropRecord, bool) {
	itemID64, err := strconv.ParseUint(attrs.ItemID, 10, 32)
	if err != nil {
		return nil, false
	}
	itemID := uint32(itemID64)
	if itemID != 0 {
		if _, known := farmCropDefinitions[itemID]; !known {
			return nil, false
		}
	}

	entityID := packetID
	if entityID == 0 {
		entityID = parseUint64Fallback(attrs.FieldProp, attrs.LinkProp)
	}
	return &farmCropRecord{
		EntityID:    entityID,
		OwnerID:     parseUint64Fallback(attrs.Owner),
		ItemID:      itemID,
		Phase:       phase,
		Support:     parseInt(attrs.Support),
		SupportIdx:  parseInt(attrs.SupportIdx),
		Special:     strings.EqualFold(attrs.Special, "true"),
		Fertility:   strings.EqualFold(attrs.Fertility, "true"),
		StartRawMS:  parseInt64(attrs.StartTime),
		LastRawMS:   parseInt64(attrs.LastTime),
		LastUpdated: at,
	}, true
}

func parseInt(value string) int {
	parsed, _ := strconv.Atoi(value)
	return parsed
}

func parseInt64(value string) int64 {
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed
}

func parseUint64Fallback(values ...string) uint64 {
	for _, value := range values {
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err == nil && parsed != 0 {
			return parsed
		}
	}
	return 0
}

func (m *FarmManager) findSameCropLocked(candidate *farmCropRecord) *farmCropRecord {
	for _, crop := range m.crops {
		if crop.OwnerID == candidate.OwnerID && crop.ItemID == candidate.ItemID && crop.StartRawMS == candidate.StartRawMS {
			return crop
		}
	}
	return nil
}

func (m *FarmManager) calibrateClockLocked(packetAt time.Time, serverRawMS int64) {
	if serverRawMS <= dotNetUnixEpochMilliseconds {
		return
	}
	serverWall := dotNetMillisecondsToWallTime(serverRawMS, packetAt.Location())
	offset := packetAt.Sub(serverWall)
	if offset < -30*time.Second || offset > 30*time.Second {
		return
	}
	if m.clockSamples == 0 {
		m.clockOffset = offset
	} else {
		m.clockOffset = (m.clockOffset*time.Duration(m.clockSamples) + offset) / time.Duration(m.clockSamples+1)
	}
	if m.clockSamples < 9 {
		m.clockSamples++
	}
}

func dotNetMillisecondsToWallTime(rawMS int64, location *time.Location) time.Time {
	unixLike := time.UnixMilli(rawMS - dotNetUnixEpochMilliseconds).UTC()
	year, month, day := unixLike.Date()
	hour, minute, second := unixLike.Clock()
	return time.Date(year, month, day, hour, minute, second, unixLike.Nanosecond(), location)
}

func cropDuration(record *farmCropRecord) int64 {
	definition, ok := farmCropDefinitions[record.ItemID]
	if !ok {
		return 0
	}
	if record.Special {
		return 2 * 60
	}
	if record.Fertility {
		return definition.DurationSeconds / 5
	}
	return definition.DurationSeconds
}

func qualityForSupport(support int) string {
	switch {
	case support >= 70:
		return "highest"
	case support >= 40:
		return "advanced"
	default:
		return "normal"
	}
}

func isReadyPhase(phase string) bool {
	return phase == "completed" || phase == "collecting"
}

func (m *FarmManager) plotFromRecordLocked(record *farmCropRecord, index int, slot farmSlotDefinition, now time.Time) FarmPlotState {
	cropDefinition := farmCropDefinitions[record.ItemID]
	totalSeconds := cropDuration(record)
	startedAt := dotNetMillisecondsToWallTime(record.StartRawMS, now.Location()).Add(m.clockOffset)
	readyAt := startedAt.Add(time.Duration(totalSeconds) * time.Second)
	ready := isReadyPhase(record.Phase) || !now.Before(readyAt)
	remaining := int64(math.Ceil(readyAt.Sub(now).Seconds()))
	if remaining < 0 || ready {
		remaining = 0
	}
	progress := float64(0)
	if totalSeconds > 0 {
		progress = float64(totalSeconds-remaining) / float64(totalSeconds) * 100
		progress = math.Max(0, math.Min(100, progress))
	}

	return FarmPlotState{
		Index:            index,
		Kind:             slot.Kind,
		Label:            slot.Label,
		EntityID:         strconv.FormatUint(record.EntityID, 10),
		Planted:          true,
		ItemID:           record.ItemID,
		CropName:         cropDefinition.Name,
		Phase:            record.Phase,
		Support:          record.Support,
		Quality:          qualityForSupport(record.Support),
		Special:          record.Special,
		Fertility:        record.Fertility,
		StartedAt:        startedAt.UnixMilli(),
		ReadyAt:          readyAt.UnixMilli(),
		TotalSeconds:     totalSeconds,
		RemainingSeconds: remaining,
		Progress:         progress,
		Ready:            ready,
	}
}

func (m *FarmManager) buildPlotsLocked(now time.Time) []FarmPlotState {
	plots := make([]FarmPlotState, len(farmSlotDefinitions))
	slotIndexByServerID := make(map[int]int, len(farmSlotDefinitions))
	for index, definition := range farmSlotDefinitions {
		plots[index] = FarmPlotState{Index: index, Kind: definition.Kind, Label: definition.Label, Quality: "empty"}
		slotIndexByServerID[definition.ServerSlotID] = index
	}

	positioned := make(map[int]*farmCropRecord)
	unpositioned := make([]*farmCropRecord, 0, len(m.crops))
	for _, record := range m.crops {
		cropDefinition, knownCrop := farmCropDefinitions[record.ItemID]
		index, knownSlot := slotIndexByServerID[record.SlotID]
		if !knownCrop || !knownSlot || farmSlotDefinitions[index].Kind != cropDefinition.Kind {
			unpositioned = append(unpositioned, record)
			continue
		}
		if current := positioned[index]; current == nil || record.LastUpdated.After(current.LastUpdated) {
			positioned[index] = record
		}
	}

	occupied := make(map[int]bool, len(positioned))
	for index, record := range positioned {
		plots[index] = m.plotFromRecordLocked(record, index, farmSlotDefinitions[index], now)
		occupied[index] = true
	}

	availableByKind := make(map[string][]int)
	for index, definition := range farmSlotDefinitions {
		if !occupied[index] {
			availableByKind[definition.Kind] = append(availableByKind[definition.Kind], index)
		}
	}
	sort.Slice(unpositioned, func(i, j int) bool {
		if unpositioned[i].StartRawMS == unpositioned[j].StartRawMS {
			return unpositioned[i].EntityID < unpositioned[j].EntityID
		}
		return unpositioned[i].StartRawMS < unpositioned[j].StartRawMS
	})

	usedByKind := make(map[string]int)
	for _, record := range unpositioned {
		definition, ok := farmCropDefinitions[record.ItemID]
		if !ok {
			continue
		}
		available := availableByKind[definition.Kind]
		position := usedByKind[definition.Kind]
		if position >= len(available) {
			continue
		}
		index := available[position]
		usedByKind[definition.Kind]++
		plots[index] = m.plotFromRecordLocked(record, index, farmSlotDefinitions[index], now)
	}
	return plots
}

func (m *FarmManager) State(now time.Time) FarmState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	updatedAt := int64(0)
	if !m.updatedAt.IsZero() {
		updatedAt = m.updatedAt.UnixMilli()
	}
	return FarmState{
		Enabled:                    m.enabled,
		ReadyNotificationEnabled:   m.readyNotificationEnabled,
		SpecialNotificationEnabled: m.specialNotificationEnabled,
		Fertility:                  m.fertility,
		FertilityMax:               farmResourceMaximum,
		FertilityKnown:             m.fertilityKnown,
		Energy:                     m.energy,
		EnergyMax:                  farmResourceMaximum,
		EnergyKnown:                m.energyKnown,
		Synced:                     m.synced,
		UpdatedAt:                  updatedAt,
		Plots:                      m.buildPlotsLocked(now),
	}
}

func (m *FarmManager) notifyReady(now time.Time) {
	if m.onReady == nil {
		return
	}

	readyPlots := make([]FarmPlotState, 0)
	m.mu.Lock()
	if !m.enabled || !m.readyNotificationEnabled {
		m.mu.Unlock()
		return
	}
	for key, notifiedAt := range m.readyNotifiedAt {
		if now.Sub(notifiedAt) >= farmReadyDebounce {
			delete(m.readyNotifiedAt, key)
		}
	}
	for _, plot := range m.buildPlotsLocked(now) {
		if !plot.Planted || !plot.Ready {
			continue
		}
		entityID, _ := strconv.ParseUint(plot.EntityID, 10, 64)
		record := m.crops[entityID]
		if record == nil || record.Notified {
			continue
		}
		key := farmReadyDedupeKey(record)
		if notifiedAt, exists := m.readyNotifiedAt[key]; exists && now.Sub(notifiedAt) < farmReadyDebounce {
			record.Notified = true
			continue
		}
		record.Notified = true
		m.readyNotifiedAt[key] = now
		readyPlots = append(readyPlots, plot)
	}
	m.mu.Unlock()

	for _, plot := range readyPlots {
		m.onReady(plot)
	}
}

func farmReadyDedupeKey(record *farmCropRecord) string {
	return fmt.Sprintf("%d:%d:%d", record.OwnerID, record.ItemID, record.StartRawMS)
}

func (m *FarmManager) notifySpecial(entityID uint64, now time.Time) {
	if m.onSpecial == nil {
		return
	}

	var specialPlot *FarmPlotState
	m.mu.Lock()
	record := m.crops[entityID]
	if record == nil || !record.Special || record.SpecialNotified {
		m.mu.Unlock()
		return
	}
	record.SpecialNotified = true
	if m.enabled && m.specialNotificationEnabled {
		for _, plot := range m.buildPlotsLocked(now) {
			if plot.EntityID == strconv.FormatUint(entityID, 10) {
				candidate := plot
				specialPlot = &candidate
				break
			}
		}
	}
	m.mu.Unlock()

	if specialPlot != nil {
		m.onSpecial(*specialPlot)
	}
}

func (m *FarmManager) emitState() {
	if m.onState != nil {
		m.onState(m.State(time.Now()))
	}
}

func playFarmSound(filename string) {
	soundDir := resolveSoundDir()
	if soundDir == "" {
		logger.Printf("[Farm] 音效目录不存在: %s", filename)
		return
	}
	audioFile := filepath.Join(soundDir, filename)
	if _, err := os.Stat(audioFile); err != nil {
		logger.Printf("[Farm] 音效文件不存在: %s", audioFile)
		return
	}
	go playWavFile(audioFile)
}
