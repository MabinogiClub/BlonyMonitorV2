<script setup lang="ts">
/**
 * 频道选择器组件
 * 提供服务器和频道的级联选择菜单
 * 在加速器模式下提供加速器选择
 */

import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useAppStore } from '../stores/app'
import { GetAccelerators, GetSelectedAccelerator, SetAccelerator } from '../../wailsjs/go/app/App'

// 获取应用状态
const appStore = useAppStore()

// 下拉菜单是否显示
const dropdownVisible = ref(false)

// 加速器下拉菜单是否显示
const acceleratorDropdownVisible = ref(false)

// 当前悬停的服务器索引
const hoverServerIndex = ref<number | null>(null)

// 延迟隐藏定时器
let hideTimer: number | null = null

// Ctrl+点击计数器（用于切换加速器模式）
const ctrlClickCount = ref(0)
let ctrlClickTimer: number | null = null

// 加速器相关
const accelerators = ref<Array<{id: string, name: string, ip: string, port: number}>>([])
const selectedAccelerator = ref('')

/**
 * 加载加速器列表
 */
async function loadAccelerators() {
  try {
    accelerators.value = await GetAccelerators()
    selectedAccelerator.value = await GetSelectedAccelerator()
  } catch (err) {
    console.error('加载加速器列表失败:', err)
  }
}

/**
 * 选择加速器
 */
async function handleSelectAccelerator(id: string) {
  try {
    const success = await SetAccelerator(id)
    if (success) {
      selectedAccelerator.value = id
    }
  } catch (err) {
    console.error('切换加速器失败:', err)
  }
}

/**
 * 获取选中的加速器显示文本
 */
const selectedAcceleratorText = computed(() => {
  const acc = accelerators.value.find(a => a.id === selectedAccelerator.value)
  return acc ? acc.name : '选择加速器'
})

/**
 * 获取服务器列表
 */
const servers = computed(() => {
  const config = appStore.channelsConfig
  return config?.servers || config?.Servers || []
})

/**
 * 获取选中的频道显示文本
 */
const selectedChannelText = computed(() => {
  if (appStore.currentChannelId <= 0) {
    return '选择频道'
  }
  
  // 查找频道名称
  for (const server of servers.value) {
    const serverName = server.name || server.Name
    const channels = server.channels || server.Channels || []
    for (const ch of channels) {
      const chId = ch.id ?? ch.ID
      if (chId === appStore.currentChannelId) {
        const chName = ch.name || ch.Name
        return `${serverName} ${chName}`
      }
    }
  }
  
  return '选择频道'
})

/**
 * 菜单是否禁用
 */
const isMenuDisabled = computed(() => appStore.autoDetect)

/**
 * 切换下拉菜单
 */
function toggleDropdown() {
  if (isMenuDisabled.value) return
  dropdownVisible.value = !dropdownVisible.value
}

/**
 * 关闭下拉菜单
 */
function closeDropdown() {
  dropdownVisible.value = false
  hoverServerIndex.value = null
}

/**
 * 切换加速器下拉菜单
 */
function toggleAcceleratorDropdown() {
  acceleratorDropdownVisible.value = !acceleratorDropdownVisible.value
}

/**
 * 关闭加速器下拉菜单
 */
function closeAcceleratorDropdown() {
  acceleratorDropdownVisible.value = false
}

/**
 * 处理自动检测切换
 */
async function handleAutoDetectChange(event: Event) {
  const target = event.target as HTMLInputElement
  await appStore.setAutoDetectMode(target.checked)
  if (target.checked) {
    closeDropdown()
  }
}

/**
 * 处理自动检测复选框的点击（用于检测 Ctrl+点击）
 */
function handleAutoDetectClick(event: MouseEvent) {
  // 只在按住 Ctrl 键时计数
  if (!event.ctrlKey) {
    return
  }

  // 阻止默认行为，避免切换复选框
  event.preventDefault()

  // 增加计数
  ctrlClickCount.value++

  // 重置超时计时器（3秒内连续点击有效）
  if (ctrlClickTimer) {
    clearTimeout(ctrlClickTimer)
  }
  ctrlClickTimer = window.setTimeout(() => {
    ctrlClickCount.value = 0
  }, 3000)

  // 判断切换逻辑
  if (ctrlClickCount.value === 7) {
    // 切换到加速器模式
    appStore.setAcceleratorMode(true)
  } else if (ctrlClickCount.value === 9) {
    // 切换回普通模式
    appStore.setAcceleratorMode(false)
    ctrlClickCount.value = 0
  }
}

