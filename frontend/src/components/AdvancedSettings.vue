<script setup lang="ts">
/**
 * 高级设置对话框组件
 * 用于手动选择网卡等高级配置
 * 使用 Vant 组件库
 */

import { ref, watch, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { GetAllNics, GetManualNic, SetManualNic, GetAccelerators, GetSelectedAccelerator, SetAccelerator, GetUploadSettings, SetUploadSettings, GetRankingParticipation, SetRankingParticipation, GetAnalysisLogEnabled, GetAnalysisLogPath, SetAnalysisLogEnabled } from '../../wailsjs/go/app/App'
import { useAppStore } from '../stores/app'
import * as api from '../composables/useApi'

// 定义 props
const props = defineProps<{
  visible: boolean
  initialSection?: 'general' | 'ranking'
}>()

// 定义 emits
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'update:visible', value: boolean): void
}>()

// 获取应用状态
const appStore = useAppStore()

// 网卡列表
const nics = ref<Array<{ name: string; description: string }>>([])
// 当前选择的网卡
const selectedNic = ref('')
// 加载状态
const loading = ref(false)
// 搜索关键词
const searchKeyword = ref('')

// 透明度
const opacity = ref(100)

// 频道相关
const autoDetect = ref(true)
const currentChannelId = ref(0)
const acceleratorMode = ref(false)
const accelerators = ref<Array<{id: string, name: string, ip: string, port: number}>>([])
const selectedAccelerator = ref('')
const channelDropdownVisible = ref(false)
const acceleratorDropdownVisible = ref(false)

// 战斗数据推送
const uploadEnabled = ref(false)
const uploadSecretReady = ref(false)
const rankingMode = ref<RankingMode>('none')
const confirmedRankingMode = ref<RankingMode>('none')
const rankingAvailable = ref(false)
const rankingPlayerReady = ref(false)
const rankingPlayerName = ref('')
const rankingLoading = ref(false)
const rankingSaving = ref(false)
const rankingSyncFailed = ref(false)
let rankingRequestVersion = 0
const rankingSectionRef = ref<HTMLElement | null>(null)

// 分析日志默认关闭，仅在需要反馈性能问题时启用
const analysisLogEnabled = ref(false)
const analysisLogPath = ref('')
const analysisLogSaving = ref(false)
const backendRefreshInterval = ref(100)
const frontendRefreshInterval = ref(200)
const refreshSettingsSaving = ref(false)

const backendRefreshOptions = [
  { value: 50, label: '50 ms（极高频）' },
  { value: 100, label: '100 ms（默认）' },
  { value: 250, label: '250 ms' },
  { value: 500, label: '500 ms' },
  { value: 1000, label: '1 秒' },
  { value: 2000, label: '2 秒' },
  { value: 5000, label: '5 秒' },
  { value: 15000, label: '15 秒（低负载测试）' },
]

const frontendRefreshOptions = [
  { value: 50, label: '50 ms（极高频）' },
  { value: 100, label: '100 ms' },
  { value: 200, label: '200 ms（默认）' },
  { value: 500, label: '500 ms' },
  { value: 1000, label: '1 秒' },
  { value: 2000, label: '2 秒' },
  { value: 5000, label: '5 秒' },
  { value: 15000, label: '15 秒（低负载测试）' },
]

const rankingModeOptions: Array<{ value: RankingMode; label: string }> = [
  { value: 'none', label: '不参与排行' },
  { value: 'anonymous', label: '匿名排行' },
  { value: 'public', label: '公开排行' },
]

function normalizeRankingMode(value: string, participating: boolean): RankingMode {
  if (value === 'anonymous' || value === 'public') return value
  if (value === 'none') return 'none'
  return participating ? 'public' : 'none'
}

const rankingStatusText = computed(() => {
  if (rankingLoading.value) return '同步中'
  if (rankingSaving.value) return '保存中'
  if (rankingSyncFailed.value) return '同步失败'
  if (!rankingAvailable.value) return '服务未配置'
  if (!rankingPlayerReady.value) return '未识别角色'
  return rankingModeOptions.find(option => option.value === rankingMode.value)?.label || '不参与排行'
})

const rankingUnavailableReason = computed(() => {
  if (rankingLoading.value || rankingSaving.value) return ''
  if (rankingSyncFailed.value) return '无法从排行服务器读取当前状态，请重新同步后再修改。'
  if (!rankingAvailable.value) return '排行服务尚未配置，当前版本无法修改此设置。'
  if (!rankingPlayerReady.value) return '尚未识别当前游戏角色。进入游戏并识别角色后，可重新检测并修改此设置。'
  return ''
})

