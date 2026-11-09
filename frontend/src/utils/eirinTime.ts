// One Eirin minute lasts 1.5 real seconds; one Eirin day lasts 36 real minutes.
export const EIRIN_MINUTE_MS = 1500
export const EIRIN_DAY_MINUTES = 1440
export const EIRIN_DAY_MS = EIRIN_DAY_MINUTES * EIRIN_MINUTE_MS
const EIRIN_ANCHOR_MS = 8 * 60 * 60 * 1000

export function getEirinDayStart(now: number, offsetMinutes: number): number {
  const phase = EIRIN_ANCHOR_MS + offsetMinutes * EIRIN_MINUTE_MS
  return Math.floor((now + phase) / EIRIN_DAY_MS) * EIRIN_DAY_MS - phase
}

export function getEirinMinutes(now: number, offsetMinutes: number): number {
  return Math.floor((now - getEirinDayStart(now, offsetMinutes)) / EIRIN_MINUTE_MS)
}

export function formatEirinTime(minutes: number): string {
  const time = ((minutes % EIRIN_DAY_MINUTES) + EIRIN_DAY_MINUTES) % EIRIN_DAY_MINUTES
  return `${String(Math.floor(time / 60)).padStart(2, '0')}:${String(time % 60).padStart(2, '0')}`
}
