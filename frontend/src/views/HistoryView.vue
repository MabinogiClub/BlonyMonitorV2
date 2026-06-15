<script setup lang="ts">
/**
 * 历史记录视图：浏览本地保存的战斗数据 + 全程 DPS 趋势图
 */

import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useAppStore } from '../stores/app'
import * as api from '../composables/useApi'
import { formatNumber, formatDuration, getDisplayName, BAR_CLASSES } from '../composables/useUtils'
import { useHistoryChart } from '../composables/useHistoryChart'
import SkillDetailItem from '../components/SkillDetailItem.vue'

const appStore = useAppStore()

const fileList = ref<string[]>([])
const saveDir = ref('')
const refreshing = ref(false)
const selectedFile = ref('')
const targets = ref<HistoryTarget[]>([])
const loading = ref(false)
const loadError = ref('')
const selectedTarget = ref<HistoryTarget | null>(null)
const expandedAttackers = ref<Set<string>>(new Set())
const targetTimeRange = ref<{ minTime: number; maxTime: number } | null>(null)

const {
  calculateTargetTimeRange,
  extractChartDataFromHistory,
  handleSkillClick,
} = useHistoryChart(targetTimeRange, selectedTarget)

function refreshHistoryChart() {
  if (!selectedTarget.value) return
  extractChartDataFromHistory(selectedTarget.value, appStore.selectedSkillFilters)
}

watch(
  () => appStore.selectedSkillFilters.map(f => `${f.attackerId ?? ''}:${f.skillId}`).join(','),
  () => {
    refreshHistoryChart()
  }
)

async function loadFileList() {
  refreshing.value = true
  try {
    const [files, dir] = await Promise.all([
      api.getCleanedTargetsList(),
      api.getSaveDir(),
    ])
    fileList.value = files
    saveDir.value = dir
  } catch (e) {
    console.error('Failed to load history file list:', e)
  } finally {
    refreshing.value = false
  }
}

function refreshIfNeeded() {
  if (appStore.activeTab === 'history') {
    loadFileList()
  }
}

async function readFile(fileName: string) {
  loading.value = true
  loadError.value = ''
  selectedFile.value = fileName
  targets.value = []
  selectedTarget.value = null
  expandedAttackers.value.clear()
  appStore.clearSelectedSkills()
  appStore.clearHistoryChartData()

  try {
    const data = await api.readCleanedTargetFileFull(fileName)
    if (data && typeof data === 'object' && 'error' in data) {
      loadError.value = String((data as { error: string }).error)
      return
    }

    let list: HistoryTarget[] = []
    if (data && typeof data === 'object' && !Array.isArray(data) && 'targets' in data) {
      list = (data as { targets: HistoryTarget[] }).targets || []
    } else if (Array.isArray(data)) {
      list = data as HistoryTarget[]
    }

    targets.value = list.map(normalizeTarget)
  } catch (e) {
    console.error('Failed to read history file:', e)
    loadError.value = '读取文件失败'
  } finally {
    loading.value = false
  }
}

function normalizeTarget(target: HistoryTarget): HistoryTarget {
  const id = target.targetId || target.id
  const name = target.targetName || target.name
  const attackers = (target.attackers || []).map((attacker): HistoryAttacker => {
    const historyAttacker = attacker as HistoryAttacker
    return {
      ...historyAttacker,
      skills: historyAttacker.skills || historyAttacker.skillsDetail?.map(s => ({
        skillId: s.skillId,
        skillName: s.skillName,
        totalDamage: s.totalDamage,
        hitCount: s.hitCount,
        critCount: s.critCount,
        avgDamage: s.avgDamage,
        minDamage: s.minDamage,
        maxDamage: s.maxDamage,
        critMinDamage: s.critMinDamage,
        critMaxDamage: s.critMaxDamage,
        percent: s.percent,
      })) || [],
    }
  })

  return {
    ...target,
    id,
    name,
    targetId: id,
    targetName: name,
    attackers,
    duration: target.duration || 1,
    dps: target.dps || (target.totalDamage / Math.max(target.duration || 1, 1)),
  }
}

