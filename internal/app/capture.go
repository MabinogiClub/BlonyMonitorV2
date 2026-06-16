package app

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"blonymonitorv2/db"
	"blonymonitorv2/internal/constants"
	"blonymonitorv2/internal/packet"
	"blonymonitorv2/internal/pcaputil"
)

// startCapture 自动检测模式启动抓包
func (a *App) startCapture() {
	if a.reportNpcapMissingIfNeeded() {
		return
	}

	a.setStatus("正在查找网卡...")
	logger.Println("查找网卡...")

	// 创建抓包专用的 context
	captureCtx, captureCancel := context.WithCancel(a.ctx)
	a.mu.Lock()
	a.captureCancel = captureCancel
	a.mu.Unlock()

	var nicName string
	var err error

	// 检查是否有手动选择的网卡
	a.mu.RLock()
	manualNic := a.manualNic
	a.mu.RUnlock()

	if manualNic != "" {
		// 使用手动选择的网卡
		nicName = manualNic
		logger.Printf("【手动选择】使用手动选择的网卡: %s\n", nicName)
	} else {
		// 自动查找网卡
		nicName, err = pcaputil.FindNic()
		if err != nil {
			a.setStatus("未找到游戏连接")
			logger.Println("FindNic failed:", err)

			// 2秒后重试（减少等待时间）
			select {
			case <-captureCtx.Done():
				return
			case <-time.After(2 * time.Second):
			}

			// 检查是否仍然是自动模式
			a.mu.RLock()
			stillAuto := a.autoDetect
			a.mu.RUnlock()

			if stillAuto {
				go a.startCapture()
			}
			return
		}
		logger.Println("【自动检测】找到网卡:", nicName)
	}

	// 记录使用的网卡信息
	logger.Printf("========================================\n")
	logger.Printf("正在使用网卡: %s\n", nicName)
	logger.Printf("加速器模式: %v\n", constants.AcceleratorMode)
	logger.Printf("选择的加速器: %s\n", constants.SelectedAccelerator)
	logger.Printf("当前过滤器: %s\n", constants.GetCurrentFilter())
	logger.Printf("========================================\n")

	a.setStatus("已连接")
	a.setConnected(true)

	r, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{
		Ctx:        captureCtx,
		NicName:    nicName,
		DisableLog: true, // 禁用 pcapng 日志
		OnServerInfo: func(ip string, port uint16) {
			channelName := constants.GetChannelName(ip, port)
			a.mu.Lock()
			a.channelName = channelName
			a.mu.Unlock()
			if channelName != "" {
				logger.Printf("【数据接收】检测到频道: %s (IP: %s, Port: %d)\n", channelName, ip, port)
				logger.Printf("【数据接收】当前使用网卡: %s\n", nicName)
				runtime.EventsEmit(a.ctx, "channel", channelName)
			}
		},
	})
	if err != nil {
		a.setStatus("读取数据包失败")
		logger.Println("NewGameServerPacketReader failed:", err)
		a.setConnected(false)

		select {
		case <-captureCtx.Done():
			return
		case <-time.After(2 * time.Second):
		}

		a.mu.RLock()
		stillAuto := a.autoDetect
		a.mu.RUnlock()

		if stillAuto {
			go a.startCapture()
		}
		return
	}

	// 超时检测：30秒内没收到任何数据包则重新查找网卡（处理频道切换）
	timeout := time.NewTimer(30 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case <-captureCtx.Done():
			r.Close()
			return
		case <-timeout.C:
			// 超时，可能频道切换了，重新查找网卡
			logger.Println("数据包超时，重新查找网卡...")
			a.setStatus("重新连接中...")
			a.setConnected(false)
			r.Close()

			a.mu.RLock()
			stillAuto := a.autoDetect
			a.mu.RUnlock()

			if stillAuto {
				go a.startCapture()
			}
			return
		case pkt := <-r.PacketCh():
			// 收到数据包，重置超时计时器
			timeout.Reset(30 * time.Second)
			a.processPacket(pkt)
		}
	}
}