const rankingRetryVisible = computed(() => (
  rankingSyncFailed.value || (rankingAvailable.value && !rankingPlayerReady.value)
))

const rankingCanToggle = computed(() => (
  rankingAvailable.value &&
  rankingPlayerReady.value &&
  !rankingLoading.value &&
  !rankingSaving.value &&
  !rankingSyncFailed.value
))

/**
 * 加载网卡列表
 */
async function loadNics() {
  loading.value = true
  try {
    nics.value = await GetAllNics()
    selectedNic.value = await GetManualNic()
  } catch (err) {
    console.error('加载网卡列表失败:', err)
  } finally {
    loading.value = false
  }
}

/**
 * 加载透明度
 */
async function loadOpacity() {
  try {
    opacity.value = appStore.opacity
  } catch (err) {
    console.error('加载透明度失败:', err)
  }
}

/**
 * 处理透明度变化
 */
async function handleOpacityChange(event: Event) {
  const target = event.target as HTMLInputElement
  const newOpacity = parseInt(target.value, 10)
  opacity.value = newOpacity
  await appStore.setOpacity(newOpacity)
}

/**
 * 加载战斗数据推送设置
 */
async function loadUploadSettings() {
  try {
    const settings = await GetUploadSettings()
    uploadEnabled.value = settings.enabled
    uploadSecretReady.value = settings.secretReady
  } catch (err) {
    console.error('加载推送设置失败:', err)
  }
}

async function loadAnalysisLogSettings() {
  try {
    analysisLogEnabled.value = await GetAnalysisLogEnabled()
    analysisLogPath.value = await GetAnalysisLogPath()
  } catch (err) {
    console.error('加载分析日志设置失败:', err)
  }
}

async function loadRefreshSettings() {
  try {
    const settings = await api.getDPSRefreshSettings()
    backendRefreshInterval.value = settings.backendIntervalMs
    frontendRefreshInterval.value = settings.frontendIntervalMs
    appStore.applyDPSRefreshSettings(settings)
  } catch (err) {
    console.error('加载刷新设置失败:', err)
  }
}

async function saveRefreshSettings() {
  if (refreshSettingsSaving.value) return
  refreshSettingsSaving.value = true
  try {
    const settings = await appStore.setDPSRefreshSettings({
      backendIntervalMs: backendRefreshInterval.value,
      frontendIntervalMs: frontendRefreshInterval.value,
    })
    backendRefreshInterval.value = settings.backendIntervalMs
    frontendRefreshInterval.value = settings.frontendIntervalMs
  } catch (err) {
    backendRefreshInterval.value = appStore.dpsBackendRefreshInterval
    frontendRefreshInterval.value = appStore.dpsFrontendRefreshInterval
    console.error('保存刷新设置失败:', err)
  } finally {
    refreshSettingsSaving.value = false
  }
}

async function handleAnalysisLogChange(event: Event) {
  const target = event.target as HTMLInputElement
  const nextValue = target.checked
  analysisLogSaving.value = true
  try {
    await SetAnalysisLogEnabled(nextValue)
    analysisLogEnabled.value = nextValue
    if (!analysisLogPath.value) analysisLogPath.value = await GetAnalysisLogPath()
  } catch (err) {
    target.checked = analysisLogEnabled.value
    console.error('设置分析日志失败:', err)
  } finally {
    analysisLogSaving.value = false
  }
}

async function saveUploadSettings() {
  try {
    await SetUploadSettings({
      enabled: uploadEnabled.value,
      endpoint: '',
      endpointReady: false,
      dungeonKeyword: '',
      secretReady: uploadSecretReady.value,
    })
  } catch (err) {
    console.error('保存推送设置失败:', err)
  }
}

async function loadRankingParticipation() {
  const requestVersion = ++rankingRequestVersion
  rankingLoading.value = true
  rankingSaving.value = false
  rankingSyncFailed.value = false
  try {
    const state = await GetRankingParticipation()
    if (requestVersion !== rankingRequestVersion) return
    rankingAvailable.value = state.available
    rankingPlayerReady.value = state.playerReady
    rankingPlayerName.value = state.playerName || ''
    rankingMode.value = normalizeRankingMode(state.mode, state.participating)
    confirmedRankingMode.value = rankingMode.value
  } catch (err) {
    if (requestVersion !== rankingRequestVersion) return
    rankingSyncFailed.value = true
    console.error('同步排行参与状态失败:', err)
  } finally {
    if (requestVersion === rankingRequestVersion) {
      rankingLoading.value = false
    }
  }
}