function selectTarget(target: HistoryTarget) {
  selectedTarget.value = target
  appStore.setSelectedTarget({
    id: target.targetId || target.id,
    name: target.targetName || target.name,
    deathTime: target.deathTime,
  })
  appStore.setBossHPHistoryData(target.bossHP ? [target.bossHP as BossHPHistoryItem] : [])
  appStore.clearSelectedSkills()
  calculateTargetTimeRange(target)
  extractChartDataFromHistory(target)
}

function goBack() {
  selectedTarget.value = null
  targetTimeRange.value = null
  appStore.clearSelectedTarget()
  appStore.clearSelectedSkills()
  appStore.clearHistoryChartData()
  appStore.setBossHPHistoryData([])
}

function isAttackerAllSelected(attacker: HistoryAttacker): boolean {
  if (!attacker.skills || attacker.skills.length === 0) return false
  return attacker.skills.every(skill =>
    appStore.selectedSkillFilters.some(
      f => f.skillId === skill.skillId && f.attackerId === attacker.id
    )
  )
}

function selectAllSkillsForAttacker(attacker: HistoryAttacker) {
  if (!attacker.skills || attacker.skills.length === 0 || !selectedTarget.value) return

  const allSelected = isAttackerAllSelected(attacker)
  attacker.skills.forEach(skill => {
    const selected = appStore.selectedSkillFilters.some(
      f => f.skillId === skill.skillId && f.attackerId === attacker.id
    )
    if (allSelected) {
      if (selected) appStore.toggleSkillFilter(skill.skillId, attacker.id)
    } else if (!selected) {
      appStore.toggleSkillFilter(skill.skillId, attacker.id)
    }
  })

  extractChartDataFromHistory(selectedTarget.value, appStore.selectedSkillFilters)
}

function toggleExpand(attackerId: string) {
  if (expandedAttackers.value.has(attackerId)) {
    expandedAttackers.value.delete(attackerId)
  } else {
    expandedAttackers.value.add(attackerId)
  }
}

function isExpanded(attackerId: string): boolean {
  return expandedAttackers.value.has(attackerId)
}

function getBarClass(index: number): string {
  return BAR_CLASSES[index % BAR_CLASSES.length]
}

function formatFileLabel(fileName: string): string {
  const stem = fileName.replace(/\.json(\.gz)?$/, '')
  return stem.replace(/_/g, ' ')
}

watch(() => appStore.activeTab, (tab) => {
  if (tab === 'history') {
    loadFileList()
  }
})

onMounted(() => {
  loadFileList()
  api.onEvent('history-saved', refreshIfNeeded)
  api.onEvent('clear', refreshIfNeeded)
})

onUnmounted(() => {
  api.offEvent('history-saved')
})
</script>

