<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import * as echarts from 'echarts'
import { showNotify } from 'vant'
import SvgIcon from '@jamescoyle/vue-icon'
import {
  mdiCheckCircleOutline,
  mdiClockOutline,
  mdiLeaf,
  mdiLightningBolt,
  mdiCashMultiple,
  mdiSprout,
  mdiStarFourPoints,
  mdiWateringCanOutline,
} from '@mdi/js'
import * as api from '../composables/useApi'
import FarmRecommendationView from './FarmRecommendationView.vue'
import FarmStorageView from './FarmStorageView.vue'

const farmItemIcons = import.meta.glob('../assets/farm-items/*.png', { eager: true, query: '?url', import: 'default' }) as Record<string, string>

const activeFarmTab = ref<'reminder' | 'recommendation' | 'storage' | 'statistics'>('reminder')

const emptyState = (): FarmState => ({
  enabled: false,
  readyNotificationEnabled: true,
  specialNotificationEnabled: true,
  fertility: 0,
  fertilityMax: 100,
  fertilityKnown: false,
  energy: 0,
  energyMax: 100,
  energyKnown: false,
  synced: false,
  updatedAt: 0,
  accountId: '',
  coinsSpent: 0,
  energySpent: 0,
  todayCoinsSpent: 0,
  todayEnergySpent: 0,
  todayLifeCoinsAdded: 0,
  todayLifeCoinsSpent: 0,
  lifeCoinsAdded: 0,
  lifeCoinsSpent: 0,
  lifeCoinsBalance: 0,
  lifeCoinsKnown: false,
  cropCounts: {},
  todayCropCounts: {},
  yesterdayCropCounts: {},
  dailyTrend: [],
  plots: [],
})

const numberFormatter = new Intl.NumberFormat('zh-CN')

const farmState = ref<FarmState>(emptyState())
const trendDays = ref<7 | 30>(7)
const knownCropNames = ['黑莓', '秋葵', '茉莉', '红梨树', '橡胶树', '蜘蛛古木', '石英矿脉']
const selectedTrendKeys = ref<string[]>(['coins', 'energy', ...knownCropNames])
const toggling = ref(false)
const readyNotificationToggling = ref(false)
const specialNotificationToggling = ref(false)
let updateInterval: number | null = null

const enabled = computed(() => farmState.value.enabled)
const cropIcons: Record<string, string> = {
  黑莓: 'item_seasonalfarming_06_s', 秋葵: 'item_seasonalfarming_05_s', 茉莉: 'item_seasonalfarming_07_s',
  红梨树: 'item_seasonalfarming_08_s', 橡胶树: 'item_seasonalfarming_09_s', 蜘蛛古木: 'item_seasonalfarming_04_s', 石英矿脉: 'item_seasonalfarming_10_s',
}
const cropSummaryRows = computed(() => knownCropNames.map(name => {
  const total = farmState.value.cropCounts?.[name] || 0
  const value = farmState.value.todayCropCounts?.[name] || 0
  const diff = (farmState.value.todayCropCounts?.[name] || 0) - (farmState.value.yesterdayCropCounts?.[name] || 0)
  return { name, value, total, diff, icon: farmItemIcons[`../assets/farm-items/${cropIcons[name]}.png`] || '' }
}))
function dailyDiffLabel(diff: number): string {
  if (diff === 0) return '—'
  return `${diff > 0 ? '↑' : '↓'}${numberFormatter.format(Math.abs(diff))}`
}

const selectedTrend = computed(() => (farmState.value.dailyTrend || []).slice(-trendDays.value))
const cropColors = ['#66bb6a', '#8bc34a', '#26a69a', '#ab87ff', '#ff9f68', '#e57373', '#4dd0e1']
const trendSeries = [
  { key: 'coins', label: '金币消耗', color: '#d4a72c', value: (day: FarmDailyUsage) => day.coinsSpent || 0 },
  { key: 'energy', label: '能量消耗', color: '#42a5f5', value: (day: FarmDailyUsage) => day.energySpent || 0 },
  ...knownCropNames.map((name, index) => ({ key: name, label: name, color: cropColors[index], value: (day: FarmDailyUsage) => day.cropCounts?.[name] || 0 })),
]
function toggleTrend(key: string) {
  if (selectedTrendKeys.value.includes(key)) {
    if (selectedTrendKeys.value.length > 1) selectedTrendKeys.value = selectedTrendKeys.value.filter(item => item !== key)
  } else selectedTrendKeys.value = [...selectedTrendKeys.value, key]
}