async function handleRankingParticipationChange(mode: RankingMode) {
  if (!rankingCanToggle.value) return
  const requestVersion = ++rankingRequestVersion
  const previousMode = confirmedRankingMode.value
  rankingSaving.value = true
  try {
    const state = await SetRankingParticipation(mode)
    if (requestVersion !== rankingRequestVersion) return
    rankingMode.value = normalizeRankingMode(state.mode, state.participating)
    confirmedRankingMode.value = rankingMode.value
    rankingPlayerName.value = state.playerName || rankingPlayerName.value
  } catch (err) {
    if (requestVersion !== rankingRequestVersion) return
    rankingMode.value = previousMode
    rankingSyncFailed.value = true
    console.error('保存排行参与状态失败:', err)
  } finally {
    if (requestVersion === rankingRequestVersion) {
      rankingSaving.value = false
    }
  }
}

async function handleUploadEnabledChange(event: Event) {
  const target = event.target as HTMLInputElement
  uploadEnabled.value = target.checked
  await saveUploadSettings()
}

/**
 * 加载频道设置
 */
async function loadChannelSettings() {
  try {
    autoDetect.value = appStore.autoDetect
    currentChannelId.value = appStore.currentChannelId
    acceleratorMode.value = appStore.acceleratorMode
  } catch (err) {
    console.error('加载频道设置失败:', err)
  }
}

/**
 * 加载加速器列表
 */
async function loadAccelerators() {
  try {
    accelerators.value = await GetAccelerators()
    selectedAccelerator.value = await GetSelectedAccelerator()
  } catch (err) {
    console.error('加载加速器列表失败:', err)
  }
}

/**
 * 处理自动检测切换
 */
async function handleAutoDetectChange(event: Event) {
  const target = event.target as HTMLInputElement
  autoDetect.value = target.checked
  await appStore.setAutoDetectMode(target.checked)
  if (target.checked) {
    closeChannelDropdown()
  }
}

/**
 * 切换频道下拉菜单
 */
function toggleChannelDropdown() {
  if (autoDetect.value) return
  channelDropdownVisible.value = !channelDropdownVisible.value
}

/**
 * 关闭频道下拉菜单
 */
function closeChannelDropdown() {
  channelDropdownVisible.value = false
}

/**
 * 选择频道
 */
async function handleSelectChannel(channelId: number) {
  currentChannelId.value = channelId
  await appStore.selectChannel(channelId)
  closeChannelDropdown()
}

/**
 * 切换加速器下拉菜单
 */
function toggleAcceleratorDropdown() {
  acceleratorDropdownVisible.value = !acceleratorDropdownVisible.value
}

/**
 * 关闭加速器下拉菜单
 */
function closeAcceleratorDropdown() {
  acceleratorDropdownVisible.value = false
}

/**
 * 选择加速器
 */
async function handleSelectAccelerator(id: string) {
  try {
    const success = await SetAccelerator(id)
    if (success) {
      selectedAccelerator.value = id
    }
  } catch (err) {
    console.error('切换加速器失败:', err)
  }
}

/**
 * 获取服务器列表
 */
const servers = computed(() => {
  const config = appStore.channelsConfig
  return config?.servers || config?.Servers || []
})

/**
 * 获取选中的频道显示文本
 */
const selectedChannelText = computed(() => {
  if (currentChannelId.value <= 0) {
    return '选择频道'
  }
  
  // 查找频道名称
  for (const server of servers.value) {
    const serverName = server.name || server.Name
    const channels = server.channels || server.Channels || []
    for (const ch of channels) {
      const chId = ch.id ?? ch.ID
      if (chId === currentChannelId.value) {
        const chName = ch.name || ch.Name
        return `${serverName} ${chName}`
      }
    }
  }
  
  return '选择频道'
})

/**
 * 获取选中的加速器显示文本
 */
const selectedAcceleratorText = computed(() => {
  const acc = accelerators.value.find(a => a.id === selectedAccelerator.value)
  return acc ? acc.name : '选择加速器'
})

/**
 * 处理网卡选择变化（即时保存）
 */
async function handleNicChange(nicName: string) {
  try {
    await SetManualNic(nicName)
  } catch (err) {
    console.error('设置网卡失败:', err)
  }
}

/**
 * 关闭对话框
 */
function close() {
  // 关闭所有下拉菜单
  closeChannelDropdown()
  closeAcceleratorDropdown()
  emit('close')
  emit('update:visible', false)
}

/**
 * 过滤后的网卡列表
 */
