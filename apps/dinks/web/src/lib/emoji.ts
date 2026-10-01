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

/**
 * Tailwind class fragments for each flow level, used to tint a calendar day so
 * the shape of a period is readable at a glance. Deliberately paired with the
 * emoji rather than replacing it — colour alone is not an accessible signal.
 */
const FLOW_CHIP: Record<string, string> = {
  none: 'bg-slate-100 text-slate-500',
  unknown: 'bg-brand-50 text-brand-700 border-brand-100',
  light: 'bg-brand-100 text-brand-700 border-brand-200',
  medium: 'bg-brand-200 text-brand-700 border-brand-300',
  heavy: 'bg-brand-500 text-white border-brand-500',
}

export function flowChipClass(flow: string) {
  return FLOW_CHIP[flow] ?? FLOW_CHIP.unknown
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
