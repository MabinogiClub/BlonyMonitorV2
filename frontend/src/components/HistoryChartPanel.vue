<script setup lang="ts">
/**
 * 图表面板组件
 * 多玩家图表的容器，管理共享状态（缩放、平移、标尺、高亮）
 * 通过 provide 向子组件 PlayerChart 注入共享状态
 */
import { ref, reactive, computed, watch, onMounted, onUnmounted, nextTick, provide } from 'vue'
import { useAppStore } from '../stores/app'
import { COLORS, formatDamage, formatSeconds } from '../composables/useUtils'
import PlayerChart from './PlayerChart.vue'

const appStore = useAppStore()

/** 多玩家图表共享状态接口 */
interface SharedChartState {
  timeOffset: number
  timeScale: number
  rulerMode: boolean
  rulerStart: { x: number; time: number } | null
  rulerEnd: { x: number; time: number } | null
  highlightedSkill: { skillId: number; attackerId?: string } | null
  globalTimeRange: { minTime: number; maxTime: number }
  isShowingHistory: boolean
  selectedTarget: any
  redrawTrigger: number
}

// 共享状态：所有 PlayerChart 子组件共享同一个 reactive 对象
const sharedState = reactive<SharedChartState>({
  timeOffset: 0,
  timeScale: 1,
  rulerMode: false,
  rulerStart: null,
  rulerEnd: null,
  highlightedSkill: null,
  globalTimeRange: { minTime: 0, maxTime: 0 },
  isShowingHistory: false,
  selectedTarget: null,
  redrawTrigger: 0
})

// 通过 provide 向所有子组件注入共享状态
provide('chartSharedState', sharedState)

/** 图表数据：历史模式使用 historyChartData，实时模式使用 chartData */
const chartData = computed(() => {
  if (appStore.isShowingHistory) {
    return appStore.historyChartData
  }
  return appStore.chartData
})

/** 图表标题 */
const chartTitle = computed(() => {
  if (appStore.isShowingHistory) {
    return '历史记录技能伤害详情图'
  }

  if (appStore.activeTab === 'taken') {
    return '单目标累计伤害趋势图'
  }

  if (appStore.activeTab === 'bySkill') {
    return '全局伤害累计伤害趋势图'
  }

  return '累计伤害趋势图'
})

/** 按玩家名称分组数据 */
const playerGroups = computed(() => {
  const groupMap = new Map<string, any[]>()

  chartData.value.forEach(series => {
    const attackerName = (series as any).attackerName || series.name.split(' - ')[0]
    if (!groupMap.has(attackerName)) {
      groupMap.set(attackerName, [])
    }
    groupMap.get(attackerName)!.push(series)
  })

  return Array.from(groupMap.entries()).map(([name, data]) => ({ name, data }))
})

/** 面板高度：根据玩家数量动态计算 */
const panelHeight = computed(() => {
  const playerCount = playerGroups.value.length
  if (playerCount === 0) return 200
  const headerHeight = 30
  const perPlayerHeight = 200
  return headerHeight + playerCount * perPlayerHeight
})

/** 标尺状态提示文本 */
const rulerStatusText = computed(() => {
  if (!sharedState.rulerMode) return ''
  if (!sharedState.rulerStart) return '👆 点击图表设置标尺起点'
  if (!sharedState.rulerEnd) return '👆 点击图表设置标尺终点'
  return '✅ 标尺已设置完成（可拖动边界调整）'
})

/** 当前缩放级别文本（如 "5秒/格"） */
const tickIntervalText = computed(() => {
  const timeRange = sharedState.globalTimeRange.maxTime - sharedState.globalTimeRange.minTime
  const visibleTimeRange = timeRange / sharedState.timeScale
  const secondsPerTick = (visibleTimeRange / 10) / 1000

  if (secondsPerTick >= 60) {
    const minutes = Math.floor(secondsPerTick / 60)
    return `${minutes}分钟/格`
  }

  if (secondsPerTick % 1 !== 0) {
    return `${secondsPerTick.toFixed(1)}秒/格`
  }
  return `${secondsPerTick}秒/格`
})

// 技能颜色缓存
const skillColorCache = new Map<number, string>()

/** 根据技能 ID 获取对应颜色 */
function getSkillColor(skillId: number): string {
  if (skillColorCache.has(skillId)) {
    return skillColorCache.get(skillId)!
  }
  const hash = ((skillId * 2654435761) >>> 0) % COLORS.length
  const color = COLORS[hash]
  skillColorCache.set(skillId, color)
  return color
}

