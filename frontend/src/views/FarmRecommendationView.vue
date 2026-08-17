<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import SvgIcon from '@jamescoyle/vue-icon'
import {
  mdiAlertCircleOutline,
  mdiCashMultiple,
  mdiClipboardTextOutline,
  mdiClockOutline,
  mdiDatabaseSyncOutline,
  mdiHammerWrench,
  mdiKeyVariant,
  mdiSprout,
  mdiTruckCheckOutline,
  mdiTruckDeliveryOutline,
} from '@mdi/js'
import * as api from '../composables/useApi'

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
  statusMessage: '正在读取本地同步数据',
  plantingSuggestions: [],
  plantingProductions: [],
  plantingReferenceCount: 0,
  recommendations: [],
})

const recommendationState = ref<FarmRecommendationState>(emptyState())
const loading = ref(true)
const switchingGoal = ref(false)
const switchingMaterials = ref(false)
let updateInterval: number | null = null

const visibleRecommendations = computed(() => (
  recommendationState.value.recommendations.filter(order => order.remainingDeliveries > 0)
))

type FarmPlotKind = NonNullable<FarmRecommendationAmount['plotKind']>

const plotDefinitions: Array<{ kind: FarmPlotKind; label: string; capacity: number }> = [
  { kind: 'field', label: '普通农田', capacity: 6 },
  { kind: 'redPear', label: '红梨木', capacity: 2 },
  { kind: 'rubber', label: '橡胶树', capacity: 2 },
  { kind: 'spider', label: '蜘蛛古木', capacity: 1 },
  { kind: 'quartz', label: '石英矿脉', capacity: 1 },
]

const plantingRecommendation = computed(() => {
  const groups = plotDefinitions
    .map(plot => ({
      ...plot,
      items: recommendationState.value.plantingSuggestions.filter(item => item.plotKind === plot.kind),
    }))
    .filter(group => group.items.length > 0)
  if (groups.length === 0) return null
  return {
    groups,
    productions: recommendationState.value.plantingProductions,
    referenceCount: recommendationState.value.plantingReferenceCount,
  }
})

function formatDuration(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return '--'
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.ceil((seconds % 3600) / 60)
  if (hours > 0) return `${hours}小时${minutes > 0 ? `${minutes}分` : ''}`
  return `${Math.max(1, minutes)}分钟`
}

function formatRate(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '--'
  return value >= 100 ? value.toFixed(0) : value.toFixed(2)
}

function materialRewardText(rewards?: Record<string, number>): string {
  if (!rewards) return ''
  return Object.entries(rewards)
    .filter(([, count]) => count > 0)
    .map(([name, count]) => `${name} x${count}`)
    .join(' · ')
}

function rewardRangeText(minimum: number, maximum: number): string {
  if (!minimum || minimum === maximum) return ''
  return `浮动 ${minimum}-${maximum}`
}

const factoryLabels: Record<NonNullable<FarmRecommendationAmount['factory']>, string> = {
  abundant: '丰饶',
  gentle: '柔和',
  delicate: '细腻',
  shining: '闪耀',
}

function factoryLabel(factory: FarmRecommendationAmount['factory']): string {
  return factory ? factoryLabels[factory] : ''
}

async function loadRecommendations(silent = false) {
  if (!silent) loading.value = true
  try {
    recommendationState.value = await api.getFarmRecommendationState()
  } catch (error) {
    console.error('加载农场订单推荐失败:', error)
    recommendationState.value.statusMessage = '读取本地推荐数据失败'
  } finally {
    loading.value = false
  }
}

async function setGoal(goal: 'keys' | 'coins') {
  if (switchingGoal.value || recommendationState.value.goal === goal) return
  switchingGoal.value = true
  try {
    recommendationState.value = await api.setFarmRecommendationGoal(goal)
  } catch (error) {
    console.error('切换订单推荐目标失败:', error)
    await loadRecommendations(true)
  } finally {
    switchingGoal.value = false
  }
}

async function setMaterialQuality(quality: 'normal' | 'advanced' | 'highest', enabled: boolean) {
  if (switchingMaterials.value) return
  const normal = quality === 'normal' ? enabled : recommendationState.value.useNormalMaterials
  const advanced = quality === 'advanced' ? enabled : recommendationState.value.useAdvancedMaterials
  const highest = quality === 'highest' ? enabled : recommendationState.value.useHighestMaterials
  if (!normal && !advanced && !highest) return
  switchingMaterials.value = true
  try {
    recommendationState.value = await api.setFarmRecommendationMaterialQualities(normal, advanced, highest)
  } catch (error) {
    console.error('切换订单材料品质失败:', error)
    await loadRecommendations(true)
  } finally {
    switchingMaterials.value = false
  }
}

