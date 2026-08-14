<script setup lang="ts">
/**
 * 主应用组件
 */

import { onMounted, onUnmounted, ref, watch } from 'vue'
import { useAppStore } from './stores/app'
import * as api from './composables/useApi'
import TitleBar from './components/TitleBar.vue'
import StatusBar from './components/StatusBar.vue'
import DebugPanel from './components/DebugPanel.vue'
import ChartPanel from './components/ChartPanel.vue'
import DpsChartPanel from './components/DpsChartPanel.vue'
import HistoryChartPanel from './components/HistoryChartPanel.vue'
import TabsPanel from './components/TabsPanel.vue'
import SidebarNav from './components/SidebarNav.vue'
import NpcapGuideDialog from './components/NpcapGuideDialog.vue'
import StartupNoticeDialog from './components/StartupNoticeDialog.vue'

const appStore = useAppStore()

let updateInterval: number | null = null
let isHistoryMode = false
const startupNoticeVisible = ref(false)
const startupNoticeKind = ref<'announcement' | 'ranking'>('announcement')
const startupAnnouncement = ref<ServerAnnouncement | null>(null)
const checkNpcapAfterSettings = ref(false)

const announcementAcknowledgedKey = 'blony-monitor:last-announcement-timestamp'
const rankingPromptAcknowledgedKey = 'blony-monitor:ranking-prompt-acknowledged'

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
  await api.setWindowMinSize(540, 600)
  await api.setWindowSize(base.width, base.height)
}

function readStoredNumber(key: string): number {
  try {
    const value = Number(window.localStorage.getItem(key) || 0)
    return Number.isFinite(value) && value >= 0 ? value : 0
  } catch {
    return 0
  }
}

function writeStoredValue(key: string, value: string) {
  try {
    window.localStorage.setItem(key, value)
  } catch {
    // A restricted WebView may disable local storage; the prompt can show again.
  }
}

function showRankingPromptIfNeeded() {
  if (readStoredNumber(rankingPromptAcknowledgedKey) === 1) {
    startupNoticeVisible.value = false
    startupAnnouncement.value = null
    finishStartupNotices()
    return
  }
  startupNoticeKind.value = 'ranking'
  startupAnnouncement.value = null
  startupNoticeVisible.value = true
}

function finishStartupNotices() {
  void appStore.checkNpcapOnStartup()
}

async function loadStartupNotices() {
  let latest: ServerAnnouncement | null = null
  try {
    const announcement = await api.getLatestAnnouncement()
    if (announcement.available && announcement.found) {
      latest = announcement
    }
  } catch (error) {
    console.error('加载服务端公告失败:', error)
  }

  if (latest && latest.timestamp > readStoredNumber(announcementAcknowledgedKey)) {
    startupNoticeKind.value = 'announcement'
    startupAnnouncement.value = latest
    startupNoticeVisible.value = true
    return
  }
  showRankingPromptIfNeeded()
}

function confirmStartupNotice() {
  if (startupNoticeKind.value === 'announcement') {
    const timestamp = startupAnnouncement.value?.timestamp || 0
    if (timestamp > 0) {
      writeStoredValue(announcementAcknowledgedKey, String(timestamp))
    }
    showRankingPromptIfNeeded()
    return
  }

  writeStoredValue(rankingPromptAcknowledgedKey, '1')
  startupNoticeVisible.value = false
  checkNpcapAfterSettings.value = true
  appStore.requestAdvancedSettings('ranking')
}

watch(() => appStore.advancedSettingsVisible, (visible, wasVisible) => {
  if (!visible && wasVisible && checkNpcapAfterSettings.value) {
    checkNpcapAfterSettings.value = false
    finishStartupNotices()
  }
})

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
  void loadStartupNotices()

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
      <SidebarNav />
      <div class="content-container">
        <TabsPanel />
        <DpsChartPanel
          v-if="appStore.activeTab === 'history' && appStore.isShowingHistory && appStore.selectedSkillFilters.length === 0"
        />
        <HistoryChartPanel
          v-if="appStore.activeTab === 'history' && appStore.isShowingHistory && appStore.selectedSkillFilters.length > 0"
        />
      </div>
    </div>

    <DebugPanel />

    <NpcapGuideDialog
      :visible="appStore.npcapDialogVisible"
      :message="appStore.npcapMessage"
      @ready="appStore.dismissNpcapDialog"
    />

    <StartupNoticeDialog
      :visible="startupNoticeVisible"
      :kind="startupNoticeKind"
      :announcement="startupAnnouncement"
      @confirm="confirmStartupNotice"
    />
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
  flex-direction: row;
  min-height: 0;
  overflow: hidden;
}

.content-container {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
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