// startCaptureForChannel 为指定频道启动抓包
func (a *App) startCaptureForChannel(channel int) {
	if a.reportNpcapMissingIfNeeded() {
		return
	}

	// 检查频道号是否有效
	if channel < 1 || channel > 20 {
		logger.Printf("无效的频道号: %d，切换到自动检测模式\n", channel)
		a.mu.Lock()
		a.autoDetect = true
		a.selectedChannel = 0
		a.mu.Unlock()
		go a.startCapture()
		return
	}

	// 获取显示友好的频道名称
	displayChannel := constants.GetDisplayChannelNumber(channel)
	a.setStatus("正在连接频道" + strconv.Itoa(displayChannel) + "...")
	logger.Printf("手动连接频道 %d...\n", channel)

	// 创建抓包专用的 context
	captureCtx, captureCancel := context.WithCancel(a.ctx)
	a.mu.Lock()
	a.captureCancel = captureCancel
	a.mu.Unlock()

	// 获取频道过滤器
	filter := constants.GetChannelFilter(channel)
	channelInfo := constants.ChannelMap[channel]

	var nicName string
	var err error

	// 检查是否有手动选择的网卡
	a.mu.RLock()
	manualNic := a.manualNic
	a.mu.RUnlock()

	if manualNic != "" {
		// 使用手动选择的网卡
		nicName = manualNic
		logger.Printf("【手动选择】使用手动选择的网卡: %s (频道 %d)\n", nicName, channel)
	} else {
		// 自动查找网卡
		nicName, err = pcaputil.FindNicForChannel(channel)
		if err != nil {
			// 获取频道的完整名称用于显示
			channelFullName := constants.GetChannelName(channelInfo.IP, channelInfo.Port)
			if channelFullName == "" {
				channelFullName = "频道" + strconv.Itoa(displayChannel)
			}
			a.setStatus("未找到" + channelFullName + "连接")
			logger.Printf("FindNicForChannel failed for channel %d: %v\n", channel, err)

			// 2秒后重试
			select {
			case <-captureCtx.Done():
				return
			case <-time.After(2 * time.Second):
			}

			// 检查是否仍然是手动模式且频道未变
			a.mu.RLock()
			stillManual := !a.autoDetect && a.selectedChannel == channel
			a.mu.RUnlock()

			if stillManual {
				go a.startCaptureForChannel(channel)
			}
			return
		}
	}

	// 记录使用的网卡信息
	logger.Printf("========================================\n")
	logger.Printf("正在使用网卡: %s (频道 %d)\n", nicName, channel)
	logger.Printf("========================================\n")

	a.setStatus("已连接")
	a.setConnected(true)

	// 设置频道名称
	a.mu.Lock()
	a.channelName = constants.GetChannelName(channelInfo.IP, channelInfo.Port)
	a.mu.Unlock()
	runtime.EventsEmit(a.ctx, "channel", a.channelName)

	r, err := packet.NewGameServerPacketReaderWithFilter(&packet.GameServerPacketReaderOpt{
		Ctx:        captureCtx,
		NicName:    nicName,
		DisableLog: true, // 禁用 pcapng 日志
		OnServerInfo: func(ip string, port uint16) {
			channelName := constants.GetChannelName(ip, port)
			a.mu.Lock()
			a.channelName = channelName
			a.mu.Unlock()
			if channelName != "" {
				logger.Printf("【数据接收】检测到频道: %s (IP: %s, Port: %d)\n", channelName, ip, port)
				logger.Printf("【数据接收】当前使用网卡: %s\n", nicName)
				runtime.EventsEmit(a.ctx, "channel", channelName)
			}
		},
	}, filter)
	if err != nil {
		a.setStatus("读取数据包失败")
		logger.Println("NewGameServerPacketReaderWithFilter failed:", err)
		a.setConnected(false)

		select {
		case <-captureCtx.Done():
			return
		case <-time.After(2 * time.Second):
		}

		a.mu.RLock()
		stillManual := !a.autoDetect && a.selectedChannel == channel
		a.mu.RUnlock()

		if stillManual {
			go a.startCaptureForChannel(channel)
		}
		return
	}

	// 超时检测
	timeout := time.NewTimer(30 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case <-captureCtx.Done():
			r.Close()
			return
		case <-timeout.C:
			logger.Println("数据包超时，重新连接频道", channel)
			a.setStatus("重新连接中...")
			a.setConnected(false)
			r.Close()

			a.mu.RLock()
			stillManual := !a.autoDetect && a.selectedChannel == channel
			a.mu.RUnlock()

			if stillManual {
				go a.startCaptureForChannel(channel)
			}
			return
		case pkt := <-r.PacketCh():
			timeout.Reset(30 * time.Second)
			a.processPacket(pkt)
		}
	}
}

