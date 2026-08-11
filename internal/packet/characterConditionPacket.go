package packet

import (
	"fmt"
	"strconv"
	"strings"
)

type ConditionDetailValue struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

func ParseConditionDetails(raw string) map[string]ConditionDetailValue {
	details := make(map[string]ConditionDetailValue)
	for _, part := range strings.Split(raw, ";") {
		if part == "" {
			continue
		}
		fields := strings.SplitN(part, ":", 3)
		if len(fields) != 3 || fields[0] == "" {
			continue
		}
		details[fields[0]] = ConditionDetailValue{Type: fields[1], Value: fields[2]}
	}
	return details
}

// extractMCAGT 从packet的[3]字段string中提取MCAGT值
// 格式示例: "...MCAGT:8:63905989472526;..."
func extractMCAGT(s string) (int64, error) {
	// 按分号分割
	parts := strings.Split(s, ";")
	for _, part := range parts {
		// 查找包含MCAGT的部分
		if strings.HasPrefix(part, "MCAGT:") {
			// 按冒号分割，格式是 MCAGT:8:63905989472526
			subParts := strings.Split(part, ":")
			if len(subParts) >= 3 {
				// 取最后一个部分（数值）
				value, err := strconv.ParseInt(subParts[2], 10, 64)
				if err != nil {
					return 0, fmt.Errorf("failed to parse MCAGT value: %v", err)
				}
				return value, nil
			}
		}
	}
	return 0, fmt.Errorf("MCAGT not found in string")
}

func extractDurationSeconds(details map[string]ConditionDetailValue) int64 {
	detail, ok := details["DURA"]
	if !ok {
		return 0
	}
	durationMillis, err := strconv.ParseInt(detail.Value, 10, 64)
	if err != nil || durationMillis <= 0 {
		return 0
	}
	return (durationMillis + 999) / 1000
}

type CharacterConditionPacket struct {
	Id        uint64
	IsEnable  bool
	DetailRaw string
	Details   map[string]ConditionDetailValue
	EntityCharacterCondition
}

func ParseCharacterConditionPacket(p *GamePacket) (*CharacterConditionPacket, error) {
	if len(p.Msg) < 2 {
		return nil, fmt.Errorf("ParseCharacterConditionPacket: packet too short")
	}
	if p.Msg[0].Type() != MessageElemTypeByte {
		return nil, fmt.Errorf("ParseCharacterConditionPacket: isEnable has unexpected type %v", p.Msg[0].Type())
	}
	if p.Msg[1].Type() != MessageElemTypeInt {
		return nil, fmt.Errorf("ParseCharacterConditionPacket: ccId has unexpected type %v", p.Msg[1].Type())
	}

	isEnable := p.Msg[0].Data().(uint8) != 0
	ccId := p.Msg[1].Data().(uint32)

	if !isEnable {
		v := &CharacterConditionPacket{
			Id:       p.Id,
			IsEnable: false,
			EntityCharacterCondition: EntityCharacterCondition{
				CCId: ccId,
			},
		}

		return v, nil
	}

	if len(p.Msg) < 5 {
		return nil, fmt.Errorf("ParseCharacterConditionPacket: packet too short2")
	}

	if p.Msg[2].Type() != MessageElemTypeLong {
		return nil, fmt.Errorf("ParseCharacterConditionPacket: disableAt has unexpected type %v", p.Msg[2].Type())
	}
	if p.Msg[4].Type() != MessageElemTypeLong {
		return nil, fmt.Errorf("ParseCharacterConditionPacket: attackerId has unexpected type %v", p.Msg[4].Type())
	}

	disableAtRaw := p.Msg[2].Data().(uint64)
	attackerId := p.Msg[4].Data().(uint64)

	// 直接保存 Mabinogi 毫秒时间戳，不转换为 Unix 时间戳
	disableAt := int64(disableAtRaw)

	// 尝试从[3]字段提取MCAGT并计算duration
	var duration int64
	var detailRaw string
	var details map[string]ConditionDetailValue
	if len(p.Msg) > 3 && p.Msg[3].Type() == MessageElemTypeString {
		detailRaw = p.Msg[3].Data().(string)
		details = ParseConditionDetails(detailRaw)
		if mcagt, err := extractMCAGT(detailRaw); err == nil {
			// 计算duration = (SBT - MCAGT) / 1000（秒）
			duration = (disableAt - mcagt) / 1000
		}
		if duration <= 0 {
			duration = extractDurationSeconds(details)
		}
	}

	v := &CharacterConditionPacket{
		Id:        p.Id,
		IsEnable:  true,
		DetailRaw: detailRaw,
		Details:   details,
		EntityCharacterCondition: EntityCharacterCondition{
			CCId:       ccId,
			DisableAt:  disableAt,
			AttackerId: attackerId,
			Duration:   duration,
		},
	}

	return v, nil
}