onMounted(async () => {
  await loadRecommendations()
  updateInterval = window.setInterval(() => loadRecommendations(true), 3000)
})

onUnmounted(() => {
  if (updateInterval !== null) clearInterval(updateInterval)
})
</script>

<template>
  <div class="recommendation-view">
    <section class="recommendation-toolbar" aria-label="推荐设置">
      <div class="goal-control" role="tablist" aria-label="推荐目标">
        <button
          type="button"
          role="tab"
          class="goal-button key-goal"
          :class="{ active: recommendationState.goal === 'keys' }"
          :aria-selected="recommendationState.goal === 'keys'"
          :disabled="switchingGoal"
          @click="setGoal('keys')"
        >
          <svg-icon type="mdi" :path="mdiKeyVariant" :size="14" />
          钥匙优先
        </button>
        <button
          type="button"
          role="tab"
          class="goal-button coin-goal"
          :class="{ active: recommendationState.goal === 'coins' }"
          :aria-selected="recommendationState.goal === 'coins'"
          :disabled="switchingGoal"
          @click="setGoal('coins')"
        >
          <svg-icon type="mdi" :path="mdiCashMultiple" :size="14" />
          硬币优先
        </button>
      </div>
      <div class="quality-control" aria-label="可使用的材料品质">
        <label class="quality-option">
          <input
            type="checkbox"
            :checked="recommendationState.useNormalMaterials"
            :disabled="switchingMaterials"
            @change="setMaterialQuality('normal', ($event.target as HTMLInputElement).checked)"
          >
          <span>普通材料</span>
        </label>
        <label class="quality-option advanced">
          <input
            type="checkbox"
            :checked="recommendationState.useAdvancedMaterials"
            :disabled="switchingMaterials"
            @change="setMaterialQuality('advanced', ($event.target as HTMLInputElement).checked)"
          >
          <span>高级材料</span>
        </label>
        <label class="quality-option highest">
          <input
            type="checkbox"
            :checked="recommendationState.useHighestMaterials"
            :disabled="switchingMaterials"
            @change="setMaterialQuality('highest', ($event.target as HTMLInputElement).checked)"
          >
          <span>最高级材料</span>
        </label>
      </div>
    </section>

    <div v-if="loading" class="recommendation-empty">
      <svg-icon type="mdi" :path="mdiDatabaseSyncOutline" :size="24" />
      <span>正在读取本地推荐数据</span>
    </div>

    <div v-else-if="visibleRecommendations.length === 0" class="recommendation-empty">
      <svg-icon type="mdi" :path="mdiClipboardTextOutline" :size="26" />
      <strong>{{ recommendationState.dataReady ? '当前没有可交货的订单' : '当前玩家数据尚未完整同步' }}</strong>
      <span v-if="!recommendationState.dataReady">{{ recommendationState.statusMessage }}</span>
    </div>

    <template v-else>
      <section v-if="plantingRecommendation" class="planting-summary" aria-label="推荐种植">
        <header class="planting-summary-heading">
          <span class="planting-summary-title">
            <svg-icon type="mdi" :path="mdiSprout" :size="16" />
            <strong>推荐种植</strong>
          </span>
          <span class="planting-source">
            参考推荐前 {{ plantingRecommendation.referenceCount }} 名
          </span>
        </header>

        <div class="plot-groups">
          <div v-for="group in plantingRecommendation.groups" :key="group.kind" class="plot-group">
            <span class="plot-label">{{ group.label }} <small>×{{ group.capacity }}</small></span>
            <div class="plot-items">
              <strong v-for="item in group.items" :key="item.itemId">{{ item.name }} ×{{ item.count }}</strong>
            </div>
          </div>
        </div>

        <div v-if="plantingRecommendation.productions.length" class="planting-production">
          <svg-icon type="mdi" :path="mdiHammerWrench" :size="13" />
          <span>关联制作</span>
          <strong
            v-for="item in plantingRecommendation.productions"
            :key="item.itemId"
          >{{ item.name }} ×{{ item.count }}<span v-if="item.factory" class="factory-name" :class="item.factory">({{ factoryLabel(item.factory) }})</span></strong>
        </div>
      </section>

      <section class="order-list" aria-label="推荐订单列表">
        <article
          v-for="order in visibleRecommendations"
          :key="order.dbKey"
          class="order-row"
          :class="{ blocked: !order.eligible && !order.deliveryStatus, 'delivery-active': !!order.deliveryStatus }"
        >
        <div class="rank-column">
          <span class="rank-label">推荐</span>
          <strong>#{{ order.rank }}</strong>
        </div>

        <div class="order-main">
          <div class="order-heading">
            <strong :title="order.name">{{ order.name }}</strong>
            <span class="delivery-count">剩余 {{ order.remainingDeliveries }}/{{ order.maximumDeliveries }}</span>
          </div>

          <div class="order-tags">
            <span v-if="order.materialSufficient" class="order-tag sufficient">材料充足</span>
            <span v-if="order.refreshRecommended" class="order-tag refresh">推荐刷新</span>
            <span v-if="order.requiresPlanting" class="order-tag planting">需种植</span>
            <span v-if="order.requiresCrafting" class="order-tag crafting">需制作</span>
          </div>

          <div class="reward-line">
            <span v-if="order.keyReward > 0" class="reward key-reward">
              <svg-icon type="mdi" :path="mdiKeyVariant" :size="17" />
              <strong>{{ order.keyReward }} 把</strong>
              <small v-if="rewardRangeText(order.keyRewardMin, order.keyRewardMax)">{{ rewardRangeText(order.keyRewardMin, order.keyRewardMax) }}</small>
            </span>
            <span class="reward coin-reward">
              <svg-icon type="mdi" :path="mdiCashMultiple" :size="17" />
              <strong>{{ order.coinReward }} 枚</strong>
              <small v-if="rewardRangeText(order.coinRewardMin, order.coinRewardMax)">{{ rewardRangeText(order.coinRewardMin, order.coinRewardMax) }}</small>
            </span>
            <span v-if="materialRewardText(order.materialRewards)" class="material-reward">
              <strong>{{ materialRewardText(order.materialRewards) }}</strong>
              <small v-if="rewardRangeText(order.materialRewardMin, order.materialRewardMax)">{{ rewardRangeText(order.materialRewardMin, order.materialRewardMax) }}</small>
            </span>
          </div>

          <div v-if="order.suggestedCrops?.length || order.suggestedProductions?.length" class="work-lines">
            <div v-if="order.suggestedCrops?.length" class="work-line">
              <svg-icon type="mdi" :path="mdiSprout" :size="13" />
              <span>种植</span>
              <strong v-for="item in order.suggestedCrops" :key="item.itemId">{{ item.name }} x{{ item.count }}</strong>
            </div>
            <div v-if="order.suggestedProductions?.length" class="work-line production-line">
              <svg-icon type="mdi" :path="mdiHammerWrench" :size="13" />
              <span>制作</span>
              <strong v-for="item in order.suggestedProductions" :key="item.itemId">{{ item.name }} x{{ item.count }}<span v-if="item.factory" class="factory-name" :class="item.factory">({{ factoryLabel(item.factory) }})</span></strong>
            </div>
          </div>

          <div v-if="order.warnings?.length" class="warning-line">
            <svg-icon type="mdi" :path="mdiAlertCircleOutline" :size="13" />
            <span>{{ order.warnings.join('；') }}</span>
            <strong v-if="order.missingRecipes?.length">缺少：{{ order.missingRecipes.join('、') }}</strong>
          </div>
        </div>

        <div class="efficiency-column" :class="recommendationState.goal">
          <span class="duration">
            <svg-icon type="mdi" :path="mdiClockOutline" :size="12" />
            {{ formatDuration(order.estimatedSeconds) }}
          </span>
          <strong>{{ formatRate(order.targetPerHour) }}</strong>
          <small>{{ recommendationState.goal === 'keys' ? '钥匙/小时' : '硬币/小时' }}</small>
          <span v-if="!order.eligible" class="blocked-label">暂不可制作</span>
        </div>

        <div v-if="order.deliveryStatus" class="delivery-overlay" :class="order.deliveryStatus">
          <svg-icon
            type="mdi"
            :path="order.deliveryStatus === 'completed' ? mdiTruckCheckOutline : mdiTruckDeliveryOutline"
            :size="24"
          />
          <strong>{{ order.deliveryStatus === 'completed' ? '配送完成' : '配送中' }}</strong>
          <span v-if="order.deliveryStatus === 'delivering' && order.deliveryRemainingSeconds">
            剩余 {{ formatDuration(order.deliveryRemainingSeconds) }}
          </span>
        </div>
        </article>
      </section>
    </template>
  </div>