const filteredNics = computed(() => {
  if (!searchKeyword.value.trim()) {
    return nics.value
  }
  const keyword = searchKeyword.value.toLowerCase()
  return nics.value.filter(nic =>
    nic.name.toLowerCase().includes(keyword) ||
    (nic.description && nic.description.toLowerCase().includes(keyword))
  )
})

// 点击外部关闭下拉菜单
function handleClickOutside(event: MouseEvent) {
  const target = event.target as HTMLElement
  if (!target.closest('.channel-select-wrapper') && !target.closest('.accelerator-select-wrapper')) {
    closeChannelDropdown()
    closeAcceleratorDropdown()
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})

// 监听网卡选择变化，即时保存
watch(selectedNic, (newVal) => {
  if (props.visible) {
    handleNicChange(newVal)
  }
})

// 监听对话框显示状态，当显示时加载数据
watch(() => props.visible, (newVal) => {
  if (newVal) {
    loadNics()
    loadOpacity()
    loadChannelSettings()
    loadUploadSettings()
    loadAnalysisLogSettings()
    loadRefreshSettings()
    loadRankingParticipation()
    if (props.initialSection === 'ranking') {
      void nextTick(() => rankingSectionRef.value?.scrollIntoView({ block: 'center' }))
    }
    if (acceleratorMode.value) {
      loadAccelerators()
    }
    searchKeyword.value = '' // 重置搜索关键词
    // 关闭下拉菜单
    closeChannelDropdown()
    closeAcceleratorDropdown()
  }
})

watch(() => appStore.selfInfo?.id, (newID, oldID) => {
  if (props.visible && newID && newID !== oldID) {
    loadRankingParticipation()
  }
})
</script>

