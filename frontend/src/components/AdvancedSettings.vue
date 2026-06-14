<script setup lang="ts">
/**
 * 高级设置对话框组件
 * 用于手动选择网卡等高级配置
 * 使用 Vant 组件库
 */

import { ref, watch, computed, onMounted, onUnmounted } from 'vue'
import { GetAllNics, GetManualNic, SetManualNic, GetAccelerators, GetSelectedAccelerator, SetAccelerator } from '../../wailsjs/go/app/App'
import { useAppStore } from '../stores/app'

// 定义 props
const props = defineProps<{
  visible: boolean
}>()

// 定义 emits
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'update:visible', value: boolean): void
}>()

// 获取应用状态
const appStore = useAppStore()

// 网卡列表
const nics = ref<Array<{ name: string; description: string }>>([])
// 当前选择的网卡
const selectedNic = ref('')
// 加载状态
const loading = ref(false)
// 搜索关键词
const searchKeyword = ref('')

// 透明度
const opacity = ref(100)

// 频道相关
const autoDetect = ref(true)
const currentChannelId = ref(0)
const acceleratorMode = ref(false)
const accelerators = ref<Array<{id: string, name: string, ip: string, port: number}>>([])
const selectedAccelerator = ref('')
const channelDropdownVisible = ref(false)
const acceleratorDropdownVisible = ref(false)

/**
 * 加载网卡列表
 */
async function loadNics() {
  loading.value = true
  try {
    nics.value = await GetAllNics()
    selectedNic.value = await GetManualNic()
  } catch (err) {
    console.error('加载网卡列表失败:', err)
  } finally {
    loading.value = false
  }
}

/**
 * 加载透明度
 */
async function loadOpacity() {
  try {
    opacity.value = appStore.opacity
  } catch (err) {
    console.error('加载透明度失败:', err)
  }
}

/**
 * 处理透明度变化
 */
async function handleOpacityChange(event: Event) {
  const target = event.target as HTMLInputElement
  const newOpacity = parseInt(target.value, 10)
  opacity.value = newOpacity
  await appStore.setOpacity(newOpacity)
}

/**
 * 加载频道设置
 */
async function loadChannelSettings() {
  try {
    autoDetect.value = appStore.autoDetect
    currentChannelId.value = appStore.currentChannelId
    acceleratorMode.value = appStore.acceleratorMode
  } catch (err) {
    console.error('加载频道设置失败:', err)
  }
}

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
 * 处理自动检测切换
 */
async function handleAutoDetectChange(event: Event) {
  const target = event.target as HTMLInputElement
  autoDetect.value = target.checked
  await appStore.setAutoDetectMode(target.checked)
  if (target.checked) {
    closeChannelDropdown()
  }
}

/**
 * 切换频道下拉菜单
 */
function toggleChannelDropdown() {
  if (autoDetect.value) return
  channelDropdownVisible.value = !channelDropdownVisible.value
}

/**
 * 关闭频道下拉菜单
 */
function closeChannelDropdown() {
  channelDropdownVisible.value = false
}

/**
 * 选择频道
 */
async function handleSelectChannel(channelId: number) {
  currentChannelId.value = channelId
  await appStore.selectChannel(channelId)
  closeChannelDropdown()
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
  if (currentChannelId.value <= 0) {
    return '选择频道'
  }
  
  // 查找频道名称
  for (const server of servers.value) {
    const serverName = server.name || server.Name
    const channels = server.channels || server.Channels || []
    for (const ch of channels) {
      const chId = ch.id ?? ch.ID
      if (chId === currentChannelId.value) {
        const chName = ch.name || ch.Name
        return `${serverName} ${chName}`
      }
    }
  }
  
  return '选择频道'
})

/**
 * 获取选中的加速器显示文本
 */
const selectedAcceleratorText = computed(() => {
  const acc = accelerators.value.find(a => a.id === selectedAccelerator.value)
  return acc ? acc.name : '选择加速器'
})

