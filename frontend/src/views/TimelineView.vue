<script setup lang="ts">
/**
 * 玩家时间轴视图组件
 * 显示指定玩家的所有行为事件
 */

import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import * as api from '../composables/useApi'
import { formatNumber, formatTime, getConditionName, getConditionColorClass, getSkillName } from '../composables/useUtils'

// 玩家列表
const players = ref<EntityInfo[]>([])

// 选中的玩家ID
const selectedPlayerId = ref<string>('')

// 时间轴数据
const timeline = ref<PlayerTimeline | null>(null)

// 事件类型筛选（默认不选中造成伤害）
const eventTypeFilters = ref<Set<string>>(new Set(['damage-dealt','damage-taken', 'appear', 'finish']))  //'condition',状态 默认不显示

// 定时器 ID
let updateInterval: number | null = null

/**
 * 筛选后的事件列表（带原始索引）
 */
const filteredEvents = computed(() => {
  if (!timeline.value) return []
  return timeline.value.events
    .map((event, index) => ({ event, originalIndex: index }))
    .filter(({ event }) => {
      // 对于伤害事件，需要区分造成伤害和受到伤害
      if (event.type === 'damage') {
        if (event.entityId === selectedPlayerId.value) {
          return eventTypeFilters.value.has('damage-dealt')
        } else {
          return eventTypeFilters.value.has('damage-taken')
        }
      }
      return eventTypeFilters.value.has(event.type)
    })
})

/**
 * 累计伤害统计 - 用于显示当前总伤害
 */
const cumulativeDamage = computed(() => {
  if (!timeline.value) return new Map<number, number>()

  const cumulative = new Map<number, number>()
  let currentTotal = 0

  for (let i = 0; i < timeline.value.events.length; i++) {
    const event = timeline.value.events[i]
    if (event.type === 'damage' && event.damage && event.entityId === selectedPlayerId.value) {
      currentTotal += event.damage
      cumulative.set(i, currentTotal)
    }
  }

  return cumulative
})

/**
 * 切换事件类型筛选
 */
function toggleEventFilter(type: string) {
  if (eventTypeFilters.value.has(type)) {
    eventTypeFilters.value.delete(type)
  } else {
    eventTypeFilters.value.add(type)
  }
  // 触发响应式更新
  eventTypeFilters.value = new Set(eventTypeFilters.value)
}

/**
 * 清空时间轴
 */
async function clearTimeline() {
  try {
    await api.clear()
    timeline.value = null
  } catch (e) {
    console.error('Failed to clear timeline:', e)
  }
}

/**
 * 格式化相对时间（从战斗开始算起）
 */
function formatRelativeTime(timestamp: number, startTime: number): string {
  const seconds = timestamp - startTime
  if (seconds < 60) {
    return `${seconds}秒`
  }
  const minutes = Math.floor(seconds / 60)
  const remainingSeconds = seconds % 60
  return `${minutes}分${remainingSeconds}秒`
}

/**
 * 获取事件描述
 */
function getEventDescription(log: EventLog, originalIndex: number): string {
  switch (log.type) {
    case 'damage':
      if (log.entityId === selectedPlayerId.value) {
        // 玩家造成伤害
        const target = log.targetIsPC ? log.targetName : `[${log.targetRaceName}]${log.targetName}`
        const totalDamage = cumulativeDamage.value.get(originalIndex)
        const totalStr = totalDamage ? ` | 总伤害: ${formatNumber(totalDamage)}` : ''
        return `对 ${target} 造成 ${formatNumber(log.damage || 0)} 伤害 [${getSkillName(log.skillId || 0)}]${log.isCritical ? ' 暴击' : ''}${totalStr}`
      } else {
        // 玩家受到伤害
        const attacker = log.isPC ? log.entityName : `[${log.raceName}]${log.entityName}`
        return `受到 ${attacker} 的 ${formatNumber(log.damage || 0)} 伤害 [${getSkillName(log.skillId || 0)}]${log.isCritical ? ' 暴击' : ''}`
      }
    case 'appear':
      return '进入战斗'
    case 'condition':
      const conditionName = log.conditionName || getConditionName(log.conditionId || 0)
      if (log.isEnable) {
        return `获得状态 [${conditionName}]${log.attackerName ? ` ← ${log.attackerName}` : ''}`
      } else {
        return `失去状态 [${conditionName}]`
      }
    case 'finish':
      if (log.entityId === selectedPlayerId.value) {
        // 玩家被击杀
        return `被击败${log.attackerName ? ` ← ${log.attackerName}` : ''}`
      } else {
        // 玩家击杀
        const target = log.isPC ? log.entityName : `[${log.raceName}]${log.entityName}`
        return `击败 ${target}`
      }
    default:
      return '未知事件'
  }
}

