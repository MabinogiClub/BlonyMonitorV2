package packet

import "fmt"

type CharacterDataPacket struct {
	Id         uint64
	Name       string
	RaceId     uint32
	Conditions []*CharacterConditionPacket
}

func ParseCharacterDataPacket(p *GamePacket) (*CharacterDataPacket, error) {
	if p == nil || len(p.Msg) < 4 {
		return nil, fmt.Errorf("ParseCharacterDataPacket: packet too short")
	}
	if p.Msg[1].Type() != MessageElemTypeLong || p.Msg[3].Type() != MessageElemTypeString {
		return nil, fmt.Errorf("ParseCharacterDataPacket: invalid character header")
	}

	result := &CharacterDataPacket{
		Id:         p.Msg[1].Data().(uint64),
		Name:       p.Msg[3].Data().(string),
		Conditions: make([]*CharacterConditionPacket, 0),
	}
	if len(p.Msg) > 6 &&
		p.Msg[4].Type() == MessageElemTypeString &&
		p.Msg[5].Type() == MessageElemTypeString &&
		p.Msg[6].Type() == MessageElemTypeInt {
		result.RaceId = p.Msg[6].Data().(uint32)
	}

	for index := 4; index < len(p.Msg); index++ {
		if p.Msg[index].Type() != MessageElemTypeInt {
			continue
		}
		count := int(p.Msg[index].Data().(uint32))
		if count <= 0 || count > 256 || index+1+count*6 > len(p.Msg) {
			continue
		}
		if !isCharacterConditionList(p.Msg[index+1:], count) {
			continue
		}

		for conditionIndex := 0; conditionIndex < count; conditionIndex++ {
			start := index + 1 + conditionIndex*6
			ccID := p.Msg[start].Data().(uint32)
			disableAt := int64(p.Msg[start+1].Data().(uint64))
			detailRaw := p.Msg[start+2].Data().(string)
			attackerID := p.Msg[start+3].Data().(uint64)
			result.Conditions = append(result.Conditions, &CharacterConditionPacket{
				Id:        result.Id,
				IsEnable:  true,
				DetailRaw: detailRaw,
				Details:   ParseConditionDetails(detailRaw),
				EntityCharacterCondition: EntityCharacterCondition{
					CCId:       ccID,
					DisableAt:  disableAt,
					AttackerId: attackerID,
					Duration:   remainingDurationSeconds(disableAt, p.At),
				},
			})
		}
		return result, nil
	}

	return result, nil
}

func isCharacterConditionList(message Message, count int) bool {
	for index := 0; index < count; index++ {
		start := index * 6
		if message[start].Type() != MessageElemTypeInt ||
			message[start+1].Type() != MessageElemTypeLong ||
			message[start+2].Type() != MessageElemTypeString ||
			message[start+3].Type() != MessageElemTypeLong ||
			message[start+4].Type() != MessageElemTypeString ||
			message[start+5].Type() != MessageElemTypeString {
			return false
		}
	}
	return true
}
