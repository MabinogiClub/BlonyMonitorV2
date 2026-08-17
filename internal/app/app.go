package app

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"blonymonitorv2/db"
	"blonymonitorv2/internal/config"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var logger *log.Logger

func init() {
	if !config.EnableFileLog {
		logger = log.New(io.Discard, "", 0)
		return
	}

	exePath, err := os.Executable()
	if err != nil {
		logger = log.New(os.Stdout, "overlay ", log.LstdFlags|log.Lshortfile)
		return
	}
	logPath := filepath.Join(filepath.Dir(exePath), "overlay.log")

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		logger = log.New(os.Stdout, "overlay ", log.LstdFlags|log.Lshortfile)
		return
	}

	logger = log.New(logFile, "overlay ", log.LstdFlags|log.Lshortfile)
}

var (
	lastLoadError     string
	parsedSkillCount  int
	parsedStringCount int
)

// App ????
type App struct {
	ctx                         context.Context
	cancel                      context.CancelFunc
	mu                          sync.RWMutex
	entities                    map[string]*EntityInfo
	creatureLib                 map[string]string
	damages                     []DamageRecord
	eventLogs                   []EventLog
	connected                   bool
	statusMsg                   string
	region                      string
	resourceURL                 string
	channelName                 string
	serverIP                    string
	serverPort                  uint16
	attackerStats               map[string]*attackerAggStats
	skillStats                  map[string]map[int]*skillAggStats
	cumulativeAttackerStats     map[string]*cumulativeAttackerAggStats
	totalDamage                 float64
	takenStats                  map[string]*targetAggStats
	targetDamages               map[string][]DamageRecord
	damageSeq                   int64
	damageSeqAtLastAutoSave     int64
	detailResetPending          bool
	bossHP                      map[string]*BossHPInfo
	bossHPHistory               map[string][]BossHPRecord
	bossHPWatch                 map[string]*BossHPWatchState
	bossHPPending               map[string]*BossHPPendingDamageWindow
	chartAggData                map[string]*chartAttackerData
	targetChartAggData          map[string]map[string]*chartAttackerData
	attackerTimerMgr            *AttackerTimerManager
	targetTimerMgr              *TargetTimerManager
	dpsUpdateThrottler          *DPSUpdateThrottler
	dpsRefreshSettings          DPSRefreshSettings
	exportDamageMu              sync.Mutex
	exportDamageCache           map[int64]float64
	exportDamageDirty           bool
	exportDamageBuiltAt         time.Time
	autoDetect                  bool
	selectedChannel             int
	captureCancel               context.CancelFunc
	manualNic                   string
	captureNic                  string
	clickThrough                bool
	opacity                     int
	soundVolume                 int
	alwaysOnTop                 bool
	currentDungeon              *DungeonInfo
	dungeonSaveName             string
	dungeonChineseNameReceived  bool
	dungeonLocalName            string
	recentDungeonNameCandidates []dungeonNameCandidate
	currentInstance             *InstanceInfo
	instanceEnterMapID          int
	instanceSaveName            string
	instanceNameReceived        bool
	instanceNameWaitMapID       int
	instanceNameWaitUntil       int64
	currentMap                  *CurrentMapInfo
	lastMapChangeAt             int64
	selfId                      string
	selfName                    string
	rankingConsentRequestMu     sync.Mutex
	rankingConsentSyncKey       string
	buffTimerMgr                *BuffTimerManager
	farmMgr                     *FarmManager
	farmSoundPlayer             *farmSoundPlayer
	farmRecommendationMgr       *farmRecommendationManager
	statusIntervals             []*statusInterval
	activeStatusIntervals       map[statusIntervalKey]*statusInterval
	musicPerformances           map[uint64]*musicPerformance
	battlefieldShockStates      map[string]*battlefieldShockState
	battlefieldShockGeneration  uint64
	analysisLog                 *analysisLogController
	onHide                      func()
}

