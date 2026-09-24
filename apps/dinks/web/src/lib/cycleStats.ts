import type { Dashboard, Period, Symptom } from '@dinks/shared'

/** Plausibility bounds for a cycle gap, matching the backend's domain/period.go EstimateNextPeriod. */
const MIN_PLAUSIBLE_CYCLE_DAYS = 15
const MAX_PLAUSIBLE_CYCLE_DAYS = 60

export function toDate(iso: string) {
  const [y, m, d] = iso.split('-').map(Number)
  return new Date(y, m - 1, d)
}

export function daysBetween(a: string, b: string) {
  return Math.round((toDate(b).getTime() - toDate(a).getTime()) / 86_400_000)
}

/** Chronological list of { start, gapDays } for consecutive period starts within the plausible range. */
export function cycleGaps(periods: Period[]): { start: string; gapDays: number }[] {
  const starts = [...periods].map((p) => p.started_on).sort()
  const gaps: { start: string; gapDays: number }[] = []
  for (let i = 1; i < starts.length; i++) {
    const gapDays = daysBetween(starts[i - 1], starts[i])
    if (gapDays >= MIN_PLAUSIBLE_CYCLE_DAYS && gapDays <= MAX_PLAUSIBLE_CYCLE_DAYS) {
      gaps.push({ start: starts[i], gapDays })
    }
  }
  return gaps
}

export function avgCycleLength(periods: Period[]): number | undefined {
  const gaps = cycleGaps(periods)
  if (gaps.length === 0) return undefined
  return Math.round(gaps.reduce((sum, g) => sum + g.gapDays, 0) / gaps.length)
}

/** Chronological list of { start, lengthDays } for periods that have both a start and an end. */
export function periodLengths(periods: Period[]): { start: string; lengthDays: number }[] {
  return [...periods]
    .filter((p) => p.ended_on)
    .map((p) => ({ start: p.started_on, lengthDays: daysBetween(p.started_on, p.ended_on!) + 1 }))
    .filter((p) => p.lengthDays > 0)
    .sort((a, b) => (a.start < b.start ? -1 : 1))
}

export function avgPeriodLength(periods: Period[]): number | undefined {
  const lengths = periodLengths(periods)
  if (lengths.length === 0) return undefined
  return Math.round((lengths.reduce((sum, p) => sum + p.lengthDays, 0) / lengths.length) * 10) / 10
}

export function symptomBreakdown(symptoms: Symptom[]): { kind: string; count: number; pct: number }[] {
  const counts = new Map<string, number>()
  for (const s of symptoms) counts.set(s.kind, (counts.get(s.kind) ?? 0) + 1)
  const total = symptoms.length
  return [...counts.entries()]
    .sort((a, b) => b[1] - a[1])
    .map(([kind, count]) => ({ kind, count, pct: total > 0 ? Math.round((count / total) * 100) : 0 }))
}

export function cycleStatsFromDashboard(data: Dashboard | undefined) {
  return {
    cycleGaps: cycleGaps(data?.periods ?? []),
    avgCycleLength: avgCycleLength(data?.periods ?? []),
    periodLengths: periodLengths(data?.periods ?? []),
    avgPeriodLength: avgPeriodLength(data?.periods ?? []),
    symptomBreakdown: symptomBreakdown(data?.symptoms ?? []),
  }
}
