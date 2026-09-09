package packet

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ItemOption 是 0x5be0 属性串后的 40 字节选项块（释放效果等）。
type ItemOption struct {
	ApplyType uint32
	StatID    int16
	StatValue int16
	OptionID  uint32
	RawHex    string
}

// ItemDetail 是 0x5be0 中的一条装备实例记录。
// 该包同时包含装备公共块、扩展块和客户端属性字符串。
type ItemDetail struct {
	InstanceID      uint64
	Item            *EntityItem
	ExtendedData    []byte
	DurabilityRaw   uint32
	DurabilityValue float32
	DurabilityMax   *float32
	Attributes      string
	OwnerName       string
	MakerName       string
	Quality         *int
	RepairCount     *int
	RepairBaseValue *float32
	MaxWear         *int
	Defense         *int
	Protection      *int
	MagicDefense    *float32
	MagicProtection *float32
	AttackMin       *int
	Options         []ItemOption
	UpgradeEntries  []string
	SpecialEntries  []string
	StatEntries     []string
	EnchantEntries  []string
	ReforgeEntries  []string
	EgoEntries      []string
}

// ParseItemDetailPacket extracts item records without depending on the
// optional empty-slot fields that vary between client versions.
func ParseItemDetailPacket(msg Message) []ItemDetail {
	result := make([]ItemDetail, 0)
	seen := make(map[uint64]struct{})

	for i := 0; i < len(msg); i++ {
		if msg[i].Type() != MessageElemTypeByte || msg[i].Data().(uint8) != 2 {
			continue
		}
		detail, ok := parseItemDetailAt(msg, i)
		if !ok {
			continue
		}
		dedupeKey := detail.InstanceID
		if dedupeKey == 0 && detail.Item != nil {
			dedupeKey = uint64(detail.Item.PocketType)<<32 | uint64(detail.Item.ItemId)
		}
		if _, exists := seen[dedupeKey]; exists {
			continue
		}
		seen[dedupeKey] = struct{}{}
		result = append(result, detail)
	}
	return result
}

func parseItemDetailAt(msg Message, flagIndex int) (ItemDetail, bool) {
	if flagIndex+2 >= len(msg) {
		return ItemDetail{}, false
	}

	publicIndex := flagIndex + 1
	publicData, ok := binOfPublicItem(msg[publicIndex])
	if !ok {
		return ItemDetail{}, false
	}

	extendedIndex := -1
	attrStart := -1

	// 常见布局: byte(2) + bin[80|84] + bin[144] + string(+string)
	if _, ok := binOfLen(msg[flagIndex+2], 144); ok {
		extendedIndex = flagIndex + 2
		attrStart = flagIndex + 3
	} else if flagIndex+3 < len(msg) && msg[flagIndex+2].Type() == MessageElemTypeString {
		// 变体布局: byte(2) + bin[80|84] + string + bin[144] + string(+string)
		if _, ok := binOfLen(msg[flagIndex+3], 144); ok {
			extendedIndex = flagIndex + 3
			attrStart = flagIndex + 4
		}
	}
	if extendedIndex < 0 || attrStart < 0 {
		return ItemDetail{}, false
	}

	extendedData, _ := binOfLen(msg[extendedIndex], 144)
	item, err := EntityItemReader(publicData)
	if err != nil || item.ItemId == 0 {
		return ItemDetail{}, false
	}

	attributes, nextIndex := joinAttributeStrings(msg, flagIndex+2, attrStart)
	detail := ItemDetail{
		Item:          item,
		ExtendedData:  append([]byte(nil), extendedData...),
		DurabilityRaw: binary.LittleEndian.Uint32(extendedData[:4]),
		Attributes:    attributes,
	}
	parseExtendedStats(&detail, extendedData)
	if flagIndex >= 1 && msg[flagIndex-1].Type() == MessageElemTypeLong {
		detail.InstanceID = msg[flagIndex-1].Data().(uint64)
	}
	detail.Options = parseItemOptions(msg, nextIndex)
	parseItemAttributes(&detail)
	buildReadableEntries(&detail)
	return detail, true
}

func binOfPublicItem(elem IMessageElem) ([]byte, bool) {
	if elem == nil || elem.Type() != MessageElemTypeBin {
		return nil, false
	}
	data, ok := elem.Data().([]byte)
	if !ok || !IsPublicItemBlock(data) {
		return nil, false
	}
	return data, true
}

