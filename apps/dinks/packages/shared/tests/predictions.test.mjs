import test from 'node:test'
import assert from 'node:assert/strict'
import { predictCycle, predictNextPeriod, predictOvulation, shiftISO, isoDays } from '../src/predictions.ts'

// 28-day cycle: 2026-03-02 → 2026-03-06 (5 days), next start 2026-03-30.
const P = [
  { id: 1, started_on: '2026-03-02', ended_on: '2026-03-06', flow: 'medium' },
  { id: 2, started_on: '2026-03-30', ended_on: '2026-04-03', flow: 'medium' },
]

test('shiftISO moves across month and year boundaries', () => {
  assert.equal(shiftISO('2026-03-30', -15), '2026-03-15')
  assert.equal(shiftISO('2026-12-30', 5), '2027-01-04')
})

test('isoDays enumerates an inclusive range', () => {
  assert.deepEqual(isoDays('2026-03-30', '2026-04-02'), ['2026-03-30', '2026-03-31', '2026-04-01', '2026-04-02'])
})

test('the next period is a window the length of the average period', () => {
  // Avg completed length is 5 days, so 04-27 (next start) → 05-01.
  const window = predictNextPeriod(P, '2026-04-27', '2026-04-01')
  assert.equal(window.start, '2026-04-27')
  assert.equal(window.end, '2026-05-01')
  assert.equal(window.days.length, 5)
})

test('the next period falls back to the average gap when the server omits it', () => {
  // Two starts 28 days apart, latest 2026-03-30 → next 04-27; today is well before.
  const window = predictNextPeriod(P, undefined, '2026-04-01')
  assert.equal(window.start, '2026-04-27')
})

test('an overdue cycle rolls forward by whole cycles', () => {
  const overdue = [
    { id: 1, started_on: '2026-01-01', ended_on: '2026-01-05', flow: 'medium' },
    { id: 2, started_on: '2026-01-29', ended_on: '2026-02-02', flow: 'medium' },
  ]
  const window = predictNextPeriod(overdue, undefined, '2026-03-15')
  assert.equal(window.start, '2026-03-26') // 01-29 + 28 is past; 03-26 is the first future one
})

test('ovulation peaks 14 cycle-days after the last start, matching the ovulatory phase', () => {
  // last start 2026-03-30, next 2026-04-27 (28-day cycle) → cycle day 14 = 2026-04-12.
  const ov = predictOvulation(P, '2026-04-27', '2026-04-01')
  assert.equal(ov.peak, '2026-04-12')
  assert.deepEqual(ov.days, ['2026-04-10', '2026-04-11', '2026-04-12', '2026-04-13', '2026-04-14'])
})

test('no prediction without at least two logged periods', () => {
  assert.equal(predictNextPeriod([], undefined, '2026-04-01'), undefined)
  assert.equal(predictOvulation([{ id: 1, started_on: '2026-03-02', flow: 'medium' }], undefined, '2026-04-01'), undefined)
})

test('predictCycle returns both windows', () => {
  const { nextPeriod, ovulation } = predictCycle(P, '2026-04-27', '2026-04-01')
  assert.equal(nextPeriod.start, '2026-04-27')
  assert.equal(ovulation.peak, '2026-04-12')
})
