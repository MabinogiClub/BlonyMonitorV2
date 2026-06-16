/**
 * 后端 API 封装
 * 统一管理与 Go 后端的通信
 * 使用 Composable 模式提供响应式 API 调用
 */

import { constants } from '../../wailsjs/go/models'

const isWailsReady = () => !!window.go?.app?.App && !!window.runtime

const previewAttackers: AttackerStats[] = [
  {
    id: '100001',
    name: 'Mirai',
    totalDamage: 2864200,
    dps: 53128,
    percent: 42.8,
    status: 'active',
    skills: [
      { skillId: 40001, totalDamage: 1180000, hitCount: 18, critCount: 7, avgDamage: 65555, minDamage: 32000, maxDamage: 142000, percent: 41.2 },
      { skillId: 40002, totalDamage: 954200, hitCount: 11, critCount: 4, avgDamage: 86745, minDamage: 41000, maxDamage: 188000, percent: 33.3 },
      { skillId: 40003, totalDamage: 730000, hitCount: 24, critCount: 6, avgDamage: 30416, minDamage: 12000, maxDamage: 76000, percent: 25.5 }
    ]
  },
  {
    id: '100002',
    name: '铃兰',
    totalDamage: 2110500,
    dps: 39120,
    percent: 31.5,
    status: 'active',
    skills: [
      { skillId: 41001, totalDamage: 1280000, hitCount: 16, critCount: 5, avgDamage: 80000, minDamage: 36000, maxDamage: 160000, percent: 60.6 },
      { skillId: 41002, totalDamage: 830500, hitCount: 29, critCount: 8, avgDamage: 28637, minDamage: 9000, maxDamage: 84000, percent: 39.4 }
    ]
  },
  {
    id: '100003',
    name: 'Nora',
    totalDamage: 1016200,
    dps: 18818,
    percent: 15.2,
    status: 'idle',
    skills: [
      { skillId: 42001, totalDamage: 616200, hitCount: 21, critCount: 3, avgDamage: 29342, minDamage: 11000, maxDamage: 72000, percent: 60.6 },
      { skillId: 42002, totalDamage: 400000, hitCount: 8, critCount: 2, avgDamage: 50000, minDamage: 26000, maxDamage: 99000, percent: 39.4 }
    ]
  },
  {
    id: '100004',
    name: 'Kuro',
    totalDamage: 703000,
    dps: 13018,
    percent: 10.5,
    status: 'idle',
    skills: [
      { skillId: 43001, totalDamage: 703000, hitCount: 32, critCount: 4, avgDamage: 21968, minDamage: 8000, maxDamage: 66000, percent: 100 }
    ]
  }
]

const previewTargets: TakenStats[] = [
  {
    id: '900001',
    name: '格伦贝尔纳 · 幻影守卫',
    totalDamage: 4790000,
    dps: 88703,
    duration: 54,
    status: 'dead',
    attackers: [
      { ...previewAttackers[0], totalDamage: 1980000, dps: 36666, percent: 41.3 },
      { ...previewAttackers[1], totalDamage: 1660000, dps: 30740, percent: 34.7 },
      { ...previewAttackers[2], totalDamage: 720000, dps: 13333, percent: 15.0 },
      { ...previewAttackers[3], totalDamage: 430000, dps: 7962, percent: 9.0 }
    ]
  },
  {
    id: '900002',
    name: '寒霜祭司',
    totalDamage: 1263900,
    dps: 30092,
    duration: 42,
    status: 'dead',
    attackers: [
      { ...previewAttackers[0], totalDamage: 564200, dps: 13433, percent: 44.6 },
      { ...previewAttackers[1], totalDamage: 450500, dps: 10726, percent: 35.6 },
      { ...previewAttackers[2], totalDamage: 249200, dps: 5933, percent: 19.8 }
    ]
  },
  {
    id: '900003',
    name: '余烬魔像',
    totalDamage: 637000,
    dps: 21233,
    duration: 30,
    status: 'active',
    attackers: [
      { ...previewAttackers[0], totalDamage: 320000, dps: 10666, percent: 50.2 },
      { ...previewAttackers[3], totalDamage: 317000, dps: 10566, percent: 49.8 }
    ]
  }
]

/**
 * 获取 Npcap 状态
 */
export async function getNpcapStatus(): Promise<NpcapStatus> {
  if (!isWailsReady()) {
    return { installed: true, message: '' }
  }
  return window.go.app.App.GetNpcapStatus()
}

