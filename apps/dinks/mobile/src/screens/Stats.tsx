import { useMemo, useState } from 'react'
import { Pressable, ScrollView, StyleSheet, Text, TextInput, View } from 'react-native'
import type { Dashboard, StatsGroupBy, StatsMetric, StatsQueryResponse } from '@dinks/shared'
import { mobileAPI } from '../api'
import { cycleGaps, periodLengths, symptomBreakdown } from '../lib/cycleStats'
import { toISO } from '../lib/dates'
import { COLORS } from '../lib/theme'

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

type Bar = { label: string; value: number }

function BarRow({ bars }: { bars: Bar[] }) {
  const max = Math.max(...bars.map((b) => b.value), 1)
  return (
    <View style={{ flexDirection: 'row', gap: 6, height: 100, alignItems: 'flex-end' }}>
      {bars.map((b, i) => (
        <View key={i} style={{ flex: 1, alignItems: 'center' }}>
          <Text style={styles.barValue}>{b.value}</Text>
          <View style={{ width: '100%', height: Math.max((b.value / max) * 70, 4), backgroundColor: COLORS.brand500, borderRadius: 4 }} />
          <Text style={styles.barLabel} numberOfLines={1}>
            {b.label}
          </Text>
        </View>
      ))}
    </View>
  )
}

function HorizontalBars({ rows }: { rows: { label: string; count: number; pct: number }[] }) {
  return (
    <View style={{ gap: 10 }}>
      {rows.map((r) => (
        <View key={r.label}>
          <View style={styles.hbarHeader}>
            <Text style={styles.hbarLabel}>{r.label}</Text>
            <Text style={styles.hbarValue}>
              {r.count} · {r.pct}%
            </Text>
          </View>
          <View style={styles.hbarTrack}>
            <View style={[styles.hbarFill, { width: `${Math.max(r.pct, 4)}%` }]} />
          </View>
        </View>
      ))}
    </View>
  )
}

function CustomStatsResults({ result }: { result: StatsQueryResponse }) {
  const groups = result.results
  const metricKeys = Array.from(new Set(groups.flatMap((g) => Object.keys(g.values))))
  const numericKeys = metricKeys.filter((k) => groups.every((g) => typeof g.values[k] === 'number'))
  const otherKeys = metricKeys.filter((k) => !numericKeys.includes(k))

  return (
    <View style={{ marginTop: 16, gap: 16 }}>
      {numericKeys.map((key) => {
        const label = METRIC_OPTIONS.find((m) => m.key === key)?.label ?? key
        const bars: Bar[] = groups.map((g) => ({ label: g.group || 'All', value: (g.values[key] as number) ?? 0 }))
        return (
          <View key={key}>
            <Text style={styles.metricLabel}>{label}</Text>
            {bars.length === 1 ? <Text style={styles.bigNumber}>{bars[0].value}</Text> : <BarRow bars={bars} />}
          </View>
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
            <View key={`${key}-${g.group}`} style={styles.breakdownCard}>
              <Text style={styles.metricLabel}>
                {label}
                {g.group ? ` — ${g.group}` : ''}
              </Text>
              <View style={{ gap: 8, marginTop: 8 }}>
                {entries.map(([k, v]) => (
                  <View key={k}>
                    <View style={styles.hbarHeader}>
                      <Text style={styles.hbarLabel}>{k}</Text>
                      <Text style={styles.hbarValue}>{v}</Text>
                    </View>
                    <View style={styles.hbarTrackWhite}>
                      <View style={[styles.hbarFill, { width: `${Math.max((v / max) * 100, 4)}%` }]} />
                    </View>
                  </View>
                ))}
              </View>
            </View>
          )
        }),
      )}
    </View>
  )
}

function CustomStatsQuery({ client }: { client: ReturnType<typeof mobileAPI> }) {
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
    } catch (e: any) {
      setResult(undefined)
      setError(e?.message ?? 'Query failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <View style={styles.card}>
      <Text style={styles.cardTitle}>Custom stats</Text>
      <Text style={styles.cardHint}>Build your own view from allowed metrics.</Text>

      <Text style={styles.fieldLabel}>Metrics</Text>
      <View style={styles.chipWrap}>
        {METRIC_OPTIONS.map((m) => (
          <Pressable key={m.key} onPress={() => toggleMetric(m.key)} style={[styles.chip, metrics.includes(m.key) && styles.chipActive]}>
            <Text style={[styles.chipText, metrics.includes(m.key) && styles.chipTextActive]}>{m.label}</Text>
          </Pressable>
        ))}
      </View>

      <Text style={styles.fieldLabel}>Group by</Text>
      <View style={styles.chipWrap}>
        {GROUP_BY_OPTIONS.map((g) => (
          <Pressable key={g.key} onPress={() => setGroupBy(g.key)} style={[styles.chip, groupBy === g.key && styles.chipActive]}>
            <Text style={[styles.chipText, groupBy === g.key && styles.chipTextActive]}>{g.label}</Text>
          </Pressable>
        ))}
      </View>

      <Text style={styles.fieldLabel}>Date range</Text>
      <View style={styles.chipWrap}>
        {RANGE_OPTIONS.map((r) => (
          <Pressable key={r.key} onPress={() => pickRange(r.key)} style={[styles.chip, range === r.key && styles.chipActive]}>
            <Text style={[styles.chipText, range === r.key && styles.chipTextActive]}>{r.label}</Text>
          </Pressable>
        ))}
      </View>

      <View style={{ flexDirection: 'row', gap: 8, marginTop: 8 }}>
        <View style={{ flex: 1 }}>
          <Text style={styles.smallLabel}>From</Text>
          <TextInput style={styles.dateInput} value={from} onChangeText={(v) => setDates((d) => ({ ...d, from: v }))} placeholder="YYYY-MM-DD" />
        </View>
        <View style={{ flex: 1 }}>
          <Text style={styles.smallLabel}>To</Text>
          <TextInput style={styles.dateInput} value={to} onChangeText={(v) => setDates((d) => ({ ...d, to: v }))} placeholder="YYYY-MM-DD" />
        </View>
      </View>

      <Pressable style={[styles.runBtn, (metrics.length === 0 || loading) && { opacity: 0.5 }]} onPress={run} disabled={metrics.length === 0 || loading}>
        <Text style={styles.runBtnText}>{loading ? 'Running…' : 'Run query'}</Text>
      </Pressable>

      {error && <Text style={styles.error}>{error}</Text>}
      {result && <CustomStatsResults result={result} />}
    </View>
  )
}