<template>
  <!-- 使用 Vant Popup 组件 -->
  <van-popup
    :show="visible"
    position="center"
    round
    closeable
    close-icon-position="top-right"
    :style="{ width: '600px', maxWidth: '90vw', maxHeight: '80vh' }"
    @click-overlay="close"
    @click-close-icon="close"
    teleport="body"
  >
    <div class="advanced-settings-dialog">
      <!-- 标题 -->
      <div class="dialog-header">
        <h3>高级设置</h3>
      </div>

      <!-- 内容 -->
      <div class="dialog-content">
        <!-- 透明度设置 -->
        <div class="setting-section">
          <div class="section-header">
            <div>
              <h4>窗口透明度</h4>
              <p class="setting-desc">调整窗口透明度（20-100）</p>
            </div>
          </div>
          <div class="opacity-control">
            <input
              type="range"
              id="opacitySlider"
              min="20"
              max="100"
              :value="opacity"
              @input="handleOpacityChange"
              @change="handleOpacityChange"
            >
            <span class="opacity-value">{{ opacity }}%</span>
          </div>
        </div>

        <!-- 分割线 -->
        <div class="setting-divider"></div>

        <!-- 频道设置 -->
        <div class="setting-section">
          <div class="section-header">
            <div>
              <h4>频道选择</h4>
              <p class="setting-desc">选择频道或启用自动检测</p>
            </div>
          </div>
          
          <!-- 自动检测开关 -->
          <div class="auto-detect-control">
            <label class="auto-detect-label">
              <input
                type="checkbox"
                :checked="autoDetect"
                @change="handleAutoDetectChange"
              >
              <span>{{ acceleratorMode ? '加速器兼容模式' : '自动检测频道' }}</span>
            </label>
          </div>

          <!-- 频道选择（普通模式） -->
          <div v-if="!acceleratorMode" class="channel-select-control">
            <div class="channel-select-wrapper">
              <div 
                class="channel-select-trigger" 
                :class="{ disabled: autoDetect }"
                @click.stop="toggleChannelDropdown"
              >
                <span>{{ selectedChannelText }}</span>
                <span class="select-arrow">▼</span>
              </div>
              <div 
                v-if="!autoDetect && channelDropdownVisible" 
                class="channel-select-dropdown"
                @click.stop
              >
                <div 
                  v-for="(server, serverIndex) in servers" 
                  :key="serverIndex"
                  class="channel-server-group"
                >
                  <div class="channel-server-name">{{ server.name || server.Name }}</div>
                  <div 
                    v-for="(channel, channelIndex) in (server.channels || server.Channels || [])" 
                    :key="channelIndex"
                    class="channel-item"
                    :class="{ active: (channel.id ?? channel.ID ?? 0) === currentChannelId }"
                    @click="handleSelectChannel(channel.id ?? channel.ID ?? 0)"
                  >
                    {{ channel.name || channel.Name }}
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- 加速器选择（加速器模式） -->
          <div v-if="acceleratorMode" class="accelerator-select-control">
            <div class="accelerator-select-wrapper">
              <div 
                class="accelerator-select-trigger"
                @click.stop="toggleAcceleratorDropdown"
              >
                <span>{{ selectedAcceleratorText }}</span>
                <span class="select-arrow">▼</span>
              </div>
              <div 
                v-if="acceleratorDropdownVisible"
                class="accelerator-select-dropdown"
                @click.stop
              >
                <div
                  v-for="acc in accelerators"
                  :key="acc.id"
                  class="accelerator-item"
                  :class="{ active: acc.id === selectedAccelerator }"
                  @click="handleSelectAccelerator(acc.id); closeAcceleratorDropdown()"
                >
                  {{ acc.name }}
                </div>
              </div>
            </div>
          </div>

        </div>

        <!-- 分割线 -->
        <div class="setting-divider"></div>

        <!-- 战斗数据推送 -->
        <div class="setting-section">
          <div class="section-header">
            <h4>战斗数据推送</h4>
            <van-tag :type="uploadSecretReady ? 'success' : 'warning'" plain>
              {{ uploadSecretReady ? '密钥已配置' : '密钥未配置' }}
            </van-tag>
          </div>

          <div class="auto-detect-control">
            <label class="auto-detect-label">
              <input
                type="checkbox"
                :checked="uploadEnabled"
                @change="handleUploadEnabledChange"
              >
              <span>启用推送</span>
            </label>
          </div>

          <div ref="rankingSectionRef" class="ranking-control">
            <div class="ranking-control-row">
              <div class="ranking-control-copy">
                <span class="ranking-control-title">
                  公开排行状态
                  <template v-if="rankingPlayerName"> · {{ rankingPlayerName }}</template>
                </span>
                <span class="ranking-control-status">{{ rankingStatusText }}</span>
              </div>
              <select
                v-model="rankingMode"
                :disabled="!rankingCanToggle"
                :title="rankingUnavailableReason || '选择当前角色的排行参与方式'"
                aria-label="公开排行状态"
                class="ranking-mode-select"
                @change="handleRankingParticipationChange(rankingMode)"
              >
                <option
                  v-for="option in rankingModeOptions"
                  :key="option.value"
                  :value="option.value"
                >
                  {{ option.label }}
                </option>
              </select>
            </div>
            <p class="ranking-consent-note">
              不参与不会进入排行；匿名排行会隐藏角色名；公开排行会展示角色名。队友上传同场数据时，服务器仍会按你的选择过滤。此设置按角色生效，不影响本地统计和战斗数据推送。
            </p>
            <p
              v-if="rankingUnavailableReason"
              class="ranking-unavailable-reason"
              role="status"
            >
              {{ rankingUnavailableReason }}
            </p>
            <button
              v-if="rankingRetryVisible"
              type="button"
              class="ranking-retry-button"
              :title="rankingSyncFailed ? '重新同步排行状态' : '重新检测当前角色'"
              @click="loadRankingParticipation"
            >
              <van-icon name="replay" aria-hidden="true" />
              <span>{{ rankingSyncFailed ? '重新同步' : '重新检测角色' }}</span>
            </button>
          </div>
        </div>

        <!-- 分割线 -->
        <div class="setting-divider"></div>

        <!-- DPS 刷新设置 -->
        <div class="setting-section">
          <div class="section-header">
            <div>
              <h4>DPS 刷新间隔</h4>
              <p class="setting-desc">分别控制后端推送和前端统计页面重算频率</p>
            </div>
          </div>
          <div class="refresh-rate-grid">
            <label class="refresh-rate-field">
              <span>后端推送</span>
              <select
                v-model.number="backendRefreshInterval"
                :disabled="refreshSettingsSaving"
                @change="saveRefreshSettings"
              >
                <option v-for="option in backendRefreshOptions" :key="option.value" :value="option.value">
                  {{ option.label }}
                </option>
              </select>
            </label>
            <label class="refresh-rate-field">
              <span>前端计算</span>
              <select
                v-model.number="frontendRefreshInterval"
                :disabled="refreshSettingsSaving"
                @change="saveRefreshSettings"
              >
                <option v-for="option in frontendRefreshOptions" :key="option.value" :value="option.value">
                  {{ option.label }}
                </option>
              </select>
            </label>
          </div>
          <p class="refresh-rate-warning" role="note">
            较短间隔会增加 CPU 占用；较长间隔会让实时 DPS 和 Buff 显示滞后，实际速度受两项中较大的间隔限制。此设置不会丢失伤害数据，也不会改变历史记录精度。
          </p>
        </div>

        <!-- 分割线 -->
        <div class="setting-divider"></div>

        <!-- 分析日志 -->
        <div class="setting-section">
          <div class="section-header">
            <div>
              <h4>输出分析日志</h4>
              <p class="setting-desc">仅在需要排查卡顿或延迟问题时开启，日志每 5 秒记录一次运行状态</p>
            </div>
          </div>
          <div class="auto-detect-control">
            <label class="auto-detect-label">
              <input
                type="checkbox"
                :checked="analysisLogEnabled"
                :disabled="analysisLogSaving"
                @change="handleAnalysisLogChange"
              >
              <span>{{ analysisLogEnabled ? '已开启' : '关闭' }}</span>
            </label>
          </div>
          <p v-if="analysisLogEnabled && analysisLogPath" class="setting-desc analysis-log-path">
            {{ analysisLogPath }}
          </p>
        </div>

        <!-- 分割线 -->
        <div class="setting-divider"></div>

        <!-- 网卡选择 -->
        <div class="setting-section">
          <div class="section-header">
            <div>
              <h4>网卡选择</h4>
              <p class="setting-desc">手动选择用于抓包的网卡（留空则自动检测）</p>
            </div>
            <van-tag v-if="!loading" type="primary" plain>
              共 {{ nics.length }} 个
              <template v-if="searchKeyword && filteredNics.length !== nics.length">
                / 显示 {{ filteredNics.length }} 个
              </template>
            </van-tag>
          </div>

          <!-- 搜索框 -->
          <van-search
            v-if="!loading"
            v-model="searchKeyword"
            placeholder="搜索网卡名称或描述..."
            show-action
            clearable
            class="nic-search"
          >
            <template #action>
              <span v-if="searchKeyword" @click="searchKeyword = ''">清除</span>
            </template>
          </van-search>

          <!-- 加载状态 -->
          <div v-if="loading" class="loading-wrapper">
            <van-loading size="24px" vertical>加载中...</van-loading>
          </div>

          <!-- 网卡列表 -->
          <van-radio-group v-else v-model="selectedNic" class="nic-list">
            <!-- 自动检测选项 -->
            <van-cell-group inset class="nic-cell-group">
              <van-cell clickable @click="selectedNic = ''">
                <template #title>
                  <div class="nic-info">
                    <span class="nic-name">自动检测</span>
                    <span class="nic-desc">让程序自动查找合适的网卡</span>
                  </div>
                </template>
                <template #right-icon>
                  <van-radio name="" />
                </template>
              </van-cell>

              <!-- 网卡选项 -->
              <van-cell
                v-for="nic in filteredNics"
                :key="nic.name"
                clickable
                @click="selectedNic = nic.name"
              >
                <template #title>
                  <div class="nic-info">
                    <span class="nic-name">{{ nic.description || nic.name }}</span>
                    <span class="nic-desc">{{ nic.name }}</span>
                  </div>
                </template>
                <template #right-icon>
                  <van-radio :name="nic.name" />
                </template>
              </van-cell>
            </van-cell-group>
          </van-radio-group>
        </div>
      </div>
    </div>
  </van-popup>
