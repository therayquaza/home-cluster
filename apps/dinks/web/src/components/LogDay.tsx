import { useEffect, useMemo, useState } from 'react'
import type { Period, Symptom, Tracker } from '@dinks/shared'
import { client } from '../api'
import { useMutationToast } from '../useToast'
import { symptomEmoji, flowLabel, SEX_EMOJI, LIBIDO_EMOJI, SEX_KINDS, LIBIDO_KINDS, shortKind } from '../lib/emoji'
import { todayISO } from '../useSelectedDate'
import { flowOnDay } from '../lib/periods'
import FlowPicker from './FlowPicker'

type Props = {
  date: string
  period?: Period
  symptoms: Symptom[]
  trackers: Tracker[]
  /** The member's preferred flow, preselected when starting a period. */
  defaultFlow: string
  onClose: () => void
  onSaved: () => void
  /** Calendar passes a handler that closes the inline panel; Today keeps the sheet. */
  layout?: 'sheet' | 'panel'
}

export const SYMPTOM_KINDS = ['Cramps', 'Headache', 'Bloating', 'Fatigue', 'Tender breasts', 'Mood swings', 'Acne', 'Nausea']

/** The check-in kind a custom indicator's value is stored under. */
export const trackerKind = (key: string) => `track:${key}`

/**
 * One continuous editor for everything about a single day: whether it's a period day,
 * the flow for that day, and every common symptom kind as a toggle with an inline
 * severity/notes editor.
 */
