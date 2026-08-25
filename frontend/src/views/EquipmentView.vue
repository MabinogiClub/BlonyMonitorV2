<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import SvgIcon from '@jamescoyle/vue-icon'
import { mdiRefresh } from '@mdi/js'
import * as api from '../composables/useApi'

const state = ref<SelfEquipmentState>({ ready: false, slots: [] })
const loading = ref(false)
let updateInterval: number | null = null

const slotMap = computed(() => {
  const map = new Map<number, SelfEquipmentSlot>()
  for (const slot of state.value.slots || []) {
    map.set(slot.pocketType, slot)
  }
  return map
})

function slot(pocket: number): SelfEquipmentSlot | undefined {
  return slotMap.value.get(pocket)
}

function formatDetail(value: number | undefined | null): string {
  if (value === undefined || value === null || !Number.isFinite(value)) return '--'
  return Number(value.toFixed(3)).toString()
}

function formatPair(item?: SelfEquipmentSlot): string {
  if (!item?.itemId) return '--'
  const current = formatDetail(item.durabilityValue)
  const max = formatDetail(item.durabilityMax)
  return max === '--' ? current : `${current}/${max}`
}

function formatDelta(value: number | undefined | null): string {
  if (value === undefined || value === null || !Number.isFinite(value) || value === 0) return ''
  const sign = value > 0 ? '+' : ''
  return `${sign}${formatDetail(value)}`
}

