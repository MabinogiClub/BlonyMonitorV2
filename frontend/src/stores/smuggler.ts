import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import * as api from '../composables/useApi'
import { useEirinTime } from '../composables/useEirinTime'
import { getSmugglerMarqueeText, getSmugglerWindow, isCurrentSmugglerNotice, type SmugglerNotice } from '../utils/smuggler'
import { formatCountdown, formatRealTime } from '../data/ferry'

export const useSmugglerStore = defineStore('smuggler', () => {
  const notice = ref<SmugglerNotice | null>(null)
  const { clockNow, eirinPhaseOffsetMinutes, eirinTimeLabel } = useEirinTime()
  const cycle = computed(() => getSmugglerWindow(clockNow.value, eirinPhaseOffsetMinutes.value))
  const currentNotice = computed(() => isCurrentSmugglerNotice(notice.value, clockNow.value, eirinPhaseOffsetMinutes.value) ? notice.value : null)
  const statusLabel = computed(() => ({
    absent: '未出现', forecast: '即将出现', appear: '出现中', disappear_forecast: '即将消失',
  })[cycle.value.phase])
  const countdown = computed(() => {
    const { phase, appearAt, goneAt } = cycle.value
    const target = phase === 'absent' || phase === 'forecast' ? appearAt : goneAt
    const action = phase === 'absent' || phase === 'forecast' ? '出现' : '消失'
    return `${formatCountdown(target - clockNow.value)} 后${action}（${formatRealTime(target)}）`
  })
  const marqueeText = computed(() => getSmugglerMarqueeText(currentNotice.value, statusLabel.value, countdown.value))

  let revision = 0
  function receiveNotice(value: SmugglerNotice | null) {
    revision += 1
    notice.value = value
  }
  async function start() {
    api.onEvent('smuggler-notice', receiveNotice)
    const snapshotRevision = revision
    try {
      const initial = await api.getSmugglerNotice()
      if (revision === snapshotRevision) notice.value = initial
    } catch (error) {
      console.error('加载走私商人提示失败:', error)
    }
  }
  function stop() {
    api.offEvent('smuggler-notice', receiveNotice)
  }
  return { currentNotice, cycle, statusLabel, countdown, marqueeText, eirinTimeLabel, start, stop }
})
