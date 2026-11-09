<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import SvgIcon from '@jamescoyle/vue-icon'
import { mdiSwapHorizontal, mdiRefresh } from '@mdi/js'
import * as api from '../composables/useApi'
import { useAppStore } from '../stores/app'
import { BARTER_PRODUCTS, BARTER_STATIONS, type BarterProduct } from '../data/barter'

const inventory = ref<InventoryState>({ databasePath: '', rows: [], holders: [] })
const itemNames = ref<Record<number, string>>({})
const loading = ref(false)
const appStore = useAppStore()

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

function exchangeableCount(product: BarterProduct): number {
  return Math.min(product.weeklyLimit, ...product.materials.map(material =>
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
    <p class="barter-note">仅统计当前角色背包。可兑换数量按背包材料和每周上限计算；周刷新商品的第六级配方未包含在当前拆包中。</p>

    <section v-for="station in stations" :key="station.id" class="station-section">
      <h3>{{ station.name }}</h3>
      <div class="product-list">
        <article v-for="product in station.products" :key="product.id" class="product-row">
          <div class="product-heading">
            <strong>{{ product.level }}级货物</strong>
            <span class="exchange-count" :class="{ ready: product.exchangeable > 0 }">可兑换 {{ product.exchangeable }} / {{ product.weeklyLimit }}</span>
          </div>
          <div class="product-meta">
            <span>每箱 {{ product.boxCapacity }} 个</span>
            <span>单个重量 {{ product.weight }}</span>
          </div>
          <div class="materials">
            <div v-for="material in product.materials" :key="material.itemId" class="material-row">
              <span>{{ itemName(material.itemId) }}</span>
              <span :class="{ enough: (backpackCounts.get(material.itemId) || 0) >= material.amount * product.exchangeable && product.exchangeable > 0 }">
                {{ backpackCounts.get(material.itemId) || 0 }} / {{ material.amount }}
              </span>
            </div>
          </div>
        </article>
        <article class="product-row unavailable-row">
          <div class="product-heading"><strong>6级货物</strong><span>配方未收录</span></div>
          <p>R147 拆包的 CommerceCommon.xml 中没有该交易站的第六级换物配方。</p>
        </article>
      </div>
    </section>
  </div>
</template>

<style lang="scss" scoped>
.barter-view { padding: 6px; color: #ddd; }
.barter-heading { display: flex; align-items: center; justify-content: space-between; gap: 10px; flex-wrap: wrap; margin: 4px 0 6px; }
.heading-title, .heading-actions { display: flex; align-items: center; gap: 8px; }
.heading-title { color: #90caf9; }
h2 { color: #eee; font-size: 14px; font-weight: 600; }
.heading-actions { color: #999; font-size: 11px; }
.heading-actions button { display: grid; place-items: center; width: 28px; height: 28px; border: 1px solid rgba(255,255,255,.12); border-radius: 4px; color: #bbb; background: rgba(255,255,255,.04); cursor: pointer; }
.heading-actions button:hover { color: #fff; background: rgba(255,255,255,.1); }
.barter-note { margin: 0 0 12px; color: #888; font-size: 11px; line-height: 1.6; }
.station-section { margin-bottom: 16px; }
h3 { margin: 0 0 6px; padding: 0 0 5px; border-bottom: 1px solid rgba(255,255,255,.12); color: #a5d6a7; font-size: 13px; font-weight: 600; }
.product-list { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 280px), 1fr)); gap: 6px; }
.product-row { min-width: 0; padding: 8px; border: 1px solid rgba(255,255,255,.1); border-radius: 5px; background: rgba(35,35,35,.55); }
.product-heading { display: flex; justify-content: space-between; align-items: center; gap: 8px; font-size: 12px; }
.product-heading strong { color: #eee; }
.exchange-count { flex-shrink: 0; color: #aaa; font-size: 11px; font-variant-numeric: tabular-nums; }
.exchange-count.ready { color: #81c784; }
.product-meta { display: flex; gap: 12px; margin: 4px 0 6px; color: #777; font-size: 10px; }
.materials { display: grid; gap: 3px; }
.material-row { display: flex; justify-content: space-between; gap: 10px; color: #aaa; font-size: 11px; }
.material-row span:first-child { min-width: 0; overflow-wrap: anywhere; }
.material-row span:last-child { flex-shrink: 0; color: #888; font-family: Consolas, monospace; font-variant-numeric: tabular-nums; }
.material-row span.enough { color: #81c784; }
.unavailable-row { color: #777; }
.unavailable-row .product-heading span { color: #d6a65d; font-size: 11px; }
.unavailable-row p { margin: 5px 0 0; font-size: 10px; line-height: 1.5; }
</style>
