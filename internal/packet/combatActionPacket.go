package packet

import (
	"bytes"
	"fmt"
)

type CombatActionPackPacket struct {
	Id                 uint64 // packet target id
	CombatActionId     uint32
	PrevCombatActionId uint32
	Hit                bool
	Type               uint8
	Unk1               uint8
	Flag               uint8

	SubPackets []*CombatActionPacket
}

type CombatActionPacket struct {
	Id             uint64 // packet target id
	CombatActionId uint32
	EntityId       uint64
	Type           CombatActionType
	Stun           uint16
	SkillId        uint16
	SubSkillId     uint16 // ?
	Unk1           uint16 // ?
	Attacker       *CombatActionPacketAttackerInfo
	Hit            *CombatActionPacketHitInfo
}

type CombatActionPacketAttackerInfo struct {
	TargetId            uint64
	Options             CombatActionAttackerOptions
	UsedWeaponSet       uint8
	WeaponParameterType uint8
	Unk1                uint32
	PosX                uint32
	PosY                uint32
}

type CombatActionPacketHitInfo struct {
	Options    CombatActionHitOptions
	Damage     float32
	Wound      float32
	ManaDamage uint32
}

type CombatActionType uint8

const (
	CombatActionTypeNone     = 0x00
	CombatActionTypeTakeHit  = 0x01 // 被击中
	CombatActionTypeAttacker = 0x02 // 攻击或防御或反击
	// CombatActionTypeUnknown = 0x04
	CombatActionTypeSkillActive          = 0x10
	CombatActionTypeSkillSuccess         = 0x20
	CombatActionTypeSkillPlayerCharacter = 0x40
)

type CombatActionAttackerOptions uint32

const (
	CombatActionAttackerOptionsUseEffect = 0x400
)

type CombatActionHitOptions uint32

const (
	CombatActionHitOptionsCritical = 0x01
	CombatActionHitOptionsMultiHit = 0x200_0000
)

func ParseCombatActionPackPacket(p *GamePacket) (*CombatActionPackPacket, error) {
	msg := p.Msg

	if msg[0].Type() != MessageElemTypeInt {
		return nil, fmt.Errorf("ParseCombatActionPacket: id has unexpected type %v", msg[0].Type())
	}
	if msg[1].Type() != MessageElemTypeInt {
		return nil, fmt.Errorf("ParseCombatActionPacket: prevId has unexpected type %v", msg[1].Type())
	}
	if msg[2].Type() != MessageElemTypeByte {
		return nil, fmt.Errorf("ParseCombatActionPacket: hit has unexpected type %v", msg[2].Type())
	}
	if msg[3].Type() != MessageElemTypeByte {
		return nil, fmt.Errorf("ParseCombatActionPacket: ttype has unexpected type %v", msg[3].Type())
	}
	if msg[4].Type() != MessageElemTypeByte {
		return nil, fmt.Errorf("ParseCombatActionPacket: unk1 has unexpected type %v", msg[4].Type())
	}
	if msg[5].Type() != MessageElemTypeByte {
		return nil, fmt.Errorf("ParseCombatActionPacket: flag has unexpected type %v", msg[5].Type())
	}

	actionPackId := msg[0].Data().(uint32)
	actionPackPrevId := msg[1].Data().(uint32)
	hit := msg[2].Data().(uint8) != 0
	ttype := msg[3].Data().(uint8)
	unk1 := msg[4].Data().(uint8)
	flag := msg[5].Data().(uint8)

	msg = msg[6:]

	// 攻击被格挡时？
	if (flag & 0x1) != 0 {
		if msg[0].Type() != MessageElemTypeInt {
			return nil, fmt.Errorf("ParseCombatActionPacket: blockedByShieldPosX has unexpected type %v", msg[0].Type())
		}
		if msg[1].Type() != MessageElemTypeInt {
			return nil, fmt.Errorf("ParseCombatActionPacket: blockedByShieldPosY has unexpected type %v", msg[1].Type())
		}
		if msg[2].Type() != MessageElemTypeLong {
			return nil, fmt.Errorf("ParseCombatActionPacket: shieldCasterId has unexpected type %v", msg[2].Type())
		}

		msg = msg[3:]
	}

	if msg[0].Type() != MessageElemTypeInt {
		return nil, fmt.Errorf("ParseCombatActionPacket: subPacketCount has unexpected type %v", msg[6].Type())
	}

	subPacketCount := msg[0].Data().(uint32)
	msg = msg[1:]

	// logger.Printf("packet op %x id %x", p.Op, p.Id)
	// logger.Println("id", id, "prevId", prevId, "hit", hit, "ttype", ttype, "unk1", unk1, "flag", flag, "packetCount", subPacketCount)

	v := &CombatActionPackPacket{
		Id:                 p.Id,
		CombatActionId:     actionPackId,
		PrevCombatActionId: actionPackPrevId,
		Hit:                hit,
		Type:               ttype,
		Unk1:               unk1,
		Flag:               flag,
	}

	for i := 0; i < int(subPacketCount); i++ {
		if msg[1].Type() != MessageElemTypeBin {
			err := fmt.Errorf("ParseCombatActionPacket: subPacket has unexpected type %v", msg[1].Type())
			return nil, err
		}

		subPacketBuf := msg[1].Data().([]byte)
		msg = msg[2:]

		op, id, subMsg, err := GamePacketBodyReader(bytes.NewReader(subPacketBuf))
		if err != nil {
			logger.Println("GamePacketBodyReader failed:", err)
			continue
		}

		_, _ = op, id

		// logger.Printf("sub packet id %x op %x", id, op)
		// for i, msg := range subMsg {
		// 	logger.Println("* msg", i, msg.Type(), msg.String())
		// }

		subPacket, err := parseCombatActionPacket(id, subMsg)
		if err != nil {
			logger.Println("parseCombatActionPacket failed:", err)

			for j, msg := range subMsg {
				logger.Println("* msg", j, msg.Type(), msg.String())
			}

			continue
		}

		v.SubPackets = append(v.SubPackets, subPacket)
	}

	return v, nil
}

