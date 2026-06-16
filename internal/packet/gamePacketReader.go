package packet

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcap"
	"github.com/gopacket/gopacket/pcapgo"
	"blonymonitorv2/internal/constants"
	"blonymonitorv2/internal/util"
)

type GameServerPacketReader struct {
	// non-mutable
	ctx      context.Context
	packetCh chan *GamePacket

	// mutable
	handle *pcap.Handle
	fd     *os.File

	logHandle *pcapgo.NgWriter
	logFd     *os.File

	// 服务器信息
	serverIP     string
	serverPort   uint16
	onServerInfo func(ip string, port uint16) // 服务器信息回调

	// 连接跟踪（用于加速器模式）
	gameConnections map[string]bool // 已识别的游戏连接：key = "srcIP:srcPort"
	connMutex       sync.RWMutex    // 保护 gameConnections 的互斥锁
}

type GameServerPacketReaderOpt struct {
	Ctx          context.Context
	FileName     string
	NicName      string
	ClientIp     string
	OnServerInfo func(ip string, port uint16) // 服务器信息回调
	DisableLog   bool                         // 禁用数据包日志记录
}

type gamePacketPayload struct {
	relSeq uint32
	data   []byte
	at     time.Time
}

type pendingTcpLayer struct {
	tcpLayer layers.TCP
	ci       gopacket.CaptureInfo
}

const (
	pcapQueueSize   = 1000 // 增加pcap队列大小，减少丢包
	pcapBufferSize  = 32 * 1024 * 1024
	pcapPromisc     = true
	packetQueueSize = 2000 // 增加数据包队列大小，应对高频战斗场景
)

var ErrTooShortPacket = errors.New("too short packet")

// isLikelyGamePacket 快速判断数据是否可能是游戏数据包
// 用于加速器模式下过滤掉大量无关的TCP流量
func isLikelyGamePacket(data []byte) bool {
	// 1. 检查最小长度（至少6字节协议头）
	if len(data) < 6 {
		return false
	}

	// 2. 检查长度字段（前4字节，小端序）
	length := binary.LittleEndian.Uint32(data[0:4])
	// 长度必须合理：至少6字节（包含头部），最大64KB
	if length < 6 || length > 65535 {
		return false
	}

	// 3. 检查opcode（后2字节，小端序）
	opcode := binary.LittleEndian.Uint16(data[4:6])
	// 洛奇协议的opcode通常在0x1000以上
	if opcode < 0x1000 {
		return false
	}

	return true
}

// 已知的游戏opcode（用于验证游戏连接）
var knownGameOpcodes = map[uint32]bool{
	0x520c: true, // 实体出现
	0x5334: true, // 多个实体出现
	0x7926: true, // 战斗动作（伤害）
	0x9095: true, // 效果延迟
	0xa028: true, // 状态更新
	0x7921: true, // 设置终结者
	0x9470: true, // 地下城信息
	0x6599: true, // 地图切换
	0x526d: true, // 中文名称包
	0x8ca0: true, // 副本名称包
	0x5209: true, // 背包数据
	0x6988: true, // 技能使用事件
}

// hasKnownGameOpcode 检查数据包是否包含已知的游戏opcode
func hasKnownGameOpcode(data []byte) bool {
	// 需要至少10字节：Sign(1) + Length(4) + Flag(1) + Op(4)
	if len(data) < 10 {
		return false
	}

	// 读取Op字段（offset 6-9，4字节，小端序）
	op := binary.LittleEndian.Uint32(data[6:10])

	return knownGameOpcodes[op]
}

// canParseAsGamePacket 尝试解析数据包，验证是否为有效的游戏包
// 这是一个严格的验证函数，只有能成功解析的包才会返回 true
// 用于统计过滤原因的全局计数器
var (
	filterStatsOnce sync.Once
	filterStats     = make(map[string]int)
	filterStatsMu   sync.Mutex
)

func canParseAsGamePacket(data []byte) bool {
	return canParseAsGamePacketWithLog(data, false)
}

