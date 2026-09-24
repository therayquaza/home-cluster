import { useState } from 'react'
import { Alert, Pressable, ScrollView, StyleSheet, Text } from 'react-native'
import { mobileAPI } from '../api'
import { COLORS } from '../lib/theme'

type Props = { client: ReturnType<typeof mobileAPI>; displayName: string; onSignedOut: () => void }

export default function Settings({ client, displayName, onSignedOut }: Props) {
  const [confirmingDelete, setConfirmingDelete] = useState(false)
  const [busy, setBusy] = useState(false)

  async function exportData() {
    try {
      const payload = await client.exportData()
      Alert.alert('Export ready', JSON.stringify(payload).slice(0, 500))
    } catch (e: any) {
      Alert.alert('Failed', e.message)
    }
  }

  async function deleteAccount() {
    setBusy(true)
    try {
      await client.deleteMe()
      onSignedOut()
    } catch (e: any) {
      Alert.alert('Failed', e.message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <ScrollView contentContainerStyle={styles.page}>
      <Text style={styles.title}>Settings</Text>

      <Pressable style={styles.card}>
        <Text style={styles.label}>Signed in as</Text>
        <Text style={styles.value}>{displayName}</Text>
      </Pressable>

      <Pressable style={styles.card} onPress={exportData}>
        <Text style={styles.actionText}>Export my data</Text>
      </Pressable>

      <Pressable style={styles.card} onPress={onSignedOut}>
        <Text style={styles.actionText}>Sign out</Text>
      </Pressable>

      <Pressable style={styles.dangerCard} onPress={() => !confirmingDelete && setConfirmingDelete(true)}>
        <Text style={styles.dangerTitle}>Danger zone</Text>
        {!confirmingDelete ? (
          <Text style={styles.dangerAction}>Delete my account</Text>
        ) : (
          <>
            <Text style={styles.dangerText}>This permanently deletes your account and all data. This cannot be undone.</Text>
            <Pressable style={styles.cancelBtn} onPress={() => setConfirmingDelete(false)}>
              <Text style={styles.cancelText}>Cancel</Text>
            </Pressable>
            <Pressable style={styles.confirmBtn} disabled={busy} onPress={deleteAccount}>
              <Text style={styles.confirmText}>Confirm delete</Text>
            </Pressable>
          </>
        )}
      </Pressable>

      <Text style={styles.disclaimer}>Estimates are not medical advice.</Text>
    </ScrollView>
  )
}

const styles = StyleSheet.create({
  page: { padding: 16, gap: 16, paddingBottom: 100 },
  title: { fontSize: 20, fontWeight: '700', color: COLORS.slate800 },
  card: { borderRadius: 20, padding: 18, backgroundColor: COLORS.white, shadowColor: '#000', shadowOpacity: 0.04, shadowRadius: 8, elevation: 1 },
  label: { fontSize: 11, fontWeight: '600', color: COLORS.slate400, textTransform: 'uppercase' },
  value: { fontSize: 16, fontWeight: '700', color: COLORS.slate800, marginTop: 4 },
  actionText: { fontWeight: '700', color: COLORS.slate700, textAlign: 'center' },
  dangerCard: { borderRadius: 20, padding: 18, backgroundColor: COLORS.red50, borderWidth: 1, borderColor: COLORS.red200, gap: 10 },
  dangerTitle: { fontWeight: '700', color: COLORS.red700 },
  dangerAction: { fontWeight: '700', color: COLORS.red700, textAlign: 'center', backgroundColor: COLORS.red100, borderRadius: 12, paddingVertical: 10 },
  dangerText: { fontSize: 13, color: COLORS.red700 },
  cancelBtn: { backgroundColor: '#f1f5f9', borderRadius: 12, paddingVertical: 10, alignItems: 'center' },
  cancelText: { fontWeight: '700', color: COLORS.slate700 },
  confirmBtn: { backgroundColor: COLORS.red600, borderRadius: 12, paddingVertical: 10, alignItems: 'center' },
  confirmText: { fontWeight: '700', color: COLORS.white },
  disclaimer: { textAlign: 'center', fontSize: 11, color: COLORS.slate400 },
})
