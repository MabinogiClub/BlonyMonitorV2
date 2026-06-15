<script setup lang="ts">
/**
 * Buff 定时器 - 精简列表，长按拖拽自定义排序
 */

import { ref, onMounted, onUnmounted } from 'vue'
import { GetBuffDisplayList, SetBuffSoundEnabled, SetBuffOrder } from '../../wailsjs/go/app/App'
import { app } from '../../wailsjs/go/models'

type BuffDisplayInfo = app.BuffDisplayInfo

const buffList = ref<BuffDisplayInfo[]>([])
const isDragging = ref(false)
const dragIndex = ref(-1)
const dragOverIndex = ref(-1)

let updateInterval: number | null = null
let longPressTimer: number | null = null
let activePointerId: number | null = null

const LONG_PRESS_MS = 450

async function loadBuffList() {
  if (isDragging.value) return
  try {
    buffList.value = await GetBuffDisplayList() || []
  } catch (error) {
    console.error('加载 Buff 列表失败:', error)
  }
}

async function toggleSound(buff: BuffDisplayInfo, enabled: boolean) {
  try {
    await SetBuffSoundEnabled(buff.ccId, enabled)
    buff.soundEnabled = enabled
    if (!enabled) {
      buff.willNotify = false
    } else if (buff.isActive) {
      buff.willNotify = buff.remainingTime > 30
    }
  } catch (error) {
    console.error('设置声音开关失败:', error)
    await loadBuffList()
  }
}

function getIconUrl(buff: BuffDisplayInfo): string {
  if (buff.iconData) {
    return `data:image/png;base64,${buff.iconData}`
  }
  return ''
}

function formatTime(seconds: number): string {
  if (seconds <= 0) return '0:00'
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  return `${m}:${s.toString().padStart(2, '0')}`
}

function getProgress(buff: BuffDisplayInfo): number {
  if (!buff.isActive || buff.totalTime <= 0) return 0
  return Math.max(0, Math.min(100, (buff.remainingTime / buff.totalTime) * 100))
}

function getBarColor(buff: BuffDisplayInfo): string {
  const p = getProgress(buff)
  if (p <= 20) return '#ee0a24'
  if (p <= 40) return '#ff976a'
  return '#42a5f5'
}

function getTimeClass(buff: BuffDisplayInfo): string {
  if (!buff.isActive) return 'idle'
  const p = getProgress(buff)
  if (p <= 20) return 'danger'
  if (p <= 40) return 'warn'
  return 'active'
}

function clearLongPressTimer() {
  if (longPressTimer !== null) {
    clearTimeout(longPressTimer)
    longPressTimer = null
  }
}

function findRowIndexAt(clientY: number): number {
  const rows = document.querySelectorAll<HTMLElement>('.buff-row')
  for (let i = 0; i < rows.length; i++) {
    const rect = rows[i].getBoundingClientRect()
    if (clientY >= rect.top && clientY <= rect.bottom) {
      return i
    }
  }
  return -1
}

function onDragAreaPointerDown(e: PointerEvent, index: number) {
  if (e.button !== 0) return
  activePointerId = e.pointerId
  const target = e.currentTarget as HTMLElement
  target.setPointerCapture(e.pointerId)

  clearLongPressTimer()
  longPressTimer = window.setTimeout(() => {
    isDragging.value = true
    dragIndex.value = index
    dragOverIndex.value = index
  }, LONG_PRESS_MS)
}

function onDragAreaPointerMove(e: PointerEvent) {
  if (!isDragging.value || activePointerId !== e.pointerId) return
  const over = findRowIndexAt(e.clientY)
  if (over >= 0) {
    dragOverIndex.value = over
  }
}

async function onDragAreaPointerUp(e: PointerEvent) {
  if (activePointerId !== e.pointerId) return

  clearLongPressTimer()
  const from = dragIndex.value
  const to = dragOverIndex.value

  if (isDragging.value && from >= 0 && to >= 0 && from !== to) {
    const newList = [...buffList.value]
    const [item] = newList.splice(from, 1)
    newList.splice(to, 0, item)
    buffList.value = newList
    try {
      await SetBuffOrder(newList.map(b => b.ccId))
    } catch (error) {
      console.error('保存排序失败:', error)
      await loadBuffList()
    }
  }

  isDragging.value = false
  dragIndex.value = -1
  dragOverIndex.value = -1
  activePointerId = null

  const target = e.currentTarget as HTMLElement
  if (target.hasPointerCapture(e.pointerId)) {
    target.releasePointerCapture(e.pointerId)
  }
}

