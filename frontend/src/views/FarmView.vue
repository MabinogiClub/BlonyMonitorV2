<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { showNotify } from 'vant'
import SvgIcon from '@jamescoyle/vue-icon'
import {
  mdiCheckCircleOutline,
  mdiClockOutline,
  mdiLeaf,
  mdiLightningBolt,
  mdiSprout,
  mdiStarFourPoints,
  mdiWateringCanOutline,
} from '@mdi/js'
import * as api from '../composables/useApi'

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
  plots: [],
})

const farmState = ref<FarmState>(emptyState())
const toggling = ref(false)
const readyNotificationToggling = ref(false)
const specialNotificationToggling = ref(false)
let updateInterval: number | null = null

const enabled = computed(() => farmState.value.enabled)

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
  farmState.value = state
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
  updateInterval = window.setInterval(loadFarmState, 1000)
})

onUnmounted(() => {
  api.offEvent('farm-state', onFarmState)
  api.offEvent('farm-ready', onFarmReady)
  api.offEvent('farm-special', onFarmSpecial)
  if (updateInterval !== null) clearInterval(updateInterval)
})
</script>

<template>
  <div class="farm-view">
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
</style>
