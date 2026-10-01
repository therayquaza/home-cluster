// The kind -> glyph vocabulary and flow labels are shared with mobile. Only the
// Tailwind class fragments for flow tinting are web-specific, so they stay here.
export { SYMPTOM_EMOJI, SEX_EMOJI, LIBIDO_EMOJI, SEX_KINDS, LIBIDO_KINDS, MOOD_EMOJI, MOODS, FLOW_LEVELS, flowEmoji, flowLabel, symptomEmoji, shortKind, moodEmoji } from '@dinks/shared'
export type { FlowLevel } from '@dinks/shared'

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
