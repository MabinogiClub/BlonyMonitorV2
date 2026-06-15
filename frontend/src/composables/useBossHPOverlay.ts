import { useAppStore } from '../stores/app'

interface SampledPoint {
  time: number
  percent: number
  threshold?: number
  locked?: boolean
  endpoint?: 'start' | 'end'
}

interface HPHistoryRecord {
  entityId?: string
  hptimestamp: number
  currentHp: number
  maxHp?: number
  percent: number
  threshold?: number
  locked?: boolean
}

const PERCENT_THRESHOLD = 5.0
const LOCK_BAND_PERCENT = 0.2
const LOCK_STEP_PERCENT = 5
const LOCK_MIN_HOLD_MS = 500
const LOCK_MIN_DROP_PERCENT = 1
const LOCK_MIN_OVERFLOW_PERCENT = 0.5

function sampleHistory(
  history: HPHistoryRecord[],
  chartMinTime: number,
  chartMaxTime: number,
  forceEndAtZero = false
): SampledPoint[] {
  if (!history || history.length === 0) return []

  const allSampled: SampledPoint[] = []
  const chartStart = chartMinTime / 10
  const chartEnd = chartMaxTime / 10
  const sorted = [...history].sort((a, b) => a.hptimestamp - b.hptimestamp)
  let lastKeptPercent = 100
  let lastSeenPercent = 100
  const shownLockThresholds = new Set<number>()
  let prevRecord: HPHistoryRecord | null = null

  allSampled.push({ time: chartStart, percent: 100, endpoint: 'start' })

  for (let i = 0; i < sorted.length; i++) {
    const curr = sorted[i]
    if (curr.hptimestamp < chartStart) {
      lastSeenPercent = curr.percent
      prevRecord = curr
      continue
    }

    if (curr.hptimestamp > chartEnd) break

    const deltaPercent = Math.abs(lastKeptPercent - curr.percent)

    const lockThreshold = getLockThreshold(curr) ?? getInferredLockThreshold(sorted, i, prevRecord)
    const shouldShowLockLabel = lockThreshold !== undefined && !shownLockThresholds.has(lockThreshold)
    if (deltaPercent >= PERCENT_THRESHOLD || shouldShowLockLabel) {
      allSampled.push({
        time: curr.hptimestamp,
        percent: curr.percent,
        threshold: shouldShowLockLabel ? lockThreshold : undefined,
        locked: shouldShowLockLabel
      })
      if (shouldShowLockLabel) {
        shownLockThresholds.add(lockThreshold)
      }
      lastKeptPercent = curr.percent
    }

    lastSeenPercent = curr.percent
    prevRecord = curr
  }

  const result: SampledPoint[] = []
  for (const p of allSampled) {
    const scaledTime = p.time * 10
    if (scaledTime >= chartMinTime && scaledTime <= chartMaxTime) {
      result.push(p)
    }
  }

  const endPercent = forceEndAtZero ? 0 : lastSeenPercent
  if (result.length === 0 || result[result.length - 1].time !== chartEnd || result[result.length - 1].percent !== endPercent) {
    result.push({ time: chartEnd, percent: endPercent, endpoint: forceEndAtZero ? 'end' : undefined })
  } else if (forceEndAtZero && result[result.length - 1].percent === 0) {
    result[result.length - 1].endpoint = 'end'
  }

  return result
}

function getLockThreshold(record: Pick<HPHistoryRecord, 'percent' | 'threshold' | 'locked'>): number | undefined {
  if (record.threshold && record.threshold > 0) {
    return record.threshold
  }
  if (record.locked) {
    return getRoundedLockThreshold(record.percent) ?? Math.round(record.percent)
  }
  return undefined
}

function getInferredLockThreshold(records: HPHistoryRecord[], index: number, prev: HPHistoryRecord | null): number | undefined {
  const curr = records[index]
  const threshold = getRoundedLockThreshold(curr.percent)
  if (threshold === undefined || !prev || curr.hptimestamp <= prev.hptimestamp) {
    return undefined
  }

  if (prev.percent - curr.percent < LOCK_MIN_DROP_PERCENT) {
    return undefined
  }

  if (getOverflowPercent(records, index) >= LOCK_MIN_OVERFLOW_PERCENT) {
    return threshold
  }

  let heldUntil = curr.hptimestamp
  for (let i = index + 1; i < records.length; i++) {
    const next = records[i]
    if (Math.abs(next.percent - curr.percent) <= LOCK_BAND_PERCENT && Math.abs(next.currentHp - curr.currentHp) < 1) {
      heldUntil = Math.max(heldUntil, next.hptimestamp)
      continue
    }
    break
  }

  if ((heldUntil - curr.hptimestamp) * 10 < LOCK_MIN_HOLD_MS) {
    return undefined
  }

  return threshold
}

function getOverflowPercent(records: HPHistoryRecord[], index: number): number {
  const curr = records[index]
  const prev = findPreviousDifferentHP(records, index)
  if (!prev || curr.maxHp === undefined || curr.maxHp <= 0) {
    return 0
  }

  const actualDrop = prev.currentHp - curr.currentHp
  if (actualDrop <= 0) {
    return 0
  }

  const damageDrop = sumDamageDropAtTimestamp(records, curr.entityId, curr.hptimestamp)
  if (damageDrop <= actualDrop) {
    return 0
  }

  return ((damageDrop - actualDrop) / curr.maxHp) * 100
}

