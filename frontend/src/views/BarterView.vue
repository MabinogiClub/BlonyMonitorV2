<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import SvgIcon from '@jamescoyle/vue-icon'
import { mdiSwapHorizontal, mdiRefresh } from '@mdi/js'
import * as api from '../composables/useApi'
import { useAppStore } from '../stores/app'
import { BARTER_PRODUCTS, BARTER_STATIONS, type BarterProduct } from '../data/barter'
import { getBarterProductImage, getTradeVehicles } from '../data/smuggler'
import { calculateBarterLoadout } from '../utils/barterLoadout'

const inventory = ref<InventoryState>({ databasePath: '', rows: [], holders: [] })
const itemNames = ref<Record<number, string>>({})
const loading = ref(false)
const appStore = useAppStore()
const vehicles = getTradeVehicles()
const selectedVehicleName = ref('[伙伴]高配版浮游船')

const backpackCounts = computed(() => {
  const counts = new Map<number, number>()
  const characterId = appStore.selfInfo?.id
  if (!characterId) return counts
  for (const row of inventory.value.rows) {
    if (row.container !== 'backpack' || row.holderType !== 'character' || row.holderId !== characterId) continue
    counts.set(row.itemId, (counts.get(row.itemId) || 0) + row.amount)
  }
  return counts
})

const stations = computed(() => BARTER_STATIONS.map(station => ({
  ...station,
  products: BARTER_PRODUCTS.filter(product => product.postId === station.id).map(product => ({
    ...product,
    exchangeable: exchangeableCount(product),
  })),
})))

const selectedVehicle = computed(() => vehicles.find(vehicle => vehicle.name === selectedVehicleName.value) || vehicles[0])
const loadout = computed(() => calculateBarterLoadout(
  BARTER_PRODUCTS,
  new Map(BARTER_PRODUCTS.map(product => [product.id, exchangeableCount(product)])),
  selectedVehicle.value,
))

function exchangeableCount(product: BarterProduct): number {
  return Math.min(...product.materials.map(material =>
    Math.floor((backpackCounts.value.get(material.itemId) || 0) / material.amount),
  ))
}

function itemName(id: number): string {
  return itemNames.value[id] || `物品 ${id}`
}

async function load() {
  loading.value = true
  try {
    inventory.value = await api.getInventoryState()
    const ids = [...new Set(BARTER_PRODUCTS.flatMap(product => product.materials.map(material => material.itemId)))]
    itemNames.value = await api.getItemDisplayNames(ids)
  } catch (error) {
    console.error('加载以物换物数据失败:', error)
  } finally {
    loading.value = false
  }
}

function handleInventoryUpdated() {
  void load()
}

onMounted(() => {
  void load()
  api.onEvent('inventory-updated', handleInventoryUpdated)
})
onUnmounted(() => api.offEvent('inventory-updated', handleInventoryUpdated))
</script>

<template>
  <div class="barter-view">
    <header class="barter-heading">
      <div class="heading-title">
        <svg-icon type="mdi" :path="mdiSwapHorizontal" :size="19" />
        <h2>以物换物</h2>
      </div>
      <div class="heading-actions">
        <span v-if="appStore.selfInfo">当前角色：{{ appStore.selfInfo.name }}</span>
        <span v-else>等待角色信息</span>
        <button type="button" title="刷新背包数据" aria-label="刷新背包数据" :disabled="loading" @click="load">
          <svg-icon type="mdi" :path="mdiRefresh" :size="17" />
        </button>
      </div>
    </header>
    <p class="barter-note">仅统计当前角色背包 · 6级周刷配方待补</p>

    <div class="station-grid">
      <section v-for="station in stations" :key="station.id" class="station-section">
        <h3>{{ station.name }}</h3>
        <div class="product-list">
          <article v-for="product in station.products" :key="product.id" class="product-row">
            <header class="product-heading">
              <img class="barter-product-image" :src="getBarterProductImage(product)" :alt="product.name" />
              <strong :title="product.name">{{ product.name }}</strong>
              <span class="product-level">{{ product.level }}级</span>
            </header>
            <div class="product-summary">
              <span class="exchange-count" :class="{ ready: product.exchangeable > 0 }">可兑 {{ product.exchangeable }}/{{ product.weeklyLimit }}</span>
              <span>箱 {{ product.boxCapacity }} · 重 {{ product.weight }}</span>
            </div>
            <div class="materials">
              <div v-for="material in product.materials" :key="material.itemId" class="material-row" :title="`${itemName(material.itemId)}：背包 ${backpackCounts.get(material.itemId) || 0}，每次需要 ${material.amount}`">
                <span>{{ itemName(material.itemId) }}</span>
                <span :class="{ enough: (backpackCounts.get(material.itemId) || 0) >= material.amount * product.exchangeable && product.exchangeable > 0 }">
                  {{ backpackCounts.get(material.itemId) || 0 }}/{{ material.amount }}
                </span>
              </div>
            </div>
          </article>
        </div>
      </section>
    </div>

    <section class="loadout-section">
      <header class="loadout-header">
        <div>
          <h2>载具装载配置</h2>
          <p>按可兑换数量、货物等级、载具格数与载重计算</p>
        </div>
        <label class="vehicle-select">
          <span>载具</span>
          <select v-model="selectedVehicleName" aria-label="选择贸易载具">
            <option v-for="vehicle in vehicles" :key="vehicle.name" :value="vehicle.name">{{ vehicle.name }}</option>
          </select>
        </label>
      </header>
      <div class="loadout-summary">
        <span>{{ loadout.slotsUsed }}/{{ selectedVehicle.block }} 格</span>
        <span>{{ loadout.totalWeight }}/{{ selectedVehicle.weight }} 负重</span>
        <strong>估算收益 {{ loadout.estimatedValue.toLocaleString() }}</strong>
      </div>
      <div class="cargo-grid" :style="{ '--slot-count': selectedVehicle.block }">
        <article v-for="slot in selectedVehicle.block" :key="slot" class="cargo-slot" :class="{ empty: !loadout.items[slot - 1] }">
          <template v-if="loadout.items[slot - 1]">
            <img :src="getBarterProductImage(loadout.items[slot - 1])" :alt="loadout.items[slot - 1].name" />
            <div class="cargo-info">
              <strong :title="loadout.items[slot - 1].name">{{ loadout.items[slot - 1].name }}</strong>
              <span>{{ loadout.items[slot - 1].quantity }} 个 · {{ loadout.items[slot - 1].level }}级</span>
            </div>
            <b>{{ loadout.items[slot - 1].estimatedValue.toLocaleString() }}</b>
          </template>
          <span v-else class="empty-label">空格 {{ slot }}</span>
        </article>
      </div>
    </section>
  </div>
