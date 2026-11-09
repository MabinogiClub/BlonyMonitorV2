import { EIRIN_DAY_MS, EIRIN_MINUTE_MS, getEirinDayStart } from './eirinTime'

export type SmugglerPhase = 'forecast' | 'appear' | 'disappear_forecast' | 'absent'
export interface SmugglerNotice {
  type: Exclude<SmugglerPhase, 'absent'>
  area: string
  item: string
  receivedAt: number
}

export function getSmugglerWindow(now: number, offset: number) {
  const dayStart = getEirinDayStart(now, offset)
  const minute = (now - dayStart) / EIRIN_MINUTE_MS
  const forecastAt = dayStart + (minute < 360 ? -300 : 1140) * EIRIN_MINUTE_MS
  const appearAt = forecastAt + 180 * EIRIN_MINUTE_MS
  const goneAt = forecastAt + 660 * EIRIN_MINUTE_MS
  const endingAt = forecastAt + 480 * EIRIN_MINUTE_MS
  const phase: SmugglerPhase = now < forecastAt ? 'absent'
    : now < appearAt ? 'forecast' : now < endingAt ? 'appear' : 'disappear_forecast'
  return { phase, forecastAt, appearAt, goneAt }
}

export function isCurrentSmugglerNotice(notice: SmugglerNotice | null, now: number, offset: number): boolean {
  const cycle = getSmugglerWindow(now, offset)
  if (!notice || cycle.phase === 'absent' || notice.receivedAt > now || now - notice.receivedAt >= EIRIN_DAY_MS) return false
  const phaseMinutes = { forecast: 1140, appear: 1320, disappear_forecast: 1620 }[notice.type]
  if (phaseMinutes === undefined) return false
  // Associate an announcement with the nearest scheduled phase using the
  // calibrated clock, so yesterday's location cannot leak into this cycle.
  const noticeDay = getEirinDayStart(notice.receivedAt, offset)
  const eventAt = noticeDay + phaseMinutes * EIRIN_MINUTE_MS
  const nearestDay = Math.round((notice.receivedAt - eventAt) / EIRIN_DAY_MS)
  const forecastAt = eventAt + nearestDay * EIRIN_DAY_MS - (phaseMinutes - 1140) * EIRIN_MINUTE_MS
  return forecastAt === cycle.forecastAt
}

export interface TradeProduct {
  name: string
  area: string
  img: string
  price: number
  weight: number
  boxCount: number
  level: number
}

export interface TradeVehicle {
  name: string
  block: number
  weight: number
  speed: number
}

export function calculateVehicleLoads(product: TradeProduct, vehicles: readonly TradeVehicle[]) {
  return vehicles.map(vehicle => {
    const byWeight = product.weight > 0 ? Math.floor(vehicle.weight / product.weight) : 0
    const totalItems = Math.max(0, Math.min(vehicle.block * product.boxCount, byWeight))
    return {
      ...vehicle,
      actualSlots: totalItems > 0 ? Math.ceil(totalItems / product.boxCount) : 0,
      totalItems,
      totalCost: totalItems * product.price,
    }
  }).sort((a, b) => b.totalCost - a.totalCost || b.speed - a.speed)
}