/**
 * 处理网卡选择变化（即时保存）
 */
async function handleNicChange(nicName: string) {
  try {
    await SetManualNic(nicName)
  } catch (err) {
    console.error('设置网卡失败:', err)
  }
}

/**
 * 关闭对话框
 */
function close() {
  // 关闭所有下拉菜单
  closeChannelDropdown()
  closeAcceleratorDropdown()
  emit('close')
  emit('update:visible', false)
}

/**
 * 过滤后的网卡列表
 */
const filteredNics = computed(() => {
  if (!searchKeyword.value.trim()) {
    return nics.value
  }
  const keyword = searchKeyword.value.toLowerCase()
  return nics.value.filter(nic =>
    nic.name.toLowerCase().includes(keyword) ||
    (nic.description && nic.description.toLowerCase().includes(keyword))
  )
})

// 点击外部关闭下拉菜单
function handleClickOutside(event: MouseEvent) {
  const target = event.target as HTMLElement
  if (!target.closest('.channel-select-wrapper') && !target.closest('.accelerator-select-wrapper')) {
    closeChannelDropdown()
    closeAcceleratorDropdown()
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})

// 监听网卡选择变化，即时保存
watch(selectedNic, (newVal) => {
  if (props.visible) {
    handleNicChange(newVal)
  }
})

// 监听对话框显示状态，当显示时加载数据
watch(() => props.visible, (newVal) => {
  if (newVal) {
    loadNics()
    loadOpacity()
    loadChannelSettings()
    if (acceleratorMode.value) {
      loadAccelerators()
    }
    searchKeyword.value = '' // 重置搜索关键词
    // 关闭下拉菜单
    closeChannelDropdown()
    closeAcceleratorDropdown()
  }
})
</script>