// NewApp ?????
func NewApp() *App {
	return &App{
		entities:                make(map[string]*EntityInfo),
		creatureLib:             make(map[string]string),
		damages:                 make([]DamageRecord, 0),
		eventLogs:               make([]EventLog, 0),
		statusMsg:               "????...",
		region:                  "cn",
		attackerStats:           make(map[string]*attackerAggStats),
		skillStats:              make(map[string]map[int]*skillAggStats),
		cumulativeAttackerStats: make(map[string]*cumulativeAttackerAggStats),
		takenStats:              make(map[string]*targetAggStats),
		targetDamages:           make(map[string][]DamageRecord),
		bossHP:                  make(map[string]*BossHPInfo),
		bossHPHistory:           make(map[string][]BossHPRecord),
		bossHPWatch:             make(map[string]*BossHPWatchState),
		bossHPPending:           make(map[string]*BossHPPendingDamageWindow),
		chartAggData:            make(map[string]*chartAttackerData),
		targetChartAggData:      make(map[string]map[string]*chartAttackerData),
		statusIntervals:         make([]*statusInterval, 0),
		activeStatusIntervals:   make(map[statusIntervalKey]*statusInterval),
		musicPerformances:       make(map[uint64]*musicPerformance),
		battlefieldShockStates:  make(map[string]*battlefieldShockState),
		analysisLog:             &analysisLogController{},
		dpsRefreshSettings:      defaultDPSRefreshSettings(),
		autoDetect:              true,
		opacity:                 100,
		soundVolume:             defaultSoundVolume,
	}
}

// Startup ???????
func (a *App) Startup(ctx context.Context) {
	a.ctx, a.cancel = context.WithCancel(ctx)
	if loadAnalysisLogEnabled() {
		if err := a.startAnalysisLog(); err != nil {
			logger.Printf("[AnalysisLog] 启动失败: %v", err)
		}
	}

	a.dpsRefreshSettings = loadDPSRefreshSettings()
	a.soundVolume = loadSoundVolume()
	a.dpsUpdateThrottler = NewDPSUpdateThrottler(a, time.Duration(a.dpsRefreshSettings.BackendIntervalMS)*time.Millisecond)
	a.attackerTimerMgr = NewAttackerTimerManager(a)
	a.targetTimerMgr = NewTargetTimerManager(a)
	a.buffTimerMgr = NewBuffTimerManagerWithVolume(a.ctx, "", a.GetSoundVolume)
	a.farmSoundPlayer = newFarmSoundPlayer(a.ctx, a.GetSoundVolume)
	a.farmMgr = NewFarmManager(
		a.ctx,
		func(state FarmState) {
			runtime.EventsEmit(a.ctx, "farm-state", state)
		},
		func(plot FarmPlotState) {
			a.farmSoundPlayer.NotifyReady()
			runtime.EventsEmit(a.ctx, "farm-ready", plot)
		},
		func(plot FarmPlotState) {
			a.farmSoundPlayer.NotifySpecial()
			runtime.EventsEmit(a.ctx, "farm-special", plot)
		},
	)
	a.farmRecommendationMgr = newFarmRecommendationManager()

	db.InitDB()

	runtime.WindowSetAlwaysOnTop(ctx, a.alwaysOnTop)

	if !a.reportNpcapMissingIfNeeded() {
		go a.startCapture()
	}
	go startClickThroughMonitor(a.ctx, a)
}

// Shutdown ???????
func (a *App) Shutdown(ctx context.Context) {
	a.shutdownSaveData()
	a.stopAnalysisLog()
	if a.farmRecommendationMgr != nil {
		a.farmRecommendationMgr.close()
	}
	if a.cancel != nil {
		a.cancel()
	}
}

// Clear 清空数据（不保存）
func (a *App) Clear() {
	a.mu.Lock()
	a.eventLogs = make([]EventLog, 0)
	a.clearDamageStateUnsafe()
	a.mu.Unlock()

	runtime.EventsEmit(a.ctx, "clear")
}

// IsConnected ?????
func (a *App) IsConnected() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.connected
}

// GetStatus ??????
func (a *App) GetStatus() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.statusMsg
}

func (a *App) setStatus(msg string) {
	a.mu.Lock()
	a.statusMsg = msg
	a.mu.Unlock()
	runtime.EventsEmit(a.ctx, "status", msg)
}

func (a *App) setConnected(connected bool) {
	a.mu.Lock()
	a.connected = connected
	a.mu.Unlock()
	runtime.EventsEmit(a.ctx, "connected", connected)
}

// LogFromFrontend ??????
func (a *App) LogFromFrontend(message string) {
	logger.Printf("[Frontend] %s\n", message)
}

// SetOnHide ????????
func (a *App) SetOnHide(callback func()) {
	a.onHide = callback
}

// Hide ????????????
func (a *App) Hide() {
	if a.onHide != nil {
		a.onHide()
	}
}