</template>

<style scoped lang="scss">
/**
 * 高级设置对话框样式
 * 使用 Vant 组件，自定义暗色主题
 */

.advanced-settings-dialog {
  background: rgba(40, 40, 40, 0.98);
  display: flex;
  flex-direction: column;
  max-height: 80vh;
}

.dialog-header {
  padding: 16px 20px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  
  h3 {
    margin: 0;
    font-size: 16px;
    color: #fff;
  }
}

.dialog-content {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
}

.setting-section {
  margin-bottom: 0;
  padding-bottom: 16px;
}

.setting-divider {
  height: 1px;
  background: rgba(255, 255, 255, 0.1);
  margin: 16px 0;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 12px;
  gap: 16px;
  
  h4 {
    margin: 0 0 6px 0;
    font-size: 14px;
    color: #fff;
    font-weight: 600;
  }
}

.setting-desc {
  margin: 0;
  font-size: 12px;
  color: #aaa;
  line-height: 1.5;
}

.analysis-log-path {
  overflow-wrap: anywhere;
  user-select: text;
}

.refresh-rate-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.refresh-rate-field {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 6px;
  color: #ccc;
  font-size: 12px;

  select {
    width: 100%;
    height: 32px;
    padding: 0 8px;
    border: 1px solid rgba(255, 255, 255, 0.2);
    border-radius: 4px;
    outline: none;
    background: rgba(50, 50, 50, 0.9);
    color: #fff;
    font-size: 12px;

    &:disabled {
      opacity: 0.55;
    }
  }
}

