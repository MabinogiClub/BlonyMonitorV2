<script setup lang="ts">
/**
 * 标题栏组件
 */

import { ref, computed } from 'vue'
import { useAppStore } from '../stores/app'
import AdvancedSettings from './AdvancedSettings.vue'
import SvgIcon from '@jamescoyle/vue-icon'
import { mdiPin, mdiPinOff, mdiBug, mdiBugPlay, mdiCog, mdiBroom, mdiWindowClose, mdiChartLine } from '@mdi/js'
import appIcon from '../assets/appicon.png'
import { showLoadingToast, showSuccessToast, showFailToast } from 'vant'

const appStore = useAppStore()

const advancedSettingsVisible = ref(false)
const closeDialogVisible = ref(false)

function openAdvancedSettings() {
  advancedSettingsVisible.value = true
}

function closeAdvancedSettings() {
  advancedSettingsVisible.value = false
}

function showCloseDialog() {
  closeDialogVisible.value = true
}

function hideCloseDialog() {
  closeDialogVisible.value = false
}

function minimizeToTray() {
  closeDialogVisible.value = false
  appStore.hide()
}

function confirmQuit() {
  closeDialogVisible.value = false
  appStore.quit()
}

async function handleClearStats() {
  const toast = showLoadingToast({
    message: '保存并清理中...',
    forbidClick: true,
    duration: 0,
  })

  try {
    await appStore.clearAndSave()
    toast.close?.()
    showSuccessToast('已保存并清理')
  } catch (e) {
    console.error('保存并清理失败:', e)
    toast.close?.()
    showFailToast('保存并清理失败')
  }
}

const alwaysOnTopIcon = computed(() => {
  return appStore.alwaysOnTop ? mdiPin : mdiPinOff
})

const debugIcon = computed(() => {
  return appStore.debugVisible ? mdiBugPlay : mdiBug
})
</script>

<template>
  <div class="titlebar">
    <div class="titlebar-left">
      <img class="app-icon" :src="appIcon" alt="BlonyMonitorV2 图标">
      <span class="titlebar-title" title="布罗妮大调查 V2">BlonyMonitorV2</span>
    </div>

    <div class="titlebar-buttons">
      <button
        class="titlebar-btn always-on-top"
        :class="{ active: appStore.alwaysOnTop }"
        title="固定在前"
        @click="appStore.toggleAlwaysOnTop"
      >
        <svg-icon type="mdi" :path="alwaysOnTopIcon" :size="14" />
      </button>

      <button
        class="titlebar-btn chart-toggle"
        :class="{ active: appStore.chartVisible }"
        title="DPS 趋势"
        @click="appStore.toggleChartVisible"
      >
        <svg-icon type="mdi" :path="mdiChartLine" :size="14" />
      </button>

      <button
        class="titlebar-btn debug"
        :class="{ active: appStore.debugVisible }"
        title="调试"
        @click="appStore.toggleDebug"
      >
        <svg-icon type="mdi" :path="debugIcon" :size="14" />
      </button>

      <button
        class="titlebar-btn settings"
        title="高级设置"
        @click="openAdvancedSettings"
      >
        <svg-icon type="mdi" :path="mdiCog" :size="14" />
      </button>

      <button
        class="titlebar-btn"
        title="保存并清空"
        @click="handleClearStats"
      >
        <svg-icon type="mdi" :path="mdiBroom" :size="14" />
      </button>

      <button
        class="titlebar-btn close"
        title="关闭"
        @click="showCloseDialog"
      >
        <svg-icon type="mdi" :path="mdiWindowClose" :size="14" />
      </button>
    </div>
  </div>

  <AdvancedSettings
    :visible="advancedSettingsVisible"
    @close="closeAdvancedSettings"
  />

  <van-popup
    v-model:show="closeDialogVisible"
    position="center"
    round
    :style="{ background: 'rgba(40, 40, 40, 0.98)', minWidth: '280px' }"
    @click-overlay="hideCloseDialog"
  >
    <div class="close-dialog">
      <div class="close-dialog-title">关闭应用</div>
      <div class="close-dialog-content">请选择关闭方式：</div>
      <div class="close-dialog-buttons">
        <van-button
          plain
          size="normal"
          block
          class="dialog-btn-minimize"
          @click="minimizeToTray"
        >
          最小化到托盘
        </van-button>
        <van-button
          type="danger"
          size="normal"
          block
          @click="confirmQuit"
        >
          退出应用
        </van-button>
      </div>
    </div>
  </van-popup>
</template>

<style lang="scss" scoped>
.titlebar {
  height: 28px;
  background: rgba(30, 30, 30, 0.95);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 8px;
  --wails-draggable: drag;
  cursor: move;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

.titlebar-left {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
  overflow: hidden;
}

.app-icon {
  width: 16px;
  height: 16px;
  border-radius: 4px;
}

.titlebar-title {
  font-size: 12px;
  font-weight: 500;
  color: #ddd;
}

.titlebar-buttons {
  display: flex;
  gap: 4px;
  --wails-draggable: no-drag;
}

.titlebar-btn {
  width: 20px;
  height: 20px;
  border: none;
  background: rgba(255, 255, 255, 0.1);
  color: #aaa;
  border-radius: 3px;
  cursor: pointer;
  font-size: 10px;
  display: flex;
  align-items: center;
  justify-content: center;

  &:hover {
    background: rgba(255, 255, 255, 0.2);
    color: #fff;
  }

  &.close:hover {
    background: rgba(244, 67, 54, 0.8);
  }

  &.debug {
    background: rgba(255, 152, 0, 0.3);

    &:hover {
      background: rgba(255, 152, 0, 0.6);
    }

    &.active {
      background: rgba(255, 152, 0, 0.8);
      color: #fff;
    }
  }

  &.always-on-top {
    background: rgba(33, 150, 243, 0.3);

    &:hover {
      background: rgba(33, 150, 243, 0.6);
    }

    &.active {
      background: rgba(33, 150, 243, 0.8);
      color: #fff;
    }
  }

  &.chart-toggle {
    background: rgba(76, 175, 80, 0.3);

    &:hover {
      background: rgba(76, 175, 80, 0.6);
    }

    &.active {
      background: rgba(76, 175, 80, 0.9);
      color: #fff;
    }
  }
}
</style>

<style lang="scss">
.close-dialog {
  padding: 20px;
}

.close-dialog-title {
  font-size: 16px;
  font-weight: 600;
  color: #fff;
  margin-bottom: 12px;
  text-align: center;
}

.close-dialog-content {
  font-size: 13px;
  color: #aaa;
  margin-bottom: 20px;
  text-align: center;
}

.close-dialog-buttons {
  display: flex;
  flex-direction: column;
  gap: 8px;

  .dialog-btn-minimize {
    background: rgba(255, 255, 255, 0.12) !important;
    border-color: rgba(255, 255, 255, 0.2) !important;
    color: #ccc !important;

    &:hover {
      background: rgba(255, 255, 255, 0.2) !important;
      color: #fff !important;
    }
  }

  .van-button--danger {
    background: rgba(244, 67, 54, 0.7) !important;
    border-color: rgba(244, 67, 54, 0.7) !important;

    &:hover {
      background: rgba(244, 67, 54, 0.9) !important;
    }
  }
}
</style>
