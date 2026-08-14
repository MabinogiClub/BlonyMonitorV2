<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import SvgIcon from '@jamescoyle/vue-icon'
import {
  mdiAccountGroupOutline,
  mdiBellRingOutline,
  mdiSproutOutline,
  mdiSwordCross,
} from '@mdi/js'
import { useAppStore } from '../stores/app'
import * as api from '../composables/useApi'
import { formatNumber } from '../composables/useUtils'

const appStore = useAppStore()
const attackers = ref<AttackerStats[]>([])
const entities = ref<EntityInfo[]>([])
const buffs = ref<BuffDisplayInfo[]>([])
const farm = ref<FarmState | null>(null)
let updateInterval: number | null = null

const totalDamage = computed(() => attackers.value.reduce((sum, item) => sum + item.totalDamage, 0))
const totalDps = computed(() => attackers.value.reduce((sum, item) => sum + item.dps, 0))
const activeBuffs = computed(() => buffs.value.filter(item => item.isActive).length)
const readyPlots = computed(() => farm.value?.plots.filter(plot => plot.ready).length || 0)

const metrics = computed(() => [
  { label: '团队 DPS', value: formatNumber(totalDps.value), icon: mdiSwordCross, tone: 'blue', target: 'bySkill' },
  { label: '累计伤害', value: formatNumber(totalDamage.value), icon: mdiSwordCross, tone: 'gold', target: 'bySkill' },
  { label: '已识别角色', value: String(entities.value.length), icon: mdiAccountGroupOutline, tone: 'green', target: 'entities' },
  { label: '生效 Buff', value: String(activeBuffs.value), icon: mdiBellRingOutline, tone: 'orange', target: 'buffTimer' },
])

async function loadOverview() {
  const [damageResult, entityResult, buffResult, farmResult] = await Promise.allSettled([
    api.getDamageBySkill(),
    api.getAllPCEntities(),
    api.getBuffDisplayList(),
    api.getFarmState(),
  ])

  if (damageResult.status === 'fulfilled') attackers.value = damageResult.value || []
  if (entityResult.status === 'fulfilled') entities.value = entityResult.value || []
  if (buffResult.status === 'fulfilled') buffs.value = buffResult.value || []
  if (farmResult.status === 'fulfilled') farm.value = farmResult.value
}

onMounted(() => {
  void loadOverview()
  updateInterval = window.setInterval(loadOverview, 1000)
})

onUnmounted(() => {
  if (updateInterval !== null) clearInterval(updateInterval)
})
</script>

<template>
  <div class="home-view">
    <header class="home-header">
      <div>
        <h1>首页</h1>
        <p>{{ appStore.selfInfo?.name || '等待识别角色' }}</p>
      </div>
      <span class="connection-state" :class="{ connected: appStore.isConnected }">
        <i />{{ appStore.isConnected ? '已连接' : '未连接' }}
      </span>
    </header>

    <section class="metric-grid" aria-label="实时概览">
      <button
        v-for="metric in metrics"
        :key="metric.label"
        type="button"
        class="metric-item"
        @click="appStore.setActiveTab(metric.target)"
      >
        <span class="metric-icon" :class="metric.tone">
          <svg-icon type="mdi" :path="metric.icon" :size="18" />
        </span>
        <span class="metric-copy">
          <strong>{{ metric.value }}</strong>
          <small>{{ metric.label }}</small>
        </span>
      </button>
    </section>

    <section class="home-section">
      <div class="section-heading">
        <h2>伤害排行</h2>
        <button type="button" @click="appStore.setActiveTab('bySkill')">查看全部</button>
      </div>
      <div v-if="attackers.length" class="ranking-list">
        <button
          v-for="(attacker, index) in attackers.slice(0, 5)"
          :key="attacker.id"
          type="button"
          class="ranking-row"
          @click="appStore.setActiveTab('bySkill')"
        >
          <span class="rank">{{ index + 1 }}</span>
          <span class="player-name">{{ attacker.name }}</span>
          <span class="player-dps">{{ formatNumber(attacker.dps) }}/s</span>
          <span class="player-percent">{{ attacker.percent.toFixed(1) }}%</span>
        </button>
      </div>
      <div v-else class="empty-overview">等待战斗数据...</div>
    </section>

    <button type="button" class="farm-summary" @click="appStore.setActiveTab('farm')">
      <span class="farm-icon"><svg-icon type="mdi" :path="mdiSproutOutline" :size="20" /></span>
      <div>
        <strong>塔汀农场</strong>
        <small v-if="farm?.synced">{{ readyPlots ? `${readyPlots} 块作物可收获` : '作物生长中' }}</small>
        <small v-else>等待农场数据</small>
      </div>
      <span class="farm-state" :class="{ ready: readyPlots > 0 }">{{ readyPlots > 0 ? '可收获' : '查看' }}</span>
    </button>
  </div>