func canParseAsGamePacketWithLog(data []byte, enableLog bool) bool {
	logFilter := func(reason string, details ...interface{}) {
		if enableLog {
			logger.Printf("【包验证失败】原因: "+reason+"\n", details...)
		}
		// 统计过滤原因
		filterStatsMu.Lock()
		filterStats[reason]++
		total := 0
		for _, count := range filterStats {
			total += count
		}
		// 每100个包输出一次统计
		if total%100 == 0 {
			logger.Printf("【过滤统计】总计: %d, 详情: %v\n", total, filterStats)
		}
		filterStatsMu.Unlock()
	}

	// 1. 基本长度检查
	if len(data) < 6 {
		logFilter("长度不足6字节", len(data))
		return false
	}

	// 2. 解析包头
	length := binary.LittleEndian.Uint32(data[1:5])
	flag := data[5]

	// 3. 验证包头字段
	if length == 0 || length > 0x100_0000 {
		logFilter("长度字段无效", length)
		return false
	}
	if flag > 4 {
		logFilter("flag字段无效", flag)
		return false
	}

	// 4. 检查数据长度是否足够
	if len(data) < int(length) {
		logFilter("数据长度不足", len(data), length)
		return false
	}

	// 5. 短包（心跳包等）直接返回 false，我们只关注完整的游戏包
	isShortPacket := flag == 1 || flag == 2
	if isShortPacket {
		logFilter("短包/心跳包", flag)
		return false
	}

	// 6. 完整包需要至少 headerSize(6) + 0xd 字节
	headerSize := 6
	if int(length) < headerSize+0xd {
		logFilter("包体长度不足", length)
		return false
	}

	// 7. 解析 opcode（大端序）
	body := data[headerSize:length]
	if len(body) < 4 {
		logFilter("body长度不足4字节", len(body))
		return false
	}
	op := binary.BigEndian.Uint32(body[0:4])

	// 8. 检查是否是已知的游戏 opcode
	if !knownGameOpcodes[op] {
		logFilter("未知opcode", fmt.Sprintf("0x%x", op))
		return false
	}

	// 9. 尝试解析消息体的基本结构
	// 跳过 op(4) + id(8) = 12 字节
	if len(body) < 12 {
		logFilter("body长度不足12字节", len(body))
		return false
	}

	msgBody := body[12:]

	// 10. 尝试读取消息元素数量（uvarint）
	elemCount, n := binary.Uvarint(msgBody)
	if n <= 0 || elemCount > 1000 { // 合理的元素数量上限
		logFilter("元素数量无效", n, elemCount)
		return false
	}

	// 11. 验证至少有一个字节的 unused field
	if len(msgBody) < n+1 {
		logFilter("msgBody长度不足", len(msgBody), n+1)
		return false
	}

	// 12. 如果是战斗动作包 (0x7926)，进行更严格的验证
	if op == 0x7926 {
		if !validateCombatActionPacket(msgBody[n+1:], elemCount) {
			logFilter("战斗动作包验证失败", op)
			return false
		}
	}

	// 其他已知 opcode 也通过
	if enableLog {
		logger.Printf("【包验证成功】opcode: 0x%x, 长度: %d\n", op, length)
	}
	return true
}

// validateCombatActionPacket 验证战斗动作包的基本结构
func validateCombatActionPacket(data []byte, elemCount uint64) bool {
	// 战斗动作包至少需要 6 个元素
	// msg[0]: Int (actionPackId)
	// msg[1]: Int (actionPackPrevId)
	// msg[2]: Byte (hit)
	// msg[3]: Byte (type)
	// msg[4]: Byte (unk1)
	// msg[5]: Byte (flag)
	if elemCount < 6 {
		return false
	}

	// 验证前 6 个元素的类型
	expectedTypes := []MessageElemType{
		MessageElemTypeInt,  // actionPackId
		MessageElemTypeInt,  // actionPackPrevId
		MessageElemTypeByte, // hit
		MessageElemTypeByte, // type
		MessageElemTypeByte, // unk1
		MessageElemTypeByte, // flag
	}

	offset := 0
	for i, expectedType := range expectedTypes {
		if offset >= len(data) {
			return false
		}

		// 读取元素类型
		elemType := MessageElemType(data[offset])
		if elemType != expectedType {
			return false
		}

		// 跳过元素数据
		offset++
		switch elemType {
		case MessageElemTypeByte:
			if offset+1 > len(data) {
				return false
			}
			offset += 1
		case MessageElemTypeShort:
			if offset+2 > len(data) {
				return false
			}
			offset += 2
		case MessageElemTypeInt:
			if offset+4 > len(data) {
				return false
			}
			offset += 4
		case MessageElemTypeLong:
			if offset+8 > len(data) {
				return false
			}
			offset += 8
		case MessageElemTypeFloat:
			if offset+4 > len(data) {
				return false
			}
			offset += 4
		default:
			return false
		}

		// 前 6 个元素验证通过即可
		if i == 5 {
			return true
		}
	}

	return true
}

