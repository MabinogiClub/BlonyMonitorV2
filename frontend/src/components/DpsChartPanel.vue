<script setup lang="ts">
/**
 * DPS趋势图组件
 * 显示每秒DPS变化曲线
 * 支持多种窗口大小和百分比模式
 */

import { ref, shallowRef, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { useAppStore } from '../stores/app'
import { COLORS, formatDps } from '../composables/useUtils'
import { drawBossHPOverlay } from '../composables/useBossHPOverlay'

// 获取应用状态
const appStore = useAppStore()

// Canvas 引用
const canvasRef = ref<HTMLCanvasElement | null>(null)
const containerRef = ref<HTMLElement | null>(null)

// 曲线缓存：key = attackerId，value = 缓存的路径数据和尺寸
interface CachedPath {
  path2d: Path2D
  dataLength: number
  chartWidth: number
  chartHeight: number
}
const pathCache = new Map<string, CachedPath>()
let lastRenderWidth = 0
let lastRenderHeight = 0

// 面板高度
const panelHeight = ref(400)
const isResizing = ref(false)
let resizeStartY = 0
let resizeStartHeight = 0
const MIN_PANEL_HEIGHT = 200
const MAX_PANEL_HEIGHT = 800

// 图表标题
const chartTitle = computed(() => {
  if (appStore.isShowingHistory) {
    return '历史记录全程DPS趋势图'
  }
  return '全程DPS趋势图'
})

/**
 * 计算 DPS 数据（纯函数，从 chartData 数组计算出 DPS 序列）
 */
function computeDpsData(data: any[]): any[] {
  if (!data || data.length === 0) return []
  
  const attackerRecordsMap = new Map<string, { 
    name: string
    attackerId: string
    records: Array<{ time: number; damage: number }>
  }>()
  
  data.forEach(series => {
    const attackerId = (series as any).attackerId
    const attackerName = (series as any).attackerName
    
    if (!attackerId || !attackerName) return
    
    if (!attackerRecordsMap.has(attackerId)) {
      attackerRecordsMap.set(attackerId, {
        name: attackerName,
        attackerId,
        records: []
      })
    }
    
    const attackerData = attackerRecordsMap.get(attackerId)!
    
    series.data.forEach((point: any) => {
      attackerData.records.push({
        time: point.time,
        damage: point.singleDamage || 0
      })
    })
  })
  
  return Array.from(attackerRecordsMap.values()).map(attacker => {
    if (attacker.records.length === 0) {
      return {
        name: attacker.name,
        attackerId: attacker.attackerId,
        data: [],
        totalDamage: 0,
        avgDps: 0,
        maxDps: 0
      }
    }
    
    attacker.records.sort((a, b) => a.time - b.time)
    
    const startTime = attacker.records[0].time
    const endTime = attacker.records[attacker.records.length - 1].time
    const totalDamage = attacker.records.reduce((sum, r) => sum + r.damage, 0)
    const duration = (endTime - startTime) / 1000 || 1
    const avgDps = totalDamage / duration
    
    const dpsPoints: Array<{ time: number; dps: number }> = []
    let maxDps = 0
    
    const prefixSum: number[] = [0]
    attacker.records.forEach(r => {
      prefixSum.push(prefixSum[prefixSum.length - 1] + r.damage)
    })
    
    let recordPtr = 0
    
    for (let currentTime = startTime; currentTime <= endTime; currentTime += 1000) {
      while (recordPtr < attacker.records.length && attacker.records[recordPtr].time <= currentTime) {
        recordPtr++
      }
      
      const cumulativeDamage = prefixSum[recordPtr]
      const elapsedSeconds = (currentTime - startTime) / 1000
      const dps = elapsedSeconds > 0 ? cumulativeDamage / elapsedSeconds : 0
      
      if (dps > maxDps) maxDps = dps
      
      dpsPoints.push({
        time: currentTime,
        dps: dps
      })
    }
    
    return {
      name: attacker.name,
      attackerId: attacker.attackerId,
      data: dpsPoints,
      totalDamage,
      avgDps,
      maxDps
    }
  })
}

/** 计算数据指纹：轻量级哈希用于判断数据是否真的变化 */
function computeDpsFingerprint(data: any[]): string {
  if (!data || data.length === 0) return '0'
  let totalPoints = 0
  for (let i = 0; i < data.length; i++) {
    totalPoints += ((data[i] as any)?.data?.length ?? 0)
  }
  const firstLen = (data[0] as any)?.data?.length ?? 0
  const lastLen = (data[data.length - 1] as any)?.data?.length ?? 0
  return `${data.length}|${firstLen}|${lastLen}|${totalPoints}`
}

/** 原始DPS数据（带指纹缓存，避免数据相同时重复计算） */
const rawDpsData = shallowRef<any[]>([])
let lastDpsFingerprint = ''

function updateDpsData() {
  const data = appStore.isShowingHistory && appStore.historyChartData.length > 0 
    ? appStore.historyChartData 
    : appStore.chartData
  
  const fp = computeDpsFingerprint(data)
  if (fp !== lastDpsFingerprint) {
    lastDpsFingerprint = fp
    rawDpsData.value = computeDpsData(data)
  }
}

/**
 * 处理后的DPS数据（支持百分比模式）
 */
const dpsData = computed(() => {
  const rawData = rawDpsData.value
  if (rawData.length === 0) return []
  
  return rawData.map(series => ({
    ...series,
    displayData: series.data.map(p => ({ ...p, displayValue: p.dps }))
  }))
})

/**
 * 当前排名数据
 */
const currentRankings = computed(() => {
  const data = rawDpsData.value
  if (data.length === 0) return []
  
  // 获取每个玩家的最后一个DPS值
  return data
    .map(series => {
      if (series.data.length === 0) {
        return { ...series, currentDps: 0 }
      }
      const lastPoint = series.data[series.data.length - 1]
      return { ...series, currentDps: lastPoint.dps }
    })
    .sort((a, b) => b.currentDps - a.currentDps)
    .slice(0, 10)
})

/**
 * 绘制DPS图表
 */
function drawChart() {
  const canvas = canvasRef.value
  if (!canvas) return
  
  const ctx = canvas.getContext('2d')
  if (!ctx) return
  
  const container = containerRef.value
  if (!container) return
  
  const rect = container.getBoundingClientRect()
  
  // 设置 canvas 尺寸
  const dpr = window.devicePixelRatio || 1
  canvas.width = rect.width * dpr
  canvas.height = rect.height * dpr
  canvas.style.width = `${rect.width}px`
  canvas.style.height = `${rect.height}px`
  ctx.scale(dpr, dpr)
  
  ctx.clearRect(0, 0, rect.width, rect.height)
  
  const data = dpsData.value
  
  if (!data || data.length === 0) {
    ctx.fillStyle = '#666'
    ctx.font = '12px Microsoft YaHei'
    ctx.textAlign = 'center'
    ctx.fillText('等待数据...', rect.width / 2, rect.height / 2)
    return
  }
  
  const padding = { top: 20, right: 20, bottom: 30, left: 50 }
  const chartWidth = rect.width - padding.left - padding.right
  const chartHeight = rect.height - padding.top - padding.bottom
  
  // 获取显示最大值
  let maxDisplayValue = 0
  data.forEach(series => {
    series.displayData.forEach(point => {
      if (point.displayValue > maxDisplayValue) maxDisplayValue = point.displayValue
    })
  })
  
  if (maxDisplayValue === 0) {
    ctx.fillStyle = '#666'
    ctx.font = '12px Microsoft YaHei'
    ctx.textAlign = 'center'
    ctx.fillText('等待数据...', rect.width / 2, rect.height / 2)
    return
  }
  
  // 向上取整
  let step: number
  if (maxDisplayValue >= 100000) {
    step = 50000
  } else if (maxDisplayValue >= 10000) {
    step = 5000
  } else if (maxDisplayValue >= 1000) {
    step = 500
  } else {
    step = 100
  }
  maxDisplayValue = Math.ceil(maxDisplayValue / step) * step
  
  // 确定时间范围
  let minTime = Infinity
  let maxTime = -Infinity
  data.forEach(series => {
    series.data.forEach(point => {
      if (point.time < minTime) minTime = point.time
      if (point.time > maxTime) maxTime = point.time
    })
  })
  
  // 如果是历史记录模式，使用全局时间范围
  if (appStore.isShowingHistory) {
    const globalTimeRange = appStore.chartTimeRange
    if (globalTimeRange.minTime !== 0 && globalTimeRange.maxTime !== 0) {
      minTime = globalTimeRange.minTime
      maxTime = globalTimeRange.maxTime
    }
  }
  
  const timeRange = maxTime - minTime
  if (timeRange === 0) return
  
  // 绘制坐标轴
  ctx.strokeStyle = 'rgba(255, 255, 255, 0.2)'
  ctx.lineWidth = 1
  ctx.beginPath()
  ctx.moveTo(padding.left, padding.top)
  ctx.lineTo(padding.left, rect.height - padding.bottom)
  ctx.lineTo(rect.width - padding.right, rect.height - padding.bottom)
  ctx.stroke()
  
  // 绘制 Y 轴刻度
  ctx.fillStyle = '#888'
  ctx.font = '11px Microsoft YaHei'
  ctx.textAlign = 'right'
  
  const tickCount = 5
  for (let i = 0; i <= tickCount; i++) {
    const y = padding.top + (chartHeight * i / tickCount)
    const displayValue = maxDisplayValue * (1 - i / tickCount)
    
    let label: string
    if (displayValue >= 1000000) {
      label = `${(displayValue / 1000000).toFixed(1)}M/s`
    } else if (displayValue >= 1000) {
      label = `${(displayValue / 1000).toFixed(0)}k/s`
    } else {
      label = `${displayValue.toFixed(0)}/s`
    }
    
    ctx.fillText(label, padding.left - 5, y + 3)
    
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
  
  const timeTickCount = Math.max(5, Math.floor(chartWidth / 80))
  const timeTickInterval = timeRange / timeTickCount
  
  for (let i = 0; i <= timeTickCount; i++) {
    const time = minTime + (timeTickInterval * i)
    const x = padding.left + ((time - minTime) / timeRange) * chartWidth
    
    let seconds: number
    if (appStore.isShowingHistory) {
      seconds = Math.floor((time - minTime) / 1000)
    } else {
      seconds = Math.floor(time / 1000)
    }
    
    let timeStr: string
    if (seconds >= 60) {
      const mins = Math.floor(seconds / 60)
      const secs = seconds % 60
      timeStr = `${mins}:${secs.toString().padStart(2, '0')}`
    } else {
      timeStr = `${seconds}s`
    }
    
    ctx.fillText(timeStr, x, rect.height - padding.bottom + 15)
    
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.1)'
    ctx.beginPath()
    ctx.moveTo(x, padding.top)
    ctx.lineTo(x, rect.height - padding.bottom)
    ctx.stroke()
  }
  
  // 检测尺寸变化，清除缓存
  if (Math.abs(rect.width - lastRenderWidth) > 2 || Math.abs(rect.height - lastRenderHeight) > 2) {
    pathCache.clear()
    lastRenderWidth = rect.width
    lastRenderHeight = rect.height
  }
  
  // 绘制DPS曲线（使用改进的Catmull-Rom样条平滑曲线）
  data.slice(0, 20).forEach((series, index) => {
    if (series.displayData.length < 2) return
    
    const color = COLORS[index % COLORS.length]
    const attackerId = series.attackerId || `series-${index}`
    
    ctx.imageSmoothingEnabled = true
    ctx.strokeStyle = color
    ctx.lineWidth = 2.5
    ctx.lineCap = 'round'
    ctx.lineJoin = 'round'
    
    // 检查缓存有效性
    const cached = pathCache.get(attackerId)
    const needsRebuild = !cached || 
      cached.dataLength !== series.displayData.length ||
      cached.chartWidth !== chartWidth ||
      cached.chartHeight !== chartHeight
    
    if (needsRebuild) {
      // 将数据点转换为屏幕坐标（使用displayValue）
      const points = series.displayData.map(point => ({
        x: padding.left + ((point.time - minTime) / timeRange) * chartWidth,
        y: padding.top + (1 - point.displayValue / maxDisplayValue) * chartHeight
      }))
      
      // 构建 Path2D
      const path2d = new Path2D()
      
      if (points.length === 2) {
        path2d.moveTo(points[0].x, points[0].y)
        path2d.lineTo(points[1].x, points[1].y)
      } else {
        path2d.moveTo(points[0].x, points[0].y)
        
        for (let i = 0; i < points.length - 1; i++) {
          const p0 = i === 0 ? points[0] : points[i - 1]
          const p1 = points[i]
          const p2 = points[i + 1]
          const p3 = i === points.length - 2 ? p2 : points[i + 2]
          
          const tension = 0.5
          const cp1x = p1.x + (p2.x - p0.x) * tension / 2
          const cp2x = p2.x - (p3.x - p1.x) * tension / 2
          
          const minY = padding.top
          const maxY = padding.top + chartHeight
          const cp1y = Math.max(minY + 5, Math.min(maxY - 5, p1.y + (p2.y - p0.y) * tension / 2))
          const cp2y = Math.max(minY + 5, Math.min(maxY - 5, p2.y - (p3.y - p1.y) * tension / 2))
          
          path2d.bezierCurveTo(cp1x, cp1y, cp2x, cp2y, p2.x, p2.y)
        }
      }
      
      // 缓存路径
      pathCache.set(attackerId, {
        path2d,
        dataLength: series.displayData.length,
        chartWidth,
        chartHeight
      })
      
      ctx.stroke(path2d)
    } else {
      // 使用缓存路径
      ctx.stroke(cached.path2d)
    }
  })

  if (appStore.isShowingHistory) {
    drawBossHPOverlay(ctx, padding.left, rect.width - padding.right, padding.top, rect.height - padding.bottom, minTime, maxTime, appStore.selectedTarget?.name, !!appStore.selectedTarget?.deathTime)
  }
}

/**
 * 开始调整大小
 */
function startResize(event: MouseEvent) {
  isResizing.value = true
  resizeStartY = event.clientY
  resizeStartHeight = panelHeight.value
  document.body.style.cursor = 'ns-resize'
  document.body.style.userSelect = 'none'
  event.preventDefault()
  event.stopPropagation()
}

let resizeRafId: number | null = null
let lastResizeTime = 0
const RESIZE_THROTTLE = 16

function handleMouseMove(event: MouseEvent) {
  if (isResizing.value) {
    const now = Date.now()
    if (now - lastResizeTime < RESIZE_THROTTLE) return
    
    lastResizeTime = now
    
    if (resizeRafId) cancelAnimationFrame(resizeRafId)
    
    resizeRafId = requestAnimationFrame(() => {
      const deltaY = resizeStartY - event.clientY
      const newHeight = Math.max(MIN_PANEL_HEIGHT, Math.min(MAX_PANEL_HEIGHT, resizeStartHeight + deltaY))
      panelHeight.value = newHeight
      nextTick(() => drawChart())
      resizeRafId = null
    })
  }
}

function stopResize() {
  if (isResizing.value) {
    isResizing.value = false
    if (resizeRafId) {
      cancelAnimationFrame(resizeRafId)
      resizeRafId = null
    }
    document.body.style.cursor = ''
    document.body.style.userSelect = ''
  }
}

let resizeTimer: number | null = null

function handleResize() {
  if (resizeTimer) clearTimeout(resizeTimer)
  resizeTimer = window.setTimeout(() => {
    drawChart()
  }, 100)
}

// 监听数据变化（引用变化时始终重绘，因为数据内容可能已更新）
watch(() => appStore.chartData, () => {
  updateDpsData()
  nextTick(() => drawChart())
})

watch(() => appStore.historyChartData, () => {
  updateDpsData()
  nextTick(() => drawChart())
})

watch(() => appStore.isShowingHistory, () => {
  updateDpsData()
  nextTick(() => drawChart())
})

watch(() => appStore.selectedSkillFilters, () => {
  updateDpsData()
  nextTick(() => drawChart())
})

onMounted(() => {
  updateDpsData()
  nextTick(() => drawChart())
  
  document.addEventListener('mousemove', handleMouseMove)
  document.addEventListener('mouseup', stopResize)
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  if (resizeTimer) clearTimeout(resizeTimer)
  if (resizeRafId) cancelAnimationFrame(resizeRafId)
  document.removeEventListener('mousemove', handleMouseMove)
  document.removeEventListener('mouseup', stopResize)
  window.removeEventListener('resize', handleResize)
})
</script>

<template>
  <div class="dps-chart-panel" :style="{ height: panelHeight + 'px' }">
    <div class="chart-header">
      <span class="chart-title-text">{{ chartTitle }}</span>
      
    </div>
    
    <!-- 当前排名列表 -->
    <div v-if="currentRankings.length > 0" class="rankings-bar">
      <div 
        v-for="(item, index) in currentRankings.slice(0, 5)" 
        :key="item.attackerId"
        class="ranking-item"
        :style="{ borderLeftColor: COLORS[index % COLORS.length] }"
      >
        <span class="rank-number">{{ index + 1 }}</span>
        <span class="rank-name">{{ item.name }}</span>
        <span class="rank-dps">{{ formatDps(item.currentDps) }}</span>
      </div>
    </div>
    
    <div ref="containerRef" class="dps-chart-container">
      <canvas ref="canvasRef"></canvas>
    </div>
    
    <!-- 调整大小手柄 -->
    <div class="resize-handle" @mousedown="startResize"></div>
  </div>
</template>

<style lang="scss" scoped>
.dps-chart-panel {
  width: 100%;
  height: 400px;
  min-height: 200px;
  max-height: 800px;
  border-top: 1px solid rgba(255, 255, 255, 0.1);
  display: flex;
  flex-direction: column;
  position: relative;
  flex-shrink: 0;
}

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

.rankings-bar {
  display: flex;
  gap: 8px;
  padding: 4px 8px;
  background: rgba(20, 20, 20, 0.9);
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  overflow-x: auto;
  flex-shrink: 0;
}

.ranking-item {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 2px 6px;
  background: rgba(255, 255, 255, 0.05);
  border-left: 3px solid;
  border-radius: 0 3px 3px 0;
  white-space: nowrap;
}

.rank-number {
  font-size: 10px;
  font-weight: bold;
  color: #888;
  min-width: 14px;
}

.rank-name {
  font-size: 10px;
  color: #ccc;
  max-width: 60px;
  overflow: hidden;
  text-overflow: ellipsis;
}

.rank-dps {
  font-size: 10px;
  font-weight: bold;
  color: #64b5f6;
}

.dps-chart-container {
  flex: 1;
  padding: 4px;
  position: relative;
  min-height: 80px;
}

canvas {
  width: 100%;
  height: 100%;
}

.resize-handle {
  position: absolute;
  width: 100%;
  height: 6px;
  top: -3px;
  left: 0;
  cursor: ns-resize;
  background: transparent;
  z-index: 10;

  &:hover {
    background: rgba(66, 165, 245, 0.3);
  }
}
</style>
