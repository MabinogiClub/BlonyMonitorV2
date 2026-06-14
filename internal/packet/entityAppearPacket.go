package packet

import (
	"bytes"
	"fmt"

	"blonymonitorv2/internal/util"
)

type EntityInfo struct {
	Id                    uint64
	Name                  string
	RaceId                uint32
	SkinColor             uint8
	EyeType               uint16
	LeftEyeColor          uint8
	RightEyeColor         uint8
	MouthType             uint16
	Height                float32
	Weight                float32
	Upper                 float32
	Lower                 float32
	CombatPower           float32 // 战斗力
	TitleId               uint32
	SubTitleId            uint32
	StyleTitleId          uint32
	StyleSubTitleId       uint32
	EquipItemMap          map[uint32]*EntityItem
	CharacterConditionMap map[uint32]*EntityCharacterCondition
	GuildName             string
	OwnerId               uint64 // 宠物, 傀儡等
	// 生命值相关
	HP         float32 // 当前生命值
	MaxHP      float32 // 最大生命值
	MP         float32 // 当前魔法值
	MaxMP      float32 // 最大魔法值
	Stamina    float32 // 当前耐力值
	MaxStamina float32 // 最大耐力值
}

type EntityItem struct {
	// public data
	PocketType uint32
	ItemId     uint32
	Color1     uint32
	Color2     uint32
	Color3     uint32
	Color4     uint32
	Color5     uint32
	Color6     uint32
	Color7     uint32
	Amount     uint16
}

type EntityCharacterCondition struct {
	CCId       uint32
	DisableAt  int64
	AttackerId uint64
	Duration   int64 // buff持续时间（秒），从packet中的(SBT-MCAGT)/1000计算得出
}

