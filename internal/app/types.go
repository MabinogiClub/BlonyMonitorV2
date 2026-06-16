package app

// 游戏数据包操作码
const (
	opcodeEntityAppear    = 0x520c
	opcodeEntitiesAppear  = 0x5334
	opcodeEntityProperty  = 0x7532
	opcodeEntityRemove    = 0x520d
	opcodeCombatAction    = 0x7926
	opcodeEffectDamage    = 0x9093
	opcodeEffectDelayed   = 0x9095 // 原文0x9094修改为0x9095兼容星辰与连击
	opcodeConditionUpdate = 0xa028
	opcodeSetFinisher     = 0x7921
	opcodeDungeonInfo     = 0x9470 // 地下城信息
	opcodeMapChange       = 0x6599 // 地图切换
	opcodeChineseName     = 0x526d // 中文名称包
	opcodeInstanceName    = 0x8ca0 // 副本名称包
)

func isRandomInstanceMapID(mapID int) bool {
	return mapID >= 35000 && mapID <= 35999
}

// PC种族ID集合
var pcRaceSet = map[int]bool{
	1:     true,
	2:     true,
	9001:  true,
	9002:  true,
	10001: true,
	10002: true,
	8001:  true,
	8002:  true,
}

// isPC 判断是否为PC种族
func isPC(raceId int) bool {
	return pcRaceSet[raceId]
}

// DamageRecord 伤害记录
type DamageRecord struct {
	Seq            int64   `json:"seq,omitempty"`
	AttackerID     string  `json:"attackerId"`
	AttackerName   string  `json:"attackerName"`
	TargetID       string  `json:"targetId"`
	TargetName     string  `json:"targetName"`
	SkillID        int     `json:"skillId"`
	Damage         float64 `json:"damage"`
	RawDamage      float64 `json:"rawDamage,omitempty"`
	OverflowDamage float64 `json:"overflowDamage,omitempty"`
	Adjusted       bool    `json:"adjusted,omitempty"`
	LockTriggered  bool    `json:"lockTriggered,omitempty"`
	LockThreshold  float64 `json:"lockThreshold,omitempty"`
	IsCritical     bool    `json:"isCritical"`
	At             int64   `json:"at"`
}

// EntityInfo 实体信息
type EntityInfo struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	RaceID          int      `json:"raceId"`
	IsPC            bool     `json:"isPC"`
	Conditions      []uint32 `json:"conditions"`      // 当前状态列表
	SkinColor       uint8    `json:"skinColor"`       // 肤色
	EyeType         uint16   `json:"eyeType"`         // 眼睛类型
	LeftEyeColor    uint8    `json:"leftEyeColor"`    // 左眼颜色
	RightEyeColor   uint8    `json:"rightEyeColor"`   // 右眼颜色
	MouthType       uint16   `json:"mouthType"`       // 嘴巴类型
	Height          float32  `json:"height"`          // 身高
	Weight          float32  `json:"weight"`          // 体重
	Upper           float32  `json:"upper"`           // 上身体型
	Lower           float32  `json:"lower"`           // 下身体型
	CombatPower     float32  `json:"combatPower"`     // 战斗力
	TitleID         uint32   `json:"titleId"`         // 主称号ID
	SubTitleID      uint32   `json:"subTitleId"`      // 副称号ID
	StyleTitleID    uint32   `json:"styleTitleId"`    // 风格主称号ID
	StyleSubTitleID uint32   `json:"styleSubTitleId"` // 风格副称号ID
	GuildName       string   `json:"guildName"`       // 公会名称
	OwnerID         uint64   `json:"ownerId"`         // 主人ID（宠物/傀儡）
	AddedAt         int64    `json:"-"`               // 添加时间戳（内部使用）
	// 生命值相关
	HP         float32 `json:"hp"`         // 当前生命值
	MaxHP      float32 `json:"maxHp"`      // 最大生命值
	MP         float32 `json:"mp"`         // 当前魔法值
	MaxMP      float32 `json:"maxMp"`      // 最大魔法值
	Stamina    float32 `json:"stamina"`    // 当前耐力值
	MaxStamina float32 `json:"maxStamina"` // 最大耐力值
}

// DamageStats 伤害统计 - 按攻击者
type DamageStats struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	TotalDamage float64 `json:"totalDamage"`
	DPS         float64 `json:"dps"`
	Percent     float64 `json:"percent"`
	HitCount    int     `json:"hitCount"`
	CritCount   int     `json:"critCount"`
	Status      string  `json:"status"` // 状态: active(战斗中), idle(空闲)
}