<template>
  <div class="history-view">
    <section class="content-panel">
      <van-loading v-if="loading" class="loading-state" vertical>加载中...</van-loading>

      <van-empty
        v-else-if="loadError"
        image="error"
        :description="loadError"
      />

      <van-empty
        v-else-if="!selectedFile"
        image="search"
        description="请选择右侧历史文件"
      />

      <van-empty
        v-else-if="targets.length === 0"
        image="search"
        description="该文件没有目标数据"
      />

      <template v-else-if="selectedTarget">
        <div
          class="damage-item back-btn"
          @click="goBack"
        >
          <div class="damage-content">
            <span class="damage-name">← 返回列表</span>
            <div class="damage-info">
              <span class="damage-duration">{{ formatDuration(selectedTarget.duration) }}</span>
              <span class="damage-dps hover-tip" :data-tooltip="selectedTarget.dps.toLocaleString() + '/s'">
                {{ formatNumber(selectedTarget.dps) }}/s
              </span>
              <span class="damage-value">{{ getDisplayName(selectedTarget.targetId || selectedTarget.id, selectedTarget.targetName || selectedTarget.name) }}</span>
              <span class="damage-percent hover-tip" :data-tooltip="'总计 ' + selectedTarget.totalDamage.toLocaleString()">
                总计 {{ formatNumber(selectedTarget.totalDamage) }}
              </span>
            </div>
          </div>
        </div>

        <div class="history-hint">
          对 {{ getDisplayName(selectedTarget.targetId || selectedTarget.id, selectedTarget.targetName || selectedTarget.name) }} 造成伤害的所有来源
        </div>

        <template v-for="(attacker, index) in selectedTarget.attackers" :key="attacker.id">
          <div
            class="damage-item history-row"
            :class="{
              expandable: attacker.skills && attacker.skills.length > 0,
              expanded: isExpanded(attacker.id)
            }"
            @click="attacker.skills && attacker.skills.length > 0 && toggleExpand(attacker.id)"
          >
            <div
              class="damage-bar"
              :class="getBarClass(index)"
              :style="{ width: `${(attacker.totalDamage / (selectedTarget.attackers[0]?.totalDamage || 1) * 100).toFixed(1)}%` }"
            />
            <div class="damage-content">
              <span class="damage-name">
                <span class="damage-name-wrapper">
                  <span v-if="attacker.skills && attacker.skills.length > 0" class="expand-icon">▶</span>
                  <span class="damage-name-text">{{ getDisplayName(attacker.id, attacker.name) }}</span>
                  <button
                    v-if="attacker.isPC !== false && attacker.skills && attacker.skills.length > 0"
                    class="select-all-btn"
                    :class="{ selected: isAttackerAllSelected(attacker) }"
                    @click.stop="selectAllSkillsForAttacker(attacker)"
                  >
                    全选
                  </button>
                </span>
              </span>
              <div class="damage-info">
                <span class="damage-dps hover-tip" :data-tooltip="attacker.dps.toLocaleString() + '/s'">
                  {{ formatNumber(attacker.dps) }}/s
                </span>
                <span class="damage-value hover-tip" :data-tooltip="attacker.totalDamage.toLocaleString()">
                  {{ formatNumber(attacker.totalDamage) }}
                </span>
                <span class="damage-percent">{{ attacker.percent.toFixed(1) }}%</span>
              </div>
            </div>
          </div>

          <div
            v-if="attacker.skills && attacker.skills.length > 0"
            class="sub-items"
            v-show="isExpanded(attacker.id)"
          >
            <SkillDetailItem
              v-for="(skill, skillIndex) in attacker.skills"
              :key="skill.skillId"
              :skill="skill"
              :skill-index="skillIndex"
              :parent-index="index"
              :max-skill-damage="attacker.skills![0]?.totalDamage || 1"
              :attacker-id="attacker.id"
              @click-skill="handleSkillClick"
            />
          </div>
        </template>
      </template>

      <template v-else>
        <div class="history-hint">共 {{ targets.length }} 个目标</div>
        <div
          v-for="(target, index) in targets"
          :key="target.targetId || target.id"
          class="damage-item expandable target-item history-row"
          @click="selectTarget(target)"
        >
          <div
            class="damage-bar"
            :class="getBarClass(index)"
            :style="{ width: `${(target.totalDamage / (targets[0]?.totalDamage || 1) * 100).toFixed(1)}%` }"
          />
          <div class="damage-content">
            <span class="damage-name">
              <span class="damage-name-wrapper">
                <span class="expand-icon">▶</span>
                <span class="damage-name-text">{{ getDisplayName(target.targetId || target.id, target.targetName || target.name) }}</span>
              </span>
            </span>
            <div class="damage-info">
              <span class="damage-duration">{{ formatDuration(target.duration) }}</span>
              <span class="damage-dps hover-tip" :data-tooltip="target.dps.toLocaleString() + '/s'">
                {{ formatNumber(target.dps) }}/s
              </span>
              <span class="damage-value hover-tip" :data-tooltip="target.totalDamage.toLocaleString()">
                {{ formatNumber(target.totalDamage) }}
              </span>
            </div>
          </div>
        </div>
      </template>
    </section>

    <aside class="file-panel">
      <div class="panel-header">
        <div class="panel-title">历史文件</div>
        <button
          class="refresh-btn"
          title="刷新列表"
          :disabled="refreshing"
          @click="loadFileList"
        >
          ↻
        </button>
      </div>
      <div v-if="saveDir" class="save-dir-hint" :title="saveDir">
        {{ saveDir }}
      </div>
      <van-empty
        v-if="fileList.length === 0"
        image="search"
        description="暂无保存记录"
      />
      <div v-else class="file-list">
        <div
          v-for="file in fileList"
          :key="file"
          class="file-item"
          :class="{ active: selectedFile === file }"
          @click="readFile(file)"
        >
          {{ formatFileLabel(file) }}
        </div>
      </div>
    </aside>
  </div>
