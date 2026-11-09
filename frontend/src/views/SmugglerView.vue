<script setup lang="ts">
import { computed } from 'vue'
import SvgIcon from '@jamescoyle/vue-icon'
import { mdiMapMarkerOutline, mdiPackageVariantClosed, mdiTruckOutline } from '@mdi/js'
import { useSmugglerStore } from '../stores/smuggler'
import { getSmugglerAreaImage, getSmugglerProduct, getSmugglerProductImage, getSmugglerVehicles } from '../data/smuggler'

const smuggler = useSmugglerStore()
const areaImage = computed(() => getSmugglerAreaImage(smuggler.currentNotice?.area || ''))
const product = computed(() => getSmugglerProduct(smuggler.currentNotice?.item || ''))
const productImage = computed(() => product.value ? getSmugglerProductImage(product.value) : undefined)
const vehicles = computed(() => product.value ? getSmugglerVehicles(product.value) : [])
const levelColors = ['#888', '#a5d6a7', '#90caf9', '#ce93d8', '#ffab91', '#ffd54f']
</script>

<template>
  <div class="smuggler-view">
    <section class="smuggler-card location-card" aria-label="走私商人位置">
      <header class="card-header">
        <svg-icon type="mdi" :path="mdiMapMarkerOutline" :size="17" />
        <h2>走私商人位置</h2>
        <span class="eirin-clock">爱琳 {{ smuggler.eirinTimeLabel }}</span>
      </header>
      <div v-if="smuggler.currentNotice" class="location-content">
        <p class="location-name">{{ smuggler.currentNotice.area }}</p>
        <img v-if="areaImage" class="area-image" :src="areaImage" :alt="`${smuggler.currentNotice.area}的走私商人位置地图`" />
        <p v-else class="muted map-unavailable">暂无该地点的地图</p>
      </div>
      <div v-else class="empty-content">
        <p>{{ smuggler.cycle.phase === 'absent' ? '走私商人尚未出现' : '等待本次走私商人的游戏提示' }}</p>
        <span>收到提示后，将在这里显示地点和地图</span>
      </div>
    </section>

    <div class="smuggler-status" :class="smuggler.cycle.phase" role="status" aria-live="off">
      <strong><span class="status-dot"></span>{{ smuggler.statusLabel }}</strong>
      <span>{{ smuggler.countdown }}</span>
    </div>

    <section class="smuggler-card" aria-label="贸易物品">
      <header class="card-header">
        <svg-icon type="mdi" :path="mdiPackageVariantClosed" :size="17" />
        <h2>贸易物品</h2>
      </header>
      <div v-if="product" class="product-content">
        <div class="product-image">
          <img v-if="productImage" :src="productImage" :alt="product.name" />
          <svg-icon v-else type="mdi" :path="mdiPackageVariantClosed" :size="30" />
        </div>
        <div class="product-details">
          <h3>{{ product.name }}</h3>
          <p class="muted">产地：<span class="origin">{{ product.area }}</span></p>
          <span class="product-level" :style="{ color: levelColors[product.level] }">{{ '★'.repeat(product.level) }} {{ product.level }}级货物</span>
          <div class="product-stats">
            <span>单价 <b>{{ product.price.toLocaleString() }} G</b></span>
            <span>重量 <b>{{ product.weight }}</b></span>
            <span>每槽 <b>{{ product.boxCount }} 个</b></span>
          </div>
        </div>
      </div>
      <div v-else class="empty-content">
        <p>{{ smuggler.currentNotice?.item || '尚未收到本次贸易物品信息' }}</p>
        <span v-if="smuggler.currentNotice?.item">暂无该物品的本地资料</span>
      </div>
    </section>

    <section v-if="vehicles.length" class="smuggler-card" aria-label="载具推荐">
      <header class="card-header">
        <svg-icon type="mdi" :path="mdiTruckOutline" :size="17" />
        <h2>载具推荐</h2>
        <span class="calculation-label">本地计算</span>
      </header>
      <div class="vehicle-table-wrapper">
        <table class="vehicle-table">
          <thead><tr><th scope="col">载具</th><th scope="col">槽位</th><th scope="col">可用槽</th><th scope="col">装载数</th><th scope="col">总花费</th></tr></thead>
          <tbody>
            <tr v-for="(vehicle, index) in vehicles" :key="vehicle.name" :class="{ optimal: index === 0 }">
              <th scope="row"><span>{{ vehicle.name }}</span><span v-if="index === 0" class="optimal-badge">推荐</span></th>
              <td>{{ vehicle.block }}</td><td>{{ vehicle.actualSlots }}</td><td>{{ vehicle.totalItems }}</td>
              <td class="total-cost">{{ vehicle.totalCost.toLocaleString() }} G</td>
            </tr>
          </tbody>
        </table>
      </div>
      <p class="calculation-hint">按载重与槽位计算，优先装载量，装载量相同时优先速度。</p>
    </section>

    <p class="capture-hint">需先转生并选择生产类，才能收到走私提示；收到后会自动更新地点和货物。</p>
    <p class="capture-hint">爱琳 19:00 预告 · 22:00 出现 · 03:00 即将消失 · 06:00 消失。倒计时为现实分秒，校准与首页同步。</p>
  </div>