</template>

<style lang="scss" scoped>
.home-view {
  min-height: 100%;
  padding: 12px;
  color: #eee;
}

.home-header {
  min-height: 54px;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);

  h1 {
    font-size: 17px;
    line-height: 1.2;
    font-weight: 600;
  }

  p {
    margin-top: 5px;
    color: #888;
    font-size: 11px;
  }
}

.connection-state {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  margin-top: 2px;
  color: #999;
  font-size: 10px;

  i {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: #777;
  }

  &.connected {
    color: #81c784;

    i { background: #4caf50; }
  }
}

.metric-grid {
  margin: 12px 0 16px;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 6px;
}

.metric-item {
  min-width: 0;
  height: 58px;
  padding: 8px;
  display: flex;
  align-items: center;
  gap: 8px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 5px;
  color: #ddd;
  background: rgba(255, 255, 255, 0.035);
  cursor: pointer;
  text-align: left;

  &:hover { background: rgba(255, 255, 255, 0.07); }
}

.metric-icon {
  width: 30px;
  height: 30px;
  flex-shrink: 0;
  display: grid;
  place-items: center;
  border-radius: 4px;

  &.blue { color: #64b5f6; background: rgba(66, 165, 245, 0.15); }
  &.gold { color: #ffd54f; background: rgba(255, 193, 7, 0.14); }
  &.green { color: #81c784; background: rgba(76, 175, 80, 0.14); }
  &.orange { color: #ffb74d; background: rgba(255, 152, 0, 0.14); }
}

.metric-copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;

  strong {
    overflow: hidden;
    color: #fff;
    font-size: 15px;
    line-height: 1;
    text-overflow: ellipsis;
  }

  small { color: #8e8e8e; font-size: 10px; }
}

.home-section {
  margin-bottom: 12px;
}

.section-heading {
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: space-between;

  h2 { font-size: 12px; font-weight: 600; }

  button {
    border: 0;
    color: #64b5f6;
    background: transparent;
    font-size: 10px;
    cursor: pointer;
  }
}

.ranking-list {
  border-top: 1px solid rgba(255, 255, 255, 0.08);
}

.ranking-row {
  width: 100%;
  height: 34px;
  display: grid;
  grid-template-columns: 22px minmax(0, 1fr) auto 42px;
  align-items: center;
  gap: 6px;
  border: 0;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  color: #ccc;
  background: transparent;
  text-align: left;
  cursor: pointer;

  &:hover { background: rgba(255, 255, 255, 0.04); }
}

.rank { color: #777; text-align: center; }
.player-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.player-dps { color: #81c784; font-size: 10px; }
.player-percent { color: #999; font-size: 10px; text-align: right; }

.empty-overview {
  height: 90px;
  display: grid;
  place-items: center;
  color: #666;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  font-size: 11px;
}

.farm-summary {
  width: 100%;
  min-height: 52px;
  padding: 8px 10px;
  display: flex;
  align-items: center;
  gap: 9px;
  border-top: 1px solid rgba(255, 255, 255, 0.09);
  border-right: 0;
  border-bottom: 1px solid rgba(255, 255, 255, 0.09);
  border-left: 0;
  color: #eee;
  background: transparent;
  font-family: inherit;
  text-align: left;
  cursor: pointer;

  &:hover { background: rgba(255, 255, 255, 0.04); }

  > div {
    min-width: 0;
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 3px;
  }

  strong { font-size: 11px; font-weight: 600; }
  small { color: #888; font-size: 10px; }
}

.farm-icon { color: #81c784; }
.farm-state {
  color: #777;
  font-size: 10px;

  &.ready { color: #ffd54f; }
}
</style>
