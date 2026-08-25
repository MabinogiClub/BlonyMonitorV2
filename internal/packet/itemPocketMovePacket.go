package packet

// ItemPocketMove 是 0x59de / 0x59df 中的一次物品栏位移动。
type ItemPocketMove struct {
	InstanceID uint64
	FromPocket uint32
	ToPocket   uint32
}

// ParseItemPocketMovePacket 解析手动拖拽换装的移动包。
// 0x59de：单件 long + from + to + byte(2) ...
// 0x59df：两件以上，每件 long + from + to + byte(2) + byte + byte。
func ParseItemPocketMovePacket(msg Message) []ItemPocketMove {
	if len(msg) < 3 {
		return nil
	}
	moves := make([]ItemPocketMove, 0, 2)
	for i := 0; i+3 <= len(msg); {
		if msg[i].Type() != MessageElemTypeLong ||
			msg[i+1].Type() != MessageElemTypeInt ||
			msg[i+2].Type() != MessageElemTypeInt {
			i++
			continue
		}
		instanceID := msg[i].Data().(uint64)
		from := msg[i+1].Data().(uint32)
		to := msg[i+2].Data().(uint32)
		if instanceID == 0 {
			i++
			continue
		}
		// 典型结尾带 byte(2)；没有也不拒绝，避免漏掉精简变体。
		if i+3 < len(msg) && msg[i+3].Type() == MessageElemTypeByte && msg[i+3].Data().(uint8) != 2 {
			i++
			continue
		}
		moves = append(moves, ItemPocketMove{
			InstanceID: instanceID,
			FromPocket: from,
			ToPocket:   to,
		})
		if i+3 < len(msg) && msg[i+3].Type() == MessageElemTypeByte {
			i += 4
			// 跳过 byte(2) 后常见的两个附加 byte。
			for skipped := 0; skipped < 2 && i < len(msg) && msg[i].Type() == MessageElemTypeByte; skipped++ {
				i++
			}
			continue
		}
		i += 3
	}
	return moves
}
