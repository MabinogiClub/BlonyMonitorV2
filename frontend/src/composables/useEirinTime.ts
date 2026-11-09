import { computed, onMounted, onUnmounted, readonly, ref } from 'vue'
import { getEirinMinutes, formatEirinTime } from '../utils/eirinTime'

const STORAGE_KEY = 'blony-monitor:eirin-time-offset-minutes'
const DEFAULT_OFFSET_MINUTES = -14
const MIN_OFFSET_MINUTES = -720
const MAX_OFFSET_MINUTES = 720

const clockNow = ref(Date.now())
const eirinPhaseOffsetMinutes = ref(DEFAULT_OFFSET_MINUTES)
let initialized = false
let clockTimer: number | undefined
let consumers = 0

function normalizeOffset(value: number) {
  const rounded = Math.round(value)
  if (!Number.isFinite(rounded)) return DEFAULT_OFFSET_MINUTES
  return Math.min(MAX_OFFSET_MINUTES, Math.max(MIN_OFFSET_MINUTES, rounded))
}

function readOffset() {
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY)
    return stored === null ? DEFAULT_OFFSET_MINUTES : normalizeOffset(Number(stored))
  } catch {
    return DEFAULT_OFFSET_MINUTES
  }
}

function persistOffset(value: number) {
  try {
    window.localStorage.setItem(STORAGE_KEY, String(value))
  } catch {
    // Keep the value in memory when storage is unavailable.
  }
}

function initialize() {
  if (initialized) return
  initialized = true
  eirinPhaseOffsetMinutes.value = readOffset()
}

function setEirinPhaseOffset(value: number) {
  const nextValue = normalizeOffset(value)
  eirinPhaseOffsetMinutes.value = nextValue
  persistOffset(nextValue)
}

export function useEirinTime() {
  initialize()
  onMounted(() => {
    clockNow.value = Date.now()
    consumers += 1
    if (clockTimer === undefined) {
      clockTimer = window.setInterval(() => { clockNow.value = Date.now() }, 250)
    }
  })
  onUnmounted(() => {
    consumers -= 1
    if (consumers === 0 && clockTimer !== undefined) {
      window.clearInterval(clockTimer)
      clockTimer = undefined
    }
  })

  const realTimeLabel = computed(() => {
    const now = new Date(clockNow.value)
    return [now.getHours(), now.getMinutes(), now.getSeconds()]
      .map(value => String(value).padStart(2, '0'))
      .join(':')
  })

  const eirinTimeLabelValue = computed(() => formatEirinTime(getEirinMinutes(clockNow.value, eirinPhaseOffsetMinutes.value)))
  const eirinPhaseOffsetLabel = computed(() => {
    const value = eirinPhaseOffsetMinutes.value
    return value > 0 ? `+${value}` : String(value)
  })

  function adjustEirinPhaseOffset(deltaMinutes: number) {
    setEirinPhaseOffset(eirinPhaseOffsetMinutes.value + deltaMinutes)
  }

  function resetEirinPhaseOffset() {
    setEirinPhaseOffset(DEFAULT_OFFSET_MINUTES)
  }

  return {
    clockNow: readonly(clockNow),
    eirinPhaseOffsetMinutes: readonly(eirinPhaseOffsetMinutes),
    eirinPhaseOffsetLabel,
    eirinTimeLabel: eirinTimeLabelValue,
    realTimeLabel,
    adjustEirinPhaseOffset,
    resetEirinPhaseOffset,
  }
}