func binOfLen(elem IMessageElem, size int) ([]byte, bool) {
	if elem == nil || elem.Type() != MessageElemTypeBin {
		return nil, false
	}
	data, ok := elem.Data().([]byte)
	if !ok || len(data) != size {
		return nil, false
	}
	return data, true
}

func joinAttributeStrings(msg Message, midIndex, attrStart int) (string, int) {
	parts := make([]string, 0, 3)
	// 夹在两个 bin 中间的 string（可能为空，也可能含前置属性）
	if midIndex < attrStart && midIndex < len(msg) && msg[midIndex].Type() == MessageElemTypeString {
		if text, ok := msg[midIndex].Data().(string); ok && text != "" {
			parts = append(parts, text)
		}
	}
	end := attrStart
	for i := attrStart; i < len(msg); i++ {
		if msg[i].Type() != MessageElemTypeString {
			break
		}
		text, ok := msg[i].Data().(string)
		if !ok {
			break
		}
		end = i + 1
		if text == "" {
			continue
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, ""), end
}

func parseExtendedStats(detail *ItemDetail, extended []byte) {
	if len(extended) < 48 {
		detail.DurabilityValue = decodeDurability(detail.DurabilityRaw)
		return
	}

	currentPts := binary.LittleEndian.Uint32(extended[16:])
	maxPts := binary.LittleEndian.Uint32(extended[20:])
	if maxPts >= 1000 && maxPts <= 500000 && currentPts <= maxPts*2 {
		current := float32(currentPts) / 1000
		max := float32(maxPts) / 1000
		detail.DurabilityValue = current
		detail.DurabilityMax = &max
	} else {
		detail.DurabilityValue = decodeDurability(detail.DurabilityRaw)
	}

	defense := int(binary.LittleEndian.Uint32(extended[40:]))
	protection := int(binary.LittleEndian.Uint32(extended[44:]))
	if defense >= 0 && defense <= 500 {
		detail.Defense = &defense
	}
	if protection >= 0 && protection <= 500 {
		detail.Protection = &protection
	}
}

func parseItemOptions(msg Message, start int) []ItemOption {
	if start >= len(msg) || msg[start].Type() != MessageElemTypeByte {
		return nil
	}
	count := int(msg[start].Data().(uint8))
	if count <= 0 || count > 64 {
		return nil
	}
	options := make([]ItemOption, 0, count)
	index := start + 1
	for n := 0; n < count && index < len(msg); n++ {
		data, ok := binOfLen(msg[index], 40)
		index++
		if !ok {
			break
		}
		opt := ItemOption{
			ApplyType: binary.LittleEndian.Uint32(data[0:]),
			StatID:    int16(binary.LittleEndian.Uint16(data[12:])),
			StatValue: int16(binary.LittleEndian.Uint16(data[14:])),
			OptionID:  binary.LittleEndian.Uint32(data[36:]),
			RawHex:    fmt.Sprintf("%x", data),
		}
		options = append(options, opt)
	}
	return options
}

func decodeDurability(raw uint32) float32 {
	value := math.Float32frombits(raw)
	if raw < 1000 || math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) || value < 0 || value > 1000 {
		return float32(raw)
	}
	return value
}

func parseItemAttributes(detail *ItemDetail) {
	for _, entry := range strings.Split(detail.Attributes, ";") {
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, ":", 3)
		if len(parts) < 3 {
			continue
		}
		key := parts[0]
		value := parts[2]
		switch {
		case key == "OWNER" && strings.HasPrefix(parts[1], "s"):
			detail.OwnerName = value
		case key == "MKNAME" && strings.HasPrefix(parts[1], "s"):
			detail.MakerName = value
		case key == "QUAL":
			if parsed, err := strconv.Atoi(value); err == nil {
				detail.Quality = &parsed
			}
		case key == "IMRC":
			if parsed, err := strconv.Atoi(value); err == nil {
				detail.RepairCount = &parsed
			}
		case key == "IMRBV":
			if parsed, err := strconv.ParseFloat(value, 32); err == nil {
				converted := float32(parsed)
				detail.RepairBaseValue = &converted
			}
		case key == "MTWR":
			if parsed, err := strconv.Atoi(value); err == nil {
				detail.MaxWear = &parsed
			}
		case key == "MDEF":
			if parsed, err := strconv.ParseFloat(value, 32); err == nil {
				converted := float32(parsed)
				detail.MagicDefense = &converted
			}
			detail.StatEntries = append(detail.StatEntries, entry)
		case key == "MPROT":
			if parsed, err := strconv.ParseFloat(value, 32); err == nil {
				converted := float32(parsed)
				detail.MagicProtection = &converted
			}
			detail.StatEntries = append(detail.StatEntries, entry)
		case key == "ATTMIN":
			if parsed, err := strconv.Atoi(value); err == nil {
				detail.AttackMin = &parsed
			}
			detail.StatEntries = append(detail.StatEntries, entry)
		case key == "ATTMAX":
			detail.StatEntries = append(detail.StatEntries, entry)
		case strings.HasPrefix(key, "UPR"):
			detail.UpgradeEntries = append(detail.UpgradeEntries, entry)
			detail.EgoEntries = append(detail.EgoEntries, formatEgoEntry(key, value))
		case strings.HasPrefix(key, "IMSIUV"):
			detail.SpecialEntries = append(detail.SpecialEntries, entry)
			detail.ReforgeEntries = append(detail.ReforgeEntries, formatReforgeEntry(key, value))
		case strings.HasPrefix(key, "SI_"):
			detail.SpecialEntries = append(detail.SpecialEntries, entry)
			detail.ReforgeEntries = append(detail.ReforgeEntries, formatSpecialStatEntry(key, value))
		case strings.HasPrefix(key, "OIC") || key == "SICID" || strings.HasPrefix(key, "EHLV") || key == "EHTY":
			detail.SpecialEntries = append(detail.SpecialEntries, entry)
		}
	}
}