.refresh-rate-warning {
  margin: 10px 0 0;
  padding: 9px 10px;
  border-left: 3px solid #ffb74d;
  background: rgba(255, 183, 77, 0.08);
  color: #d0b88f;
  font-size: 11px;
  line-height: 1.55;
}

@media (max-width: 480px) {
  .refresh-rate-grid {
    grid-template-columns: 1fr;
  }
}

// 搜索框样式
.nic-search {
  margin-bottom: 12px;
  
  :deep(.van-search__content) {
    background: rgba(50, 50, 50, 0.8);
    border: 1px solid rgba(255, 255, 255, 0.1);
    
    .van-field__control {
      color: #fff;
      
      &::placeholder {
        color: #666;
      }
    }
  }
  
  :deep(.van-search__action) {
    color: #42a5f5;
    font-size: 12px;
    padding-left: 8px;
  }
}

// 加载状态
.loading-wrapper {
  padding: 40px 20px;
  text-align: center;
}

// 网卡列表
.nic-list {
  max-height: 350px;
  overflow-y: auto;
  
  &::-webkit-scrollbar {
    width: 6px;
  }

  &::-webkit-scrollbar-track {
    background: rgba(0, 0, 0, 0.2);
    border-radius: 3px;
  }

  &::-webkit-scrollbar-thumb {
    background: rgba(255, 255, 255, 0.2);
    border-radius: 3px;
    
    &:hover {
      background: rgba(255, 255, 255, 0.3);
    }
  }
}

// 网卡单元格组
.nic-cell-group {
  margin: 0;
  
  :deep(.van-cell-group--inset) {
    margin: 0;
  }
  
  :deep(.van-cell) {
    background: rgba(50, 50, 50, 0.6);
    padding: 10px 12px;
  }
  
  :deep(.van-cell::after) {
    border-color: rgba(255, 255, 255, 0.08);
  }
  
  :deep(.van-cell:hover) {
    background: rgba(60, 60, 60, 0.8);
  }
  
  :deep(.van-cell:first-child) {
    border-radius: 8px 8px 0 0;
  }
  
  :deep(.van-cell:last-child) {
    border-radius: 0 0 8px 8px;
  }
  
  :deep(.van-cell:last-child::after) {
    display: none;
  }
  
  :deep(.van-cell:only-child) {
    border-radius: 8px;
  }
}

// 网卡信息
.nic-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.nic-name {
  font-size: 13px;
  color: #fff;
  font-weight: 500;
  line-height: 1.4;
}

.nic-desc {
  font-size: 11px;
  color: #888;
  word-break: break-all;
  line-height: 1.4;
}

// 单选按钮样式
:deep(.van-radio) {
  .van-radio__icon {
    font-size: 18px;
    
    .van-icon {
      border-color: rgba(255, 255, 255, 0.3);
      background: transparent;
    }
  }
  
  &.van-radio--checked .van-radio__icon .van-icon {
    background: #42a5f5;
    border-color: #42a5f5;
  }
}

// 透明度控制
.opacity-control {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 0;
}

#opacitySlider {
  flex: 1;
  height: 6px;
  -webkit-appearance: none;
  appearance: none;
  background: rgba(255, 255, 255, 0.2);
  border-radius: 3px;
  outline: none;
  cursor: pointer;

  &::-webkit-slider-thumb {
    -webkit-appearance: none;
    appearance: none;
    width: 16px;
    height: 16px;
    background: #42a5f5;
    border-radius: 50%;
    cursor: pointer;
    transition: background 0.2s;

    &:hover {
      background: #1e88e5;
    }
  }

  &::-moz-range-thumb {
    width: 16px;
    height: 16px;
    background: #42a5f5;
    border: none;
    border-radius: 50%;
    cursor: pointer;
  }
}

.opacity-value {
  font-size: 13px;
  color: #fff;
  font-weight: 500;
  min-width: 45px;
  text-align: right;
}

// 自动检测控制
.auto-detect-control {
  margin-bottom: 12px;
}

.auto-detect-label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: #fff;
  cursor: pointer;
  padding: 8px 0;

  input[type="checkbox"] {
    width: 16px;
    height: 16px;
    margin: 0;
    cursor: pointer;
    accent-color: #42a5f5;
  }

  &:hover {
    color: #42a5f5;
  }
}

.ranking-control {
  margin-top: 4px;
  padding-top: 12px;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
}

