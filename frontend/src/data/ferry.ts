import { EIRIN_DAY_MS, EIRIN_MINUTE_MS, getEirinDayStart } from '../utils/eirinTime'

export interface FerryGroup {
  id: string
  name: string
  routes: string[]
  // Boarding, departure, arrival in Eirin minutes; arrival may cross midnight.
  schedule: readonly (readonly [number, number, number])[]
}

export const FERRY_GROUPS: readonly FerryGroup[] = [
  {
    id: 'ceann', name: '凯安港口系',
    routes: ['凯安港口 ↔ 肯努斯港口', '凯安港口 ↔ 塞拉港口'],
    schedule: [
      [0, 120, 270], [270, 390, 540], [540, 660, 810],
      [810, 930, 1080], [1080, 1200, 1350],
    ],
  },
  {
    id: 'qilla', name: '克拉港口系',
    routes: ['克拉港口 ↔ 卡普港口'],
    schedule: [
      [60, 180, 330], [330, 450, 600], [600, 720, 870],
      [870, 990, 1140], [1140, 1260, 1410],
    ],
  },
  {
    id: 'belvast', name: '贝岛港口系',
    routes: ['贝尔法斯特 ↔ 卡普港口'],
    schedule: [
      [60, 150, 230], [240, 330, 410], [420, 510, 590], [600, 690, 770],
      [780, 870, 950], [960, 1050, 1130], [1140, 1230, 1310], [1320, 1410, 1490],
    ],
  },
]

export interface FerrySailing {
  boarding: number
  departure: number
  arrival: number
}

export type FerryStatus = 'waiting' | 'boarding' | 'sailing'

export function getSailingStatus(sailing: FerrySailing, now: number): FerryStatus {
  if (now < sailing.boarding) return 'waiting'
  return now < sailing.departure ? 'boarding' : 'sailing'
}

export function getUpcomingSailings(group: FerryGroup, now: number, offsetMinutes: number): FerrySailing[] {
  const dayStart = getEirinDayStart(now, offsetMinutes)
  const sailings: FerrySailing[] = []
  // Include yesterday's last sailing, which can arrive after Eirin midnight.
  for (let day = -1; day <= 2; day++) {
    const start = dayStart + day * EIRIN_DAY_MS
    for (const [boarding, departure, arrival] of group.schedule) {
      const sailing = {
        boarding: start + boarding * EIRIN_MINUTE_MS,
        departure: start + departure * EIRIN_MINUTE_MS,
        arrival: start + arrival * EIRIN_MINUTE_MS,
      }
      if (sailing.arrival > now) sailings.push(sailing)
    }
  }
  return sailings.sort((a, b) => a.boarding - b.boarding).slice(0, group.schedule.length)
}

export function formatCountdown(milliseconds: number): string {
  const seconds = Math.max(0, Math.ceil(milliseconds / 1000))
  return `${Math.floor(seconds / 60)}分${String(seconds % 60).padStart(2, '0')}秒`
}

export function formatRealTime(timestamp: number): string {
  const date = new Date(timestamp)
  return [date.getHours(), date.getMinutes(), date.getSeconds()]
    .map(value => String(value).padStart(2, '0')).join(':')
}

export function realDateLabel(timestamp: number, now: number): string {
  const date = new Date(timestamp)
  const today = new Date(now)
  if (date.toDateString() === today.toDateString()) return ''
  return `${date.getMonth() + 1}/${date.getDate()} `
}
