import { useMemo, useState } from 'react'
import { Pressable, StyleSheet, Text, View } from 'react-native'
import type { Dashboard } from '@dinks/shared'
import { flowEmoji, flowOnDay, periodOnDay, toDate, toISO, todayISO } from '@dinks/shared'
import { COLORS } from '../lib/theme'

const WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']
const WEEKDAYS_FULL = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']
const MONTHS = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December']

type Props = {
  data?: Dashboard
  displayName: string
  selectedDate: string
  onSelect: (iso: string) => void
  onOpenSettings: () => void
  onOpenCalendar: () => void
}

// Mirrors web/src/components/TopBar.tsx: the same avatar / date header, the same
// day strip states (selected, ongoing, closed, predicted, symptom) and the same
// Today / Past day / Future day wording.
export default function TopBar({ data, displayName, selectedDate, onSelect, onOpenSettings, onOpenCalendar }: Props) {
  const todayIso = todayISO()
  const [windowOffset, setWindowOffset] = useState(0)
  const initial = displayName ? displayName.trim().charAt(0).toUpperCase() : '?'

  const strip = useMemo(() => {
    const base = new Date()
    const days: { iso: string; weekday: string; dayNum: number }[] = []
    for (let offset = -3; offset <= 3; offset++) {
      const d = new Date(base.getFullYear(), base.getMonth(), base.getDate() + windowOffset + offset)
      days.push({ iso: toISO(d), weekday: WEEKDAYS[d.getDay()], dayNum: d.getDate() })
    }
    return days
  }, [windowOffset])

  const selected = useMemo(() => toDate(selectedDate), [selectedDate])

  return (
    <View style={styles.header}>
      <View style={styles.titleRow}>
        <Pressable style={styles.avatar} onPress={onOpenSettings} accessibilityLabel="Profile and settings">
          <Text style={styles.avatarText}>{initial}</Text>
        </Pressable>
        <View style={styles.titleCenter}>
          <Text style={styles.title}>
            {WEEKDAYS_FULL[selected.getDay()]}, {MONTHS[selected.getMonth()]} {selected.getDate()}
          </Text>
          <Text style={styles.titleStatus}>{selectedDate === todayIso ? 'Today' : selectedDate < todayIso ? 'Past day' : 'Future day'}</Text>
        </View>
        <Pressable style={styles.calendarBtn} onPress={onOpenCalendar} accessibilityLabel="Open calendar">
          <Text style={styles.calendarGlyph}>🗓</Text>
        </Pressable>
      </View>

      <View style={styles.stripRow}>
        <Pressable style={styles.arrow} onPress={() => setWindowOffset((o) => o - 3)} accessibilityLabel="Show earlier days">
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
            const onPeriod = ongoing || closed
            const flow = period ? flowOnDay(period, day.iso) : undefined

            const cellStyle = isSelected || ongoing ? styles.cellOn : closed ? styles.cellClosed : predicted ? styles.cellPredicted : null
            const textColor = isSelected || ongoing ? COLORS.white : closed ? COLORS.brand700 : COLORS.slate500
            const todayCircle = isToday ? (isSelected ? styles.todayOnSelected : onPeriod ? styles.todayOnPeriod : styles.todayIdle) : null
            const todayText = isToday ? (isSelected ? COLORS.brand700 : COLORS.white) : undefined

            return (
              <Pressable
                key={day.iso}
                style={[styles.day, cellStyle]}
                onPress={() => onSelect(day.iso)}
                accessibilityLabel={`${WEEKDAYS_FULL[toDate(day.iso).getDay()]} ${day.dayNum}${isToday ? ', today' : ''}${isSelected ? ', selected' : ''}${onPeriod ? ', period day' : ''}`}
              >
                <Text style={[styles.weekday, { color: textColor }]}>{day.weekday.toUpperCase()}</Text>
                <View style={styles.dayNumRow}>
                  <View style={[styles.dayNumWrap, todayCircle]}>
                    <Text style={[styles.dayNum, { color: todayText ?? textColor }]}>{day.dayNum}</Text>
                  </View>
                  {onPeriod && <Text style={styles.flow}>{flowEmoji(flow ?? 'unknown')}</Text>}
                </View>
                {symptom && <View style={styles.dot} />}
              </Pressable>
            )
          })}
        </View>
        <Pressable style={styles.arrow} onPress={() => setWindowOffset((o) => o + 3)} accessibilityLabel="Show later days">
          <Text style={styles.arrowText}>›</Text>
        </Pressable>
      </View>
    </View>
  )
}

const styles = StyleSheet.create({
  header: { paddingHorizontal: 12, paddingTop: 8, paddingBottom: 10, backgroundColor: COLORS.bg },
  titleRow: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', marginBottom: 12 },
  avatar: { width: 36, height: 36, borderRadius: 18, backgroundColor: COLORS.brand500, alignItems: 'center', justifyContent: 'center' },
  avatarText: { color: COLORS.white, fontWeight: '700', fontSize: 14 },
  titleCenter: { flex: 1, alignItems: 'center' },
  title: { fontSize: 14, fontWeight: '700', color: COLORS.slate800 },
  titleStatus: { fontSize: 10, fontWeight: '600', letterSpacing: 0.6, textTransform: 'uppercase', color: COLORS.brand700, marginTop: 1 },
  calendarBtn: { width: 36, height: 36, borderRadius: 18, backgroundColor: COLORS.white, alignItems: 'center', justifyContent: 'center' },
  calendarGlyph: { fontSize: 16 },
  stripRow: { flexDirection: 'row', alignItems: 'center', gap: 2 },
  arrow: { width: 20, height: 32, alignItems: 'center', justifyContent: 'center' },
  arrowText: { fontSize: 18, color: '#cbd5e1' },
  strip: { flex: 1, flexDirection: 'row', justifyContent: 'space-between' },
  day: { flex: 1, alignItems: 'center', gap: 4, borderRadius: 16, paddingVertical: 8, marginHorizontal: 1 },
  cellOn: { backgroundColor: COLORS.brand500 },
  cellClosed: { backgroundColor: COLORS.brand100 },
  cellPredicted: { borderWidth: 2, borderColor: COLORS.brand500, borderStyle: 'dashed' },
  weekday: { fontSize: 10, fontWeight: '600', letterSpacing: 0.5 },
  dayNumRow: { flexDirection: 'row', alignItems: 'center', gap: 2 },
  dayNumWrap: { width: 24, height: 24, borderRadius: 12, alignItems: 'center', justifyContent: 'center' },
  todayOnSelected: { backgroundColor: COLORS.white },
  todayOnPeriod: { borderWidth: 2, borderColor: COLORS.white },
  todayIdle: { backgroundColor: COLORS.brand500 },
  dayNum: { fontSize: 13, fontWeight: '700' },
  flow: { fontSize: 10 },
  dot: { position: 'absolute', bottom: 3, width: 5, height: 5, borderRadius: 3, backgroundColor: COLORS.brand700 },
})
