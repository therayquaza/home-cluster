import type { Period } from '@dinks/shared'
import { toDate, toISO } from './dates'

/**
 * An ongoing period (no ended_on) only counts as covering days up to today —
 * never future days. Every period/day check in the app goes through this so an
 * open-ended period can't paint the rest of the calendar.
 */
export function periodEndISO(period: Period, todayIso: string): string {
  return period.ended_on ?? todayIso
}

export function isPeriodDay(period: Period, iso: string, todayIso: string): boolean {
  return iso >= period.started_on && iso <= periodEndISO(period, todayIso)
}

export function periodOnDay(periods: Period[], iso: string, todayIso: string): Period | undefined {
  return periods.find((p) => isPeriodDay(p, iso, todayIso))
}

/** Every ISO date covered by any period, capped at today for ongoing ones. */
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

/** The flow recorded for one day of a period, falling back to the period's summary. */
export function flowOnDay(period: Period, iso: string): string {
  return period.days?.find((d) => d.date === iso)?.flow ?? period.flow
}

/** ISO date -> flow for every day of a period, so the calendar can paint each day differently. */
export function flowDayMap(period: Period, todayIso: string): Map<string, string> {
  const map = new Map<string, string>()
  const explicit = new Map((period.days ?? []).map((d) => [d.date, d.flow]))
  let cursor = toDate(period.started_on)
  const end = toDate(periodEndISO(period, todayIso))
  while (cursor <= end) {
    const iso = toISO(cursor)
    map.set(iso, explicit.get(iso) ?? period.flow)
    cursor = new Date(cursor.getFullYear(), cursor.getMonth(), cursor.getDate() + 1)
  }
  return map
}

/** Every ISO date -> flow across all periods, for the calendar's per-day styling. */
export function flowByDate(periods: Period[], todayIso: string): Map<string, string> {
  const out = new Map<string, string>()
  for (const p of periods) {
    for (const [iso, flow] of flowDayMap(p, todayIso)) out.set(iso, flow)
  }
  return out
}
