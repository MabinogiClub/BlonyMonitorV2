<script setup lang="ts">
/**
 * 标签页面板组件
 */

import { ref, watch } from 'vue'
import { useAppStore } from '../stores/app'
import DamageView from '../views/DamageView.vue'
import TakenView from '../views/TakenView.vue'
import TimelineView from '../views/TimelineView.vue'
import EntitiesView from '../views/EntitiesView.vue'
import BuffTimerView from '../views/BuffTimerView.vue'
import HistoryView from '../views/HistoryView.vue'

const appStore = useAppStore()

const tabs = [
  { name: 'bySkill', title: '造成伤害' },
  { name: 'taken', title: '受到伤害' },
  { name: 'history', title: '历史记录' },
  { name: 'timeline', title: '玩家时间轴' },
  { name: 'entities', title: '角色列表' },
  { name: 'buffTimer', title: 'Buff通知' },
]

const activeTab = ref(appStore.activeTab)

watch(() => appStore.activeTab, (newVal) => {
  activeTab.value = newVal
})

watch(activeTab, (newVal) => {
  appStore.setActiveTab(newVal)
})
</script>

<template>
  <div class="data-panel">
    <van-tabs
      v-model:active="activeTab"
      shrink
      :ellipsis="false"
      class="custom-tabs"
    >
      <van-tab
        v-for="tab in tabs"
        :key="tab.name"
        :name="tab.name"
        :title="tab.title"
      >
        <div class="tab-content-wrapper" :class="{ 'history-tab-content': tab.name === 'history' }">
          <DamageView v-if="tab.name === 'bySkill' && activeTab === 'bySkill'" />
          <TakenView v-else-if="tab.name === 'taken' && activeTab === 'taken'" />
          <HistoryView v-else-if="tab.name === 'history' && activeTab === 'history'" />
          <TimelineView v-else-if="tab.name === 'timeline' && activeTab === 'timeline'" />
          <EntitiesView v-else-if="tab.name === 'entities' && activeTab === 'entities'" />
          <BuffTimerView v-else-if="tab.name === 'buffTimer' && activeTab === 'buffTimer'" />
        </div>
      </van-tab>
    </van-tabs>
  </div>
</template>

<style lang="scss" scoped>
.data-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: visible;
  background: rgba(20, 20, 20, 0.8);
}

.custom-tabs {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;

  :deep(.van-tabs__wrap) {
    background: rgba(30, 30, 30, 0.8);
    border-bottom: 1px solid rgba(255, 255, 255, 0.1);
    min-height: 36px;
    flex-shrink: 0;
  }

  :deep(.van-tab) {
    padding: 0 12px;
    font-size: 11px;
    color: #888;
    min-width: auto;
  }

  :deep(.van-tab:hover) {
    color: #aaa;
    background: rgba(255, 255, 255, 0.05);
  }

  :deep(.van-tab--active) {
    color: #fff;
    font-weight: 500;
  }

  :deep(.van-tabs__line) {
    background: #42a5f5;
    height: 2px;
  }

  :deep(.van-tabs__content) {
    flex: 1;
    min-height: 0;
    overflow: visible;
    background: transparent;
  }

  :deep(.van-tab__panel) {
    height: 100%;
    overflow: visible;
  }
}

.tab-content-wrapper {
  height: 100%;
  overflow-y: auto;
  padding: 6px;
  background: transparent;

  &.history-tab-content {
    overflow: hidden;
    padding: 4px 6px;
  }

  &::-webkit-scrollbar {
    width: 4px;
  }

  &::-webkit-scrollbar-track {
    background: transparent;
  }

  &::-webkit-scrollbar-thumb {
    background: rgba(255, 255, 255, 0.2);
    border-radius: 2px;
  }
}
</style>