func NewGameServerPacketReader(opt *GameServerPacketReaderOpt) (*GameServerPacketReader, error) {
	return NewGameServerPacketReaderWithFilter(opt, "")
}

func NewGameServerPacketReaderWithFilter(opt *GameServerPacketReaderOpt, customFilter string) (*GameServerPacketReader, error) {
	if opt == nil {
		return nil, errors.New("opt is nil")
	}

	filter := customFilter
	if filter == "" {
		filter = constants.GetCurrentFilter()
	}
	if opt.ClientIp != "" {
		// 无论如何，客户端向服务器发送的数据包都是加密的
		filter = " dst host " + opt.ClientIp
	}

	logger.Println("game packet filter...", filter)

	v := &GameServerPacketReader{
		ctx:             opt.Ctx,
		packetCh:        make(chan *GamePacket, packetQueueSize),
		onServerInfo:    opt.OnServerInfo,
		gameConnections: make(map[string]bool),
	}

	// 仅在未禁用日志时创建日志文件
	if !opt.DisableLog {
		if err := v.openLog(); err != nil {
			logger.Println("openLog failed", err)
			return nil, err
		}
	}

	payloadCh := (<-chan gamePacketPayload)(nil)
	err := error(nil)
	if opt.FileName != "" {
		payloadCh, err = v.openFile(opt.FileName, filter)
		if err != nil {
			logger.Println("openFile failed", err)
			return nil, err
		}
	} else {
		payloadCh, err = v.openNic(opt.NicName, filter)
		if err != nil {
			logger.Println("openNic failed", err)
			return nil, err
		}
	}

	go v.packetLoop(payloadCh)

	return v, nil
}

func (t *GameServerPacketReader) packetLoop(payloadCh <-chan gamePacketPayload) {
	buffer := bytes.NewBuffer(nil)
	lastRelSeq, lastAt := uint32(0), time.Now()
	payloads := make([]gamePacketPayload, 0, 100)

	skipPayload := func(n int) {
		for n > 0 {
			if n < len(payloads[0].data) {
				lastRelSeq, lastAt = payloads[0].relSeq, payloads[0].at
				payloads[0].data = payloads[0].data[n:]
				return
			}

			n -= len(payloads[0].data)
			lastRelSeq, lastAt = payloads[0].relSeq, payloads[0].at
			payloads = payloads[1:]
		}
	}

	nextPayload := func() {
		buffer.Reset()

		if len(payloads) < 1 {
			return
		}

		payloads = payloads[1:]
		if len(payloads) < 1 {
			return
		}

		for _, v := range payloads {
			buffer.Write(v.data)
		}

		lastRelSeq = payloads[0].relSeq
	}

	pushPayload := func(payloadData gamePacketPayload) {
		if buffer.Len() < 1 {
			buffer.Reset()
		}

		if len(payloads) < 1 {
			lastRelSeq, lastAt = payloadData.relSeq, payloadData.at
		}

		payloads = append(payloads, payloadData)
		buffer.Write(payloadData.data)
	}

	for {
		select {
		case <-t.ctx.Done():
			return

		case payloadData := <-payloadCh:
			pushPayload(payloadData)
		}

	readerLoop:
		for {
			msg, err := gamePacketReader(buffer, lastAt)
			if err != nil {
				if err == io.EOF {
					break readerLoop
				}

				logger.Printf("game packet parse error %v %v", lastRelSeq, err)
				nextPayload()
				continue
			}

			if msg != nil {
				// logger.Println("game packet", msg.Op, msg.Id, len(msg.Msg), lastRelSeq, lastAt)
				t.packetCh <- msg
				skipPayload(len(msg.RawPacket))
			}
		}
	}
}

