package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"blonymonitorv2/internal/config"
)

const (
	analysisLogFileName     = "blonymonitor-analysis.log"
	analysisLogSettingsName = "blonymonitor-analysis-settings.json"
	analysisLogSamplePeriod = 5 * time.Second
	analysisLogMaxSize      = 16 * 1024 * 1024
)

type analysisLogSettings struct {
	Enabled bool `json:"enabled"`
}

type analysisLogCounters struct {
	packetCount          atomic.Uint64
	conditionPacketCount atomic.Uint64
	packetNanos          atomic.Uint64
	packetMaxNanos       atomic.Uint64
	damageTakenCalls     atomic.Uint64
	damageTakenNanos     atomic.Uint64
	damageTakenMaxNanos  atomic.Uint64
	coverageCalls        atomic.Uint64
	coverageNanos        atomic.Uint64
	coverageMaxNanos     atomic.Uint64
}

type analysisLogController struct {
	mu            sync.Mutex
	toggleMu      sync.Mutex
	file          *os.File
	cancel        context.CancelFunc
	done          chan struct{}
	enabled       atomic.Bool
	cpuAvailable  bool
	previousCPU   time.Duration
	previousCPUAt time.Time
	counters      analysisLogCounters
}

func analysisLogDirectory() string {
	// MABI_WORK_DIR is used by tests and portable development builds.
	if dir := os.Getenv("MABI_WORK_DIR"); dir != "" {
		return dir
	}
	if exe, err := os.Executable(); err == nil {
		return filepath.Dir(exe)
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

func analysisLogPath() string {
	return filepath.Join(analysisLogDirectory(), analysisLogFileName)
}

func analysisLogSettingsPath() string {
	return filepath.Join(analysisLogDirectory(), analysisLogSettingsName)
}

func loadAnalysisLogEnabled() bool {
	data, err := os.ReadFile(analysisLogSettingsPath())
	if err != nil {
		return false
	}
	var settings analysisLogSettings
	return json.Unmarshal(data, &settings) == nil && settings.Enabled
}

func saveAnalysisLogEnabled(enabled bool) error {
	data, err := json.Marshal(analysisLogSettings{Enabled: enabled})
	if err != nil {
		return err
	}
	return os.WriteFile(analysisLogSettingsPath(), data, 0o644)
}

func (a *App) analysisLoggingEnabled() bool {
	return a.analysisLog != nil && a.analysisLog.enabled.Load()
}

func (a *App) writeAnalysisLog(format string, args ...any) {
	if a.analysisLog == nil {
		return
	}
	a.analysisLog.mu.Lock()
	defer a.analysisLog.mu.Unlock()
	if a.analysisLog.file == nil {
		return
	}
	_, _ = fmt.Fprintf(a.analysisLog.file, "%s "+format+"\n", append([]any{time.Now().Format("2006-01-02 15:04:05.000")}, args...)...)
}

// GetAnalysisLogEnabled reports whether diagnostic sampling is active.
func (a *App) GetAnalysisLogEnabled() bool {
	return a.analysisLoggingEnabled()
}

// GetAnalysisLogPath returns the path of the diagnostic log file.
func (a *App) GetAnalysisLogPath() string {
	return analysisLogPath()
}

// SetAnalysisLogEnabled toggles the low-frequency analysis log.
func (a *App) SetAnalysisLogEnabled(enabled bool) error {
	if a.analysisLog == nil {
		a.analysisLog = &analysisLogController{}
	}
	a.analysisLog.toggleMu.Lock()
	defer a.analysisLog.toggleMu.Unlock()
	if enabled == a.analysisLoggingEnabled() {
		return saveAnalysisLogEnabled(enabled)
	}
	if enabled {
		return a.startAnalysisLog()
	}
	a.stopAnalysisLog()
	return saveAnalysisLogEnabled(false)
}

func (a *App) startAnalysisLog() error {
	c := a.analysisLog
	if c == nil {
		c = &analysisLogController{}
		a.analysisLog = c
	}

	flags := os.O_CREATE | os.O_APPEND | os.O_WRONLY
	if info, statErr := os.Stat(analysisLogPath()); statErr == nil && info.Size() >= analysisLogMaxSize {
		flags = os.O_CREATE | os.O_TRUNC | os.O_WRONLY
	}
	file, err := os.OpenFile(analysisLogPath(), flags, 0o644)
	if err != nil {
		return err
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	monitorCtx, cancel := context.WithCancel(ctx)
	previousCPU, cpuErr := processCPUTime()
	c.resetCounters()
	c.mu.Lock()
	c.file = file
	c.cancel = cancel
	c.done = make(chan struct{})
	c.cpuAvailable = cpuErr == nil
	c.previousCPU = previousCPU
	c.previousCPUAt = time.Now()
	c.enabled.Store(true)
	c.mu.Unlock()
	if err := saveAnalysisLogEnabled(true); err != nil {
		a.stopAnalysisLog()
		return err
	}

	a.writeAnalysisLog("SESSION_START version=%q pid=%d go=%s os=%s arch=%s cpu_count=%d gomaxprocs=%d log=%q",
		config.ClientVersion, os.Getpid(), runtime.Version(), runtime.GOOS, runtime.GOARCH,
		runtime.NumCPU(), runtime.GOMAXPROCS(0), analysisLogPath())
	if cpuErr != nil {
		a.writeAnalysisLog("PROCESS_CPU_UNAVAILABLE error=%q", cpuErr)
	}
	go a.runAnalysisLog(monitorCtx, c)
	return nil
}

func (c *analysisLogController) resetCounters() {
	c.counters.packetCount.Store(0)
	c.counters.conditionPacketCount.Store(0)
	c.counters.packetNanos.Store(0)
	c.counters.packetMaxNanos.Store(0)
	c.counters.damageTakenCalls.Store(0)
	c.counters.damageTakenNanos.Store(0)
	c.counters.damageTakenMaxNanos.Store(0)
	c.counters.coverageCalls.Store(0)
	c.counters.coverageNanos.Store(0)
	c.counters.coverageMaxNanos.Store(0)
}

func (a *App) stopAnalysisLog() {
	c := a.analysisLog
	if c == nil || !c.enabled.Swap(false) {
		return
	}
	c.mu.Lock()
	cancel := c.cancel
	done := c.done
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
	c.mu.Lock()
	if c.file != nil {
		_ = c.file.Close()
	}
	c.file = nil
	c.cancel = nil
	c.done = nil
	c.mu.Unlock()
}

func (a *App) runAnalysisLog(ctx context.Context, c *analysisLogController) {
	ticker := time.NewTicker(analysisLogSamplePeriod)
	defer ticker.Stop()
	defer close(c.done)
	for {
		select {
		case <-ctx.Done():
			a.writeAnalysisLog("SESSION_END reason=%q", ctx.Err())
			return
		case <-ticker.C:
			a.writeAnalysisSnapshot(c)
		}
	}
}

func swapAverage(total, count uint64) float64 {
	if count == 0 {
		return 0
	}
	return float64(total) / float64(count) / float64(time.Millisecond)
}

func (a *App) writeAnalysisSnapshot(c *analysisLogController) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	a.mu.RLock()
	connected := a.connected
	channel := a.channelName
	autoDetect := a.autoDetect
	manualNic := a.manualNic
	captureNic := a.captureNic
	backendRefreshMS := a.dpsRefreshSettings.BackendIntervalMS
	frontendRefreshMS := a.dpsRefreshSettings.FrontendIntervalMS
	entityCount := len(a.entities)
	damageCount := len(a.damages)
	targetCount := len(a.takenStats)
	statusCount := len(a.statusIntervals)
	activeStatusCount := len(a.activeStatusIntervals)
	a.mu.RUnlock()
	activeTimerCount := 0
	if a.buffTimerMgr != nil {
		activeTimerCount = a.buffTimerMgr.GetActiveTimers()
	}

	packetCount := c.counters.packetCount.Swap(0)
	conditionCount := c.counters.conditionPacketCount.Swap(0)
	packetNanos := c.counters.packetNanos.Swap(0)
	packetMax := c.counters.packetMaxNanos.Swap(0)
	damageCalls := c.counters.damageTakenCalls.Swap(0)
	damageNanos := c.counters.damageTakenNanos.Swap(0)
	damageMax := c.counters.damageTakenMaxNanos.Swap(0)
	coverageCalls := c.counters.coverageCalls.Swap(0)
	coverageNanos := c.counters.coverageNanos.Swap(0)
	coverageMax := c.counters.coverageMaxNanos.Swap(0)
	cpuTotalPercent := 0.0
	cpuOneCorePercent := 0.0
	if c.cpuAvailable {
		now := time.Now()
		if currentCPU, err := processCPUTime(); err == nil {
			wall := now.Sub(c.previousCPUAt)
			if wall > 0 {
				cpuOneCorePercent = float64(currentCPU-c.previousCPU) / float64(wall) * 100
				cpuTotalPercent = cpuOneCorePercent / float64(runtime.NumCPU())
			}
			c.previousCPU = currentCPU
			c.previousCPUAt = now
		} else {
			c.cpuAvailable = false
			a.writeAnalysisLog("PROCESS_CPU_UNAVAILABLE error=%q", err)
		}
	}

	a.writeAnalysisLog("RUNTIME connected=%t channel=%q auto_detect=%t manual_nic=%q capture_nic=%q backend_refresh_ms=%d frontend_refresh_ms=%d cpu_total_pct=%.2f cpu_one_core_pct=%.2f entities=%d damage_records=%d targets=%d status_intervals=%d active_status_intervals=%d active_buff_timers=%d goroutines=%d heap_mb=%.2f heap_objects=%d sys_mb=%.2f gc_count=%d gc_pause_total_ms=%.2f",
		connected, channel, autoDetect, manualNic, captureNic, backendRefreshMS, frontendRefreshMS, cpuTotalPercent, cpuOneCorePercent,
		entityCount, damageCount, targetCount, statusCount, activeStatusCount, activeTimerCount,
		runtime.NumGoroutine(), float64(mem.HeapAlloc)/(1024*1024), mem.HeapObjects,
		float64(mem.Sys)/(1024*1024), mem.NumGC, float64(mem.PauseTotalNs)/float64(time.Millisecond))
	a.writeAnalysisLog("INTERVAL packets=%d condition_packets=%d packet_avg_ms=%.3f packet_max_ms=%.3f damage_taken_calls=%d damage_taken_avg_ms=%.3f damage_taken_max_ms=%.3f coverage_calls=%d coverage_avg_ms=%.3f coverage_max_ms=%.3f",
		packetCount, conditionCount, swapAverage(packetNanos, packetCount), float64(packetMax)/float64(time.Millisecond),
		damageCalls, swapAverage(damageNanos, damageCalls), float64(damageMax)/float64(time.Millisecond),
		coverageCalls, swapAverage(coverageNanos, coverageCalls), float64(coverageMax)/float64(time.Millisecond))
}

func updateMaxUint64(target *atomic.Uint64, value uint64) {
	for {
		old := target.Load()
		if old >= value || target.CompareAndSwap(old, value) {
			return
		}
	}
}

func (a *App) recordPacketProcessing(op uint32, elapsed time.Duration) {
	if !a.analysisLoggingEnabled() {
		return
	}
	c := &a.analysisLog.counters
	c.packetCount.Add(1)
	if op == opcodeConditionUpdate {
		c.conditionPacketCount.Add(1)
	}
	nanos := uint64(elapsed)
	c.packetNanos.Add(nanos)
	updateMaxUint64(&c.packetMaxNanos, nanos)
}

func (a *App) recordDamageTaken(elapsed time.Duration) {
	if !a.analysisLoggingEnabled() {
		return
	}
	c := &a.analysisLog.counters
	c.damageTakenCalls.Add(1)
	nanos := uint64(elapsed)
	c.damageTakenNanos.Add(nanos)
	updateMaxUint64(&c.damageTakenMaxNanos, nanos)
}

func (a *App) recordBuffCoverage(elapsed time.Duration) {
	if !a.analysisLoggingEnabled() {
		return
	}
	c := &a.analysisLog.counters
	c.coverageCalls.Add(1)
	nanos := uint64(elapsed)
	c.coverageNanos.Add(nanos)
	updateMaxUint64(&c.coverageMaxNanos, nanos)
}