<template>
  <!-- 使用 Vant Popup 组件 -->
  <van-popup
    :show="visible"
    position="center"
    round
    closeable
    close-icon-position="top-right"
    :style="{ width: '600px', maxWidth: '90vw', maxHeight: '80vh' }"
    @click-overlay="close"
    @click-close-icon="close"
    teleport="body"
  >
    <div class="advanced-settings-dialog">
      <!-- 标题 -->
      <div class="dialog-header">
        <h3>高级设置</h3>
      </div>

      <!-- 内容 -->
      <div class="dialog-content">
        <!-- 透明度设置 -->
        <div class="setting-section">
          <div class="section-header">
            <div>
              <h4>窗口透明度</h4>
              <p class="setting-desc">调整窗口透明度（20-100）</p>
            </div>
          </div>
          <div class="opacity-control">
            <input
              type="range"
              id="opacitySlider"
              min="20"
              max="100"
              :value="opacity"
              @input="handleOpacityChange"
              @change="handleOpacityChange"
            >
            <span class="opacity-value">{{ opacity }}%</span>
          </div>
        </div>

        <!-- 分割线 -->
        <div class="setting-divider"></div>

        <!-- 频道设置 -->
        <div class="setting-section">
          <div class="section-header">
            <div>
              <h4>频道选择</h4>
              <p class="setting-desc">选择频道或启用自动检测</p>
            </div>
          </div>
          
          <!-- 自动检测开关 -->
          <div class="auto-detect-control">
            <label class="auto-detect-label">
              <input
                type="checkbox"
                :checked="autoDetect"
                @change="handleAutoDetectChange"
              >
              <span>{{ acceleratorMode ? '加速器兼容模式' : '自动检测频道' }}</span>
            </label>
          </div>

          <!-- 频道选择（普通模式） -->
          <div v-if="!acceleratorMode" class="channel-select-control">
            <div class="channel-select-wrapper">
              <div 
                class="channel-select-trigger" 
                :class="{ disabled: autoDetect }"
                @click.stop="toggleChannelDropdown"
              >
                <span>{{ selectedChannelText }}</span>
                <span class="select-arrow">▼</span>
              </div>
              <div 
                v-if="!autoDetect && channelDropdownVisible" 
                class="channel-select-dropdown"
                @click.stop
              >
                <div 
                  v-for="(server, serverIndex) in servers" 
                  :key="serverIndex"
                  class="channel-server-group"
                >
                  <div class="channel-server-name">{{ server.name || server.Name }}</div>
                  <div 
                    v-for="(channel, channelIndex) in (server.channels || server.Channels || [])" 
                    :key="channelIndex"
                    class="channel-item"
                    :class="{ active: (channel.id ?? channel.ID ?? 0) === currentChannelId }"
                    @click="handleSelectChannel(channel.id ?? channel.ID ?? 0)"
                  >
                    {{ channel.name || channel.Name }}
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- 加速器选择（加速器模式） -->
          <div v-if="acceleratorMode" class="accelerator-select-control">
            <div class="accelerator-select-wrapper">
              <div 
                class="accelerator-select-trigger"
                @click.stop="toggleAcceleratorDropdown"
              >
                <span>{{ selectedAcceleratorText }}</span>
                <span class="select-arrow">▼</span>
              </div>
              <div 
                v-if="acceleratorDropdownVisible"
                class="accelerator-select-dropdown"
                @click.stop
              >
                <div
                  v-for="acc in accelerators"
                  :key="acc.id"
                  class="accelerator-item"
                  :class="{ active: acc.id === selectedAccelerator }"
                  @click="handleSelectAccelerator(acc.id); closeAcceleratorDropdown()"
                >
                  {{ acc.name }}
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 分割线 -->
        <div class="setting-divider"></div>

        <!-- 网卡选择 -->
        <div class="setting-section">
          <div class="section-header">
            <div>
              <h4>网卡选择</h4>
              <p class="setting-desc">手动选择用于抓包的网卡（留空则自动检测）</p>
            </div>
            <van-tag v-if="!loading" type="primary" plain>
              共 {{ nics.length }} 个
              <template v-if="searchKeyword && filteredNics.length !== nics.length">
                / 显示 {{ filteredNics.length }} 个
              </template>
            </van-tag>
          </div>

          <!-- 搜索框 -->
          <van-search
            v-if="!loading"
            v-model="searchKeyword"
            placeholder="搜索网卡名称或描述..."
            show-action
            clearable
            class="nic-search"
          >
            <template #action>
              <span v-if="searchKeyword" @click="searchKeyword = ''">清除</span>
            </template>
          </van-search>

          <!-- 加载状态 -->
          <div v-if="loading" class="loading-wrapper">
            <van-loading size="24px" vertical>加载中...</van-loading>
          </div>

          <!-- 网卡列表 -->
          <van-radio-group v-else v-model="selectedNic" class="nic-list">
            <!-- 自动检测选项 -->
            <van-cell-group inset class="nic-cell-group">
              <van-cell clickable @click="selectedNic = ''">
                <template #title>
                  <div class="nic-info">
                    <span class="nic-name">自动检测</span>
                    <span class="nic-desc">让程序自动查找合适的网卡</span>
                  </div>
                </template>
                <template #right-icon>
                  <van-radio name="" />
                </template>
              </van-cell>

              <!-- 网卡选项 -->
              <van-cell
                v-for="nic in filteredNics"
                :key="nic.name"
                clickable
                @click="selectedNic = nic.name"
              >
                <template #title>
                  <div class="nic-info">
                    <span class="nic-name">{{ nic.description || nic.name }}</span>
                    <span class="nic-desc">{{ nic.name }}</span>
                  </div>
                </template>
                <template #right-icon>
                  <van-radio :name="nic.name" />
                </template>
              </van-cell>
            </van-cell-group>
          </van-radio-group>
        </div>
      </div>
    </div>
  </van-popup>
</template>

<style scoped lang="scss">
/**
 * 高级设置对话框样式
 * 使用 Vant 组件，自定义暗色主题
 */

.advanced-settings-dialog {
  background: rgba(40, 40, 40, 0.98);
  display: flex;
  flex-direction: column;
  max-height: 80vh;
}

