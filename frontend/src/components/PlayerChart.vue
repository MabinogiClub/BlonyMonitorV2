<script setup lang="ts">
/**
 * 单玩家图表子组件
 * 从 ChartPanel 拆分出来，每个玩家独立的 Canvas 图表
 * 包含折线图/柱状图渲染、交互逻辑（平移、缩放、标尺）、Tooltip、高亮技能
 */
import { ref, computed, watch, onMounted, onUnmounted, nextTick, inject } from 'vue'
import { useAppStore } from '../stores/app'
import { COLORS, formatDamage, formatSeconds } from '../composables/useUtils'
import { drawBossHPOverlay } from '../composables/useBossHPOverlay'

/**
 * 将最大伤害值向上取整到合适的刻度
 * 用于 Y 轴刻度计算，确保刻度值为整数且美观
 */
function roundUpMaxDamage(maxDamage: number): number {
  if (maxDamage <= 0) return 0
  let step: number
  if (maxDamage >= 1000000) {
    const million = maxDamage / 1000000
    if (million <= 2) step = 1000000
    else if (million <= 5) step = 2000000
    else step = 5000000
  } else if (maxDamage >= 10000) {
    const tenThousand = maxDamage / 10000
    if (tenThousand <= 2) step = 10000
    else if (tenThousand <= 5) step = 50000
    else step = 100000
  } else if (maxDamage >= 1000) {
    const thousand = maxDamage / 1000
    if (thousand <= 2) step = 1000
    else if (thousand <= 5) step = 2000
    else step = 5000
  } else {
    if (maxDamage <= 200) step = 100
    else if (maxDamage <= 500) step = 200
    else step = 500
  }
  return Math.ceil(maxDamage / step) * step
}

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

const props = defineProps<{
  playerData: any[]
  playerName: string
}>()

const appStore = useAppStore()
// 注入 ChartPanel 提供的共享状态，用于多图表同步
const shared = inject<SharedChartState>('chartSharedState')!

const canvasRef = ref<HTMLCanvasElement | null>(null)
const containerRef = ref<HTMLElement | null>(null)

// 平移状态
const isPanning = ref(false)
let panStartX = 0
let panStartOffset = 0

// 标尺拖拽状态
const isDraggingRulerStart = ref(false)
const isDraggingRulerEnd = ref(false)
const RULER_DRAG_THRESHOLD = 10

interface DamageTooltip {
  skillName: string
  damage: number
  rawDamage?: number
  overflowDamage?: number
  adjusted?: boolean
  lockTriggered?: boolean
  lockThreshold?: number
  isKill?: boolean
  time: number
  timeDiffFromPrev?: number | null
  visible?: boolean
  x?: number
  y?: number
}

type PositionedDamageTooltip = DamageTooltip & {
  x: number
  y: number
  visible: boolean
}

type ChartPadding = {
  top: number
  right: number
  bottom: number
  left: number
}

// Tooltip 状态
const tooltipVisible = ref(false)
const tooltipX = ref(0)
const tooltipY = ref(0)
const tooltipContent = ref<DamageTooltip | null>(null)

// 高亮技能的 Tooltip 列表
const highlightedTooltips = ref<PositionedDamageTooltip[]>([])

// 溢出伤害常驻 Tooltip 列表
const overflowTooltips = ref<PositionedDamageTooltip[]>([])

// 技能颜色缓存，避免重复计算
const skillColorCache = new Map<number, string>()

// 柱状图渐变分桶缓存，避免每个柱子都创建 createLinearGradient
const GRADIENT_BUCKET_SIZE = 10
const gradientCache = new Map<string, CanvasGradient>()
const TOOLTIP_ROW_HEIGHT = 18
const TOOLTIP_VERTICAL_PADDING = 8
const TOOLTIP_HORIZONTAL_PADDING = 16
const TOOLTIP_MAX_WIDTH = 190
const PERSISTENT_OVERFLOW_TOOLTIP_MIN_DAMAGE = 1000

function getPointEffectiveDamage(point: any): number {
  return point.singleDamage ?? point.damage ?? 0
}

function isFullyOverflowedHistoryPoint(point: any): boolean {
  if (!shared.isShowingHistory) return false
  const effectiveDamage = getPointEffectiveDamage(point)
  return effectiveDamage <= 0 &&
    Number(point?.rawDamage ?? 0) > 0 &&
    Number(point?.overflowDamage ?? 0) > 0
}

function getPointVisualDamage(point: any): number {
  const effectiveDamage = getPointEffectiveDamage(point)
  if (isFullyOverflowedHistoryPoint(point)) {
    return 0
  }
  if (shared.isShowingHistory && point.rawDamage && point.rawDamage > effectiveDamage) {
    return point.rawDamage
  }
  return effectiveDamage
}

function getTooltipPayload(skillName: string, point: any, time: number): DamageTooltip {
  const effectiveDamage = shared.isShowingHistory ? getPointEffectiveDamage(point) : (point.damage ?? 0)
  return {
    skillName,
    damage: effectiveDamage,
    rawDamage: point.rawDamage,
    overflowDamage: point.overflowDamage,
    adjusted: point.adjusted,
    lockTriggered: point.lockTriggered,
    lockThreshold: point.lockThreshold,
    isKill: point.isKill,
    time
  }
}

function hasOverflowDamage(point: any): boolean {
  return Number(point?.overflowDamage ?? 0) > 0
}

function hasPersistentOverflowDamage(point: any): boolean {
  if (isFullyOverflowedHistoryPoint(point)) return false
  return Number(point?.overflowDamage ?? 0) >= PERSISTENT_OVERFLOW_TOOLTIP_MIN_DAMAGE || !!point?.lockTriggered
}

function shouldShowOverflowDamage(tip: DamageTooltip): boolean {
  return Number(tip.overflowDamage ?? 0) > 0
}

function shouldShowRawDamage(tip: DamageTooltip): boolean {
  return shouldShowOverflowDamage(tip) && tip.rawDamage !== undefined
}

function formatTooltipDamage(value?: number): string {
  return formatDamage(value ?? 0)
}

function getDamageTooltipHeight(tip: DamageTooltip): number {
  const baseHeight = TOOLTIP_VERTICAL_PADDING * 2 + TOOLTIP_ROW_HEIGHT * 3
  let extraRows = 0

  if (shouldShowRawDamage(tip)) {
    extraRows += 1
  }
  if (shouldShowOverflowDamage(tip)) {
    extraRows += 1
  }
  if (tip.lockTriggered && tip.lockThreshold !== undefined) {
    extraRows += 1
  }
  if (tip.isKill) {
    extraRows += 1
  }
  if (tip.timeDiffFromPrev !== null && tip.timeDiffFromPrev !== undefined) {
    extraRows += 1
  }

  return baseHeight + TOOLTIP_ROW_HEIGHT * extraRows
}

function estimateTooltipTextWidth(text: string): number {
  let width = 0
  for (const char of text) {
    width += char.charCodeAt(0) > 255 ? 12 : 7
  }
  return width
}

function getDamageTooltipWidth(tip: DamageTooltip, minWidth = 72): number {
  const rows = [
    tip.skillName,
    `计入 ${formatDamage(tip.damage)}`,
    shouldShowRawDamage(tip) ? `原始 ${formatTooltipDamage(tip.rawDamage)}` : '',
    shouldShowOverflowDamage(tip) ? `扣除 ${formatTooltipDamage(tip.overflowDamage)}` : '',
    tip.lockTriggered && tip.lockThreshold !== undefined ? `触发锁血 ${Math.round(tip.lockThreshold)}%` : '',
    tip.isKill ? '击杀' : '',
    formatSeconds(tip.time),
    tip.timeDiffFromPrev !== null && tip.timeDiffFromPrev !== undefined ? `间隔 ${formatSeconds(tip.timeDiffFromPrev)}` : ''
  ].filter(Boolean)

  const contentWidth = rows.reduce((max, row) => Math.max(max, estimateTooltipTextWidth(row)), 0)
  return clampTooltipPosition(Math.ceil(contentWidth + TOOLTIP_HORIZONTAL_PADDING), minWidth, TOOLTIP_MAX_WIDTH)
}

