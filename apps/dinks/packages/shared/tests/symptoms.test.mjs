import test from 'node:test'
import assert from 'node:assert/strict'
import { isNoteKind, isTrackKind, isSymptomKind, trackKey, symptomsByPhase, parseDayNote, formatDayNote } from '../src/index.ts'

test('note and indicator values are not symptoms', () => {
  assert.equal(isNoteKind('note'), true)
  assert.equal(isNoteKind('  Note '), true)
  assert.equal(isTrackKind('track:body_weight'), true)
  assert.equal(isNoteKind('Cramps'), false)
  assert.equal(isTrackKind('trackings'), false)

  assert.equal(isSymptomKind('Cramps'), true)
  assert.equal(isSymptomKind('Sex — protected'), true)
  assert.equal(isSymptomKind('note'), false)
  assert.equal(isSymptomKind('track:body_weight'), false)
})

test('trackKey names the indicator, or nothing at all', () => {
  assert.equal(trackKey('track:body_weight'), 'body_weight')
  assert.equal(trackKey('Cramps'), '')
})

test('the phase correlation ignores notes and indicator values', () => {
  // "note" was previously the top row of every phase simply because the UI
  // writes one note per day.
  const symptoms = [
    { recorded_on: '2026-03-02', kind: 'note', severity: 3, notes: 'Mood: Calm' },
    { recorded_on: '2026-03-03', kind: 'track:body_weight', severity: 1, notes: '62.5' },
    { recorded_on: '2026-03-04', kind: 'Cramps', severity: 3, notes: '' },
  ]
  const periods = [{ started_on: '2026-03-01', ended_on: '2026-03-05', flow: 'medium', days: [], notes: '' }]
  const rows = [...symptomsByPhase(symptoms, periods, 28).values()].flat()
  assert.deepEqual(rows.map((r) => r.label), ['Cramps'])
})

test('a day note round-trips its moods and text', () => {
  const cases = [
    { moods: ['Calm', 'Happy'], text: 'Slept well. Woke early.' },
    { moods: ['Calm'], text: '' },
    { moods: [], text: 'Just a plain note.' },
    { moods: [], text: '' },
  ]
  for (const c of cases) {
    assert.deepEqual(parseDayNote(formatDayNote(c.moods, c.text)), c)
  }
})

test('a note written by the Flo importer reads back', () => {
  assert.deepEqual(parseDayNote('Mood: Calm, Happy. Slept well.'), { moods: ['Calm', 'Happy'], text: 'Slept well.' })
  // The importer writes no trailing space, and no prefix at all when there is no mood.
  assert.deepEqual(parseDayNote('Mood: Calm, Happy.'), { moods: ['Calm', 'Happy'], text: '' })
  assert.deepEqual(parseDayNote('A plain imported note.'), { moods: [], text: 'A plain imported note.' })
})

test('the legacy "Mood: none." form carries no mood', () => {
  assert.deepEqual(parseDayNote('Mood: none. Tired.'), { moods: [], text: 'Tired.' })
  assert.deepEqual(parseDayNote(''), { moods: [], text: '' })
})
