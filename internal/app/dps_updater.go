package app

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// DPSUpdateThrottler DPS更新节流器
// 用于合并多个攻击者/目标的更新事件，减少前端刷新频率
type DPSUpdateThrottler struct {
	app           *App
	pending       atomic.Bool // 是否有待发送的更新
	minInterval   time.Duration
	lastEmit      time.Time
	lastEmitMu    sync.Mutex
}

// NewDPSUpdateThrottler 创建DPS更新节流器
func NewDPSUpdateThrottler(app *App) *DPSUpdateThrottler {
	return &DPSUpdateThrottler{
		app:         app,
		minInterval: 100 * time.Millisecond, // 最小间隔100ms
	}
}

// RequestUpdate 请求更新（会被节流）
func (t *DPSUpdateThrottler) RequestUpdate() {
	// 如果已经有待处理的更新，直接返回
	if t.pending.Load() {
		return
	}

	t.lastEmitMu.Lock()
	timeSinceLastEmit := time.Since(t.lastEmit)
	t.lastEmitMu.Unlock()

	if timeSinceLastEmit >= t.minInterval {
		// 已经过了最小间隔，直接发送
		t.emitUpdate()
	} else {
		// 还没到最小间隔，设置定时发送
		if t.pending.CompareAndSwap(false, true) {
			go func() {
				time.Sleep(t.minInterval - timeSinceLastEmit)
				t.emitUpdate()
				t.pending.Store(false)
			}()
		}
	}
}

// emitUpdate 实际发送更新事件
func (t *DPSUpdateThrottler) emitUpdate() {
	t.lastEmitMu.Lock()
	t.lastEmit = time.Now()
	t.lastEmitMu.Unlock()

	runtime.EventsEmit(t.app.ctx, "dps-update")
}

// AttackerTimer 单个攻击者的DPS更新计时器
type AttackerTimer struct {
	attackerId string
	lastAttack time.Time
	ticker     *time.Ticker
	stopChan   chan bool
	isRunning  bool
	mu         sync.Mutex

	// 固定配置
	updateInterval time.Duration // 1秒
	idleTimeout    time.Duration // 8秒

	// 回调函数
	onUpdate func(attackerId string)
	onStop   func(attackerId string)
}

// Start 启动攻击者计时器
func (t *AttackerTimer) Start() {
	t.mu.Lock()
	if t.isRunning {
		t.mu.Unlock()
		return
	}

	t.isRunning = true
	t.lastAttack = time.Now()
	t.ticker = time.NewTicker(t.updateInterval)
	t.mu.Unlock()

	go func() {
		for {
			select {
			case <-t.ticker.C:
				// 检查是否超时
				t.mu.Lock()
				idleTime := time.Since(t.lastAttack)
				t.mu.Unlock()

				if idleTime > t.idleTimeout {
					// 超时，先触发最后一次更新（让前端获取idle状态）
					if t.onUpdate != nil {
						t.onUpdate(t.attackerId)
					}
					// 然后停止计时器
					t.Stop()
					if t.onStop != nil {
						t.onStop(t.attackerId)
					}
					return
				}

				// 触发更新回调
				if t.onUpdate != nil {
					t.onUpdate(t.attackerId)
				}

			case <-t.stopChan:
				return
			}
		}
	}()
}

// Stop 停止攻击者计时器
func (t *AttackerTimer) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.isRunning {
		return
	}

	t.isRunning = false
	t.ticker.Stop()
	close(t.stopChan)
}

// Reset 重置超时时间（收到新攻击时调用）
func (t *AttackerTimer) Reset() {
	t.mu.Lock()
	t.lastAttack = time.Now()
	wasRunning := t.isRunning
	t.mu.Unlock()

	// 如果计时器未运行，启动它
	if !wasRunning {
		t.Start()
	}
}

// AttackerTimerManager 攻击者计时器管理器
type AttackerTimerManager struct {
	timers map[string]*AttackerTimer
	mu     sync.RWMutex
	app    *App
}

// NewAttackerTimerManager 创建攻击者计时器管理器
func NewAttackerTimerManager(app *App) *AttackerTimerManager {
	return &AttackerTimerManager{
		timers: make(map[string]*AttackerTimer),
		app:    app,
	}
}

// OnAttack 处理新的攻击事件
func (m *AttackerTimerManager) OnAttack(attackerId string) {
	m.mu.Lock()
	timer := m.timers[attackerId]
	if timer == nil {
		// 创建新计时器
		timer = &AttackerTimer{
			attackerId:     attackerId,
			updateInterval: 1 * time.Second,
			idleTimeout:    8 * time.Second,
			stopChan:       make(chan bool),
			onUpdate: func(id string) {
				// 使用节流器触发前端更新
				if m.app.dpsUpdateThrottler != nil {
					m.app.dpsUpdateThrottler.RequestUpdate()
				}
			},
			onStop: func(id string) {
				// 清理计时器
				m.removeTimer(id)
			},
		}
		m.timers[attackerId] = timer
		m.mu.Unlock()
		timer.Start()
	} else {
		m.mu.Unlock()
		timer.Reset()
	}
}