func (t *GameServerPacketReader) openNic(nic string, filter string) (<-chan gamePacketPayload, error) {
	handle, err := pcap.OpenLive(nic, pcapBufferSize, pcapPromisc, pcap.BlockForever)
	if err != nil {
		logger.Println(err)
		return nil, err
	}
	t.handle = handle

	if err := handle.SetBPFFilter(filter); err != nil { // optional
		return nil, err
	}

	ch := make(chan gamePacketPayload, pcapQueueSize)
	// ps := gopacket.NewPacketSource(handle, handle.LinkType())

	go t.readPacketLoop(ch)

	return ch, nil
}

func (t *GameServerPacketReader) openFile(file string, filter string) (<-chan gamePacketPayload, error) {
	fd, err := os.OpenFile(file, os.O_RDONLY, 0o644)
	if err != nil {
		logger.Println(err)
		return nil, err
	}

	t.fd = fd

	/*
		调用OpenOffline函数时，fileName以UTF8传递，
		libpcap使用多字节fopen打开文件 -> 如果路径包含韩文则会损坏
		最好在libpcap中调用pcap_init(PCAP_CHAR_ENC_UTF_8)
	*/
	handle, err := pcap.OpenOfflineFile(fd)
	if err != nil {
		logger.Println(err)
		return nil, err
	}

	if err := handle.SetBPFFilter(filter); err != nil { // optional
		logger.Println(err)
		return nil, err
	}

	t.handle = handle

	ch := make(chan gamePacketPayload, pcapQueueSize)

	time.AfterFunc(20*time.Second, func() {
		logger.Println("start readPacketLoop", file)
		go t.readPacketLoop(ch)
	})

	return ch, nil
}

func (t *GameServerPacketReader) openLog() error {
	// 确保 dev_logs 目录存在
	logDir := "dev_logs"
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		logger.Println("创建日志目录失败:", err)
		return err
	}

	fileName := fmt.Sprintf("%s/packet_capture_%v.pcapng", logDir, constants.SERVER_START_AT)
	fd, err := os.OpenFile(fileName, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		logger.Println(err)
		return err
	}

	t.logFd = fd

	handle, err := pcapgo.NewNgWriter(fd, layers.LinkTypeEthernet)
	if err != nil {
		logger.Println(err)
		return err
	}

	t.logHandle = handle

	return nil
}

