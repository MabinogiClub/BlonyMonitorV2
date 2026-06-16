/**
 * 全局类型声明
 * 定义 Wails 运行时和 Go 后端 API 类型
 */

/**
 * Wails 运行时类型声明
 */
interface WailsRuntime {
  EventsOn(eventName: string, callback: (...args: any[]) => void): void
  EventsOff(eventName: string): void
  Quit(): void
}

/**
 * Go 后端 API 类型声明
 */
interface GoApp {
  IsConnected(): Promise<boolean>
  GetChannelName(): Promise<string>
  GetAllChannels(): Promise<string[]>
  GetChannelConfig(): Promise<ChannelConfig>
  GetAutoDetect(): Promise<boolean>
  GetSelectedChannel(): Promise<number>
  SetAutoDetect(auto: boolean): Promise<void>
  SetChannel(channel: number): Promise<void>
  GetAcceleratorMode(): Promise<boolean>
  SetAcceleratorMode(enabled: boolean): Promise<void>
  GetAllSkillNames(): Promise<Record<number, string>>
  GetAllConditionNames(): Promise<Record<number, string>>
  GetAllConditionIcons(): Promise<Record<number, string>>
  GetAllSkillIcons(): Promise<Record<number, string>>
  GetResourceURL(): Promise<string>
  GetRegion(): Promise<string>
  GetDebugInfo(): Promise<DebugInfo>
  ReloadResourceData(): Promise<void>
  GetDamageBySkill(): Promise<AttackerStats[]>
  GetDamageByAttacker(): Promise<any[]>
  GetDamageTaken(): Promise<TakenStats[]>
  GetCurrentBattleReport(): Promise<BattleReportPayload>
  GetChartData(): Promise<ChartSeries[]>
  GetChartDataForTarget(targetId: string): Promise<ChartSeries[]>
  GetEventLogs(limit: number, filter: string): Promise<EventLog[]>
  GetAllPCEntities(): Promise<EntityInfo[]>
  GetAllCreatures(): Promise<CreatureInfo[]>
  Clear(): Promise<void>
  ClearAndSave(): Promise<void>
  GetCleanedTargetsList(): Promise<string[]>
  ReadCleanedTargetFileFull(fileName: string): Promise<HistoryFileData | HistoryTarget[] | { error: string }>
  GetSaveDir(): Promise<string>
  SetClickThrough(enabled: boolean): Promise<void>
  GetClickThrough(): Promise<boolean>
  SetOpacity(opacity: number): Promise<void>
  GetOpacity(): Promise<number>
  SetAlwaysOnTop(enabled: boolean): Promise<void>
  GetAlwaysOnTop(): Promise<boolean>
  // 窗口大小
  SetWindowSize(width: number, height: number): Promise<void>
  GetWindowSize(): Promise<{ width: number; height: number }>
  SetWindowMinSize(width: number, height: number): Promise<void>
  SetWindowMaxSize(width: number, height: number): Promise<void>
  SetWindowResizable(resizable: boolean): Promise<void>
  // 窗口隐藏
  Hide(): Promise<void>
  // 地图 API
  GetCurrentMap(): Promise<CurrentMapInfo | null>
  GetCurrentDungeon(): Promise<DungeonInfo | null>
  // 玩家自身 API
  GetSelfInfo(): Promise<SelfInfo | null>
  // 背包 API
  GetInventories(): Promise<Record<string, InventoryInfo>>
  GetInventory(characterName: string): Promise<InventoryInfo | null>
  ClearInventories(): Promise<void>
  // 背包数据库 API
  GetInventoriesFromDB(): Promise<any[]>
  GetInventoriesByCharacterFromDB(characterName: string): Promise<any[]>
  GetInventoriesByTypeFromDB(backpackType: string): Promise<any[]>
  GetInventoryFromDB(characterName: string, backpackType: string, backpackIndex: number): Promise<any>
  DeleteInventoryFromDB(characterName: string, backpackType: string, backpackIndex: number): Promise<void>
  DeleteCharacterInventoriesFromDB(characterName: string): Promise<void>
  // 时间轴 API
  GetPlayerTimeline(playerId: string): Promise<PlayerTimeline>
  // Buff 定时器 API
  GetBuffDisplayList(): Promise<BuffDisplayInfo[]>
  GetActiveBuffTimers(): Promise<ActiveBuffTimerInfo[]>
  GetMonitoredBuffs(): Promise<BuffInfo[]>
  CancelBuffTimer(entityId: number, ccId: number): Promise<void>
  SetBuffSoundEnabled(ccId: number, enabled: boolean): Promise<void>
  SetBuffOrder(order: number[]): Promise<void>
  SetBuffNotifyThreshold(seconds: number): Promise<void>
  GetBuffNotifyThreshold(): Promise<number>
  GetNpcapStatus(): Promise<NpcapStatus>
  OpenNpcapDownloadPage(): Promise<void>
  RecheckNpcap(): Promise<NpcapStatus>
}