.dialog-header {
  padding: 16px 20px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  
  h3 {
    margin: 0;
    font-size: 16px;
    color: #fff;
  }
}

.dialog-content {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
}

.setting-section {
  margin-bottom: 0;
  padding-bottom: 16px;
}

.setting-divider {
  height: 1px;
  background: rgba(255, 255, 255, 0.1);
  margin: 16px 0;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 12px;
  gap: 16px;
  
  h4 {
    margin: 0 0 6px 0;
    font-size: 14px;
    color: #fff;
    font-weight: 600;
  }
}

.setting-desc {
  margin: 0;
  font-size: 12px;
  color: #aaa;
  line-height: 1.5;
}

// 搜索框样式
.nic-search {
  margin-bottom: 12px;
  
  :deep(.van-search__content) {
    background: rgba(50, 50, 50, 0.8);
    border: 1px solid rgba(255, 255, 255, 0.1);
    
    .van-field__control {
      color: #fff;
      
      &::placeholder {
        color: #666;
      }
    }
  }
  
  :deep(.van-search__action) {
    color: #42a5f5;
    font-size: 12px;
    padding-left: 8px;
  }
}

// 加载状态
.loading-wrapper {
  padding: 40px 20px;
  text-align: center;
}

// 网卡列表
.nic-list {
  max-height: 350px;
  overflow-y: auto;
  
  &::-webkit-scrollbar {
    width: 6px;
  }

  &::-webkit-scrollbar-track {
    background: rgba(0, 0, 0, 0.2);
    border-radius: 3px;
  }

  &::-webkit-scrollbar-thumb {
    background: rgba(255, 255, 255, 0.2);
    border-radius: 3px;
    
    &:hover {
      background: rgba(255, 255, 255, 0.3);
    }
  }
}

// 网卡单元格组
.nic-cell-group {
  margin: 0;
  
  :deep(.van-cell-group--inset) {
    margin: 0;
  }
  
  :deep(.van-cell) {
    background: rgba(50, 50, 50, 0.6);
    padding: 10px 12px;
  }
  
  :deep(.van-cell::after) {
    border-color: rgba(255, 255, 255, 0.08);
  }
  
  :deep(.van-cell:hover) {
    background: rgba(60, 60, 60, 0.8);
  }
  
  :deep(.van-cell:first-child) {
    border-radius: 8px 8px 0 0;
  }
  
  :deep(.van-cell:last-child) {
    border-radius: 0 0 8px 8px;
  }
  
  :deep(.van-cell:last-child::after) {
    display: none;
  }
  
  :deep(.van-cell:only-child) {
    border-radius: 8px;
  }
}

// 网卡信息
.nic-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.nic-name {
  font-size: 13px;
  color: #fff;
  font-weight: 500;
  line-height: 1.4;
}

.nic-desc {
  font-size: 11px;
  color: #888;
  word-break: break-all;
  line-height: 1.4;
}

// 单选按钮样式
:deep(.van-radio) {
  .van-radio__icon {
    font-size: 18px;
    
    .van-icon {
      border-color: rgba(255, 255, 255, 0.3);
      background: transparent;
    }
  }
  
  &.van-radio--checked .van-radio__icon .van-icon {
    background: #42a5f5;
    border-color: #42a5f5;
  }
}

// 透明度控制
.opacity-control {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 0;
}

#opacitySlider {
  flex: 1;
  height: 6px;
  -webkit-appearance: none;
  appearance: none;
  background: rgba(255, 255, 255, 0.2);
  border-radius: 3px;
  outline: none;
  cursor: pointer;

  &::-webkit-slider-thumb {
    -webkit-appearance: none;
    appearance: none;
    width: 16px;
    height: 16px;
    background: #42a5f5;
    border-radius: 50%;
    cursor: pointer;
    transition: background 0.2s;

    &:hover {
      background: #1e88e5;
    }
  }

  &::-moz-range-thumb {
    width: 16px;
    height: 16px;
    background: #42a5f5;
    border: none;
    border-radius: 50%;
    cursor: pointer;
  }
}

