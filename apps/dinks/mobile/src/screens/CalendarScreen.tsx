import { useMemo, useState } from 'react'
import { Pressable, ScrollView, StyleSheet, Text, View } from 'react-native'
import type { Dashboard } from '@dinks/shared'
import { periodDaySet, periodOnDay, predictCycle, toISO, todayISO, formatDateRange, formatShortDate } from '@dinks/shared'
import type { ApiClient } from '../api'
import { COLORS } from '../lib/theme'
import LogDay from '../components/LogDay'

const WEEKDAYS = ['S', 'M', 'T', 'W', 'T', 'F', 'S']
const MONTHS = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December']

type Props = { client: ApiClient; data?: Dashboard; refresh: () => Promise<void>; selectedDate: string; onSelect: (iso: string) => void }

export default function CalendarScreen({ client, data, refresh, selectedDate, onSelect }: Props) {
  const todayIso = todayISO()
  const [month, setMonth] = useState(() => new Date())
  const [openDate, setOpenDate] = useState<string | undefined>()

  const periodDates = useMemo(() => periodDaySet(data?.periods ?? [], todayIso), [data, todayIso])
  const symptomDates = useMemo(() => {
    const set = new Set<string>()
    for (const s of data?.symptoms ?? []) set.add(s.recorded_on)
    return set
  }, [data])
  // Widen the server's single `next_period` start into the expected period span,
  // plus the ovulation window for the cycle that ends at that start.
  const predictions = useMemo(() => predictCycle(data?.periods ?? [], data?.next_period, todayIso), [data, todayIso])
  const predictedDays = useMemo(() => new Set(predictions.nextPeriod?.days ?? []), [predictions])
  const ovulationDays = useMemo(() => new Set(predictions.ovulation?.days ?? []), [predictions])

  const grid = useMemo(() => {
    const first = new Date(month.getFullYear(), month.getMonth(), 1)
    const startOffset = first.getDay()
    const daysInMonth = new Date(month.getFullYear(), month.getMonth() + 1, 0).getDate()
    const cells: (Date | null)[] = []
    for (let i = 0; i < startOffset; i++) cells.push(null)
    for (let d = 1; d <= daysInMonth; d++) cells.push(new Date(month.getFullYear(), month.getMonth(), d))
    while (cells.length % 7 !== 0) cells.push(null)
    return cells
  }, [month])

  function handlePress(iso: string) {
    if (iso === selectedDate) setOpenDate(iso)
    else onSelect(iso)
  }

  return (
    <ScrollView contentContainerStyle={styles.page}>
      <View style={styles.headerRow}>
        <Text style={styles.title}>Calendar</Text>
        <Text style={styles.today}>Today: {todayIso}</Text>
      </View>

      <View style={styles.card}>
        <View style={styles.monthRow}>
          <Pressable onPress={() => setMonth((m) => new Date(m.getFullYear(), m.getMonth() - 1, 1))}>
            <Text style={styles.monthArrow}>‹</Text>
          </Pressable>
          <Text style={styles.monthLabel}>
            {MONTHS[month.getMonth()]} {month.getFullYear()}
          </Text>
          <Pressable onPress={() => setMonth((m) => new Date(m.getFullYear(), m.getMonth() + 1, 1))}>
            <Text style={styles.monthArrow}>›</Text>
          </Pressable>
        </View>
        <View style={styles.weekdayRow}>
          {WEEKDAYS.map((w, i) => (
            <Text key={i} style={styles.weekdayText}>
              {w}
            </Text>
          ))}
        </View>
        <View style={styles.grid}>
          {grid.map((d, i) => {
            if (!d) return <View key={i} style={styles.cell} />
            const iso = toISO(d)
            const isToday = iso === todayIso
            const isSelected = iso === selectedDate
            const isPeriod = periodDates.has(iso)
            const isPredicted = predictedDays.has(iso)
            const isOvulation = ovulationDays.has(iso)
            const isOvulationPeak = iso === predictions.ovulation?.peak
            const isSymptom = symptomDates.has(iso)
            return (
              <Pressable key={i} style={styles.cell} onPress={() => handlePress(iso)}>
                <View
                  style={[
                    styles.cellInner,
                    isPeriod && { backgroundColor: COLORS.brand500 },
                    isPredicted && !isPeriod && styles.predictedCell,
                    isOvulation && !isOvulationPeak && !isPeriod && styles.ovulationCell,
                    isOvulationPeak && !isPeriod && styles.ovulationPeakCell,
                    isToday && !isPeriod && styles.todayCell,
                    isSelected && !isPeriod && styles.selectedCell,
                  ]}
                >
                  <Text style={[styles.cellText, (isPeriod || isOvulationPeak) && { color: COLORS.white }]}>{d.getDate()}</Text>
                  {isSymptom && <View style={styles.cellDot} />}
                </View>
              </Pressable>
            )
          })}
        </View>
      </View>

      {(predictions.nextPeriod || predictions.ovulation) && (
        <Text style={styles.predictionLine}>
          {predictions.nextPeriod ? `Next period ${formatDateRange(predictions.nextPeriod.start, predictions.nextPeriod.end)}` : ''}
          {predictions.nextPeriod && predictions.ovulation ? ' · ' : ''}
          {predictions.ovulation ? `Ovulation ~${formatShortDate(predictions.ovulation.peak)}` : ''}
        </Text>
      )}

      <View style={styles.legendRow}>
        <View style={styles.legendItem}>
          <View style={[styles.legendDot, { backgroundColor: COLORS.brand500 }]} />
          <Text style={styles.legendText}>Period</Text>
        </View>
        <View style={styles.legendItem}>
          <View style={[styles.legendDot, { borderWidth: 2, borderColor: COLORS.brand500, backgroundColor: 'transparent' }]} />
          <Text style={styles.legendText}>Predicted period</Text>
        </View>
        <View style={styles.legendItem}>
          <View style={[styles.legendDot, { borderWidth: 2, borderColor: COLORS.ovulation300, backgroundColor: 'transparent' }]} />
          <Text style={styles.legendText}>Ovulation</Text>
        </View>
        <View style={styles.legendItem}>
          <View style={[styles.legendDot, { backgroundColor: COLORS.ovulation500 }]} />
          <Text style={styles.legendText}>Ovulation (peak)</Text>
        </View>
        <View style={styles.legendItem}>
          <View style={[styles.legendDot, { width: 6, height: 6, backgroundColor: COLORS.brand700 }]} />
          <Text style={styles.legendText}>Symptom logged</Text>
        </View>
      </View>

      {openDate && (
        <LogDay
          date={openDate}
          period={periodOnDay(data?.periods ?? [], openDate, todayIso)}
          symptoms={(data?.symptoms ?? []).filter((s) => s.recorded_on === openDate)}
          onClose={() => setOpenDate(undefined)}
          onCreatePeriod={async (started, f) => {
            await client.createPeriod({ started_on: started, flow: f, notes: '' })
            await refresh()
          }}
          onUpdatePeriod={async (id, ended, f, notes) => {
            const p = data?.periods.find((x) => x.id === id)
            if (!p) return
            await client.updatePeriod(id, { started_on: p.started_on, ended_on: ended, flow: f, notes })
            await refresh()
          }}
          onCreateSymptom={async (kind) => {
            await client.createSymptom({ recorded_on: openDate, kind, severity: 3, notes: '' })
            await refresh()
          }}
          onDeleteSymptoms={async (ids) => {
            await Promise.all(ids.map((id) => client.deleteSymptom(id)))
            await refresh()
          }}
        />
      )}

      <Text style={styles.disclaimer}>Estimates are not medical advice.</Text>
    </ScrollView>
  )
}

