import { useNavigate } from 'react-router-dom'
import { useDashboard } from '../useDashboard'
import { avgCycleLength, avgPeriodLength, symptomBreakdown } from '../lib/cycleStats'

export default function Insights() {
  const { data } = useDashboard()
  const navigate = useNavigate()

  const cycleLength = avgCycleLength(data?.periods ?? [])
  const periodLength = avgPeriodLength(data?.periods ?? [])
  const breakdown = symptomBreakdown(data?.symptoms ?? [])

  return (
    <main className="flex flex-col gap-4 p-4">
      <header className="flex items-center gap-3">
        <button className="text-sm font-semibold text-brand-700" onClick={() => navigate(-1)}>
          ← Back
        </button>
        <h1 className="text-xl font-bold text-slate-800">Insights</h1>
      </header>

      <section className="grid grid-cols-2 gap-3">
        <div className="rounded-3xl bg-white p-5 shadow-sm">
          <p className="text-xs uppercase tracking-wide text-slate-400">Avg cycle length</p>
          <p className="mt-1 text-2xl font-bold text-brand-700">{cycleLength ? `${cycleLength}d` : '—'}</p>
          {!cycleLength && <p className="mt-1 text-xs text-slate-400">Log at least two periods to see this.</p>}
        </div>
        <div className="rounded-3xl bg-white p-5 shadow-sm">
          <p className="text-xs uppercase tracking-wide text-slate-400">Avg period length</p>
          <p className="mt-1 text-2xl font-bold text-brand-700">{periodLength ? `${periodLength}d` : '—'}</p>
          {!periodLength && <p className="mt-1 text-xs text-slate-400">Log a completed period to see this.</p>}
        </div>
      </section>

      <section className="rounded-3xl bg-white p-5 shadow-sm">
        <h2 className="mb-3 font-semibold text-slate-800">Symptom frequency</h2>
        {breakdown.length === 0 && <p className="text-sm text-slate-400">No symptoms logged yet.</p>}
        <div className="flex flex-col gap-2">
          {breakdown.map((s) => (
            <div key={s.kind}>
              <div className="mb-1 flex items-center justify-between text-xs text-slate-500">
                <span className="font-medium text-slate-700">{s.kind}</span>
                <span>
                  {s.count} · {s.pct}%
                </span>
              </div>
              <div className="h-2 w-full rounded-full bg-brand-50">
                <div className="h-2 rounded-full bg-brand-500" style={{ width: `${Math.max(s.pct, 4)}%` }} />
              </div>
            </div>
          ))}
        </div>
      </section>

      <p className="px-1 text-center text-xs text-slate-400">
        Estimates are not medical advice. See the <button className="underline" onClick={() => navigate('/stats')}>Stats</button> tab for trends over time.
      </p>
    </main>
  )
}