export default function LogDay({ date, period, symptoms, trackers, defaultFlow, onClose, onSaved, layout = 'sheet' }: Props) {
  const run = useMutationToast()
  const isFuture = date > todayISO()
  const [periodOn, setPeriodOn] = useState(!!period)
  // The flow for THIS day, which may differ from the period's summary flow.
  const [flow, setFlow] = useState(() => (period ? flowOnDay(period, date) : defaultFlow))
  const [endOn, setEndOn] = useState(period?.ended_on ?? '')
  const [periodNotes, setPeriodNotes] = useState(period?.notes ?? '')
  const [periodBusy, setPeriodBusy] = useState(false)
  const [busyKind, setBusyKind] = useState<string | null>(null)
  const [optimistic, setOptimistic] = useState<Record<string, boolean>>({})

  // Resync local period fields whenever the underlying record changes (e.g. after a refresh).
  useEffect(() => {
    setPeriodOn(!!period)
    setFlow(period ? flowOnDay(period, date) : defaultFlow)
    setEndOn(period?.ended_on ?? '')
    setPeriodNotes(period?.notes ?? '')
  }, [period?.id, period?.flow, period?.ended_on, period?.notes, period?.days, date, defaultFlow])

  const byKind = useMemo(() => {
    const map = new Map<string, Symptom>()
    for (const s of symptoms) map.set(s.kind, s)
    return map
  }, [symptoms])

  const allByKind = useMemo(() => {
    const map = new Map<string, Symptom[]>()
    for (const s of symptoms) map.set(s.kind, [...(map.get(s.kind) ?? []), s])
    return map
  }, [symptoms])

  async function togglePeriod(next: boolean) {
    if (next && isFuture) return
    setOptimistic((prev) => ({ ...prev, __period: next }))
    setPeriodOn(next)
    setPeriodBusy(true)
    try {
      if (next) {
        if (period) {
          const merged = withDayFlow(period, date, flow)
          await run(() => client.updatePeriod(period.id, { started_on: period.started_on, ended_on: undefined, flow: merged.flow, days: merged.days, notes: periodNotes }), 'Period resumed')
        } else {
          await run(() => client.createPeriod({ started_on: date, flow, notes: periodNotes }), 'Period logged')
        }
      } else if (period) {
        // Ending a period on this day must not silently drop the flow recorded
        // for days after it, so the per-day list is carried over as-is.
        await run(
          () => client.updatePeriod(period.id, { started_on: period.started_on, ended_on: date, flow: period.flow, days: period.days ?? [], notes: period.notes }),
          'Period ended',
        )
      }
      await onSaved()
    } catch {
      // toast already shown; revert optimistic toggle
      setPeriodOn(!!period)
    } finally {
      setPeriodBusy(false)
      setOptimistic((prev) => {
        const { __period: _drop, ...rest } = prev
        return rest
      })
    }
  }

  /**
   * Writes this day's flow into the period's per-day list, leaving the period's
   * summary flow and every other day untouched. A day whose flow equals the
   * summary is dropped rather than stored, so the list only ever holds days
   * that genuinely deviate — and clearing a day back to the summary is how you
   * remove an entry.
   */
  function withDayFlow(p: Period, day: string, dayFlow: string): { days: { date: string; flow: string }[]; flow: string } {
    const days = (p.days ?? []).filter((d) => d.date !== day && d.flow !== p.flow)
    if (dayFlow !== p.flow) days.push({ date: day, flow: dayFlow })
    days.sort((a, b) => a.date.localeCompare(b.date))
    return { days, flow: p.flow }
  }

  async function setDayFlow(nextFlow: string) {
    if (!period) return
    setPeriodBusy(true)
    const merged = withDayFlow(period, date, nextFlow)
    try {
      await run(
        () => client.updatePeriod(period.id, { started_on: period.started_on, ended_on: period.ended_on, flow: merged.flow, days: merged.days, notes: period.notes }),
        `Flow set to ${flowLabel(nextFlow)}`,
      )
      await onSaved()
    } catch {
      setFlow(flowOnDay(period, date))
    } finally {
      setPeriodBusy(false)
    }
  }

  async function savePeriodFields() {
    if (!period) return
    setPeriodBusy(true)
    const merged = withDayFlow(period, date, flow)
    try {
      await run(() => client.updatePeriod(period.id, { started_on: period.started_on, ended_on: endOn || undefined, flow: merged.flow, days: merged.days, notes: periodNotes }), 'Period updated')
      await onSaved()
    } catch {
      // toast already shown
    } finally {
      setPeriodBusy(false)
    }
  }

  async function toggleSymptom(kind: string, next: boolean) {
    const existing = allByKind.get(kind) ?? []
    setOptimistic((prev) => ({ ...prev, [kind]: next }))
    setBusyKind(kind)
    try {
      if (next && !existing.length) {
        await run(() => client.createSymptom({ recorded_on: date, kind, severity: 3, notes: '' }), `${kind} logged`)
      } else if (!next && existing.length) {
        await run(() => Promise.all(existing.map((s) => client.deleteSymptom(s.id))), `${kind} removed`)
      }
      await onSaved()
    } catch {
      // toast already shown; drop the optimistic override so it falls back to server truth
    } finally {
      setBusyKind(null)
      setOptimistic((prev) => {
        const { [kind]: _drop, ...rest } = prev
        return rest
      })
    }
  }

  async function saveSymptomFields(existing: Symptom, severity: number, notes: string) {
    setBusyKind(existing.kind)
    try {
      await run(() => client.updateSymptom(existing.id, { recorded_on: existing.recorded_on, kind: existing.kind, severity, notes }), `${existing.kind} updated`)
      await onSaved()
    } catch {
      // toast already shown
    } finally {
      setBusyKind(null)
    }
  }

  /**
   * Libido records one level per day, so choosing a level clears whichever other
   * level was set, and tapping the active one clears it outright. Sex kinds are
   * independent check-ins and use toggleSymptom.
   */
  async function setLibido(kind: string, next: boolean) {
    // Libido is single-choice: selecting a level replaces the previous one, and
    // tapping the active level clears it. `previous` therefore excludes only
    // the kind being set when clearing, but never when replacing.
    const previous = LIBIDO_KINDS.filter((k) => byKind.has(k)).map((k) => byKind.get(k)!)
    const stale = previous.filter((s) => s.kind !== kind)
    setBusyKind(kind)
    try {
      if (next) {
        await run(() => client.createSymptom({ recorded_on: date, kind, severity: 1, notes: '' }), `Libido — ${shortKind(kind).toLowerCase()} logged`)
        if (stale.length) await run(() => Promise.all(stale.map((s) => client.deleteSymptom(s.id))), 'Previous level cleared')
      } else {
        await run(() => Promise.all(previous.map((s) => client.deleteSymptom(s.id))), 'Libido cleared')
      }
      await onSaved()
    } catch {
      // toast already shown
    } finally {
      setBusyKind(null)
    }
  }

  /**
   * A custom indicator is a numeric value stored as a check-in whose kind is
   * "track:<key>" and whose notes hold the number. Keeping it in the same
   * collection means a tracker value travels through import, export, the
   * calendar and stats with no new plumbing — the trade-off is that the value
   * is text, so it is parsed defensively everywhere it is displayed.
   */
  const trackerValue = (key: string) => {
    const found = allByKind.get(trackerKind(key))?.[0]
    if (!found) return ''
    const n = Number(found.notes)
    return Number.isFinite(n) ? String(n) : ''
  }

  async function saveTracker(key: string, raw: string) {
    const kind = trackerKind(key)
    const existing = allByKind.get(kind)?.[0]
    const trimmed = raw.trim()
    setBusyKind(kind)
    try {
      if (trimmed === '') {
        if (existing) await run(() => client.deleteSymptom(existing.id), 'Value cleared')
      } else if (!Number.isFinite(Number(trimmed))) {
        // Refuse to store something that is not a number: a later average over
        // the tracker's values would have to skip it silently.
        return
      } else if (existing) {
        await run(() => client.updateSymptom(existing.id, { recorded_on: date, kind, severity: 1, notes: trimmed }), 'Value saved')
      } else {
        await run(() => client.createSymptom({ recorded_on: date, kind, severity: 1, notes: trimmed }), 'Value saved')
      }
      await onSaved()
    } catch {
      // toast already shown
    } finally {
      setBusyKind(null)
    }
  }

  // 'panel' renders inline under the calendar (keeping the month visible while
  // editing a day); 'sheet' is the full-screen overlay used from Today.
  const body = (
    <>
      <div className="flex items-center justify-between border-b border-slate-100 px-5 py-4">
        <h2 className="text-lg font-bold text-slate-800">{date}</h2>
        <button className="text-sm font-medium text-slate-400" onClick={onClose}>
          Done
        </button>
      </div>

      <div className="flex flex-col gap-5 p-5">
          <section className="rounded-2xl bg-brand-50 p-4">
            <label className="flex items-center justify-between">
              <span className="font-semibold text-brand-700">On my period</span>
              <button
                type="button"
                role="switch"
                aria-checked={optimistic.__period ?? periodOn}
                disabled={periodBusy || (isFuture && !periodOn)}
                onClick={() => togglePeriod(!(optimistic.__period ?? periodOn))}
                className={`relative h-7 w-12 shrink-0 rounded-full transition disabled:opacity-50 ${(optimistic.__period ?? periodOn) ? 'bg-brand-500' : 'bg-brand-200'}`}
              >
                <span
                  className={`absolute top-0.5 h-6 w-6 rounded-full bg-white shadow transition ${(optimistic.__period ?? periodOn) ? 'left-[22px]' : 'left-0.5'}`}
                />
              </button>
            </label>
            {isFuture && !periodOn && <p className="mt-2 text-xs text-brand-700">Can't log a period for a future day.</p>}

            {(optimistic.__period ?? periodOn) && (
              <div className="mt-4 flex flex-col gap-3">
                {period ? (
                  <div>
                    <label className="mb-1 block text-xs font-medium text-slate-600">Flow on {date}</label>
                    <FlowPicker
                      value={flow}
                      onChange={(next) => {
                        setFlow(next)
                        void setDayFlow(next)
                      }}
                      disabled={periodBusy}
                    />
                    <p className="mt-1 text-xs text-slate-400">
                      {period.days?.length
                        ? `This period varies by day — ${period.days.length} day${period.days.length > 1 ? 's' : ''} differ from its usual flow.`
                        : 'Set a different level on another day of this period to record how it changes.'}
                    </p>
                  </div>
                ) : (
                  <div>
                    <label className="mb-1 block text-xs font-medium text-slate-600">Flow</label>
                    <FlowPicker value={flow} onChange={setFlow} />
                  </div>
                )}
                <div>
                  <label className="mb-1 block text-xs font-medium text-slate-600">End date (leave blank if ongoing)</label>
                  <input
                    type="date"
                    className="w-full rounded-xl border border-brand-100 bg-white px-3 py-2 text-sm"
                    value={endOn}
                    onChange={(e) => setEndOn(e.target.value)}
                    onBlur={savePeriodFields}
                  />
                </div>
                <div>
                  <label className="mb-1 block text-xs font-medium text-slate-600">Notes</label>
                  <textarea
                    className="w-full rounded-xl border border-brand-100 bg-white px-3 py-2 text-sm"
                    value={periodNotes}
                    onChange={(e) => setPeriodNotes(e.target.value)}
                    onBlur={savePeriodFields}
                  />
                </div>
              </div>
            )}
          </section>

          <section className="rounded-2xl bg-slate-50 p-4">
            <h3 className="mb-3 font-semibold text-slate-700">Symptoms</h3>
            <div className="flex flex-col gap-2">
              {SYMPTOM_KINDS.map((kind) => {
                const existing = byKind.get(kind)
                const checked = optimistic[kind] ?? !!existing
                const busy = busyKind === kind
                return (
                  <div key={`${kind}-${existing?.id ?? 'new'}`} className="rounded-xl bg-white p-3 shadow-sm">
                    <label className="flex items-center justify-between gap-2">
                      <span className="flex items-center gap-2 text-sm font-medium text-slate-700">
                        <input
                          type="checkbox"
                          className="h-4 w-4 accent-brand-500"
                          checked={checked}
                          disabled={busy}
                          onChange={(e) => toggleSymptom(kind, e.target.checked)}
                        />
                        {symptomEmoji(kind)} {kind}
                      </span>
                      {busy && <span className="text-xs text-slate-400">Saving…</span>}
                    </label>
                    {checked && existing && (
                      <SymptomFields key={existing.id} existing={existing} busy={busy} onSave={saveSymptomFields} />
                    )}
                  </div>
                )
              })}
            </div>
          </section>

          <section className="rounded-2xl bg-slate-50 p-4">
            <h3 className="font-semibold text-slate-700">Sex &amp; libido</h3>
            <p className="mb-3 text-xs text-slate-400">Private to you — never shared with a partner unless you turn that on.</p>
            <p className="mb-2 text-xs font-medium uppercase tracking-wide text-slate-400">Sex</p>
            <div className="mb-4 flex flex-wrap gap-2">
              {SEX_KINDS.map((kind) => {
                const existing = byKind.get(kind)
                const checked = optimistic[kind] ?? !!existing
                return (
                  <button
                    key={kind}
                    type="button"
                    aria-pressed={checked}
                    disabled={busyKind === kind}
                    onClick={() => toggleSymptom(kind, !checked)}
                    className={`rounded-full px-3 py-1.5 text-sm font-medium transition disabled:opacity-50 ${
                      checked ? 'bg-brand-500 text-white' : 'bg-white text-slate-600 shadow-sm'
                    }`}
                  >
                    {SEX_EMOJI[kind]} {shortKind(kind)}
                  </button>
                )
              })}
            </div>
            <p className="mb-2 text-xs font-medium uppercase tracking-wide text-slate-400">Libido</p>
            <div className="flex flex-wrap gap-2">
              {LIBIDO_KINDS.map((kind) => {
                // Libido is one value at a time, so these behave as a radio
                // group rather than independent toggles.
                const existing = byKind.get(kind)
                const checked = optimistic[kind] ?? !!existing
                return (
                  <button
                    key={kind}
                    type="button"
                    aria-pressed={checked}
                    disabled={busyKind === kind}
                    onClick={() => setLibido(kind, !checked)}
                    className={`rounded-full px-3 py-1.5 text-sm font-medium transition disabled:opacity-50 ${
                      checked ? 'bg-brand-500 text-white' : 'bg-white text-slate-600 shadow-sm'
                    }`}
                  >
                    {LIBIDO_EMOJI[kind]} {shortKind(kind)}
                  </button>
                )
              })}
            </div>
          </section>

          {trackers.length > 0 && (
            <section className="rounded-2xl bg-slate-50 p-4">
              <h3 className="mb-3 font-semibold text-slate-700">Your indicators</h3>
              <div className="flex flex-col gap-2">
                {trackers.map((t) => (
                  <TrackerField
                    key={t.key}
                    tracker={t}
                    value={trackerValue(t.key)}
                    busy={busyKind === trackerKind(t.key)}
                    onSave={(v) => saveTracker(t.key, v)}
                  />
                ))}
              </div>
            </section>
          )}
        </div>
    </>
  )

  if (layout === 'panel') {
    return <div className="mt-4 flex flex-col overflow-hidden rounded-3xl bg-white shadow-sm">{body}</div>
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-4" onClick={onClose}>
      <div className="flex max-h-[85vh] w-full max-w-lg flex-col overflow-y-auto rounded-3xl bg-white shadow-xl" onClick={(e) => e.stopPropagation()}>
        {body}
      </div>
    </div>
  )
}

function TrackerField({
  tracker,
  value,
  busy,
  onSave,
}: {
  tracker: Tracker
  value: string
  busy: boolean
  onSave: (v: string) => void
}) {
  const [draft, setDraft] = useState(value)

  useEffect(() => setDraft(value), [value])

  return (
    <label className="flex items-center justify-between gap-3 rounded-xl bg-white px-3 py-2 shadow-sm">
      <span className="text-sm font-medium text-slate-700">
        {tracker.emoji} {tracker.label}
      </span>
      <span className="flex items-center gap-2">
        <input
          type="number"
          inputMode="decimal"
          step="any"
          value={draft}
          disabled={busy}
          aria-label={`${tracker.label} value`}
          onChange={(e) => setDraft(e.target.value)}
          onBlur={() => onSave(draft)}
          className="w-24 rounded-lg border border-slate-200 px-2 py-1 text-right text-sm"
        />
        {tracker.unit && <span className="w-10 text-xs text-slate-400">{tracker.unit}</span>}
      </span>
    </label>
  )
}

function SymptomFields({ existing, busy, onSave }: { existing: Symptom; busy: boolean; onSave: (s: Symptom, severity: number, notes: string) => void }) {
  const [severity, setSeverity] = useState(existing.severity)
  const [notes, setNotes] = useState(existing.notes ?? '')

  return (
    <div className="mt-3 flex flex-col gap-2 border-t border-slate-100 pt-3">
      <label className="flex items-center gap-3 text-xs font-medium text-slate-500">
        Severity
        <input
          type="range"
          min={1}
          max={5}
          value={severity}
          disabled={busy}
          onChange={(e) => setSeverity(Number(e.target.value))}
          onMouseUp={() => onSave(existing, severity, notes)}
          onTouchEnd={() => onSave(existing, severity, notes)}
          className="flex-1 accent-brand-500"
        />
        <span className="w-4 text-center text-sm font-bold text-brand-700">{severity}</span>
      </label>
      <textarea
        className="w-full rounded-lg border border-slate-200 px-2 py-1.5 text-sm"
        placeholder="Notes"
        value={notes}
        disabled={busy}
        onChange={(e) => setNotes(e.target.value)}
        onBlur={() => onSave(existing, severity, notes)}
      />
    </div>
  )
}