</template>

<style lang="scss" scoped>
.barter-view { padding: 4px; color: #ddd; }
.barter-heading { display: flex; align-items: center; justify-content: space-between; gap: 8px; flex-wrap: wrap; margin: 2px 0 4px; }
.heading-title, .heading-actions { display: flex; align-items: center; gap: 8px; }
.heading-title { color: #90caf9; }
h2 { color: #eee; font-size: 14px; font-weight: 600; }
.heading-actions { color: #999; font-size: 10px; }
.heading-actions button { display: grid; place-items: center; width: 24px; height: 24px; border: 1px solid rgba(255,255,255,.12); border-radius: 4px; color: #bbb; background: rgba(255,255,255,.04); cursor: pointer; }
.heading-actions button:hover { color: #fff; background: rgba(255,255,255,.1); }
.barter-note { margin: 0 0 7px; color: #888; font-size: 10px; line-height: 1.4; }
.station-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; align-items: start; }
.station-section { min-width: 0; }
h3 { margin: 0 0 4px; padding: 0 0 4px; border-bottom: 1px solid rgba(255,255,255,.12); color: #a5d6a7; font-size: 11px; font-weight: 600; }
.product-list { display: grid; gap: 3px; }
.product-row { min-width: 0; padding: 4px 5px; border: 1px solid rgba(255,255,255,.08); border-radius: 3px; background: rgba(35,35,35,.45); }
.product-heading { display: flex; align-items: baseline; gap: 4px; min-width: 0; font-size: 10px; line-height: 1.3; }
.barter-product-image { width: 25px; height: 25px; flex: 0 0 25px; align-self: center; object-fit: contain; image-rendering: pixelated; }
.product-heading strong { min-width: 0; overflow: hidden; color: #eee; font-size: 11px; font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.product-level { flex-shrink: 0; color: #777; font-size: 9px; }
.product-summary { display: flex; justify-content: space-between; gap: 4px; margin: 2px 0; color: #777; font-size: 9px; white-space: nowrap; }
.exchange-count { flex-shrink: 0; color: #aaa; font-size: 9px; font-variant-numeric: tabular-nums; }
.exchange-count.ready { color: #81c784; }
.materials { display: grid; gap: 1px; }
.material-row { display: flex; justify-content: space-between; gap: 4px; color: #999; font-size: 9px; line-height: 1.25; }
.material-row span:first-child { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.material-row span:last-child { flex-shrink: 0; color: #888; font-family: Consolas, monospace; font-variant-numeric: tabular-nums; }
.material-row span.enough { color: #81c784; }
.loadout-section { margin-top: 14px; padding-top: 10px; border-top: 1px solid rgba(255,255,255,.12); }
.loadout-header { display: flex; align-items: center; justify-content: space-between; gap: 10px; flex-wrap: wrap; }
.loadout-header h2 { margin: 0; color: #eee; font-size: 13px; font-weight: 600; }
.loadout-header p { margin: 3px 0 0; color: #888; font-size: 10px; }
.vehicle-select { display: flex; align-items: center; gap: 6px; color: #999; font-size: 10px; }
.vehicle-select select { max-width: min(260px, 65vw); padding: 5px 7px; border: 1px solid rgba(255,255,255,.14); border-radius: 4px; color: #ddd; background: #292929; font: inherit; }
.loadout-summary { display: flex; align-items: center; gap: 12px; margin: 8px 0 6px; color: #aaa; font-size: 10px; font-variant-numeric: tabular-nums; }
.loadout-summary strong { margin-left: auto; color: #a5d6a7; font-weight: 600; }
.cargo-grid { display: grid; grid-template-columns: repeat(var(--slot-count), minmax(0, 1fr)); gap: 4px; }
.cargo-slot { display: flex; align-items: center; gap: 5px; min-width: 0; min-height: 50px; padding: 5px; border: 1px solid rgba(255,255,255,.11); border-radius: 3px; background: rgba(35,35,35,.55); }
.cargo-slot img { width: 30px; height: 30px; flex: 0 0 30px; object-fit: contain; image-rendering: pixelated; }
.cargo-info { display: grid; min-width: 0; gap: 2px; }
.cargo-info strong { overflow: hidden; color: #eee; font-size: 9px; font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.cargo-info span { color: #888; font-size: 9px; white-space: nowrap; }
.cargo-slot b { margin-left: auto; color: #a5d6a7; font-size: 9px; font-variant-numeric: tabular-nums; }
.cargo-slot.empty { justify-content: center; border-style: dashed; background: transparent; }
.empty-label { color: #666; font-size: 9px; }
@media (max-width: 520px) { .station-grid { grid-template-columns: 1fr; } }
@media (max-width: 760px) { .cargo-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