</template>

<style scoped>
.recommendation-view {
  min-height: 100%;
  display: flex;
  flex-direction: column;
  gap: 8px;
  color: #e8e8e8;
}

.recommendation-toolbar {
  min-height: 36px;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 7px 10px;
}

.goal-control {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 5px;
}

.goal-button {
  height: 30px;
  min-width: 82px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  border: 0;
  color: #999;
  background: rgba(255, 255, 255, 0.025);
  font-family: inherit;
  font-size: 10px;
  padding: 0 9px;
  cursor: pointer;
}

.goal-button + .goal-button {
  border-left: 1px solid rgba(255, 255, 255, 0.1);
}

.goal-button:hover {
  color: #ddd;
  background: rgba(255, 255, 255, 0.06);
}

.goal-button.active {
  color: #fff;
  background: rgba(255, 255, 255, 0.09);
}

.goal-button.key-goal.active svg {
  color: #80cbc4;
}

.goal-button.coin-goal.active svg {
  color: #ffd54f;
}

.goal-button:disabled {
  cursor: wait;
  opacity: 0.55;
}

.quality-control {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 3px 8px;
}

.quality-option {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  color: #b7c9bd;
  font-size: 9px;
  cursor: pointer;
}

.quality-option input {
  width: 12px;
  height: 12px;
  margin: 0;
  accent-color: #75b889;
}

