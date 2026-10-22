package app

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

var defaultBuffOrder = []uint32{515, 680, 192, 193, 681, 194, 1225, 63, 1121, 1150, 835}

var targetOnlyNotificationBuffs = map[uint32]struct{}{
	1026: {}, // 倒吊者，仅出现在被标记的 Boss 身上
}

// buffFallbackDurations 用于状态包未提供可计算时长的 Buff。
// 当前数据库没有结构化的 Buff 时长字段。
// 835 马纽斯功能饮料（普通 10 分钟 / 强化 20 分钟）共用同一个状态 ID，
// 时长正常由 DURA 给出；缺失时按普通版兜底。
var buffFallbackDurations = map[uint32]int64{
	63:   1800,
	1121: 1800,
	1150: 1800,
	1225: 180,
	835:  600,
}

// buffNameVariant 让同一个状态 ID 按包内时长显示不同的名字。
type buffNameVariant struct {
	minDurationSec int64
	name           string
}

// buffNameVariants 处理"一个状态对应多个道具"的情况：
// 马纽斯功能饮料 835 的普通版和强化版共用一个状态 ID，只能按时长区分。
var buffNameVariants = map[uint32][]buffNameVariant{
	835: {
		{minDurationSec: 900, name: "强化的马纽斯功能饮料"}, // 20 分钟版
	},
}

// buffDisplayName 返回状态在该时长下应显示的名字；durationSec<=0 表示当前
// 没有活跃实例，返回白名单里的基础名字。
func (m *BuffTimerManager) buffDisplayName(ccId uint32, durationSec int64) string {
	name := m.targetBuffs[ccId]
	if durationSec <= 0 {
		return name
	}
	for _, variant := range buffNameVariants[ccId] {
		if durationSec >= variant.minDurationSec {
			name = variant.name
		}
	}
	return name
}

func resolveBuffDuration(ccID uint32, packetDuration int64) int64 {
	if packetDuration > 0 {
		return packetDuration
	}
	return buffFallbackDurations[ccID]
}

// adjustRepeatedBuffDuration handles 1026's stacked-time representation. The
// server keeps ACCVV1 at the base duration, but advances SBT by the extra
// duration when a new card extends an already active instance.
func adjustRepeatedBuffDuration(ccID uint32, duration int64, previousDisableAt, disableAt int64, elapsed time.Duration) int64 {
	if ccID != 1026 || duration <= 0 || previousDisableAt <= 0 || disableAt <= previousDisableAt {
		return duration
	}
	extraMilliseconds := (disableAt - previousDisableAt) - elapsed.Milliseconds()
	// Ignore normal packet jitter; the observed doubled activation adds about
	// 53 seconds beyond the wall-clock interval.
	if extraMilliseconds <= 1500 {
		return duration
	}
	extraSeconds := (extraMilliseconds + 500) / 1000
	return duration + extraSeconds
}

// BuffTimer 表示一个buff的定时器
type BuffTimer struct {
	CCId        uint32
	EntityId    uint64
	EntityName  string
	DisableAt   int64
	DurationSec int64
	StartTime   time.Time
	CancelFunc  context.CancelFunc
}

// BuffTimerManager 管理buff倒计时和语音提醒
type BuffTimerManager struct {
	timers           map[string]*BuffTimer
	mu               sync.RWMutex
	targetBuffs      map[uint32]string
	buffOrder        []uint32
	soundEnabled     map[uint32]bool
	notifyThresholds map[uint32]int64
	ctx              context.Context
	notifyThreshold  int64
	selfId           string
	soundDir         string
	getSoundVolume   func() int
}

// NewBuffTimerManager 创建新的BuffTimerManager
func NewBuffTimerManager(ctx context.Context, selfId string) *BuffTimerManager {
	return NewBuffTimerManagerWithVolume(ctx, selfId, func() int { return defaultSoundVolume })
}