function clampTooltipPosition(value: number, min: number, max: number): number {
  if (max < min) return min
  return Math.max(min, Math.min(value, max))
}

function placeTooltipNearPoint(
  pointX: number,
  pointY: number,
  boxWidth: number,
  boxHeight: number,
  bounds: { left: number; right: number; top: number; bottom: number },
  horizontalGap = 8,
  verticalGap = 6
) {
  let x = pointX + horizontalGap
  if (x + boxWidth > bounds.right) {
    x = pointX - boxWidth - horizontalGap
  }
  x = clampTooltipPosition(x, bounds.left, bounds.right - boxWidth)

  let y = pointY - boxHeight - verticalGap
  if (y < bounds.top) {
    y = pointY + verticalGap
  }
  y = clampTooltipPosition(y, bounds.top, bounds.bottom - boxHeight)

  return { x, y }
}

function arrangeTooltipOverlaps(
  tooltips: PositionedDamageTooltip[],
  bounds: { left: number; right: number; top: number; bottom: number },
  verticalGap = 6
) {
  const placed: Array<{ x: number; y: number; width: number; height: number }> = []
  tooltips.sort((a, b) => a.x - b.x || a.y - b.y)

  for (const tip of tooltips) {
    const width = getDamageTooltipWidth(tip)
    const height = getDamageTooltipHeight(tip)
    tip.x = clampTooltipPosition(tip.x, bounds.left, bounds.right - width)
    tip.y = clampTooltipPosition(tip.y, bounds.top, bounds.bottom - height)

    for (let attempts = 0; attempts < 12; attempts++) {
      const blocker = placed.find(box => {
        const overlapsX = tip.x < box.x + box.width + 4 && tip.x + width + 4 > box.x
        const overlapsY = tip.y < box.y + box.height + verticalGap && tip.y + height + verticalGap > box.y
        return overlapsX && overlapsY
      })
      if (!blocker) break

      const belowY = blocker.y + blocker.height + verticalGap
      const aboveY = blocker.y - height - verticalGap
      if (belowY + height <= bounds.bottom) {
        tip.y = belowY
      } else if (aboveY >= bounds.top) {
        tip.y = aboveY
      } else {
        tip.y = clampTooltipPosition(tip.y + height + verticalGap, bounds.top, bounds.bottom - height)
      }
    }

    placed.push({ x: tip.x, y: tip.y, width, height })
  }
}

function placeFloatingTooltip(mouseX: number, mouseY: number, tip: DamageTooltip, rect: DOMRect) {
  const width = getDamageTooltipWidth(tip, 110)
  const height = getDamageTooltipHeight(tip)
  const bounds = {
    left: 4,
    right: rect.width - 4,
    top: 4,
    bottom: rect.height - 4
  }

  let x = mouseX + 10
  if (x + width > bounds.right) {
    x = mouseX - width - 10
  }

  let y = mouseY - 10
  if (y + height > bounds.bottom) {
    y = mouseY - height - 10
  }

  return {
    x: clampTooltipPosition(x, bounds.left, bounds.right - width),
    y: clampTooltipPosition(y, bounds.top, bounds.bottom - height)
  }
}

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

/** 获取分桶缓存的柱状图渐变（视觉零差异，性能大幅提升） */
function getCachedBarGradient(ctx: CanvasRenderingContext2D, color: string, y: number, chartBottom: number): CanvasGradient {
  const bucketY = Math.floor(y / GRADIENT_BUCKET_SIZE) * GRADIENT_BUCKET_SIZE
  const key = `${color}_${bucketY}`

  if (!gradientCache.has(key)) {
    const g = ctx.createLinearGradient(0, bucketY, 0, chartBottom)
    g.addColorStop(0, color)
    g.addColorStop(1, color + '40')
    gradientCache.set(key, g)
  }
  return gradientCache.get(key)!
}

/** 当前可见时间范围（考虑平移和缩放） */
const currentVisibleTimeRange = computed(() => {
  const globalMinTime = shared.globalTimeRange.minTime
  const globalMaxTime = shared.globalTimeRange.maxTime
  const baseTimeRange = globalMaxTime === globalMinTime ? 1 : globalMaxTime - globalMinTime
  const timeRange = baseTimeRange / shared.timeScale
  const minTime = globalMinTime + shared.timeOffset
  const maxTime = minTime + timeRange
  return { minTime, maxTime, timeRange }
})

function bisectLeft(data: any[], target: number): number {
  let lo = 0, hi = data.length
  while (lo < hi) {
    const mid = (lo + hi) >>> 1
    if (data[mid].time < target) lo = mid + 1
    else hi = mid
  }
  return lo
}

/** 预处理数据：使用二分查找过滤可见时间范围，返回索引视图避免 slice 内存分配 */
interface PreprocessedSeries {
  series: any
  startIdx: number
  endIdx: number
}

const preprocessedData = computed((): PreprocessedSeries[] => {
  const visible = currentVisibleTimeRange.value
  const result: PreprocessedSeries[] = []

  props.playerData.forEach(series => {
    if (!series.data || series.data.length === 0) return

    const startIdx = Math.max(0, bisectLeft(series.data, visible.minTime - 1000) - 1)
    const endIdx = Math.min(series.data.length, bisectLeft(series.data, visible.maxTime + 1000) + 1)

    if (startIdx < endIdx) {
      result.push({ series, startIdx, endIdx })
    }
  })

  return result
})

/** 图例数据：显示技能名称、颜色、是否被选中 */
const legendData = computed(() => {
  const skills: Array<{ name: string; color: string; skillId?: number; attackerId?: string; isSelected: boolean }> = []

  props.playerData.forEach((series, index) => {
    // 从 "玩家名 - 技能名" 格式中提取技能名
    const nameParts = series.name.split(' - ')
    const displayName = nameParts.length > 1 ? nameParts.slice(1).join(' - ') : series.name

    const skillId = series.skillId
    const attackerId = series.attackerId
    const isSelected = appStore.selectedSkillFilters.some(
      filter => filter.skillId === skillId && filter.attackerId === attackerId
    )

    const color = skillId !== undefined ? getSkillColor(skillId) : COLORS[index % COLORS.length]

    skills.push({
      name: displayName,
      color,
      skillId,
      attackerId,
      isSelected
    })
  })

  return skills
})

/** 主绘制函数：设置 Canvas 尺寸并调用图表绘制 */
function drawChart() {
  const canvas = canvasRef.value
  if (!canvas) return

  const ctx = canvas.getContext('2d')
  if (!ctx) return

  const container = containerRef.value
  if (!container) return

  const rect = container.getBoundingClientRect()

  // 处理高 DPI 显示
  const dpr = window.devicePixelRatio || 1
  canvas.width = rect.width * dpr
  canvas.height = rect.height * dpr
  canvas.style.width = `${rect.width}px`
  canvas.style.height = `${rect.height}px`
  ctx.scale(dpr, dpr)

  ctx.clearRect(0, 0, rect.width, rect.height)

  gradientCache.clear()

  drawCumulativeChart(ctx, rect)
}

/**
 * 绘制累计伤害图表
 * 实时模式：折线图 + 填充区域
 * 历史模式：柱状图（每次攻击一个柱子）
 */
