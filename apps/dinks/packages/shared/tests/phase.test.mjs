import test from 'node:test'
import assert from 'node:assert/strict'
import { phaseOn, symptomsByPhase } from '../src/phase.ts'

// A 28-day cycle: 2026-03-02 → 2026-03-06 (5 days), next period 2026-03-30.
const P = [
  { id: 1, started_on: '2026-03-02', ended_on: '2026-03-06', flow: 'medium' },
  { id: 2, started_on: '2026-03-30', flow: 'medium' },
]

test('days inside a period are menstrual', () => {
  assert.equal(phaseOn('2026-03-02', P).phase, 'menstrual')
  assert.equal(phaseOn('2026-03-06', P).phase, 'menstrual')
})

test('the phases after a period ends are still in that cycle', () => {
  // The regression this guards: a date must belong to the cycle the last period
  // started, not require falling inside that period's own days.
  assert.equal(phaseOn('2026-03-07', P).phase, 'follicular')
  assert.equal(phaseOn('2026-03-15', P).phase, 'ovulatory')
  assert.equal(phaseOn('2026-03-16', P).phase, 'luteal')
  assert.equal(phaseOn('2026-03-29', P).phase, 'luteal')
})

test('cycle day and length come from the observed gap', () => {
  const p = phaseOn('2026-03-15', P)
  assert.equal(p.cycleDay, 14)
  assert.equal(p.cycleLength, 28)
})

test('a date before any period has no phase', () => {
  // Claiming a phase here would mean inventing a cycle to hang it on.
  assert.equal(phaseOn('2026-03-01', P).phase, 'unknown')
  assert.equal(phaseOn('2026-03-01', P).cycleDay, 0)
})

test('a single period falls back to the average cycle length', () => {
  const one = [{ id: 1, started_on: '2026-03-02', ended_on: '2026-03-06', flow: 'light' }]
  assert.equal(phaseOn('2026-03-04', one).phase, 'menstrual')
  // No second period to measure against, so the default 28-day average places
  // ovulation on day 14 (2026-03-15).
  assert.equal(phaseOn('2026-03-15', one).cycleLength, 28)
  assert.equal(phaseOn('2026-03-15', one).phase, 'ovulatory')
})

test('an ongoing period counts today as menstrual', () => {
  const today = new Date().toISOString().slice(0, 10)
  const ongoing = [{ id: 1, started_on: today, flow: 'heavy' }]
  assert.equal(phaseOn(today, ongoing).phase, 'menstrual')
  assert.equal(phaseOn(today, ongoing).cycleDay, 1)
})

test('a custom average cycle length shifts the phase boundaries', () => {
  const one = [{ id: 1, started_on: '2026-03-02', ended_on: '2026-03-06', flow: 'light' }]
  // On a 35-day average, ovulation moves to day 21 (2026-03-22), so day 14 is
  // no longer ovulatory.
  assert.equal(phaseOn('2026-03-15', one, 35).phase, 'follicular')
  assert.equal(phaseOn('2026-03-22', one, 35).phase, 'ovulatory')
  assert.equal(phaseOn('2026-03-25', one, 35).phase, 'luteal')
})

test('an ongoing period cannot place today past the end of its cycle', () => {
  const today = new Date().toISOString().slice(0, 10)
  const ongoing = [{ id: 1, started_on: today, flow: 'heavy' }]
  const p = phaseOn(today, ongoing)
  assert.ok(p.cycleDay < p.cycleLength, `cycleDay ${p.cycleDay} must be < cycleLength ${p.cycleLength}`)
})

test('byPhase counts a repeated same-day symptom once', () => {
  const rows = symptomsByPhase(
    [
      { id: 1, recorded_on: '2026-03-02', kind: 'Cramps', severity: 3 },
      { id: 2, recorded_on: '2026-03-02', kind: 'Cramps', severity: 4 },
      { id: 3, recorded_on: '2026-03-02', kind: 'Fatigue', severity: 2 },
    ],
    P,
  )
  const menstrual = rows.get('menstrual')
  assert.equal(menstrual.find((r) => r.label === 'Cramps').count, 1)
})

test('byPhase percentages count logged days, not check-ins', () => {
  // Two symptoms on one day must not double that day's weight in the
  // denominator, or a chatty day would dominate the phase.
  const rows = symptomsByPhase(
    [
      { id: 1, recorded_on: '2026-03-02', kind: 'Cramps', severity: 3 },
      { id: 2, recorded_on: '2026-03-02', kind: 'Fatigue', severity: 2 },
      { id: 3, recorded_on: '2026-03-03', kind: 'Fatigue', severity: 2 },
    ],
    P,
  )
  const menstrual = rows.get('menstrual')
  // Two logged days in the phase. Cramps on 1 of them, Fatigue on both.
  assert.equal(menstrual.find((r) => r.label === 'Cramps').pct, 50)
  assert.equal(menstrual.find((r) => r.label === 'Fatigue').pct, 100)
})

test('byPhase leaves a phase empty rather than dividing by zero', () => {
  const rows = symptomsByPhase([{ id: 1, recorded_on: '2026-03-02', kind: 'Cramps', severity: 3 }], P)
  for (const [, entries] of rows) {
    for (const r of entries) assert.ok(Number.isFinite(r.pct))
  }
  assert.deepEqual(rows.get('ovulatory'), [])
})