// NewBuffTimerManagerWithVolume creates a Buff timer manager with an application-wide sound volume source.
func NewBuffTimerManagerWithVolume(ctx context.Context, selfId string, getSoundVolume func() int) *BuffTimerManager {
	if getSoundVolume == nil {
		getSoundVolume = func() int { return defaultSoundVolume }
	}
	soundDir := resolveSoundDir()
	logger.Printf("[BuffTimer] 音效目录: %s", soundDir)

	targetBuffs := map[uint32]string{
		515:  "状态支援（逆转）", // 以逆转(515)代表状态支援五件套
		192:  "活跃进行曲",
		193:  "行进曲",
		194:  "丰收之歌",
		680:  "战争序曲",
		681:  "忍耐之歌",
		1225: "超燃咚咚",
		63:   "攻击力增加",
		1121: "魔法攻击强化",
		1150: "炼金术伤害增加",
		835:  "马纽斯功能饮料",
		1026: "倒吊者",
	}
	buffOrder := append(append([]uint32(nil), defaultBuffOrder...), 1026)
	soundEnabled := make(map[uint32]bool, len(buffOrder))
	for _, ccId := range buffOrder {
		soundEnabled[ccId] = true
	}

	mgr := &BuffTimerManager{
		timers:       make(map[string]*BuffTimer),
		targetBuffs:  targetBuffs,
		buffOrder:    buffOrder,
		soundEnabled: soundEnabled,
		notifyThresholds: map[uint32]int64{
			515:  10, // 状态支援（逆转）提前 10 秒提醒
			1026: 10, // 倒吊者提前 10 秒提醒
			63:   30,
			1121: 30,
			1150: 30,
		},
		ctx:             ctx,
		notifyThreshold: 30,
		selfId:          selfId,
		getSoundVolume:  getSoundVolume,
		soundDir:        soundDir,
	}
	mgr.loadBuffOrder()
	return mgr
}

func resolveSoundDir() string {
	candidates := make([]string, 0, 3)

	if workDir := os.Getenv("MABI_WORK_DIR"); workDir != "" {
		candidates = append(candidates, filepath.Join(workDir, "sounds"))
	}

	if exePath, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exePath), "sounds"))
	}

	if cwd, err := os.Getwd(); err == nil && cwd != "" {
		for current := cwd; current != ""; current = filepath.Dir(current) {
			candidates = append(candidates, filepath.Join(current, "sounds"))
			parent := filepath.Dir(current)
			if parent == current {
				break
			}
		}
	}

	for _, dir := range candidates {
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
	}

	if len(candidates) > 0 {
		return candidates[0]
	}
	return ""
}

func buffOrderFilePath() string {
	candidates := make([]string, 0, 2)
	if workDir := os.Getenv("MABI_WORK_DIR"); workDir != "" {
		candidates = append(candidates, filepath.Join(workDir, "buff_order.json"))
	}
	if os.Getenv("MABI_WORK_DIR") == "" {
		candidates = append(candidates, filepath.Join(ensureUserDataDirectory(), "buff_order.json"))
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return "buff_order.json"
}

func (m *BuffTimerManager) isValidBuffOrder(order []uint32) bool {
	if len(order) != len(m.targetBuffs) {
		return false
	}
	seen := make(map[uint32]bool, len(order))
	for _, id := range order {
		if _, ok := m.targetBuffs[id]; !ok {
			return false
		}
		if seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

func (m *BuffTimerManager) loadBuffOrder() {
	path := buffOrderFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var order []uint32
	if err := json.Unmarshal(data, &order); err != nil {
		logger.Printf("[BuffTimer] 读取排序配置失败: %v", err)
		return
	}
	if !m.isValidBuffOrder(order) {
		logger.Printf("[BuffTimer] 排序配置无效，使用默认顺序")
		return
	}

	m.buffOrder = order
	logger.Printf("[BuffTimer] 已加载自定义排序: %v", order)
}

func (m *BuffTimerManager) saveBuffOrder() error {
	path := buffOrderFilePath()
	data, err := json.Marshal(m.buffOrder)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		logger.Printf("[BuffTimer] 创建排序配置目录失败: %v", err)
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		logger.Printf("[BuffTimer] 保存排序配置失败: %v", err)
		return err
	}
	return nil
}

// SetBuffOrder 设置buff显示顺序
func (m *BuffTimerManager) SetBuffOrder(order []uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isValidBuffOrder(order) {
		return fmt.Errorf("invalid buff order")
	}

	m.buffOrder = append([]uint32(nil), order...)
	if err := m.saveBuffOrder(); err != nil {
		return err
	}
	logger.Printf("[BuffTimer] 排序已更新: %v", order)
	return nil
}

// GetBuffOrder 获取当前buff显示顺序
func (m *BuffTimerManager) GetBuffOrder() []uint32 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]uint32(nil), m.buffOrder...)
}