function drawCumulativeChart(ctx: CanvasRenderingContext2D, rect: DOMRect) {
  const data = preprocessedData.value

  if (!data || data.length === 0) {
    overflowTooltips.value = []
    ctx.fillStyle = '#666'
    ctx.font = '12px Microsoft YaHei'
    ctx.textAlign = 'center'
    ctx.fillText('等待数据...', rect.width / 2, rect.height / 2)
    return
  }

  const padding = { top: 15, right: 20, bottom: 25, left: 50 }
  const chartWidth = rect.width - padding.left - padding.right
  const chartHeight = rect.height - padding.top - padding.bottom

  // 计算最大伤害值
  let maxDamage = 0

  if (shared.isShowingHistory) {
    data.forEach(item => {
      const d = item.series.data
      for (let i = item.startIdx; i < item.endIdx; i++) {
        const singleDamage = getPointVisualDamage(d[i])
        if (singleDamage > maxDamage) maxDamage = singleDamage
      }
    })
  } else {
    data.forEach(item => {
      const d = item.series.data
      for (let i = item.startIdx; i < item.endIdx; i++) {
        if (d[i].damage > maxDamage) maxDamage = d[i].damage
      }
    })
  }

  if (maxDamage > 0) {
    maxDamage = roundUpMaxDamage(maxDamage)
  } else {
    overflowTooltips.value = []
    ctx.fillStyle = '#666'
    ctx.font = '12px Microsoft YaHei'
    ctx.textAlign = 'center'
    ctx.fillText('等待数据...', rect.width / 2, rect.height / 2)
    return
  }

  // 获取全局时间范围
  let globalMinTime = shared.globalTimeRange.minTime
  let globalMaxTime = shared.globalTimeRange.maxTime

  // 如果全局时间范围无效，从数据中计算
  if (globalMinTime === 0 || globalMaxTime === 0 || globalMinTime > globalMaxTime) {
    let dataMinTime = Infinity
    let dataMaxTime = -Infinity
    data.forEach(item => {
      const d = item.series.data
      for (let i = item.startIdx; i < item.endIdx; i++) {
        if (d[i].time < dataMinTime) dataMinTime = d[i].time
        if (d[i].time > dataMaxTime) dataMaxTime = d[i].time
      }
    })
    if (dataMinTime !== Infinity) globalMinTime = dataMinTime
    if (dataMaxTime !== -Infinity) globalMaxTime = dataMaxTime
  }

  const baseTimeRange = globalMaxTime === globalMinTime ? 1 : globalMaxTime - globalMinTime
  const timeRange = baseTimeRange / shared.timeScale
  const minTime = globalMinTime + shared.timeOffset
  const maxTime = minTime + timeRange

  // 历史模式或有选中目标时使用相对时间
  const useRelativeTime = shared.isShowingHistory || !!shared.selectedTarget

  // 绘制坐标轴
  ctx.strokeStyle = 'rgba(255, 255, 255, 0.2)'
  ctx.lineWidth = 1
  ctx.beginPath()
  ctx.moveTo(padding.left, padding.top)
  ctx.lineTo(padding.left, rect.height - padding.bottom)
  ctx.lineTo(rect.width - padding.right, rect.height - padding.bottom)
  ctx.stroke()

  // 绘制 Y 轴刻度（使用 sqrt 比例）
  ctx.fillStyle = '#888'
  ctx.font = '11px Microsoft YaHei'
  ctx.textAlign = 'right'

  const tickCount = 5
  const sqrtMaxDamage = Math.sqrt(maxDamage)

  for (let i = 0; i < tickCount; i++) {
    const y = padding.top + (chartHeight * i / tickCount)

    // sqrt 比例：实际伤害 = (归一化值 * sqrtMaxDamage)^2
    const normalizedValue = 1 - (i / tickCount)
    const actualDamage = Math.pow(normalizedValue * sqrtMaxDamage, 2)

    let label: string
    if (actualDamage >= 1000000) {
      label = `${(actualDamage / 1000000).toFixed(2)}m`
    } else if (actualDamage >= 1000) {
      label = `${(actualDamage / 1000).toFixed(0)}k`
    } else {
      label = actualDamage.toFixed(0)
    }

    ctx.fillText(label, padding.left - 5, y + 3)

    // 绘制水平网格线
    if (i > 0 && i < tickCount) {
      ctx.strokeStyle = 'rgba(255, 255, 255, 0.1)'
      ctx.beginPath()
      ctx.moveTo(padding.left, y)
      ctx.lineTo(rect.width - padding.right, y)
      ctx.stroke()
    }
  }

  // 绘制 X 轴时间刻度
  ctx.fillStyle = '#888'
  ctx.font = '11px Microsoft YaHei'
  ctx.textAlign = 'center'

  const minTickSpacing = 80
  const timeTickCount = Math.max(5, Math.floor(chartWidth / minTickSpacing))
  const timeTickInterval = timeRange / timeTickCount

  for (let i = 0; i <= timeTickCount; i++) {
    const time = minTime + (timeTickInterval * i)

    const x = padding.left + ((time - minTime) / timeRange) * chartWidth

    // 格式化时间标签
    let timeStr: string
    if (useRelativeTime) {
      const seconds = Math.floor((time - globalMinTime) / 1000)
      if (seconds >= 3600) {
        const hours = Math.floor(seconds / 3600)
        const mins = Math.floor((seconds % 3600) / 60)
        const secs = seconds % 60
        timeStr = `${hours}:${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`
      } else if (seconds >= 60) {
        const mins = Math.floor(seconds / 60)
        const secs = seconds % 60
        timeStr = `${mins}:${secs.toString().padStart(2, '0')}`
      } else {
        timeStr = `${seconds}s`
      }
    } else {
      const date = new Date(time)
      const hours = date.getHours()
      const mins = date.getMinutes()
      const secs = date.getSeconds()
      if (hours > 0) {
        timeStr = `${hours}:${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`
      } else {
        timeStr = `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`
      }
    }

    ctx.fillText(timeStr, x, rect.height - padding.bottom + 15)

    // 绘制垂直网格线
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.1)'
    ctx.beginPath()
    ctx.moveTo(x, padding.top)
    ctx.lineTo(x, rect.height - padding.bottom)
    ctx.stroke()
  }

  // 绘制标尺区域
  if (shared.rulerStart && shared.rulerEnd) {
    const rulerStartX = padding.left + ((shared.rulerStart.time - minTime) / timeRange) * chartWidth
    const rulerEndX = padding.left + ((shared.rulerEnd.time - minTime) / timeRange) * chartWidth
    const minX = Math.min(rulerStartX, rulerEndX)
    const maxX = Math.max(rulerStartX, rulerEndX)

    // 标尺区域半透明背景
    ctx.fillStyle = 'rgba(255, 235, 59, 0.15)'
    ctx.fillRect(minX, padding.top, maxX - minX, chartHeight)

    // 标尺边界虚线
    ctx.strokeStyle = '#ffeb3b'
    ctx.lineWidth = 2
    ctx.setLineDash([5, 3])
    ctx.beginPath()
    ctx.moveTo(rulerStartX, padding.top)
    ctx.lineTo(rulerStartX, rect.height - padding.bottom)
    ctx.moveTo(rulerEndX, padding.top)
    ctx.lineTo(rulerEndX, rect.height - padding.bottom)
    ctx.stroke()
    ctx.setLineDash([])

    // 标尺拖拽手柄
    const handleSize = 8
    const handleY = padding.top + chartHeight / 2 - handleSize / 2

    ctx.fillStyle = isDraggingRulerStart.value ? '#ffeb3b' : 'rgba(255, 235, 59, 0.8)'
    ctx.fillRect(rulerStartX - handleSize / 2, handleY, handleSize, handleSize)
    ctx.strokeStyle = '#fff'
    ctx.lineWidth = 1
    ctx.strokeRect(rulerStartX - handleSize / 2, handleY, handleSize, handleSize)

    ctx.fillStyle = isDraggingRulerEnd.value ? '#ffeb3b' : 'rgba(255, 235, 59, 0.8)'
    ctx.fillRect(rulerEndX - handleSize / 2, handleY, handleSize, handleSize)
    ctx.strokeStyle = '#fff'
    ctx.lineWidth = 1
    ctx.strokeRect(rulerEndX - handleSize / 2, handleY, handleSize, handleSize)
  }

  // 绘制数据系列
  data.forEach((item, index) => {
    const series = item.series
    const { startIdx, endIdx } = item
    if (endIdx - startIdx < 1) return

    const skillId = series.skillId
    const color = skillId !== undefined ? getSkillColor(skillId) : COLORS[index % COLORS.length]
    ctx.strokeStyle = color
    ctx.lineWidth = 2

    const allData = series.data

    if (shared.isShowingHistory) {
      const barWidth = 3
      const minLabelSpacing = 30
      let lastLabelX = -Infinity

      for (let i = startIdx; i < endIdx; i++) {
        const point = allData[i]
        if (isFullyOverflowedHistoryPoint(point)) continue
        const x = padding.left + ((point.time - minTime) / timeRange) * chartWidth

        const singleDamage = getPointVisualDamage(point)
        if (singleDamage <= 0) continue
        const normalizedHeight = Math.sqrt(singleDamage) / sqrtMaxDamage
        const y = padding.top + (1 - normalizedHeight) * chartHeight
        const barHeight = chartHeight * normalizedHeight

        const itemSkillId = series?.skillId
        const itemColor = itemSkillId !== undefined ? getSkillColor(itemSkillId) : COLORS[index % COLORS.length]

        ctx.fillStyle = getCachedBarGradient(ctx, itemColor, y, padding.top + chartHeight)
        ctx.fillRect(x - barWidth / 2, y, barWidth, barHeight)

        ctx.strokeStyle = itemColor
        ctx.lineWidth = 1
        ctx.strokeRect(x - barWidth / 2, y, barWidth, barHeight)

        if (hasPersistentOverflowDamage(point)) {
          const markerY = Math.max(padding.top + 9, y - 4)
          ctx.fillStyle = '#ff2f6d'
          ctx.strokeStyle = '#2a0010'
          ctx.lineWidth = 1
          ctx.beginPath()
          ctx.moveTo(x, markerY)
          ctx.lineTo(x - 4, markerY - 7)
          ctx.lineTo(x + 4, markerY - 7)
          ctx.closePath()
          ctx.fill()
          ctx.stroke()
        }

        if (x - lastLabelX >= minLabelSpacing) {
          let damageLabel: string
          const labelDamage = getPointVisualDamage(point)
          if (labelDamage >= 1000000) {
            damageLabel = `${(labelDamage / 1000000).toFixed(1)}m`
          } else if (labelDamage >= 1000) {
            damageLabel = `${(labelDamage / 1000).toFixed(0)}k`
          } else {
            damageLabel = `${Math.round(labelDamage)}`
          }

          ctx.font = 'bold 9px Microsoft YaHei'
          ctx.textAlign = 'center'
          ctx.textBaseline = 'bottom'
          ctx.strokeStyle = '#000'
          ctx.lineWidth = 2
          ctx.strokeText(damageLabel, x, y - 6)
          ctx.fillStyle = itemColor
          ctx.fillText(damageLabel, x, y - 6)

          lastLabelX = x
        }
      }
    } else {
      // 实时模式：绘制折线图
      ctx.beginPath()
      let started = false
      for (let i = startIdx; i < endIdx; i++) {
        const point = allData[i]
        const x = padding.left + ((point.time - minTime) / timeRange) * chartWidth
        const normalizedDamage = Math.sqrt(point.damage) / sqrtMaxDamage
        const y = padding.top + (1 - normalizedDamage) * chartHeight

        if (!started) {
          ctx.moveTo(x, y)
          started = true
        } else {
          ctx.lineTo(x, y)
        }
      }
      ctx.stroke()

      // 绘制填充区域
      const firstPoint = allData[startIdx]
      const lastPoint = allData[endIdx - 1]
      if (firstPoint && lastPoint) {
        const firstX = padding.left + ((firstPoint.time - minTime) / timeRange) * chartWidth
        const lastX = padding.left + ((lastPoint.time - minTime) / timeRange) * chartWidth
        const bottomY = padding.top + chartHeight

        ctx.beginPath()
        started = false
        for (let i = startIdx; i < endIdx; i++) {
          const point = allData[i]
          const x = padding.left + ((point.time - minTime) / timeRange) * chartWidth
          const normalizedDamage = Math.sqrt(point.damage) / sqrtMaxDamage
          const y = padding.top + (1 - normalizedDamage) * chartHeight

          if (!started) {
            ctx.moveTo(x, y)
            started = true
          } else {
            ctx.lineTo(x, y)
          }
        }
        ctx.lineTo(lastX, bottomY)
        ctx.lineTo(firstX, bottomY)
        ctx.closePath()

        // 渐变填充
        const gradient = ctx.createLinearGradient(0, padding.top, 0, bottomY)
        gradient.addColorStop(0, color + '40')
        gradient.addColorStop(1, color + '05')
        ctx.fillStyle = gradient
        ctx.fill()
      }

      // 高亮技能时绘制数据点圆点
      if (shared.highlightedSkill) {
        const hlSkillId = shared.highlightedSkill.skillId
        const hlAttackerId = shared.highlightedSkill.attackerId

        if (skillId === hlSkillId && series.attackerId === hlAttackerId) {
          for (let i = startIdx; i < endIdx; i++) {
            const point = allData[i]
            const x = padding.left + ((point.time - minTime) / timeRange) * chartWidth
            const normalizedDamage = Math.sqrt(point.damage) / sqrtMaxDamage
            const y = padding.top + (1 - normalizedDamage) * chartHeight

            ctx.fillStyle = color
            ctx.beginPath()
            ctx.arc(x, y, 4, 0, Math.PI * 2)
            ctx.fill()

            ctx.strokeStyle = '#fff'
            ctx.lineWidth = 1
            ctx.beginPath()
            ctx.arc(x, y, 4, 0, Math.PI * 2)
            ctx.stroke()
          }
        }
      }
    }
  })

  if (shared.isShowingHistory) {
    drawBossHPOverlay(ctx, padding.left, rect.width - padding.right, padding.top, rect.height - padding.bottom, minTime, maxTime, shared.selectedTarget?.name, !!shared.selectedTarget?.deathTime)
  }

  calculateOverflowTooltips(rect, padding, chartWidth, chartHeight, minTime, timeRange, sqrtMaxDamage)
}