/** 根据玩家名称获取颜色（用于标尺信息面板） */
function getSkillColorForPlayer(playerName: string): string {
  const matchingSeries = chartData.value.find(series =>
    (series as any).attackerName === playerName
  )
  const skillId = (matchingSeries as any)?.skillId
  if (skillId !== undefined) {
    return getSkillColor(skillId)
  }
  let hash = 0
  for (let i = 0; i < playerName.length; i++) {
    hash = ((hash << 5) - hash) + playerName.charCodeAt(i)
    hash |= 0
  }
  return COLORS[Math.abs(hash) % COLORS.length]
}

/** 标尺信息：计算标尺范围内的伤害统计 */
const rulerInfo = computed(() => {
  if (!sharedState.rulerStart || !sharedState.rulerEnd) return null

  const timeDiff = Math.abs(sharedState.rulerEnd.time - sharedState.rulerStart.time)
  const seconds = timeDiff / 1000

  // 格式化时间差
  let timeText = ''
  if (seconds >= 60) {
    const minutes = Math.floor(seconds / 60)
    const remainingSeconds = (seconds % 60).toFixed(2)
    timeText = `${minutes}分${remainingSeconds}秒`
  } else {
    timeText = `${seconds.toFixed(2)}秒`
  }

  // 按技能系列统计
  const seriesStats: Array<{ name: string; damage: number; dps: number; count: number }> = []

  // 按攻击者统计（汇总同一玩家的所有技能）
  const attackerStatsMap = new Map<string, {
    name: string;
    damage: number;
    skills: Map<number, { count: number; damage: number; skillName: string }>;
    minTime: number;
    maxTime: number;
  }>()

  chartData.value.forEach(series => {
    const minTime = Math.min(sharedState.rulerStart!.time, sharedState.rulerEnd!.time)
    const maxTime = Math.max(sharedState.rulerStart!.time, sharedState.rulerEnd!.time)

    let totalDamage = 0
    let count = 0
    let seriesMinTime = Infinity
    let seriesMaxTime = -Infinity

    series.data.forEach((point: any) => {
      if (point.time >= minTime && point.time <= maxTime) {
        totalDamage += point.singleDamage || 0
        count++
        if (point.time < seriesMinTime) seriesMinTime = point.time
        if (point.time > seriesMaxTime) seriesMaxTime = point.time
      }
    })

    const dps = seconds > 0 ? totalDamage / seconds : 0

    seriesStats.push({
      name: series.name,
      damage: totalDamage,
      dps: dps,
      count: count
    })

    // 汇总到攻击者维度
    const attackerId = (series as any).attackerId
    const attackerName = (series as any).attackerName
    const skillId = (series as any).skillId
    const skillName = series.name.split(' - ').slice(1).join(' - ') || series.name

    if (attackerId && attackerName) {
      if (!attackerStatsMap.has(attackerId)) {
        attackerStatsMap.set(attackerId, {
          name: attackerName,
          damage: 0,
          skills: new Map<number, { count: number; damage: number; skillName: string }>(),
          minTime: seriesMinTime,
          maxTime: seriesMaxTime
        })
      }
      const attackerStat = attackerStatsMap.get(attackerId)!
      attackerStat.damage += totalDamage

      if (seriesMinTime < attackerStat.minTime) attackerStat.minTime = seriesMinTime
      if (seriesMaxTime > attackerStat.maxTime) attackerStat.maxTime = seriesMaxTime

      if (!attackerStat.skills.has(skillId)) {
        attackerStat.skills.set(skillId, { count: 0, damage: 0, skillName })
      }
      attackerStat.skills.get(skillId)!.count += count
      attackerStat.skills.get(skillId)!.damage += totalDamage
    }
  })

  // 转换为数组并排序
  const attackerStats = Array.from(attackerStatsMap.entries())
    .map(([id, stat]) => ({
      name: stat.name,
      damage: stat.damage,
      dps: seconds > 0 ? stat.damage / seconds : 0,
      skills: Array.from(stat.skills.entries()).map(([skillId, skillStat]) => ({
        skillId,
        ...skillStat
      })),
      minTime: stat.minTime,
      maxTime: stat.maxTime
    }))
    .sort((a, b) => {
      if (a.maxTime !== b.maxTime) {
        return a.maxTime - b.maxTime
      }
      return a.minTime - b.minTime
    })

  return {
    timeDiff: timeText,
    seconds: seconds,
    seriesStats: seriesStats,
    attackerStats: attackerStats
  }
})

/** 标尺信息面板 X 坐标 */
const rulerInfoBoxX = computed(() => {
  if (!sharedState.rulerStart || !sharedState.rulerEnd) return 0
  return 10
})

/** 标尺信息面板 Y 坐标 */
const rulerInfoBoxY = computed(() => {
  return 40
})