.quality-option.advanced {
  color: #9ec2e8;
}

.quality-option.highest {
  color: #e4c781;
}

.quality-option input:disabled {
  cursor: wait;
}

.recommendation-empty {
  min-height: 180px;
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 7px;
  color: #777;
  text-align: center;
  font-size: 10px;
}

.recommendation-empty strong {
  color: #aaa;
  font-size: 12px;
}

.recommendation-empty span {
  max-width: 280px;
  line-height: 1.5;
}

.planting-summary {
  padding: 8px 9px;
  background: rgba(89, 128, 76, 0.08);
  border-top: 1px solid rgba(139, 197, 143, 0.2);
  border-bottom: 1px solid rgba(139, 197, 143, 0.2);
}

.planting-summary-heading,
.planting-summary-title,
.planting-production {
  display: flex;
  align-items: center;
}

.planting-summary-heading {
  min-width: 0;
  gap: 8px;
}

.planting-summary-title {
  flex-shrink: 0;
  gap: 5px;
  color: #a1d4a6;
}

.planting-summary-title strong {
  color: #e4ede3;
  font-size: 11px;
}

.planting-source {
  min-width: 0;
  margin-left: auto;
  overflow: hidden;
  color: #879087;
  font-size: 8px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.plot-groups {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(118px, 1fr));
  gap: 5px 10px;
  margin-top: 7px;
}

.plot-group {
  min-width: 0;
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: baseline;
  gap: 6px;
}

.plot-label {
  color: #8d9d8b;
  font-size: 8px;
  white-space: nowrap;
}

.plot-label small {
  color: #667064;
  font-size: 8px;
}

.plot-items {
  min-width: 0;
  display: flex;
  flex-wrap: wrap;
  gap: 3px 7px;
}

.plot-items strong {
  color: #c4ddc1;
  font-size: 10px;
  font-weight: 600;
}

.planting-production {
  flex-wrap: wrap;
  gap: 4px 7px;
  margin-top: 7px;
  padding-top: 6px;
  color: #90caf9;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  font-size: 9px;
}

.planting-production span {
  color: #7c858c;
}

.planting-production strong {
  color: #c0d3e2;
  font-weight: 500;
}

.order-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.order-row {
  position: relative;
  min-width: 0;
  display: grid;
  grid-template-columns: 42px minmax(0, 1fr) 72px;
  overflow: hidden;
  background: rgba(255, 255, 255, 0.035);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 6px;
}

.order-row.delivery-active {
  border-color: rgba(255, 255, 255, 0.12);
}

.delivery-overlay {
  position: absolute;
  z-index: 3;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  color: #d9ded7;
  background: rgba(0, 0, 0, 0.64);
  pointer-events: all;
}

.delivery-overlay svg {
  color: #9eb7a4;
}

.delivery-overlay strong {
  font-size: 13px;
}

