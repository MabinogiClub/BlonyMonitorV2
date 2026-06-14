<script setup lang="ts">
/**
 * 图表面板组件
 * 显示DPS趋势图表
 * - 使用 ECharts 绘制折线图
 * - 每30秒聚合一个数据点
 * - 最多显示1小时（120个数据点）
 * - 数据按角色名称聚合（同一玩家不同实体ID合并）
 * - 支持 dataZoom 缩放浏览历史数据
 * - 默认只展示最后10个数据点
 */

import { ref, computed, watch, onMounted, onUnmounted, nextTick, shallowRef } from 'vue'
import { useAppStore } from '../stores/app'
import * as echarts from 'echarts'

// 获取应用状态
const appStore = useAppStore()

// 图表容器引用
const chartContainerRef = ref<HTMLElement | null>(null)

// ECharts 实例（使用 shallowRef 避免深度响应式）
const chartInstance = shallowRef<echarts.ECharts | null>(null)

// 面板高度（默认240）
const panelHeight = ref(240)

// 是否正在调整大小
const isResizing = ref(false)

// 面板引用
const panelRef = ref<HTMLElement | null>(null)

// 调整大小时的起始位置
const resizeStartY = ref(0)
const resizeStartHeight = ref(0)

// 用户是否手动设置了 dataZoom（如果手动设置，就不再自动调整）
const userZoomSet = ref(false)

/**
 * 图表数据
 */
const chartData = computed(() => appStore.chartData)

/**
 * 图表显示颜色（6个不同颜色，对应最多6个角色）
 */
const CHART_COLORS = [
  '#ffc107', // 金色
  '#42a5f5', // 蓝色
  '#e91e63', // 粉色
  '#4caf50', // 绿色
  '#9c27b0', // 紫色
  '#ff9800', // 橙色
]

/**
 * 格式化数字（简化显示）
 */
function formatNumber(value: number): string {
  if (value >= 1000000000) {
    return (value / 1000000000).toFixed(1) + 'B'
  }
  if (value >= 1000000) {
    return (value / 1000000).toFixed(1) + 'M'
  }
  if (value >= 1000) {
    return (value / 1000).toFixed(1) + 'K'
  }
  return value.toFixed(0)
}

/**
 * 格式化时间为 HH:mm:ss
 */
function formatTime(timestamp: number): string {
  const date = new Date(timestamp)
  return date.toLocaleTimeString('zh-CN', { 
    hour: '2-digit', 
    minute: '2-digit', 
    second: '2-digit',
    hour12: false 
  })
}

/**
 * 初始化图表
 */
function initChart() {
  if (!chartContainerRef.value) return
  
  // 如果已有实例，先销毁
  if (chartInstance.value) {
    chartInstance.value.dispose()
  }
  
  // 创建 ECharts 实例（不使用内置主题，避免自带背景色）
  chartInstance.value = echarts.init(chartContainerRef.value, undefined, {
    renderer: 'canvas'
  })
  
  // 监听 dataZoom 事件，记录用户是否手动设置
  chartInstance.value.on('datazoom', (params: any) => {
    // 只有通过用户交互触发的才标记
    if (params.batch || params.dataZoomId !== undefined) {
      userZoomSet.value = true
    }
  })
  
  // 初始更新
  updateChart()
}

/**
 * 更新图表数据和配置
 */