const styles = StyleSheet.create({
  page: { padding: 16, gap: 16, paddingBottom: 100 },
  headerRow: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'baseline' },
  title: { fontSize: 20, fontWeight: '700', color: COLORS.slate800 },
  today: { fontSize: 12, fontWeight: '600', color: COLORS.brand700 },
  card: { borderRadius: 24, padding: 12, backgroundColor: COLORS.white, shadowColor: '#000', shadowOpacity: 0.04, shadowRadius: 8, elevation: 1 },
  monthRow: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', paddingHorizontal: 8, paddingBottom: 8 },
  monthLabel: { fontWeight: '700', color: COLORS.slate800 },
  monthArrow: { fontSize: 20, color: COLORS.brand500, paddingHorizontal: 8 },
  weekdayRow: { flexDirection: 'row' },
  weekdayText: { flex: 1, textAlign: 'center', fontSize: 11, fontWeight: '600', color: COLORS.slate400 },
  grid: { flexDirection: 'row', flexWrap: 'wrap' },
  cell: { width: `${100 / 7}%`, aspectRatio: 1, alignItems: 'center', justifyContent: 'center' },
  cellInner: { width: '78%', height: '78%', borderRadius: 999, alignItems: 'center', justifyContent: 'center' },
  cellText: { fontSize: 13, color: COLORS.slate700 },
  predictedCell: { borderWidth: 2, borderColor: COLORS.brand500, borderStyle: 'dashed' },
  ovulationCell: { borderWidth: 2, borderColor: COLORS.ovulation300 },
  ovulationPeakCell: { backgroundColor: COLORS.ovulation500 },
  todayCell: { borderWidth: 2, borderColor: COLORS.brand700 },
  selectedCell: { backgroundColor: COLORS.brand100 },
  predictionLine: { textAlign: 'center', fontSize: 12, color: COLORS.slate500 },
  cellDot: { position: 'absolute', bottom: 2, width: 4, height: 4, borderRadius: 2, backgroundColor: COLORS.brand700 },
  legendRow: { flexDirection: 'row', flexWrap: 'wrap', gap: 16 },
  legendItem: { flexDirection: 'row', alignItems: 'center', gap: 6 },
  legendDot: { width: 12, height: 12, borderRadius: 6 },
  legendText: { fontSize: 12, color: COLORS.slate500 },
  disclaimer: { textAlign: 'center', fontSize: 11, color: COLORS.slate400 },
})