const trendChartRef = ref<HTMLElement | null>(null)
let trendChart: echarts.ECharts | null = null
let trendResizeObserver: ResizeObserver | null = null
const lifeCoinChartRef = ref<HTMLElement | null>(null)
let lifeCoinChart: echarts.ECharts | null = null
let lifeCoinResizeObserver: ResizeObserver | null = null

function disposeTrendChart() {
  trendResizeObserver?.disconnect()
  trendResizeObserver = null
  trendChart?.dispose()
  trendChart = null
}

function renderTrendChart() {
  const element = trendChartRef.value
  if (!element) {
    disposeTrendChart()
    return
  }
  if (trendChart && trendChart.getDom() !== element) disposeTrendChart()
  if (!trendChart) {
    trendChart = echarts.init(element, undefined, { renderer: 'canvas' })
    if (typeof ResizeObserver !== 'undefined') {
      trendResizeObserver = new ResizeObserver(() => resizeTrendChart())
      trendResizeObserver.observe(element)
    }
  }
  const days = selectedTrend.value
  const enabled = new Set(selectedTrendKeys.value)
  const cropSeries = knownCropNames.filter(name => enabled.has(name)).map((name, index) => ({
    name, type: 'bar' as const, stack: 'crops', yAxisIndex: 1,
    barMaxWidth: 18, itemStyle: { color: cropColors[index] },
    data: days.map(day => day.cropCounts?.[name] || 0),
  }))
  const series: echarts.SeriesOption[] = [
    ...(enabled.has('coins') ? [{ name: '金币消耗', type: 'line' as const, smooth: true, symbol: 'circle', symbolSize: 5, yAxisIndex: 0, lineStyle: { width: 2, color: '#d4a72c' }, itemStyle: { color: '#d4a72c' }, data: days.map(day => day.coinsSpent || 0) }] : []),
    ...(enabled.has('energy') ? [{ name: '能量消耗', type: 'line' as const, smooth: true, symbol: 'circle', symbolSize: 5, yAxisIndex: 1, lineStyle: { width: 2, color: '#42a5f5' }, itemStyle: { color: '#42a5f5' }, data: days.map(day => day.energySpent || 0) }] : []),
    ...cropSeries,
  ]
  trendChart.setOption({
    animation: false,
    color: ['#d4a72c', '#42a5f5', ...cropColors],
    grid: { left: 42, right: 12, top: 24, bottom: 28 },
    tooltip: { trigger: 'axis' },
    legend: { show: false },
    xAxis: { type: 'category', data: days.map(day => day.date.slice(5)), axisLabel: { color: '#777', fontSize: 9 }, axisLine: { lineStyle: { color: 'rgba(255,255,255,.15)' } } },
    yAxis: [
      { type: 'value', axisLabel: { color: '#888', fontSize: 9 }, splitLine: { lineStyle: { color: 'rgba(255,255,255,.06)' } } },
      { type: 'value', axisLabel: { color: '#888', fontSize: 9 }, splitLine: { show: false } },
    ],
    series,
  } as echarts.EChartsOption, true)
}
function resizeTrendChart() { trendChart?.resize() }

function disposeLifeCoinChart() {
  lifeCoinResizeObserver?.disconnect()
  lifeCoinResizeObserver = null
  lifeCoinChart?.dispose()
  lifeCoinChart = null
}

function renderLifeCoinChart() {
  const element = lifeCoinChartRef.value
  if (!element) {
    disposeLifeCoinChart()
    return
  }
  if (lifeCoinChart && lifeCoinChart.getDom() !== element) disposeLifeCoinChart()
  if (!lifeCoinChart) {
    lifeCoinChart = echarts.init(element, undefined, { renderer: 'canvas' })
    if (typeof ResizeObserver !== 'undefined') {
      lifeCoinResizeObserver = new ResizeObserver(() => resizeLifeCoinChart())
      lifeCoinResizeObserver.observe(element)
    }
  }
  const days = selectedTrend.value
  lifeCoinChart.setOption({
    animation: false,
    grid: { left: 42, right: 12, top: 16, bottom: 28 },
    tooltip: { trigger: 'axis' },
    legend: { show: false },
    xAxis: { type: 'category', data: days.map(day => day.date.slice(5)), axisLabel: { color: '#777', fontSize: 9 }, axisLine: { lineStyle: { color: 'rgba(255,255,255,.15)' } } },
    yAxis: { type: 'value', axisLabel: { color: '#888', fontSize: 9 }, splitLine: { lineStyle: { color: 'rgba(255,255,255,.06)' } } },
    series: [
      { name: '获得', type: 'bar', barMaxWidth: 18, itemStyle: { color: '#66bb6a' }, data: days.map(day => day.lifeCoinsAdded || 0) },
      { name: '消耗', type: 'bar', barMaxWidth: 18, itemStyle: { color: '#ef7070' }, data: days.map(day => -(day.lifeCoinsSpent || 0)) },
    ],
  } as echarts.EChartsOption, true)
}
function resizeLifeCoinChart() { lifeCoinChart?.resize() }