func (t *GameServerPacketReader) readPacketLoop(ch chan<- gamePacketPayload) {
	// 添加 panic recovery 防止程序崩溃
	defer func() {
		if r := recover(); r != nil {
			logger.Printf("【Panic恢复】错误: %v\n", r)
		}
	}()

	logger.Println("【开始读取数据包循环】")
	ethLayer := layers.Ethernet{}
	loopbackLayer := layers.Loopback{}
	ip4Layer := layers.IPv4{}
	tcpLayer := layers.TCP{}
	payload := gopacket.Payload{}

	// 根据网卡类型选择合适的解析器
	linkType := t.handle.LinkType()
	linkTypeStr := linkType.String()
	logger.Printf("【链路层类型】%v (值: %d, 字符串: %s)\n", linkType, linkType, linkTypeStr)

	var layerParser *gopacket.DecodingLayerParser
	if linkType == layers.LinkTypeNull || linkType == layers.LinkTypeLoop {
		layerParser = gopacket.NewDecodingLayerParser(layers.LayerTypeLoopback, &loopbackLayer, &ip4Layer, &tcpLayer, &payload)
		logger.Println("【使用 Loopback 解析器】")
	} else if linkType == layers.LinkTypeRaw || linkTypeStr == "Raw" || linkType == 101 {
		layerParser = gopacket.NewDecodingLayerParser(layers.LayerTypeIPv4, &ip4Layer, &tcpLayer, &payload)
		logger.Println("【使用 Raw IP 解析器】")
	} else {
		layerParser = gopacket.NewDecodingLayerParser(layers.LayerTypeEthernet, &ethLayer, &ip4Layer, &tcpLayer, &payload)
		logger.Println("【使用 Ethernet 解析器】")
	}
	packetLayers := []gopacket.LayerType(nil)

	// 防止数据包顺序混乱
	baseSeq := uint32(0)
	nextSeq, prevDstPort := uint32(0), layers.TCPPort(0)
	pendingTcpLayers := make([]pendingTcpLayer, 0, packetQueueSize)

	// 服务器信息是否已报告
	serverInfoReported := false

	for i := 0; t.ctx.Err() == nil; i++ {
		if i == 0 {
			logger.Printf("【准备读取】handle状态: %v, context错误: %v\n", t.handle != nil, t.ctx.Err())
		}

		b, ci, err := t.handle.ReadPacketData()
		if err != nil {
			logger.Printf("【读取错误】包序号: %d, 错误: %v, 错误类型: %T\n", i, err, err)
			break
		}

		if i == 0 {
			logger.Printf("【首个数据包】长度: %d, 时间: %v\n", len(b), ci.Timestamp)
		}
		if i < 10 || i%100 == 0 {
			logger.Printf("【数据包 #%d】长度: %d\n", i, len(b))
		}

		if t.logHandle != nil {
			// ignore error
			_ = t.logHandle.WritePacket(ci, b)
		}

		if err := layerParser.DecodeLayers(b, &packetLayers); err != nil {
			if i < 10 {
				logger.Printf("【解析失败】包序号: %d, 错误: %v\n", i, err)
			}
			continue
		}

		if i < 10 {
			logger.Printf("【解析成功】包序号: %d, 层数: %d, 层类型: %v\n", i, len(packetLayers), packetLayers)
		}

		if i == 0 {
			baseSeq = tcpLayer.Seq
		}

		for _, layer := range packetLayers {
			if layer != layers.LayerTypeTCP || len(tcpLayer.Payload) < 1 {
				if i < 10 {
					logger.Printf("【跳过】包序号: %d, 层类型: %v, TCP payload长度: %d\n", i, layer, len(tcpLayer.Payload))
				}
				continue
			}

			if i < 10 {
				logger.Printf("【TCP层】包序号: %d, payload长度: %d, 源: %s:%d\n", i, len(tcpLayer.Payload), ip4Layer.SrcIP, tcpLayer.SrcPort)
			}

			// 在加速器模式下，使用连接跟踪过滤
			if constants.AcceleratorMode {
				srcIP := ip4Layer.SrcIP.String()
				srcPort := uint16(tcpLayer.SrcPort)
				connKey := fmt.Sprintf("%s:%d", srcIP, srcPort)

				// 检查是否是已识别的游戏连接
				t.connMutex.RLock()
				isGameConn := t.gameConnections[connKey]
				gameConnCount := len(t.gameConnections)
				t.connMutex.RUnlock()

				// 如果不是已识别的游戏连接，进行严格验证
				if !isGameConn {
					// 使用严格的解析验证：只有能成功解析为游戏包的数据才通过
					if !canParseAsGamePacket(tcpLayer.Payload) {
						if i < 10 {
							logger.Printf("【加速器过滤】包序号: %d, 连接: %s, 原因: 非游戏包, 已识别连接数: %d, payload长度: %d\n",
								i, connKey, gameConnCount, len(tcpLayer.Payload))
						}
						// 如果已经有游戏连接，过滤掉无法解析的包
						if gameConnCount > 0 {
							continue
						}
						// 初始探测阶段也过滤掉无法解析的包
						continue
					}

					// 能够成功解析为游戏包，记录这个连接
					t.connMutex.Lock()
					t.gameConnections[connKey] = true
					t.connMutex.Unlock()
					logger.Printf("【加速器识别】包序号: %d, 新游戏连接: %s, payload长度: %d\n", i, connKey, len(tcpLayer.Payload))
				} else if i < 10 {
					logger.Printf("【加速器通过】包序号: %d, 已知游戏连接: %s, payload长度: %d\n", i, connKey, len(tcpLayer.Payload))
				}
			}

			// 捕获并报告服务器信息
			if !serverInfoReported || prevDstPort != tcpLayer.DstPort {
				srcIP := ip4Layer.SrcIP.String()
				srcPort := uint16(tcpLayer.SrcPort)
				t.serverIP = srcIP
				t.serverPort = srcPort

				if t.onServerInfo != nil {
					t.onServerInfo(srcIP, srcPort)
				}
				serverInfoReported = true
			}

			if nextSeq != 0 && tcpLayer.Seq != nextSeq {
				// 连接变更的情况（如频道移动等）
				if prevDstPort != tcpLayer.DstPort {
					for _, v := range pendingTcpLayers {
						ch <- gamePacketPayload{
							relSeq: v.tcpLayer.Seq - baseSeq,
							data:   v.tcpLayer.Payload,
							at:     v.ci.Timestamp,
						}
					}

					pendingTcpLayers = pendingTcpLayers[:0]
					prevDstPort = tcpLayer.DstPort

					baseSeq = tcpLayer.Seq
					nextSeq = tcpLayer.Seq + uint32(len(tcpLayer.Payload))

					if len(tcpLayer.Payload) == 4 {
						// crypt key
						continue
					}

					ch <- gamePacketPayload{
						relSeq: tcpLayer.Seq - baseSeq,
						data:   tcpLayer.Payload,
						at:     ci.Timestamp,
					}

					continue
				}

				/*
					对齐错误的情况
					1. 重传 - 重传时需要丢弃重叠部分
					2. 乱序 - 前面的数据包在后面到达的情况
				*/

				logger.Println("packet align error", i, nextSeq, tcpLayer.Seq)

				if tcpLayer.Seq < nextSeq {
					if tcpLayer.Seq+uint32(len(tcpLayer.Payload)) >= nextSeq {
						// 丢弃重叠部分
						payload := tcpLayer.Payload[nextSeq-tcpLayer.Seq:]
						if len(payload) > 0 {
							ch <- gamePacketPayload{
								relSeq: nextSeq - baseSeq,
								data:   payload,
								at:     ci.Timestamp,
							}
						}

						nextSeq = tcpLayer.Seq + uint32(len(tcpLayer.Payload))
						continue
					}
				}

				if len(pendingTcpLayers) >= packetQueueSize {
					// 满了的情况下保留列表并清空
					for _, v := range pendingTcpLayers {
						ch <- gamePacketPayload{
							relSeq: v.tcpLayer.Seq - baseSeq,
							data:   v.tcpLayer.Payload,
							at:     v.ci.Timestamp,
						}
					}

					pendingTcpLayers = pendingTcpLayers[:0]

					ch <- gamePacketPayload{
						relSeq: tcpLayer.Seq - baseSeq,
						data:   tcpLayer.Payload,
						at:     ci.Timestamp,
					}
					nextSeq = tcpLayer.Seq + uint32(len(tcpLayer.Payload))
					continue
				}

				pendingTcpLayers = append(pendingTcpLayers, pendingTcpLayer{
					tcpLayer: tcpLayer,
					ci:       ci,
				})
				continue
			}

			ch <- gamePacketPayload{
				relSeq: tcpLayer.Seq - baseSeq,
				data:   tcpLayer.Payload,
				at:     ci.Timestamp,
			}
			nextSeq = tcpLayer.Seq + uint32(len(tcpLayer.Payload))
			prevDstPort = tcpLayer.DstPort

			if len(pendingTcpLayers) > 0 {
				/*
					对齐错误的情况
					1. 重传 - 重传时需要丢弃重叠部分
					2. 乱序 - 前面的数据包在后面到达的情况
				*/

				for _, v := range pendingTcpLayers {
					if v.tcpLayer.Seq == nextSeq {
						ch <- gamePacketPayload{
							relSeq: v.tcpLayer.Seq - baseSeq,
							data:   v.tcpLayer.Payload,
							at:     v.ci.Timestamp,
						}
						nextSeq = v.tcpLayer.Seq + uint32(len(v.tcpLayer.Payload))
						pendingTcpLayers = pendingTcpLayers[1:]
						continue
					}

					if v.tcpLayer.Seq < nextSeq {
						payload := v.tcpLayer.Payload

						if v.tcpLayer.Seq+uint32(len(v.tcpLayer.Payload)) < nextSeq {
							pendingTcpLayers = pendingTcpLayers[1:]
							continue
						}

						// 丢弃重叠部分
						payload = payload[nextSeq-v.tcpLayer.Seq:]
						if len(payload) > 0 {
							ch <- gamePacketPayload{
								relSeq: nextSeq - baseSeq,
								data:   payload,
								at:     v.ci.Timestamp,
							}
						}

						nextSeq = v.tcpLayer.Seq + uint32(len(v.tcpLayer.Payload))
						pendingTcpLayers = pendingTcpLayers[1:]
						continue
					}

					// 还有未接收的数据包
					break
				}
			}

			continue
		}

		if i&((1<<10)-1) == 0 {
			time.Sleep(5 * time.Millisecond)
		}
	}

	for _, v := range pendingTcpLayers {
		ch <- gamePacketPayload{
			relSeq: v.tcpLayer.Seq - baseSeq,
			data:   v.tcpLayer.Payload,
			at:     v.ci.Timestamp,
		}
	}
}

