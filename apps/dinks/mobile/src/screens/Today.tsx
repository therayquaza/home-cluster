import { useEffect, useMemo, useState } from 'react'
import { Alert, Pressable, ScrollView, StyleSheet, Text, TextInput, View } from 'react-native'
import type { Dashboard, Prediction } from '@dinks/shared'
import { mobileAPI } from '../api'
import { periodOnDay } from '../lib/periods'
import { todayISO } from '../lib/dates'
import { symptomEmoji, moodEmoji } from '../lib/emoji'
import { COLORS } from '../lib/theme'
import LogDay from '../components/LogDay'

const moods = ['Calm', 'Happy', 'Sensitive', 'Irritable', 'Anxious', 'Sad', 'Energetic', 'Tired', 'Confident', 'Stressed', 'Grateful']
const symptomKinds = ['Cramps', 'Headache', 'Bloating', 'Fatigue', 'Tender breasts']

type Props = { client: ReturnType<typeof mobileAPI>; data?: Dashboard; refresh: () => Promise<void>; selectedDate: string }

export default function Today({ client, data, refresh, selectedDate }: Props) {
  const day = selectedDate
  const todayIso = todayISO()
  const isToday = day === todayIso
  const isFuture = day > todayIso
  const [flow, setFlow] = useState('medium')
  const [moodSelection, setMoodSelection] = useState<string[]>(['Calm'])
  const [note, setNote] = useState('')
  const [editorOpen, setEditorOpen] = useState(false)
  const [prediction, setPrediction] = useState<Prediction>()

  useEffect(() => {
    client.prediction().then(setPrediction).catch(() => undefined)
  }, [data])

  const active = useMemo(() => data?.periods.find((p) => !p.ended_on), [data])
  const periodForDay = useMemo(() => periodOnDay(data?.periods ?? [], day, todayIso), [data, day, todayIso])
  const ongoingForDay = !!periodForDay && !periodForDay.ended_on
  const todaysSymptoms = useMemo(() => (data?.symptoms ?? []).filter((s) => s.recorded_on === day), [data, day])
  const savedNotes = useMemo(() => todaysSymptoms.filter((s) => s.kind === 'note'), [todaysSymptoms])

  async function endPeriod() {
    if (!active) return
    await client.updatePeriod(active.id, { started_on: active.started_on, ended_on: day, flow: active.flow, notes: active.notes }).catch((e) => Alert.alert('Failed', e.message))
    refresh()
  }

  async function startPeriod() {
    if (isFuture) return
    await client.createPeriod({ started_on: day, flow, notes: '' }).catch((e) => Alert.alert('Failed', e.message))
    refresh()
  }

  async function toggleSymptom(kind: string) {
    const existing = todaysSymptoms.filter((s) => s.kind === kind)
    try {
      if (existing.length) await Promise.all(existing.map((s) => client.deleteSymptom(s.id)))
      else await client.createSymptom({ recorded_on: day, kind, severity: 3, notes: '' })
      await refresh()
    } catch (e: any) {
      Alert.alert('Failed', e.message)
    }
  }

  function toggleMood(m: string) {
    setMoodSelection((prev) => (prev.includes(m) ? prev.filter((x) => x !== m) : [...prev, m]))
  }

  async function saveNote() {
    if (!note.trim()) return
    const moodText = moodSelection.length ? moodSelection.join(', ') : 'none'
    await client.createSymptom({ recorded_on: day, kind: 'note', severity: 3, notes: `Mood: ${moodText}. ${note}` }).catch((e) => Alert.alert('Failed', e.message))
    setNote('')
    refresh()
  }

  return (
    <ScrollView contentContainerStyle={styles.page}>
      <View style={styles.hero}>
        <Text style={styles.eyebrow}>{isToday ? 'TODAY' : day}</Text>
        <Text style={styles.heroTitle}>{ongoingForDay ? 'On your period' : periodForDay ? 'Period logged' : 'Not on your period'}</Text>
        <Text style={styles.heroSub}>
          {periodForDay
            ? `Started ${periodForDay.started_on}${periodForDay.ended_on ? `, ended ${periodForDay.ended_on}` : ''}.`
            : prediction?.predicted_period_start
              ? `Predicted next period: ${prediction.predicted_period_start}.`
              : 'Log two periods to receive an estimate.'}
        </Text>

        {isFuture ? (
          <Text style={styles.hint}>Can't log a period for a future day.</Text>
        ) : (
          <>
            {ongoingForDay && (
              <Pressable style={styles.primaryBtn} onPress={endPeriod}>
                <Text style={styles.primaryBtnText}>Log period end</Text>
              </Pressable>
            )}
            {!periodForDay && active && <Text style={styles.hint}>You already have an active period started {active.started_on}. End it before starting a new one.</Text>}
            {!periodForDay && !active && (
              <>
                <View style={styles.pillRow}>
                  {['light', 'medium', 'heavy'].map((f) => (
                    <Pressable key={f} onPress={() => setFlow(f)} style={[styles.flowPill, flow === f && styles.flowPillActive]}>
                      <Text style={[styles.flowPillText, flow === f && styles.flowPillTextActive]}>{f}</Text>
                    </Pressable>
                  ))}
                </View>
                <Pressable style={styles.primaryBtn} onPress={startPeriod}>
                  <Text style={styles.primaryBtnText}>Log period</Text>
                </Pressable>
              </>
            )}
            {periodForDay && !ongoingForDay && <Text style={styles.hint}>To edit or clear this period, use the + button below.</Text>}
          </>
        )}
        {data?.reminder && <Text style={styles.hint}>{data.reminder}</Text>}
      </View>

      <View style={styles.card}>
        <View style={styles.cardHeaderRow}>
          <Text style={styles.cardTitle}>How are you feeling?</Text>
          <Pressable onPress={() => setEditorOpen(true)}>
            <Text style={styles.moreOptions}>More options</Text>
          </Pressable>
        </View>
        <Text style={styles.cardHint}>Tap a symptom to log it — tap again to remove it.</Text>
        <View style={styles.chipWrap}>
          {symptomKinds.map((k) => {
            const logged = todaysSymptoms.some((s) => s.kind === k)
            return (
              <Pressable key={k} onPress={() => toggleSymptom(k)} style={[styles.chip, logged && styles.chipActive]}>
                <Text style={[styles.chipText, logged && styles.chipTextActive]}>
                  {logged ? '✓ ' : ''}
                  {symptomEmoji(k)} {k}
                </Text>
              </Pressable>
            )
          })}
        </View>
      </View>

      <View style={styles.card}>
        <Text style={styles.cardTitle}>Add a note</Text>
        <Text style={styles.cardHint}>Optional — mood, sleep, energy, anything worth remembering.</Text>
        <Text style={styles.fieldLabel}>Mood (pick any)</Text>
        <View style={styles.chipWrap}>
          {moods.map((m) => {
            const picked = moodSelection.includes(m)
            return (
              <Pressable key={m} onPress={() => toggleMood(m)} style={[styles.chip, picked && styles.chipActive]}>
                <Text style={[styles.chipText, picked && styles.chipTextActive]}>
                  {picked ? '✓ ' : ''}
                  {moodEmoji(m)} {m}
                </Text>
              </Pressable>
            )
          })}
        </View>
        <TextInput style={styles.textarea} placeholder="Notes, sleep, energy, bowel movements…" value={note} onChangeText={setNote} multiline />
        <Pressable style={[styles.primaryBtn, !note.trim() && styles.disabled]} onPress={saveNote} disabled={!note.trim()}>
          <Text style={styles.primaryBtnText}>Save note</Text>
        </Pressable>
        {savedNotes.length > 0 && (
          <View style={{ marginTop: 14, gap: 8 }}>
            <Text style={styles.fieldLabel}>Saved notes for this day</Text>
            {savedNotes.map((n) => (
              <View key={n.id} style={styles.noteRow}>
                <Text style={styles.noteText}>{n.notes}</Text>
              </View>
            ))}
          </View>
        )}
      </View>

      <Text style={styles.disclaimer}>{prediction?.disclaimer ?? 'Estimates are not medical advice.'}</Text>

      <Pressable style={styles.fab} onPress={() => setEditorOpen(true)}>
        <Text style={styles.fabText}>+</Text>
      </Pressable>

      {editorOpen && (
        <LogDay
          date={day}
          period={periodForDay}
          symptoms={todaysSymptoms}
          onClose={() => setEditorOpen(false)}
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
            await client.createSymptom({ recorded_on: day, kind, severity: 3, notes: '' })
            await refresh()
          }}
          onDeleteSymptoms={async (ids) => {
            await Promise.all(ids.map((id) => client.deleteSymptom(id)))
            await refresh()
          }}
        />
      )}
    </ScrollView>
  )
}