function clampPercent(value: number, maximum: number): number {
  if (maximum <= 0) return 0
  return Math.max(0, Math.min(100, (value / maximum) * 100))
}

function formatTime(seconds = 0): string {
  if (seconds <= 0) return '00:00'
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const remainder = seconds % 60
  if (hours > 0) {
    return `${hours}:${minutes.toString().padStart(2, '0')}:${remainder.toString().padStart(2, '0')}`
  }
  return `${minutes.toString().padStart(2, '0')}:${remainder.toString().padStart(2, '0')}`
}

function phaseLabel(plot: FarmPlotState): string {
  if (plot.ready) return '可收获'
  const labels: Record<string, string> = {
    seed: '幼苗',
    bud: '萌芽',
    grow: '生长',
    normal: '生长',
    empty: '幼苗',
    growing: '生长中',
  }
  return labels[plot.phase || ''] || '生长中'
}

function qualityLabel(plot: FarmPlotState): string {
  const labels: Record<string, string> = {
    empty: '未种植',
    normal: '普通',
    advanced: '高级',
    highest: '最高级',
  }
  return labels[plot.quality] || '普通'
}

function emptyCropLabel(plot: FarmPlotState): string {
  if (plot.kind === 'field') return '黑莓 · 秋葵 · 茉莉'
  return '未种植'
}

async function loadFarmState() {
  try {
    farmState.value = await api.getFarmState()
  } catch (error) {
    console.error('加载农场状态失败:', error)
  }
}

async function setEnabled(value: boolean) {
  if (toggling.value) return
  toggling.value = true
  try {
    farmState.value = await api.setFarmMonitorEnabled(value)
  } catch (error) {
    console.error('设置农场提醒开关失败:', error)
    await loadFarmState()
  } finally {
    toggling.value = false
  }
}

async function setReadyNotificationEnabled(value: boolean) {
  if (readyNotificationToggling.value) return
  readyNotificationToggling.value = true
  try {
    farmState.value = await api.setFarmReadyNotificationEnabled(value)
  } catch (error) {
    console.error('设置成熟提醒开关失败:', error)
    await loadFarmState()
  } finally {
    readyNotificationToggling.value = false
  }
}

async function setSpecialNotificationEnabled(value: boolean) {
  if (specialNotificationToggling.value) return
  specialNotificationToggling.value = true
  try {
    farmState.value = await api.setFarmSpecialNotificationEnabled(value)
  } catch (error) {
    console.error('设置高级种子提醒开关失败:', error)
    await loadFarmState()
  } finally {
    specialNotificationToggling.value = false
  }
}

const onFarmState = (state: FarmState) => {
  const latest = state.dailyTrend?.[state.dailyTrend.length - 1]
  farmState.value = {
    ...state,
    todayCoinsSpent: state.todayCoinsSpent ?? latest?.coinsSpent ?? 0,
    todayEnergySpent: state.todayEnergySpent ?? latest?.energySpent ?? 0,
    todayLifeCoinsAdded: state.todayLifeCoinsAdded ?? latest?.lifeCoinsAdded ?? 0,
    todayLifeCoinsSpent: state.todayLifeCoinsSpent ?? latest?.lifeCoinsSpent ?? 0,
  }
}

const onFarmReady = (plot: FarmPlotState) => {
  showNotify({ type: 'success', message: `${plot.cropName || plot.label}已经成熟`, duration: 5000 })
  void loadFarmState()
}

const onFarmSpecial = (plot: FarmPlotState) => {
  showNotify({ type: 'warning', message: `${plot.cropName || plot.label}是一颗高级种子`, duration: 5000 })
  void loadFarmState()
}

onMounted(async () => {
  api.onEvent('farm-state', onFarmState)
  api.onEvent('farm-ready', onFarmReady)
  api.onEvent('farm-special', onFarmSpecial)
  await loadFarmState()
  await nextTick()
  renderTrendChart()
  renderLifeCoinChart()
  window.addEventListener('resize', resizeTrendCharts)
  updateInterval = window.setInterval(loadFarmState, 1000)
})

function resizeTrendCharts() {
  resizeTrendChart()
  resizeLifeCoinChart()
}

watch([selectedTrend, selectedTrendKeys, trendDays], () => { void nextTick().then(() => { renderTrendChart(); renderLifeCoinChart() }) }, { deep: true })
watch(activeFarmTab, tab => {
  if (tab === 'statistics') {
    void nextTick().then(() => { renderTrendChart(); renderLifeCoinChart() })
  } else {
    disposeTrendChart()
    disposeLifeCoinChart()
  }
})

