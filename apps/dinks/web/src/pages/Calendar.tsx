import { useMemo, useState } from 'react'
import { DayPicker } from 'react-day-picker'
import { useDashboard } from '../useDashboard'
import { useSelectedDate, todayISO } from '../useSelectedDate'
import { toDate, toISO } from '../lib/dates'
import { periodDaySet, periodOnDay } from '../lib/periods'
import LogDay from '../components/LogDay'

export default function CalendarPage() {
  const { data, refresh } = useDashboard()
  const { selectedDate, setSelectedDate } = useSelectedDate()
  const [month, setMonth] = useState(new Date())
  const [openDate, setOpenDate] = useState<string | undefined>()
  const todayIso = todayISO()

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

  const periodDates = useMemo(() => periodDaySet(data?.periods ?? [], todayIso), [data, todayIso])

  const symptomDates = useMemo(() => {
    const set = new Set<string>()
    for (const s of data?.symptoms ?? []) set.add(s.recorded_on)
    return set
  }, [data])

  const predictedDate = data?.next_period

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
            period: (date) => periodDates.has(toISO(date)),
            predicted: (date) => predictedDate === toISO(date),
            symptom: (date) => symptomDates.has(toISO(date)),
            today: (date) => toISO(date) === todayIso,
          }}
          modifiersClassNames={{
            period: 'rdp-day_period',
            predicted: 'rdp-day_predicted',
            symptom: 'rdp-day_symptom',
            today: 'rdp-day_today',
          }}
          className="mx-auto"
        />
      </div>
      <div className="mt-4 flex flex-wrap gap-4 text-xs text-slate-500">
        <span className="flex items-center gap-1.5">
          <span className="h-3 w-3 rounded-full bg-brand-500" /> Period
        </span>
        <span className="flex items-center gap-1.5">
          <span className="h-3 w-3 rounded-full border-2 border-dashed border-brand-500" /> Predicted
        </span>
        <span className="flex items-center gap-1.5">
          <span className="h-1.5 w-1.5 rounded-full bg-brand-700" /> Symptom logged
        </span>
      </div>
      {openDate && (
        <LogDay
          date={openDate}
          period={periodOnDay(data?.periods ?? [], openDate, todayIso)}
          symptoms={(data?.symptoms ?? []).filter((s) => s.recorded_on === openDate)}
          onClose={() => setOpenDate(undefined)}
          onSaved={refresh}
        />
      )}
      <p className="mt-4 text-center text-xs text-slate-400">Estimates are not medical advice.</p>
    </main>
  )
}