// StartTimer 启动buff定时器
func (m *BuffTimerManager) StartTimer(ccId uint32, entityId uint64, entityName string, disableAt int64, duration int64) {
	if _, isTarget := m.targetBuffs[ccId]; !isTarget {
		return
	}
	duration = resolveBuffDuration(ccId, duration)
	buffName := m.buffDisplayName(ccId, duration)

	entityIdStr := strconv.FormatUint(entityId, 10)
	m.mu.RLock()
	selfID := m.selfId
	m.mu.RUnlock()
	_, targetOnly := targetOnlyNotificationBuffs[ccId]
	if targetOnly {
		if entityIdStr == selfID || !shouldMonitorTargetBuffCoverage(entityName) {
			return
		}
	} else if entityIdStr != selfID {
		return
	}

	if duration <= 0 {
		logger.Printf("[BuffTimer] duration<=0，不启动定时器")
		return
	}

	key := fmt.Sprintf("%d_%d", entityId, ccId)
	m.mu.RLock()
	existing := m.timers[key]
	m.mu.RUnlock()
	// 状态包也会发送属性更新；结束时间不变表示仍是同一个 Buff 实例。
	if existing != nil && disableAt > 0 && existing.DisableAt == disableAt {
		return
	}
	if existing != nil {
		adjustedDuration := adjustRepeatedBuffDuration(ccId, duration, existing.DisableAt, disableAt, time.Since(existing.StartTime))
		if adjustedDuration != duration {
			logger.Printf("[BuffTimer] 检测到倒吊者叠加时间: %d 秒 -> %d 秒", duration, adjustedDuration)
		}
		duration = adjustedDuration
	}
	m.cancelTimer(key)

	ctx, cancel := context.WithCancel(m.ctx)
	timer := &BuffTimer{
		CCId:        ccId,
		EntityId:    entityId,
		EntityName:  entityName,
		DisableAt:   disableAt,
		DurationSec: duration,
		StartTime:   time.Now(),
		CancelFunc:  cancel,
	}

	m.mu.Lock()
	m.timers[key] = timer
	m.mu.Unlock()

	go m.monitorBuff(ctx, timer, buffName, duration, ccId)
}

// StopTimer 停止buff定时器
func (m *BuffTimerManager) StopTimer(entityId uint64, ccId uint32) {
	key := fmt.Sprintf("%d_%d", entityId, ccId)
	m.cancelTimer(key)
}

func (m *BuffTimerManager) ResetForChannelChange() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for key, timer := range m.timers {
		if isChannelPersistentStatus(timer.CCId) {
			continue
		}
		timer.CancelFunc()
		delete(m.timers, key)
	}
}

func (m *BuffTimerManager) cancelTimer(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if timer, exists := m.timers[key]; exists {
		timer.CancelFunc()
		delete(m.timers, key)
	}
}

func (m *BuffTimerManager) cleanupTimer(entityId uint64, ccId uint32) {
	key := fmt.Sprintf("%d_%d", entityId, ccId)
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.timers, key)
}

func (m *BuffTimerManager) getNotifyThreshold(ccId uint32) int64 {
	if threshold, ok := m.notifyThresholds[ccId]; ok {
		return threshold
	}
	return m.notifyThreshold
}

