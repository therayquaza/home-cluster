import { useEffect, useMemo, useState } from 'react'
import type { Prediction } from '@dinks/shared'
import { client } from '../api'
import { useDashboard } from '../useDashboard'
import { useSelectedDate, todayISO } from '../useSelectedDate'
import { periodOnDay } from '../lib/periods'
import { useMutationToast } from '../useToast'
import { symptomEmoji, moodEmoji } from '../lib/emoji'
import LogDay from '../components/LogDay'

const moods = ['Calm', 'Happy', 'Sensitive', 'Irritable', 'Anxious', 'Sad', 'Energetic', 'Tired', 'Confident', 'Stressed', 'Grateful']
const symptomKinds = ['Cramps', 'Headache', 'Bloating', 'Fatigue', 'Tender breasts']

export default function Today() {
  const { data, refresh } = useDashboard()
  const { selectedDate } = useSelectedDate()
  const [flow, setFlow] = useState('medium')
  const [moodSelection, setMoodSelection] = useState<string[]>(['Calm'])
  const [note, setNote] = useState('')
  const [dayEditorOpen, setDayEditorOpen] = useState(false)
  const [busyKind, setBusyKind] = useState<string | null>(null)
  const run = useMutationToast()
  const day = selectedDate
  const todayIso = todayISO()
  const isToday = day === todayIso
  const [prediction, setPrediction] = useState<Prediction>()

  useEffect(() => {
    client.prediction().then(setPrediction).catch(() => undefined)
  }, [data])

  // The globally active (unfinished) period, if any — at most one can exist.
  const active = useMemo(() => data?.periods.find((p) => !p.ended_on), [data])
  // The period covering the currently selected day, whether active or already closed.
  const periodForDay = useMemo(() => periodOnDay(data?.periods ?? [], day, todayIso), [data, day, todayIso])
  const ongoingForDay = !!periodForDay && !periodForDay.ended_on
  const isFuture = day > todayIso
  const todaysSymptoms = useMemo(() => (data?.symptoms ?? []).filter((s) => s.recorded_on === day), [data, day])
  const savedNotes = useMemo(() => todaysSymptoms.filter((s) => s.kind === 'note'), [todaysSymptoms])

  async function endPeriod() {
    if (!active) return
    await run(
      () => client.updatePeriod(active.id, { started_on: active.started_on, ended_on: day, flow: active.flow, notes: active.notes }),
      'Period ended',
    ).catch(() => undefined)
    refresh()
  }

  async function startPeriod() {
    if (isFuture) return
    await run(() => client.createPeriod({ started_on: day, flow, notes: '' }), 'Period started').catch(() => undefined)
    refresh()
  }

  /** Tap-to-toggle: logs the symptom for today, or removes it if it's already logged — mirrors the full-day editor. */
  async function toggleSymptom(kind: string) {
    const existing = todaysSymptoms.filter((s) => s.kind === kind)
    setBusyKind(kind)
    try {
      if (existing.length) {
        await run(() => Promise.all(existing.map((s) => client.deleteSymptom(s.id))), `${kind} removed`)
      } else {
        await run(() => client.createSymptom({ recorded_on: day, kind, severity: 3, notes: '' }), `${kind} logged`)
      }
      await refresh()
    } catch {
      // toast already shown
    } finally {
      setBusyKind(null)
    }
  }

  function toggleMood(m: string) {
    setMoodSelection((prev) => (prev.includes(m) ? prev.filter((x) => x !== m) : [...prev, m]))
  }

  async function saveNote() {
    if (!note.trim()) return
    const moodText = moodSelection.length ? moodSelection.join(', ') : 'none'
    await run(() => client.createSymptom({ recorded_on: day, kind: 'note', severity: 3, notes: `Mood: ${moodText}. ${note}` }), 'Note saved').catch(() => undefined)
    setNote('')
    refresh()
  }

  return (
    <main className="relative flex flex-col gap-4 p-4">
      <section className="rounded-3xl bg-gradient-to-br from-brand-100 to-brand-200 p-6">
        <p className="text-xs font-semibold uppercase tracking-widest text-brand-700">{isToday ? 'Today' : day}</p>
        <h1 className="my-2 text-2xl font-bold text-slate-800">
          {ongoingForDay ? 'On your period' : periodForDay ? 'Period logged' : 'Not on your period'}
        </h1>
        <p className="text-sm text-slate-600">
          {periodForDay
            ? `Started ${periodForDay.started_on}${periodForDay.ended_on ? `, ended ${periodForDay.ended_on}` : ''}.`
            : prediction?.predicted_period_start
              ? `Predicted next period: ${prediction.predicted_period_start}.`
              : 'Log two periods to receive an estimate.'}
        </p>

        {isFuture ? (
          <p className="mt-4 text-xs text-brand-700">Can't log a period for a future day.</p>
        ) : (
          <>
            {ongoingForDay && (
              <button className="mt-4 w-full rounded-xl bg-brand-500 py-3 font-semibold text-white shadow-sm" onClick={endPeriod}>
                Log period end
              </button>
            )}

            {!periodForDay && active && (
              <p className="mt-4 text-xs text-brand-700">You already have an active period started {active.started_on}. End it before starting a new one.</p>
            )}

            {!periodForDay && !active && (
              <>
                <select className="mt-3 w-full rounded-xl border border-brand-200 bg-white px-3 py-2 text-sm" value={flow} onChange={(e) => setFlow(e.target.value)}>
                  <option value="light">Light</option>
                  <option value="medium">Medium</option>
                  <option value="heavy">Heavy</option>
                </select>
                <button className="mt-4 w-full rounded-xl bg-brand-500 py-3 font-semibold text-white shadow-sm" onClick={startPeriod}>
                  Log period
                </button>
              </>
            )}
          </>
        )}

        {periodForDay && !ongoingForDay && (
          <p className="mt-4 text-xs text-brand-700">To edit or clear this period, use the + button below.</p>
        )}

        {data?.reminder && <p className="mt-3 text-xs text-brand-700">{data.reminder}</p>}
      </section>

      <section className="rounded-3xl bg-white p-5 shadow-sm">
        <div className="mb-1 flex items-center justify-between">
          <h2 className="font-semibold text-slate-800">How are you feeling?</h2>
          <button className="text-xs font-semibold text-brand-500" onClick={() => setDayEditorOpen(true)}>
            More options
          </button>
        </div>
        <p className="mb-3 text-xs text-slate-400">Tap a symptom to log it — tap again to remove it.</p>
        <div className="flex flex-wrap gap-2">
          {symptomKinds.map((k) => {
            const logged = todaysSymptoms.some((s) => s.kind === k)
            return (
              <button
                key={k}
                onClick={() => toggleSymptom(k)}
                disabled={busyKind === k}
                aria-pressed={logged}
                className={`rounded-full px-3 py-1.5 text-sm font-medium transition disabled:opacity-50 ${
                  logged ? 'bg-brand-500 text-white' : 'bg-brand-50 text-brand-700'
                }`}
              >
                {logged ? '✓ ' : ''}
                {symptomEmoji(k)} {k}
              </button>
            )
          })}
        </div>
      </section>

      <section className="rounded-3xl bg-white p-5 shadow-sm">
        <h2 className="mb-1 font-semibold text-slate-800">Add a note</h2>
        <p className="mb-3 text-xs text-slate-400">Optional — mood, sleep, energy, anything worth remembering.</p>
        <label className="mb-2 block text-sm font-medium text-slate-600">Mood (pick any)</label>
        <div className="flex flex-wrap gap-2">
          {moods.map((m) => {
            const picked = moodSelection.includes(m)
            return (
              <button
                key={m}
                onClick={() => toggleMood(m)}
                aria-pressed={picked}
                className={`rounded-full px-3 py-1.5 text-sm ${picked ? 'bg-brand-500 text-white' : 'bg-brand-50 text-brand-700'}`}
              >
                {picked ? '✓ ' : ''}
                {moodEmoji(m)} {m}
              </button>
            )
          })}
        </div>
        <textarea
          className="mt-4 w-full rounded-xl border border-brand-100 p-3 text-sm"
          placeholder="Notes, sleep, energy, bowel movements…"
          value={note}
          onChange={(e) => setNote(e.target.value)}
        />
        <button
          className="mt-3 w-full rounded-xl bg-brand-500 py-2.5 font-semibold text-white disabled:opacity-40"
          onClick={saveNote}
          disabled={!note.trim()}
        >
          Save note
        </button>

        {savedNotes.length > 0 && (
          <div className="mt-4 flex flex-col gap-2 border-t border-slate-100 pt-4">
            <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-400">Saved notes for this day</h3>
            {savedNotes.map((n) => (
              <p key={n.id} className="rounded-xl bg-slate-50 p-3 text-sm text-slate-600">
                {n.notes}
              </p>
            ))}
          </div>
        )}
      </section>

      <p className="px-1 text-center text-xs text-slate-400">{prediction?.disclaimer ?? 'Estimates are not medical advice.'}</p>

      <button
        className="fixed bottom-24 right-5 z-20 grid h-14 w-14 place-items-center rounded-full bg-brand-500 text-3xl font-light text-white shadow-lg"
        onClick={() => setDayEditorOpen(true)}
        aria-label={isToday ? 'Add or edit everything for today' : `Add or edit everything for ${day}`}
      >
        +
      </button>

      {dayEditorOpen && (
        <LogDay
          date={day}
          period={periodForDay}
          symptoms={todaysSymptoms}
          onClose={() => setDayEditorOpen(false)}
          onSaved={refresh}
        />
      )}
    </main>
  )
}