func buildReadableEntries(detail *ItemDetail) {
	for _, opt := range detail.Options {
		line := formatOptionEntry(opt)
		if line == "" {
			continue
		}
		detail.EnchantEntries = append(detail.EnchantEntries, line)
	}
}

func formatOptionEntry(opt ItemOption) string {
	kind := optionApplyLabel(opt.ApplyType)
	parts := []string{kind}
	if opt.OptionID >= 1000 {
		parts = append(parts, fmt.Sprintf("选项ID %d", opt.OptionID))
	}
	if opt.StatID != 0 || opt.StatValue != 0 {
		parts = append(parts, fmt.Sprintf("%s %+d", optionStatLabel(opt.StatID), opt.StatValue))
	}
	if len(parts) == 1 && opt.OptionID == 0 && opt.StatID == 0 && opt.StatValue == 0 {
		return ""
	}
	return strings.Join(parts, " · ")
}

func optionApplyLabel(applyType uint32) string {
	switch applyType {
	case 0:
		return "接尾效果"
	case 1:
		return "接头效果"
	case 7:
		return "释放效果"
	case 10:
		return "特殊效果"
	default:
		return fmt.Sprintf("选项类型%d", applyType)
	}
}

func optionStatLabel(statID int16) string {
	switch statID {
	case 13:
		return "力量"
	case 14:
		return "智力"
	case 15:
		return "灵巧"
	case 16:
		return "意志"
	case 17:
		return "幸运"
	case 9, 21:
		return "最大伤害"
	case 10, 22:
		return "最小伤害"
	case 11, 23:
		return "暴击"
	case 1:
		return "防御"
	case 2:
		return "保护"
	case 25:
		return "最大生命"
	case 26:
		return "最大魔力"
	case 27:
		return "最大耐力"
	case 52:
		return "修理费"
	case 153:
		return "修理倍率"
	case 155:
		return "释放等级相关"
	case 157:
		return "释放附加"
	case 178:
		return "伤害相关"
	default:
		return fmt.Sprintf("属性#%d", statID)
	}
}

func formatEgoEntry(key, value string) string {
	fields := strings.Split(value, ",")
	slot := strings.TrimPrefix(key, "UPR")
	if len(fields) == 0 {
		return key + ":" + value
	}
	itemID := fields[0]
	extra := ""
	if len(fields) > 1 {
		extra = " · " + strings.Join(fields[1:], ",")
	}
	return fmt.Sprintf("觉醒槽%s · 材料ID %s%s", slot, itemID, extra)
}

func formatReforgeEntry(key, value string) string {
	skillID := strings.TrimPrefix(key, "IMSIUV")
	return fmt.Sprintf("改造 · 技能ID %s · 等级 %s", skillID, value)
}

func formatSpecialStatEntry(key, value string) string {
	labels := map[string]string{
		"SI_SE":  "套装效果",
		"SI_ASE": "追加套装效果",
		"SI_PR2": "套装进度",
	}
	if label, ok := labels[key]; ok {
		return fmt.Sprintf("%s · %s", label, value)
	}
	return fmt.Sprintf("%s · %s", key, value)
}
