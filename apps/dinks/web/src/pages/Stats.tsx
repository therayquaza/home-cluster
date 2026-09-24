import { useMemo, useState } from 'react'
import type { StatsGroupBy, StatsMetric, StatsQueryResponse } from '@dinks/shared'
import { client } from '../api'
import { useDashboard } from '../useDashboard'
import { cycleGaps, periodLengths, symptomBreakdown } from '../lib/cycleStats'
import { toISO } from '../lib/dates'

type DateRangeKey = 'week' | 'month' | '3months' | 'all'
const RANGE_OPTIONS: { key: DateRangeKey; label: string }[] = [
  { key: 'week', label: 'Last week' },
  { key: 'month', label: 'Last month' },
  { key: '3months', label: 'Last 3 months' },
  { key: 'all', label: 'All time' },
]
function rangeFor(key: DateRangeKey): { from: string; to: string } {
  if (key === 'all') return { from: '', to: '' }
  const to = new Date()
  const from = new Date(to)
  if (key === 'week') from.setDate(from.getDate() - 7)
  if (key === 'month') from.setMonth(from.getMonth() - 1)
  if (key === '3months') from.setMonth(from.getMonth() - 3)
  return { from: toISO(from), to: toISO(to) }
}

const METRIC_OPTIONS: { key: StatsMetric; label: string }[] = [
  { key: 'period_count', label: 'Period count' },
  { key: 'checkin_count', label: 'Check-in count' },
  { key: 'cycles_sampled', label: 'Cycles sampled' },
  { key: 'average_cycle_days', label: 'Avg cycle length' },
  { key: 'cycle_length_stddev', label: 'Cycle length variability' },
  { key: 'average_period_length_days', label: 'Avg period length' },
  { key: 'symptom_counts', label: 'Symptom counts' },
  { key: 'average_symptom_severity', label: 'Avg symptom severity' },
]
const GROUP_BY_OPTIONS: { key: StatsGroupBy; label: string }[] = [
  { key: '', label: 'No grouping' },
  { key: 'month', label: 'By month' },
  { key: 'flow', label: 'By flow' },
  { key: 'symptom_kind', label: 'By symptom' },
]

