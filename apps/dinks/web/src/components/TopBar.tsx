import { useMemo, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { useDashboard } from '../useDashboard'
import { useSelectedDate, todayISO } from '../useSelectedDate'
import { toISO } from '../lib/dates'
import { periodOnDay } from '../lib/periods'

const WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']
const WEEKDAYS_FULL = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']
const MONTHS = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December']

export default function TopBar() {
  const navigate = useNavigate()
  const location = useLocation()
  const { displayName, data } = useDashboard()
  const { selectedDate, setSelectedDate } = useSelectedDate()
  const initial = displayName ? displayName.trim().charAt(0).toUpperCase() : '?'
  const todayIso = todayISO()
  const [windowOffset, setWindowOffset] = useState(0)

  const strip = useMemo(() => {
    const base = new Date()
    const days: { iso: string; weekday: string; dayNum: number }[] = []
    for (let offset = -3; offset <= 3; offset++) {
      const d = new Date(base.getFullYear(), base.getMonth(), base.getDate() + windowOffset + offset)
      days.push({ iso: toISO(d), weekday: WEEKDAYS[d.getDay()], dayNum: d.getDate() })
    }
    return days
  }, [windowOffset])

  const selected = useMemo(() => {
    const [y, m, d] = selectedDate.split('-').map(Number)
    return new Date(y, m - 1, d)
  }, [selectedDate])

  return (
    <header className="sticky top-0 z-30 bg-brand-bg/95 px-4 pb-3 pt-[calc(env(safe-area-inset-top,0px)+0.75rem)] backdrop-blur">
      <div className="flex items-center justify-between">
        <button
          className="grid h-9 w-9 place-items-center rounded-full bg-brand-500 text-sm font-bold text-white"
          onClick={() => navigate('/settings')}
          aria-label="Profile and settings"
        >
          {initial}
        </button>
        <div className="text-center">
          <p className="text-sm font-bold text-slate-800">
            {WEEKDAYS_FULL[selected.getDay()]}, {MONTHS[selected.getMonth()]} {selected.getDate()}
          </p>
        </div>
        <button
          className="grid h-9 w-9 place-items-center rounded-full bg-white text-brand-500 shadow-sm"
          onClick={() => navigate('/calendar')}
          aria-label="Open calendar"
        >
          <svg viewBox="0 0 24 24" className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={2}>
            <rect x="3" y="5" width="18" height="16" rx="2" />
            <path d="M8 3v4M16 3v4M3 10h18" />
          </svg>
        </button>
      </div>
      <div className="mt-3 flex items-center gap-1">
        <button
          className="grid h-8 w-6 shrink-0 place-items-center text-slate-300"
          onClick={() => setWindowOffset((o) => o - 3)}
          aria-label="Show earlier days"
        >
          <svg viewBox="0 0 24 24" className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth={2.5}>
            <path d="M15 6l-6 6 6 6" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        </button>
        <div className="flex flex-1 justify-between gap-1">
          {strip.map((day) => {
            const isSelected = day.iso === selectedDate
            const isToday = day.iso === todayIso
            const period = periodOnDay(data?.periods ?? [], day.iso, todayIso)
            // Ongoing: period started, not yet marked ended — still being registered. Closed: a finished, logged period day.
            const ongoing = !!period && !period.ended_on
            const closed = !!period && !!period.ended_on
            const predicted = day.iso === data?.next_period
            const symptom = data?.symptoms.some((s) => s.recorded_on === day.iso)
            const filled = isSelected || period
            return (
              <button
                key={day.iso}
                onClick={() => {
                  setSelectedDate(day.iso)
                  if (location.pathname !== '/today') navigate('/today')
                }}
                className={`relative flex flex-1 flex-col items-center gap-1 rounded-2xl py-2 text-xs font-medium transition ${
                  isSelected
                    ? 'bg-brand-500 text-white'
                    : ongoing
                      ? 'bg-brand-500 text-white'
                      : closed
                        ? 'bg-brand-300 text-white'
                        : predicted
                          ? 'border-2 border-dashed border-brand-500 text-brand-700'
                          : 'text-slate-500'
                }`}
              >
                <span className="uppercase tracking-wide">{day.weekday}</span>
                <span
                  className={`grid h-6 w-6 place-items-center rounded-full text-sm font-bold ${
                    isToday ? (filled ? 'ring-2 ring-white' : 'ring-2 ring-brand-500') : ''
                  }`}
                >
                  {day.dayNum}
                </span>
                {predicted && !period && <span className="sr-only">Predicted period day</span>}
                {ongoing && <span className="sr-only">Period day, still ongoing</span>}
                {closed && <span className="sr-only">Period day, ended</span>}
                {symptom && <span className="absolute bottom-1 h-1.5 w-1.5 rounded-full bg-brand-700" aria-label="Symptom logged" />}
              </button>
            )
          })}
        </div>
        <button
          className="grid h-8 w-6 shrink-0 place-items-center text-slate-300"
          onClick={() => setWindowOffset((o) => o + 3)}
          aria-label="Show later days"
        >
          <svg viewBox="0 0 24 24" className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth={2.5}>
            <path d="M9 6l6 6-6 6" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        </button>
      </div>
    </header>
  )
}
