<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { getBuffIconUrl } from '../utils/buffIcons'

const props = defineProps<{
  player?: PlayerBuffCoverage
}>()

interface CoverageTooltip {
  buff: BuffCoverage
  left: number
  top: number
}

const tooltip = ref<CoverageTooltip | null>(null)
const tooltipElement = ref<HTMLElement | null>(null)

const visibleBuffs = computed(() => {
  return (props.player?.buffs || []).filter(buff => buff.coveragePercent > 0 && buff.activeSeconds > 0)
})

onMounted(() => {
  window.addEventListener('scroll', hideTooltip, true)
  window.addEventListener('resize', hideTooltip)
})

onUnmounted(() => {
  window.removeEventListener('scroll', hideTooltip, true)
  window.removeEventListener('resize', hideTooltip)
})

function formatSeconds(value: number): string {
  if (value >= 60) {
    const minutes = Math.floor(value / 60)
    const seconds = Math.round(value % 60)
    return `${minutes}:${seconds.toString().padStart(2, '0')}`
  }
  return `${value.toFixed(1)}秒`
}

function formatStrength(conditionId: number, value?: number): string {
  if (value === undefined) return '-'
  if (conditionId === 680 || conditionId === 192) return `${value.toFixed(2)}%`
  if (conditionId === 193) return `${((value - 1) * 100).toFixed(2)}%`
  return value.toFixed(4)
}

function canMergeSegments(buff: BuffCoverage, previous: BuffCoverageSegment, next: BuffCoverageSegment): boolean {
  if (next.startOffset - previous.endOffset > 1.0) return false
  if (buff.conditionId !== 680 && buff.conditionId !== 192 && buff.conditionId !== 193) return true
  return previous.strength !== undefined && next.strength !== undefined && previous.strength === next.strength
}

function getDisplaySegments(buff: BuffCoverage): BuffCoverageSegment[] {
  const segments = [...(buff.segments || [])].sort((a, b) => a.startOffset - b.startOffset)
  if (segments.length < 2) return segments

  const merged: BuffCoverageSegment[] = []
  for (const segment of segments) {
    const previous = merged[merged.length - 1]
    if (!previous || !canMergeSegments(buff, previous, segment)) {
      merged.push({ ...segment })
      continue
    }
    previous.endedAt = Math.max(previous.endedAt, segment.endedAt)
    previous.endOffset = Math.max(previous.endOffset, segment.endOffset)
    previous.activeSeconds += segment.activeSeconds
  }
  return merged
}

const timelineDuration = computed(() => {
  const battleSeconds = props.player?.battleSeconds
  if (battleSeconds && battleSeconds > 0) return battleSeconds
  return Math.max(0, ...(props.player?.buffs || []).flatMap(buff => (buff.segments || []).map(segment => segment.endOffset)))
})

function getTimelineSegmentStyle(segment: BuffCoverageSegment): Record<string, string> {
  const duration = timelineDuration.value
  if (duration <= 0) return { display: 'none' }
  const start = Math.max(0, Math.min(duration, segment.startOffset))
  const end = Math.max(start, Math.min(duration, segment.endOffset))
  return {
    left: `${(start / duration) * 100}%`,
    width: `${((end - start) / duration) * 100}%`,
  }
}

function getTimelineTone(buff: BuffCoverage, segmentIndex: number): 'base' | 'alt' {
  const segments = getDisplaySegments(buff)
  let toneChanges = 0
  for (let index = 1; index <= segmentIndex; index += 1) {
    const previousStrength = segments[index - 1]?.strength
    const currentStrength = segments[index]?.strength
    if (previousStrength !== undefined && currentStrength !== undefined && previousStrength !== currentStrength) {
      toneChanges += 1
    }
  }
  return toneChanges % 2 === 0 ? 'base' : 'alt'
}

async function showTooltip(event: MouseEvent | FocusEvent, buff: BuffCoverage) {
  const target = event.currentTarget as HTMLElement
  const rect = target.getBoundingClientRect()
  tooltip.value = {
    buff,
    left: rect.left + rect.width / 2,
    top: rect.bottom + 8,
  }

  await nextTick()
  const element = tooltipElement.value
  if (!element || !tooltip.value) return

  const margin = 8
  const halfWidth = element.offsetWidth / 2
  tooltip.value.left = Math.max(margin + halfWidth, Math.min(window.innerWidth - margin - halfWidth, tooltip.value.left))
  if (tooltip.value.top + element.offsetHeight > window.innerHeight - margin) {
    tooltip.value.top = Math.max(margin, rect.top - element.offsetHeight - 8)
  }
}

function hideTooltip() {
  tooltip.value = null
}
</script>