func ParseEntityAppearPacket(msg Message) (*EntityInfo, error) {
	origMsg := msg

	curPos := func() int {
		return len(origMsg) - len(msg)
	}

	if len(msg) < 2 || msg[1].Type() != MessageElemTypeByte {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return nil, err
	}

	if msg[1].Data().(uint8) != 5 {
		// 仅读取public数据
		return nil, nil
	}

	v := &EntityInfo{
		EquipItemMap:          make(map[uint32]*EntityItem),
		CharacterConditionMap: make(map[uint32]*EntityCharacterCondition),
	}

	if len(msg) < 40 {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		return nil, err
	}

	if msg[0].Type() != MessageElemTypeLong {
		err := fmt.Errorf("id has unexpected type %v", msg[0].Type())
		logger.Println(err)
		return nil, err
	}

	v.Id = msg[0].Data().(uint64)

	if msg[2].Type() != MessageElemTypeString {
		err := fmt.Errorf("name has unexpected type %v", msg[2].Type())
		logger.Println(err)
		return nil, err
	}

	v.Name = msg[2].Data().(string)

	if msg[5].Type() != MessageElemTypeInt {
		err := fmt.Errorf("raceId has unexpected type %v", msg[5].Type())
		logger.Println(err)
		return nil, err
	}

	v.RaceId = msg[5].Data().(uint32)

	if msg[6].Type() != MessageElemTypeByte {
		err := fmt.Errorf("skinColor has unexpected type %v", msg[6].Type())
		logger.Println(err)
		return nil, err
	}

	v.SkinColor = msg[6].Data().(uint8)

	if msg[7].Type() != MessageElemTypeShort {
		err := fmt.Errorf("eyeType has unexpected type %v", msg[7].Type())
		logger.Println(err)
		return nil, err
	}

	v.EyeType = msg[7].Data().(uint16)

	if msg[8].Type() != MessageElemTypeByte {
		err := fmt.Errorf("eyeColor has unexpected type %v", msg[8].Type())
		logger.Println(err)
		return nil, err
	}

	eyeColor := msg[8].Data().(uint8)

	if msg[9].Type() != MessageElemTypeShort {
		err := fmt.Errorf("mouthType has unexpected type %v", msg[9].Type())
		logger.Println(err)
		return nil, err
	}

	v.MouthType = msg[9].Data().(uint16)

	if msg[13].Type() != MessageElemTypeFloat {
		err := fmt.Errorf("height has unexpected type %v", msg[13].Type())
		logger.Println(err)
		return nil, err
	}

	v.Height = msg[13].Data().(float32)

	if msg[14].Type() != MessageElemTypeFloat {
		err := fmt.Errorf("weight has unexpected type %v", msg[14].Type())
		logger.Println(err)
		return nil, err
	}

	v.Weight = msg[14].Data().(float32)

	if msg[15].Type() != MessageElemTypeFloat {
		err := fmt.Errorf("upper has unexpected type %v", msg[15].Type())
		logger.Println(err)
		return nil, err
	}

	v.Upper = msg[15].Data().(float32)

	if msg[16].Type() != MessageElemTypeFloat {
		err := fmt.Errorf("lower has unexpected type %v", msg[16].Type())
		logger.Println(err)
		return nil, err
	}

	v.Lower = msg[16].Data().(float32)

	// msg[26] 是战斗力 (CombatPower)
	if msg[26].Type() == MessageElemTypeFloat {
		v.CombatPower = msg[26].Data().(float32)
	}

	if msg[28].Type() != MessageElemTypeByte {
		err := fmt.Errorf("leftEyeColor has unexpected type %v", msg[28].Type())
		logger.Println(err)
		return nil, err
	}

	v.LeftEyeColor = msg[28].Data().(uint8)

	if v.LeftEyeColor == 0 {
		v.LeftEyeColor = eyeColor
	}

	if msg[29].Type() != MessageElemTypeByte {
		err := fmt.Errorf("rightEyeColor has unexpected type %v", msg[29].Type())
		logger.Println(err)
		return nil, err
	}

	v.RightEyeColor = msg[29].Data().(uint8)

	if v.RightEyeColor == 0 {
		v.RightEyeColor = eyeColor
	}

	if msg[39].Type() != MessageElemTypeInt {
		err := fmt.Errorf("regenCount has unexpected type %v", msg[39].Type())
		logger.Println(err)
		return nil, err
	}

	regenCount := msg[39].Data().(uint32)
	msg = msg[40:]

	if len(msg) < 7*int(regenCount) {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return nil, err
	}

	// 解析regen数据（生命值、魔法值、耐力值等）
	// logger.Printf("[EntityAppear] Entity: %s (ID: %d), regenCount: %d\n", v.Name, v.Id, regenCount)
	for i := 0; i < int(regenCount); i++ {
		if len(msg) < 7 {
			break
		}

		// 每个regen数据占7个元素
		// msg[0]: 属性类型 (byte) - 0=HP, 1=MP, 2=Stamina
		// msg[1]: 当前值 (float)
		// msg[2]: 最大值 (float)
		// msg[3-6]: 其他属性

		if msg[0].Type() == MessageElemTypeByte {
			attrType := msg[0].Data().(uint8)

			// 解析当前值和最大值
			if msg[1].Type() == MessageElemTypeFloat && msg[2].Type() == MessageElemTypeFloat {
				currentValue := msg[1].Data().(float32)
				maxValue := msg[2].Data().(float32)

				switch attrType {
				case 0: // HP
					v.HP = currentValue
					v.MaxHP = maxValue
					logger.Printf("[EntityAppear] Entity: %s, HP: %.0f/%.0f\n", v.Name, currentValue, maxValue)
				case 1: // MP
					v.MP = currentValue
					v.MaxMP = maxValue
					logger.Printf("[EntityAppear] Entity: %s, MP: %.0f/%.0f\n", v.Name, currentValue, maxValue)
				case 2: // Stamina
					v.Stamina = currentValue
					v.MaxStamina = maxValue
					logger.Printf("[EntityAppear] Entity: %s, Stamina: %.0f/%.0f\n", v.Name, currentValue, maxValue)
				}
			}
		}

		msg = msg[7:]
	}

	if len(msg) < 1 {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return nil, err
	}

	if msg[0].Type() != MessageElemTypeInt {
		err := fmt.Errorf("regen2Count has unexpected type %v", msg[0].Type())
		logger.Println(err)
		return nil, err
	}

	regen2Count := msg[0].Data().(uint32)
	msg = msg[1:]

	if len(msg) < 7*int(regen2Count) {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return nil, err
	}

	// 解析regen2数据（可能是额外的属性或备用值）
	// logger.Printf("[EntityAppear] Entity: %s, regen2Count: %d\n", v.Name, regen2Count)
	for i := 0; i < int(regen2Count); i++ {
		if len(msg) < 7 {
			break
		}

		// 如果regen2也包含HP/MP/Stamina数据，同样解析
		if msg[0].Type() == MessageElemTypeByte {
			attrType := msg[0].Data().(uint8)

			if msg[1].Type() == MessageElemTypeFloat && msg[2].Type() == MessageElemTypeFloat {
				currentValue := msg[1].Data().(float32)
				maxValue := msg[2].Data().(float32)

				switch attrType {
				case 0: // HP
					if v.MaxHP == 0 { // 如果第一个regen没有设置，使用这个
						v.HP = currentValue
						v.MaxHP = maxValue
						logger.Printf("[EntityAppear] Entity: %s, HP (regen2): %.0f/%.0f\n", v.Name, currentValue, maxValue)
					}
				case 1: // MP
					if v.MaxMP == 0 {
						v.MP = currentValue
						v.MaxMP = maxValue
						logger.Printf("[EntityAppear] Entity: %s, MP (regen2): %.0f/%.0f\n", v.Name, currentValue, maxValue)
					}
				case 2: // Stamina
					if v.MaxStamina == 0 {
						v.Stamina = currentValue
						v.MaxStamina = maxValue
						logger.Printf("[EntityAppear] Entity: %s, Stamina (regen2): %.0f/%.0f\n", v.Name, currentValue, maxValue)
					}
				}
			}
		}

		msg = msg[7:]
	}

	if len(msg) < 10 {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return nil, err
	}

	if msg[0].Type() != MessageElemTypeInt {
		err := fmt.Errorf("titleId has unexpected type %v", msg[0].Type())
		logger.Println(err)
		return nil, err
	}

	v.TitleId = msg[0].Data().(uint32)

	if msg[2].Type() != MessageElemTypeInt {
		err := fmt.Errorf("subTitleId has unexpected type %v", msg[2].Type())
		logger.Println(err)
		return nil, err
	}

	v.SubTitleId = msg[2].Data().(uint32)

	if msg[3].Type() != MessageElemTypeInt {
		err := fmt.Errorf("styleTitleId has unexpected type %v", msg[3].Type())
		logger.Println(err)
		return nil, err
	}

	v.StyleTitleId = msg[3].Data().(uint32)

	if msg[4].Type() != MessageElemTypeInt {
		err := fmt.Errorf("styleSubTitleId has unexpected type %v", msg[4].Type())
		logger.Println(err)
		return nil, err
	}

	v.StyleSubTitleId = msg[4].Data().(uint32)

	unk1Count := msg[9].Data().(uint32)
	msg = msg[10:]

	if len(msg) < 2*int(unk1Count) {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return nil, err
	}

	msg = msg[2*unk1Count:]

	if len(msg) < 1 {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return nil, err
	}

	if msg[0].Type() != MessageElemTypeInt {
		err := fmt.Errorf("equipItemCount has unexpected type %v", msg[0].Type())
		logger.Println(err)
		return nil, err
	}

	equipItemCount := int(msg[0].Data().(uint32))
	if len(msg) < 2*equipItemCount {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return nil, err
	}

	msg = msg[1:]

	for i := 0; i < equipItemCount; i, msg = i+1, msg[2:] {
		if msg[1].Type() != MessageElemTypeBin {
			err := fmt.Errorf("equipItemData has unexpected type %v", msg[1].Type())
			logger.Println(err)
			return nil, err
		}

		b := msg[1].Data().([]byte)
		d, err := EntityItemReader(b)
		if err != nil {
			logger.Println("EntityItemReader failed:", err, i)
			return nil, err
		}

		v.EquipItemMap[d.PocketType] = d

		if msg[2].Type() == MessageElemTypeString {
			// 公会长袍
			msg = msg[1:]
		}
	}

	// 技能相关
	if len(msg) < 4 {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return nil, err
	}

	if msg[3].Type() != MessageElemTypeInt {
		err := fmt.Errorf("skillCount has unexpected type %v", msg[3].Type())
		logger.Println(err)
		return nil, err
	}

	skillCount := int(msg[3].Data().(uint32))
	msg = msg[4:]

	if len(msg) < skillCount {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return nil, err
	}

	msg = msg[skillCount:]

	// unknown field
	if len(msg) < 2 {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return nil, err
	}

	msg = msg[2:]

	// 队伍相关
	if len(msg) < 2 {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return nil, err
	}

	msg = msg[2:]

	// PVP相关
	if len(msg) < 16 {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return nil, err
	}

	msg = msg[16:]

	// 状态相关
	if len(msg) < 3 {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return nil, err
	}

	if msg[2].Type() != MessageElemTypeInt {
		err := fmt.Errorf("conditionCount has unexpected type %v", msg[2].Type())
		logger.Println(err)
		return nil, err
	}

	conditionCount := int(msg[2].Data().(uint32))
	msg = msg[3:]

	if len(msg) < (conditionCount * 6) {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return nil, err
	}

	for i := 0; i < conditionCount; i, msg = i+1, msg[6:] {
		/*
			uint32 ccId
			uint64 disableAt
			// string metadata 以后可能需要
			uint64 attackerId
			string unknown1
			// string 解除时消息？
		*/

		if msg[0].Type() != MessageElemTypeInt {
			err := fmt.Errorf("ccId has unexpected type %v", msg[0].Type())
			logger.Println(err)
			return nil, err
		}

		ccId := msg[0].Data().(uint32)

		if msg[1].Type() != MessageElemTypeLong {
			err := fmt.Errorf("disableAt has unexpected type %v", msg[1].Type())
			logger.Println(err)
			return nil, err
		}

		disableAtRaw := msg[1].Data().(uint64)
		disableAt := util.ParseMabiTime(disableAtRaw).Unix()

		if msg[3].Type() != MessageElemTypeLong {
			err := fmt.Errorf("attackerId has unexpected type %v", msg[3].Type())
			logger.Println(err)
			return nil, err
		}

		attackerId := msg[3].Data().(uint64)

		v.CharacterConditionMap[ccId] = &EntityCharacterCondition{
			CCId:       ccId,
			DisableAt:  disableAt,
			AttackerId: attackerId,
		}
	}

	// unknown field
	if len(msg) < 1 {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return nil, err
	}

	msg = msg[1:]

	// 公会相关
	if len(msg) < 19 {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return nil, err
	}

	if msg[1].Type() != MessageElemTypeString {
		err := fmt.Errorf("guildName has unexpected type %v", msg[1].Type())
		logger.Println(err)
		return nil, err
	}

	v.GuildName = msg[1].Data().(string)
	msg = msg[19:]

	// 宠物相关
	if len(msg) < 2 {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return nil, err
	}

	if msg[1].Type() != MessageElemTypeLong {
		err := fmt.Errorf("ownerId has unexpected type %v", msg[1].Type())
		logger.Println(err)
		return nil, err
	}

	v.OwnerId = msg[1].Data().(uint64)
	msg = msg[2:]

	// logger.Printf("[EntityAppear] Entity: %s (ID: %d) 解析完成 - HP: %.0f/%.0f, MP: %.0f/%.0f, Stamina: %.0f/%.0f\n", v.Name, v.Id, v.HP, v.MaxHP, v.MP, v.MaxMP, v.Stamina, v.MaxStamina)
	return v, nil
}