/**
 * 频道配置类型
 */
interface ChannelConfig {
  servers?: Server[]
  Servers?: Server[]
}

interface Server {
  name?: string
  Name?: string
  channels?: Channel[]
  Channels?: Channel[]
}

interface Channel {
  id?: number
  ID?: number
  name?: string
  Name?: string
}

/**
 * Npcap 状态类型
 */
interface NpcapStatus {
  installed: boolean
  message: string
}

/**
 * 调试信息类型
 */
interface DebugInfo {
  connected: boolean
  skillCount: number
  parsedSkills: number
  parsedStrings: number
  raceCount: number
  conditionCount: number
  entityCount: number
  damageCount: number
  chartDataLen: number
  region: string
  resourceURL: string
  sampleSkills?: string[]
  loadError?: string
}

/**
 * 攻击者统计类型
 */
interface AttackerStats {
  id: string
  name: string
  totalDamage: number
  dps: number
  percent: number
  skills: SkillStats[]
  status?: string  // 状态: active(战斗中), idle(空闲)
}

interface SkillStats {
  skillId: number
  skillName?: string
  totalDamage: number
  hitCount: number
  critCount: number
  avgDamage: number
  minDamage: number
  maxDamage: number
  critMinDamage?: number
  critMaxDamage?: number
  percent: number
}

/**
 * 受到伤害统计类型
 */
interface TakenStats {
  id: string
  name: string
  totalDamage: number
  dps: number        // 每秒受到伤害
  duration: number   // 存活时间（秒）
  attackers: AttackerDetail[]
  status?: string    // 状态: active(战斗中), idle(空闲), dead(死亡)
}

interface SkillHitRecord {
  seq?: number
  damage: number
  rawDamage?: number
  overflowDamage?: number
  adjusted?: boolean
  lockTriggered?: boolean
  lockThreshold?: number
  isCritical: boolean
  timestamp: number
}

interface HistorySkillDetail extends SkillStats {
  hitRecords?: SkillHitRecord[]
}

interface HistoryAttacker extends AttackerDetail {
  isPC?: boolean
  skillsDetail?: HistorySkillDetail[]
  appearedAt?: number
  lastHit?: number
}

interface BossHPHistoryItem {
  entityId: string
  history: BossHPRecord[]
}

interface HistoryTarget extends Omit<TakenStats, 'attackers'> {
  targetId?: string
  targetName?: string
  cleanedAt?: number
  appearedAt?: number
  deathTime?: number
  bossHP?: BossHPExport
  attackers: HistoryAttacker[]
}

interface HistoryFileData {
  targets: HistoryTarget[]
}

interface AttackerDetail {
  id: string
  name: string
  totalDamage: number
  dps: number        // 该攻击者对该目标的DPS
  percent: number
  skills?: SkillStats[]
  status?: string    // 状态: active(战斗中), idle(空闲)
}

/**
 * 图表数据类型
 */
interface ChartSeries {
  id: string
  name: string
  data: ChartPoint[]
}

interface ChartPoint {
  time: number
  damage: number
}

/**
 * 事件日志类型
 */
interface EventLog {
  type: string
  at: number
  entityId?: string
  entityName?: string
  targetId?: string
  targetName?: string
  targetRaceId?: number
  targetRaceName?: string
  targetIsPC?: boolean
  skillId?: number
  skillName?: string
  damage?: number
  isCritical?: boolean
  isPC?: boolean
  raceName?: string
  raceId?: number
  conditionName?: string
  conditionId?: number
  isEnable?: boolean
  attackerId?: string
  attackerName?: string
}