/**
 * 选择频道
 */
async function handleSelectChannel(channelId: number) {
  await appStore.selectChannel(channelId)
  closeDropdown()
}

/**
 * 鼠标进入服务器
 */
function handleServerEnter(index: number) {
  if (hideTimer) {
    clearTimeout(hideTimer)
    hideTimer = null
  }
  hoverServerIndex.value = index
}

/**
 * 鼠标离开服务器
 */
function handleServerLeave() {
  hideTimer = window.setTimeout(() => {
    hoverServerIndex.value = null
  }, 300)
}

/**
 * 鼠标进入频道子菜单
 */
function handleChannelsEnter() {
  if (hideTimer) {
    clearTimeout(hideTimer)
    hideTimer = null
  }
}

/**
 * 鼠标离开频道子菜单
 */
function handleChannelsLeave() {
  hideTimer = window.setTimeout(() => {
    hoverServerIndex.value = null
  }, 300)
}

/**
 * 点击外部关闭菜单
 */
function handleClickOutside(event: MouseEvent) {
  const target = event.target as HTMLElement
  if (!target.closest('.channel-selector')) {
    closeDropdown()
    closeAcceleratorDropdown()
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
  // 加载加速器列表
  loadAccelerators()
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
  if (hideTimer) {
    clearTimeout(hideTimer)
  }
})
</script>

<template>
  <div class="channel-selector">
    <!-- 自动检测复选框 -->
    <label class="auto-detect-label">
      <input
        type="checkbox"
        :checked="appStore.autoDetect"
        @click="handleAutoDetectClick"
        @change="handleAutoDetectChange"
      >
      <span>{{ appStore.acceleratorMode ? '加速器兼容模式' : '自动' }}</span>
    </label>

    <!-- 频道级联菜单（普通模式） -->
    <div v-if="!appStore.acceleratorMode" class="cascade-menu">
      <!-- 触发器 -->
      <div
        class="cascade-trigger"
        :class="{ disabled: isMenuDisabled }"
        @click.stop="toggleDropdown"
      >
        <span>{{ selectedChannelText }}</span>
        <span class="cascade-arrow">▼</span>
      </div>
      
      <!-- 下拉菜单 -->
      <div class="cascade-dropdown" :class="{ show: dropdownVisible }">
        <!-- 服务器列表 -->
        <div 
          v-for="(server, serverIndex) in servers" 
          :key="serverIndex"
          class="cascade-server"
          @mouseenter="handleServerEnter(serverIndex)"
          @mouseleave="handleServerLeave"
        >
          <span>{{ server.name || server.Name }}</span>
          <span class="cascade-server-arrow">▶</span>
          
          <!-- 频道子菜单 -->
          <div 
            class="cascade-channels" 
            :class="{ show: hoverServerIndex === serverIndex }"
            @mouseenter="handleChannelsEnter"
            @mouseleave="handleChannelsLeave"
          >
            <div class="cascade-channels-inner">
              <div 
                v-for="(channel, channelIndex) in (server.channels || server.Channels || [])" 
                :key="channelIndex"
                class="cascade-channel"
                @click.stop="handleSelectChannel(channel.id ?? channel.ID ?? 0)"
              >
                {{ channel.name || channel.Name }}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 加速器选择（加速器模式） -->
    <div v-if="appStore.acceleratorMode" class="accelerator-selector">
      <!-- 触发器 -->
      <div
        class="accelerator-trigger"
        @click.stop="toggleAcceleratorDropdown"
      >
        <span>{{ selectedAcceleratorText }}</span>
        <span class="accelerator-arrow">▼</span>
      </div>

      <!-- 下拉菜单 -->
      <div class="accelerator-dropdown" :class="{ show: acceleratorDropdownVisible }">
        <div
          v-for="acc in accelerators"
          :key="acc.id"
          class="accelerator-item"
          :class="{ active: acc.id === selectedAccelerator }"
          @click.stop="handleSelectAccelerator(acc.id); closeAcceleratorDropdown()"
        >
          {{ acc.name }}
        </div>
      </div>
    </div>
  </div>
</template>

<style lang="scss" scoped>
/**
 * 频道选择器样式
 * 定义频道选择菜单的样式
 */

