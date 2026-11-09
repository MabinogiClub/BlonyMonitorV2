<script setup lang="ts">
import { computed, ref } from 'vue'
import SvgIcon from '@jamescoyle/vue-icon'
import { mdiFerry, mdiChevronDown } from '@mdi/js'
import { useEirinTime } from '../composables/useEirinTime'
import SmugglerView from './SmugglerView.vue'
import BarterView from './BarterView.vue'
import {
  FERRY_GROUPS, formatCountdown, formatRealTime, getSailingStatus, getUpcomingSailings, realDateLabel,
} from '../data/ferry'

defineProps<{ section: string }>()

const { clockNow, eirinPhaseOffsetMinutes, eirinTimeLabel, realTimeLabel } = useEirinTime()
const statusLabels = { waiting: '待登船', boarding: '登船中', sailing: '航行中' }
const expandedGroups = ref<Set<string>>(new Set())

function toggleGroup(id: string) {
  if (expandedGroups.value.has(id)) expandedGroups.value.delete(id)
  else expandedGroups.value.add(id)
}

const groups = computed(() => FERRY_GROUPS.map(group => {
  const sailings = getUpcomingSailings(group, clockNow.value, eirinPhaseOffsetMinutes.value)
  const next = sailings.find(sailing => sailing.departure > clockNow.value)!
  const boarding = clockNow.value >= next.boarding
  return {
    ...group,
    sailings,
    boarding,
    remaining: formatCountdown((boarding ? next.departure : next.boarding) - clockNow.value),
  }
}))
</script>

<template>
  <div v-if="section === 'tradeFerry'" class="trade-view">
    <div class="trade-heading">
      <div class="heading-title">
        <svg-icon type="mdi" :path="mdiFerry" :size="20" />
        <h2>商船时间表</h2>
      </div>
      <div class="clocks">
        <span>爱琳 <strong>{{ eirinTimeLabel }}</strong></span>
        <span>现实 <strong>{{ realTimeLabel }}</strong></span>
      </div>
    </div>
    <p class="schedule-hint">以下班次均为现实时间，倒计时为现实分秒。校准请在首页调整爱琳时间。</p>

    <section v-for="group in groups" :key="group.id" class="port-card" :class="group.id" :aria-label="group.name">
      <header>
        <button
          type="button"
          class="port-header"
          :aria-expanded="expandedGroups.has(group.id)"
          :aria-controls="`sailings-${group.id}`"
          :aria-label="`${group.name}，${expandedGroups.has(group.id) ? '收起' : '展开'}全部班次`"
          @click="toggleGroup(group.id)"
        >
          <span class="port-info">
            <span class="port-name">{{ group.name }}</span>
            <span class="routes"><span v-for="route in group.routes" :key="route">{{ route }}</span></span>
          </span>
          <span class="next-sailing" :class="{ boarding: group.boarding }" aria-live="off">
            <span>{{ group.boarding ? '登船中 · 距开船' : '下班船 · 距登船' }}</span>
            <strong>{{ group.remaining }}</strong>
          </span>
          <svg-icon type="mdi" :path="mdiChevronDown" :size="18" class="expand-icon" :class="{ expanded: expandedGroups.has(group.id) }" aria-hidden="true" />
        </button>
      </header>
      <table class="sailing-table">
        <thead><tr><th scope="col">登船时间</th><th scope="col">开船时间</th><th scope="col">到达时间</th><th scope="col">状态</th></tr></thead>
        <tbody :id="`sailings-${group.id}`">
          <tr v-for="sailing in (expandedGroups.has(group.id) ? group.sailings : group.sailings.slice(0, 1))" :key="sailing.boarding" :class="getSailingStatus(sailing, clockNow)">
            <td v-for="(time, index) in [sailing.boarding, sailing.departure, sailing.arrival]" :key="index">
              <span v-if="realDateLabel(time, clockNow)" class="date-label">{{ realDateLabel(time, clockNow) }}</span>
              <time :datetime="new Date(time).toISOString()">{{ formatRealTime(time) }}</time>
            </td>
            <td><span class="status-badge">{{ statusLabels[getSailingStatus(sailing, clockNow)] }}</span></td>
          </tr>
        </tbody>
      </table>
    </section>
    <p class="schedule-hint footer-hint">双向航线共用所属港口系的时刻表，点击港口概览展开全部班次。</p>
  </div>
  <SmugglerView v-else-if="section === 'tradeSmuggler'" />
  <BarterView v-else-if="section === 'tradeBarter'" />