function updateChart() {
  if (!chartInstance.value) return
  
  const data = chartData.value
  
  // 无数据时显示提示
  if (!data || data.length === 0) {
    chartInstance.value.setOption({
      title: {
        text: '等待数据...',
        left: 'center',
        top: 'center',
        textStyle: {
          color: '#666',
          fontSize: 12,
          fontFamily: 'Microsoft YaHei'
        }
      },
      xAxis: { show: false },
      yAxis: { show: false },
      series: []
    }, true)
    return
  }
  
  // 收集所有时间点并排序
  const allTimes = new Set<number>()
  data.forEach(series => {
    series.data.forEach(point => {
      allTimes.add(point.time)
    })
  })
  const sortedTimes = Array.from(allTimes).sort((a, b) => a - b)
  
  if (sortedTimes.length === 0) {
    chartInstance.value.setOption({
      title: {
        text: '等待更多数据...',
        left: 'center',
        top: 'center',
        textStyle: {
          color: '#666',
          fontSize: 12,
          fontFamily: 'Microsoft YaHei'
        }
      },
      xAxis: { show: false },
      yAxis: { show: false },
      series: []
    }, true)
    return
  }
  
  // 为每个系列创建数据映射（时间 -> 伤害）
  const seriesData = data.slice(0, 6).map((series, index) => {
    const timeMap = new Map<number, number>()
    series.data.forEach(point => {
      timeMap.set(point.time, point.damage)
    })
    
    // 转换为折线图数据
    const lineData = sortedTimes.map(time => {
      const damage = timeMap.get(time)
      return damage !== undefined ? damage : null
    })
    
    return {
      name: series.name,
      type: 'line' as const,
      smooth: true,
      symbol: 'circle',
      symbolSize: 4,
      showSymbol: sortedTimes.length <= 20, // 数据点多时隐藏标记
      connectNulls: true, // 连接空值，保持曲线连续
      lineStyle: {
        width: 2,
        color: CHART_COLORS[index]
      },
      itemStyle: {
        color: CHART_COLORS[index]
      },
      emphasis: {
        focus: 'series' as const,
        itemStyle: {
          borderWidth: 2
        }
      },
      data: lineData
    }
  })
  
  // 计算 dataZoom 的起始位置（默认只显示最后 10 个点）
  let dataZoomStart = 0
  if (!userZoomSet.value && sortedTimes.length > 10) {
    // 计算百分比：显示最后 10 个点
    dataZoomStart = ((sortedTimes.length - 10) / sortedTimes.length) * 100
  }
  
  // 获取当前的 dataZoom 状态（如果用户已设置）
  let currentDataZoom: { start?: number; end?: number } | null = null
  if (userZoomSet.value) {
    const option = chartInstance.value.getOption() as any
    if (option && option.dataZoom && option.dataZoom[0]) {
      currentDataZoom = {
        start: option.dataZoom[0].start,
        end: option.dataZoom[0].end
      }
    }
  }
  
  // 图表配置
  const option: echarts.EChartsOption = {
    backgroundColor: 'transparent',
    title: undefined,
    tooltip: {
      trigger: 'axis',
      backgroundColor: 'rgba(30, 30, 30, 0.95)',
      borderColor: 'rgba(255, 255, 255, 0.1)',
      borderWidth: 1,
      textStyle: {
        color: '#ddd',
        fontSize: 11
      },
      axisPointer: {
        type: 'cross',
        lineStyle: {
          color: 'rgba(255, 255, 255, 0.3)'
        },
        crossStyle: {
          color: 'rgba(255, 255, 255, 0.3)'
        }
      },
      formatter: (params: any) => {
        if (!Array.isArray(params) || params.length === 0) return ''
        const time = formatTime(sortedTimes[params[0].dataIndex])
        let result = `<div style="font-weight:bold;margin-bottom:4px;">${time}</div>`
        params.forEach((item: any) => {
          if (item.value !== null && item.value !== undefined) {
            result += `<div style="display:flex;align-items:center;gap:4px;margin:2px 0;">
              <span style="display:inline-block;width:8px;height:8px;border-radius:50%;background:${item.color}"></span>
              <span>${item.seriesName}:</span>
              <span style="font-weight:bold;">${formatNumber(item.value)}/s</span>
            </div>`
          }
        })
        return result
      }
    },
    legend: {
      show: true,
      type: 'scroll',
      bottom: 30,
      left: 'center',
      itemWidth: 12,
      itemHeight: 8,
      textStyle: {
        color: '#aaa',
        fontSize: 10
      },
      pageTextStyle: {
        color: '#888'
      },
      data: data.slice(0, 6).map((series, index) => ({
        name: series.name,
        itemStyle: {
          color: CHART_COLORS[index]
        }
      }))
    },
    grid: {
      left: 50,
      right: 15,
      top: 15,
      bottom: 75
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: sortedTimes.map(t => formatTime(t)),
      axisLine: {
        lineStyle: {
          color: 'rgba(255, 255, 255, 0.2)'
        }
      },
      axisLabel: {
        color: '#888',
        fontSize: 9,
        rotate: 0,
        interval: 'auto'
      },
      axisTick: {
        lineStyle: {
          color: 'rgba(255, 255, 255, 0.1)'
        }
      },
      splitLine: {
        show: false
      }
    },
    yAxis: {
      type: 'value',
      axisLine: {
        show: true,
        lineStyle: {
          color: 'rgba(255, 255, 255, 0.2)'
        }
      },
      axisLabel: {
        color: '#888',
        fontSize: 9,
        formatter: (value: number) => formatNumber(value) + '/s'
      },
      splitLine: {
        lineStyle: {
          color: 'rgba(255, 255, 255, 0.1)'
        }
      }
    },
    dataZoom: [
      {
        type: 'slider',
        show: true,
        xAxisIndex: [0],
        start: currentDataZoom?.start ?? dataZoomStart,
        end: currentDataZoom?.end ?? 100,
        bottom: 5,
        height: 18,
        borderColor: 'rgba(255, 255, 255, 0.1)',
        backgroundColor: 'rgba(30, 30, 30, 0.6)',
        fillerColor: 'rgba(66, 165, 245, 0.2)',
        handleStyle: {
          color: '#42a5f5',
          borderColor: '#42a5f5'
        },
        textStyle: {
          color: '#888',
          fontSize: 9
        },
        dataBackground: {
          lineStyle: {
            color: 'rgba(255, 255, 255, 0.2)'
          },
          areaStyle: {
            color: 'rgba(255, 255, 255, 0.05)'
          }
        },
        selectedDataBackground: {
          lineStyle: {
            color: 'rgba(66, 165, 245, 0.5)'
          },
          areaStyle: {
            color: 'rgba(66, 165, 245, 0.1)'
          }
        }
      },
      {
        type: 'inside',
        xAxisIndex: [0],
        start: currentDataZoom?.start ?? dataZoomStart,
        end: currentDataZoom?.end ?? 100,
        zoomOnMouseWheel: true,
        moveOnMouseMove: true
      }
    ],
    series: seriesData
  }
  
  chartInstance.value.setOption(option, { notMerge: false })
}

