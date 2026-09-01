<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import SvgIcon from '@jamescoyle/vue-icon'
import { mdiChevronLeft, mdiChevronRight, mdiMagnify, mdiRefresh } from '@mdi/js'
import * as api from '../composables/useApi'

const state = ref<InventoryState>({ databasePath: '', rows: [], holders: [] })
const loading = ref(false)
const query = ref('')
const holderType = ref('all')
const container = ref('all')
const holderID = ref('all')
const page = ref(1)
const pageSize = 200
let inventoryLoadInFlight: Promise<void> | null = null
let inventoryRefreshQueued = false

const containerLabels: Record<string, string> = {
  backpack: '背包',
  bank: '银行',
  gacha: '抽蛋栏',
  dressing: '美容室',
  cash: '商城栏',
  currency: '货币保管盒',
}

function subjectType(row: InventoryRow): string {
  return row.holderType === 'account' ? 'character' : row.holderType
}

function subjectID(row: InventoryRow): string {
  return row.holderType === 'account' ? (row.scannedById || row.holderId) : row.holderId
}

function subjectName(row: InventoryRow): string {
  return row.holderType === 'account' ? (row.scannedByName || '未记录角色') : (row.holderName || row.holderId)
}

const holderOptions = computed(() => {
  const seen = new Map<string, { id: string; label: string }>()
  for (const row of state.value.rows || []) {
    if (holderType.value !== 'all' && subjectType(row) !== holderType.value) continue
    const id = subjectID(row)
    if (!seen.has(id)) {
      seen.set(id, { id, label: subjectName(row) })
    }
  }
  return [...seen.values()].sort((a, b) => a.label.localeCompare(b.label, 'zh-CN'))
})

const filteredRows = computed(() => {
  // Read query directly here so every input change invalidates the complete
  // filtering result, including after an earlier multi-keyword search.
  const queryTerms = query.value
    .split(/[\s,，]+/)
    .map(term => term.trim().toLowerCase())
    .filter(Boolean)
    .slice(0, 5)

  return (state.value.rows || []).filter((row) => {
    if (holderType.value !== 'all' && subjectType(row) !== holderType.value) return false
    if (container.value !== 'all' && row.container !== container.value) return false
    if (holderID.value !== 'all' && subjectID(row) !== holderID.value) return false
    if (queryTerms.length === 0) return true
    const searchable = `${row.name} ${row.itemId}`.toLowerCase()
    return queryTerms.every(term => searchable.includes(term))
  })
})

const pageCount = computed(() => Math.max(1, Math.ceil(filteredRows.value.length / pageSize)))
const visibleRows = computed(() => {
  const start = (page.value - 1) * pageSize
  return filteredRows.value.slice(start, start + pageSize)
})

watch([query, holderType, container, holderID], () => {
  page.value = 1
})
watch(holderOptions, (options) => {
  if (holderID.value !== 'all' && !options.some(option => option.id === holderID.value)) {
    holderID.value = 'all'
  }
})

function holderTypeLabel(type: string): string {
  if (type === 'pet') return '宠物'
  if (type === 'character') return '角色'
  return '账号共享'
}