function formatTime(value: number | undefined): string {
  if (!value) return ''
  return new Date(value).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

/** 不可损坏（0/0）或无有效上限时不染色，沿用默认边框。 */
function durabilityTone(item?: SelfEquipmentSlot): string {
  if (!item?.itemId) return ''
  const current = item.durabilityValue
  const max = item.durabilityMax
  if (
    current === undefined ||
    current === null ||
    max === undefined ||
    max === null ||
    !Number.isFinite(current) ||
    !Number.isFinite(max) ||
    max <= 0
  ) {
    return ''
  }
  const ratio = current / max
  if (ratio >= 0.5) return 'dur-ok'
  if (ratio >= 0.25) return 'dur-fair'
  if (ratio >= 0.1) return 'dur-warn'
  if (ratio >= 0.05) return 'dur-low'
  return 'dur-crit'
}

function slotClass(pocket: number) {
  const item = slot(pocket)
  const tone = durabilityTone(item)
  return {
    filled: !!item?.itemId,
    [tone]: !!tone,
  }
}

const recentChanges = computed(() => {
  const rows: Array<{ pocketType: number; label: string; name: string; sample: { at: number; value: number; raw: number }; delta?: number }> = []
  for (const item of state.value.slots || []) {
    const history = item.durabilityHistory || []
    for (let i = 1; i < history.length; i++) {
      rows.push({
        pocketType: item.pocketType,
        label: item.slotLabel,
        name: item.name || `物品 #${item.itemId}`,
        sample: history[i],
        delta: history[i].value - history[i - 1].value,
      })
    }
  }
  return rows.sort((a, b) => b.sample.at - a.sample.at).slice(0, 10)
})

async function updateView() {
  loading.value = true
  try {
    state.value = await api.getSelfEquipmentState()
  } catch (error) {
    console.error('加载装备耐久失败:', error)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void updateView()
  updateInterval = window.setInterval(updateView, 500)
})

onUnmounted(() => {
  if (updateInterval) clearInterval(updateInterval)
})
</script>

<template>
  <div class="durability-view">
    <div class="toolbar">
      <div class="toolbar-title">装备耐久</div>
      <button type="button" class="refresh-button" title="刷新" :disabled="loading" @click="updateView">
        <svg-icon type="mdi" :path="mdiRefresh" :size="16" />
      </button>
    </div>

    <div v-if="!state.ready" class="empty-state">
      <p>等待自己的装备快照...</p>
      <small>登录后自动读取装备栏；使用/修理时会更新精确耐久</small>
    </div>

    <template v-else>
      <div class="equip-stage">
        <div class="equip-window">
          <div class="equip-grid" aria-label="装备栏">
            <!-- 顶行：左饰 / 头 / 右饰 -->
            <div class="slot" :class="slotClass(16)">
              <span class="slot-part">左饰</span>
              <span class="slot-name">{{ slot(16)?.name || '空' }}</span>
              <span class="slot-dur">{{ formatPair(slot(16)) }}</span>
              <span v-if="formatDelta(slot(16)?.durabilityDelta)" class="slot-delta">{{ formatDelta(slot(16)?.durabilityDelta) }}</span>
            </div>
            <div class="slot" :class="slotClass(8)">
              <span class="slot-part">头部</span>
              <span class="slot-name">{{ slot(8)?.name || '空' }}</span>
              <span class="slot-dur">{{ formatPair(slot(8)) }}</span>
              <span v-if="formatDelta(slot(8)?.durabilityDelta)" class="slot-delta">{{ formatDelta(slot(8)?.durabilityDelta) }}</span>
            </div>
            <div class="slot" :class="slotClass(17)">
              <span class="slot-part">右饰</span>
              <span class="slot-name">{{ slot(17)?.name || '空' }}</span>
              <span class="slot-dur">{{ formatPair(slot(17)) }}</span>
              <span v-if="formatDelta(slot(17)?.durabilityDelta)" class="slot-delta">{{ formatDelta(slot(17)?.durabilityDelta) }}</span>
            </div>

            <!-- 中行：武器I（主+副） / 衣服 / 武器II（主+副） -->
            <div class="weapon-slot">
              <div class="slot nested" :class="slotClass(10)">
                <span class="slot-part">主手</span>
                <span class="slot-name">{{ slot(10)?.name || '空' }}</span>
                <span class="slot-dur">{{ formatPair(slot(10)) }}</span>
                <span v-if="formatDelta(slot(10)?.durabilityDelta)" class="slot-delta">{{ formatDelta(slot(10)?.durabilityDelta) }}</span>
              </div>
              <div class="slot nested" :class="slotClass(12)">
                <span class="slot-part">副手</span>
                <span class="slot-name">{{ slot(12)?.name || '空' }}</span>
                <span class="slot-dur">{{ formatPair(slot(12)) }}</span>
                <span v-if="formatDelta(slot(12)?.durabilityDelta)" class="slot-delta">{{ formatDelta(slot(12)?.durabilityDelta) }}</span>
              </div>
            </div>

            <div class="slot body" :class="slotClass(5)">
              <span class="slot-part">衣服</span>
              <span class="slot-name">{{ slot(5)?.name || '空' }}</span>
              <span class="slot-dur">{{ formatPair(slot(5)) }}</span>
              <span v-if="formatDelta(slot(5)?.durabilityDelta)" class="slot-delta">{{ formatDelta(slot(5)?.durabilityDelta) }}</span>
            </div>

            <div class="weapon-slot">
              <div class="slot nested" :class="slotClass(11)">
                <span class="slot-part">主手</span>
                <span class="slot-name">{{ slot(11)?.name || '空' }}</span>
                <span class="slot-dur">{{ formatPair(slot(11)) }}</span>
                <span v-if="formatDelta(slot(11)?.durabilityDelta)" class="slot-delta">{{ formatDelta(slot(11)?.durabilityDelta) }}</span>
              </div>
              <div class="slot nested" :class="slotClass(13)">
                <span class="slot-part">副手</span>
                <span class="slot-name">{{ slot(13)?.name || '空' }}</span>
                <span class="slot-dur">{{ formatPair(slot(13)) }}</span>
                <span v-if="formatDelta(slot(13)?.durabilityDelta)" class="slot-delta">{{ formatDelta(slot(13)?.durabilityDelta) }}</span>
              </div>
            </div>

            <!-- 底行：手 / 脚 / 长袍 -->
            <div class="slot" :class="slotClass(6)">
              <span class="slot-part">手部</span>
              <span class="slot-name">{{ slot(6)?.name || '空' }}</span>
              <span class="slot-dur">{{ formatPair(slot(6)) }}</span>
              <span v-if="formatDelta(slot(6)?.durabilityDelta)" class="slot-delta">{{ formatDelta(slot(6)?.durabilityDelta) }}</span>
            </div>
            <div class="slot" :class="slotClass(7)">
              <span class="slot-part">脚部</span>
              <span class="slot-name">{{ slot(7)?.name || '空' }}</span>
              <span class="slot-dur">{{ formatPair(slot(7)) }}</span>
              <span v-if="formatDelta(slot(7)?.durabilityDelta)" class="slot-delta">{{ formatDelta(slot(7)?.durabilityDelta) }}</span>
            </div>
            <div class="slot" :class="slotClass(9)">
              <span class="slot-part">长袍</span>
              <span class="slot-name">{{ slot(9)?.name || '空' }}</span>
              <span class="slot-dur">{{ formatPair(slot(9)) }}</span>
              <span v-if="formatDelta(slot(9)?.durabilityDelta)" class="slot-delta">{{ formatDelta(slot(9)?.durabilityDelta) }}</span>
            </div>
          </div>

          <div class="window-footer">
            <span class="char-name">{{ state.character || '自己' }}</span>
            <span class="snap-time">精确耐久 · {{ formatTime(state.capturedAt) || '已捕获' }}</span>
          </div>
        </div>

        <section class="history-panel">
          <h4>最近变化</h4>
          <div v-if="recentChanges.length === 0" class="history-empty">使用或修理后，这里列出每次精确变化</div>
          <ul v-else>
            <li v-for="row in recentChanges" :key="`${row.pocketType}-${row.sample.at}-${row.sample.raw}`">
              <span class="time">{{ formatTime(row.sample.at) }}</span>
              <span class="who">{{ row.label }} · {{ row.name }}</span>
              <b>{{ formatDetail(row.sample.value) }}</b>
              <em v-if="formatDelta(row.delta)">{{ formatDelta(row.delta) }}</em>
            </li>
          </ul>
        </section>
      </div>
    </template>
  </div>
</template>

<style lang="scss" scoped>
.durability-view {
  min-height: 100%;
  color: #e6e6e6;
}

.toolbar {
  height: 38px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  margin: -6px -6px 10px;
  padding: 0 10px;
  background: rgba(30, 30, 30, 0.65);
}

.toolbar-title {
  font-size: 13px;
  font-weight: 600;
}

.refresh-button {
  width: 28px;
  height: 28px;
  border: 0;
  border-radius: 4px;
  display: grid;
  place-items: center;
  color: #aaa;
  background: transparent;
  cursor: pointer;

  &:hover {
    color: #fff;
    background: rgba(255, 255, 255, 0.08);
  }

  &:disabled {
    opacity: 0.5;
    cursor: wait;
  }
}

.empty-state {
  min-height: 220px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  text-align: center;
  color: #8b949e;

  p {
    color: #d0d7de;
    font-size: 13px;
  }

  small {
    max-width: 300px;
    line-height: 1.5;
    font-size: 11px;
  }
}

.equip-stage {
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 100%;
}

.equip-window {
  width: min(100%, 360px);
  border: 1px solid #2a2a2a;
  border-radius: 2px;
  background: linear-gradient(180deg, #1a1a1a 0%, #141414 100%);
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.03);
}

.equip-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  grid-template-rows: minmax(88px, auto) minmax(148px, auto) minmax(88px, auto);
  gap: 3px;
  padding: 6px;
  background: #0d0d0d;
}

