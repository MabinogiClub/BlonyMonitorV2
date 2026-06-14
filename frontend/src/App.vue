<script setup lang="ts">
/**
 * 主应用组件
 */

import { onMounted, onUnmounted } from 'vue'
import { useAppStore } from './stores/app'
import TitleBar from './components/TitleBar.vue'
import StatusBar from './components/StatusBar.vue'
import DebugPanel from './components/DebugPanel.vue'
import ChartPanel from './components/ChartPanel.vue'
import TabsPanel from './components/TabsPanel.vue'

const appStore = useAppStore()

let updateInterval: number | null = null

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
  <div class="app-container">
    <TitleBar />
    <StatusBar />

    <div class="chart-drawer" :class="{ expanded: appStore.chartVisible }">
      <ChartPanel />
    </div>

    <TabsPanel />
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

.app-container > :nth-child(4) {
  flex: 1;
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
</style>
