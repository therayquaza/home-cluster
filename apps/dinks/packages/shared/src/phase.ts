import type { Period, Symptom } from './index.ts'
import { isSymptomKind } from './symptoms.ts'

export type PhaseName = 'menstrual' | 'follicular' | 'ovulatory' | 'luteal' | 'unknown'

export const PHASES: { key: PhaseName; label: string; blurb: string }[] = [
  { key: 'menstrual', label: 'Menstrual', blurb: 'Day 1 to the end of your period.' },
  { key: 'follicular', label: 'Follicular', blurb: 'After your period ends, up to ovulation.' },
  { key: 'ovulatory', label: 'Ovulatory', blurb: 'The days around ovulation.' },
  { key: 'luteal', label: 'Luteal', blurb: 'After ovulation, until your next period.' },
]

/** The luteal phase is conventionally ~14 days, which places ovulation too. */
export const LUTEAL_DAYS = 14

export type DayPhase = { day: number; phase: PhaseName; cycleDay: number; cycleLength: number }

const DAY = 86_400_000

/**
 * phaseOn derives where in the cycle a date falls, from the recorded periods.
 *
 * A date belongs to the cycle that the most recent period *started*, not to a
 * period whose own days contain it — the follicular, ovulatory and luteal phases
 * all fall after a period ends, so requiring containment would report "unknown"
 * for most of every cycle.
 *
 * The cycle length is the observed gap from that start to the next one, falling
 * back to an average of recent cycles. When no period has started on or before
 * the date, no phase is claimed: a phase derived from a guessed cycle length
 * would be a medical-sounding claim built on nothing.
 */
export function phaseOn(date: string, periods: Period[], avgCycleDays = 28, avgPeriodDays = 5): DayPhase {
  const starts = [...periods]
    .map((p) => p.started_on)
    .filter((started) => started <= date)
    .sort()
  const start = starts[starts.length - 1]
  if (start === undefined) return { day: 0, phase: 'unknown', cycleDay: 0, cycleLength: 0 }

  const cycleDay = daysBetween(start, date) + 1

  // The cycle length is only known once the next period has started; until then
  // the average is the best estimate available.
  const next = periods.map((p) => p.started_on).filter((s) => s > start).sort()
  const observed = next[0] ? daysBetween(start, next[0]) : 0
  let cycleLength = observed > 0 ? observed : avgCycleDays > 0 ? avgCycleDays : 28
  if (cycleLength <= cycleDay) cycleLength = cycleDay + 1 // never place today past the end

  // The menstrual phase is this cycle's own period length when it has been
  // recorded, otherwise the average. An ongoing period is still inside it, which
  // this overstates once it runs longer than the average — adding an end date
  // corrects that.
  const period = periods.find((p) => p.started_on === start)
  const periodLength = period?.ended_on
    ? daysBetween(start, period.ended_on) + 1
    : avgPeriodDays > 0
      ? avgPeriodDays
      : 5

  let phase: PhaseName
  if (cycleDay <= periodLength) phase = 'menstrual'
  else if (cycleDay === cycleLength - LUTEAL_DAYS) phase = 'ovulatory'
  else if (cycleDay > cycleLength - LUTEAL_DAYS) phase = 'luteal'
  else phase = 'follicular'

  return { day: cycleDay, phase, cycleDay, cycleLength }
}

function daysBetween(from: string, to: string): number {
  return Math.round((Date.parse(to + 'T00:00:00Z') - Date.parse(from + 'T00:00:00Z')) / DAY)
}

export type PhaseRow = { label: string; count: number; pct: number }

/**
 * symptomsByPhase counts how often each symptom falls in each phase, which is
 * the question the removed Insights page answered: "is this thing worse before
 * my period?". Percentages are within-phase so a phase with more check-ins does
 * not simply win every row.
 */
export function symptomsByPhase(symptoms: Symptom[], periods: Period[], avgCycleDays = 28): Map<PhaseName, PhaseRow[]> {
  const out = new Map<PhaseName, PhaseRow[]>()
  for (const p of PHASES) out.set(p.key, [])

  const seen = new Set<string>()
  // Distinct logged days per phase, so the denominator counts days rather than
  // check-ins — a day with three symptoms must not triple a phase's weight.
  const phaseDays = new Map<PhaseName, Set<string>>()
  for (const s of symptoms) {
    // A day's note and a custom indicator's value are check-ins, not symptoms;
    // counting them would make "note" the top row of every phase.
    if (!isSymptomKind(s.kind)) continue
    const key = `${s.kind}|${s.recorded_on}`
    if (seen.has(key)) continue
    seen.add(key)

    const { phase } = phaseOn(s.recorded_on, periods, avgCycleDays)
    if (phase === 'unknown') continue

    const rows = out.get(phase)!
    const row = rows.find((r) => r.label === s.kind)
    if (row) row.count += 1
    else rows.push({ label: s.kind, count: 1, pct: 0 })

    if (!phaseDays.has(phase)) phaseDays.set(phase, new Set())
    phaseDays.get(phase)!.add(s.recorded_on)
  }

  for (const [phase, rows] of out) {
    const days = phaseDays.get(phase)?.size ?? 0
    for (const r of rows) {
      r.pct = days > 0 ? Math.round((r.count / days) * 100) : 0
    }
    rows.sort((a, b) => b.count - a.count || a.label.localeCompare(b.label))
  }
  return out
}