function calculateOverflowTooltips(
  rect: DOMRect,
  padding: ChartPadding,
  chartWidth: number,
  chartHeight: number,
  minTime: number,
  timeRange: number,
  sqrtMaxDamage: number
) {
  if (!shared.isShowingHistory || timeRange <= 0 || sqrtMaxDamage <= 0) {
    overflowTooltips.value = []
    return
  }

  const verticalGap = 6
  const chartRight = rect.width - padding.right
  const chartBottom = rect.height - padding.bottom
  const bounds = {
    left: padding.left,
    right: chartRight,
    top: padding.top,
    bottom: chartBottom
  }
  const tooltips: PositionedDamageTooltip[] = []

  preprocessedData.value.forEach(item => {
    const series = item.series
    const nameParts = series.name.split(' - ')
    const skillName = nameParts.length > 1 ? nameParts.slice(1).join(' - ') : series.name
    const points = series.data

    for (let i = item.startIdx; i < item.endIdx; i++) {
      const point = points[i]
      if (!hasPersistentOverflowDamage(point)) continue

      const x = padding.left + ((point.time - minTime) / timeRange) * chartWidth
      if (x < padding.left - 20 || x > chartRight + 20) continue

      const damage = getPointVisualDamage(point)
      const normalizedDamage = Math.sqrt(damage) / sqrtMaxDamage
      const y = padding.top + (1 - normalizedDamage) * chartHeight
      const payload = getTooltipPayload(skillName, point, (point.time - shared.globalTimeRange.minTime) / 1000)
      const boxWidth = getDamageTooltipWidth(payload, 110)
      const boxHeight = getDamageTooltipHeight(payload)
      const position = placeTooltipNearPoint(x, y, boxWidth, boxHeight, bounds, 8, verticalGap)

      tooltips.push({
        ...payload,
        x: position.x,
        y: position.y,
        visible: true
      })
    }
  })

  arrangeTooltipOverlaps(tooltips, bounds, verticalGap)

  overflowTooltips.value = tooltips
}