/**
 * 获取事件类型标签
 */
function getEventTypeLabel(log: EventLog): string {
  switch (log.type) {
    case 'damage':
      return log.entityId === selectedPlayerId.value ? '造成伤害' : '受到伤害'
    case 'appear':
      return '出现'
    case 'condition':
      return log.isEnable ? '状态+' : '状态-'
    case 'finish':
      return log.entityId === selectedPlayerId.value ? '被击败' : '击杀'
    default:
      return '未知'
  }
}

/**
 * 获取事件类型样式类
 */
function getEventTypeClass(log: EventLog): string {
  switch (log.type) {
    case 'damage':
      return log.entityId === selectedPlayerId.value ? 'event-damage-dealt' : 'event-damage-taken'
    case 'appear':
      return 'event-appear'
    case 'condition':
      return 'event-condition'
    case 'finish':
      return log.entityId === selectedPlayerId.value ? 'event-death' : 'event-kill'
    default:
      return ''
  }
}

/**
 * 更新玩家列表
 */
async function updatePlayers() {
  try {
    players.value = await api.getAllPCEntities()
    // 如果还没有选中玩家，自动选择第一个
    if (!selectedPlayerId.value && players.value.length > 0) {
      selectedPlayerId.value = players.value[0].id
    }
  } catch (e) {
    console.error('Failed to get players:', e)
  }
}

/**
 * 更新时间轴数据
 */
async function updateTimeline() {
  if (!selectedPlayerId.value) return

  try {
    timeline.value = await api.getPlayerTimeline(selectedPlayerId.value)
  } catch (e) {
    console.error('Failed to get timeline:', e)
  }
}

// 监听玩家选择变化
watch(selectedPlayerId, () => {
  updateTimeline()
})

onMounted(() => {
  updatePlayers()
  updateInterval = window.setInterval(() => {
    updatePlayers()
    updateTimeline()
  }, 1000)
})

onUnmounted(() => {
  if (updateInterval) {
    clearInterval(updateInterval)
  }
})
</script>