func (m *BuffTimerManager) monitorBuff(ctx context.Context, timer *BuffTimer, buffName string, totalSec int64, ccId uint32) {
	notifyThreshold := m.getNotifyThreshold(ccId)
	notifyAfter := totalSec - notifyThreshold

	if notifyAfter > 0 {
		waitDuration := time.Duration(notifyAfter) * time.Second
		select {
		case <-time.After(waitDuration):
			elapsed := time.Since(timer.StartTime).Seconds()
			remaining := totalSec - int64(elapsed)
			if remaining > 0 {
				m.sendNotification(buffName, remaining, timer.EntityName, ccId)
			}
		case <-ctx.Done():
			logger.Printf("[BuffTimer] 定时器被取消: %s", buffName)
			return
		}
	} else {
		elapsed := time.Since(timer.StartTime).Seconds()
		remaining := totalSec - int64(elapsed)
		if remaining > 0 {
			m.sendNotification(buffName, remaining, timer.EntityName, ccId)
		}
	}

	remainingWait := time.Duration(totalSec)*time.Second - time.Since(timer.StartTime)
	if remainingWait > 0 {
		select {
		case <-time.After(remainingWait):
			logger.Printf("[BuffTimer] Buff已过期: %s", buffName)
		case <-ctx.Done():
			logger.Printf("[BuffTimer] 定时器被取消: %s", buffName)
			return
		}
	}

	m.cleanupTimer(timer.EntityId, timer.CCId)
}

func (m *BuffTimerManager) sendNotification(buffName string, remainingSec int64, entityName string, ccId uint32) {
	logger.Printf("[BuffTimer] 发送通知: %s 还剩 %d 秒 (角色: %s)", buffName, remainingSec, entityName)
	m.playSound(ccId)
}

func (m *BuffTimerManager) playSound(ccId uint32) {
	m.mu.RLock()
	enabled := m.soundEnabled[ccId]
	m.mu.RUnlock()

	if !enabled {
		logger.Printf("[BuffTimer] Buff %d 声音已关闭，不播放音效", ccId)
		return
	}

	audioFile := m.resolveAudioFile(fmt.Sprintf("%d.wav", ccId))
	if audioFile == "" {
		logger.Printf("[BuffTimer] 音效文件不存在: %d.wav", ccId)
		return
	}

	go playWavFile(audioFile, m.getSoundVolume())
}