/** 鼠标按下：开始平移或标尺操作 */
function startPan(event: MouseEvent) {
  if (event.button !== 0) return

  const container = containerRef.value
  if (!container) return

  const rect = container.getBoundingClientRect()
  const x = event.clientX - rect.left

  // 检查是否点击了标尺手柄
  if (shared.rulerStart && shared.rulerEnd) {
    const padding = { left: 50, right: 20 }
    const chartWidth = rect.width - padding.left - padding.right
    const baseTimeRange = shared.globalTimeRange.maxTime - shared.globalTimeRange.minTime
    const visibleTimeRange = baseTimeRange / shared.timeScale
    const startTime = shared.globalTimeRange.minTime + shared.timeOffset

    const rulerStartX = padding.left + ((shared.rulerStart.time - startTime) / visibleTimeRange) * chartWidth
    const rulerEndX = padding.left + ((shared.rulerEnd.time - startTime) / visibleTimeRange) * chartWidth

    const distanceToStart = Math.abs(x - rulerStartX)
    const distanceToEnd = Math.abs(x - rulerEndX)

    if (distanceToStart <= RULER_DRAG_THRESHOLD) {
      isDraggingRulerStart.value = true
      document.body.style.cursor = 'ew-resize'
      event.preventDefault()
      return
    }

    if (distanceToEnd <= RULER_DRAG_THRESHOLD) {
      isDraggingRulerEnd.value = true
      document.body.style.cursor = 'ew-resize'
      event.preventDefault()
      return
    }
  }

  // 标尺模式下点击设置标尺起止点
  if (shared.rulerMode && (!shared.rulerStart || !shared.rulerEnd)) {
    const padding = { left: 50, right: 20 }
    const chartWidth = rect.width - padding.left - padding.right
    const baseTimeRange = shared.globalTimeRange.maxTime - shared.globalTimeRange.minTime
    const visibleTimeRange = baseTimeRange / shared.timeScale
    const startTime = shared.globalTimeRange.minTime + shared.timeOffset

    const mouseRatio = Math.max(0, Math.min(1, (x - padding.left) / chartWidth))
    const time = startTime + mouseRatio * visibleTimeRange

    if (!shared.rulerStart) {
      shared.rulerStart = { x, time }
      shared.rulerEnd = null
    } else {
      shared.rulerEnd = { x, time }
    }

    shared.redrawTrigger++
    return
  }

  // 开始平移
  isPanning.value = true
  panStartX = event.clientX
  panStartOffset = shared.timeOffset
  document.body.style.cursor = 'grabbing'
  event.preventDefault()
}

/** 鼠标移动：处理标尺拖拽或图表平移 */
function handlePanMove(event: MouseEvent) {
  // 标尺拖拽
  if (isDraggingRulerStart.value || isDraggingRulerEnd.value) {
    const container = containerRef.value
    if (!container) return

    const rect = container.getBoundingClientRect()
    const x = event.clientX - rect.left

    const padding = { left: 50, right: 20 }
    const chartWidth = rect.width - padding.left - padding.right
    const baseTimeRange = shared.globalTimeRange.maxTime - shared.globalTimeRange.minTime
    const visibleTimeRange = baseTimeRange / shared.timeScale
    const startTime = shared.globalTimeRange.minTime + shared.timeOffset

    const mouseRatio = Math.max(0, Math.min(1, (x - padding.left) / chartWidth))
    const time = startTime + mouseRatio * visibleTimeRange

    if (isDraggingRulerStart.value && shared.rulerStart) {
      shared.rulerStart = { x, time }
    } else if (isDraggingRulerEnd.value && shared.rulerEnd) {
      shared.rulerEnd = { x, time }
    }

    shared.redrawTrigger++
    return
  }

  // 图表平移
  if (!isPanning.value) return

  const deltaX = event.clientX - panStartX
  const container = containerRef.value
  if (!container) return

  const rect = container.getBoundingClientRect()
  const chartWidth = rect.width - 70
  const baseTimeRange = shared.globalTimeRange.maxTime - shared.globalTimeRange.minTime
  if (baseTimeRange <= 0) return
  const timeRange = baseTimeRange / shared.timeScale

  const timeDelta = (deltaX / chartWidth) * timeRange
  shared.timeOffset = panStartOffset - timeDelta

  // 限制平移范围
  const maxOffset = Math.max(0, baseTimeRange - baseTimeRange / shared.timeScale)
  shared.timeOffset = Math.max(0, Math.min(shared.timeOffset, maxOffset))

  if (shared.highlightedSkill) {
    calculateHighlightedTooltips()
  }

  shared.redrawTrigger++
}

/** 鼠标松开：停止平移或标尺拖拽 */
function stopPan() {
  if (isDraggingRulerStart.value || isDraggingRulerEnd.value) {
    isDraggingRulerStart.value = false
    isDraggingRulerEnd.value = false
    document.body.style.cursor = ''
    return
  }

  if (isPanning.value) {
    isPanning.value = false
    document.body.style.cursor = ''
  }
}

/**
 * 鼠标滚轮：缩放时间轴
 * 使用预定义的标准时间间隔，确保缩放到合理的刻度
 */
function handleWheel(event: WheelEvent) {
  event.preventDefault()
  event.stopPropagation()

  const container = containerRef.value
  if (!container) return

  const rect = container.getBoundingClientRect()
  const mouseX = event.clientX - rect.left
  const padding = { left: 50, right: 20 }
  const chartWidth = rect.width - padding.left - padding.right

  // 鼠标在图表中的水平比例
  const mouseRatio = Math.max(0, Math.min(1, (mouseX - padding.left) / chartWidth))

  const baseTimeRange = shared.globalTimeRange.maxTime - shared.globalTimeRange.minTime
  if (baseTimeRange <= 0) return
  const globalMinTime = shared.globalTimeRange.minTime

  const oldTimeRange = baseTimeRange / shared.timeScale
  const oldStartTime = globalMinTime + shared.timeOffset

  // 鼠标位置对应的绝对时间
  const mouseAbsoluteTime = oldStartTime + mouseRatio * oldTimeRange

  // 当前每格秒数
  const currentSecondsPerTick = (oldTimeRange / 10) / 1000

  // 标准时间间隔列表
  const standardIntervals = [0.1, 0.5, 1, 2.5, 5, 7.5, 10, 15, 30, 60]

  // 找到当前最接近的标准间隔
  let currentIndex = 0
  let minDiff = Infinity
  standardIntervals.forEach((interval, index) => {
    const diff = Math.abs(interval - currentSecondsPerTick)
    if (diff < minDiff) {
      minDiff = diff
      currentIndex = index
    }
  })

  // 根据滚轮方向选择目标间隔
  let targetIndex = currentIndex
  if (event.deltaY < 0) {
    if (currentIndex > 0) {
      targetIndex = currentIndex - 1
    } else {
      return
    }
  } else {
    if (currentIndex < standardIntervals.length - 1 && shared.timeScale > 1.01) {
      targetIndex = currentIndex + 1
    } else {
      return
    }
  }

  const targetInterval = standardIntervals[targetIndex]

  // 计算新的时间范围和缩放比例
  const newTimeRange = targetInterval * 10 * 1000
  let newTimeScale = baseTimeRange / newTimeRange
  if (newTimeScale < 1) newTimeScale = 1

  // 保持鼠标位置不变
  const newStartTime = mouseAbsoluteTime - mouseRatio * newTimeRange
  shared.timeOffset = newStartTime - globalMinTime
  shared.timeScale = newTimeScale

  // 限制偏移范围
  const maxOffset = Math.max(0, baseTimeRange - baseTimeRange / newTimeScale)
  shared.timeOffset = Math.max(0, Math.min(shared.timeOffset, maxOffset))

  if (shared.highlightedSkill) {
    calculateHighlightedTooltips()
  }

  shared.redrawTrigger++
}