</template>

<style lang="scss" scoped>
.history-view {
  display: flex;
  height: 100%;
  min-height: 0;
  gap: 8px;
  overflow: hidden;
}

.content-panel {
  flex: 1;
  min-width: 0;
  overflow-y: auto;
  overflow-x: hidden;
  padding-right: 2px;
}

.file-panel {
  width: 210px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  min-height: 0;
  background: rgba(30, 30, 30, 0.6);
  border-radius: 4px;
  padding: 6px;
  border-left: 1px solid rgba(255, 255, 255, 0.08);
}

.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
  margin-bottom: 4px;
}

.panel-title {
  font-size: 11px;
  color: #aaa;
}

.refresh-btn {
  border: none;
  background: rgba(255, 255, 255, 0.1);
  color: #bbb;
  border-radius: 3px;
  width: 20px;
  height: 20px;
  cursor: pointer;
  font-size: 12px;
  line-height: 1;

  &:hover:not(:disabled) {
    background: rgba(66, 165, 245, 0.3);
    color: #fff;
  }

  &:disabled {
    opacity: 0.5;
    cursor: default;
  }
}

.save-dir-hint {
  font-size: 9px;
  color: #666;
  margin-bottom: 6px;
  line-height: 1.2;
  word-break: break-all;
  max-height: 2.4em;
  overflow: hidden;
}

.file-list {
  overflow-y: auto;
  flex: 1;
}

.file-item {
  font-size: 10px;
  color: #bbb;
  padding: 6px 4px;
  border-radius: 3px;
  cursor: pointer;
  line-height: 1.3;
  word-break: break-all;

  &:hover {
    background: rgba(255, 255, 255, 0.08);
    color: #fff;
  }

  &.active {
    background: rgba(66, 165, 245, 0.25);
    color: #fff;
  }
}

.loading-state {
  display: flex;
  justify-content: center;
  padding: 24px 0;
}

.history-hint {
  margin: 6px 0;
  padding: 6px 8px;
  background: rgba(40, 40, 40, 0.8);
  border-radius: 4px;
  font-size: 11px;
  color: #aaa;
}

.back-btn {
  background: rgba(66, 165, 245, 0.2);
  cursor: pointer;
}

.history-row,
.history-sub-row {
  overflow: visible !important;
}

.history-view :deep(.damage-item) {
  overflow: visible;
}

.history-view :deep(.damage-name) {
  min-width: 0;
  max-width: none;
  flex: 1;
}

.history-view :deep(.damage-content) {
  gap: 8px;
  align-items: flex-start;
}

.history-view :deep(.damage-info) {
  flex-shrink: 0;
  flex-wrap: nowrap;
  white-space: nowrap;
}

.history-view :deep(.sub-item-name) {
  max-width: none;
  display: flex;
  align-items: center;
  gap: 6px;
}

.select-all-btn {
  margin-left: 6px;
  padding: 1px 6px;
  font-size: 10px;
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 3px;
  background: rgba(255, 255, 255, 0.08);
  color: #bbb;
  cursor: pointer;
  flex-shrink: 0;

  &:hover,
  &.selected {
    background: rgba(66, 165, 245, 0.25);
    color: #fff;
    border-color: rgba(66, 165, 245, 0.5);
  }
}
</style>