/**
 * 打开 Npcap 下载页
 */
export function openNpcapDownloadPage(): void {
  if (!isWailsReady()) return
  window.go.app.App.OpenNpcapDownloadPage()
}

/**
 * 重新检测 Npcap
 */
export async function recheckNpcap(): Promise<NpcapStatus> {
  if (!isWailsReady()) {
    return { installed: true, message: '' }
  }
  return window.go.app.App.RecheckNpcap()
}

/**
 * 获取连接状态
 */
export async function isConnected(): Promise<boolean> {
  if (!isWailsReady()) return false
  return window.go.app.App.IsConnected()
}

/**
 * 获取频道名称
 */
export async function getChannelName(): Promise<string> {
  if (!isWailsReady()) return ''
  return window.go.app.App.GetChannelName()
}

/**
 * 获取所有频道列表
 */
export async function getAllChannels(): Promise<constants.ChannelInfo[]> {
  if (!isWailsReady()) return []
  return window.go.app.App.GetAllChannels()
}

/**
 * 获取完整频道配置（包含服务器和频道信息）
 */
export async function getChannelConfig(): Promise<ChannelConfig> {
  if (!isWailsReady()) return { servers: [] }
  return window.go.app.App.GetChannelConfig()
}

/**
 * 获取是否自动检测频道
 */
export async function getAutoDetect(): Promise<boolean> {
  if (!isWailsReady()) return true
  return window.go.app.App.GetAutoDetect()
}

/**
 * 获取当前选择的频道号
 */
export async function getSelectedChannel(): Promise<number> {
  if (!isWailsReady()) return 0
  return window.go.app.App.GetSelectedChannel()
}

/**
 * 设置是否自动检测频道
 */
export async function setAutoDetect(auto: boolean): Promise<void> {
  if (!isWailsReady()) return
  return window.go.app.App.SetAutoDetect(auto)
}

/**
 * 手动设置频道
 */
export async function setChannel(channel: number): Promise<void> {
  if (!isWailsReady()) return
  return window.go.app.App.SetChannel(channel)
}

/**
 * 设置加速器兼容模式
 */
export async function setAcceleratorMode(enabled: boolean): Promise<void> {
  if (!isWailsReady()) return
  return window.go.app.App.SetAcceleratorMode(enabled)
}

/**
 * 获取所有技能名称映射
 */
export async function getAllSkillNames(): Promise<Record<number, string>> {
  if (!isWailsReady()) {
    return {
      40001: '终焉之击',
      40002: '星陨爆裂',
      40003: '连环斩',
      41001: '符文风暴',
      41002: '灵魂冲击',
      42001: '影袭',
      42002: '月光斩',
      43001: '烈焰弹'
    }
  }
  return window.go.app.App.GetAllSkillNames()
}

/**
 * 获取所有状态名称映射
 */
export async function getAllConditionNames(): Promise<Record<number, string>> {
  if (!isWailsReady()) return {}
  return window.go.app.App.GetAllConditionNames()
}

/**
 * 获取所有技能图标映射 (base64)
 */
export async function getAllSkillIcons(): Promise<Record<number, string>> {
  if (!isWailsReady()) return {}
  return window.go.app.App.GetAllSkillIcons()
}

/**
 * 获取区域设置
 */
export async function getRegion(): Promise<string> {
  if (!isWailsReady()) return 'cn'
  return window.go.app.App.GetRegion()
}

/**
 * 获取调试信息
 */
export async function getDebugInfo(): Promise<DebugInfo> {
  if (!isWailsReady()) {
    return {
      connected: false,
      skillCount: 0,
      parsedSkills: 0,
      parsedStrings: 0,
      raceCount: 0,
      conditionCount: 0,
      entityCount: 0,
      damageCount: 0,
      chartDataLen: 0,
      region: 'cn',
      resourceURL: ''
    }
  }
  return window.go.app.App.GetDebugInfo()
}

/**
 * 重新加载资源数据
 */
export async function reloadResourceData(): Promise<void> {
  if (!isWailsReady()) return
  return window.go.app.App.ReloadResourceData()
}

/**
 * 获取按技能分组的伤害统计
 */
export async function getDamageBySkill(): Promise<AttackerStats[]> {
  if (!isWailsReady()) return previewAttackers
  return window.go.app.App.GetDamageBySkill()
}