export default function Stats({ client, data }: { client: ReturnType<typeof mobileAPI>; data?: Dashboard }) {
  const gaps = useMemo(() => cycleGaps(data?.periods ?? []), [data])
  const lengths = useMemo(() => periodLengths(data?.periods ?? []), [data])
  const breakdown = useMemo(() => symptomBreakdown(data?.symptoms ?? []), [data])
  const cycleBars: Bar[] = gaps.slice(-8).map((g) => ({ label: g.start.slice(5), value: g.gapDays }))
  const lengthBars: Bar[] = lengths.slice(-8).map((p) => ({ label: p.start.slice(5), value: p.lengthDays }))

  return (
    <ScrollView contentContainerStyle={styles.page}>
      <Text style={styles.title}>Stats</Text>

      <View style={styles.card}>
        <Text style={styles.cardTitle}>Cycle length</Text>
        <Text style={styles.cardHint}>Days between the start of each period.</Text>
        {cycleBars.length === 0 ? <Text style={styles.empty}>Log at least two periods to see this trend.</Text> : <BarRow bars={cycleBars} />}
      </View>

      <View style={styles.card}>
        <Text style={styles.cardTitle}>Period length</Text>
        <Text style={styles.cardHint}>Days from start to end of each completed period.</Text>
        {lengthBars.length === 0 ? <Text style={styles.empty}>Log a completed period (with an end date) to see this trend.</Text> : <BarRow bars={lengthBars} />}
      </View>

      <View style={styles.card}>
        <Text style={styles.cardTitle}>Symptom breakdown</Text>
        {breakdown.length === 0 ? <Text style={styles.empty}>No symptoms logged yet.</Text> : <HorizontalBars rows={breakdown.map((s) => ({ label: s.kind, count: s.count, pct: s.pct }))} />}
      </View>

      <CustomStatsQuery client={client} />

      <Text style={styles.disclaimer}>Estimates are not medical advice.</Text>
    </ScrollView>
  )
}

const styles = StyleSheet.create({
  page: { padding: 16, gap: 16, paddingBottom: 100 },
  title: { fontSize: 20, fontWeight: '700', color: COLORS.slate800 },
  card: { borderRadius: 24, padding: 20, backgroundColor: COLORS.white, shadowColor: '#000', shadowOpacity: 0.04, shadowRadius: 8, elevation: 1 },
  cardTitle: { fontWeight: '700', color: COLORS.slate800, fontSize: 15 },
  cardHint: { fontSize: 12, color: COLORS.slate400, marginTop: 2, marginBottom: 10 },
  empty: { fontSize: 13, color: COLORS.slate400 },
  barValue: { fontSize: 10, color: COLORS.slate600, marginBottom: 2 },
  barLabel: { fontSize: 9, color: '#94a3b8', marginTop: 4 },
  hbarHeader: { flexDirection: 'row', justifyContent: 'space-between', marginBottom: 4 },
  hbarLabel: { fontSize: 12, fontWeight: '600', color: COLORS.slate700 },
  hbarValue: { fontSize: 12, color: COLORS.slate500 },
  hbarTrack: { height: 8, borderRadius: 4, backgroundColor: COLORS.brand100 },
  hbarTrackWhite: { height: 6, borderRadius: 3, backgroundColor: COLORS.white },
  hbarFill: { height: '100%', borderRadius: 4, backgroundColor: COLORS.brand500 },
  breakdownCard: { backgroundColor: COLORS.brand50, borderRadius: 16, padding: 12 },
  metricLabel: { fontSize: 11, fontWeight: '700', color: COLORS.brand700, textTransform: 'uppercase', letterSpacing: 0.5 },
  bigNumber: { fontSize: 26, fontWeight: '700', color: COLORS.slate800, marginTop: 4 },
  fieldLabel: { fontSize: 12, fontWeight: '600', color: COLORS.slate600, marginTop: 12, marginBottom: 6 },
  smallLabel: { fontSize: 11, color: COLORS.slate500, marginBottom: 4 },
  chipWrap: { flexDirection: 'row', flexWrap: 'wrap', gap: 6 },
  chip: { paddingHorizontal: 10, paddingVertical: 6, borderRadius: 999, backgroundColor: COLORS.brand50 },
  chipActive: { backgroundColor: COLORS.brand500 },
  chipText: { fontSize: 11, fontWeight: '600', color: COLORS.brand700 },
  chipTextActive: { color: COLORS.white },
  dateInput: { borderWidth: 1, borderColor: COLORS.brand100, borderRadius: 10, padding: 8, fontSize: 13 },
  runBtn: { marginTop: 14, backgroundColor: COLORS.brand500, borderRadius: 12, paddingVertical: 10, alignItems: 'center' },
  runBtnText: { color: COLORS.white, fontWeight: '700', fontSize: 13 },
  error: { marginTop: 8, fontSize: 12, color: '#ef4444' },
  disclaimer: { textAlign: 'center', fontSize: 11, color: COLORS.slate400 },
})
