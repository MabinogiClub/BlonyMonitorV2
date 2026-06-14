<script setup lang="ts">
/**
 * 调试面板组件
 * 显示应用的调试信息
 * 使用 Vant Popup 组件
 */

import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useAppStore } from '../stores/app'
import * as api from '../composables/useApi'

// 获取应用状态
const appStore = useAppStore()

// 调试信息
const debugInfo = ref<DebugInfo | null>(null)

// 定时器 ID
let updateInterval: number | null = null

/**
 * 是否显示面板（双向绑定）
 */
const showPopup = computed({
  get: () => appStore.debugVisible,
  set: (val) => {
    if (!val) appStore.toggleDebug()
  }
})

/**
 * 连接状态文本
 */
const connectionStatus = computed(() => {
  return debugInfo.value?.connected ? '已连接' : '未连接'
})

/**
 * 前端映射信息
 */
const frontendMappingInfo = computed(() => {
  const skillCount = Object.keys(appStore.skillNameMap).length
  const condCount = Object.keys(appStore.conditionNameMap).length
  return `${skillCount} 技能, ${condCount} 状态`
})

/**
 * 示例技能
 */
const sampleSkills = computed(() => {
  return debugInfo.value?.sampleSkills?.join(', ') || '无'
})

/**
 * 加载错误
 */
const loadError = computed(() => {
  return debugInfo.value?.loadError || '无'
})

/**
 * 更新调试信息
 */
async function updateDebugInfo() {
  if (!appStore.debugVisible) return
  
  try {
    debugInfo.value = await api.getDebugInfo()
  } catch (e) {
    appStore.lastError = String(e)
  }
}

// 监听显示状态，显示时立即更新数据
watch(() => appStore.debugVisible, (visible) => {
  if (visible) {
    updateDebugInfo()
  }
})

onMounted(() => {
  // 定期更新调试信息
  updateInterval = window.setInterval(updateDebugInfo, 1000)
})

onUnmounted(() => {
  if (updateInterval) {
    clearInterval(updateInterval)
  }
})
</script>

<template>
  <!-- 使用 Vant Popup 组件 -->
  <van-popup
    v-model:show="showPopup"
    position="top"
    :overlay="false"
    :style="{ 
      top: '28px',
      background: 'rgba(0, 0, 0, 0.95)',
      borderBottom: '1px solid rgba(255, 152, 0, 0.5)'
    }"
    class="debug-popup"
  >
    <div class="debug-panel">
      <div class="debug-item">
        <span class="debug-label">状态:</span>
        <span class="debug-value" :class="{ 'debug-error': !debugInfo?.connected }">
          {{ connectionStatus }}
        </span>
      </div>
      <div class="debug-item">
        <span class="debug-label">技能数量:</span>
        <span class="debug-value">{{ debugInfo?.skillCount ?? 0 }}</span>
      </div>
      <div class="debug-item">
        <span class="debug-label">解析技能:</span>
        <span class="debug-value">{{ debugInfo?.parsedSkills ?? 0 }}</span>
      </div>
      <div class="debug-item">
        <span class="debug-label">解析字符串:</span>
        <span class="debug-value">{{ debugInfo?.parsedStrings ?? 0 }}</span>
      </div>
      <div class="debug-item">
        <span class="debug-label">种族数量:</span>
        <span class="debug-value">{{ debugInfo?.raceCount ?? 0 }}</span>
      </div>
      <div class="debug-item">
        <span class="debug-label">状态数量:</span>
        <span class="debug-value">{{ debugInfo?.conditionCount ?? 0 }}</span>
      </div>
      <div class="debug-item">
        <span class="debug-label">实体数量:</span>
        <span class="debug-value">{{ debugInfo?.entityCount ?? 0 }}</span>
      </div>
      <div class="debug-item">
        <span class="debug-label">伤害记录:</span>
        <span class="debug-value">{{ debugInfo?.damageCount ?? 0 }}</span>
      </div>
      <div class="debug-item">
        <span class="debug-label">图表数据:</span>
        <span class="debug-value">{{ debugInfo?.chartDataLen ?? 0 }}</span>
      </div>
      <div class="debug-item">
        <span class="debug-label">区域:</span>
        <span class="debug-value">{{ debugInfo?.region ?? '-' }}</span>
      </div>
      <div class="debug-item">
        <span class="debug-label">资源URL:</span>
        <span class="debug-value">{{ debugInfo?.resourceURL ?? '-' }}</span>
      </div>
      <div class="debug-item">
        <span class="debug-label">示例技能:</span>
        <span class="debug-value">{{ sampleSkills }}</span>
      </div>
      <div class="debug-item">
        <span class="debug-label">前端映射:</span>
        <span class="debug-value">{{ frontendMappingInfo }}</span>
      </div>
      <div class="debug-item">
        <span class="debug-label">加载错误:</span>
        <span :class="loadError !== '无' ? 'debug-error' : 'debug-value'">{{ loadError }}</span>
      </div>
      <div class="debug-item">
        <span class="debug-label">JS错误:</span>
        <span :class="appStore.lastError !== '无' ? 'debug-error' : 'debug-value'">
          {{ appStore.lastError }}
        </span>
      </div>
    </div>
  </van-popup>
</template>

<style lang="scss" scoped>
/**
 * 调试面板样式
 */

// Popup 样式覆盖 - 自适应高度，不出现滚动条
:deep(.debug-popup) {
  max-height: none !important;
  height: auto !important;
  overflow: visible !important;
  
  .van-popup__content {
    max-height: none !important;
    height: auto !important;
    overflow: visible !important;
  }
}

.debug-panel {
  padding: 8px;
  font-size: 10px;
  color: #ff9800;
  overflow: visible;
  width: 100%;
}

.debug-item {
  margin: 2px 0;
  white-space: nowrap;
}

.debug-label {
  color: #aaa;
  margin-right: 8px;
}

.debug-value {
  color: #4caf50;
}

.debug-error {
  color: #f44336;
}
</style>