/**
 * 实体信息类型
 */
interface EntityInfo {
  id: string
  name: string
  raceId: number
  isSelf?: boolean          // 是否为玩家自身
  conditions?: number[]
  conditionNames?: string[]
  skinColor?: number        // 肤色
  eyeType?: number          // 眼睛类型
  leftEyeColor?: number     // 左眼颜色
  rightEyeColor?: number    // 右眼颜色
  mouthType?: number        // 嘴巴类型
  height?: number           // 身高
  weight?: number           // 体重
  upper?: number            // 上身体型
  lower?: number            // 下身体型
  combatPower?: number      // 战斗力
  titleId?: number          // 主称号ID
  subTitleId?: number       // 副称号ID
  styleTitleId?: number     // 风格主称号ID
  styleSubTitleId?: number  // 风格副称号ID
  guildName?: string        // 公会名称
  ownerId?: number          // 主人ID（宠物/傀儡）
  hp?: number
  maxHp?: number
  mp?: number
  maxMp?: number
  stamina?: number
  maxStamina?: number
}

/**
 * 玩家自身信息类型
 */
interface SelfInfo {
  id: string
  name: string
}

/**
 * 生物信息类型
 */
interface CreatureInfo {
  id: string
  name: string
  raceId: number
  raceName?: string
  isPC: boolean
  isAlive: boolean
}

/**
 * 当前地下城信息类型
 */
interface DungeonInfo {
  instanceId: number
  dungeonName: string
  dungeonId: number
  seed: number
  difficulty: number
  floorCount: number
  floorLayout: number[]
  enteredAt: number
  completedAt: number
  isCompleted: boolean
}

/**
 * 当前地图信息类型
 */
interface CurrentMapInfo {
  mapId: number
  mapName: string    // 区
  localName: string  // 城
}

/**
 * 玩家自身信息类型
 */
interface SelfInfo {
  id: string
  name: string
}

/**
 * 背包信息类型
 */
interface InventoryInfo {
  characterName: string
  backpacks: BackpackInfo[]
  updatedAt: number
}

/**
 * 单个背包信息类型
 */
interface BackpackInfo {
  width: number
  height: number
  itemCount: number
  items: InventoryItem[]
}

/**
 * 背包物品类型
 */
interface InventoryItem {
  itemId: number
  itemName: string
  quantity: number
  x: number
  y: number
  width: number
  height: number
  colorR: number
  colorG: number
  colorB: number
  durability: number
  reinforce: number
  refine: number
  rotation: number
  identified: boolean
  cursed: boolean
  sealed: boolean
  enchantLvl: number
  elemental: number
  effects?: Record<string, any>
  slotCount: number
}

/**
 * 玩家时间轴类型
 */
interface PlayerTimeline {
  playerId: string
  playerName: string
  startTime: number
  endTime: number
  events: EventLog[]
}

interface BuffInfo {
  ccId: number
  buffName: string
}

interface BuffDisplayInfo {
  ccId: number
  buffName: string
  iconData: string
  soundEnabled: boolean
  isActive: boolean
  entityId: number
  entityName: string
  remainingTime: number
  totalTime: number
  willNotify: boolean
}

interface ActiveBuffTimerInfo {
  ccId: number
  buffName: string
  entityId: number
  entityName: string
  remainingTime: number
  totalTime: number
  notifyAt: number
  willNotify: boolean
}

interface BossHPRecord {
  entityId: string
  raceId: number
  currentHp: number
  maxHp: number
  percent: number
  hptimestamp: number
  damageSeq?: number
  threshold?: number
  locked?: boolean
}

interface BossHPExport {
  entityId: string
  raceId: number
  maxHp: number
  history: BossHPRecord[]
}

interface TargetBattleReport extends TakenStats {
  rawDamage: number
  overflowDamage: number
  bossHp?: BossHPExport
  lockEvents: EventLog[]
}

interface BattleUploadSummary {
  targetName: string
  participantCount: number
  effectiveDamage: number
  rawDamage: number
  overflowDamage: number
}

interface BattleReportPayload {
  generatedAt: number
  target?: TargetBattleReport
  otherTargets: TakenStats[]
  currentMap?: CurrentMapInfo | null
  currentDungeon?: DungeonInfo | null
  uploadSummary: BattleUploadSummary
}