function formatTime(value: number): string {
  if (!value) return '--'
  return new Date(value).toLocaleString([], {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

async function loadInventory() {
	if (inventoryLoadInFlight) {
		inventoryRefreshQueued = true
		return inventoryLoadInFlight
	}
	loading.value = true
	const load = (async () => {
		try {
			state.value = await api.getInventoryState()
			if (page.value > pageCount.value) page.value = pageCount.value
		} catch (error) {
			console.error('加载背包数据失败:', error)
		} finally {
			loading.value = false
			inventoryLoadInFlight = null
			if (inventoryRefreshQueued) {
				inventoryRefreshQueued = false
				void loadInventory()
			}
		}
	})()
	inventoryLoadInFlight = load
	return load
}

function handleInventoryUpdated() {
  void loadInventory()
}

onMounted(() => {
  void loadInventory()
  api.onEvent('inventory-updated', handleInventoryUpdated)
})

onUnmounted(() => {
  api.offEvent('inventory-updated', handleInventoryUpdated)
})
</script>

<template>
  <div class="inventory-view">
    <header class="toolbar">
      <div class="search-control">
        <svg-icon type="mdi" :path="mdiMagnify" :size="16" />
        <input v-model="query" type="search" placeholder="搜索物品名称或 ID（空格或逗号分隔）" aria-label="搜索物品" />
      </div>

      <select v-model="holderType" aria-label="主体类型">
        <option value="all">全部主体</option>
        <option value="character">角色</option>
        <option value="pet">宠物</option>
      </select>

      <select v-model="container" aria-label="存放位置">
        <option value="all">全部位置</option>
        <option value="backpack">背包</option>
        <option value="bank">银行</option>
        <option value="gacha">抽蛋栏</option>
        <option value="dressing">美容室</option>
        <option value="cash">商城栏</option>
        <option value="currency">货币保管盒</option>
      </select>

      <select v-model="holderID" aria-label="角色或宠物">
        <option value="all">全部角色/宠物</option>
        <option v-for="option in holderOptions" :key="option.id" :value="option.id">
          {{ option.label }}
        </option>
      </select>

      <button type="button" class="icon-button" title="刷新" :disabled="loading" @click="loadInventory">
        <svg-icon type="mdi" :path="mdiRefresh" :size="17" />
      </button>
    </header>

    <div class="list-header inventory-grid" role="row">
      <span>物品</span>
      <span>数量</span>
      <span>角色/宠物</span>
      <span>归属角色</span>
      <span>存放位置</span>
      <span>更新时间</span>
    </div>

    <div class="list-body" role="table" aria-label="背包物品列表">
      <div v-if="!loading && visibleRows.length === 0" class="empty-state">暂无物品数据</div>
      <div v-for="row in visibleRows" :key="`${row.itemId}-${row.name}-${row.pocketType}-${row.holderId}-${row.container}-${row.subLocation || ''}`" class="inventory-row inventory-grid" role="row">
        <div class="item-cell">
          <span class="item-name">
            <template v-if="row.enchantName">
              <span class="enchant-name">{{ row.enchantName }}</span>{{ row.name.slice(row.enchantName.length) }}
            </template>
            <template v-else>{{ row.name }}</template>
          </span>
          <span class="item-id">#{{ row.itemId }}</span>
        </div>
        <span class="amount">{{ row.amount.toLocaleString() }}</span>
        <div class="holder-cell">
          <span>{{ subjectName(row) }}</span>
          <span class="secondary">{{ row.holderType === 'account' ? '当前角色' : holderTypeLabel(row.holderType) }}</span>
        </div>
        <span>{{ row.holderType === 'pet' ? (row.ownerCharacterName || '未记录') : '--' }}</span>
        <div class="location-cell">
          <span>{{ containerLabels[row.container] || row.container }}</span>
          <span v-if="row.subLocation" class="secondary">{{ row.subLocation }}</span>
        </div>
        <span class="updated-at">{{ formatTime(row.updatedAt) }}</span>
      </div>
    </div>

    <footer class="list-footer">
      <span>共 {{ filteredRows.length.toLocaleString() }} 项</span>
      <div class="pager">
        <button type="button" class="icon-button" title="上一页" :disabled="page <= 1" @click="page--">
          <svg-icon type="mdi" :path="mdiChevronLeft" :size="17" />
        </button>
        <span>{{ page }} / {{ pageCount }}</span>
        <button type="button" class="icon-button" title="下一页" :disabled="page >= pageCount" @click="page++">
          <svg-icon type="mdi" :path="mdiChevronRight" :size="17" />
        </button>
      </div>
    </footer>
  </div>
</template>

<style lang="scss" scoped>
.inventory-view {
  width: 100%;
  height: 100%;
  min-height: 0;
  min-width: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-sizing: border-box;
  color: #ddd;
}

.toolbar {
  min-height: 38px;
  padding: 3px 4px 7px;
  display: flex;
  align-items: center;
  gap: 6px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

.search-control {
  width: min(280px, 32%);
  height: 28px;
  padding: 0 8px;
  display: flex;
  align-items: center;
  gap: 6px;
  color: #888;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 4px;

  input {
    width: 100%;
    min-width: 0;
    border: 0;
    outline: 0;
    color: #eee;
    background: transparent;
    font: inherit;
    font-size: 12px;
    letter-spacing: 0;
  }
}

select {
  height: 28px;
  min-width: 100px;
  max-width: 170px;
  padding: 0 24px 0 7px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 4px;
  color: #d8d8d8;
  background: #292929;
  font: inherit;
  font-size: 11px;
  letter-spacing: 0;
}

.icon-button {
  width: 28px;
  height: 28px;
  flex: 0 0 28px;
  padding: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 4px;
  color: #bbb;
  background: rgba(255, 255, 255, 0.05);
  cursor: pointer;

  &:hover:not(:disabled) {
    color: #fff;
    background: rgba(255, 255, 255, 0.1);
  }

  &:disabled {
    opacity: 0.35;
    cursor: default;
  }
}

.inventory-grid {
  display: grid;
  grid-template-columns: minmax(220px, 2fr) minmax(64px, 0.5fr) minmax(120px, 1fr) minmax(110px, 0.9fr) minmax(130px, 1fr) minmax(110px, 0.8fr);
  align-items: center;
  column-gap: 12px;
}

.list-header {
  min-height: 31px;
  padding: 0 10px;
  flex: 0 0 31px;
  color: #888;
  background: rgba(255, 255, 255, 0.025);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  font-size: 11px;
}

.list-body {
  flex: 1;
  min-height: 0;
  min-width: 0;
  overflow-x: hidden;
  overflow-y: auto;
}

.inventory-grid > * {
  min-width: 0;
}

.inventory-row {
  min-height: 45px;
  padding: 5px 10px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.055);
  font-size: 12px;

  &:hover {
    background: rgba(66, 165, 245, 0.07);
  }
}

.item-cell,
.holder-cell,
.location-cell {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.item-name {
  overflow: hidden;
  color: #f0f0f0;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.enchant-name {
  color: #52b9ff;
}

.item-id,
.secondary,
.updated-at {
  color: #858585;
  font-size: 10px;
}

.amount {
  color: #90caf9;
  font-variant-numeric: tabular-nums;
}

.empty-state {
  height: 100%;
  min-height: 120px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #777;
  font-size: 12px;
}

.list-footer {
  min-height: 35px;
  padding: 3px 7px 3px 10px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: #888;
  border-top: 1px solid rgba(255, 255, 255, 0.1);
  font-size: 11px;
}

.pager {
  display: flex;
  align-items: center;
  gap: 8px;
}

@media (max-width: 850px) {
  .toolbar {
    flex-wrap: wrap;
  }

  .search-control {
    width: 100%;
  }

  .inventory-grid {
    grid-template-columns: minmax(130px, 2fr) minmax(48px, 0.5fr) minmax(90px, 1fr) minmax(90px, 0.9fr);
    column-gap: 6px;
  }

  .inventory-grid > :nth-child(4),
  .inventory-grid > :nth-child(6) {
    display: none;
  }
}
</style>