onUnmounted(() => {
  api.offEvent('farm-state', onFarmState)
  api.offEvent('farm-ready', onFarmReady)
  api.offEvent('farm-special', onFarmSpecial)
  if (updateInterval !== null) clearInterval(updateInterval)
  window.removeEventListener('resize', resizeTrendCharts)
  disposeTrendChart()
  disposeLifeCoinChart()
})
</script>

<template>
  <div class="farm-view">
    <div class="farm-tabs" role="tablist" aria-label="塔汀农场功能">
      <button
        type="button"
        role="tab"
        class="farm-tab"
        :class="{ active: activeFarmTab === 'reminder' }"
        :aria-selected="activeFarmTab === 'reminder'"
        @click="activeFarmTab = 'reminder'"
      >
        农场提醒
      </button>
      <button
        type="button"
        role="tab"
        class="farm-tab"
        :class="{ active: activeFarmTab === 'recommendation' }"
        :aria-selected="activeFarmTab === 'recommendation'"
        @click="activeFarmTab = 'recommendation'"
      >
        推荐订单
      </button>
      <button
        type="button"
        role="tab"
        class="farm-tab"
        :class="{ active: activeFarmTab === 'storage' }"
        :aria-selected="activeFarmTab === 'storage'"
        @click="activeFarmTab = 'storage'"
      >
        储存库
      </button>
      <button
        type="button"
        role="tab"
        class="farm-tab"
        :class="{ active: activeFarmTab === 'statistics' }"
        :aria-selected="activeFarmTab === 'statistics'"
        @click="activeFarmTab = 'statistics'"
      >
        统计
      </button>
    </div>

    <div v-if="activeFarmTab === 'reminder'" class="reminder-view">
    <div class="feature-row">
      <div class="feature-name">
        <svg-icon type="mdi" :path="mdiSprout" :size="18" />
        <span>农场提醒</span>
        <span class="feature-state" :class="{ active: enabled }">{{ enabled ? '已开启' : '已关闭' }}</span>
      </div>
      <van-switch
        :model-value="enabled"
        :loading="toggling"
        size="18px"
        @update:model-value="setEnabled"
      />
    </div>

    <div class="notification-controls" :class="{ disabled: !enabled }">
      <label class="notification-toggle">
        <span>可收获提醒</span>
        <van-switch
          :model-value="farmState.readyNotificationEnabled"
          :loading="readyNotificationToggling"
          :disabled="!enabled"
          size="15px"
          @update:model-value="setReadyNotificationEnabled"
        />
      </label>
      <label class="notification-toggle">
        <span>高级种子提醒</span>
        <van-switch
          :model-value="farmState.specialNotificationEnabled"
          :loading="specialNotificationToggling"
          :disabled="!enabled"
          size="15px"
          @update:model-value="setSpecialNotificationEnabled"
        />
      </label>
    </div>

    <div class="farm-content" :class="{ disabled: !enabled }">
      <div class="resource-grid">
        <div class="resource-meter fertility-meter">
          <div class="resource-line">
            <span class="resource-name">
              <svg-icon type="mdi" :path="mdiLeaf" :size="15" />
              肥沃度
            </span>
            <span class="resource-value">{{ farmState.fertilityKnown ? `${farmState.fertility}/${farmState.fertilityMax}` : '--/100' }}</span>
          </div>
          <div class="resource-track">
            <div class="resource-fill" :style="{ width: `${clampPercent(farmState.fertility, farmState.fertilityMax)}%` }" />
          </div>
        </div>

        <div class="resource-meter energy-meter">
          <div class="resource-line">
            <span class="resource-name">
              <svg-icon type="mdi" :path="mdiLightningBolt" :size="15" />
              能量
            </span>
            <span class="resource-value">{{ farmState.energyKnown ? `${farmState.energy}/${farmState.energyMax}` : '--/100' }}</span>
          </div>
          <div class="resource-track">
            <div class="resource-fill" :style="{ width: `${clampPercent(farmState.energy, farmState.energyMax)}%` }" />
          </div>
        </div>
      </div>

      <div class="farm-grid">
        <div
          v-for="plot in farmState.plots"
          :key="plot.index"
          class="farm-plot"
          :class="[`quality-${plot.quality}`, { special: plot.special, ready: plot.ready }]"
        >
          <div class="plot-header">
            <span class="plot-label">{{ plot.label }}</span>
            <span v-if="plot.special" class="special-mark" title="高级种子">
              <svg-icon type="mdi" :path="mdiStarFourPoints" :size="12" />
              高级
            </span>
            <span v-else class="quality-name">{{ qualityLabel(plot) }}</span>
          </div>

          <template v-if="plot.planted">
            <div class="crop-line">
              <span class="crop-name">{{ plot.cropName }}</span>
              <span class="phase-name" :class="{ ready: plot.ready }">{{ phaseLabel(plot) }}</span>
            </div>

            <div class="crop-time" :class="{ ready: plot.ready }">
              <svg-icon type="mdi" :path="plot.ready ? mdiCheckCircleOutline : mdiClockOutline" :size="14" />
              {{ plot.ready ? '可收获' : formatTime(plot.remainingSeconds) }}
            </div>

            <div class="plot-footer">
              <span class="support-score">
                <svg-icon type="mdi" :path="mdiWateringCanOutline" :size="13" />
                {{ plot.support }}
              </span>
              <span v-if="plot.fertility" class="fertile-mark">肥沃</span>
            </div>

            <div class="growth-track">
              <div class="growth-fill" :style="{ width: `${plot.progress}%` }" />
            </div>
          </template>

          <div v-else class="empty-plot">
            <svg-icon type="mdi" :path="mdiSprout" :size="20" />
            <span>{{ emptyCropLabel(plot) }}</span>
          </div>
        </div>
      </div>
    </div>

      <div v-if="!enabled" class="disabled-state">功能已关闭</div>
    </div>

    <FarmRecommendationView v-else-if="activeFarmTab === 'recommendation'" />
    <FarmStorageView v-else-if="activeFarmTab === 'storage'" />
    <div v-else class="statistics-view">
      <div class="association-coins-row">
        <span class="association-coins-label">生活协会硬币</span>
        <div class="association-coins-values">
          <span class="association-coins-balance">{{ farmState.lifeCoinsKnown ? numberFormatter.format(farmState.lifeCoinsBalance || 0) : '--' }}</span>
          <span class="association-coins-added">+{{ numberFormatter.format(farmState.todayLifeCoinsAdded || 0) }}</span>
          <span class="association-coins-spent">-{{ numberFormatter.format(farmState.todayLifeCoinsSpent || 0) }}</span>
        </div>
      </div>
      <div class="statistics-grid">
        <div class="statistic-item coins primary-stat">
          <span class="statistic-icon"><svg-icon type="mdi" :path="mdiCashMultiple" :size="22" /></span>
          <span class="statistic-label">今日消耗金币</span>
          <div class="statistic-value"><strong>{{ numberFormatter.format(farmState.todayCoinsSpent || 0) }}</strong></div>
          <small class="statistic-total">累计 {{ numberFormatter.format(farmState.coinsSpent || 0) }}</small>
        </div>
        <div class="statistic-item energy primary-stat">
          <span class="statistic-icon"><svg-icon type="mdi" :path="mdiLightningBolt" :size="22" /></span>
          <span class="statistic-label">今日消耗能量</span>
          <div class="statistic-value"><strong>{{ numberFormatter.format(farmState.todayEnergySpent || 0) }}</strong></div>
          <small class="statistic-total">累计 {{ numberFormatter.format(farmState.energySpent || 0) }}</small>
        </div>
        <div v-for="(row, index) in cropSummaryRows" :key="row.name" class="statistic-item crop-summary" :class="index < 3 ? 'field-crop' : 'tree-crop'">
          <span class="statistic-icon crop-icon"><img v-if="row.icon" :src="row.icon" :alt="row.name"></span>
          <span class="statistic-label">{{ row.name }}</span>
          <div class="statistic-value"><strong>{{ numberFormatter.format(row.value) }}</strong>
          <small class="daily-change" :class="{ up: row.diff > 0, down: row.diff < 0 }">{{ dailyDiffLabel(row.diff) }}</small>
          </div>
          <small class="statistic-total">累计 {{ numberFormatter.format(row.total) }}</small>
        </div>
      </div>
      <section class="trend-section">
        <div class="section-title">消耗走势
          <div class="trend-switch">
            <button type="button" :class="{ active: trendDays === 7 }" @click="trendDays = 7">7天</button>
            <button type="button" :class="{ active: trendDays === 30 }" @click="trendDays = 30">30天</button>
          </div>
        </div>
        <div v-if="selectedTrend.length" class="trend-chart-wrap">
          <div class="trend-options">
            <button v-for="series in trendSeries" :key="series.key" type="button" :class="{ selected: selectedTrendKeys.includes(series.key) }" @click="toggleTrend(series.key)">
              <i :style="{ backgroundColor: series.color }" />{{ series.label }}
            </button>
          </div>
          <div ref="trendChartRef" class="trend-chart" role="img" aria-label="农场消耗走势" />
          <div class="chart-labels"><span>{{ selectedTrend[0].date.slice(5) }}</span><span>{{ selectedTrend[selectedTrend.length - 1].date.slice(5) }}</span></div>
        </div>
        <div v-else class="empty-stat">暂无走势数据</div>
      </section>
      <section class="trend-section life-coin-trend-section">
        <div class="section-title">生活协会硬币走势
          <div class="trend-switch">
            <button type="button" :class="{ active: trendDays === 7 }" @click="trendDays = 7">7天</button>
            <button type="button" :class="{ active: trendDays === 30 }" @click="trendDays = 30">30天</button>
          </div>
        </div>
        <div v-if="selectedTrend.length" class="trend-chart-wrap">
          <div class="life-coin-legend"><span><i class="life-coin-positive" />获得</span><span><i class="life-coin-negative" />消耗</span></div>
          <div ref="lifeCoinChartRef" class="trend-chart" role="img" aria-label="生活协会硬币获得与消耗走势" />
          <div class="chart-labels"><span>{{ selectedTrend[0].date.slice(5) }}</span><span>{{ selectedTrend[selectedTrend.length - 1].date.slice(5) }}</span></div>
        </div>
        <div v-else class="empty-stat">暂无生活协会硬币数据</div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.farm-view {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-height: 100%;
  padding: 0 4px 4px;
  color: #e8e8e8;
}