/** 重置视图：恢复默认缩放和偏移 */
function resetView() {
  sharedState.timeOffset = 0
  sharedState.timeScale = 1
  sharedState.rulerStart = null
  sharedState.rulerEnd = null
  sharedState.redrawTrigger++
}

/** 切换标尺模式 */
function toggleRulerMode() {
  sharedState.rulerMode = !sharedState.rulerMode
  if (!sharedState.rulerMode) {
    sharedState.rulerStart = null
    sharedState.rulerEnd = null
    sharedState.redrawTrigger++
  }
}

/** 清除标尺 */
function clearRuler() {
  sharedState.rulerStart = null
  sharedState.rulerEnd = null
  sharedState.redrawTrigger++
}

/** 清除所有图表状态 */
function clearAllChartState() {
  sharedState.highlightedSkill = null
  sharedState.rulerStart = null
  sharedState.rulerEnd = null
  sharedState.timeOffset = 0
  sharedState.timeScale = 1
  sharedState.rulerMode = false
  sharedState.redrawTrigger++
}

// 监听数据源变化，同步更新共享状态
watch([chartData, () => appStore.chartTimeRange, () => appStore.isShowingHistory, () => appStore.selectedTarget], () => {
  sharedState.globalTimeRange = { ...appStore.chartTimeRange }
  sharedState.isShowingHistory = appStore.isShowingHistory
  sharedState.selectedTarget = appStore.selectedTarget
  sharedState.redrawTrigger++
}, { immediate: true })

onMounted(() => {
  window.addEventListener('clearChartState', clearAllChartState)
})

onUnmounted(() => {
  window.removeEventListener('clearChartState', clearAllChartState)

  clearAllChartState()
})
</script>

<template>
  <div class="chart-panel" :style="{ height: panelHeight + 'px' }">
    <!-- 图表头部：标题 + 工具栏 -->
    <div class="chart-header">
      <span class="chart-title-text">{{ chartTitle }}</span>

      <!-- 标尺状态提示 -->
      <span v-if="rulerStatusText" class="ruler-status-text">{{ rulerStatusText }}</span>

      <div class="chart-header-actions">
        <!-- 标尺工具按钮 -->
        <button
          :class="['ruler-btn', { active: sharedState.rulerMode }]"
          @click="toggleRulerMode"
          title="标尺工具：点击启用后，在图表上点击两次标记时间范围"
        >
          📏 标尺
        </button>
        <!-- 清除标尺按钮 -->
        <button v-if="sharedState.rulerStart && sharedState.rulerEnd" class="clear-ruler-btn" @click="clearRuler" title="清除标尺">✕</button>

        <!-- 缩放级别显示 -->
        <div class="zoom-level">📏 {{ tickIntervalText }}</div>

        <!-- 重置视图按钮 -->
        <button v-if="sharedState.timeOffset > 0 || sharedState.timeScale !== 1" class="reset-pan-btn" @click="resetView" title="重置视图">⟲ 重置</button>
      </div>
    </div>

    <!-- 图表主体：按玩家分组显示 -->
    <div class="chart-body">
      <div
        v-for="group in playerGroups"
        :key="group.name"
        class="player-chart-wrapper"
      >
        <PlayerChart
          :player-data="group.data"
          :player-name="group.name"
        />
      </div>
    </div>

    <!-- 标尺信息面板：显示标尺范围内的伤害统计 -->
    <div
      v-if="rulerInfo && sharedState.rulerStart && sharedState.rulerEnd"
      class="ruler-info-box"
        :style="{ left: rulerInfoBoxX + 'px', top: rulerInfoBoxY + 'px' }"
      >
        <div class="ruler-time">{{ rulerInfo.timeDiff }}</div>
        <div class="ruler-players">
          <div
            v-for="(stat, idx) in rulerInfo.attackerStats.filter(s => s.damage > 0)"
            :key="idx"
            class="ruler-player-column"
          >
            <div class="ruler-player" :style="{ '--player-color': getSkillColorForPlayer(stat.name) }">
              <span class="player-dot"></span>
              <span class="player-name">{{ stat.name }}</span>
              <span class="player-damage">{{ formatDamage(stat.damage) }}</span>
              <span class="player-dps">| {{ formatDamage(Math.round(stat.dps)) }}/s</span>
            </div>
            <div
              v-for="skill in stat.skills.filter(s => s.count > 0)"
              :key="skill.skillId"
              class="ruler-skill"
            >
              <span class="skill-name">{{ skill.skillName }}</span>
              <span class="skill-count">{{ skill.count }}次</span>
              <span class="skill-damage">| {{ formatDamage(skill.damage) }}</span>
            </div>
          </div>
        </div>
      </div>

  </div>
