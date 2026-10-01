// Some things ride on a check-in without being something the member felt. A
// day's free text and a custom indicator's value are stored as check-ins so
// they round-trip through import, show up on the calendar and can be shared
// with a partner — but neither is a symptom, and treating them as one buries
// the real ones in the breakdown and skews the phase correlation.

const NOTE_KIND = 'note'
export const TRACK_PREFIX = 'track:'

export function isNoteKind(kind: string): boolean {
  return kind.trim().toLowerCase() === NOTE_KIND
}

export function isTrackKind(kind: string): boolean {
  return kind.startsWith(TRACK_PREFIX)
}

/** trackKey returns the indicator a track check-in holds a value for, or "". */
export function trackKey(kind: string): string {
  return isTrackKind(kind) ? kind.slice(TRACK_PREFIX.length) : ''
}

export function isSymptomKind(kind: string): boolean {
  return !isNoteKind(kind) && !isTrackKind(kind)
}

// A day's moods and its free text share one check-in: the note. The moods are
// written as a "Mood: a, b." prefix, so a whole day is one record rather than one
// per mood, and an imported note reads like one the member typed. These two
// functions are the only definition of that format — the writer and the reader
// must agree, or the picker silently loses whatever was saved.
const MOOD_PREFIX = 'Mood:'

/** Splits an "a, b" mood list, dropping blanks and the legacy "none" placeholder. */
function splitMoods(list: string): string[] {
  return list
    .split(',')
    .map((mood) => mood.trim())
    .filter((mood) => mood !== '' && mood.toLowerCase() !== 'none')
}

/** Recovers a day's moods and text from the note that stores them. */
export function parseDayNote(notes: string): { moods: string[]; text: string } {
  const raw = (notes ?? '').trim()
  // The mood list cannot contain a period, so the first one ends it; anything
  // after it is the free text, which may itself contain periods.
  const match = /^Mood:\s*([^.]*)(?:\.\s?([\s\S]*))?$/.exec(raw)
  if (!match) return { moods: [], text: raw }
  return { moods: splitMoods(match[1]), text: (match[2] ?? '').trim() }
}

/** Renders a day's moods and text back into the note that stores them. */
export function formatDayNote(moods: string[], text: string): string {
  const body = text.trim()
  // With no moods the note is just the text, matching what the Flo importer
  // writes for a day that has free text but no mood.
  if (!moods.length) return body
  const list = moods.join(', ')
  return body ? `${MOOD_PREFIX} ${list}. ${body}` : `${MOOD_PREFIX} ${list}.`
}