.delivery-overlay span {
  color: #9ca39d;
  font-size: 9px;
}

.delivery-overlay.completed svg,
.delivery-overlay.completed strong {
  color: #8bc58f;
}

.order-row.blocked {
  border-color: rgba(239, 83, 80, 0.25);
  background: rgba(239, 83, 80, 0.035);
}

.rank-column,
.efficiency-column {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}

.rank-column {
  background: rgba(255, 255, 255, 0.025);
  border-right: 1px solid rgba(255, 255, 255, 0.07);
}

.rank-label {
  color: #777;
  font-size: 8px;
}

.rank-column strong {
  margin-top: 2px;
  color: #e0e0e0;
  font-family: Consolas, monospace;
  font-size: 15px;
}

.order-main {
  min-width: 0;
  padding: 7px 8px;
}

.order-heading,
.reward-line,
.work-line,
.warning-line {
  min-width: 0;
  display: flex;
  align-items: center;
}

.order-heading {
  gap: 7px;
}

.order-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 5px;
}

.order-tag {
  padding: 1px 4px;
  border: 1px solid currentColor;
  border-radius: 3px;
  font-size: 8px;
  line-height: 1.3;
}

.order-tag.sufficient {
  color: #8bc58f;
}

.order-tag.refresh {
  color: #f1c75b;
}

.order-tag.crafting {
  color: #90bce9;
}

.order-tag.planting {
  color: #a1d4a6;
}

.order-heading > strong {
  min-width: 0;
  overflow: hidden;
  color: #f0f0f0;
  font-size: 14px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.delivery-count {
  margin-left: auto;
  flex-shrink: 0;
  color: #aaa;
  font-size: 9px;
}

.reward-line {
  margin-top: 5px;
  flex-wrap: wrap;
  gap: 4px 9px;
  color: #aaa;
  font-size: 10px;
}

.reward {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  font-weight: 700;
}

.reward strong,
.material-reward strong {
  font-size: 11px;
  line-height: 1;
}

.reward small,
.material-reward small {
  color: #89919a;
  font-size: 8px;
  font-weight: 400;
}

.key-reward {
  color: #80cbc4;
}

.coin-reward {
  color: #ffd54f;
}

.material-reward {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: #b0bec5;
}

.work-lines {
  margin-top: 6px;
  padding-top: 5px;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
}

.work-line {
  flex-wrap: wrap;
  gap: 3px 6px;
  color: #81c784;
  font-size: 9px;
  line-height: 1.45;
}

.work-line > span {
  color: #777;
}

.work-line > strong {
  color: #b9d6bb;
  font-weight: 500;
}

.production-line {
  margin-top: 3px;
  color: #90caf9;
}

.production-line > strong {
  color: #b9cee0;
}

.factory-name {
  margin-left: 2px;
  font-weight: 600;
}

.factory-name.abundant {
  color: #ef8177;
}

.factory-name.gentle {
  color: #f1f3f4;
}

.factory-name.delicate {
  color: #82c98b;
}

.factory-name.shining {
  color: #64b5f6;
}

.warning-line {
  margin-top: 6px;
  align-items: flex-start;
  flex-wrap: wrap;
  gap: 3px 5px;
  color: #ffb4ab;
  font-size: 9px;
  line-height: 1.4;
}

.warning-line > svg {
  flex-shrink: 0;
}

.warning-line strong {
  color: #ff8a80;
}

.efficiency-column {
  padding: 6px 4px;
  border-left: 1px solid rgba(255, 255, 255, 0.07);
  background: rgba(0, 0, 0, 0.1);
  text-align: center;
}

.efficiency-column .duration {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  color: #888;
  font-size: 8px;
}

.efficiency-column > strong {
  margin-top: 3px;
  color: #80cbc4;
  font-family: Consolas, monospace;
  font-size: 16px;
}

.efficiency-column.coins > strong {
  color: #ffd54f;
}

.efficiency-column small {
  color: #888;
  font-size: 8px;
}

.blocked .efficiency-column > strong,
.blocked .efficiency-column small {
  color: #777;
}

.blocked-label {
  margin-top: 4px;
  color: #ff8a80;
  font-size: 8px;
}

@media (max-width: 480px) {
  .goal-button {
    min-width: 72px;
    padding: 0 6px;
  }

  .order-row {
    grid-template-columns: 36px minmax(0, 1fr) 64px;
  }

  .order-main {
    padding-right: 6px;
    padding-left: 6px;
  }
}
</style>