// startCaptureWithMode 根据模式启动抓包
func (a *App) startCaptureWithMode() {
	a.mu.RLock()
	autoDetect := a.autoDetect
	selectedChannel := a.selectedChannel
	a.mu.RUnlock()

	if autoDetect {
		a.startCapture()
	} else {
		a.startCaptureForChannel(selectedChannel)
	}
}

// restartCapture 重启抓包
func (a *App) restartCapture() {
	// 取消当前抓包
	a.mu.Lock()
	if a.captureCancel != nil {
		a.captureCancel()
	}
	a.mu.Unlock()

	// 短暂延迟后重启
	time.Sleep(100 * time.Millisecond)

	// 启动新的抓包
	go a.startCaptureWithMode()
}

// processPacket 处理数据包
func (a *App) processPacket(pkt *packet.GamePacket) {
	if pkt == nil || pkt.Msg == nil {
		return
	}

	switch pkt.Op {
	case opcodeEntityAppear:
		entity, err := packet.ParseEntityAppearPacket(pkt.Msg)
		if err != nil {
			return
		}
		if entity != nil && len(entity.Name) > 0 && entity.Name[0] != '_' {
			a.addEntity(entity)
		}

	case opcodeEntitiesAppear:
		entities, err := packet.ParseEntitiesAppearPacket(pkt)
		if err != nil {
			return
		}
		for _, entity := range entities {
			if entity != nil && len(entity.Name) > 0 && entity.Name[0] != '_' {
				a.addEntity(entity)
			}
		}

	case opcodeEntityProperty:
		a.handleEntityProperty(pkt)

	case opcodeEntityRemove:
		a.clearBossHP(strconv.FormatUint(pkt.Id, 10))

	case opcodeCombatAction:
		pack, err := packet.ParseCombatActionPackPacket(pkt)
		if err != nil {
			return
		}

		attackerId := uint64(0)
		attackSkillId := uint16(0)

		// 找到攻击者
		for _, v := range pack.SubPackets {
			if v.Hit == nil {
				attackerId = v.EntityId
				attackSkillId = v.SkillId
				break
			}
		}

		// 处理伤害
		for _, v := range pack.SubPackets {
			if v.Hit == nil {
				continue
			}

			targetId := v.EntityId
			damage := v.Hit.Damage
			isCritical := v.Hit.Options&0x1 != 0

			a.addDamage(attackerId, targetId, attackSkillId, damage, isCritical)
		}

	case opcodeEffectDamage:
		if len(pkt.Msg) < 7 {
			return
		}
		if pkt.Msg[0].Type() != packet.MessageElemTypeInt ||
			pkt.Msg[2].Type() != packet.MessageElemTypeInt ||
			pkt.Msg[4].Type() != packet.MessageElemTypeLong ||
			pkt.Msg[5].Type() != packet.MessageElemTypeShort {
			return
		}

		effectType := pkt.Msg[0].Data().(uint32)
		if effectType != 353 {
			return
		}

		damage := pkt.Msg[2].Data().(uint32)
		attackerId := pkt.Msg[4].Data().(uint64)
		attackSkillId := pkt.Msg[5].Data().(uint16)
		targetId := pkt.Id

		a.addDamage(attackerId, targetId, attackSkillId, float32(damage), false)

	case opcodeEffectDelayed:
		if len(pkt.Msg) < 7 {
			return
		}
		if pkt.Msg[0].Type() != packet.MessageElemTypeInt ||
			pkt.Msg[1].Type() != packet.MessageElemTypeInt {
			return
		}

		ttype := pkt.Msg[1].Data().(uint32)
		if ttype != 318 {
			return
		}

		if pkt.Msg[2].Type() != packet.MessageElemTypeInt ||
			pkt.Msg[5].Type() != packet.MessageElemTypeLong ||
			pkt.Msg[6].Type() != packet.MessageElemTypeShort {
			return
		}

		damage := pkt.Msg[2].Data().(uint32)
		attackerId := pkt.Msg[5].Data().(uint64)
		attackSkillId := pkt.Msg[6].Data().(uint16)
		targetId := pkt.Id

		a.addDamage(attackerId, targetId, attackSkillId, float32(damage), false)

	case opcodeConditionUpdate:
		cond, err := packet.ParseCharacterConditionPacket(pkt)
		if err != nil {
			return
		}
		a.addConditionEvent(cond.Id, cond.CCId, cond.IsEnable, cond.AttackerId, cond.DisableAt, cond.Duration)

	case opcodeSetFinisher:
		if len(pkt.Msg) < 1 || pkt.Msg[0].Type() != packet.MessageElemTypeLong {
			return
		}
		attackerId := pkt.Msg[0].Data().(uint64)
		a.addFinishEvent(pkt.Id, attackerId)

	case opcodeDungeonInfo:
		dungeonInfo, err := ParseDungeonInfoPacket(pkt)
		if err != nil {
			logger.Printf("[Dungeon] 解析地下城信息失败: %v\n", err)
			return
		}
		a.onDungeonEnter(pkt, dungeonInfo)

	case opcodeChineseName:
		a.handleChineseName(pkt)

	case opcodeInstanceName:
		a.handleInstanceName(pkt)

	case opcodeMapChange:
		a.handleMapChange(pkt)

	default:
	}
}