<template>
  <div v-if="player && visibleBuffs.length > 0" class="buff-coverage-bar">
    <span class="coverage-label">状态</span>
    <div class="buff-items">
      <button
        v-for="buff in visibleBuffs"
        :key="buff.conditionId"
        type="button"
        class="buff-item"
        :aria-label="`${player.playerName} ${buff.conditionName} 覆盖率 ${buff.coveragePercent.toFixed(1)}%`"
        @mouseenter="showTooltip($event, buff)"
        @mouseleave="hideTooltip"
        @focus="showTooltip($event, buff)"
        @blur="hideTooltip"
      >
        <img :src="getBuffIconUrl(buff.conditionId)" :alt="buff.conditionName" width="16" height="16">
        <span>{{ buff.coveragePercent.toFixed(1) }}%</span>
      </button>
    </div>

    <Teleport to="body">
      <div
        v-if="tooltip"
        ref="tooltipElement"
        class="coverage-tooltip"
        role="tooltip"
        :style="{ left: `${tooltip.left}px`, top: `${tooltip.top}px` }"
      >
        <div class="tooltip-heading">
          <span>{{ player.playerName }}</span>
          <span>{{ tooltip.buff.conditionName }}</span>
        </div>
        <div class="coverage-timeline" aria-label="Buff 持续进度">
          <div class="coverage-timeline-track">
            <span
              v-for="(segment, index) in getDisplaySegments(tooltip.buff)"
              :key="`${segment.startedAt}-${index}`"
              class="coverage-timeline-active"
              :class="{ 'coverage-timeline-active-alt': getTimelineTone(tooltip.buff, index) === 'alt' }"
              :style="getTimelineSegmentStyle(segment)"
            />
          </div>
        </div>
        <div class="tooltip-metrics">
          <span>覆盖率</span><strong>{{ tooltip.buff.coveragePercent.toFixed(1) }}%</strong>
          <span>持续时间</span><strong>{{ formatSeconds(tooltip.buff.activeSeconds) }}</strong>
          <template v-if="tooltip.buff.averageStrength !== undefined">
            <span>平均</span><strong>{{ formatStrength(tooltip.buff.conditionId, tooltip.buff.averageStrength) }}</strong>
            <span>范围</span>
            <strong>
              {{ formatStrength(tooltip.buff.conditionId, tooltip.buff.minStrength) }} -
              {{ formatStrength(tooltip.buff.conditionId, tooltip.buff.maxStrength) }}
            </strong>
          </template>
        </div>
        <div v-if="getDisplaySegments(tooltip.buff).length > 1" class="tooltip-segments">
          <div v-for="(segment, index) in getDisplaySegments(tooltip.buff)" :key="`${segment.startedAt}-${index}`" class="tooltip-segment">
            <span>{{ segment.startOffset.toFixed(1) }}s - {{ segment.endOffset.toFixed(1) }}s</span>
            <span>{{ formatStrength(tooltip.buff.conditionId, segment.strength) }}</span>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.buff-coverage-bar {
  display: flex;
  align-items: center;
  min-height: 26px;
  padding: 3px 8px;
  border-left: 2px solid rgba(127, 196, 255, 0.7);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(31, 40, 48, 0.82);
}

.coverage-label {
  flex: 0 0 auto;
  margin-right: 8px;
  color: #91a4b5;
  font-size: 10px;
}

.buff-items {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px 10px;
  min-width: 0;
}

.buff-item {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  height: 20px;
  padding: 1px 2px;
  border: 0;
  border-radius: 2px;
  background: transparent;
  color: #d6dce1;
  font: inherit;
  font-size: 10px;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
  cursor: help;
}

.buff-item:hover,
.buff-item:focus-visible {
  background: rgba(127, 196, 255, 0.14);
  outline: 1px solid rgba(127, 196, 255, 0.4);
}

.buff-item img {
  display: block;
  flex: 0 0 16px;
  width: 16px;
  height: 16px;
  image-rendering: auto;
}

:global(.coverage-tooltip) {
  position: fixed;
  z-index: 100000;
  width: min(280px, calc(100vw - 16px));
  max-height: min(360px, calc(100vh - 16px));
  padding: 9px 10px;
  overflow: auto;
  transform: translateX(-50%);
  border: 1px solid rgba(127, 196, 255, 0.4);
  border-radius: 4px;
  background: rgba(24, 24, 24, 0.98);
  box-shadow: 0 5px 18px rgba(0, 0, 0, 0.45);
  color: #bbb;
  font-size: 10px;
  pointer-events: none;
}

:global(.coverage-tooltip .tooltip-heading) {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding-bottom: 7px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  color: #eee;
  font-size: 11px;
  font-weight: 600;
}

:global(.coverage-tooltip .tooltip-metrics) {
  display: grid;
  grid-template-columns: 72px 1fr;
  gap: 5px 10px;
  padding-top: 7px;
}

:global(.coverage-tooltip .coverage-timeline) {
  padding-top: 8px;
}

:global(.coverage-tooltip .coverage-timeline-track) {
  position: relative;
  height: 8px;
  overflow: hidden;
  border-radius: 2px;
  background: #4a4d50;
}

:global(.coverage-tooltip .coverage-timeline-active) {
  position: absolute;
  top: 0;
  bottom: 0;
  min-width: 1px;
  background: #83c7ff;
  box-shadow: 0 0 5px rgba(131, 199, 255, 0.75);
}

:global(.coverage-tooltip .coverage-timeline-active-alt) {
  background: #4d9ed1;
  box-shadow: inset 1px 0 rgba(220, 241, 255, 0.75), 0 0 5px rgba(77, 158, 209, 0.75);
}

:global(.coverage-tooltip .tooltip-metrics strong) {
  color: #d7e9d9;
  font-weight: 500;
  text-align: right;
}

:global(.coverage-tooltip .tooltip-segments) {
  margin-top: 7px;
  padding-top: 6px;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
}

:global(.coverage-tooltip .tooltip-segment) {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  min-height: 18px;
  color: #888;
  font-variant-numeric: tabular-nums;
}
</style>