.opacity-value {
  font-size: 13px;
  color: #fff;
  font-weight: 500;
  min-width: 45px;
  text-align: right;
}

// 自动检测控制
.auto-detect-control {
  margin-bottom: 12px;
}

.auto-detect-label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: #fff;
  cursor: pointer;
  padding: 8px 0;

  input[type="checkbox"] {
    width: 16px;
    height: 16px;
    margin: 0;
    cursor: pointer;
    accent-color: #42a5f5;
  }

  &:hover {
    color: #42a5f5;
  }
}

// 频道选择控制
.channel-select-control {
  margin-top: 12px;
}

.channel-select-wrapper {
  position: relative;
}

.channel-select-trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 4px;
  background: rgba(50, 50, 50, 0.6);
  color: #fff;
  cursor: pointer;
  font-size: 13px;
  user-select: none;

  &:hover {
    border-color: rgba(66, 165, 245, 0.5);
    background: rgba(60, 60, 60, 0.8);
  }

  &.disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
}

.select-arrow {
  font-size: 10px;
  color: #888;
  margin-left: 8px;
}

.channel-select-dropdown {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  right: 0;
  max-height: 300px;
  overflow-y: auto;
  background: rgba(30, 30, 30, 0.98);
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 4px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.5);
  z-index: 1001;

  &::-webkit-scrollbar {
    width: 6px;
  }

  &::-webkit-scrollbar-track {
    background: rgba(0, 0, 0, 0.2);
    border-radius: 3px;
  }

  &::-webkit-scrollbar-thumb {
    background: rgba(255, 255, 255, 0.2);
    border-radius: 3px;
    
    &:hover {
      background: rgba(255, 255, 255, 0.3);
    }
  }
}

.channel-server-group {
  padding: 4px 0;
}

.channel-server-name {
  padding: 6px 12px;
  font-size: 12px;
  color: #ffc107;
  font-weight: 600;
  background: rgba(255, 193, 7, 0.1);
}

.channel-item {
  padding: 6px 24px;
  font-size: 13px;
  color: #ddd;
  cursor: pointer;

  &:hover {
    background: rgba(66, 165, 245, 0.2);
    color: #fff;
  }

  &.active {
    background: rgba(66, 165, 245, 0.3);
    color: #42a5f5;
    font-weight: 500;
  }
}

// 加速器选择控制
.accelerator-select-control {
  margin-top: 12px;
}

.accelerator-select-wrapper {
  position: relative;
}

.accelerator-select-trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 4px;
  background: rgba(50, 50, 50, 0.6);
  color: #fff;
  cursor: pointer;
  font-size: 13px;
  user-select: none;

  &:hover {
    border-color: rgba(66, 165, 245, 0.5);
    background: rgba(60, 60, 60, 0.8);
  }
}

.accelerator-select-dropdown {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  right: 0;
  max-height: 200px;
  overflow-y: auto;
  background: rgba(30, 30, 30, 0.98);
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 4px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.5);
  z-index: 1001;

  &::-webkit-scrollbar {
    width: 6px;
  }

  &::-webkit-scrollbar-track {
    background: rgba(0, 0, 0, 0.2);
    border-radius: 3px;
  }

  &::-webkit-scrollbar-thumb {
    background: rgba(255, 255, 255, 0.2);
    border-radius: 3px;
    
    &:hover {
      background: rgba(255, 255, 255, 0.3);
    }
  }
}

.accelerator-item {
  padding: 8px 12px;
  font-size: 13px;
  color: #ddd;
  cursor: pointer;

  &:hover {
    background: rgba(66, 165, 245, 0.2);
    color: #fff;
  }

  &.active {
    background: rgba(66, 165, 245, 0.3);
    color: #42a5f5;
    font-weight: 500;
  }
}

</style>
