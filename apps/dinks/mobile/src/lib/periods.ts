import type { Period } from '@dinks/shared'
import { toDate, toISO } from './dates'

export function periodEndISO(period: Period, todayIso: string): string {
  return period.ended_on ?? todayIso
}

export function isPeriodDay(period: Period, iso: string, todayIso: string): boolean {
  return iso >= period.started_on && iso <= periodEndISO(period, todayIso)
}

export function periodOnDay(periods: Period[], iso: string, todayIso: string): Period | undefined {
  return periods.find((p) => isPeriodDay(p, iso, todayIso))
}

export function periodDaySet(periods: Period[], todayIso: string): Set<string> {
  const set = new Set<string>()
  for (const p of periods) {
    let cursor = toDate(p.started_on)
    const end = toDate(periodEndISO(p, todayIso))
    while (cursor <= end) {
      set.add(toISO(cursor))
      cursor = new Date(cursor.getFullYear(), cursor.getMonth(), cursor.getDate() + 1)
    }
  }
  return set
}
