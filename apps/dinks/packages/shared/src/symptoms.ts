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