// handleMapChange 处理地图切换数据包
func (a *App) handleMapChange(pkt *packet.GamePacket) {
	// 数据包格式:
	// [0] byte: 1
	// [1] int: 地图ID
	// [2] int: X坐标
	// [3] int: Y坐标
	if len(pkt.Msg) < 2 {
		return
	}

	// 获取地图ID（第二个元素，索引1）
	if pkt.Msg[1].Type() != packet.MessageElemTypeInt {
		return
	}

	mapID := int(pkt.Msg[1].Data().(uint32))
	targetEntityId := strconv.FormatUint(pkt.Id, 10)

	// 检查是否在地下城中，如果是则忽略地下城内部地图切换
	// 地下城内部地图ID通常是 10000-19999 范围
	a.mu.RLock()
	inDungeon := a.currentDungeon != nil
	inInstance := a.currentInstance != nil
	currentSelfId := a.selfId
	a.mu.RUnlock()

	// 如果已经识别了玩家自身，检查这个数据包是否是针对玩家自身的
	if currentSelfId != "" && targetEntityId != currentSelfId {
		// 忽略其他玩家的地图切换
		return
	}

	if inDungeon && mapID >= 10000 && mapID < 20000 {
		// 在地下城中，忽略内部地图切换，不更新标题栏
		return
	}

	if inInstance && isRandomInstanceMapID(mapID) {
		a.mu.RLock()
		existingMapID := a.currentInstance.MapID
		a.mu.RUnlock()
		if existingMapID != 0 && uint32(mapID) == existingMapID {
			logger.Printf("[Instance] 检测到副本内部切换 (%d)，忽略\n", mapID)
			return
		}
	}

	a.mu.RLock()
	oldMapName := ""
	if a.currentMap != nil {
		oldMapName = a.currentMap.LocalName
		if oldMapName == "" || oldMapName == "???" {
			oldMapName = a.currentMap.MapName
		}
	}
	a.mu.RUnlock()

	a.cleanupAndSaveTakenStats(mapID, oldMapName)

	now := time.Now().UnixMilli()

	// 如果切换到非地下城地图，清除地下城状态
	a.mu.Lock()
	if a.currentDungeon != nil && (mapID < 10000 || mapID >= 20000) {
		logger.Printf("[Map] 离开地下城: %s\n", a.currentDungeon.DungeonName)
		a.currentDungeon = nil
		a.dungeonSaveName = ""
		a.dungeonChineseNameReceived = false
	}

	if a.currentInstance != nil && !isRandomInstanceMapID(mapID) {
		logger.Printf("[Map] 离开副本: %s\n", a.currentInstance.InstanceName)
		a.currentInstance = nil
		a.instanceEnterMapID = 0
		a.instanceSaveName = ""
		a.instanceNameReceived = false
	}

	// 地图切换时清空旧实体，但保留最近添加的（玩家自身可能在地图切换前加载）
	// 保留地图切换前500ms内添加的实体（玩家自身）
	threshold := now - 500
	newEntities := make(map[string]*EntityInfo)

	for id, entity := range a.entities {
		if entity.AddedAt >= threshold {
			newEntities[id] = entity
		}
	}

	a.entities = newEntities
	a.lastMapChangeAt = now

	// 只有在尚未识别玩家自身时才尝试识别
	selfIdChanged := false
	if a.selfId == "" {
		// 通过数据包目标ID识别玩家自身
		if targetEntityId != "" && targetEntityId != "0" {
			if entity, ok := newEntities[targetEntityId]; ok && entity.IsPC {
				a.selfId = entity.ID
				a.selfName = entity.Name
				selfIdChanged = true
				logger.Printf("[Map] 识别玩家自身: %s (ID: %s)\n", entity.Name, entity.ID)
			}
		}

		// 如果还是没有识别到，使用第一个PC实体
		if a.selfId == "" {
			for _, entity := range newEntities {
				if entity.IsPC {
					a.selfId = entity.ID
					a.selfName = entity.Name
					selfIdChanged = true
					logger.Printf("[Map] 识别玩家自身(备选): %s (ID: %s)\n", entity.Name, entity.ID)
					break
				}
			}
		}
	}

	selfId := a.selfId
	selfName := a.selfName
	a.mu.Unlock()

	// 发送玩家自身信息事件（在锁外发送）
	if selfIdChanged && selfId != "" {
		if a.buffTimerMgr != nil {
			a.buffTimerMgr.SetSelfId(selfId)
		}
		runtime.EventsEmit(a.ctx, "selfInfo", &SelfInfo{ID: selfId, Name: selfName})
	}

	// 从数据库获取地图信息
	mapInfo := db.NewMinimapInfo_FieldMapInfoList(mapID)
	mapName := mapInfo.MapName
	localName := mapInfo.MapLocalName

	if mapName == "" {
		mapName = fmt.Sprintf("地图 #%d", mapID)
	}
	if localName == "" {
		localName = "未知区域"
	}

	if isRandomInstanceMapID(mapID) {
		a.mu.Lock()
		if a.currentInstance != nil && a.currentInstance.InstanceName != "" {
			mapName = a.currentInstance.InstanceName
			localName = "副本"
			a.currentInstance.MapID = uint32(mapID)
			if a.instanceEnterMapID == 0 {
				a.instanceEnterMapID = mapID
			}
			a.mu.Unlock()
		} else {
			a.mu.Unlock()
			time.Sleep(100 * time.Millisecond)
			a.mu.Lock()
			if a.currentInstance != nil && a.currentInstance.InstanceName != "" {
				mapName = a.currentInstance.InstanceName
				localName = "副本"
				if a.instanceEnterMapID == 0 {
					a.instanceEnterMapID = mapID
				}
			} else {
				mapName = "副本"
				localName = "副本"
			}
			a.mu.Unlock()
		}
	}

	currentMap := &CurrentMapInfo{
		MapID:     mapID,
		MapName:   mapName,
		LocalName: localName,
	}

	logger.Printf("[Map] 地图切换: %s - %s (ID: %d)\n", localName, mapName, mapID)
	a.setCurrentMap(currentMap)
}
