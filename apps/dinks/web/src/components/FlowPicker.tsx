import { FLOW_LEVELS, flowChipClass, flowEmoji, flowLabel } from '../lib/emoji'

type Props = {
  value: string
  onChange: (flow: string) => void
  /** Shown as the period's summary flow rather than a single day's. */
  withNone?: boolean
  disabled?: boolean
  size?: 'sm' | 'md'
}

/**
 * Flow as a row of tappable levels rather than a <select>. A dropdown hides the
 * options behind a second tap and reads as a form field; the levels are a short
 * closed set, so showing all of them at once is both faster and clearer. The
 * selected level is a button with aria-pressed, so it stays reachable by keyboard
 * and announces its state to a screen reader.
 */
export default function FlowPicker({ value, onChange, withNone, disabled, size = 'md' }: Props) {
  const levels = withNone ? FLOW_LEVELS : FLOW_LEVELS.filter((l) => l !== 'none')
  return (
    <div className="flex flex-wrap gap-1.5" role="group" aria-label="Flow">
      {levels.map((level) => {
        const selected = value === level
        return (
          <button
            key={level}
            type="button"
            disabled={disabled}
            aria-pressed={selected}
            onClick={() => onChange(level)}
            title={flowLabel(level)}
            className={`flex items-center gap-1.5 rounded-full border font-medium transition disabled:opacity-50 ${
              size === 'sm' ? 'px-2.5 py-1 text-xs' : 'px-3 py-1.5 text-sm'
            } ${flowChipClass(level)} ${selected ? 'ring-2 ring-brand-700 ring-offset-1' : 'opacity-70 hover:opacity-100'}`}
          >
            <span aria-hidden="true">{flowEmoji(level)}</span>
            {flowLabel(level)}
          </button>
        )
      })}
    </div>
  )
}