/** 鼠标移动：处理 Tooltip 显示和标尺光标样式 */
function handleRulerHover(event: MouseEvent) {
  if (isDraggingRulerStart.value || isDraggingRulerEnd.value || isPanning.value) return

  const container = containerRef.value
  if (!container) return

  const rect = container.getBoundingClientRect()
  const x = event.clientX - rect.left
  const y = event.clientY - rect.top

  // 只在标准间隔下显示 Tooltip
  const secondsPerTick = (shared.globalTimeRange.maxTime - shared.globalTimeRange.minTime) / shared.timeScale / 10 / 1000
  const validIntervals = [0.1, 0.5, 1, 2.5, 5, 7.5, 10]
  const isValidInterval = validIntervals.some(interval => Math.abs(secondsPerTick - interval) < 0.01)

  if (shared.highlightedSkill) {
    showHighlightedSkillTooltip(x, y)
  } else if (isValidInterval) {
    showTooltip(x, y)
  } else {
    hideTooltip()
  }

  // 标尺光标样式
  if (shared.rulerStart && shared.rulerEnd) {
    const padding = { left: 50, right: 20 }
    const chartWidth = rect.width - padding.left - padding.right
    const baseTimeRange = shared.globalTimeRange.maxTime - shared.globalTimeRange.minTime
    const visibleTimeRange = baseTimeRange / shared.timeScale
    const startTime = shared.globalTimeRange.minTime + shared.timeOffset

    const rulerStartX = padding.left + ((shared.rulerStart.time - startTime) / visibleTimeRange) * chartWidth
    const rulerEndX = padding.left + ((shared.rulerEnd.time - startTime) / visibleTimeRange) * chartWidth

    const distanceToStart = Math.abs(x - rulerStartX)
    const distanceToEnd = Math.abs(x - rulerEndX)

    if (distanceToStart <= RULER_DRAG_THRESHOLD || distanceToEnd <= RULER_DRAG_THRESHOLD) {
      container.style.cursor = 'ew-resize'
    } else {
      container.style.cursor = 'grab'
    }
  } else {
    container.style.cursor = 'grab'
  }
}

/** 显示高亮技能的 Tooltip */
function showHighlightedSkillTooltip(mouseX: number, mouseY: number) {
  if (!shared.highlightedSkill) return

  const container = containerRef.value
  if (!container) return

  const rect = container.getBoundingClientRect()
  const padding = { left: 50, right: 20, top: 10, bottom: 25 }
  const chartWidth = rect.width - padding.left - padding.right

  const baseTimeRange = shared.globalTimeRange.maxTime - shared.globalTimeRange.minTime
  const visibleTimeRange = baseTimeRange / shared.timeScale
  const startTime = shared.globalTimeRange.minTime + shared.timeOffset

  const mouseRatio = Math.max(0, Math.min(1, (mouseX - padding.left) / chartWidth))
  const absoluteTime = startTime + mouseRatio * visibleTimeRange

  // 找到最近的数据点
  let closestPoint: any = null
  let minTimeDiff = Infinity
  let seriesName = ''

  props.playerData.forEach(series => {
    const skillId = series.skillId
    const attackerId = series.attackerId

    if (skillId === shared.highlightedSkill?.skillId && attackerId === shared.highlightedSkill?.attackerId) {
      series.data.forEach((point: any) => {
        if (isFullyOverflowedHistoryPoint(point)) return
        const timeDiff = Math.abs(point.time - absoluteTime)
        if (timeDiff < minTimeDiff) {
          minTimeDiff = timeDiff
          closestPoint = point
          seriesName = series.name
        }
      })
    }
  })

  if (closestPoint && minTimeDiff < 1000) {
    const nameParts = seriesName.split(' - ')
    const skillName = nameParts.length > 1 ? nameParts.slice(1).join(' - ') : seriesName

    const payload = getTooltipPayload(
      skillName,
      closestPoint,
      (closestPoint.time - shared.globalTimeRange.minTime) / 1000
    )
    const position = placeFloatingTooltip(mouseX, mouseY, payload, rect)

    tooltipContent.value = payload
    tooltipX.value = position.x
    tooltipY.value = position.y
    tooltipVisible.value = true
  } else {
    hideTooltip()
  }
}

/** 显示普通 Tooltip（鼠标悬停时） */
function showTooltip(mouseX: number, mouseY: number) {
  const container = containerRef.value
  if (!container) return

  const rect = container.getBoundingClientRect()
  const padding = { left: 50, right: 20, top: 10, bottom: 25 }
  const chartWidth = rect.width - padding.left - padding.right

  const baseTimeRange = shared.globalTimeRange.maxTime - shared.globalTimeRange.minTime
  const visibleTimeRange = baseTimeRange / shared.timeScale
  const startTime = shared.globalTimeRange.minTime + shared.timeOffset

  const mouseRatio = Math.max(0, Math.min(1, (mouseX - padding.left) / chartWidth))
  const absoluteTime = startTime + mouseRatio * visibleTimeRange

  // 找到所有系列中最近的数据点
  let closestPoint: any = null
  let minTimeDiff = Infinity

  props.playerData.forEach(series => {
    series.data.forEach((point: any) => {
      if (isFullyOverflowedHistoryPoint(point)) return
      const timeDiff = Math.abs(point.time - absoluteTime)
      if (timeDiff < minTimeDiff) {
        minTimeDiff = timeDiff
        closestPoint = { ...point, seriesName: series.name }
      }
    })
  })

  if (closestPoint && minTimeDiff < 1000) {
    const nameParts = closestPoint.seriesName.split(' - ')
    const skillName = nameParts.length > 1 ? nameParts.slice(1).join(' - ') : closestPoint.seriesName

    const payload = getTooltipPayload(
      skillName,
      closestPoint,
      (closestPoint.time - shared.globalTimeRange.minTime) / 1000
    )
    const position = placeFloatingTooltip(mouseX, mouseY, payload, rect)

    tooltipContent.value = payload
    tooltipX.value = position.x
    tooltipY.value = position.y
    tooltipVisible.value = true
  } else {
    hideTooltip()
  }
}

/** 隐藏 Tooltip */
function hideTooltip() {
  tooltipVisible.value = false
  tooltipContent.value = null
}

/** 切换技能高亮（点击图例时） */
function toggleSkillHighlight(skillId: number, attackerId?: string) {
  if (shared.highlightedSkill?.skillId === skillId && shared.highlightedSkill?.attackerId === attackerId) {
    shared.highlightedSkill = null
    highlightedTooltips.value = []
  } else {
    shared.highlightedSkill = { skillId, attackerId }
    calculateHighlightedTooltips()
  }
  shared.redrawTrigger++
}

/**
 * 计算高亮技能的 Tooltip 位置
 * 包含防重叠算法：按列分组，垂直方向自动调整位置
 */