<template>
  <div class="timeline-view">
    <!-- 玩家选择器 -->
    <div class="player-selector">
      <label>选择玩家:</label>
      <select v-model="selectedPlayerId" class="player-select">
        <option value="">请选择玩家</option>
        <option v-for="player in players" :key="player.id" :value="player.id">
          {{ player.name }}
        </option>
      </select>
      <button @click="clearTimeline" class="clear-button" title="清空时间轴">清空</button>
    </div>

    <!-- 事件类型筛选 -->
    <div v-if="timeline && timeline.events.length > 0" class="event-filters">
      <span class="filter-label">筛选:</span>
      <button
        @click="toggleEventFilter('damage-dealt')"
        :class="['filter-button', { active: eventTypeFilters.has('damage-dealt') }]"
      >
        造成伤害
      </button>
      <button
        @click="toggleEventFilter('damage-taken')"
        :class="['filter-button', { active: eventTypeFilters.has('damage-taken') }]"
      >
        受到伤害
      </button>
      <button
        @click="toggleEventFilter('appear')"
        :class="['filter-button', { active: eventTypeFilters.has('appear') }]"
      >
        出现
      </button>
      <button
        @click="toggleEventFilter('condition')"
        :class="['filter-button', { active: eventTypeFilters.has('condition') }]"
      >
        状态
      </button>
      <button
        @click="toggleEventFilter('finish')"
        :class="['filter-button', { active: eventTypeFilters.has('finish') }]"
      >
        击杀/死亡
      </button>
    </div>

    <!-- 空状态 -->
    <van-empty
      v-if="!timeline || timeline.events.length === 0"
      image="search"
      :description="selectedPlayerId ? '该玩家暂无事件记录' : '请选择一个玩家查看时间轴'"
    />

    <!-- 时间轴内容 -->
    <div v-else class="timeline-content">
      <!-- 统计信息 -->
      <div class="timeline-stats">
        <div class="stat-item">
          <span class="stat-label">战斗时长</span>
          <span class="stat-value">{{ formatRelativeTime(timeline.endTime, timeline.startTime) }}</span>
        </div>
        <div class="stat-item">
          <span class="stat-label">事件数量</span>
          <span class="stat-value">{{ filteredEvents.length }} / {{ timeline.events.length }}</span>
        </div>
      </div>

      <!-- 事件列表 -->
      <div class="timeline-events">
        <div
          v-for="(item, index) in filteredEvents"
          :key="`${item.event.type}-${item.event.at}-${item.originalIndex}`"
          class="timeline-event"
          :class="getEventTypeClass(item.event)"
        >
          <!-- 时间线 -->
          <div class="event-timeline">
            <div class="event-dot"></div>
            <div v-if="index < filteredEvents.length - 1" class="event-line"></div>
          </div>

          <!-- 事件内容 -->
          <div class="event-content">
            <div class="event-header">
              <span class="event-time">{{ formatTime(item.event.at) }}</span>
              <span class="event-relative-time">(+{{ formatRelativeTime(item.event.at, timeline.startTime) }})</span>
              <span class="event-type-label">{{ getEventTypeLabel(item.event) }}</span>
            </div>
            <div class="event-description">
              {{ getEventDescription(item.event, item.originalIndex) }}
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style lang="scss" scoped>
.timeline-view {
  height: 100%;
  display: flex;
  flex-direction: column;
}

// 玩家选择器
.player-selector {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px;
  background: rgba(30, 30, 30, 0.8);
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);

  label {
    font-size: 11px;
    color: #aaa;
  }

  .player-select {
    flex: 1;
    padding: 4px 8px;
    background: rgba(40, 40, 40, 0.8);
    border: 1px solid rgba(255, 255, 255, 0.2);
    border-radius: 3px;
    color: #fff;
    font-size: 11px;
    cursor: pointer;

    &:focus {
      outline: none;
      border-color: #42a5f5;
    }
  }

  .clear-button {
    padding: 4px 12px;
    background: rgba(244, 67, 54, 0.3);
    border: 1px solid rgba(244, 67, 54, 0.5);
    border-radius: 3px;
    color: #ef5350;
    font-size: 11px;
    cursor: pointer;
    transition: all 0.2s;

    &:hover {
      background: rgba(244, 67, 54, 0.5);
      border-color: #f44336;
    }

    &:active {
      transform: scale(0.95);
    }
  }
}

// 事件类型筛选
.event-filters {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px;
  background: rgba(30, 30, 30, 0.6);
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  flex-wrap: wrap;

  .filter-label {
    font-size: 10px;
    color: #888;
  }

  .filter-button {
    padding: 3px 8px;
    background: rgba(40, 40, 40, 0.6);
    border: 1px solid rgba(255, 255, 255, 0.2);
    border-radius: 3px;
    color: #888;
    font-size: 10px;
    cursor: pointer;
    transition: all 0.2s;

    &:hover {
      background: rgba(60, 60, 60, 0.8);
      color: #aaa;
    }

    &.active {
      background: rgba(66, 165, 245, 0.3);
      border-color: #42a5f5;
      color: #42a5f5;
    }

    &:active {
      transform: scale(0.95);
    }
  }
}

