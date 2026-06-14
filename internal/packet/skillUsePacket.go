package packet

import "fmt"

type SkillUsePacket struct {
	EntityID uint64
	SkillID  uint16
}

func ParseSkillUsePacket(pkt *GamePacket) (*SkillUsePacket, error) {
	if len(pkt.Msg) < 1 {
		return nil, fmt.Errorf("invalid skill use packet")
	}

	if pkt.Msg[0].Type() != MessageElemTypeShort {
		return nil, fmt.Errorf("invalid skill id type")
	}

	skillID := pkt.Msg[0].Data().(uint16)

	return &SkillUsePacket{
		EntityID: pkt.Id,
		SkillID:  skillID,
	}, nil
}