.farm-tabs {
  height: 34px;
  min-height: 34px;
  display: flex;
  align-items: stretch;
  gap: 2px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

.farm-tab {
  min-width: 76px;
  padding: 0 10px;
  position: relative;
  border: 0;
  color: #888;
  background: transparent;
  font-family: inherit;
  font-size: 11px;
  cursor: pointer;
}

.farm-tab:hover {
  color: #ccc;
  background: rgba(255, 255, 255, 0.04);
}

.farm-tab.active {
  color: #fff;
  font-weight: 500;
}

.farm-tab.active::after {
  content: '';
  position: absolute;
  right: 10px;
  bottom: 0;
  left: 10px;
  height: 2px;
  background: #66bb6a;
}

.reminder-view {
  position: relative;
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.feature-row {
  min-height: 34px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 6px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

.feature-name {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 12px;
  font-weight: 600;
}

.feature-name > svg {
  color: #66bb6a;
}

.feature-state {
  color: #777;
  font-size: 10px;
  font-weight: 500;
}

.feature-state.active {
  color: #66bb6a;
}

.notification-controls {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  transition: opacity 0.2s ease;
}

.notification-controls.disabled {
  opacity: 0.35;
}

.notification-toggle {
  min-width: 0;
  min-height: 27px;
  padding: 0 7px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  color: #aaa;
  background: rgba(255, 255, 255, 0.035);
  border: 1px solid rgba(255, 255, 255, 0.09);
  border-radius: 5px;
  font-size: 10px;
}

.farm-content {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
  transition: opacity 0.2s ease;
}

.farm-content.disabled {
  opacity: 0.24;
  pointer-events: none;
  filter: grayscale(0.8);
}

.resource-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.statistics-view {
  flex: 1;
  min-height: 0;
  padding: 8px 6px;
  overflow-y: auto;
}

.statistics-grid {
  display: grid;
  grid-template-columns: repeat(12, minmax(0, 1fr));
  gap: 12px;
}

.association-coins-row {
  min-height: 54px;
  margin-bottom: 12px;
  padding: 10px 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  background: rgba(255, 255, 255, 0.035);
  border: 1px solid rgba(255, 255, 255, 0.09);
  border-left: 3px solid #d4a72c;
  border-radius: 5px;
}

.association-coins-label,
.association-coins-values {
  color: #bbb;
  font-size: 14px;
  line-height: 1.2;
  white-space: nowrap;
}

.association-coins-values {
  display: flex;
  align-items: baseline;
  justify-content: flex-end;
  gap: 16px;
  color: #f5f5f5;
  font-family: Consolas, monospace;
}

.association-coins-balance,
.association-coins-added,
.association-coins-spent {
  font-size: 16px;
}

.association-coins-added { color: #66bb6a; }
.association-coins-spent { color: #ef7070; }

.statistic-item {
  min-width: 0;
  height: 92px;
  padding: 14px 16px;
  display: grid;
  grid-template-columns: 30px minmax(0, 1fr);
  grid-template-rows: auto 1fr auto;
  align-items: center;
  column-gap: 8px;
  background: rgba(255, 255, 255, 0.035);
  border: 1px solid rgba(255, 255, 255, 0.09);
  border-left: 3px solid #d4a72c;
  border-radius: 5px;
}
.primary-stat { grid-column: span 6; height: 78px; }
.primary-stat .statistic-total { grid-column: 2; }
.daily-change { grid-column: 2; color: #777; font: 9px Consolas, monospace; white-space: nowrap; }
.daily-change.up { color: #66bb6a; }
.daily-change.down { color: #ef7070; }

.statistic-item.energy {
  border-left-color: #42a5f5;
}

.statistic-icon {
  grid-row: 1 / 4;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #d4a72c;
}

.statistic-item.energy .statistic-icon {
  color: #42a5f5;
}

.statistic-item.crop-summary {
  grid-column: span 4;
  height: 72px;
  padding: 10px 12px;
  grid-template-columns: 28px minmax(0, 1fr);
  border-left-color: #66bb6a;
}
.tree-crop { grid-column: span 3; }
.crop-summary .statistic-icon { color: #66bb6a; }
.crop-icon img { width: 25px; height: 25px; object-fit: contain; image-rendering: auto; }
.crop-summary .statistic-label { font-size: 10px; }
.crop-summary strong { font-size: 18px; }
.statistic-value { display: flex; align-items: baseline; gap: 7px; }
.statistic-value .daily-change { grid-column: auto; }
.statistic-total {
  grid-column: 2;
  color: #777;
  font: 9px Consolas, monospace;
  white-space: nowrap;
}

.statistic-label {
  min-width: 0;
  color: #bbb;
  font-size: 11px;
  white-space: nowrap;
}

.statistic-item strong {
  min-width: 0;
  color: #f5f5f5;
  font-family: Consolas, monospace;
  font-size: 24px;
  line-height: 1;
  overflow-wrap: anywhere;
}

.today-section,
.trend-section {
  margin-top: 14px;
  padding: 10px 12px;
  background: rgba(255, 255, 255, 0.025);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 5px;
}

.section-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: #ddd;
  font-size: 12px;
  font-weight: 600;
}

.section-title > span {
  margin-left: 8px;
  color: #777;
  font-size: 9px;
  font-weight: 400;
}

.crop-stats { margin-top: 7px; }
.crop-stat-row {
  display: grid;
  grid-template-columns: 1fr auto 42px;
  align-items: baseline;
  gap: 8px;
  min-height: 25px;
  border-bottom: 1px solid rgba(255,255,255,.05);
}
.crop-stat-row:last-child { border-bottom: 0; }
.crop-stat-name { color: #bbb; font-size: 11px; }
.crop-stat-row strong { color: #f0f0f0; font: 700 12px Consolas, monospace; }
.crop-stat-row small { text-align: right; font: 10px Consolas, monospace; }
.crop-stat-row .up { color: #66bb6a; }
.crop-stat-row .down { color: #ef7070; }
.crop-stat-row .flat { color: #777; }
.empty-stat { padding: 12px 0 4px; color: #777; font-size: 10px; text-align: center; }

.trend-switch { display: inline-flex; gap: 3px; }
.trend-switch button {
  padding: 3px 8px;
  color: #888;
  background: transparent;
  border: 1px solid rgba(255,255,255,.12);
  border-radius: 3px;
  font: 10px inherit;
  cursor: pointer;
}
.trend-switch button.active { color: #fff; background: rgba(102,187,106,.25); border-color: #66bb6a; }
.trend-chart-wrap { margin-top: 8px; }
.trend-chart { width: 100%; height: 230px; min-height: 180px; }
.life-coin-legend { display: flex; justify-content: flex-end; gap: 12px; margin-bottom: 2px; color: #888; font-size: 9px; }
.life-coin-legend span { display: inline-flex; align-items: center; gap: 4px; }
.life-coin-legend i { width: 8px; height: 8px; display: inline-block; border-radius: 1px; }
.life-coin-positive { background: #66bb6a; }
.life-coin-negative { background: #ef7070; }
.chart-axis { stroke: rgba(255,255,255,.14); stroke-width: 1; }
.chart-line { fill: none; stroke: #d4a72c; stroke-width: 2.5; vector-effect: non-scaling-stroke; }
.energy-line { stroke: #42a5f5; stroke-dasharray: 4 3; }
.trend-options { display:flex; flex-wrap:wrap; gap:5px; margin:8px 0 3px; }
.trend-options button { border:1px solid rgba(255,255,255,.12); border-radius:3px; padding:3px 7px; color:#777; background:transparent; font:10px inherit; cursor:pointer; }
.trend-options button.selected { color:#ddd; border-color:rgba(255,255,255,.3); background:rgba(255,255,255,.06); }
.trend-options i { display:inline-block; width:8px; height:8px; margin-right:4px; border-radius:50%; vertical-align:-1px; opacity:.45; }
.trend-options button.selected i { opacity:1; }
.chart-labels { display:flex; justify-content:space-between; color:#666; font:9px Consolas, monospace; margin-top:3px; }

.resource-meter {
  padding: 6px 8px;
  background: rgba(255, 255, 255, 0.035);
  border: 1px solid rgba(255, 255, 255, 0.09);
  border-radius: 5px;
}

.resource-line {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 5px;
}

.resource-name {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: #bbb;
  font-size: 10px;
}

.resource-value {
  color: #eee;
  font-family: Consolas, monospace;
  font-size: 11px;
  font-weight: 700;
}

.resource-track,
.growth-track {
  overflow: hidden;
  background: rgba(255, 255, 255, 0.08);
}

.resource-track {
  height: 4px;
  border-radius: 2px;
}

.resource-fill,
.growth-fill {
  height: 100%;
  transition: width 0.35s linear;
}

.fertility-meter .resource-name {
  color: #66bb6a;
}

.fertility-meter .resource-fill {
  background: #66bb6a;
}

.energy-meter .resource-name {
  color: #ffd54f;
}

.energy-meter .resource-fill {
  background: #ffd54f;
}

.farm-grid {
  flex: 1;
  min-height: 332px;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  grid-template-rows: repeat(4, minmax(78px, 1fr));
  gap: 6px;
}

.farm-plot {
  position: relative;
  min-width: 0;
  min-height: 78px;
  padding: 6px 7px 7px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: rgba(34, 34, 34, 0.92);
  border: 2px solid #050505;
  border-radius: 6px;
}

.farm-plot.quality-normal {
  border-color: #43a047;
}

.farm-plot.quality-advanced {
  border-color: #42a5f5;
}

.farm-plot.quality-highest {
  border-color: #e040fb;
}

.farm-plot.special {
  background: rgba(154, 116, 24, 0.34);
}

.farm-plot.ready {
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.12);
}

.plot-header,
.crop-line,
.plot-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
  min-width: 0;
}

.plot-header {
  min-height: 14px;
  color: #888;
  font-size: 9px;
}

.plot-label,
.quality-name,
.crop-name,
.phase-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.special-mark {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
  color: #ffe082;
}

.crop-line {
  margin-top: 3px;
}

.crop-name {
  color: #f2f2f2;
  font-size: 12px;
  font-weight: 700;
}

.phase-name {
  flex-shrink: 0;
  color: #aaa;
  font-size: 9px;
}

.phase-name.ready {
  color: #81c784;
}

.crop-time {
  flex: 1;
  min-height: 22px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  color: #d5d5d5;
  font-family: Consolas, monospace;
  font-size: 15px;
  font-weight: 700;
}

.crop-time.ready {
  color: #81c784;
  font-family: 'Microsoft YaHei', sans-serif;
  font-size: 12px;
}

.plot-footer {
  min-height: 14px;
  color: #aaa;
  font-size: 9px;
}

.support-score {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  color: #b7d7bb;
  font-weight: 700;
}

.fertile-mark {
  color: #81c784;
}

.growth-track {
  position: absolute;
  left: 5px;
  right: 5px;
  bottom: 3px;
  height: 2px;
}

.growth-fill {
  background: #90a4ae;
}

.farm-plot.ready .growth-fill {
  background: #66bb6a;
}

.empty-plot {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  min-width: 0;
  color: #595959;
  text-align: center;
  font-size: 9px;
  line-height: 1.25;
}

.empty-plot span {
  max-width: 100%;
  white-space: normal;
}

.disabled-state {
  position: absolute;
  left: 50%;
  top: 54%;
  transform: translate(-50%, -50%);
  padding: 7px 12px;
  color: #aaa;
  background: rgba(18, 18, 18, 0.94);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 5px;
  font-size: 11px;
  pointer-events: none;
}

@media (max-height: 620px) {
  .farm-grid {
    min-height: 304px;
    grid-template-rows: repeat(4, minmax(72px, 1fr));
  }

  .farm-plot {
    min-height: 72px;
  }
}

@media (max-width: 480px) {
  .statistics-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
  .primary-stat, .crop-summary, .tree-crop { grid-column: span 1; }
  .statistic-item { padding-left: 10px; padding-right: 10px; }
  .statistic-item.crop-summary { padding-left: 9px; padding-right: 9px; }
  .association-coins-row { padding-left: 10px; padding-right: 10px; gap: 8px; }
  .association-coins-values { gap: 8px; }
  .association-coins-label, .association-coins-values { font-size: 12px; }
  .association-coins-balance, .association-coins-added, .association-coins-spent { font-size: 13px; }
}
</style>
