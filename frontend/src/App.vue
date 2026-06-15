<script setup lang="ts">
/**
 * 主应用组件
 */

import { onMounted, onUnmounted, watch } from 'vue'
import { useAppStore } from './stores/app'
import * as api from './composables/useApi'
import TitleBar from './components/TitleBar.vue'
import StatusBar from './components/StatusBar.vue'
import DebugPanel from './components/DebugPanel.vue'
import ChartPanel from './components/ChartPanel.vue'
import DpsChartPanel from './components/DpsChartPanel.vue'
import HistoryChartPanel from './components/HistoryChartPanel.vue'
import TabsPanel from './components/TabsPanel.vue'

const appStore = useAppStore()

let updateInterval: number | null = null
let isHistoryMode = false

const HISTORY_WIDTH_EXTRA = 420
const HISTORY_HEIGHT_EXTRA = 200

async function applyHistoryWindowSize() {
  if (!isHistoryMode || appStore.activeTab !== 'history') return

  const base = appStore.historyWindowSize
  const targetWidth = base.width + HISTORY_WIDTH_EXTRA
  const targetHeight = base.height + HISTORY_HEIGHT_EXTRA
  const current = await api.getWindowSize()

  if (current.width >= targetWidth && current.height >= targetHeight) {
    return
  }

  const newWidth = Math.max(current.width, targetWidth)
  const newHeight = Math.max(current.height, targetHeight)

  await api.setWindowMinSize(newWidth, newHeight)
  await api.setWindowSize(newWidth, newHeight)
}

async function restoreNormalWindowSize() {
  const base = appStore.historyWindowSize
  await api.setWindowMinSize(440, 600)
  await api.setWindowSize(base.width, base.height)
}

watch(() => appStore.activeTab, async (newTab, oldTab) => {
  if (newTab === 'history') {
    if (!isHistoryMode) {
      const currentSize = await api.getWindowSize()
      appStore.historyWindowSize = { width: currentSize.width, height: currentSize.height }
      isHistoryMode = true
      await applyHistoryWindowSize()
    }
    return
  }

  if (oldTab === 'history' && isHistoryMode) {
    isHistoryMode = false
    await restoreNormalWindowSize()
  }
})

onMounted(async () => {
  await appStore.initialize()
  appStore.registerEvents()

  updateInterval = window.setInterval(() => {
    appStore.updateAllViews()
  }, 1000)
})

onUnmounted(() => {
  if (updateInterval) {
    clearInterval(updateInterval)
  }
})
</script>

<template>
  <div class="app-container" :class="{ 'history-mode': appStore.activeTab === 'history' }">
    <TitleBar />
    <StatusBar />

    <div class="chart-drawer" :class="{ expanded: appStore.chartVisible && appStore.activeTab !== 'history' }">
      <ChartPanel />
    </div>

    <div class="main-container">
      <TabsPanel />
      <DpsChartPanel
        v-if="appStore.activeTab === 'history' && appStore.isShowingHistory && appStore.selectedSkillFilters.length === 0"
      />
      <HistoryChartPanel
        v-if="appStore.activeTab === 'history' && appStore.isShowingHistory && appStore.selectedSkillFilters.length > 0"
      />
    </div>

    <DebugPanel />
  </div>
</template>

<style scoped>
.app-container {
  width: 100%;
  height: 100vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: rgba(20, 20, 20, 0.6);
}

.main-container {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}

.chart-drawer {
  max-height: 0;
  overflow: hidden;
  transition: max-height 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.chart-drawer.expanded {
  max-height: 240px;
}

.history-mode .main-container {
  min-height: 0;
}
</style>