func ParseEntitiesAppearPacket(p *GamePacket) ([]*EntityInfo, error) {
	entities := []*EntityInfo(nil)
	msg := p.Msg
	if len(msg) < 1 || msg[0].Type() != MessageElemTypeShort {
		return nil, fmt.Errorf("invalid packet")
	}

	count := int(msg[0].Data().(uint16))
	msg = msg[1:]

	for i := 0; i < count; i++ {
		if len(msg) < 3 {
			break
		}

		if msg[0].Type() != MessageElemTypeShort ||
			msg[1].Type() != MessageElemTypeInt ||
			msg[2].Type() != MessageElemTypeBin {

			logger.Println("invalid packet", i)
			continue
		}

		t, b := msg[0].Data().(uint16), msg[2].Data().([]byte)
		if t != 16 {
			// 不是角色
			// logger.Println("invalid packet", i, t)
			msg = msg[3:]
			continue
		}

		msg = msg[3:]

		_, _, subMsg, err := GamePacketBodyReader(bytes.NewReader(b))
		if err != nil {
			logger.Println("GamePacketBodyReader failed:", err)
			continue
		}

		v, err := ParseEntityAppearPacket(subMsg)
		if err != nil {
			logger.Println("ParseEntityAppearPacket failed:", err)
			continue
		}

		if v != nil {
			entities = append(entities, v)
		}
	}

	return entities, nil
}

func EntityItemReader(b []byte) (*EntityItem, error) {
	r := new(EntityItem)
	if len(b) < 38 {
		err := fmt.Errorf("item public info data is too short %v", len(b))
		return nil, err
	}

	r.PocketType = le.Uint32(b[0:]) // 可能是uint8类型
	r.ItemId = le.Uint32(b[4:])
	r.Color1 = le.Uint32(b[8:])
	r.Color2 = le.Uint32(b[12:])
	r.Color3 = le.Uint32(b[16:])
	r.Color4 = le.Uint32(b[20:])
	r.Color5 = le.Uint32(b[24:])
	r.Color6 = le.Uint32(b[28:])
	r.Color7 = le.Uint32(b[32:])
	r.Amount = le.Uint16(b[36:])
	if r.Amount == 0 {
		r.Amount = 1
	}

	return r, nil
}
