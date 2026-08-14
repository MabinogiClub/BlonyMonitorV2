<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import SvgIcon from '@jamescoyle/vue-icon'
import {
  mdiAccountGroupOutline,
  mdiBellRingOutline,
  mdiCogOutline,
  mdiHome,
  mdiSproutOutline,
  mdiSwordCross,
  mdiTimelineClockOutline,
} from '@mdi/js'
import { useAppStore } from '../stores/app'

const appStore = useAppStore()
const dpsTabs = new Set(['bySkill', 'taken', 'history'])
const lastDpsTab = ref(dpsTabs.has(appStore.activeTab) ? appStore.activeTab : 'bySkill')

const navigation = [
  { name: 'home', title: '首页', icon: mdiHome },
  { name: 'dps', title: 'DPS统计', icon: mdiSwordCross },
  { name: 'timeline', title: '玩家时间轴', icon: mdiTimelineClockOutline },
  { name: 'entities', title: '角色列表', icon: mdiAccountGroupOutline },
  { name: 'buffTimer', title: 'Buff通知', icon: mdiBellRingOutline },
  { name: 'farm', title: '塔汀农场', icon: mdiSproutOutline },
]

const activePage = computed(() => dpsTabs.has(appStore.activeTab) ? 'dps' : appStore.activeTab)

watch(() => appStore.activeTab, (tab) => {
  if (dpsTabs.has(tab)) lastDpsTab.value = tab
})

function selectPage(name: string) {
  appStore.setActiveTab(name === 'dps' ? lastDpsTab.value : name)
}
</script>

<template>
  <nav class="sidebar-nav" aria-label="主导航">
    <button
      v-for="item in navigation"
      :key="item.name"
      type="button"
      class="nav-item"
      :class="{ active: activePage === item.name }"
      :aria-current="activePage === item.name ? 'page' : undefined"
      :title="item.title"
      @click="selectPage(item.name)"
    >
      <svg-icon type="mdi" :path="item.icon" :size="19" class="nav-icon" />
      <span class="nav-label">{{ item.title }}</span>
    </button>

    <button
      type="button"
      class="nav-item settings-item"
      :class="{ active: appStore.advancedSettingsVisible }"
      :aria-expanded="appStore.advancedSettingsVisible"
      title="高级设置"
      @click="appStore.requestAdvancedSettings('general')"
    >
      <svg-icon type="mdi" :path="mdiCogOutline" :size="19" class="nav-icon" />
      <span class="nav-label">设置</span>
    </button>
  </nav>
</template>

<style lang="scss" scoped>
.sidebar-nav {
  width: 104px;
  min-width: 104px;
  height: 100%;
  padding: 6px;
  display: flex;
  flex-direction: column;
  gap: 3px;
  overflow-y: auto;
  background: rgba(25, 25, 25, 0.94);
  border-right: 1px solid rgba(255, 255, 255, 0.1);
}

.nav-item {
  width: 100%;
  min-height: 48px;
  padding: 6px 5px;
  border: 0;
  border-radius: 4px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  color: #969696;
  background: transparent;
  cursor: pointer;
  font-family: inherit;
  transition: color 0.15s ease, background-color 0.15s ease;

  &:hover {
    color: #ddd;
    background: rgba(255, 255, 255, 0.06);
  }

  &.active {
    color: #fff;
    background: rgba(66, 165, 245, 0.18);
    box-shadow: inset 3px 0 0 #42a5f5;
  }
}

.nav-icon {
  flex-shrink: 0;
}

.settings-item {
  margin-top: auto;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
}

.nav-label {
  width: 100%;
  overflow: hidden;
  color: inherit;
  font-size: 11px;
  line-height: 1.2;
  text-align: center;
  white-space: nowrap;
}

.sidebar-nav::-webkit-scrollbar {
  width: 3px;
}

.sidebar-nav::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.16);
}
</style>
