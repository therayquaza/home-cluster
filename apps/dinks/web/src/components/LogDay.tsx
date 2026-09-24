import { useEffect, useMemo, useState } from 'react'
import type { Period, Symptom } from '@dinks/shared'
import { client } from '../api'
import { useMutationToast } from '../useToast'
import { symptomEmoji } from '../lib/emoji'
import { todayISO } from '../useSelectedDate'

type Props = {
  date: string
  period?: Period
  symptoms: Symptom[]
  onClose: () => void
  onSaved: () => void
}

export const SYMPTOM_KINDS = ['Cramps', 'Headache', 'Bloating', 'Fatigue', 'Tender breasts', 'Mood swings', 'Acne', 'Nausea']

/**
 * One continuous scrollable sheet for logging everything about a single day: whether it's a
 * period day, and every common symptom kind as a toggle with an inline severity/notes editor.
 * Replaces the old one-form-per-symptom modal cycle in the removed DayDetailPanel.
 */
export default function LogDay({ date, period, symptoms, onClose, onSaved }: Props) {
  const run = useMutationToast()
  const isFuture = date > todayISO()
  const [periodOn, setPeriodOn] = useState(!!period)
  const [flow, setFlow] = useState(period?.flow ?? 'medium')
  const [endOn, setEndOn] = useState(period?.ended_on ?? '')
  const [periodNotes, setPeriodNotes] = useState(period?.notes ?? '')
  const [periodBusy, setPeriodBusy] = useState(false)
  const [busyKind, setBusyKind] = useState<string | null>(null)
  const [optimistic, setOptimistic] = useState<Record<string, boolean>>({})

  // Resync local period fields whenever the underlying record changes (e.g. after a refresh).
  useEffect(() => {
    setPeriodOn(!!period)
    setFlow(period?.flow ?? 'medium')
    setEndOn(period?.ended_on ?? '')
    setPeriodNotes(period?.notes ?? '')
  }, [period?.id, period?.flow, period?.ended_on, period?.notes])

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
          await run(() => client.updatePeriod(period.id, { started_on: period.started_on, ended_on: undefined, flow, notes: periodNotes }), 'Period resumed')
        } else {
          await run(() => client.createPeriod({ started_on: date, flow, notes: periodNotes }), 'Period logged')
        }
      } else if (period) {
        await run(
          () => client.updatePeriod(period.id, { started_on: period.started_on, ended_on: date, flow: period.flow, notes: period.notes }),
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

  async function savePeriodFields() {
    if (!period) return
    setPeriodBusy(true)
    try {
      await run(() => client.updatePeriod(period.id, { started_on: period.started_on, ended_on: endOn || undefined, flow, notes: periodNotes }), 'Period updated')
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

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-4" onClick={onClose}>
      <div className="flex max-h-[85vh] w-full max-w-lg flex-col overflow-y-auto rounded-3xl bg-white shadow-xl" onClick={(e) => e.stopPropagation()}>
        <div className="sticky top-0 z-10 flex items-center justify-between rounded-t-3xl border-b border-slate-100 bg-white/95 px-5 py-4 backdrop-blur">
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
                <div>
                  <label className="mb-1 block text-xs font-medium text-slate-600">Flow</label>
                  <select
                    className="w-full rounded-xl border border-brand-100 bg-white px-3 py-2 text-sm"
                    value={flow}
                    onChange={(e) => setFlow(e.target.value)}
                    onBlur={savePeriodFields}
                  >
                    <option value="light">Light</option>
                    <option value="medium">Medium</option>
                    <option value="heavy">Heavy</option>
                  </select>
                </div>
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
        </div>
      </div>
    </div>
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