function calculateHighlightedTooltips() {
  highlightedTooltips.value = []
  if (!shared.highlightedSkill || !canvasRef.value || !containerRef.value) return

  const container = containerRef.value
  const rect = container.getBoundingClientRect()
  const padding = { left: 50, right: 20, top: 10, bottom: 25 }
  const chartWidth = rect.width - padding.left - padding.right
  const chartHeight = rect.height - padding.top - padding.bottom

  const baseTimeRange = shared.globalTimeRange.maxTime - shared.globalTimeRange.minTime
  const visibleTimeRange = baseTimeRange / shared.timeScale
  const startTime = shared.globalTimeRange.minTime + shared.timeOffset

  // 计算最大伤害值（与 drawCumulativeChart 保持一致）
  let maxDamage = 0

  if (shared.isShowingHistory) {
    preprocessedData.value.forEach(item => {
      const d = item.series.data
      for (let i = item.startIdx; i < item.endIdx; i++) {
        const singleDamage = getPointVisualDamage(d[i])
        if (singleDamage > maxDamage) maxDamage = singleDamage
      }
    })
  } else {
    preprocessedData.value.forEach(item => {
      const d = item.series.data
      for (let i = item.startIdx; i < item.endIdx; i++) {
        if (d[i].damage > maxDamage) maxDamage = d[i].damage
      }
    })
  }

  if (maxDamage === 0) return

  maxDamage = roundUpMaxDamage(maxDamage)
  const sqrtMaxDamage = Math.sqrt(maxDamage)

  const tooltips: PositionedDamageTooltip[] = []

  // 计算每个高亮数据点的 Tooltip 位置
  preprocessedData.value.forEach(item => {
    const series = item.series
    const { startIdx, endIdx } = item
    const skillId = series.skillId
    const attackerId = series.attackerId

    if (skillId === shared.highlightedSkill?.skillId && attackerId === shared.highlightedSkill?.attackerId) {
      let prevTime: number | null = null
      const d = series.data

      for (let i = startIdx; i < endIdx; i++) {
        const point = d[i]
        if (isFullyOverflowedHistoryPoint(point)) continue
        const timeRatio = (point.time - startTime) / visibleTimeRange
        const x = padding.left + timeRatio * chartWidth

        // 跳过图表范围外的点
        if (x < padding.left - 20 || x > rect.width - padding.right + 20) continue

        const damage = shared.isShowingHistory
          ? getPointVisualDamage(point)
          : point.damage
        if (damage <= 0) continue
        const normalizedDamage = Math.sqrt(damage) / sqrtMaxDamage
        const y = padding.top + (1 - normalizedDamage) * chartHeight

        const nameParts = series.name.split(' - ')
        const skillName = nameParts.length > 1 ? nameParts.slice(1).join(' - ') : series.name

        // 计算与上一个点的时间间隔
        let timeDiffFromPrev: number | null = null
        const currentTime = (point.time - shared.globalTimeRange.minTime) / 1000
        if (prevTime !== null) {
          timeDiffFromPrev = currentTime - prevTime
        }
        prevTime = currentTime

        const payload = getTooltipPayload(
          skillName,
          point,
          currentTime
        )
        payload.timeDiffFromPrev = timeDiffFromPrev

        const bounds = {
          left: padding.left,
          right: rect.width - padding.right,
          top: padding.top,
          bottom: rect.height - padding.bottom
        }
        const boxWidth = getDamageTooltipWidth(payload, 100)
        const boxHeight = getDamageTooltipHeight(payload)
        const position = placeTooltipNearPoint(x, y, boxWidth, boxHeight, bounds, 6, 8)

        tooltips.push({
          ...payload,
          x: position.x,
          y: position.y,
          visible: true
        })
      }
    }
  })

  // Tooltip 防重叠算法
  const verticalGap = 8
  const bounds = {
    left: padding.left,
    right: rect.width - padding.right,
    top: padding.top,
    bottom: rect.height - padding.bottom
  }

  // 按列分组（x 坐标接近的 Tooltip 归为同一列）
  const columns: PositionedDamageTooltip[][] = []

  tooltips.forEach(tip => {
    const boxWidth = getDamageTooltipWidth(tip, 100)
    let closestColumn = -1
    let minDiff = Infinity

    columns.forEach((col, colIndex) => {
      if (col.length > 0) {
        const diff = Math.abs(tip.x - col[0].x)
        if (diff < minDiff && diff < boxWidth) {
          minDiff = diff
          closestColumn = colIndex
        }
      }
    })

    if (closestColumn >= 0) {
      columns[closestColumn].push(tip)
    } else {
      columns.push([tip])
    }
  })

  // 同列内按时间排序并调整垂直位置避免重叠
  columns.forEach(column => {
    column.sort((a, b) => b.time - a.time)

    for (let i = 1; i < column.length; i++) {
      const prev = column[i - 1]
      const current = column[i]

      const prevHeight = getDamageTooltipHeight(prev)
      const currentHeight = getDamageTooltipHeight(current)

      const targetY = prev.y - currentHeight - verticalGap

      if (targetY >= padding.top) {
        current.y = targetY
      } else {
        // 上方空间不足，尝试放下方
        const downTargetY = prev.y + prevHeight + verticalGap
        if (downTargetY + currentHeight <= rect.height - padding.bottom) {
          current.y = downTargetY
        } else {
          current.y = Math.max(padding.top, rect.height - padding.bottom - currentHeight)
        }
      }
    }

    arrangeTooltipOverlaps(column, bounds, verticalGap)
  })

  highlightedTooltips.value = tooltips
}

/** 移除技能筛选（点击图例的 ✕ 按钮） */
function removeSkillFilter(skillId: number, attackerId?: string) {
  // 如果移除的是当前高亮的技能，清除高亮状态
  if (shared.highlightedSkill?.skillId === skillId && shared.highlightedSkill?.attackerId === attackerId) {
    shared.highlightedSkill = null
    highlightedTooltips.value = []
  }

  appStore.toggleSkillFilter(skillId, attackerId)

  window.dispatchEvent(new CustomEvent('skillFilterChanged'))
}

/** 窗口大小变化时重绘 */
function handleResize() {
  if (shared.highlightedSkill) {
    calculateHighlightedTooltips()
  }
  drawChart()
}

// 监听数据变化，延迟重绘（防抖 500ms）
let drawTimer: number | null = null
watch(() => props.playerData, () => {
  if (drawTimer) {
    clearTimeout(drawTimer)
  }
  drawTimer = window.setTimeout(() => {
    nextTick(() => {
      drawChart()
      drawTimer = null
    })
  }, 500)
})

// 监听共享重绘触发器，同步所有图表刷新
watch(() => shared.redrawTrigger, () => {
  nextTick(() => {
    if (shared.highlightedSkill) {
      calculateHighlightedTooltips()
    } else {
      highlightedTooltips.value = []
    }
    drawChart()
  })
})

/** 清除本地图表状态 */
function clearLocalState() {
  highlightedTooltips.value = []
  overflowTooltips.value = []
  tooltipVisible.value = false
  tooltipContent.value = null
  isPanning.value = false
  isDraggingRulerStart.value = false
  isDraggingRulerEnd.value = false
  document.body.style.cursor = ''
}

onMounted(() => {
  if (containerRef.value) {
    containerRef.value.addEventListener('mousedown', startPan)
    containerRef.value.addEventListener('wheel', handleWheel, { passive: false })
    containerRef.value.addEventListener('mousemove', handleRulerHover)
    containerRef.value.addEventListener('mouseleave', hideTooltip)
  }
  document.addEventListener('mousemove', handlePanMove)
  document.addEventListener('mouseup', stopPan)
  window.addEventListener('resize', handleResize)
  window.addEventListener('clearChartState', clearLocalState)

  nextTick(() => drawChart())
})

onUnmounted(() => {
  if (containerRef.value) {
    containerRef.value.removeEventListener('mousedown', startPan)
    containerRef.value.removeEventListener('wheel', handleWheel)
    containerRef.value.removeEventListener('mousemove', handleRulerHover)
    containerRef.value.removeEventListener('mouseleave', hideTooltip)
  }
  document.removeEventListener('mousemove', handlePanMove)
  document.removeEventListener('mouseup', stopPan)
  window.removeEventListener('resize', handleResize)
  window.removeEventListener('clearChartState', clearLocalState)
  // 重置光标样式
  document.body.style.cursor = ''

  if (drawTimer) {
    clearTimeout(drawTimer)
  }
})

defineExpose({ drawChart })
</script>

