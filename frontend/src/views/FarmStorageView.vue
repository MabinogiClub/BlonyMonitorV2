<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import SvgIcon from '@jamescoyle/vue-icon'
import {
  mdiClockOutline,
  mdiDatabaseSyncOutline,
  mdiMagnify,
  mdiPackageVariantClosed,
} from '@mdi/js'
import * as api from '../composables/useApi'

type StorageCategory = 'all' | FarmStorageItem['category']

const emptyState = (): FarmRecommendationState => ({
  goal: 'keys',
  useNormalMaterials: true,
  useAdvancedMaterials: false,
  useHighestMaterials: false,
  dataReady: false,
  ordersSynced: false,
  inventorySynced: false,
  recipesSynced: false,
  storageLevel: 0,
  storageUsed: 0,
  storageCapacity: 0,
  storageItems: [],
  captureEnabled: false,
  statusMessage: '正在读取本地储存库',
  plantingSuggestions: [],
  plantingProductions: [],
  plantingReferenceCount: 0,
  recommendations: [],
})

const iconModules = import.meta.glob('../assets/farm-items/*.png', {
  eager: true,
  query: '?url',
  import: 'default',
}) as Record<string, string>

const storageState = ref<FarmRecommendationState>(emptyState())
const loading = ref(true)
const search = ref('')
const activeCategory = ref<StorageCategory>('all')
let updateInterval: number | null = null

const categories: Array<{ value: StorageCategory; label: string }> = [
  { value: 'all', label: '全部' },
  { value: 'crop', label: '作物' },
  { value: 'product', label: '制成品' },
  { value: 'upgrade', label: '扩建材料' },
]

const filteredItems = computed(() => {
  const keyword = search.value.trim().toLocaleLowerCase()
  return storageState.value.storageItems.filter((item) => {
    if (activeCategory.value !== 'all' && item.category !== activeCategory.value) return false
    return !keyword || item.name.toLocaleLowerCase().includes(keyword) || String(item.itemId).includes(keyword)
  })
})

const emptySlotCount = computed(() => Math.max(0, 42 - filteredItems.value.length))

const capacityPercent = computed(() => {
  const capacity = storageState.value.storageCapacity
  if (capacity <= 0) return 0
  return Math.min(100, (storageState.value.storageUsed / capacity) * 100)
})

function itemIcon(item: FarmStorageItem): string {
  return iconModules[`../assets/farm-items/${item.icon}.png`] || ''
}

function qualityLabel(quality: FarmStorageItem['quality']): string {
  if (quality === 'advanced') return '高'
  if (quality === 'highest') return '最'
  return ''
}

