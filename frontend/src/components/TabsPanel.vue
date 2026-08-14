<script setup lang="ts">
import { computed } from 'vue'
import { useAppStore } from '../stores/app'
import HomeView from '../views/HomeView.vue'
import DamageView from '../views/DamageView.vue'
import TakenView from '../views/TakenView.vue'
import HistoryView from '../views/HistoryView.vue'
import TimelineView from '../views/TimelineView.vue'
import EntitiesView from '../views/EntitiesView.vue'
import BuffTimerView from '../views/BuffTimerView.vue'
import FarmView from '../views/FarmView.vue'

const appStore = useAppStore()
const dpsTabs = [
  { name: 'bySkill', title: '造成伤害' },
  { name: 'taken', title: '受到伤害' },
  { name: 'history', title: '历史记录' },
]
const isDpsPage = computed(() => dpsTabs.some(tab => tab.name === appStore.activeTab))
</script>

<template>
  <div class="data-panel">
    <div v-if="isDpsPage" class="secondary-tabs" role="tablist" aria-label="DPS统计">
      <button
        v-for="tab in dpsTabs"
        :key="tab.name"
        type="button"
        role="tab"
        class="secondary-tab"
        :class="{ active: appStore.activeTab === tab.name }"
        :aria-selected="appStore.activeTab === tab.name"
        @click="appStore.setActiveTab(tab.name)"
      >
        {{ tab.title }}
      </button>
    </div>

    <div class="tab-content-wrapper" :class="{ 'history-tab-content': appStore.activeTab === 'history' }">
      <HomeView v-if="appStore.activeTab === 'home'" />
      <DamageView v-else-if="appStore.activeTab === 'bySkill'" />
      <TakenView v-else-if="appStore.activeTab === 'taken'" />
      <HistoryView v-else-if="appStore.activeTab === 'history'" />
      <TimelineView v-else-if="appStore.activeTab === 'timeline'" />
      <EntitiesView v-else-if="appStore.activeTab === 'entities'" />
      <BuffTimerView v-else-if="appStore.activeTab === 'buffTimer'" />
      <FarmView v-else-if="appStore.activeTab === 'farm'" />
    </div>
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

.secondary-tabs {
  height: 36px;
  min-height: 36px;
  flex-shrink: 0;
  padding: 0 8px;
  display: flex;
  align-items: stretch;
  gap: 2px;
  background: rgba(30, 30, 30, 0.8);
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

.secondary-tab {
  min-width: 72px;
  padding: 0 10px;
  position: relative;
  border: 0;
  color: #888;
  background: transparent;
  font-family: inherit;
  font-size: 11px;
  cursor: pointer;

  &:hover {
    color: #ccc;
    background: rgba(255, 255, 255, 0.04);
  }

  &.active {
    color: #fff;
    font-weight: 500;

    &::after {
      content: '';
      position: absolute;
      right: 10px;
      bottom: 0;
      left: 10px;
      height: 2px;
      background: #42a5f5;
    }
  }
}

.tab-content-wrapper {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 6px;
  background: transparent;

  &.history-tab-content {
    overflow: hidden;
    padding: 4px 6px;
  }

  &:has(.home-view) {
    padding: 0;
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
