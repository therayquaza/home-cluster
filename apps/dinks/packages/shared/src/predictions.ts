import type { Period } from './index.ts'
import { toDate, toISO, todayISO } from './dates.ts'
import { avgCycleLength, avgPeriodLength, daysBetween } from './cycleStats.ts'
import { LUTEAL_DAYS } from './phase.ts'

/**
 * Cycle predictions shared by web and mobile: the *window* of days the next
 * period is expected to cover, and the ovulation days around that cycle's most
 * probable day. Neither is a medical or contraceptive claim — they are the same
 * average-over-recent-cycles estimate the backend already publishes as
 * `next_period`, widened so the UI can show a span rather than a single day.
 */

/** Used when no completed period has been logged yet, so there is no average. */
export const PERIOD_LENGTH_FALLBACK_DAYS = 5
/** Ovulation is shown as the most probable day plus this many days either side. */
export const OVULATION_DAYS_BEFORE_PEAK = 2
export const OVULATION_DAYS_AFTER_PEAK = 2
export const PREDICTION_DISCLAIMER = 'Estimates are not medical advice.'

/** A contiguous run of days, e.g. a predicted period or the ovulation window. */
export type DateWindow = { start: string; end: string; days: string[] }
export type OvulationPrediction = { peak: string; days: string[] }
export type CyclePredictions = { nextPeriod?: DateWindow; ovulation?: OvulationPrediction }

/** ISO day shifted by whole days, using the local calendar so "day" means one day. */
export function shiftISO(iso: string, days: number): string {
  const date = toDate(iso)
  return toISO(new Date(date.getFullYear(), date.getMonth(), date.getDate() + days))
}

/** Every ISO day from start to end inclusive. */
export function isoDays(start: string, end: string): string[] {
  const out: string[] = []
  let cursor = toDate(start)
  const last = toDate(end)
  while (cursor <= last) {
    out.push(toISO(cursor))
    cursor = new Date(cursor.getFullYear(), cursor.getMonth(), cursor.getDate() + 1)
  }
  return out
}

function sortedStarts(periods: Period[]): string[] {
  return periods
    .map((p) => p.started_on)
    .filter((s) => !!s)
    .sort()
}

/**
 * The predicted start of the next period. Prefers the server's `next_period` so
 * every client agrees with the API; otherwise mirrors the backend's
 * EstimateNextPeriod: average the plausible cycle gaps, project from the last
 * start, and roll forward if that lands in the past (a member can be overdue).
 */
export function nextPeriodStart(
  periods: Period[],
  provided: string | undefined,
  todayIso: string = todayISO(),
): string | undefined {
  if (provided) return provided
  const starts = sortedStarts(periods)
  const last = starts[starts.length - 1]
  const avg = avgCycleLength(periods)
  if (!last || avg === undefined || avg <= 0) return undefined
  let next = shiftISO(last, avg)
  let guard = 0
  // A guard keeps a pathological avg (e.g. 0) from spinning; the backend's
  // 15-60 day window means this runs at most a handful of times in practice.
  while (next <= todayIso && guard++ < 240) next = shiftISO(next, avg)
  return next
}

/** The predicted next period as a start–end window using the average period length. */
export function predictNextPeriod(
  periods: Period[],
  provided: string | undefined,
  todayIso: string = todayISO(),
): DateWindow | undefined {
  const start = nextPeriodStart(periods, provided, todayIso)
  if (!start) return undefined
  const lengthDays = Math.max(1, Math.round(avgPeriodLength(periods) ?? PERIOD_LENGTH_FALLBACK_DAYS))
  const end = shiftISO(start, lengthDays - 1)
  return { start, end, days: isoDays(start, end) }
}

/**
 * The ovulation days for the cycle that ends at the predicted next period.
 *
 * The most probable day is `LUTEAL_DAYS` before the next start, matching the
 * day phase.ts reports as "ovulatory" (phase.ts counts cycle days from 1, so
 * that day sits LUTEAL_DAYS + 1 calendar days before the next start). The
 * surrounding window is shown as an estimate, never the peak on its own.
 */
export function predictOvulation(
  periods: Period[],
  provided: string | undefined,
  todayIso: string = todayISO(),
): OvulationPrediction | undefined {
  const start = nextPeriodStart(periods, provided, todayIso)
  if (!start) return undefined
  const starts = sortedStarts(periods)
  const last = starts[starts.length - 1]
  if (!last) return undefined
  const cycleLength = daysBetween(last, start)
  if (cycleLength <= LUTEAL_DAYS) return undefined
  const peak = shiftISO(last, cycleLength - LUTEAL_DAYS - 1)
  return {
    peak,
    days: isoDays(shiftISO(peak, -OVULATION_DAYS_BEFORE_PEAK), shiftISO(peak, OVULATION_DAYS_AFTER_PEAK)),
  }
}

/** Both predictions in one pass, for the screens that render them together. */
export function predictCycle(
  periods: Period[],
  nextPeriod: string | undefined,
  todayIso: string = todayISO(),
): CyclePredictions {
  return {
    nextPeriod: predictNextPeriod(periods, nextPeriod, todayIso),
    ovulation: predictOvulation(periods, nextPeriod, todayIso),
  }
}