func parseCombatActionPacket(id uint64, msg Message) (*CombatActionPacket, error) {
	origMsg := msg

	if msg[0].Type() != MessageElemTypeInt {
		return nil, fmt.Errorf("parseCombatActionPacket: combatActionId has unexpected type %v", msg[1].Type())
	}
	if msg[1].Type() != MessageElemTypeLong {
		return nil, fmt.Errorf("parseCombatActionPacket: entityId has unexpected type %v", msg[2].Type())
	}
	if msg[2].Type() != MessageElemTypeByte {
		return nil, fmt.Errorf("parseCombatActionPacket: type has unexpected type %v", msg[3].Type())
	}
	if msg[3].Type() != MessageElemTypeShort {
		return nil, fmt.Errorf("parseCombatActionPacket: stun has unexpected type %v", msg[4].Type())
	}
	if msg[4].Type() != MessageElemTypeShort {
		return nil, fmt.Errorf("parseCombatActionPacket: skillId has unexpected type %v", msg[5].Type())
	}
	if msg[5].Type() != MessageElemTypeShort {
		return nil, fmt.Errorf("parseCombatActionPacket: subSkillId has unexpected type %v", msg[6].Type())
	}
	if msg[6].Type() != MessageElemTypeShort {
		return nil, fmt.Errorf("parseCombatActionPacket: unk1 has unexpected type %v", msg[7].Type())
	}

	combatActionId := msg[0].Data().(uint32)
	entityId := msg[1].Data().(uint64)
	ttype := CombatActionType(msg[2].Data().(uint8))
	stun := msg[3].Data().(uint16)
	skillId := msg[4].Data().(uint16)
	subSkillId := msg[5].Data().(uint16)
	unk1 := msg[6].Data().(uint16)

	v := &CombatActionPacket{
		Id:             id,
		CombatActionId: combatActionId,
		EntityId:       entityId,
		Type:           ttype,
		Stun:           stun,
		SkillId:        skillId,
		SubSkillId:     subSkillId,
		Unk1:           unk1,
	}

	msg = msg[7:]

	if (ttype & CombatActionTypeAttacker) != 0 {
		if len(msg) < 7 {
			return nil, fmt.Errorf("parseCombatActionPacket: attacker has too few elements")
		}
		if msg[0].Type() != MessageElemTypeLong {
			return nil, fmt.Errorf("parseCombatActionPacket: attacker targetId has unexpected type %v", msg[0].Type())
		}
		if msg[1].Type() != MessageElemTypeInt {
			return nil, fmt.Errorf("parseCombatActionPacket: attacker options has unexpected type %v", msg[1].Type())
		}
		if msg[2].Type() != MessageElemTypeByte {
			return nil, fmt.Errorf("parseCombatActionPacket: attacker usedWeaponSet has unexpected type %v", msg[2].Type())
		}
		if msg[3].Type() != MessageElemTypeByte {
			return nil, fmt.Errorf("parseCombatActionPacket: attacker weaponParameterType has unexpected type %v", msg[3].Type())
		}
		if msg[4].Type() != MessageElemTypeInt {
			return nil, fmt.Errorf("parseCombatActionPacket: attacker unk1 has unexpected type %v", msg[4].Type())
		}
		if msg[5].Type() != MessageElemTypeInt {
			return nil, fmt.Errorf("parseCombatActionPacket: attacker posX has unexpected type %v", msg[5].Type())
		}
		if msg[6].Type() != MessageElemTypeInt {
			return nil, fmt.Errorf("parseCombatActionPacket: attacker posY has unexpected type %v", msg[6].Type())
		}

		targetId := msg[0].Data().(uint64)
		options := CombatActionAttackerOptions(msg[1].Data().(uint32))
		usedWeaponSet := msg[2].Data().(uint8)
		weaponParameterType := msg[3].Data().(uint8)
		unk1 := msg[4].Data().(uint32)
		posX := msg[5].Data().(uint32)
		posY := msg[6].Data().(uint32)

		v.Attacker = &CombatActionPacketAttackerInfo{
			TargetId:            targetId,
			Options:             options,
			UsedWeaponSet:       usedWeaponSet,
			WeaponParameterType: weaponParameterType,
			Unk1:                unk1,
			PosX:                posX,
			PosY:                posY,
		}

		msg = msg[7:]

		if (options & CombatActionAttackerOptionsUseEffect) != 0 {
			if len(msg) >= 1 && msg[0].Type() == MessageElemTypeLong {
				// prop id?
				msg = msg[1:]

				_ = origMsg
				// for i, v := range origMsg {
				// 	logger.Println("* msg", i, v.Type(), v.String())
				// }
			}
		}
	}

	if (ttype & CombatActionTypeTakeHit) != 0 {
		if len(msg) < 4 {
			return nil, fmt.Errorf("parseCombatActionPacket: hit has too few elements")
		}
		if msg[0].Type() != MessageElemTypeInt {
			return nil, fmt.Errorf("parseCombatActionPacket: hit options has unexpected type %v", msg[0].Type())
		}
		if msg[1].Type() != MessageElemTypeFloat {
			return nil, fmt.Errorf("parseCombatActionPacket: hit damage has unexpected type %v", msg[1].Type())
		}
		if msg[2].Type() != MessageElemTypeFloat {
			return nil, fmt.Errorf("parseCombatActionPacket: hit wound has unexpected type %v", msg[2].Type())
		}
		if msg[3].Type() != MessageElemTypeInt {
			return nil, fmt.Errorf("parseCombatActionPacket: hit manaDamage has unexpected type %v", msg[3].Type())
		}

		options := CombatActionHitOptions(msg[0].Data().(uint32))
		damage := msg[1].Data().(float32)
		wound := msg[2].Data().(float32)
		manaDamage := msg[3].Data().(uint32)

		v.Hit = &CombatActionPacketHitInfo{
			Options:    options,
			Damage:     damage,
			Wound:      wound,
			ManaDamage: manaDamage,
		}

		msg = msg[4:]

		if len(msg) >= 2 {
			// unk1, unk2
			msg = msg[2:]
		}

		if len(msg) >= 2 {
			// x dist, y dist
			msg = msg[2:]
		}

		if (options&CombatActionHitOptionsMultiHit) != 0 && len(msg) >= 4 {
			// hit count, unk2, unk3, unk4
			msg = msg[4:]
		}

		if len(msg) >= 5 {
			// effect flags, delay, attacker id, unk3, attacker id
			msg = msg[5:]
		}

		_ = msg
	}

	/*
		if len(msg) > 0 {
			logger.Println("ttype ", ttype)
			if v.Attacker != nil {
				logger.Println("attacker options", v.Attacker.Options)
			}
			if v.Hit != nil {
				logger.Println("hit options", v.Hit.Options)
			}
			for i, msg := range msg {
				logger.Println("* msg", i, msg.Type(), msg.String())
			}
		}
	*/

	return v, nil
}
