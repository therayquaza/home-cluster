/**
 * Emoji and flow vocabulary shared by web and mobile. Deliberately free of any
 * presentational classes (Tailwind, React Native styles): both shells render
 * these strings their own way, but the mapping from a logged kind to its glyph
 * — and the meaning of a flow level — must not drift between platforms.
 */
export const SYMPTOM_EMOJI: Record<string, string> = {
  Cramps: '😣',
  Headache: '🤕',
  Bloating: '🎈',
  Fatigue: '😴',
  'Tender breasts': '💗',
  'Mood swings': '🎭',
  Acne: '🔴',
  Nausea: '🤢',
  'Abdominal pain': '😖',
  'Increased appetite': '🍽️',
  Backache: '🦴',
  'Vaginal itching': '🌡️',
  Insomnia: '🌙',
  Diarrhea: '💧',
  'Vaginal dryness': '🏜️',
}

/**
 * Sex and libido are recorded as check-in kinds, matching how the Flo importer
 * writes them. They are deliberately not in SYMPTOM_KINDS: they are a separate
 * section in the day editor, because a user logging sex is not reporting a
 * symptom and should not have to scroll past fourteen symptom toggles to reach
 * it.
 */
export const SEX_EMOJI: Record<string, string> = {
  'Sex — protected': '🛡️',
  'Sex — unprotected': '⚠️',
  'Sex — oral': '💋',
  'Sex — toys': '🎲',
  Orgasm: '✨',
}

export const LIBIDO_EMOJI: Record<string, string> = {
  'Libido — high': '🔥',
  'Libido — moderate': '🙂',
  'Libido — low': '😐',
  'Libido — none': '😶',
}

export const SEX_KINDS = Object.keys(SEX_EMOJI)
export const LIBIDO_KINDS = Object.keys(LIBIDO_EMOJI)

export const MOOD_EMOJI: Record<string, string> = {
  Calm: '😌',
  Happy: '😄',
  Sensitive: '🥺',
  Irritable: '😤',
  Anxious: '😰',
  Sad: '😢',
  Energetic: '⚡',
  Tired: '🥱',
  Confident: '😎',
  Stressed: '😩',
  Grateful: '🥰',
  Confused: '🌫️',
}

/** The moods the picker offers, in display order. */
export const MOODS = Object.keys(MOOD_EMOJI)

/**
 * Flow levels, in increasing order. `unknown` is a real value, not a fallback:
 * Flo recorded no intensity for most days, and the app should say "not recorded"
 * rather than imply light bleeding.
 */
export const FLOW_LEVELS = ['none', 'unknown', 'light', 'medium', 'heavy'] as const
export type FlowLevel = (typeof FLOW_LEVELS)[number]

const FLOW_EMOJI: Record<string, string> = {
  none: '⚪',
  unknown: '◦',
  light: '💧',
  medium: '🩸',
  heavy: '🌊',
}

const FLOW_LABEL: Record<string, string> = {
  none: 'No flow',
  unknown: 'Not recorded',
  light: 'Light',
  medium: 'Medium',
  heavy: 'Heavy',
}

export function flowEmoji(flow: string) {
  return FLOW_EMOJI[flow] ?? FLOW_EMOJI.unknown
}

export function flowLabel(flow: string) {
  return FLOW_LABEL[flow] ?? 'Not recorded'
}

export function symptomEmoji(kind: string) {
  return SYMPTOM_EMOJI[kind] ?? ''
}

/** Strips the "Sex — "/"Libido — " prefix for compact display. */
export function shortKind(kind: string) {
  // "Sex — protected" reads as "protected" and "track:body_weight" as
  // "body_weight": the prefix is an internal marker, not something to show.
  return kind.replace(/^(Sex|Libido) — /, '').replace(/^track:/, '')
}

export function moodEmoji(mood: string) {
  return MOOD_EMOJI[mood] ?? ''
}