func (m *BuffTimerManager) resolveAudioFile(filename string) string {
	candidates := make([]string, 0, 4)
	if m.soundDir != "" {
		candidates = append(candidates, m.soundDir)
	}
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates, filepath.Join(exeDir, "..", "..", "sounds"))
	}
	if cwd, err := os.Getwd(); err == nil && cwd != "" {
		candidates = append(candidates, filepath.Join(cwd, "sounds"))
	}

	for _, dir := range candidates {
		path := filepath.Join(dir, filename)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

var (
	audioPlaybackSequence atomic.Uint64
	audioPlaybackMu       sync.Mutex
	audioPlaybackBuffers  = make(map[uint64][]byte)
)

func playWavFile(audioFile string, volume int) {
	audioData, duration, err := prepareWavPlayback(audioFile, volume)
	if err != nil {
		logger.Printf("[Audio] 读取音效失败: %s, error: %v", audioFile, err)
		return
	}
	if len(audioData) == 0 {
		logger.Printf("[Audio] 音效文件为空: %s", audioFile)
		return
	}

	winmm := syscall.NewLazyDLL("winmm.dll")
	playSound := winmm.NewProc("PlaySoundW")
	playbackID := audioPlaybackSequence.Add(1)
	audioPlaybackMu.Lock()
	audioPlaybackBuffers[playbackID] = audioData
	audioPlaybackMu.Unlock()

	ret, _, lastErr := playSound.Call(
		uintptr(unsafe.Pointer(&audioData[0])),
		0,
		0x00000007, // SND_ASYNC | SND_MEMORY | SND_NODEFAULT
	)
	if ret == 0 {
		audioPlaybackMu.Lock()
		delete(audioPlaybackBuffers, playbackID)
		audioPlaybackMu.Unlock()
		logger.Printf("[Audio] 播放音效失败: %s, error: %v", audioFile, lastErr)
		return
	}

	time.AfterFunc(duration+time.Second, func() {
		audioPlaybackMu.Lock()
		delete(audioPlaybackBuffers, playbackID)
		audioPlaybackMu.Unlock()
	})
	logger.Printf("[Audio] 播放成功: %s", audioFile)
}

func prepareWavPlayback(audioFile string, volume int) ([]byte, time.Duration, error) {
	audioData, err := os.ReadFile(audioFile)
	if err != nil {
		return nil, 0, err
	}
	if len(audioData) < 12 || string(audioData[:4]) != "RIFF" || string(audioData[8:12]) != "WAVE" {
		return audioData, 30 * time.Second, nil
	}

	var (
		audioFormat   uint16
		bitsPerSample uint16
		byteRate      uint32
		dataOffset    int
		dataLength    int
	)
	for offset := 12; offset+8 <= len(audioData); {
		chunkSize := int(binary.LittleEndian.Uint32(audioData[offset+4 : offset+8]))
		chunkStart := offset + 8
		chunkEnd := chunkStart + chunkSize
		if chunkEnd > len(audioData) {
			break
		}
		switch string(audioData[offset : offset+4]) {
		case "fmt ":
			if chunkSize >= 16 {
				audioFormat = binary.LittleEndian.Uint16(audioData[chunkStart : chunkStart+2])
				byteRate = binary.LittleEndian.Uint32(audioData[chunkStart+8 : chunkStart+12])
				bitsPerSample = binary.LittleEndian.Uint16(audioData[chunkStart+14 : chunkStart+16])
			}
		case "data":
			dataOffset = chunkStart
			dataLength = chunkSize
		}
		offset = chunkEnd
		if chunkSize%2 != 0 {
			offset++
		}
	}

	duration := 30 * time.Second
	if byteRate > 0 && dataLength > 0 {
		duration = time.Duration(dataLength) * time.Second / time.Duration(byteRate)
	}
	if volume == defaultSoundVolume || dataLength == 0 || audioFormat != 1 || bitsPerSample != 16 {
		return audioData, duration, nil
	}

	scaledData := append([]byte(nil), audioData...)
	volume = normalizeSoundVolume(volume)
	for offset := dataOffset; offset+1 < dataOffset+dataLength; offset += 2 {
		sample := int16(binary.LittleEndian.Uint16(scaledData[offset : offset+2]))
		scaledSample := int16(int32(sample) * int32(volume) / defaultSoundVolume)
		binary.LittleEndian.PutUint16(scaledData[offset:offset+2], uint16(scaledSample))
	}
	return scaledData, duration, nil
}

// SetNotifyThreshold 设置通知阈值（秒）
func (m *BuffTimerManager) SetNotifyThreshold(seconds int64) {
	m.notifyThreshold = seconds
	logger.Printf("[BuffTimer] 通知阈值设置为 %d 秒", seconds)
}

// GetActiveTimers 获取当前活跃的定时器数量
func (m *BuffTimerManager) GetActiveTimers() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.timers)
}

// BuffInfo buff信息
type BuffInfo struct {
	CCId     uint32 `json:"ccId"`
	BuffName string `json:"buffName"`
}

// BuffDisplayInfo 界面展示的buff信息（含定时器与声音设置）
type BuffDisplayInfo struct {
	CCId            uint32 `json:"ccId"`
	BuffName        string `json:"buffName"`
	SoundEnabled    bool   `json:"soundEnabled"`
	IsActive        bool   `json:"isActive"`
	EntityId        uint64 `json:"entityId"`
	EntityName      string `json:"entityName"`
	RemainingTime   int64  `json:"remainingTime"`
	TotalTime       int64  `json:"totalTime"`
	NotifyThreshold int64  `json:"notifyThreshold"`
	WillNotify      bool   `json:"willNotify"`
}

// ActiveBuffTimerInfo 活跃的buff定时器信息
type ActiveBuffTimerInfo struct {
	CCId          uint32 `json:"ccId"`
	BuffName      string `json:"buffName"`
	EntityId      uint64 `json:"entityId"`
	EntityName    string `json:"entityName"`
	RemainingTime int64  `json:"remainingTime"`
	TotalTime     int64  `json:"totalTime"`
	NotifyAt      int64  `json:"notifyAt"`
	WillNotify    bool   `json:"willNotify"`
}