<template>
  <div class="player-chart">
    <!-- 玩家名称 -->
    <div class="player-chart-name">{{ playerName }}</div>
    <!-- 图表容器（Canvas + Tooltip + 标签） -->
    <div ref="containerRef" class="player-chart-container">
      <canvas ref="canvasRef"></canvas>

      <!-- 鼠标悬停 Tooltip -->
      <div
        v-if="tooltipVisible && tooltipContent"
        class="chart-tooltip"
        :style="{ left: tooltipX + 'px', top: tooltipY + 'px' }"
      >
        <div class="tooltip-row">{{ tooltipContent.skillName }}</div>
        <div class="tooltip-row">计入 {{ formatDamage(tooltipContent.damage) }}</div>
        <div v-if="tooltipContent.isKill" class="tooltip-row kill-row">击杀</div>
        <div v-if="shouldShowRawDamage(tooltipContent)" class="tooltip-row overflow-row">
          原始 {{ formatTooltipDamage(tooltipContent.rawDamage) }}
        </div>
        <div v-if="shouldShowOverflowDamage(tooltipContent)" class="tooltip-row overflow-row">
          扣除 {{ formatTooltipDamage(tooltipContent.overflowDamage) }}
        </div>
        <div v-if="tooltipContent.lockTriggered && tooltipContent.lockThreshold !== undefined" class="tooltip-row lock-row">
          触发锁血 {{ Math.round(tooltipContent.lockThreshold) }}%
        </div>
        <div class="tooltip-row">{{ formatSeconds(tooltipContent.time) }}</div>
      </div>

      <!-- 溢出伤害常驻 Tooltip -->
      <div
        v-for="(tooltip, idx) in overflowTooltips"
        :key="`overflow-${idx}-${tooltip.time}`"
        v-show="tooltip.visible"
        class="chart-tooltip overflow-persistent-tooltip"
        :style="{ left: tooltip.x + 'px', top: tooltip.y + 'px' }"
      >
        <div class="tooltip-row overflow-title">{{ tooltip.skillName }}</div>
        <div v-if="shouldShowRawDamage(tooltip)" class="tooltip-row overflow-row">
          造成 {{ formatTooltipDamage(tooltip.rawDamage) }}
        </div>
        <div v-if="shouldShowOverflowDamage(tooltip)" class="tooltip-row overflow-row overflow-cut-row">
          溢出 {{ formatTooltipDamage(tooltip.overflowDamage) }}
        </div>
        <div class="tooltip-row">计入 {{ formatDamage(tooltip.damage) }}</div>
        <div v-if="tooltip.isKill" class="tooltip-row kill-row">击杀</div>
        <div v-if="tooltip.lockTriggered && tooltip.lockThreshold !== undefined" class="tooltip-row lock-row">
          触发锁血 {{ Math.round(tooltip.lockThreshold) }}%
        </div>
        <div class="tooltip-row">{{ formatSeconds(tooltip.time) }}</div>
      </div>

      <!-- 高亮技能的 Tooltip 列表 -->
      <div
        v-for="(tooltip, idx) in highlightedTooltips"
        :key="idx"
        v-show="tooltip.visible"
        class="chart-tooltip highlighted-tooltip"
        :style="{ left: tooltip.x + 'px', top: tooltip.y + 'px' }"
      >
        <div class="tooltip-row">{{ tooltip.skillName }}</div>
        <div class="tooltip-row">计入 {{ formatDamage(tooltip.damage) }}</div>
        <div v-if="tooltip.isKill" class="tooltip-row kill-row">击杀</div>
        <div v-if="shouldShowRawDamage(tooltip)" class="tooltip-row overflow-row">
          原始 {{ formatTooltipDamage(tooltip.rawDamage) }}
        </div>
        <div v-if="shouldShowOverflowDamage(tooltip)" class="tooltip-row overflow-row">
          扣除 {{ formatTooltipDamage(tooltip.overflowDamage) }}
        </div>
        <div v-if="tooltip.lockTriggered && tooltip.lockThreshold !== undefined" class="tooltip-row lock-row">
          触发锁血 {{ Math.round(tooltip.lockThreshold) }}%
        </div>
        <div class="tooltip-row">{{ formatSeconds(tooltip.time) }}</div>
        <div v-if="tooltip.timeDiffFromPrev != null" class="tooltip-row time-diff">
          间隔 {{ formatSeconds(tooltip.timeDiffFromPrev) }}
        </div>
      </div>
    </div>

    <!-- 技能图例 -->
    <div class="player-chart-legend">
      <div
        v-for="(skill, idx) in legendData"
        :key="skill.name"
        class="legend-item"
        :class="{ 'selected': skill.isSelected, 'highlighted': shared.highlightedSkill?.skillId === skill.skillId && shared.highlightedSkill?.attackerId === skill.attackerId }"
        @click="skill.skillId !== undefined ? toggleSkillHighlight(skill.skillId, skill.attackerId) : null"
      >
        <div class="legend-color" :style="{ background: skill.color }"></div>
        <span class="legend-name">{{ skill.name }}</span>
        <button
          v-if="skill.isSelected && skill.skillId !== undefined"
          class="remove-filter-btn"
          @click.stop="removeSkillFilter(skill.skillId!, skill.attackerId)"
          title="取消该技能筛选"
        >
          ✕
        </button>
      </div>
    </div>
  </div>
</template>

<style lang="scss" scoped>
/* 单玩家图表容器 */
.player-chart {
  display: flex;
  flex-direction: column;
  height: 200px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

/* 玩家名称标签 */
.player-chart-name {
  padding: 3px 8px;
  font-size: 11px;
  font-weight: bold;
  color: #fff;
  background: rgba(30, 30, 30, 0.6);
  flex-shrink: 0;
}

/* Canvas 容器 */
.player-chart-container {
  flex: 1;
  padding: 2px;
  position: relative;
  min-height: 0;
  cursor: grab;

  &:active {
    cursor: grabbing;
  }
}

canvas {
  width: 100%;
  height: 100%;
}

/* 技能图例栏 */
.player-chart-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 4px 8px;
  background: rgba(30, 30, 30, 0.4);
  font-size: 10px;
  flex-shrink: 0;
}

/* 图例项 */
.legend-item {
  display: flex;
  align-items: center;
  gap: 4px;
  cursor: pointer;

  &.selected {
    background: rgba(66, 165, 245, 0.15);
    border-radius: 3px;
    padding: 1px 4px;
    margin: -1px -4px;
  }

  &.highlighted {
    background: rgba(255, 235, 59, 0.3);
    border-radius: 3px;
    padding: 1px 4px;
    margin: -1px -4px;
    box-shadow: 0 0 8px rgba(255, 235, 59, 0.5);
  }

  &:hover {
    background: rgba(255, 255, 255, 0.1);
    border-radius: 3px;
  }
}

/* 图例颜色块 */
.legend-color {
  width: 10px;
  height: 10px;
  border-radius: 2px;
}

/* 图例技能名称 */
.legend-name {
  color: #aaa;
  max-width: 100px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 移除筛选按钮 */
.remove-filter-btn {
  background: none;
  border: none;
  color: #ff5252;
  cursor: pointer;
  font-size: 9px;
  padding: 0 2px;
  line-height: 1;
  opacity: 0.7;

  &:hover {
    opacity: 1;
    transform: scale(1.2);
  }
}

/* 鼠标悬停 Tooltip */
.chart-tooltip {
  position: absolute;
  background: rgba(30, 30, 30, 0.95);
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 4px;
  padding: 8px 12px;
  color: #fff;
  font-size: 12px;
  pointer-events: none;
  z-index: 100;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
  white-space: nowrap;
  width: auto;
  max-width: 190px;
  box-sizing: border-box;
  overflow: hidden;
}

/* 高亮技能 Tooltip */
.chart-tooltip.highlighted-tooltip {
  background: rgba(30, 30, 30, 0.95);
  border: 1px solid rgba(255, 235, 59, 0.8);
  box-shadow: 0 0 8px rgba(255, 235, 59, 0.3);
  padding: 4px 8px;
  font-size: 12px;
  z-index: 99;
  white-space: nowrap;
  width: auto;
  max-width: 190px;
}

/* 溢出伤害常驻 Tooltip */
.chart-tooltip.overflow-persistent-tooltip {
  background: transparent;
  border: none;
  box-shadow: none;
  color: #ffd0dc;
  padding: 4px 8px;
  font-size: 12px;
  z-index: 101;
  max-width: 190px;
}

.overflow-title {
  color: #ff8faf;
}

.overflow-row {
  color: #ffd0dc;
}

.overflow-cut-row {
  color: #ff5b87;
}

/* 时间间隔行 */
.time-diff {
  color: #ffca28;
  font-size: 11px;
  padding-top: 2px;
  border-top: 1px solid rgba(255, 202, 40, 0.3);
  margin-top: 2px;
}

.lock-row {
  color: #ff5252;
  font-weight: 700;
}

.kill-row {
  color: #ffca28;
  font-weight: 700;
}

/* Tooltip 行 */
.tooltip-row {
  margin: 2px 0;
  line-height: 1.4;
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>