/**
 * 获取按攻击者分组的伤害统计
 */
export async function getDamageByAttacker(): Promise<any[]> {
  if (!isWailsReady()) return previewAttackers
  return window.go.app.App.GetDamageByAttacker()
}

/**
 * 获取受到的伤害统计
 */
export async function getDamageTaken(): Promise<TakenStats[]> {
  if (!isWailsReady()) return previewTargets
  return window.go.app.App.GetDamageTaken()
}

/**
 * 获取图表数据
 */
export async function getChartData(): Promise<ChartSeries[]> {
  if (!isWailsReady()) {
    return previewAttackers.map((item, index) => ({
      id: item.id,
      name: item.name,
      data: [
        { time: 0, damage: 0 },
        { time: 15, damage: item.totalDamage * (0.18 + index * 0.03) },
        { time: 30, damage: item.totalDamage * (0.52 + index * 0.02) },
        { time: 45, damage: item.totalDamage * 0.78 },
        { time: 60, damage: item.totalDamage }
      ]
    }))
  }
  return window.go.app.App.GetChartData()
}

/**
 * 获取针对特定目标的图表数据（用于怪物跟踪模式）
 */
export async function getChartDataForTarget(targetId: string): Promise<ChartSeries[]> {
  if (!isWailsReady()) return []
  return window.go.app.App.GetChartDataForTarget(targetId)
}

/**
 * 检查是否有活跃玩家（最近8秒内有攻击）
 */
export async function hasActivePlayer(): Promise<boolean> {
  if (!isWailsReady()) return false
  return window.go.app.App.HasActivePlayer()
}

/**
 * 获取事件日志
 */
export async function getEventLogs(limit = 100, filter = 'all'): Promise<EventLog[]> {
  if (!isWailsReady()) return []
  return window.go.app.App.GetEventLogs(limit, filter)
}

/**
 * 获取所有 PC 实体
 */
export async function getAllPCEntities(): Promise<EntityInfo[]> {
  if (!isWailsReady()) return []
  return window.go.app.App.GetAllPCEntities()
}

/**
 * 获取所有生物
 */
export async function getAllCreatures(): Promise<CreatureInfo[]> {
  if (!isWailsReady()) return []
  return window.go.app.App.GetAllCreatures()
}

/**
 * 清空统计数据（不保存）
 */
export async function clear(): Promise<void> {
  if (!isWailsReady()) return
  return window.go.app.App.Clear()
}

/**
 * 保存并清空统计数据
 */
export async function clearAndSave(): Promise<void> {
  if (!isWailsReady()) return
  return window.go.app.App.ClearAndSave()
}

/**
 * 获取历史记录文件列表
 */
export async function getCleanedTargetsList(): Promise<string[]> {
  if (!isWailsReady()) return []
  return window.go.app.App.GetCleanedTargetsList()
}

/**
 * 读取历史记录文件
 */
export async function readCleanedTargetFileFull(fileName: string): Promise<HistoryFileData | HistoryTarget[] | { error: string }> {
  if (!isWailsReady()) return { error: '应用未就绪' }
  return window.go.app.App.ReadCleanedTargetFileFull(fileName)
}

/**
 * 获取战斗记录保存目录
 */
export async function getSaveDir(): Promise<string> {
  if (!isWailsReady()) return ''
  return window.go.app.App.GetSaveDir()
}

/**
 * 退出应用
 */
export function quit(): void {
  if (!isWailsReady()) return
  window.runtime.Quit()
}

/**
 * 隐藏窗口（最小化到托盘）
 */
export async function hide(): Promise<void> {
  if (!isWailsReady()) return
  return window.go.app.App.Hide()
}

/**
 * 设置鼠标穿透状态
 */
export async function setClickThrough(enabled: boolean): Promise<void> {
  if (!isWailsReady()) return
  return window.go.app.App.SetClickThrough(enabled)
}

/**
 * 获取鼠标穿透状态
 */
export async function getClickThrough(): Promise<boolean> {
  if (!isWailsReady()) return false
  return window.go.app.App.GetClickThrough()
}

/**
 * 设置窗口透明度 (0-100)
 */
export async function setOpacity(opacity: number): Promise<void> {
  if (!isWailsReady()) return
  return window.go.app.App.SetOpacity(opacity)
}

/**
 * 获取窗口透明度
 */