.slot {
  min-width: 0;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 3px;
  padding: 7px 6px;
  border: 1px solid #3a3a3a;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.03), transparent 28%),
    #1c1c1c;
  text-align: center;
}

.slot.filled {
  border-color: #5a5a5a;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.05), transparent 30%),
    #222;
}

.slot.dur-ok {
  border-color: #3d9b5f;
}

.slot.dur-fair {
  border-color: #7bc47f;
}

.slot.dur-warn {
  border-color: #d4b83a;
}

.slot.dur-low {
  border-color: #e08a3c;
}

.slot.dur-crit {
  border-color: #d94c4c;
}

.slot.body {
  min-height: 100%;
}

.slot.nested {
  flex: 1;
  min-height: 0;
}

.slot-part {
  color: #8d8d8d;
  font-size: 9px;
  letter-spacing: 0.04em;
}

.slot-name {
  color: #ececec;
  font-size: 11px;
  font-weight: 500;
  line-height: 1.3;
  word-break: break-all;
}

.slot-dur {
  color: #7fd4c8;
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.slot-delta {
  color: #ffb74d;
  font-size: 9px;
}

.weapon-slot {
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-height: 100%;
}

.window-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 8px;
  border-top: 1px solid #2f2f2f;
  background: #121212;
}

.char-name {
  color: #ddd;
  font-size: 11px;
}

.snap-time {
  color: #777;
  font-size: 9px;
}

.history-panel {
  margin-top: 12px;
  width: min(100%, 360px);

  h4 {
    margin-bottom: 6px;
    color: #cfcfcf;
    font-size: 11px;
  }

  ul {
    display: grid;
    gap: 3px;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  li {
    display: grid;
    grid-template-columns: 58px minmax(0, 1fr) auto auto;
    gap: 6px;
    align-items: center;
    padding: 4px 5px;
    border: 1px solid #2a2a2a;
    background: #171717;
    font-size: 10px;
  }

  .time {
    color: #888;
  }

  .who {
    overflow: hidden;
    color: #ccc;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  b {
    color: #7fd4c8;
    font-variant-numeric: tabular-nums;
  }

  em {
    color: #ffb74d;
    font-style: normal;
  }
}

.history-empty {
  color: #6f6f6f;
  font-size: 10px;
}

@media (max-width: 420px) {
  .equip-window {
    width: 100%;
  }
}
</style>
