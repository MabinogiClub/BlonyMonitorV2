package packet

import (
	"fmt"
	"strconv"
	"strings"
)

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

type CharacterConditionPacket struct {
	Id       uint64
	IsEnable bool
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
	var duration int64 = 0
	if len(p.Msg) > 3 && p.Msg[3].Type() == MessageElemTypeString {
		detailStr := p.Msg[3].Data().(string)
		if mcagt, err := extractMCAGT(detailStr); err == nil {
			// 计算duration = (SBT - MCAGT) / 1000（秒）
			duration = (disableAt - mcagt) / 1000
		}
	}

	v := &CharacterConditionPacket{
		Id:       p.Id,
		IsEnable: true,
		EntityCharacterCondition: EntityCharacterCondition{
			CCId:       ccId,
			DisableAt:  disableAt,
			AttackerId: attackerId,
			Duration:   duration,
		},
	}

	return v, nil
}
