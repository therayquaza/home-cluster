import type { Period, Symptom } from '@dinks/shared'
import { toDate } from './dates'

const MIN_PLAUSIBLE_CYCLE_DAYS = 15
const MAX_PLAUSIBLE_CYCLE_DAYS = 60

export function daysBetween(a: string, b: string) {
  return Math.round((toDate(b).getTime() - toDate(a).getTime()) / 86_400_000)
}

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

export function periodLengths(periods: Period[]): { start: string; lengthDays: number }[] {
  return [...periods]
    .filter((p) => p.ended_on)
    .map((p) => ({ start: p.started_on, lengthDays: daysBetween(p.started_on, p.ended_on!) + 1 }))
    .filter((p) => p.lengthDays > 0)
    .sort((a, b) => (a.start < b.start ? -1 : 1))
}

export function symptomBreakdown(symptoms: Symptom[]): { kind: string; count: number; pct: number }[] {
  const counts = new Map<string, number>()
  for (const s of symptoms) counts.set(s.kind, (counts.get(s.kind) ?? 0) + 1)
  const total = symptoms.length
  return [...counts.entries()]
    .sort((a, b) => b[1] - a[1])
    .map(([kind, count]) => ({ kind, count, pct: total > 0 ? Math.round((count / total) * 100) : 0 }))
}