export async function getOpacity(): Promise<number> {
  if (!isWailsReady()) return 100
  return window.go.app.App.GetOpacity()
}

/**
 * 设置窗口固定在前
 */
export async function setAlwaysOnTop(enabled: boolean): Promise<void> {
  if (!isWailsReady()) return
  return window.go.app.App.SetAlwaysOnTop(enabled)
}

/**
 * 获取窗口固定在前状态
 */
export async function getAlwaysOnTop(): Promise<boolean> {
  if (!isWailsReady()) return false
  return window.go.app.App.GetAlwaysOnTop()
}

/**
 * 设置窗口大小
 */
export async function setWindowSize(width: number, height: number): Promise<void> {
  if (!isWailsReady()) return
  return window.go.app.App.SetWindowSize(width, height)
}

/**
 * 获取窗口大小
 */
export async function getWindowSize(): Promise<{ width: number; height: number }> {
  if (!isWailsReady()) return { width: window.innerWidth, height: window.innerHeight }
  return window.go.app.App.GetWindowSize()
}

/**
 * 设置窗口最小尺寸
 */
export async function setWindowMinSize(width: number, height: number): Promise<void> {
  if (!isWailsReady()) return
  return window.go.app.App.SetWindowMinSize(width, height)
}

/**
 * 设置窗口最大尺寸
 */
export async function setWindowMaxSize(width: number, height: number): Promise<void> {
  if (!isWailsReady()) return
  return window.go.app.App.SetWindowMaxSize(width, height)
}

/**
 * 设置窗口是否可调整大小
 */
export async function setWindowResizable(resizable: boolean): Promise<void> {
  if (!isWailsReady()) return
  return window.go.app.App.SetWindowResizable(resizable)
}

/**
 * 注册事件监听
 * @param eventName 事件名称
 * @param callback 回调函数
 */
export function onEvent(eventName: string, callback: (...args: any[]) => void): void {
  if (!isWailsReady()) return
  window.runtime.EventsOn(eventName, callback)
}

/**
 * 取消事件监听
 * @param eventName 事件名称
 */
export function offEvent(eventName: string): void {
  if (!isWailsReady()) return
  window.runtime.EventsOff(eventName)
}

/**
 * 获取当前地图信息
 */
export async function getCurrentMap(): Promise<CurrentMapInfo | null> {
  if (!isWailsReady()) return { mapId: 35012, mapName: '格伦贝尔纳', localName: '地下城' }
  return window.go.app.App.GetCurrentMap()
}

/**
 * 获取当前地下城信息
 */
export async function getCurrentDungeon(): Promise<DungeonInfo | null> {
  if (!isWailsReady()) {
    return {
      instanceId: 1780501,
      dungeonName: 'Glenn Bearna Hard',
      dungeonId: 35012,
      seed: 904217,
      difficulty: 3,
      floorCount: 4,
      floorLayout: [1, 2, 3, 4],
      enteredAt: Math.floor(Date.now() / 1000) - 64,
      completedAt: 0,
      isCompleted: false
    }
  }
  return window.go.app.App.GetCurrentDungeon()
}

/**
 * 获取玩家自身信息
 */
export async function getSelfInfo(): Promise<SelfInfo | null> {
  if (!isWailsReady()) return { id: '100001', name: 'Mirai' }
  return window.go.app.App.GetSelfInfo()
}

/**
 * 获取玩家时间轴
 */
export async function getPlayerTimeline(playerId: string): Promise<PlayerTimeline> {
  if (!isWailsReady()) {
    return { playerId, playerName: '', startTime: 0, endTime: 0, events: [] }
  }
  return window.go.app.App.GetPlayerTimeline(playerId)
}

/**
 * 获取 Buff 展示列表（含声音开关与倒计时）
 */
export async function getBuffDisplayList(): Promise<BuffDisplayInfo[]> {
  if (!isWailsReady()) return []
  return window.go.app.App.GetBuffDisplayList()
}

/**
 * 设置单个 Buff 的声音开关
 */
export async function setBuffSoundEnabled(ccId: number, enabled: boolean): Promise<void> {
  if (!isWailsReady()) return
  return window.go.app.App.SetBuffSoundEnabled(ccId, enabled)
}

/**
 * 设置 Buff 显示顺序
 */
export async function setBuffOrder(order: number[]): Promise<void> {
  if (!isWailsReady()) return
  return window.go.app.App.SetBuffOrder(order)
}
