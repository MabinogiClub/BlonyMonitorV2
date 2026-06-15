package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"blonymonitorv2/db"
)

var defaultBuffOrder = []uint32{680, 192, 193, 681, 194}

// BuffTimer 表示一个buff的定时器
type BuffTimer struct {
	CCId        uint32
	EntityId    uint64
	EntityName  string
	DurationSec int64
	StartTime   time.Time
	CancelFunc  context.CancelFunc
}

// BuffTimerManager 管理buff倒计时和语音提醒
type BuffTimerManager struct {
	timers          map[string]*BuffTimer
	mu              sync.RWMutex
	targetBuffs     map[uint32]string
	buffOrder       []uint32
	soundEnabled    map[uint32]bool
	ctx             context.Context
	notifyThreshold int64
	selfId          string
	soundDir        string
}

// NewBuffTimerManager 创建新的BuffTimerManager
func NewBuffTimerManager(ctx context.Context, selfId string) *BuffTimerManager {
	soundDir := resolveSoundDir()
	logger.Printf("[BuffTimer] 音效目录: %s", soundDir)

	targetBuffs := map[uint32]string{
		192: "活跃进行曲",
		193: "行进曲",
		194: "丰收之歌",
		680: "战争序曲",
		681: "忍耐之歌",
	}
	buffOrder := append([]uint32(nil), defaultBuffOrder...)
	soundEnabled := make(map[uint32]bool, len(buffOrder))
	for _, ccId := range buffOrder {
		soundEnabled[ccId] = true
	}

	mgr := &BuffTimerManager{
		timers:          make(map[string]*BuffTimer),
		targetBuffs:     targetBuffs,
		buffOrder:       buffOrder,
		soundEnabled:    soundEnabled,
		ctx:             ctx,
		notifyThreshold: 30,
		selfId:          selfId,
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
		candidates = append(candidates, filepath.Join(cwd, "sounds"))
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
	candidates := make([]string, 0, 3)
	if workDir := os.Getenv("MABI_WORK_DIR"); workDir != "" {
		candidates = append(candidates, filepath.Join(workDir, "buff_order.json"))
	}
	if exePath, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exePath), "buff_order.json"))
	}
	if cwd, err := os.Getwd(); err == nil && cwd != "" {
		candidates = append(candidates, filepath.Join(cwd, "buff_order.json"))
	}

	for _, path := range candidates {
		if _, err := os.Stat(filepath.Dir(path)); err == nil {
			return path
		}
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
func (m *BuffTimerManager) StartTimer(ccId uint32, entityId uint64, entityName string, duration int64) {
	buffName, isTarget := m.targetBuffs[ccId]
	if !isTarget {
		return
	}

	entityIdStr := strconv.FormatUint(entityId, 10)
	if entityIdStr != m.selfId {
		return
	}

	if duration <= 0 {
		logger.Printf("[BuffTimer] duration<=0，不启动定时器")
		return
	}

	key := fmt.Sprintf("%d_%d", entityId, ccId)
	m.cancelTimer(key)

	ctx, cancel := context.WithCancel(m.ctx)
	timer := &BuffTimer{
		CCId:        ccId,
		EntityId:    entityId,
		EntityName:  entityName,
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

func (m *BuffTimerManager) monitorBuff(ctx context.Context, timer *BuffTimer, buffName string, totalSec int64, ccId uint32) {
	notifyAfter := totalSec - m.notifyThreshold

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

	go playWavFile(audioFile)
}

func (m *BuffTimerManager) resolveAudioFile(filename string) string {
	if m.soundDir != "" {
		path := filepath.Join(m.soundDir, filename)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	if cwd, err := os.Getwd(); err == nil && cwd != "" {
		path := filepath.Join(cwd, "sounds", filename)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

func playWavFile(audioFile string) {
	winmm := syscall.NewLazyDLL("winmm.dll")
	playSound := winmm.NewProc("PlaySoundW")

	utf16Path, err := syscall.UTF16PtrFromString(audioFile)
	if err != nil {
		logger.Printf("[BuffTimer] 路径转换失败: %v", err)
		return
	}

	ret, _, lastErr := playSound.Call(
		uintptr(unsafe.Pointer(utf16Path)),
		0,
		0x00020003,
	)
	if ret == 0 {
		logger.Printf("[BuffTimer] 播放音效失败: %s, lastErr: %v", audioFile, lastErr)
	} else {
		logger.Printf("[BuffTimer] 播放成功: %s", audioFile)
	}
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
	CCId          uint32 `json:"ccId"`
	BuffName      string `json:"buffName"`
	IconData      string `json:"iconData"`
	SoundEnabled  bool   `json:"soundEnabled"`
	IsActive      bool   `json:"isActive"`
	EntityId      uint64 `json:"entityId"`
	EntityName    string `json:"entityName"`
	RemainingTime int64  `json:"remainingTime"`
	TotalTime     int64  `json:"totalTime"`
	WillNotify    bool   `json:"willNotify"`
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
			CCId:         ccId,
			BuffName:     m.targetBuffs[ccId],
			IconData:     db.GetConditionIcon(int(ccId)),
			SoundEnabled: m.soundEnabled[ccId],
		}

		if timer, ok := activeByCCId[ccId]; ok {
			elapsed := time.Since(timer.StartTime).Seconds()
			remainingSec := timer.DurationSec - int64(elapsed)
			if remainingSec < 0 {
				remainingSec = 0
			}

			notifyAtSec := remainingSec - m.notifyThreshold
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

		notifyAtSec := remainingSec - m.notifyThreshold
		willNotify := notifyAtSec > 0

		infos = append(infos, ActiveBuffTimerInfo{
			CCId:          timer.CCId,
			BuffName:      m.targetBuffs[timer.CCId],
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

	go playWavFile(audioFile)
}