// GetDebugInfo ??????
func (a *App) GetDebugInfo() DebugInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()

	sampleSkills := make([]string, 0)
	count := 0
	skillNameMap := db.GetAllSkills()
	raceNameMap := db.GetAllRace()
	conditionNameMap := db.GetAllCondition()
	for id, name := range skillNameMap {
		if count >= 5 {
			break
		}
		sampleSkills = append(sampleSkills, strconv.Itoa(id)+":"+name)
		count++
	}

	chartDataLen := 0
	for _, d := range a.damages {
		entity := a.entities[d.AttackerID]
		if entity != nil && entity.IsPC {
			chartDataLen++
		}
	}

	announcementDebug, rankingDebug := getServerInteractionDebug()
	return DebugInfo{
		SkillCount:     len(skillNameMap),
		RaceCount:      len(raceNameMap),
		ConditionCount: len(conditionNameMap),
		EntityCount:    len(a.entities),
		DamageCount:    len(a.damages),
		Region:         a.region,
		ResourceURL:    a.resourceURL,
		Connected:      a.connected,
		StatusMsg:      a.statusMsg,
		SampleSkills:   sampleSkills,
		ChartDataLen:   chartDataLen,
		ParsedSkills:   parsedSkillCount,
		ParsedStrings:  parsedStringCount,
		LoadError:      lastLoadError,
		Upload:         getUploadDebugInfo(),
		Announcement:   announcementDebug,
		Ranking:        rankingDebug,
	}
}

// GetCurrentMap ????????
func (a *App) GetCurrentMap() *CurrentMapInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.currentMap
}

func (a *App) setCurrentMap(mapInfo *CurrentMapInfo) {
	a.mu.Lock()
	a.currentMap = mapInfo
	a.mu.Unlock()
	runtime.EventsEmit(a.ctx, "mapChange", mapInfo)
}

// GetSelfInfo ????????
func (a *App) GetSelfInfo() *SelfInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.selfId == "" {
		return nil
	}
	return &SelfInfo{
		ID:   a.selfId,
		Name: a.selfName,
	}
}

func (a *App) setSelfInfo(id, name string) {
	a.mu.Lock()
	a.selfId = id
	a.selfName = name
	a.mu.Unlock()

	if a.buffTimerMgr != nil {
		a.buffTimerMgr.SetSelfId(id)
	}

	runtime.EventsEmit(a.ctx, "selfInfo", &SelfInfo{ID: id, Name: name})
	a.scheduleRankingConsentSync()
}

func (a *App) scheduleRankingConsentSync() {
	if !rankingConsentConfigured() {
		return
	}
	playerID, _ := a.currentPlayerIdentity()
	serverID := a.currentServerID()
	if playerID == "" || serverID == "" {
		return
	}
	key := playerID + "|" + serverID
	a.mu.Lock()
	if a.rankingConsentSyncKey == key {
		a.mu.Unlock()
		return
	}
	a.rankingConsentSyncKey = key
	a.mu.Unlock()
	go func(syncKey string) {
		if _, err := a.syncRankingParticipation(); err != nil {
			logger.Printf("[Ranking] 角色识别后同步失败: %v", err)
			a.mu.Lock()
			if a.rankingConsentSyncKey == syncKey {
				a.rankingConsentSyncKey = ""
			}
			a.mu.Unlock()
		}
	}(key)
}

// GetActiveBuffTimers 获取当前活跃的buff定时器
func (a *App) GetActiveBuffTimers() []ActiveBuffTimerInfo {
	if a.buffTimerMgr == nil {
		return []ActiveBuffTimerInfo{}
	}
	return a.buffTimerMgr.GetActiveTimersInfo()
}

// GetMonitoredBuffs 获取所有监控的buff列表
func (a *App) GetMonitoredBuffs() []BuffInfo {
	if a.buffTimerMgr == nil {
		return []BuffInfo{}
	}
	return a.buffTimerMgr.GetMonitoredBuffs()
}

// CancelBuffTimer 取消指定的buff定时器
func (a *App) CancelBuffTimer(entityId uint64, ccId uint32) {
	if a.buffTimerMgr == nil {
		return
	}
	a.buffTimerMgr.StopTimer(entityId, ccId)
}

// SetBuffNotifyThreshold 设置buff语音提醒阈值（秒）
func (a *App) SetBuffNotifyThreshold(seconds int64) {
	if a.buffTimerMgr == nil {
		return
	}
	a.buffTimerMgr.SetNotifyThreshold(seconds)
}

// GetBuffDisplayList 获取所有监控buff的展示信息
func (a *App) GetBuffDisplayList() []BuffDisplayInfo {
	if a.buffTimerMgr == nil {
		return []BuffDisplayInfo{}
	}
	return a.buffTimerMgr.GetBuffDisplayList()
}

// SetBuffSoundEnabled 设置单个buff的声音开关
func (a *App) SetBuffSoundEnabled(ccId uint32, enabled bool) {
	if a.buffTimerMgr == nil {
		return
	}
	a.buffTimerMgr.SetBuffSoundEnabled(ccId, enabled)
}