// removeTimer 移除计时器
func (m *AttackerTimerManager) removeTimer(attackerId string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.timers, attackerId)
}

// TargetTimer 单个目标的DPS更新计时器
type TargetTimer struct {
	targetId  string
	lastHit   time.Time
	ticker    *time.Ticker
	stopChan  chan bool
	isRunning bool
	isDead    bool // 标记目标是否已死亡
	mu        sync.Mutex

	// 固定配置
	updateInterval time.Duration // 1秒
	idleTimeout    time.Duration // 8秒

	// 回调函数
	onUpdate func(targetId string)
	onStop   func(targetId string)
}

// Start 启动目标计时器
func (t *TargetTimer) Start() {
	t.mu.Lock()
	if t.isRunning {
		t.mu.Unlock()
		return
	}

	t.isRunning = true
	t.lastHit = time.Now()
	t.ticker = time.NewTicker(t.updateInterval)
	t.mu.Unlock()

	go func() {
		for {
			select {
			case <-t.ticker.C:
				t.mu.Lock()

				// 检查是否已死亡
				if t.isDead {
					t.mu.Unlock()
					// 先触发最后一次更新（让前端获取dead状态）
					if t.onUpdate != nil {
						t.onUpdate(t.targetId)
					}
					t.Stop()
					if t.onStop != nil {
						t.onStop(t.targetId)
					}
					return
				}

				// 检查是否超时
				idleTime := time.Since(t.lastHit)
				t.mu.Unlock()

				if idleTime > t.idleTimeout {
					// 先触发最后一次更新（让前端获取idle状态）
					if t.onUpdate != nil {
						t.onUpdate(t.targetId)
					}
					t.Stop()
					if t.onStop != nil {
						t.onStop(t.targetId)
					}
					return
				}

				// 触发更新
				if t.onUpdate != nil {
					t.onUpdate(t.targetId)
				}

			case <-t.stopChan:
				return
			}
		}
	}()
}

// Stop 停止目标计时器
func (t *TargetTimer) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.isRunning {
		return
	}

	t.isRunning = false
	t.ticker.Stop()
	close(t.stopChan)
}

// Reset 重置超时时间（目标受到新攻击时调用）
func (t *TargetTimer) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lastHit = time.Now()
	// 只更新时间，不重新启动计时器
	// 重新启动的逻辑由 OnHit 处理
}

// MarkDead 标记目标死亡
func (t *TargetTimer) MarkDead() {
	t.mu.Lock()
	t.isDead = true
	t.mu.Unlock()

	// 死亡后立即停止计时器
	t.Stop()
}

// TargetTimerManager 目标计时器管理器
type TargetTimerManager struct {
	timers map[string]*TargetTimer
	mu     sync.RWMutex
	app    *App
}

// NewTargetTimerManager 创建目标计时器管理器
func NewTargetTimerManager(app *App) *TargetTimerManager {
	return &TargetTimerManager{
		timers: make(map[string]*TargetTimer),
		app:    app,
	}
}

// OnHit 处理目标受击事件
func (m *TargetTimerManager) OnHit(targetId string) {
	m.mu.Lock()
	timer := m.timers[targetId]

	if timer == nil {
		// 创建新计时器
		timer = m.createTargetTimer(targetId)
		m.timers[targetId] = timer
		m.mu.Unlock()
		timer.Start()
		return
	}

	// 检查计时器是否还在运行
	timer.mu.Lock()
	isRunning := timer.isRunning
	timer.mu.Unlock()

	if !isRunning {
		// 计时器已停止，删除旧的，创建新的
		delete(m.timers, targetId)
		timer = m.createTargetTimer(targetId)
		m.timers[targetId] = timer
		m.mu.Unlock()
		timer.Start()
	} else {
		// 计时器还在运行，重置时间
		m.mu.Unlock()
		timer.Reset()
	}
}

// OnDeath 处理目标死亡事件
func (m *TargetTimerManager) OnDeath(targetId string) {
	m.mu.RLock()
	timer := m.timers[targetId]
	m.mu.RUnlock()

	if timer != nil {
		timer.MarkDead()
	}
}

// removeTimer 移除计时器
func (m *TargetTimerManager) removeTimer(targetId string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.timers, targetId)
}

// createTargetTimer 创建新的目标计时器
func (m *TargetTimerManager) createTargetTimer(targetId string) *TargetTimer {
	timer := &TargetTimer{
		targetId:       targetId,
		updateInterval: 1 * time.Second,
		idleTimeout:    8 * time.Second,
		stopChan:       make(chan bool),
		onUpdate: func(id string) {
			// 使用节流器触发前端更新
			if m.app.dpsUpdateThrottler != nil {
				m.app.dpsUpdateThrottler.RequestUpdate()
			}
		},
	}

	// onStop 回调中检查计时器对象是否匹配
	// 避免删除已被替换的新计时器
	timer.onStop = func(id string) {
		m.mu.Lock()
		defer m.mu.Unlock()
		// 只有当 map 中的计时器对象和当前计时器对象相同时，才删除
		if m.timers[id] == timer {
			delete(m.timers, id)
		}
	}

	return timer
}