</template>

<style lang="scss" scoped>
/* 图表面板容器 */
.chart-panel {
  width: 100%;
  border-top: 1px solid rgba(255, 255, 255, 0.1);
  display: flex;
  flex-direction: column;
  position: relative;
  flex-shrink: 0;
}

/* 图表头部 */
.chart-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 8px;
  background: rgba(30, 30, 30, 0.8);
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  font-size: 11px;
  color: #aaa;
  flex-shrink: 0;
}

.chart-title-text {
  flex: 1;
}

/* 标尺状态提示文本（带呼吸动画） */
.ruler-status-text {
  color: #ffeb3b;
  font-size: 11px;
  font-weight: bold;
  padding: 2px 8px;
  background: rgba(255, 235, 59, 0.1);
  border-radius: 3px;
  margin: 0 8px;
  white-space: nowrap;
  animation: pulse 2s ease-in-out infinite;
}

@keyframes pulse {
  0%, 100% {
    opacity: 1;
  }
  50% {
    opacity: 0.7;
  }
}

/* 头部操作按钮区 */
.chart-header-actions {
  display: flex;
  align-items: center;
  gap: 4px;
}

/* 图表主体区域 */
.chart-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  position: relative;
}

/* 玩家图表包装器 */
.player-chart-wrapper {
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);

  &:last-child {
    border-bottom: none;
  }
}

/* 标尺工具按钮 */
.ruler-btn {
  background: rgba(255, 235, 59, 0.2);
  border: none;
  color: #ffeb3b;
  cursor: pointer;
  font-size: 9px;
  padding: 3px 8px;
  border-radius: 3px;
  transition: all 0.2s;
  margin-left: 4px;

  &:hover {
    background: rgba(255, 235, 59, 0.3);
    color: #fff;
  }

  &.active {
    background: rgba(255, 235, 59, 0.5);
    color: #fff;
    box-shadow: 0 0 8px rgba(255, 235, 59, 0.5);
  }
}

/* 清除标尺按钮 */
.clear-ruler-btn {
  background: rgba(255, 82, 82, 0.2);
  border: none;
  color: #ff5252;
  cursor: pointer;
  font-size: 10px;
  padding: 2px 6px;
  border-radius: 3px;
  transition: all 0.2s;
  margin-left: 2px;
  line-height: 1;

  &:hover {
    background: rgba(255, 82, 82, 0.4);
    color: #fff;
  }
}

/* 缩放级别显示 */
.zoom-level {
  color: #42a5f5;
  font-size: 10px;
  font-weight: bold;
  padding: 2px 6px;
  background: rgba(66, 165, 245, 0.1);
  border-radius: 3px;
  margin-left: 4px;
}

/* 重置视图按钮 */
.reset-pan-btn {
  margin-left: auto;
  background: rgba(66, 165, 245, 0.2);
  border: none;
  color: #42a5f5;
  cursor: pointer;
  font-size: 9px;
  padding: 3px 8px;
  border-radius: 3px;
  transition: all 0.2s;

  &:hover {
    background: rgba(66, 165, 245, 0.3);
    color: #fff;
  }
}


/* 标尺信息面板 */
.ruler-info-box {
  position: absolute;
  background: rgba(0, 0, 0, 0.9);
  border: 1px solid #ffeb3b;
  border-radius: 4px;
  padding: 8px;
  pointer-events: none;
  z-index: 50;
  font-size: 11px;
}

/* 标尺时间显示 */
.ruler-time {
  text-align: center;
  color: #ffeb3b;
  font-weight: bold;
  margin-bottom: 6px;
  padding-bottom: 4px;
  border-bottom: 1px solid rgba(255, 235, 59, 0.3);
}

/* 标尺玩家统计区 */
.ruler-players {
  display: flex;
  gap: 20px;
}

/* 标尺玩家列 */
.ruler-player-column {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

/* 标尺玩家行 */
.ruler-player {
  display: flex;
  align-items: center;
  gap: 4px;
  color: #fff;
  font-weight: bold;

  .player-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background-color: var(--player-color);
    flex-shrink: 0;
  }

  .player-name {
    margin-right: 4px;
  }

  .player-damage {
    color: #42a5f5;
  }

  .player-dps {
    color: #66bb6a;
    font-weight: normal;
  }
}

/* 标尺技能行 */
.ruler-skill {
  display: flex;
  align-items: center;
  gap: 4px;
  color: #aaa;
  padding-left: 12px;

  .skill-name {
    font-size: 10px;
  }

  .skill-count {
    font-size: 10px;
    color: #ffca28;
  }

  .skill-damage {
    font-size: 10px;
    color: #42a5f5;
  }
}
</style>
