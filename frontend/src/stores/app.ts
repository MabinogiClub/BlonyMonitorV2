/**
 * 应用状态管理
 */

import { defineStore } from 'pinia'
import { ref, reactive } from 'vue'
import * as api from '../composables/useApi'

interface LogFilters {
  damage: boolean
  appear: boolean
  condition: boolean
  finish: boolean
}

interface Config {
  resourceURL: string
  region: string
}

interface SelectedTarget {
  id: string
  name: string
}

export const useAppStore = defineStore('app', () => {
  const expandedItems = ref<Set<string>>(new Set())
  const selectedTarget = ref<SelectedTarget | null>(null)
  const debugVisible = ref(false)
  const lastError = ref('无')
  const activeTab = ref('bySkill')
  const isConnected = ref(false)
  const channelName = ref('')
  const clickThroughEnabled = ref(false)
  const alwaysOnTop = ref(false)
  const chartVisible = ref(false)
  const opacity = ref(100)

  const chartData = ref<ChartSeries[]>([])
  const skillNameMap = ref<Record<number, string>>({})
  const skillIconMap = ref<Record<number, string>>({})
  const conditionNameMap = ref<Record<number, string>>({})
  const lastEntitiesJson = ref('')

  const logFilters = reactive<LogFilters>({
    damage: true,
    appear: true,
    condition: false,
    finish: true
  })

  const config = reactive<Config>({
    resourceURL: 'https://mabires.pril.cc',
    region: 'cn'
  })

  const channelsConfig = ref<ChannelConfig | null>(null)
  const autoDetect = ref(true)
  const currentChannelId = ref(0)
  const acceleratorMode = ref(false)
  const currentMap = ref<CurrentMapInfo | null>(null)
  const currentDungeon = ref<DungeonInfo | null>(null)
  const selfInfo = ref<SelfInfo | null>(null)

  function updateConfig(key: keyof Config, value: string) {
    config[key] = value
  }

  function toggleExpanded(id: string) {
    if (expandedItems.value.has(id)) {
      expandedItems.value.delete(id)
    } else {
      expandedItems.value.add(id)
    }
  }

  function isExpanded(id: string): boolean {
    return expandedItems.value.has(id)
  }

  function setSelectedTarget(target: SelectedTarget) {
    selectedTarget.value = target
  }

  function clearSelectedTarget() {
    selectedTarget.value = null
  }

  function toggleLogFilter(type: keyof LogFilters) {
    logFilters[type] = !logFilters[type]
  }

  function toggleDebug() {
    debugVisible.value = !debugVisible.value
  }

  function setActiveTab(tab: string) {
    activeTab.value = tab
  }

  function resetState() {
    expandedItems.value.clear()
    chartData.value = []
    selectedTarget.value = null
    lastEntitiesJson.value = ''
  }

  function updateConnectionStatus(connected: boolean) {
    isConnected.value = connected
  }

  async function toggleClickThrough() {
    clickThroughEnabled.value = !clickThroughEnabled.value
    await api.setClickThrough(clickThroughEnabled.value)
  }

  async function toggleAlwaysOnTop() {
    alwaysOnTop.value = !alwaysOnTop.value
    await api.setAlwaysOnTop(alwaysOnTop.value)
  }

  async function clearStats() {
    await api.clear()
    resetState()
  }

  async function initialize() {
    try {
      isConnected.value = await api.isConnected()
    } catch (e) {
      console.error('检查连接状态失败:', e)
    }

    try {
      const name = await api.getChannelName()
      if (name) {
        channelName.value = name
      }
    } catch (e) {
      console.error('获取频道名称失败:', e)
    }

    try {
      skillNameMap.value = await api.getAllSkillNames()
      skillIconMap.value = await api.getAllSkillIcons()
      conditionNameMap.value = await api.getAllConditionNames()
      config.region = await api.getRegion()
    } catch (e) {
      console.error('加载技能名称失败:', e)
    }

    await loadChannelsConfig()

    try {
      autoDetect.value = await api.getAutoDetect()
      currentChannelId.value = await api.getSelectedChannel()
    } catch (e) {
      console.error('加载频道设置失败:', e)
    }

    try {
      alwaysOnTop.value = await api.getAlwaysOnTop()
    } catch (e) {
      console.error('加载窗口固定在前状态失败:', e)
    }

    try {
      opacity.value = await api.getOpacity()
    } catch (e) {
      console.error('加载窗口透明度失败:', e)
    }

    try {
      currentMap.value = await api.getCurrentMap()
    } catch (e) {
      console.error('加载地图信息失败:', e)
    }

    try {
      currentDungeon.value = await api.getCurrentDungeon()
    } catch (e) {
      console.error('加载地下城信息失败:', e)
    }

    try {
      selfInfo.value = await api.getSelfInfo()
    } catch (e) {
      console.error('加载玩家信息失败:', e)
    }

    setTimeout(async () => {
      try {
        skillNameMap.value = await api.getAllSkillNames()
        skillIconMap.value = await api.getAllSkillIcons()
        conditionNameMap.value = await api.getAllConditionNames()
      } catch {
        // ignore
      }
    }, 3000)
  }

  async function loadChannelsConfig() {
    try {
      channelsConfig.value = await api.getChannelConfig()
    } catch (e) {
      console.error('加载频道配置失败:', e)
    }
  }

  async function selectChannel(channelId: number) {
    currentChannelId.value = channelId
    try {
      await api.setChannel(channelId)
    } catch (e) {
      console.error('设置频道失败:', e)
    }
  }

  async function setAutoDetectMode(auto: boolean) {
    autoDetect.value = auto
    try {
      if (auto) {
        await api.setAutoDetect(true)
      } else if (currentChannelId.value > 0) {
        await api.setChannel(currentChannelId.value)
      } else {
        await api.setAutoDetect(false)
      }
    } catch (e) {
      console.error('设置频道模式失败:', e)
    }
  }

  async function setAcceleratorMode(enabled: boolean) {
    acceleratorMode.value = enabled
    try {
      await api.setAcceleratorMode(enabled)
    } catch (e) {
      console.error('设置加速器模式失败:', e)
    }
  }

  async function reloadResources() {
    try {
      await api.reloadResourceData()
      setTimeout(async () => {
        skillNameMap.value = await api.getAllSkillNames()
        skillIconMap.value = await api.getAllSkillIcons()
        conditionNameMap.value = await api.getAllConditionNames()
      }, 2000)
    } catch (e) {
      lastError.value = String(e)
      console.error('重载资源失败:', e)
    }
  }

  function registerEvents() {
    api.onEvent('damage', () => {})
    api.onEvent('clear', () => resetState())
    api.onEvent('connected', (connected: boolean) => updateConnectionStatus(connected))
    api.onEvent('channel', (name: string) => { channelName.value = name })
    api.onEvent('autoDetectChanged', (auto: boolean) => { autoDetect.value = auto })
    api.onEvent('mapChange', async (mapInfo: CurrentMapInfo) => {
      currentMap.value = mapInfo
      try {
        currentDungeon.value = await api.getCurrentDungeon()
      } catch (e) {
        console.error('刷新地下城信息失败:', e)
      }
    })
    api.onEvent('selfInfo', (info: SelfInfo) => { selfInfo.value = info })
  }

  async function updateAllViews() {
    try {
      const hasActive = await api.hasActivePlayer()
      if (!hasActive) {
        return
      }
      chartData.value = await api.getChartData()
    } catch (e) {
      console.error('更新图表数据失败:', e)
    }
  }

  async function setOpacity(newOpacity: number) {
    try {
      await api.setOpacity(newOpacity)
      opacity.value = newOpacity
    } catch (e) {
      console.error('设置透明度失败:', e)
    }
  }

  function quit() {
    api.quit()
  }

  function hide() {
    api.hide()
  }

  function toggleChartVisible() {
    chartVisible.value = !chartVisible.value
  }

  return {
    expandedItems,
    selectedTarget,
    debugVisible,
    lastError,
    activeTab,
    isConnected,
    channelName,
    clickThroughEnabled,
    alwaysOnTop,
    chartData,
    skillNameMap,
    skillIconMap,
    conditionNameMap,
    lastEntitiesJson,
    logFilters,
    config,
    channelsConfig,
    autoDetect,
    currentChannelId,
    acceleratorMode,
    currentMap,
    currentDungeon,
    selfInfo,
    chartVisible,
    opacity,
    updateConfig,
    toggleExpanded,
    isExpanded,
    setSelectedTarget,
    clearSelectedTarget,
    toggleLogFilter,
    toggleDebug,
    setActiveTab,
    resetState,
    updateConnectionStatus,
    toggleClickThrough,
    toggleAlwaysOnTop,
    clearStats,
    initialize,
    loadChannelsConfig,
    selectChannel,
    setAutoDetectMode,
    setAcceleratorMode,
    reloadResources,
    registerEvents,
    updateAllViews,
    setOpacity,
    quit,
    hide,
    toggleChartVisible,
  }
})