// SkillDamageStats 技能伤害统计
type SkillDamageStats struct {
	SkillID       int     `json:"skillId"`
	SkillName     string  `json:"skillName"`
	TotalDamage   float64 `json:"totalDamage"`
	Percent       float64 `json:"percent"`
	HitCount      int     `json:"hitCount"`
	CritCount     int     `json:"critCount"`
	AvgDamage     float64 `json:"avgDamage"`
	MinDamage     float64 `json:"minDamage"`
	MaxDamage     float64 `json:"maxDamage"`
	CritMinDamage float64 `json:"critMinDamage"`
	CritMaxDamage float64 `json:"critMaxDamage"`
}

// AttackerWithSkills 攻击者及其技能统计
type AttackerWithSkills struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	TotalDamage float64            `json:"totalDamage"`
	DPS         float64            `json:"dps"`
	Percent     float64            `json:"percent"`
	IsPC        bool               `json:"isPC,omitempty"`
	Skills      []SkillDamageStats `json:"skills"`
	Status      string             `json:"status"` // 状态: active(战斗中), idle(空闲)
}

// TargetDamageStats 受到伤害统计 - 按目标
type TargetDamageStats struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	TotalDamage float64              `json:"totalDamage"`
	DPS         float64              `json:"dps"`      // 每秒受到伤害
	Duration    int64                `json:"duration"` // 存活时间（秒）
	Attackers   []AttackerWithSkills `json:"attackers"`
	Status      string               `json:"status"` // 状态: active(战斗中), idle(空闲), dead(死亡)
}

// ChartDataPoint 图表数据点
type ChartDataPoint struct {
	Time   int64   `json:"time"`
	Damage float64 `json:"damage"`
}

// ChartSeries 图表数据系列
type ChartSeries struct {
	ID   string           `json:"id"`   // 攻击者ID，用于前端稳定映射颜色
	Name string           `json:"name"` // 攻击者名称
	Data []ChartDataPoint `json:"data"` // 图表数据点
}

// EventLog 事件日志
type EventLog struct {
	Seq            int64   `json:"seq,omitempty"`
	Type           string  `json:"type"`
	At             int64   `json:"at"`
	EntityID       string  `json:"entityId"`
	EntityName     string  `json:"entityName"`
	TargetID       string  `json:"targetId,omitempty"`
	TargetName     string  `json:"targetName,omitempty"`
	TargetRaceID   int     `json:"targetRaceId,omitempty"`
	TargetRaceName string  `json:"targetRaceName,omitempty"`
	TargetIsPC     bool    `json:"targetIsPC,omitempty"`
	SkillID        int     `json:"skillId,omitempty"`
	SkillName      string  `json:"skillName,omitempty"`
	Damage         float64 `json:"damage,omitempty"`
	RawDamage      float64 `json:"rawDamage,omitempty"`
	OverflowDamage float64 `json:"overflowDamage,omitempty"`
	Adjusted       bool    `json:"adjusted,omitempty"`
	LockTriggered  bool    `json:"lockTriggered,omitempty"`
	LockThreshold  float64 `json:"lockThreshold,omitempty"`
	IsCritical     bool    `json:"isCritical,omitempty"`
	RaceID         int     `json:"raceId,omitempty"`
	RaceName       string  `json:"raceName,omitempty"`
	IsPC           bool    `json:"isPC,omitempty"`
	ConditionID    uint32  `json:"conditionId,omitempty"`
	ConditionName  string  `json:"conditionName,omitempty"`
	IsEnable       bool    `json:"isEnable,omitempty"`
	AttackerID     string  `json:"attackerId,omitempty"`
	AttackerName   string  `json:"attackerName,omitempty"`
	CurrentHP      float64 `json:"currentHp,omitempty"`
	MaxHP          float64 `json:"maxHp,omitempty"`
	Percent        float64 `json:"percent,omitempty"`
	PrevHP         float64 `json:"prevHp,omitempty"`
	PrevPercent    float64 `json:"prevPercent,omitempty"`
	Threshold      float64 `json:"threshold,omitempty"`
	Locked         bool    `json:"locked,omitempty"`
	StallMs        int64   `json:"stallMs,omitempty"`
}

// DebugInfo 调试信息结构
type DebugInfo struct {
	SkillCount     int      `json:"skillCount"`
	RaceCount      int      `json:"raceCount"`
	ConditionCount int      `json:"conditionCount"`
	EntityCount    int      `json:"entityCount"`
	DamageCount    int      `json:"damageCount"`
	Region         string   `json:"region"`
	ResourceURL    string   `json:"resourceURL"`
	Connected      bool     `json:"connected"`
	StatusMsg      string   `json:"statusMsg"`
	SampleSkills   []string `json:"sampleSkills"`
	ChartDataLen   int      `json:"chartDataLen"`
	ParsedSkills   int      `json:"parsedSkills"`  // 解析到的技能数
	ParsedStrings  int      `json:"parsedStrings"` // 解析到的字符串数
	LoadError      string   `json:"loadError"`     // 加载错误
}