/**
 * 调整图表大小
 */
function resizeChart() {
  if (chartInstance.value) {
    chartInstance.value.resize()
  }
}

/**
 * 开始调整大小
 */
function startResize(event: MouseEvent) {
  isResizing.value = true
  resizeStartY.value = event.clientY
  resizeStartHeight.value = panelHeight.value
  document.body.style.cursor = 'ns-resize'
  event.preventDefault()
}

/**
 * 处理鼠标移动
 */
function handleMouseMove(event: MouseEvent) {
  if (!isResizing.value) return
  
  // 鼠标向下拖动时增加高度，向上拖动时减小高度
  const deltaY = event.clientY - resizeStartY.value
  const newHeight = resizeStartHeight.value + deltaY
  const minHeight = 150
  const maxHeight = 500
  if (newHeight >= minHeight && newHeight <= maxHeight) {
    panelHeight.value = newHeight
    nextTick(() => resizeChart())
  }
}

/**
 * 结束调整大小
 */
function stopResize() {
  if (isResizing.value) {
    isResizing.value = false
    document.body.style.cursor = ''
  }
}

/**
 * 处理窗口大小变化
 */
function handleResize() {
  resizeChart()
}

// 监听图表数据变化
watch(chartData, () => {
  nextTick(() => updateChart())
}, { deep: true })

onMounted(() => {
  document.addEventListener('mousemove', handleMouseMove)
  document.addEventListener('mouseup', stopResize)
  window.addEventListener('resize', handleResize)
  
  // 初始化图表
  nextTick(() => initChart())
})

onUnmounted(() => {
  document.removeEventListener('mousemove', handleMouseMove)
  document.removeEventListener('mouseup', stopResize)
  window.removeEventListener('resize', handleResize)
  
  // 销毁图表实例
  if (chartInstance.value) {
    chartInstance.value.dispose()
    chartInstance.value = null
  }
})
</script>

<template>
  <div ref="panelRef" class="chart-panel" :style="{ height: panelHeight + 'px' }">
    <!-- 图表容器 -->
    <div ref="chartContainerRef" class="chart-container"></div>
    
    <!-- 调整大小手柄 -->
     <!-- TODO: 这个调整大小有问题，先不允许调整大小 -->
    <!-- <div class="resize-handle" @mousedown="startResize"></div> -->
  </div>
</template>

<style lang="scss" scoped>
/**
 * 图表面板样式
 */

// 图表面板
.chart-panel {
  height: 240px;
  min-height: 150px;
  max-height: 500px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  display: flex;
  flex-direction: column;
  position: relative;
  background: rgba(20,20,20,.8);
}

// 图表容器
.chart-container {
  flex: 1;
  width: 100%;
  min-height: 0;
}

// 调整大小手柄（底部）
.resize-handle {
  position: absolute;
  width: 100%;
  height: 4px;
  bottom: 0;
  left: 0;
  cursor: ns-resize;
  background: transparent;

  &:hover {
    background: rgba(66, 165, 245, 0.3);
  }
}
</style>