func (t *GameServerPacketReader) Close() {
	if t.handle != nil {
		t.handle.Close()
		t.handle = nil
	}

	if t.fd != nil {
		t.fd.Close()
		t.fd = nil
	}

	if t.logHandle != nil {
		t.logHandle.Flush()
		t.logHandle = nil
	}

	if t.logFd != nil {
		t.logFd.Close()
		t.logFd = nil
	}
}

func (t *GameServerPacketReader) PacketCh() <-chan *GamePacket {
	return t.packetCh
}

// GetServerIP 获取服务器IP
func (t *GameServerPacketReader) GetServerIP() string {
	return t.serverIP
}

// GetServerPort 获取服务器端口
func (t *GameServerPacketReader) GetServerPort() uint16 {
	return t.serverPort
}

func gamePacketReader(buffer *bytes.Buffer, at time.Time) (*GamePacket, error) {
	headerSize := 6

	rawPacketBuffer := bytes.NewBuffer(nil)
	b := buffer.Bytes()

	// 头部读取仍然不足
	if len(b) < 6 {
		return nil, io.EOF
	}

	sign := b[0]
	// 数据包总大小（包含头部）
	length := le.Uint32(b[1:])
	// maybe
	flag := b[5]

	// ?
	if length == 0 || length > 0x100_0000 {
		err := fmt.Errorf("invalid packet length %v", length)
		return nil, err
	}

	if flag > 4 {
		err := fmt.Errorf("invalid flag %v", flag)
		return nil, err
	}

	isShortPacket := flag == 1 || // heartbeat
		flag == 2 // ? server only

	if isShortPacket {
		// 数据包仍然不足
		if len(b) < int(length)-6 {
			return nil, io.EOF
		}

		shortBody := b[6:int(length)]
		rawPacketBuffer.Write(shortBody)

		buffer.Next(int(length))

		// checksum := uint32(0)
		v := &GamePacket{
			At:     at,
			Sign:   sign,
			Length: length,
			Flag:   flag,

			IsShortPacket: true,
			ShortBody:     shortBody,

			RawPacket: rawPacketBuffer.Bytes(),
		}

		return v, nil
	}

	// too small
	if int(length) < headerSize+0xd {
		buffer.Next(int(length))
		return nil, ErrTooShortPacket
	}

	if buffer.Len() < int(length) {
		return nil, io.EOF
	}

	body := b[:int(length)]
	rawPacketBuffer.Write(body)

	buffer.Next(int(length))

	body = body[headerSize:]

	op := be.Uint32(body)
	body = body[4:]

	id := be.Uint64(body)
	body = body[8:]

	_, lenbytes := binary.Uvarint(body)
	if lenbytes <= 0 {
		err := fmt.Errorf("invalid message length %v", lenbytes)
		return nil, err
	}

	if len(body) < lenbytes {
		err := fmt.Errorf("invalid message length %v %v", len(body), lenbytes)
		return nil, err
	}

	body = body[lenbytes:]

	msg, err := NewMessage(bytes.NewReader(body))
	if err != nil {
		logger.Println("gameProxy packetHeader body read error", err)
		return nil, err
	}

	p := &GamePacket{
		At:     at,
		Sign:   sign,
		Length: length,
		Flag:   flag,

		Op:  op,
		Id:  id,
		Msg: msg,

		RawPacket: rawPacketBuffer.Bytes(),
	}

	return p, nil
}

// op, id, msg, err
func GamePacketBodyReader(r io.Reader) (uint32, uint64, Message, error) {
	b := make([]byte, 8)

	if _, err := io.ReadFull(r, b[:4]); err != nil {
		logger.Println(err)
		return 0, 0, nil, err
	}

	op := be.Uint32(b[:4])

	if _, err := io.ReadFull(r, b[:8]); err != nil {
		logger.Println(err)
		return 0, 0, nil, err
	}

	id := be.Uint64(b[:8])

	_, lenbytes, err := util.ReadUvarint(r)
	if err != nil {
		logger.Println(err)
		return 0, 0, nil, err
	}

	_ = lenbytes

	msg, err := NewMessage(r)
	if err != nil {
		logger.Println(err)
		return 0, 0, nil, err
	}

	return op, id, msg, nil
}
