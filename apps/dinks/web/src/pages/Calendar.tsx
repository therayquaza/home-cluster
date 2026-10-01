import { useMemo, useState } from 'react'
import { DayPicker } from 'react-day-picker'
import type { FlowDay, Period } from '@dinks/shared'
import { useDashboard } from '../useDashboard'
import { useSelectedDate, todayISO } from '../useSelectedDate'
import { toDate, toISO } from '../lib/dates'
import { periodOnDay, flowByDate, flowOnDay } from '../lib/periods'
import { client } from '../api'
import { useMutationToast } from '../useToast'
import { flowEmoji, flowLabel } from '../lib/emoji'
import FlowPicker from '../components/FlowPicker'
import LogDay from '../components/LogDay'

export default function CalendarPage() {
  const { data, preferences, refresh } = useDashboard()
  const { selectedDate, setSelectedDate } = useSelectedDate()
  const [month, setMonth] = useState(new Date())
  const [openDate, setOpenDate] = useState<string | undefined>()
  const todayIso = todayISO()
  const run = useMutationToast()

  const selected = toDate(selectedDate)

  function handleSelect(date: Date | undefined) {
    if (!date) return
    const iso = toISO(date)
    if (iso === selectedDate) {
      setOpenDate(iso)
    } else {
      setSelectedDate(iso)
    }
  }

  const flowByDay = useMemo(() => flowByDate(data?.periods ?? [], todayIso), [data, todayIso])

  const symptomDates = useMemo(() => {
    const set = new Set<string>()
    for (const s of data?.symptoms ?? []) set.add(s.recorded_on)
    return set
  }, [data])

  const predictedDate = data?.next_period
  const selectedPeriod = useMemo(() => periodOnDay(data?.periods ?? [], selectedDate, todayIso), [data, selectedDate, todayIso])
  const selectedFlow = selectedPeriod ? flowOnDay(selectedPeriod, selectedDate) : undefined

  /**
   * Toggling a day on or off the period straight from the grid, without opening
   * the editor. Adding a day extends the current period (or opens a new one on
   * that date); removing one shortens it, and removes the whole period if it
   * drops below two days — a one-day period is almost always a mis-tap.
   */
  async function togglePeriodDay(iso: string) {
    const period = periodOnDay(data?.periods ?? [], iso, todayIso)
    try {
      if (period) {
        // Removing the first day deletes the period outright — there is nothing
        // left of it. Otherwise the period is shortened to end on the day before,
        // which is the only way to actually drop a day in the middle. Re-opening
        // the period is a different, additive action, so it does not belong here.
        if (iso === period.started_on) {
          await run(() => client.deletePeriod(period.id), 'Day removed')
        } else {
          const prev = new Date(`${iso}T00:00:00Z`)
          prev.setUTCDate(prev.getUTCDate() - 1)
          const end = prev.toISOString().slice(0, 10)
          await run(
            () => client.updatePeriod(period.id, { started_on: period.started_on, ended_on: end, flow: period.flow, days: withoutDay(period, iso), notes: period.notes }),
            'Day removed',
          )
        }
      } else {
        if (iso > todayIso) return
        await run(
          () => client.createPeriod({ started_on: iso, flow: preferences.default_flow, days: [], notes: '' }),
          'Period started',
        )
      }
      await refresh()
    } catch {
      // toast already shown
    }
  }

  /**
 * Drops a day from the period's per-day list. Once the day is outside the period
 * its own entry would be rejected by validation, so removing the day has to
 * remove the entry with it.
 */
function withoutDay(p: Period, iso: string): FlowDay[] {
  return (p.days ?? []).filter((d) => d.date !== iso)
}

/** Sets the flow for one day of a period without opening the editor. */  async function setDayFlow(iso: string, periodId: number, next: string) {
    const period = (data?.periods ?? []).find((p) => p.id === periodId)
    if (!period) return
    const days = (period.days ?? []).filter((d) => d.date !== iso && d.flow !== period.flow)
    if (next !== period.flow) days.push({ date: iso, flow: next })
    days.sort((a, b) => a.date.localeCompare(b.date))
    try {
      await run(() => client.updatePeriod(period.id, { started_on: period.started_on, ended_on: period.ended_on, flow: period.flow, days, notes: period.notes }), `Flow set to ${flowLabel(next)}`)
      await refresh()
    } catch {
      // toast already shown
    }
  }

  return (
    <main className="p-4">
      <div className="mb-4 flex items-baseline justify-between">
        <h1 className="text-xl font-bold text-slate-800">Calendar</h1>
        <p className="text-xs font-medium text-brand-700">Today: {todayIso}</p>
      </div>
      <div className="rounded-3xl bg-white p-3 shadow-sm">
        <DayPicker
          mode="single"
          month={month}
          onMonthChange={setMonth}
          selected={selected}
          onSelect={handleSelect}
          modifiers={{
            predicted: (date) => predictedDate === toISO(date),
            symptom: (date) => symptomDates.has(toISO(date)),
            today: (date) => toISO(date) === todayIso,
            flowLight: (date) => flowByDay.get(toISO(date)) === 'light',
            flowMedium: (date) => flowByDay.get(toISO(date)) === 'medium',
            flowHeavy: (date) => flowByDay.get(toISO(date)) === 'heavy',
            flowUnknown: (date) => {
              const f = flowByDay.get(toISO(date))
              return f === 'unknown' || f === 'none'
            },
          }}
          modifiersClassNames={{
            predicted: 'rdp-day_predicted',
            symptom: 'rdp-day_symptom',
            today: 'rdp-day_today',
            flowLight: 'rdp-day_flow_light',
            flowMedium: 'rdp-day_flow_medium',
            flowHeavy: 'rdp-day_flow_heavy',
            flowUnknown: 'rdp-day_flow_unknown',
          }}
          className="mx-auto"
        />
      </div>

      {/* Live editor for the selected day: edit the period and its flow in place. */}
      {selectedPeriod ? (
        <section className="mt-4 flex flex-col gap-3 rounded-3xl bg-white p-4 shadow-sm">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="font-semibold text-slate-800">{selectedDate}</h2>
              <p className="text-xs text-slate-500">
                {flowEmoji(selectedFlow ?? 'unknown')} {flowLabel(selectedFlow ?? 'unknown')} ·{' '}
                {selectedPeriod.ended_on ? `ended ${selectedPeriod.ended_on}` : 'ongoing'}
              </p>
            </div>
            <button className="rounded-xl bg-slate-100 px-3 py-1.5 text-xs font-semibold text-slate-600" onClick={() => void togglePeriodDay(selectedDate)}>
              Remove day
            </button>
          </div>
          <FlowPicker
            value={selectedFlow ?? 'unknown'}
            onChange={(next) => void setDayFlow(selectedDate, selectedPeriod.id, next)}
            withNone
            size="sm"
          />
          <button className="text-xs font-semibold text-brand-500" onClick={() => setOpenDate(selectedDate)}>
            Log symptoms for this day
          </button>
        </section>
      ) : (
        <section className="mt-4 flex flex-col gap-3 rounded-3xl bg-white p-4 shadow-sm">
          <div>
            <h2 className="font-semibold text-slate-800">{selectedDate}</h2>
            <p className="text-xs text-slate-500">Not a period day.</p>
          </div>
          {selectedDate <= todayIso ? (
            <button className="rounded-xl bg-brand-500 py-2.5 font-semibold text-white" onClick={() => void togglePeriodDay(selectedDate)}>
              Mark as period day
            </button>
          ) : (
            <p className="text-xs text-slate-400">Can't log a period for a future day.</p>
          )}
          <button className="text-xs font-semibold text-brand-500" onClick={() => setOpenDate(selectedDate)}>
            Log symptoms for this day
          </button>
        </section>
      )}

      <div className="mt-4 flex flex-wrap gap-3 text-xs text-slate-500">
        <span className="flex items-center gap-1.5">
          <span className="h-3 w-3 rounded-full bg-brand-500" /> Heavy
        </span>
        <span className="flex items-center gap-1.5">
          <span className="h-3 w-3 rounded-full bg-brand-200" /> Medium
        </span>
        <span className="flex items-center gap-1.5">
          <span className="h-3 w-3 rounded-full bg-brand-100" /> Light
        </span>
        <span className="flex items-center gap-1.5">
          <span className="h-3 w-3 rounded-full border border-brand-100 bg-brand-50" /> Not recorded
        </span>
        <span className="flex items-center gap-1.5">
          <span className="h-3 w-3 rounded-full border-2 border-dashed border-brand-500" /> Predicted
        </span>
      </div>

      {openDate && (
        <LogDay
          layout="panel"
          date={openDate}
          period={periodOnDay(data?.periods ?? [], openDate, todayIso)}
          symptoms={(data?.symptoms ?? []).filter((s) => s.recorded_on === openDate)}
          trackers={preferences.trackers}
          defaultFlow={preferences.default_flow}
          onClose={() => setOpenDate(undefined)}
          onSaved={refresh}
        />
      )}
      <p className="mt-4 text-center text-xs text-slate-400">Estimates are not medical advice.</p>
    </main>
  )
}