</template>

<style lang="scss" scoped>
.smuggler-view {
  padding: 6px;
}

.smuggler-card {
  margin-bottom: 12px;
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 6px;
  background: rgba(35, 35, 35, 0.6);
}

.card-header {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 9px 12px;
  color: #90caf9;
  background: rgba(255, 255, 255, 0.025);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);

  h2 { font-size: 12px; font-weight: 600; }
}

.eirin-clock, .calculation-label {
  margin-left: auto;
  color: #888;
  font-size: 10px;
}

.location-content { padding: 10px; }
.location-name { margin-bottom: 8px; color: #ddd; font-size: 13px; font-weight: 600; }
.area-image { display: block; width: 100%; border-radius: 4px; }
.map-unavailable { padding: 12px 0; }
.muted { color: #999; font-size: 11px; }

.empty-content {
  padding: 24px 12px;
  text-align: center;
  color: #aaa;
  line-height: 1.8;

  span { color: #777; font-size: 11px; }
}

.smuggler-status {
  --status-color: #999;
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 6px 12px;
  margin-bottom: 12px;
  padding: 11px 12px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-left: 3px solid var(--status-color);
  border-radius: 6px;
  background: rgba(35, 35, 35, 0.6);

  &.forecast { --status-color: #ffb74d; }
  &.appear { --status-color: #81c784; }
  &.disappear_forecast { --status-color: #ef9a9a; }
  strong { display: flex; align-items: center; gap: 7px; color: var(--status-color); font-size: 13px; }
  > span { color: #aaa; font-size: 11px; font-variant-numeric: tabular-nums; }
}

.status-dot { width: 6px; height: 6px; border-radius: 50%; background: currentColor; }
.product-content { display: flex; gap: 12px; padding: 14px 12px; }
.product-image {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 64px;
  height: 64px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 5px;
  background: rgba(0, 0, 0, 0.12);
  color: #888;

  img { width: 52px; height: 52px; object-fit: contain; image-rendering: pixelated; }
}
.product-details { display: flex; flex-direction: column; gap: 6px; min-width: 0; }
.product-details h3 { color: #ffd54f; font-size: 16px; }
.origin { color: #90caf9; }
.product-level { align-self: flex-start; font-size: 11px; }
.product-stats { display: flex; flex-wrap: wrap; gap: 5px; }
.product-stats > span { padding: 3px 6px; border-radius: 3px; background: rgba(255, 255, 255, 0.04); color: #888; font-size: 10px; }
.product-stats b { color: #ccc; font-weight: 500; }
.vehicle-table-wrapper { overflow-x: auto; padding: 6px; }
.vehicle-table { width: 100%; border-collapse: collapse; text-align: center; font-size: 11px; }
.vehicle-table thead th { padding: 7px 4px; color: #888; font-size: 10px; font-weight: 500; white-space: nowrap; }
.vehicle-table tbody th { text-align: left; font-weight: 500; min-width: 120px; }
.vehicle-table tbody th, .vehicle-table td { padding: 9px 4px; border-top: 1px solid rgba(255, 255, 255, 0.05); color: #bbb; font-variant-numeric: tabular-nums; }
.vehicle-table tbody tr:nth-child(even) { background: rgba(255, 255, 255, 0.015); }
.vehicle-table .optimal { background: rgba(255, 213, 79, 0.06); }
.vehicle-table .optimal th, .vehicle-table .optimal td { color: #ffd54f; }
.optimal-badge { display: inline-block; padding: 1px 3px; margin-left: 4px; border-radius: 3px; background: rgba(255, 213, 79, 0.12); color: #ffd54f; font-size: 9px; }
.total-cost { white-space: nowrap; }
.calculation-hint, .capture-hint { color: #777; font-size: 10px; line-height: 1.8; }
.calculation-hint { padding: 0 10px 8px; }
.capture-hint { margin-bottom: 4px; }
</style>