// PCEntityInfo PC实体信息（用于角色列表）
type PCEntityInfo struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	RaceID          int      `json:"raceId"`
	IsSelf          bool     `json:"isSelf"` // 是否为玩家自身
	Conditions      []uint32 `json:"conditions"`
	ConditionNames  []string `json:"conditionNames"`
	SkinColor       uint8    `json:"skinColor"`       // 肤色
	EyeType         uint16   `json:"eyeType"`         // 眼睛类型
	LeftEyeColor    uint8    `json:"leftEyeColor"`    // 左眼颜色
	RightEyeColor   uint8    `json:"rightEyeColor"`   // 右眼颜色
	MouthType       uint16   `json:"mouthType"`       // 嘴巴类型
	Height          float32  `json:"height"`          // 身高
	Weight          float32  `json:"weight"`          // 体重
	Upper           float32  `json:"upper"`           // 上身体型
	Lower           float32  `json:"lower"`           // 下身体型
	CombatPower     float32  `json:"combatPower"`     // 战斗力
	TitleID         uint32   `json:"titleId"`         // 主称号ID
	SubTitleID      uint32   `json:"subTitleId"`      // 副称号ID
	StyleTitleID    uint32   `json:"styleTitleId"`    // 风格主称号ID
	StyleSubTitleID uint32   `json:"styleSubTitleId"` // 风格副称号ID
	GuildName       string   `json:"guildName"`       // 公会名称
	OwnerID         uint64   `json:"ownerId"`         // 主人ID（宠物/傀儡）
	// 生命值相关
	HP         float32 `json:"hp"`         // 当前生命值
	MaxHP      float32 `json:"maxHp"`      // 最大生命值
	MP         float32 `json:"mp"`         // 当前魔法值
	MaxMP      float32 `json:"maxMp"`      // 最大魔法值
	Stamina    float32 `json:"stamina"`    // 当前耐力值
	MaxStamina float32 `json:"maxStamina"` // 最大耐力值
}

// SelfInfo 玩家自身信息
type SelfInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CreatureInfo 生物信息（用于生物库）
type CreatureInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	RaceID     int    `json:"raceId"`
	RaceName   string `json:"raceName"`
	IsPC       bool   `json:"isPC"`
	IsAlive    bool   `json:"isAlive"`
	AppearedAt int64  `json:"appearedAt"`
}

// attackerAggStats 攻击者聚合统计
type attackerAggStats struct {
	total    float64
	hits     int
	crits    int
	firstHit int64
	lastHit  int64
}

// skillAggStats 技能聚合统计
type skillAggStats struct {
	total   float64
	hits    int
	crits   int
	min     float64
	max     float64
	critMin float64
	critMax float64
}

// takenSkillAggStats 受到伤害技能聚合统计
type SkillHitRecord struct {
	Seq            int64   `json:"seq,omitempty"`
	Damage         float64 `json:"damage"`
	RawDamage      float64 `json:"rawDamage,omitempty"`
	OverflowDamage float64 `json:"overflowDamage,omitempty"`
	Adjusted       bool    `json:"adjusted,omitempty"`
	LockTriggered  bool    `json:"lockTriggered,omitempty"`
	LockThreshold  float64 `json:"lockThreshold,omitempty"`
	IsCritical     bool    `json:"isCritical"`
	Timestamp      int64   `json:"timestamp"`
}

type takenSkillAggStats struct {
	total   float64
	hits    int
	crits   int
	min     float64
	max     float64
	critMin float64
	critMax float64
	records []SkillHitRecord
}

// takenAggStats 受到伤害聚合统计（按目标-攻击者分组）
type takenAggStats struct {
	total    float64
	hits     int
	crits    int
	firstHit int64                       // 该攻击者首次攻击该目标的时间
	lastHit  int64                       // 该攻击者最后攻击该目标的时间
	skills   map[int]*takenSkillAggStats // skillId -> 技能统计
	name     string                      // 攻击者名字（缓存）
	raceId   int                         // 攻击者种族ID（用于从数据库查询名字）
	isPC     bool                        // 攻击者是否为PC
}

