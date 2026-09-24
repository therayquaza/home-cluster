import { useMemo, useState } from 'react'
import { Pressable, StyleSheet, Text, View } from 'react-native'
import type { Dashboard } from '@dinks/shared'
import { periodOnDay } from '../lib/periods'
import { toISO, todayISO } from '../lib/dates'
import { COLORS } from '../lib/theme'

const WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']
const WEEKDAYS_FULL = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']
const MONTHS = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December']

type Props = {
  data?: Dashboard
  selectedDate: string
  onSelect: (iso: string) => void
}

export default function DayStrip({ data, selectedDate, onSelect }: Props) {
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
    <View style={styles.header}>
      <Text style={styles.title}>
        {WEEKDAYS_FULL[selected.getDay()]}, {MONTHS[selected.getMonth()]} {selected.getDate()}
      </Text>
      <View style={styles.row}>
        <Pressable style={styles.arrow} onPress={() => setWindowOffset((o) => o - 3)}>
          <Text style={styles.arrowText}>‹</Text>
        </Pressable>
        <View style={styles.strip}>
          {strip.map((day) => {
            const isSelected = day.iso === selectedDate
            const isToday = day.iso === todayIso
            const period = periodOnDay(data?.periods ?? [], day.iso, todayIso)
            const ongoing = !!period && !period.ended_on
            const closed = !!period && !!period.ended_on
            const predicted = day.iso === data?.next_period
            const symptom = data?.symptoms.some((s) => s.recorded_on === day.iso)
            const bg = isSelected || ongoing ? COLORS.brand500 : closed ? COLORS.brand300 : 'transparent'
            const textColor = isSelected || ongoing || closed ? COLORS.white : COLORS.slate500
            return (
              <Pressable key={day.iso} style={[styles.day, { backgroundColor: bg }, predicted && !period && styles.predicted]} onPress={() => onSelect(day.iso)}>
                <Text style={[styles.weekday, { color: textColor }]}>{day.weekday.toUpperCase()}</Text>
                <View style={[styles.dayNumWrap, isToday && { borderWidth: 2, borderColor: bg === 'transparent' ? COLORS.brand500 : COLORS.white }]}>
                  <Text style={[styles.dayNum, { color: textColor }]}>{day.dayNum}</Text>
                </View>
                {symptom && <View style={styles.dot} />}
              </Pressable>
            )
          })}
        </View>
        <Pressable style={styles.arrow} onPress={() => setWindowOffset((o) => o + 3)}>
          <Text style={styles.arrowText}>›</Text>
        </Pressable>
      </View>
    </View>
  )
}

const styles = StyleSheet.create({
  header: { paddingHorizontal: 12, paddingTop: 8, paddingBottom: 10, backgroundColor: COLORS.bg },
  title: { textAlign: 'center', fontSize: 14, fontWeight: '700', color: COLORS.slate800, marginBottom: 8 },
  row: { flexDirection: 'row', alignItems: 'center', gap: 2 },
  arrow: { width: 20, height: 32, alignItems: 'center', justifyContent: 'center' },
  arrowText: { fontSize: 18, color: '#cbd5e1' },
  strip: { flex: 1, flexDirection: 'row', justifyContent: 'space-between' },
  day: { flex: 1, alignItems: 'center', gap: 4, borderRadius: 16, paddingVertical: 8, marginHorizontal: 1 },
  predicted: { borderWidth: 2, borderColor: COLORS.brand500, borderStyle: 'dashed' },
  weekday: { fontSize: 10, fontWeight: '600', letterSpacing: 0.5 },
  dayNumWrap: { width: 24, height: 24, borderRadius: 12, alignItems: 'center', justifyContent: 'center' },
  dayNum: { fontSize: 13, fontWeight: '700' },
  dot: { position: 'absolute', bottom: 3, width: 5, height: 5, borderRadius: 3, backgroundColor: COLORS.brand700 },
})