function onDragAreaPointerCancel(e: PointerEvent) {
  clearLongPressTimer()
  isDragging.value = false
  dragIndex.value = -1
  dragOverIndex.value = -1
  activePointerId = null

  const target = e.currentTarget as HTMLElement
  if (target.hasPointerCapture(e.pointerId)) {
    target.releasePointerCapture(e.pointerId)
  }
}

onMounted(async () => {
  await loadBuffList()
  updateInterval = window.setInterval(loadBuffList, 1000)
})

onUnmounted(() => {
  clearLongPressTimer()
  if (updateInterval !== null) clearInterval(updateInterval)
})
</script>

<template>
  <div class="buff-list" :class="{ dragging: isDragging }">
    <div
      v-for="(buff, index) in buffList"
      :key="buff.ccId"
      class="buff-row"
      :class="{
        running: buff.isActive,
        dragging: isDragging && dragIndex === index,
        'drag-over': isDragging && dragOverIndex === index && dragIndex !== index
      }"
    >
      <div
        class="buff-drag-area"
        title="长按拖动排序"
        @pointerdown="onDragAreaPointerDown($event, index)"
        @pointermove="onDragAreaPointerMove"
        @pointerup="onDragAreaPointerUp"
        @pointercancel="onDragAreaPointerCancel"
      >
        <div class="buff-icon-wrap">
          <img
            v-if="getIconUrl(buff)"
            class="buff-icon"
            :src="getIconUrl(buff)"
            :alt="buff.buffName"
          >
          <span v-else class="buff-icon-fallback">🎵</span>
        </div>

        <div class="buff-body">
          <div class="buff-line">
            <span class="buff-name">
              {{ buff.buffName }}<span class="buff-id">({{ buff.ccId }})</span>
            </span>
            <span class="buff-time" :class="getTimeClass(buff)">
              {{ buff.isActive ? formatTime(buff.remainingTime) : '--:--' }}
            </span>
          </div>
          <div class="buff-bar">
            <div
              class="buff-bar-fill"
              :style="{
                width: `${getProgress(buff)}%`,
                backgroundColor: buff.isActive ? getBarColor(buff) : 'transparent'
              }"
            />
          </div>
        </div>
      </div>

      <van-switch
        class="buff-switch"
        :model-value="buff.soundEnabled"
        size="16px"
        @update:model-value="toggleSound(buff, $event)"
        @pointerdown.stop
      />
    </div>
  </div>
</template>

<style scoped>
.buff-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  height: 100%;
  padding: 0 10px;
}

.buff-list.dragging {
  user-select: none;
  touch-action: none;
}

.buff-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 2px;
  border-radius: 6px;
  min-height: 40px;
  position: relative;
}

.buff-row.running {
  background: rgba(66, 165, 245, 0.08);
}

.buff-row.dragging {
  opacity: 0.55;
  background: rgba(66, 165, 245, 0.15);
}

.buff-row.drag-over::before {
  content: '';
  position: absolute;
  top: -2px;
  left: 0;
  right: 0;
  height: 2px;
  background: #42a5f5;
  border-radius: 1px;
}

.buff-drag-area {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: grab;
  touch-action: pan-y;
}

.buff-list.dragging .buff-drag-area {
  touch-action: none;
  cursor: grabbing;
}

.buff-icon-wrap {
  width: 28px;
  height: 28px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}

.buff-icon {
  width: 28px;
  height: 28px;
  border-radius: 4px;
  object-fit: cover;
  image-rendering: pixelated;
  pointer-events: none;
}

.buff-icon-fallback {
  font-size: 18px;
  line-height: 1;
  pointer-events: none;
}

.buff-body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.buff-line {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.buff-name {
  font-size: 11px;
  font-weight: 500;
  color: #ddd;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.buff-id {
  color: #888;
  font-weight: 400;
}

.buff-time {
  font-size: 12px;
  font-weight: 700;
  font-family: 'Courier New', monospace;
  flex-shrink: 0;
}

.buff-time.idle {
  color: #555;
}

.buff-time.active {
  color: #42a5f5;
}

.buff-time.warn {
  color: #ff976a;
}

.buff-time.danger {
  color: #ee0a24;
}

.buff-bar {
  height: 3px;
  background: rgba(255, 255, 255, 0.08);
  border-radius: 2px;
  overflow: hidden;
}

.buff-bar-fill {
  height: 100%;
  border-radius: 2px;
  transition: width 0.3s linear;
}

.buff-switch {
  flex-shrink: 0;
  transform: scale(0.85);
  transform-origin: center right;
}
</style>