// targetAggStats 目标受到伤害聚合统计
type targetAggStats struct {
	total     float64
	attackers map[string]*takenAggStats // attackerId -> 统计
	firstHit  int64                     // 首次受击时间
	lastHit   int64                     // 最后受击时间
	deathTime int64                     // 死亡时间 (0表示未死亡)
	name      string                    // 目标名字（缓存）
	raceId    int                       // 目标种族ID（用于从数据库查询名字）
	isPC      bool                      // 目标是否为PC
}

// 图表数据聚合常量
const (
	ChartBucketSeconds   = 30                                   // 每30秒聚合一个数据点（更平滑的曲线）
	ChartMaxBuckets      = 240                                  // 保留240个数据点（2小时）
	ChartMaxDurationSecs = ChartBucketSeconds * ChartMaxBuckets // 7200秒 = 2小时
)

// chartBucketData 图表时间桶数据
type chartBucketData struct {
	time   int64   // 时间桶时间戳（已对齐到5秒）
	damage float64 // 该时间段内的总伤害
}

// chartAttackerData 单个攻击者的图表数据
type chartAttackerData struct {
	buckets map[int64]float64 // time_bucket -> total_damage
	times   []int64           // 有序的时间桶列表（用于快速清理和遍历）
}

// DungeonInfo 地下城信息
type DungeonInfo struct {
	InstanceID  uint64  `json:"instanceId"`  // 地下城实例ID
	DungeonName string  `json:"dungeonName"` // 地下城内部名称
	DungeonID   uint32  `json:"dungeonId"`   // 地下城变体ID
	Seed        uint32  `json:"seed"`        // 随机种子
	Difficulty  uint32  `json:"difficulty"`  // 难度等级
	FloorCount  uint32  `json:"floorCount"`  // 总层数
	FloorLayout []uint8 `json:"floorLayout"` // 层布局数据
	EnteredAt   int64   `json:"enteredAt"`   // 进入时间
	CompletedAt int64   `json:"completedAt"` // 完成时间 (0表示未完成)
	IsCompleted bool    `json:"isCompleted"` // 是否已完成
}

// InstanceInfo 副本信息
type InstanceInfo struct {
	InstanceID   uint64 `json:"instanceId"`
	InstanceName string `json:"instanceName"`
	MapID        uint32 `json:"mapId"`
	EnteredAt    int64  `json:"enteredAt"`
}

// CurrentMapInfo 当前地图信息
type CurrentMapInfo struct {
	MapID     int    `json:"mapId"`     // 地图ID
	MapName   string `json:"mapName"`   // 地图名称（区）
	LocalName string `json:"localName"` // 本地化名称（城）
}

// PlayerTimeline 玩家时间轴
type PlayerTimeline struct {
	PlayerID   string     `json:"playerId"`   // 玩家ID
	PlayerName string     `json:"playerName"` // 玩家名称
	StartTime  int64      `json:"startTime"`  // 首次出现时间
	EndTime    int64      `json:"endTime"`    // 最后活动时间
	Events     []EventLog `json:"events"`     // 事件列表
}

// BossHPInfo Boss ???????
type BossHPInfo struct {
	EntityID  string  `json:"entityId"`
	Name      string  `json:"name"`
	RaceID    int     `json:"raceId"`
	CurrentHP float64 `json:"currentHp"`
	MaxHP     float64 `json:"maxHp"`
	Percent   float64 `json:"percent"`
	UpdatedAt int64   `json:"updatedAt"`
	DamageSeq int64   `json:"damageSeq,omitempty"`
}

// BossHPRecord Boss ?????
type BossHPRecord struct {
	EntityID    string  `json:"entityId"`
	RaceID      int     `json:"raceId"`
	CurrentHP   float64 `json:"currentHp"`
	MaxHP       float64 `json:"maxHp"`
	Percent     float64 `json:"percent"`
	HpTimestamp int64   `json:"hptimestamp"`
	DamageSeq   int64   `json:"damageSeq,omitempty"`
	Threshold   float64 `json:"threshold,omitempty"`
	Locked      bool    `json:"locked,omitempty"`
}

// BossHPWatchState Boss ???????
type BossHPWatchState struct {
	Emitted            map[float64]bool
	ActiveThreshold    float64
	CandidateThreshold float64
	CandidateAt        int64
}

// BossHPPendingDamageWindow tracks an HP packet that arrived before its
// matching damage packets, so the later damage can still be aligned to HP.
type BossHPPendingDamageWindow struct {
	FromSeq           int64
	HPDelta           float64
	LockThreshold     float64
	MarkLockTrigger   bool
	MaxHP             float64
	CurrentHP         float64
	CurrentPercent    float64
	PrevHP            float64
	PrevPercent       float64
	Timestamp         int64
	LastAttemptSeq    int64
	LastAttemptDamage float64
}