.ranking-control-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 32px;
}

.ranking-control-copy {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 2px;
}

.ranking-control-title {
  color: #fff;
  font-size: 13px;
  line-height: 1.4;
  overflow-wrap: anywhere;
}

.ranking-control-status {
  color: #999;
  font-size: 11px;
  line-height: 1.4;
}

.ranking-mode-select {
  width: 126px;
  min-width: 0;
  height: 30px;
  padding: 0 8px;
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 4px;
  outline: none;
  background: rgba(50, 50, 50, 0.9);
  color: #fff;
  font-size: 12px;
  cursor: pointer;

  &:disabled {
    opacity: 0.55;
    cursor: not-allowed;
  }
}

.ranking-consent-note {
  margin: 8px 0 0;
  color: #aaa;
  font-size: 11px;
  line-height: 1.55;
}

.ranking-unavailable-reason {
  margin: 8px 0 0;
  color: #ffb74d;
  font-size: 11px;
  line-height: 1.5;
}

.ranking-retry-button {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  margin-top: 8px;
  padding: 5px 8px;
  border: 1px solid rgba(66, 165, 245, 0.5);
  border-radius: 4px;
  background: transparent;
  color: #64b5f6;
  font-size: 11px;
  cursor: pointer;

  &:hover {
    background: rgba(66, 165, 245, 0.1);
  }
}

// 频道选择控制
.channel-select-control {
  margin-top: 12px;
}

.channel-select-wrapper {
  position: relative;
}

.channel-select-trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 4px;
  background: rgba(50, 50, 50, 0.6);
  color: #fff;
  cursor: pointer;
  font-size: 13px;
  user-select: none;

  &:hover {
    border-color: rgba(66, 165, 245, 0.5);
    background: rgba(60, 60, 60, 0.8);
  }

  &.disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
}

.select-arrow {
  font-size: 10px;
  color: #888;
  margin-left: 8px;
}

.channel-select-dropdown {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  right: 0;
  max-height: 300px;
  overflow-y: auto;
  background: rgba(30, 30, 30, 0.98);
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 4px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.5);
  z-index: 1001;

  &::-webkit-scrollbar {
    width: 6px;
  }

  &::-webkit-scrollbar-track {
    background: rgba(0, 0, 0, 0.2);
    border-radius: 3px;
  }

  &::-webkit-scrollbar-thumb {
    background: rgba(255, 255, 255, 0.2);
    border-radius: 3px;
    
    &:hover {
      background: rgba(255, 255, 255, 0.3);
    }
  }
}

.channel-server-group {
  padding: 4px 0;
}

.channel-server-name {
  padding: 6px 12px;
  font-size: 12px;
  color: #ffc107;
  font-weight: 600;
  background: rgba(255, 193, 7, 0.1);
}

.channel-item {
  padding: 6px 24px;
  font-size: 13px;
  color: #ddd;
  cursor: pointer;

  &:hover {
    background: rgba(66, 165, 245, 0.2);
    color: #fff;
  }

  &.active {
    background: rgba(66, 165, 245, 0.3);
    color: #42a5f5;
    font-weight: 500;
  }
}

// 加速器选择控制
.accelerator-select-control {
  margin-top: 12px;
}

.accelerator-select-wrapper {
  position: relative;
}

.accelerator-select-trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 4px;
  background: rgba(50, 50, 50, 0.6);
  color: #fff;
  cursor: pointer;
  font-size: 13px;
  user-select: none;

  &:hover {
    border-color: rgba(66, 165, 245, 0.5);
    background: rgba(60, 60, 60, 0.8);
  }
}

.accelerator-select-dropdown {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  right: 0;
  max-height: 200px;
  overflow-y: auto;
  background: rgba(30, 30, 30, 0.98);
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 4px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.5);
  z-index: 1001;

  &::-webkit-scrollbar {
    width: 6px;
  }

  &::-webkit-scrollbar-track {
    background: rgba(0, 0, 0, 0.2);
    border-radius: 3px;
  }

  &::-webkit-scrollbar-thumb {
    background: rgba(255, 255, 255, 0.2);
    border-radius: 3px;
    
    &:hover {
      background: rgba(255, 255, 255, 0.3);
    }
  }
}

.accelerator-item {
  padding: 8px 12px;
  font-size: 13px;
  color: #ddd;
  cursor: pointer;

  &:hover {
    background: rgba(66, 165, 245, 0.2);
    color: #fff;
  }

  &.active {
    background: rgba(66, 165, 245, 0.3);
    color: #42a5f5;
    font-weight: 500;
  }
}

</style>