const styles = StyleSheet.create({
  page: { padding: 16, gap: 16, paddingBottom: 100 },
  hero: { borderRadius: 24, padding: 24, backgroundColor: COLORS.brand200 },
  eyebrow: { fontSize: 11, fontWeight: '700', letterSpacing: 1, color: COLORS.brand700, textTransform: 'uppercase' },
  heroTitle: { fontSize: 24, fontWeight: '700', color: COLORS.slate800, marginVertical: 8 },
  heroSub: { fontSize: 13, color: COLORS.slate600 },
  hint: { marginTop: 12, fontSize: 12, color: COLORS.brand700 },
  primaryBtn: { marginTop: 16, backgroundColor: COLORS.brand500, borderRadius: 12, paddingVertical: 12, alignItems: 'center' },
  primaryBtnText: { color: COLORS.white, fontWeight: '700' },
  disabled: { opacity: 0.4 },
  pillRow: { flexDirection: 'row', gap: 8, marginTop: 10 },
  flowPill: { flex: 1, paddingVertical: 8, borderRadius: 10, backgroundColor: COLORS.white, alignItems: 'center', borderWidth: 1, borderColor: COLORS.brand200 },
  flowPillActive: { backgroundColor: COLORS.brand500, borderColor: COLORS.brand500 },
  flowPillText: { fontSize: 13, color: COLORS.slate600, textTransform: 'capitalize' },
  flowPillTextActive: { color: COLORS.white },
  card: { borderRadius: 24, padding: 20, backgroundColor: COLORS.white, shadowColor: '#000', shadowOpacity: 0.04, shadowRadius: 8, elevation: 1 },
  cardHeaderRow: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' },
  cardTitle: { fontWeight: '700', color: COLORS.slate800, fontSize: 15 },
  moreOptions: { fontSize: 12, fontWeight: '700', color: COLORS.brand500 },
  cardHint: { fontSize: 12, color: COLORS.slate400, marginTop: 2, marginBottom: 10 },
  chipWrap: { flexDirection: 'row', flexWrap: 'wrap', gap: 8 },
  chip: { paddingHorizontal: 12, paddingVertical: 8, borderRadius: 999, backgroundColor: COLORS.brand50 },
  chipActive: { backgroundColor: COLORS.brand500 },
  chipText: { fontSize: 13, fontWeight: '600', color: COLORS.brand700 },
  chipTextActive: { color: COLORS.white },
  fieldLabel: { fontSize: 13, fontWeight: '600', color: COLORS.slate600, marginBottom: 8 },
  textarea: { marginTop: 14, borderWidth: 1, borderColor: COLORS.brand100, borderRadius: 12, padding: 12, fontSize: 14, minHeight: 70, textAlignVertical: 'top' },
  noteRow: { backgroundColor: '#f8fafc', borderRadius: 12, padding: 12 },
  noteText: { fontSize: 13, color: COLORS.slate600 },
  disclaimer: { textAlign: 'center', fontSize: 11, color: COLORS.slate400, paddingHorizontal: 4 },
  fab: { position: 'absolute', bottom: 24, right: 8, width: 56, height: 56, borderRadius: 28, backgroundColor: COLORS.brand500, alignItems: 'center', justifyContent: 'center', shadowColor: '#000', shadowOpacity: 0.2, shadowRadius: 8, elevation: 4 },
  fabText: { color: COLORS.white, fontSize: 28, fontWeight: '300', marginTop: -2 },
})
