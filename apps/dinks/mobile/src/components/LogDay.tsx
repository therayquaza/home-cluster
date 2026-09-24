import { useEffect, useMemo, useState } from 'react'
import { Modal, Pressable, ScrollView, StyleSheet, Switch, Text, TextInput, View } from 'react-native'
import type { Period, Symptom } from '@dinks/shared'
import { symptomEmoji } from '../lib/emoji'
import { COLORS } from '../lib/theme'

export const SYMPTOM_KINDS = ['Cramps', 'Headache', 'Bloating', 'Fatigue', 'Tender breasts', 'Mood swings', 'Acne', 'Nausea']

type Props = {
  date: string
  period?: Period
  symptoms: Symptom[]
  onClose: () => void
  onCreatePeriod: (started: string, flow: string) => Promise<void>
  onUpdatePeriod: (id: number, ended: string | undefined, flow: string, notes: string) => Promise<void>
  onCreateSymptom: (kind: string) => Promise<void>
  onDeleteSymptoms: (ids: number[]) => Promise<void>
}

export default function LogDay({ date, period, symptoms, onClose, onCreatePeriod, onUpdatePeriod, onCreateSymptom, onDeleteSymptoms }: Props) {
  const [periodOn, setPeriodOn] = useState(!!period)
  const [flow, setFlow] = useState(period?.flow ?? 'medium')
  const [endOn, setEndOn] = useState(period?.ended_on ?? '')
  const [periodNotes, setPeriodNotes] = useState(period?.notes ?? '')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    setPeriodOn(!!period)
    setFlow(period?.flow ?? 'medium')
    setEndOn(period?.ended_on ?? '')
    setPeriodNotes(period?.notes ?? '')
  }, [period?.id, period?.flow, period?.ended_on, period?.notes])

  const allByKind = useMemo(() => {
    const map = new Map<string, Symptom[]>()
    for (const s of symptoms) map.set(s.kind, [...(map.get(s.kind) ?? []), s])
    return map
  }, [symptoms])

  async function togglePeriod(next: boolean) {
    setPeriodOn(next)
    setBusy(true)
    try {
      if (next && !period) await onCreatePeriod(date, flow)
      else if (!next && period) await onUpdatePeriod(period.id, date, period.flow, period.notes ?? '')
    } finally {
      setBusy(false)
    }
  }

  async function toggleSymptom(kind: string, next: boolean) {
    const existing = allByKind.get(kind) ?? []
    setBusy(true)
    try {
      if (next && !existing.length) await onCreateSymptom(kind)
      else if (!next && existing.length) await onDeleteSymptoms(existing.map((s) => s.id))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal visible animationType="slide" transparent onRequestClose={onClose}>
      <View style={styles.backdrop}>
        <View style={styles.sheet}>
          <View style={styles.sheetHeader}>
            <Text style={styles.sheetTitle}>{date}</Text>
            <Pressable onPress={onClose}>
              <Text style={styles.done}>Done</Text>
            </Pressable>
          </View>
          <ScrollView contentContainerStyle={{ padding: 20, gap: 20 }}>
            <View style={styles.periodCard}>
              <View style={styles.switchRow}>
                <Text style={styles.periodLabel}>On my period</Text>
                <Switch value={periodOn} onValueChange={togglePeriod} disabled={busy} trackColor={{ true: COLORS.brand500, false: COLORS.brand200 }} />
              </View>
              {periodOn && (
                <View style={{ marginTop: 12, gap: 10 }}>
                  <Text style={styles.fieldLabel}>Flow</Text>
                  <View style={styles.pillRow}>
                    {['light', 'medium', 'heavy'].map((f) => (
                      <Pressable key={f} onPress={() => setFlow(f)} style={[styles.pill, flow === f && styles.pillActive]}>
                        <Text style={[styles.pillText, flow === f && styles.pillTextActive]}>{f}</Text>
                      </Pressable>
                    ))}
                  </View>
                  <Text style={styles.fieldLabel}>End date (leave blank if ongoing)</Text>
                  <TextInput style={styles.input} placeholder="YYYY-MM-DD" value={endOn} onChangeText={setEndOn} onBlur={() => period && onUpdatePeriod(period.id, endOn || undefined, flow, periodNotes)} />
                  <Text style={styles.fieldLabel}>Notes</Text>
                  <TextInput style={styles.input} value={periodNotes} onChangeText={setPeriodNotes} onBlur={() => period && onUpdatePeriod(period.id, endOn || undefined, flow, periodNotes)} multiline />
                </View>
              )}
            </View>

            <View>
              <Text style={styles.sectionTitle}>Symptoms</Text>
              <View style={{ gap: 8, marginTop: 8 }}>
                {SYMPTOM_KINDS.map((kind) => {
                  const checked = (allByKind.get(kind) ?? []).length > 0
                  return (
                    <View key={kind} style={styles.symptomRow}>
                      <Text style={styles.symptomLabel}>
                        {symptomEmoji(kind)} {kind}
                      </Text>
                      <Switch value={checked} onValueChange={(v) => toggleSymptom(kind, v)} disabled={busy} trackColor={{ true: COLORS.brand500, false: '#e2e8f0' }} />
                    </View>
                  )
                })}
              </View>
            </View>
          </ScrollView>
        </View>
      </View>
    </Modal>
  )
}

const styles = StyleSheet.create({
  backdrop: { flex: 1, backgroundColor: 'rgba(0,0,0,0.3)', justifyContent: 'flex-end' },
  sheet: { backgroundColor: COLORS.white, borderTopLeftRadius: 24, borderTopRightRadius: 24, maxHeight: '85%' },
  sheetHeader: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', padding: 16, borderBottomWidth: 1, borderBottomColor: '#f1f5f9' },
  sheetTitle: { fontSize: 17, fontWeight: '700', color: COLORS.slate800 },
  done: { color: COLORS.slate400, fontWeight: '600' },
  periodCard: { backgroundColor: COLORS.brand50, borderRadius: 16, padding: 16 },
  switchRow: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' },
  periodLabel: { fontWeight: '700', color: COLORS.brand700 },
  fieldLabel: { fontSize: 12, fontWeight: '600', color: COLORS.slate600 },
  pillRow: { flexDirection: 'row', gap: 8 },
  pill: { paddingHorizontal: 14, paddingVertical: 8, borderRadius: 999, backgroundColor: COLORS.white, borderWidth: 1, borderColor: COLORS.brand100 },
  pillActive: { backgroundColor: COLORS.brand500, borderColor: COLORS.brand500 },
  pillText: { fontSize: 13, color: COLORS.slate600, textTransform: 'capitalize' },
  pillTextActive: { color: COLORS.white },
  input: { backgroundColor: COLORS.white, borderWidth: 1, borderColor: COLORS.brand100, borderRadius: 12, padding: 10, fontSize: 14 },
  sectionTitle: { fontWeight: '700', color: COLORS.slate700 },
  symptomRow: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', backgroundColor: COLORS.white, borderRadius: 12, padding: 12, shadowColor: '#000', shadowOpacity: 0.04, shadowRadius: 4, elevation: 1 },
  symptomLabel: { fontSize: 14, fontWeight: '600', color: COLORS.slate700 },
})