// 时间轴内容
.timeline-content {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
}

// 统计信息
.timeline-stats {
  display: flex;
  gap: 12px;
  margin-bottom: 12px;
  padding: 8px;
  background: rgba(40, 40, 40, 0.6);
  border-radius: 4px;

  .stat-item {
    display: flex;
    flex-direction: column;
    gap: 2px;

    .stat-label {
      font-size: 9px;
      color: #888;
    }

    .stat-value {
      font-size: 12px;
      color: #fff;
      font-weight: 500;
    }
  }
}

// 事件列表
.timeline-events {
  position: relative;
}

// 单个事件
.timeline-event {
  display: flex;
  gap: 12px;
  margin-bottom: 8px;

  // 时间线部分
  .event-timeline {
    display: flex;
    flex-direction: column;
    align-items: center;
    width: 20px;
    flex-shrink: 0;

    .event-dot {
      width: 10px;
      height: 10px;
      border-radius: 50%;
      background: #42a5f5;
      border: 2px solid rgba(30, 30, 30, 0.8);
      flex-shrink: 0;
    }

    .event-line {
      width: 2px;
      flex: 1;
      background: rgba(255, 255, 255, 0.1);
      margin-top: 4px;
    }
  }

  // 事件内容
  .event-content {
    flex: 1;
    padding: 6px 8px;
    background: rgba(40, 40, 40, 0.4);
    border-radius: 4px;
    border-left: 3px solid #42a5f5;

    .event-header {
      display: flex;
      align-items: center;
      gap: 6px;
      margin-bottom: 4px;

      .event-time {
        font-size: 10px;
        color: #888;
      }

      .event-relative-time {
        font-size: 9px;
        color: #666;
      }

      .event-type-label {
        font-size: 9px;
        padding: 2px 6px;
        border-radius: 2px;
        background: rgba(66, 165, 245, 0.3);
        color: #42a5f5;
      }
    }

    .event-description {
      font-size: 11px;
      color: #ddd;
      line-height: 1.4;
    }
  }

  // 不同事件类型的颜色
  &.event-damage-dealt {
    .event-timeline .event-dot {
      background: #f44336;
    }
    .event-content {
      border-left-color: #f44336;
      .event-type-label {
        background: rgba(244, 67, 54, 0.3);
        color: #ef5350;
      }
    }
  }

  &.event-damage-taken {
    .event-timeline .event-dot {
      background: #ff9800;
    }
    .event-content {
      border-left-color: #ff9800;
      .event-type-label {
        background: rgba(255, 152, 0, 0.3);
        color: #ffb74d;
      }
    }
  }

  &.event-appear {
    .event-timeline .event-dot {
      background: #4caf50;
    }
    .event-content {
      border-left-color: #4caf50;
      .event-type-label {
        background: rgba(76, 175, 80, 0.3);
        color: #81c784;
      }
    }
  }

  &.event-condition {
    .event-timeline .event-dot {
      background: #9c27b0;
    }
    .event-content {
      border-left-color: #9c27b0;
      .event-type-label {
        background: rgba(156, 39, 176, 0.3);
        color: #ba68c8;
      }
    }
  }

  &.event-kill {
    .event-timeline .event-dot {
      background: #ffc107;
    }
    .event-content {
      border-left-color: #ffc107;
      .event-type-label {
        background: rgba(255, 193, 7, 0.3);
        color: #ffc107;
      }
    }
  }

  &.event-death {
    .event-timeline .event-dot {
      background: #e91e63;
    }
    .event-content {
      border-left-color: #e91e63;
      .event-type-label {
        background: rgba(233, 30, 99, 0.3);
        color: #f06292;
      }
    }
  }
}
</style>