/** One bar chart per numeric metric, one bar per group in the response — falls back to a plain list for non-numeric metrics (e.g. symptom_counts, an object per group). */
function CustomStatsResults({ result }: { result: StatsQueryResponse }) {
  const groups = result.results
  const metricKeys = Array.from(new Set(groups.flatMap((g) => Object.keys(g.values))))
  const numericKeys = metricKeys.filter((k) => groups.every((g) => typeof g.values[k] === 'number'))
  const otherKeys = metricKeys.filter((k) => !numericKeys.includes(k))

  return (
    <div className="mt-4 flex flex-col gap-4">
      {numericKeys.map((key) => {
        const label = METRIC_OPTIONS.find((m) => m.key === key)?.label ?? key
        const bars: Bar[] = groups.map((g) => ({ label: g.group || 'All', value: (g.values[key] as number) ?? 0 }))
        return (
          <div key={key}>
            <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-brand-700">{label}</p>
            {bars.length === 1 ? (
              <p className="text-2xl font-bold text-slate-800">{bars[0].value}</p>
            ) : (
              <BarChart bars={bars} unit="" />
            )}
          </div>
        )
      })}

      {otherKeys.map((key) =>
        groups.map((g) => {
          const value = g.values[key]
          if (!value || typeof value !== 'object') return null
          const entries = Object.entries(value as Record<string, number>).sort((a, b) => b[1] - a[1])
          const max = Math.max(...entries.map(([, v]) => v), 1)
          const label = METRIC_OPTIONS.find((m) => m.key === key)?.label ?? key
          return (
            <div key={`${key}-${g.group}`} className="rounded-2xl bg-brand-50 p-3">
              <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-brand-700">
                {label}
                {g.group ? ` — ${g.group}` : ''}
              </p>
              <div className="flex flex-col gap-2">
                {entries.map(([k, v]) => (
                  <div key={k}>
                    <div className="mb-0.5 flex items-center justify-between text-xs text-slate-600">
                      <span className="font-medium text-slate-700">{k}</span>
                      <span>{v}</span>
                    </div>
                    <div className="h-1.5 w-full rounded-full bg-white">
                      <div className="h-1.5 rounded-full bg-brand-500" style={{ width: `${Math.max((v / max) * 100, 4)}%` }} />
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )
        }),
      )}
    </div>
  )
}

/** Custom stats query builder: user picks metrics + grouping, request stays inside the backend's allowlist. */
function CustomStatsQuery() {
  const [metrics, setMetrics] = useState<StatsMetric[]>(['average_cycle_days'])
  const [groupBy, setGroupBy] = useState<StatsGroupBy>('')
  const [range, setRange] = useState<DateRangeKey>('month')
  const [{ from, to }, setDates] = useState(() => rangeFor('month'))
  const [result, setResult] = useState<StatsQueryResponse>()
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  function toggleMetric(key: StatsMetric) {
    setMetrics((prev) => (prev.includes(key) ? prev.filter((m) => m !== key) : [...prev, key]))
  }

  function pickRange(key: DateRangeKey) {
    setRange(key)
    setDates(rangeFor(key))
  }

  async function run() {
    setError('')
    setLoading(true)
    try {
      const res = await client.statsQuery({ metrics, group_by: groupBy || undefined, from: from || undefined, to: to || undefined })
      setResult(res)
    } catch (e) {
      setResult(undefined)
      setError(e instanceof Error ? e.message : 'Query failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <section className="rounded-3xl bg-white p-5 shadow-sm">
      <h2 className="mb-1 font-semibold text-slate-800">Custom stats</h2>
      <p className="mb-3 text-xs text-slate-400">Build your own view from allowed metrics — nothing here can query the database directly.</p>

      <p className="mb-1 text-xs font-medium text-slate-600">Metrics</p>
      <div className="mb-3 flex flex-wrap gap-2">
        {METRIC_OPTIONS.map((m) => (
          <button
            key={m.key}
            onClick={() => toggleMetric(m.key)}
            className={`rounded-full px-3 py-1.5 text-xs font-medium ${metrics.includes(m.key) ? 'bg-brand-500 text-white' : 'bg-brand-50 text-brand-700'}`}
          >
            {m.label}
          </button>
        ))}
      </div>

      <p className="mb-1 text-xs font-medium text-slate-600">Group by</p>
      <div className="mb-3 flex flex-wrap gap-2">
        {GROUP_BY_OPTIONS.map((g) => (
          <button
            key={g.key}
            onClick={() => setGroupBy(g.key)}
            className={`rounded-full px-3 py-1.5 text-xs font-medium ${groupBy === g.key ? 'bg-brand-500 text-white' : 'bg-brand-50 text-brand-700'}`}
          >
            {g.label}
          </button>
        ))}
      </div>

      <p className="mb-1 text-xs font-medium text-slate-600">Date range</p>
      <div className="mb-3 flex flex-wrap gap-2">
        {RANGE_OPTIONS.map((r) => (
          <button
            key={r.key}
            onClick={() => pickRange(r.key)}
            className={`rounded-full px-3 py-1.5 text-xs font-medium ${range === r.key ? 'bg-brand-500 text-white' : 'bg-brand-50 text-brand-700'}`}
          >
            {r.label}
          </button>
        ))}
      </div>

      <div className="mb-3 flex gap-2">
        <label className="flex-1 text-xs text-slate-500">
          From
          <input
            type="date"
            className="mt-1 w-full rounded-xl border border-brand-100 px-2 py-1.5 text-sm"
            value={from}
            onChange={(e) => setDates((d) => ({ ...d, from: e.target.value }))}
          />
        </label>
        <label className="flex-1 text-xs text-slate-500">
          To
          <input
            type="date"
            className="mt-1 w-full rounded-xl border border-brand-100 px-2 py-1.5 text-sm"
            value={to}
            onChange={(e) => setDates((d) => ({ ...d, to: e.target.value }))}
          />
        </label>
      </div>

      <button className="w-full rounded-xl bg-brand-500 py-2.5 text-sm font-semibold text-white disabled:opacity-50" onClick={run} disabled={metrics.length === 0 || loading}>
        {loading ? 'Running…' : 'Run query'}
      </button>

      {error && <p className="mt-3 text-xs text-red-500">{error}</p>}

      {result && <CustomStatsResults result={result} />}
    </section>
  )
}

const BRAND = '#cc4f82'
const BRAND_WASH = '#fbdde9'
const GRID = '#e6e1e6'

type Bar = { label: string; value: number }

/** Small inline-SVG bar chart: single brand hue, rounded data-ends, hairline baseline, value at each tip. */
function BarChart({ bars, unit, height = 140 }: { bars: Bar[]; unit: string; height?: number }) {
  const width = 300
  const paddingLeft = 8
  const paddingRight = 8
  const paddingTop = 20
  const paddingBottom = 22
  const plotW = width - paddingLeft - paddingRight
  const plotH = height - paddingTop - paddingBottom
  const max = Math.max(...bars.map((b) => b.value), 1)
  const gap = 6
  const barW = Math.max((plotW - gap * (bars.length - 1)) / bars.length, 4)

  return (
    <svg viewBox={`0 0 ${width} ${height}`} className="w-full" role="img" aria-label={`Bar chart of ${unit} over time`}>
      <line x1={paddingLeft} y1={height - paddingBottom} x2={width - paddingRight} y2={height - paddingBottom} stroke={GRID} strokeWidth={1} />
      {bars.map((b, i) => {
        const barH = Math.max((b.value / max) * plotH, 2)
        const x = paddingLeft + i * (barW + gap)
        const y = height - paddingBottom - barH
        return (
          <g key={i}>
            <title>{`${b.label}: ${b.value} ${unit}`}</title>
            <rect x={x} y={y} width={barW} height={barH} rx={4} fill={BRAND} />
            <text x={x + barW / 2} y={y - 4} textAnchor="middle" fontSize={9} fill="#475569">
              {b.value}
            </text>
            <text x={x + barW / 2} y={height - 8} textAnchor="middle" fontSize={7} fill="#94a3b8">
              {b.label}
            </text>
          </g>
        )
      })}
    </svg>
  )
}

/** Horizontal bars for a categorical breakdown, sorted by magnitude, single hue with a lighter track. */
function HorizontalBars({ rows }: { rows: { label: string; count: number; pct: number }[] }) {
  return (
    <div className="flex flex-col gap-2.5">
      {rows.map((r) => (
        <div key={r.label}>
          <div className="mb-1 flex items-center justify-between text-xs text-slate-500">
            <span className="font-medium text-slate-700">{r.label}</span>
            <span>
              {r.count} · {r.pct}%
            </span>
          </div>
          <div className="h-2 w-full rounded-full" style={{ background: BRAND_WASH }}>
            <div className="h-2 rounded-full" style={{ width: `${Math.max(r.pct, 4)}%`, background: BRAND }} />
          </div>
        </div>
      ))}
    </div>
  )
}

export default function Stats() {
  const { data } = useDashboard()

  const gaps = useMemo(() => cycleGaps(data?.periods ?? []), [data])
  const lengths = useMemo(() => periodLengths(data?.periods ?? []), [data])
  const breakdown = useMemo(() => symptomBreakdown(data?.symptoms ?? []), [data])

  const cycleBars: Bar[] = gaps.slice(-8).map((g) => ({ label: g.start.slice(5), value: g.gapDays }))
  const lengthBars: Bar[] = lengths.slice(-8).map((p) => ({ label: p.start.slice(5), value: p.lengthDays }))

  return (
    <main className="flex flex-col gap-4 p-4">
      <h1 className="text-xl font-bold text-slate-800">Stats</h1>

      <section className="rounded-3xl bg-white p-5 shadow-sm">
        <h2 className="mb-1 font-semibold text-slate-800">Cycle length</h2>
        <p className="mb-3 text-xs text-slate-400">Days between the start of each period.</p>
        {cycleBars.length === 0 ? (
          <p className="text-sm text-slate-400">Log at least two periods to see this trend.</p>
        ) : (
          <BarChart bars={cycleBars} unit="days" />
        )}
      </section>

      <section className="rounded-3xl bg-white p-5 shadow-sm">
        <h2 className="mb-1 font-semibold text-slate-800">Period length</h2>
        <p className="mb-3 text-xs text-slate-400">Days from start to end of each completed period.</p>
        {lengthBars.length === 0 ? (
          <p className="text-sm text-slate-400">Log a completed period (with an end date) to see this trend.</p>
        ) : (
          <BarChart bars={lengthBars} unit="days" />
        )}
      </section>

      <section className="rounded-3xl bg-white p-5 shadow-sm">
        <h2 className="mb-3 font-semibold text-slate-800">Symptom breakdown</h2>
        {breakdown.length === 0 ? (
          <p className="text-sm text-slate-400">No symptoms logged yet.</p>
        ) : (
          <HorizontalBars rows={breakdown.map((s) => ({ label: s.kind, count: s.count, pct: s.pct }))} />
        )}
      </section>

      <CustomStatsQuery />

      <p className="px-1 text-center text-xs text-slate-400">Estimates are not medical advice.</p>
    </main>
  )
}