// 频道选择器
.channel-selector {
  display: flex;
  align-items: center;
  gap: 6px;
  --wails-draggable: no-drag;
}

.auto-detect-label {
  display: flex;
  align-items: center;
  gap: 3px;
  font-size: 10px;
  color: #aaa;
  cursor: pointer;

  input[type="checkbox"] {
    width: 12px;
    height: 12px;
    margin: 0;
    cursor: pointer;
    accent-color: #ffc107;
  }

  &:hover {
    color: #fff;
  }
}

// 级联菜单
.cascade-menu {
  position: relative;
}

.cascade-trigger {
  display: flex;
  align-items: center;
  gap: 4px;
  height: 18px;
  font-size: 10px;
  padding: 0 6px;
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 3px;
  background: rgba(30, 30, 30, 0.9);
  color: #ddd;
  cursor: pointer;
  user-select: none;
  min-width: 70px;

  &:hover {
    border-color: rgba(255, 193, 7, 0.5);
    background-color: rgba(40, 40, 40, 0.95);
  }

  &.disabled {
    opacity: 0.5;
    cursor: not-allowed;
    pointer-events: none;
  }
}

.cascade-arrow {
  font-size: 8px;
  color: #888;
  margin-left: auto;
}

// 下拉菜单
.cascade-dropdown {
  display: none;
  position: absolute;
  top: 100%;
  left: 0;
  margin-top: 2px;
  background: rgba(30, 30, 30, 0.98);
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 4px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.5);
  z-index: 1000;
  min-width: 80px;

  &.show {
    display: block;
  }
}

// 服务器项
.cascade-server {
  position: relative;
  padding: 6px 10px;
  font-size: 10px;
  color: #ffc107;
  font-weight: 600;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);

  &:last-child {
    border-bottom: none;
  }

  &:hover {
    background: rgba(255, 193, 7, 0.15);
  }
}

.cascade-server-arrow {
  font-size: 8px;
  color: #888;
}

// 频道子菜单
.cascade-channels {
  display: none;
  position: absolute;
  left: 100%;
  top: -1px;
  margin-left: 0;
  padding-left: 2px;
  background: transparent;
  min-width: 70px;

  &.show {
    display: block;
  }
}

.cascade-channels-inner {
  background: rgba(30, 30, 30, 0.98);
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 4px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.5);
}

.cascade-channel {
  padding: 5px 10px;
  font-size: 10px;
  color: #ddd;
  cursor: pointer;
  white-space: nowrap;

  &:hover {
    background: rgba(255, 193, 7, 0.2);
    color: #fff;
  }

  &:first-child {
    border-radius: 3px 3px 0 0;
  }

  &:last-child {
    border-radius: 0 0 3px 3px;
  }
}

// 加速器选择器
.accelerator-selector {
  position: relative;
}

.accelerator-trigger {
  display: flex;
  align-items: center;
  gap: 4px;
  height: 18px;
  font-size: 10px;
  padding: 0 6px;
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 3px;
  background: rgba(30, 30, 30, 0.9);
  color: #ddd;
  cursor: pointer;
  user-select: none;
  min-width: 70px;

  &:hover {
    border-color: rgba(255, 193, 7, 0.5);
    background-color: rgba(40, 40, 40, 0.95);
  }
}

.accelerator-arrow {
  font-size: 8px;
  margin-left: auto;
}

.accelerator-dropdown {
  position: absolute;
  top: calc(100% + 2px);
  left: 0;
  min-width: 100%;
  background: rgba(30, 30, 30, 0.98);
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 4px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.5);
  opacity: 0;
  visibility: hidden;
  transform: translateY(-5px);
  transition: all 0.2s ease;
  z-index: 1000;

  &.show {
    opacity: 1;
    visibility: visible;
    transform: translateY(0);
  }
}

.accelerator-item {
  padding: 5px 10px;
  font-size: 10px;
  color: #ddd;
  cursor: pointer;
  white-space: nowrap;

  &:hover {
    background: rgba(255, 193, 7, 0.2);
    color: #fff;
  }

  &.active {
    background: rgba(255, 193, 7, 0.3);
    color: #ffc107;
  }

  &:first-child {
    border-radius: 3px 3px 0 0;
  }

  &:last-child {
    border-radius: 0 0 3px 3px;
  }
}

</style>