// SetBuffOrder 设置buff显示顺序
func (a *App) SetBuffOrder(order []uint32) {
	if a.buffTimerMgr == nil {
		return
	}
	if err := a.buffTimerMgr.SetBuffOrder(order); err != nil {
		logger.Printf("[BuffTimer] 设置排序失败: %v", err)
	}
}

// GetBuffOrder 获取buff显示顺序
func (a *App) GetBuffOrder() []uint32 {
	if a.buffTimerMgr == nil {
		return nil
	}
	return a.buffTimerMgr.GetBuffOrder()
}

// GetBuffNotifyThreshold 获取buff语音提醒阈值（秒）
func (a *App) GetBuffNotifyThreshold() int64 {
	if a.buffTimerMgr == nil {
		return 30
	}
	return a.buffTimerMgr.GetNotifyThreshold()
}

// GetFarmState returns the latest server-authoritative farm state.
func (a *App) GetFarmState() FarmState {
	if a.farmMgr == nil {
		plots := make([]FarmPlotState, len(farmSlotDefinitions))
		for index, definition := range farmSlotDefinitions {
			plots[index] = FarmPlotState{Index: index, Kind: definition.Kind, Label: definition.Label, Unlocked: true, Quality: "empty"}
		}
		return FarmState{FertilityMax: farmResourceMaximum, EnergyMax: farmResourceMaximum, Plots: plots}
	}
	return a.farmMgr.State(time.Now())
}

// SetFarmMonitorEnabled controls farm reminders. Packet state continues to be synchronized while disabled.
func (a *App) SetFarmMonitorEnabled(enabled bool) FarmState {
	if a.farmMgr == nil {
		return a.GetFarmState()
	}
	a.farmMgr.SetEnabled(enabled)
	return a.farmMgr.State(time.Now())
}

// SetFarmReadyNotificationEnabled controls mature crop notifications.
func (a *App) SetFarmReadyNotificationEnabled(enabled bool) FarmState {
	if a.farmMgr == nil {
		return a.GetFarmState()
	}
	a.farmMgr.SetReadyNotificationEnabled(enabled)
	return a.farmMgr.State(time.Now())
}

// SetFarmSpecialNotificationEnabled controls special seed notifications.
func (a *App) SetFarmSpecialNotificationEnabled(enabled bool) FarmState {
	if a.farmMgr == nil {
		return a.GetFarmState()
	}
	a.farmMgr.SetSpecialNotificationEnabled(enabled)
	return a.farmMgr.State(time.Now())
}

// GetFarmRecommendationState returns locally synchronized delivery recommendations.
func (a *App) GetFarmRecommendationState() FarmRecommendationState {
	if a.farmRecommendationMgr == nil {
		return FarmRecommendationState{Goal: farmGoalKeys, StatusMessage: "推荐服务尚未初始化", Recommendations: []FarmOrderRecommendation{}}
	}
	return a.farmRecommendationMgr.state(a.GetFarmState())
}

// SetFarmRecommendationGoal switches between key and coin efficiency.
func (a *App) SetFarmRecommendationGoal(goal string) FarmRecommendationState {
	if a.farmRecommendationMgr == nil {
		return a.GetFarmRecommendationState()
	}
	if err := a.farmRecommendationMgr.setGoal(goal); err != nil {
		logger.Printf("[FarmRecommendation] 保存目标失败: %v", err)
	}
	return a.GetFarmRecommendationState()
}

// SetFarmRecommendationMaterialQualities controls which crop quality tiers may be consumed.
func (a *App) SetFarmRecommendationMaterialQualities(normal, advanced, highest bool) FarmRecommendationState {
	if a.farmRecommendationMgr == nil {
		return a.GetFarmRecommendationState()
	}
	if err := a.farmRecommendationMgr.setMaterialQualities(normal, advanced, highest); err != nil {
		logger.Printf("[FarmRecommendation] 保存材料品质偏好失败: %v", err)
	}
	return a.GetFarmRecommendationState()
}

// SetFarmRecommendationCaptureEnabled records a short local-only packet sample.
func (a *App) SetFarmRecommendationCaptureEnabled(enabled bool) FarmRecommendationState {
	if a.farmRecommendationMgr == nil {
		return a.GetFarmRecommendationState()
	}
	if err := a.farmRecommendationMgr.setCaptureEnabled(enabled); err != nil {
		logger.Printf("[FarmRecommendation] 切换本地采集失败: %v", err)
	}
	return a.GetFarmRecommendationState()
}