</template>

<style lang="scss" scoped>
.trade-view {
  padding: 6px;
}

.trade-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
  margin: 4px 0 8px;
}

.heading-title {
  display: flex;
  align-items: center;
  gap: 7px;
  color: #90caf9;
}

h2 {
  color: #eee;
  font-size: 14px;
  font-weight: 600;
}

.clocks {
  display: flex;
  gap: 14px;
  color: #999;
  font-size: 11px;
}

.clocks strong {
  color: #ddd;
  font-family: Consolas, monospace;
  font-size: 12px;
  font-weight: 500;
}

.schedule-hint {
  color: #888;
  font-size: 11px;
  line-height: 1.7;
  margin-bottom: 12px;
}

.port-card {
  --port-color: #90caf9;
  margin-bottom: 12px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 6px;
  overflow: hidden;
  background: rgba(35, 35, 35, 0.6);
}

.port-card.qilla {
  --port-color: #a5d6a7;
}

.port-card.belvast {
  --port-color: #ce93d8;
}

.port-header {
  width: 100%;
  border: 0;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
  padding: 10px 12px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  background: rgba(255, 255, 255, 0.025);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  border-left: 3px solid var(--port-color);
}

.port-header:hover {
  background: rgba(255, 255, 255, 0.05);
}

.port-header:focus-visible {
  outline: 2px solid var(--port-color);
  outline-offset: -2px;
}

.expand-icon {
  flex-shrink: 0;
  color: #888;
  transition: transform 0.15s ease;
}

.expand-icon.expanded {
  transform: rotate(180deg);
}

.port-name {
  display: block;
  color: var(--port-color);
  font-size: 13px;
  font-weight: 600;
  margin-bottom: 5px;
}

.routes {
  display: flex;
  flex-wrap: wrap;
  gap: 3px 14px;
  color: #aaa;
  font-size: 11px;
}

.routes span {
  white-space: nowrap;
}

.next-sailing {
  margin-left: auto;
  flex-shrink: 0;
  text-align: right;
  color: #aaa;
  font-size: 10px;
}

.next-sailing strong {
  display: block;
  color: var(--port-color);
  font-family: Consolas, 'Microsoft YaHei', monospace;
  font-size: 16px;
  margin-top: 4px;
  font-variant-numeric: tabular-nums;
}

.next-sailing.boarding, .next-sailing.boarding strong {
  color: #81c784;
}

.sailing-table {
  width: 100%;
  border-collapse: collapse;
  table-layout: fixed;
  font-size: 12px;
  text-align: center;
}

th {
  padding: 7px 3px;
  color: #888;
  font-size: 11px;
  font-weight: 500;
  background: rgba(0, 0, 0, 0.15);
}

td {
  padding: 8px 3px;
  border-top: 1px solid rgba(255, 255, 255, 0.045);
  color: #ccc;
  font-family: Consolas, 'Microsoft YaHei', monospace;
  font-variant-numeric: tabular-nums;
}

tbody tr:nth-child(even) {
  background: rgba(255, 255, 255, 0.015);
}

tbody tr.boarding {
  background: rgba(102, 187, 106, 0.07);
}

.date-label {
  display: block;
  color: #888;
  font-size: 10px;
}

.status-badge {
  display: inline-block;
  color: #888;
  font-size: 10px;
  font-family: 'Microsoft YaHei', sans-serif;
  padding: 2px 6px;
  border-radius: 3px;
  background: rgba(255, 255, 255, 0.04);
}

.boarding .status-badge {
  color: #81c784;
  background: rgba(102, 187, 106, 0.12);
}

.sailing .status-badge {
  color: #90caf9;
  background: rgba(66, 165, 245, 0.12);
}

.footer-hint {
  margin-bottom: 4px;
}

</style>