// GetMonitoredBuffs 获取所有监控的buff列表
func (m *BuffTimerManager) GetMonitoredBuffs() []BuffInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	buffs := make([]BuffInfo, 0, len(m.buffOrder))
	for _, ccId := range m.buffOrder {
		buffs = append(buffs, BuffInfo{
			CCId:     ccId,
			BuffName: m.targetBuffs[ccId],
		})
	}
	return buffs
}

// GetBuffDisplayList 获取所有监控buff的展示信息（含活跃定时器与声音开关）
func (m *BuffTimerManager) GetBuffDisplayList() []BuffDisplayInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	activeByCCId := make(map[uint32]*BuffTimer, len(m.timers))
	for _, timer := range m.timers {
		activeByCCId[timer.CCId] = timer
	}

	infos := make([]BuffDisplayInfo, 0, len(m.buffOrder))
	for _, ccId := range m.buffOrder {
		info := BuffDisplayInfo{
			CCId:            ccId,
			BuffName:        m.targetBuffs[ccId],
			SoundEnabled:    m.soundEnabled[ccId],
			NotifyThreshold: m.getNotifyThreshold(ccId),
		}

		if timer, ok := activeByCCId[ccId]; ok {
			info.BuffName = m.buffDisplayName(ccId, timer.DurationSec)
			elapsed := time.Since(timer.StartTime).Seconds()
			remainingSec := timer.DurationSec - int64(elapsed)
			if remainingSec < 0 {
				remainingSec = 0
			}

			notifyAtSec := remainingSec - info.NotifyThreshold
			info.IsActive = true
			info.EntityId = timer.EntityId
			info.EntityName = timer.EntityName
			info.RemainingTime = remainingSec
			info.TotalTime = timer.DurationSec
			info.WillNotify = notifyAtSec > 0 && info.SoundEnabled
		}

		infos = append(infos, info)
	}

	return infos
}

// SetBuffSoundEnabled 设置单个buff的声音开关
func (m *BuffTimerManager) SetBuffSoundEnabled(ccId uint32, enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.targetBuffs[ccId]; !ok {
		return
	}

	m.soundEnabled[ccId] = enabled
	logger.Printf("[BuffTimer] Buff %d 声音开关: %v", ccId, enabled)
}

// IsBuffSoundEnabled 获取单个buff的声音开关状态
func (m *BuffTimerManager) IsBuffSoundEnabled(ccId uint32) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.soundEnabled[ccId]
}

// GetActiveTimersInfo 获取当前活跃的定时器详细信息
func (m *BuffTimerManager) GetActiveTimersInfo() []ActiveBuffTimerInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]ActiveBuffTimerInfo, 0, len(m.timers))
	for _, timer := range m.timers {
		elapsed := time.Since(timer.StartTime).Seconds()
		remainingSec := timer.DurationSec - int64(elapsed)
		if remainingSec < 0 {
			remainingSec = 0
		}

		notifyThreshold := m.getNotifyThreshold(timer.CCId)
		notifyAtSec := remainingSec - notifyThreshold
		willNotify := notifyAtSec > 0

		infos = append(infos, ActiveBuffTimerInfo{
			CCId:          timer.CCId,
			BuffName:      m.buffDisplayName(timer.CCId, timer.DurationSec),
			EntityId:      timer.EntityId,
			EntityName:    timer.EntityName,
			RemainingTime: remainingSec,
			TotalTime:     timer.DurationSec,
			NotifyAt:      notifyAtSec,
			WillNotify:    willNotify,
		})
	}

	return infos
}

// GetNotifyThreshold 获取通知阈值
func (m *BuffTimerManager) GetNotifyThreshold() int64 {
	return m.notifyThreshold
}

// SetSelfId 设置玩家自身ID
func (m *BuffTimerManager) SetSelfId(selfId string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.selfId = selfId
	logger.Printf("[BuffTimer] 设置玩家自身ID: %s", selfId)
}

// PlayCustomSound 播放自定义音效文件
func (m *BuffTimerManager) PlayCustomSound(filename string) {
	audioFile := m.resolveAudioFile(filename)
	if audioFile == "" {
		logger.Printf("[BuffTimer] 音效文件不存在: %s", filename)
		return
	}

	go playWavFile(audioFile, m.getSoundVolume())
}