function formatSyncTime(timestamp?: number): string {
  if (!timestamp) return '尚未同步'
  return new Date(timestamp).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

async function loadStorage(silent = false) {
  if (!silent) loading.value = true
  try {
    storageState.value = await api.getFarmRecommendationState()
  } catch (error) {
    console.error('加载农场储存库失败:', error)
    storageState.value.statusMessage = '读取本地储存库失败'
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await loadStorage()
  updateInterval = window.setInterval(() => loadStorage(true), 1000)
})

onUnmounted(() => {
  if (updateInterval !== null) clearInterval(updateInterval)
})
</script>

<template>
  <div class="storage-view">
    <header class="storage-header">
      <div class="storage-heading">
        <svg-icon type="mdi" :path="mdiPackageVariantClosed" :size="18" />
        <div>
          <strong>储存库</strong>
          <span v-if="storageState.storageLevel">Lv.{{ storageState.storageLevel }}</span>
        </div>
      </div>

      <div class="capacity-block" :title="`已存放 ${storageState.storageUsed} 件物品`">
        <div class="capacity-value">
          <strong>{{ storageState.storageUsed }}</strong>
          <span>/ {{ storageState.storageCapacity || '--' }}</span>
        </div>
        <div class="capacity-track" aria-hidden="true">
          <div class="capacity-fill" :style="{ width: `${capacityPercent}%` }" />
        </div>
      </div>
    </header>

    <div class="storage-controls">
      <div class="category-control" role="tablist" aria-label="储存库分类">
        <button
          v-for="category in categories"
          :key="category.value"
          type="button"
          role="tab"
          :aria-selected="activeCategory === category.value"
          :class="{ active: activeCategory === category.value }"
          @click="activeCategory = category.value"
        >
          {{ category.label }}
        </button>
      </div>

      <label class="storage-search">
        <svg-icon type="mdi" :path="mdiMagnify" :size="14" />
        <input v-model="search" type="search" placeholder="搜索物品" aria-label="搜索储存库物品">
      </label>
    </div>

    <div v-if="loading" class="storage-empty">
      <svg-icon type="mdi" :path="mdiDatabaseSyncOutline" :size="26" />
      <span>正在读取本地储存库</span>
    </div>

    <div v-else-if="!storageState.inventorySynced" class="storage-empty">
      <svg-icon type="mdi" :path="mdiPackageVariantClosed" :size="28" />
      <strong>尚未同步储存库</strong>
      <span>{{ storageState.statusMessage }}</span>
    </div>

    <div v-else class="storage-grid-wrap">
      <div class="storage-grid" role="list" aria-label="储存库物品">
        <div
          v-for="item in filteredItems"
          :key="item.itemId"
          role="listitem"
          class="storage-slot occupied"
          :class="`quality-${item.quality}`"
          :title="`${item.name} x${item.count}`"
        >
          <span class="item-count">{{ item.count }}</span>
          <span v-if="qualityLabel(item.quality)" class="quality-mark">{{ qualityLabel(item.quality) }}</span>
          <img v-if="itemIcon(item)" :src="itemIcon(item)" :alt="item.name" draggable="false">
        </div>
        <div v-for="index in emptySlotCount" :key="`empty-${index}`" class="storage-slot" aria-hidden="true" />
      </div>
      <div v-if="filteredItems.length === 0" class="filter-empty">没有匹配的物品</div>
    </div>

    <footer class="storage-footer">
      <span>
        <svg-icon type="mdi" :path="mdiClockOutline" :size="12" />
        {{ formatSyncTime(storageState.syncedAt) }}
      </span>
      <span>共 {{ storageState.storageItems.length }} 种</span>
    </footer>
  </div>
</template>

<style scoped>
.storage-view {
  min-height: 100%;
  display: flex;
  flex-direction: column;
  gap: 8px;
  color: #e8e8e8;
}

.storage-header {
  min-height: 46px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 0 7px 7px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

.storage-heading {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 7px;
}

.storage-heading > svg {
  flex-shrink: 0;
  color: #a7c88d;
}

.storage-heading > div {
  display: flex;
  align-items: baseline;
  gap: 6px;
}

.storage-heading strong {
  font-size: 13px;
}

.storage-heading span {
  color: #8f9a87;
  font-family: Consolas, monospace;
  font-size: 9px;
}

.capacity-block {
  width: min(150px, 44%);
}

.capacity-value {
  display: flex;
  align-items: baseline;
  justify-content: flex-end;
  gap: 4px;
  font-family: Consolas, monospace;
}

.capacity-value strong {
  color: #fff;
  font-size: 14px;
}

.capacity-value span {
  color: #a8a8a8;
  font-size: 10px;
}

.capacity-track {
  height: 3px;
  margin-top: 4px;
  overflow: hidden;
  background: rgba(255, 255, 255, 0.08);
}

.capacity-fill {
  height: 100%;
  background: #8cab70;
  transition: width 0.25s ease;
}

.storage-controls {
  min-height: 30px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.category-control {
  min-width: 0;
  display: flex;
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.11);
  border-radius: 4px;
}

.category-control button {
  height: 27px;
  padding: 0 8px;
  border: 0;
  color: #8f8f8f;
  background: rgba(255, 255, 255, 0.025);
  font-family: inherit;
  font-size: 9px;
  cursor: pointer;
}

.category-control button + button {
  border-left: 1px solid rgba(255, 255, 255, 0.08);
}

.category-control button:hover,
.category-control button.active {
  color: #e6e6e6;
  background: rgba(151, 181, 119, 0.14);
}

.storage-search {
  width: min(134px, 38%);
  height: 28px;
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 0 7px;
  color: #777;
  background: rgba(0, 0, 0, 0.18);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 4px;
}

.storage-search input {
  width: 100%;
  min-width: 0;
  border: 0;
  outline: 0;
  color: #ddd;
  background: transparent;
  font-family: inherit;
  font-size: 9px;
}

.storage-search input::placeholder {
  color: #676767;
}

.storage-grid-wrap {
  position: relative;
  min-height: 0;
  flex: 1;
  overflow: auto;
  padding: 5px;
  background: #171914;
  border: 1px solid #41463a;
  box-shadow: inset 0 0 0 1px #090a08;
}

.storage-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(52px, 1fr));
  grid-auto-rows: minmax(52px, 1fr);
  gap: 3px;
}

.storage-slot {
  position: relative;
  min-width: 0;
  aspect-ratio: 1;
  overflow: hidden;
  background: #20251c;
  border: 1px solid #46503f;
  box-shadow: inset 0 0 0 1px #0c0e0b;
}

.storage-slot.occupied {
  background: #252b21;
}

.storage-slot.quality-normal {
  border-color: #58774b;
}

.storage-slot.quality-advanced {
  border-color: #4d7e9e;
  background: #202a2c;
}

.storage-slot.quality-highest {
  border-color: #b18a4d;
  background: #2c2920;
}

.storage-slot img {
  width: 100%;
  height: 100%;
  display: block;
  object-fit: contain;
  object-position: center;
  image-rendering: auto;
  user-select: none;
}

.item-count,
.quality-mark {
  position: absolute;
  z-index: 1;
  color: #fff;
  font-family: Consolas, monospace;
  line-height: 1;
  text-shadow: -1px 0 #000, 0 1px #000, 1px 0 #000, 0 -1px #000;
}

.item-count {
  top: 3px;
  left: 4px;
  font-size: 12px;
  font-weight: 700;
}

.quality-mark {
  top: 3px;
  right: 4px;
  color: #d7e8f5;
  font-size: 8px;
}

.quality-highest .quality-mark {
  color: #ffe0a3;
}

.storage-empty {
  min-height: 220px;
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 7px;
  color: #747b70;
  text-align: center;
  font-size: 10px;
}

.storage-empty strong {
  color: #b4b8b0;
  font-size: 12px;
}

.storage-empty span {
  max-width: 260px;
  line-height: 1.5;
}

.filter-empty {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #70756c;
  font-size: 10px;
  pointer-events: none;
}

.storage-footer {
  min-height: 20px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 3px;
  color: #777;
  font-size: 9px;
}

.storage-footer span {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

@media (max-width: 480px) {
  .category-control button {
    padding: 0 6px;
  }

  .storage-search {
    width: 104px;
  }

  .storage-grid {
    grid-template-columns: repeat(auto-fill, minmax(46px, 1fr));
    grid-auto-rows: minmax(46px, 1fr);
  }
}
</style>