function findPreviousDifferentHP(records: HPHistoryRecord[], index: number): HPHistoryRecord | undefined {
  const curr = records[index]
  for (let i = index - 1; i >= 0; i--) {
    const prev = records[i]
    if (prev.entityId === curr.entityId && Math.abs(prev.currentHp - curr.currentHp) >= 1) {
      return prev
    }
  }
  return undefined
}

function sumDamageDropAtTimestamp(records: HPHistoryRecord[], entityId: string | undefined, timestamp: number): number {
  if (!entityId) {
    return 0
  }

  let beforeHP: number | undefined
  let afterHP: number | undefined
  for (const record of records) {
    if (record.entityId !== entityId || record.hptimestamp !== timestamp) {
      continue
    }
    if (beforeHP === undefined || record.currentHp > beforeHP) {
      beforeHP = record.currentHp
    }
    if (afterHP === undefined || record.currentHp < afterHP) {
      afterHP = record.currentHp
    }
  }

  if (beforeHP === undefined || afterHP === undefined) {
    return 0
  }
  return beforeHP - afterHP
}

function getRoundedLockThreshold(percent: number): number | undefined {
  const threshold = Math.round(percent / LOCK_STEP_PERCENT) * LOCK_STEP_PERCENT
  if (threshold <= 0 || threshold >= 100) {
    return undefined
  }
  if (Math.abs(percent - threshold) > LOCK_BAND_PERCENT) {
    return undefined
  }
  return threshold
}

export function drawBossHPOverlay(
  ctx: CanvasRenderingContext2D,
  chartLeft: number,
  chartRight: number,
  chartTop: number,
  chartBottom: number,
  minTime: number,
  maxTime: number,
  targetName?: string,
  forceEndAtZero = false
) {
  const appStore = useAppStore()
  const bossHPData = appStore.bossHPHistoryData

  if (!bossHPData || bossHPData.length === 0) return

  const timeRange = maxTime - minTime
  if (timeRange <= 0) return

  const chartWidth = chartRight - chartLeft
  const chartHeight = chartBottom - chartTop

  ctx.save()

  ctx.beginPath()
  ctx.rect(chartLeft - 24, chartTop - 18, chartWidth + 48, chartHeight + 18)
  ctx.clip()

  ctx.strokeStyle = 'rgba(244, 67, 54, 0.7)'
  ctx.lineWidth = 1.5
  ctx.beginPath()
  ctx.moveTo(chartLeft, chartTop)
  ctx.lineTo(chartRight, chartTop)
  ctx.stroke()

  const shownLabels = new Set<string>()

  for (const boss of bossHPData) {
    if (!boss.history || boss.history.length === 0) continue

    const sampled = sampleHistory(boss.history, minTime, maxTime, forceEndAtZero)
    if (sampled.length < 2) continue

    for (const p of sampled) {
      const x = chartLeft + ((p.time * 10 - minTime) / timeRange) * chartWidth
      const isLockPoint = !!p.locked

      if (isLockPoint) {
        ctx.strokeStyle = 'rgba(244, 67, 54, 0.55)'
        ctx.fillStyle = 'rgba(244, 67, 54, 0.85)'
      } else {
        ctx.strokeStyle = 'rgba(255, 255, 255, 0.35)'
        ctx.fillStyle = 'rgba(255, 255, 255, 0.65)'
      }

      ctx.lineWidth = 1
      ctx.setLineDash([6, 8])
      ctx.beginPath()
      ctx.moveTo(x, chartTop)
      ctx.lineTo(x, chartBottom)
      ctx.stroke()
      ctx.setLineDash([])

      const endpointPercent = p.endpoint === 'start' ? 100 : p.endpoint === 'end' ? 0 : undefined
      const shouldShowLabel = isLockPoint || endpointPercent !== undefined

      if (shouldShowLabel) {
        const labelPercent = endpointPercent ?? p.threshold ?? p.percent
        const labelKey = `${Math.round(labelPercent)}:${Math.round(x)}`
        if (!shownLabels.has(labelKey)) {
          shownLabels.add(labelKey)

          const labelX = Math.max(chartLeft + 6, Math.min(x, chartRight - 6))
          ctx.font = 'bold 10px Microsoft YaHei'
          ctx.textAlign = labelX <= chartLeft + 6 ? 'left' : labelX >= chartRight - 6 ? 'right' : 'center'
          ctx.fillStyle = endpointPercent === undefined ? 'rgba(244, 67, 54, 0.9)' : 'rgba(255, 82, 82, 0.95)'
          ctx.fillText(`${Math.round(labelPercent)}%`, labelX, chartTop - 4)
        }
      }

      ctx.fillStyle = 'rgba(244, 67, 54, 0.6)'
      ctx.fillRect(x - 3, chartBottom - 6, 6, 6)
    }
  }

  ctx.restore()
}
