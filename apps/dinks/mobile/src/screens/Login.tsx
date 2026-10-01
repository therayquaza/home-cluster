import { useState } from 'react'
import { KeyboardAvoidingView, Platform, Pressable, ScrollView, StyleSheet, Text, TextInput, View } from 'react-native'
import { publicClient } from '../api'
import { signInWithKeycloak } from '../auth'
import { COLORS } from '../lib/theme'

type Props = { onSignedIn: () => void | Promise<void> }

// Mirrors web/src/pages/Login.tsx: email/password first, Keycloak as the
// alternative. Keeping the two in step means a member can move between the web
// and mobile apps without meeting a different sign-in story.
export default function Login({ onSignedIn }: Props) {
  const [mode, setMode] = useState<'login' | 'register'>('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit() {
    setError('')
    setBusy(true)
    try {
      if (mode === 'register') await publicClient.register({ email, password, display_name: displayName })
      else await publicClient.login({ email, password })
      await onSignedIn()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Something went wrong')
    } finally {
      setBusy(false)
    }
  }

  async function keycloak() {
    setError('')
    setBusy(true)
    try {
      if (await signInWithKeycloak()) await onSignedIn()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Keycloak sign in failed')
    } finally {
      setBusy(false)
    }
  }

  const canSubmit = mode === 'register' ? email.length > 0 && password.length >= 8 : email.length > 0 && password.length > 0

  return (
    <KeyboardAvoidingView style={styles.flex} behavior={Platform.OS === 'ios' ? 'padding' : undefined}>
      <ScrollView contentContainerStyle={styles.page} keyboardShouldPersistTaps="handled">
        <Text style={styles.wordmark}>dinks</Text>
        <View style={styles.card}>
          <View style={styles.toggle}>
            <Pressable style={[styles.toggleItem, mode === 'login' && styles.toggleItemActive]} onPress={() => setMode('login')}>
              <Text style={[styles.toggleText, mode === 'login' && styles.toggleTextActive]}>Sign in</Text>
            </Pressable>
            <Pressable style={[styles.toggleItem, mode === 'register' && styles.toggleItemActive]} onPress={() => setMode('register')}>
              <Text style={[styles.toggleText, mode === 'register' && styles.toggleTextActive]}>Register</Text>
            </Pressable>
          </View>

          {mode === 'register' && (
            <TextInput style={styles.input} placeholder="Display name" placeholderTextColor={COLORS.slate400} value={displayName} onChangeText={setDisplayName} autoCapitalize="words" />
          )}
          <TextInput
            style={styles.input}
            placeholder="Email"
            placeholderTextColor={COLORS.slate400}
            value={email}
            onChangeText={setEmail}
            autoCapitalize="none"
            keyboardType="email-address"
            autoComplete="email"
          />
          <TextInput
            style={styles.input}
            placeholder="Password"
            placeholderTextColor={COLORS.slate400}
            value={password}
            onChangeText={setPassword}
            secureTextEntry
            autoCapitalize="none"
            autoComplete={mode === 'register' ? 'new-password' : 'current-password'}
          />
          {error !== '' && <Text style={styles.error}>{error}</Text>}
          <Pressable style={[styles.submit, (busy || !canSubmit) && styles.disabled]} onPress={submit} disabled={busy || !canSubmit}>
            <Text style={styles.submitText}>{mode === 'register' ? 'Create account' : 'Sign in'}</Text>
          </Pressable>

          <View style={styles.divider}>
            <View style={styles.dividerLine} />
            <Text style={styles.dividerText}>or</Text>
            <View style={styles.dividerLine} />
          </View>

          <Pressable style={styles.keycloak} onPress={keycloak} disabled={busy}>
            <Text style={styles.keycloakText}>Continue with Keycloak</Text>
          </Pressable>
        </View>
        <Text style={styles.disclaimer}>Estimates are not medical advice.</Text>
      </ScrollView>
    </KeyboardAvoidingView>
  )
}

const styles = StyleSheet.create({
  flex: { flex: 1, backgroundColor: COLORS.bg },
  page: { flexGrow: 1, justifyContent: 'center', alignItems: 'center', paddingHorizontal: 24, paddingVertical: 32, gap: 24 },
  wordmark: { fontSize: 36, fontWeight: '800', color: COLORS.brand500 },
  card: { width: '100%', maxWidth: 384, borderRadius: 24, backgroundColor: COLORS.white, padding: 24, gap: 12, shadowColor: '#000', shadowOpacity: 0.08, shadowRadius: 16, elevation: 3 },
  toggle: { flexDirection: 'row', borderRadius: 999, backgroundColor: COLORS.brand50, padding: 4, marginBottom: 4 },
  toggleItem: { flex: 1, borderRadius: 999, paddingVertical: 8, alignItems: 'center' },
  toggleItemActive: { backgroundColor: COLORS.brand500 },
  toggleText: { fontSize: 14, fontWeight: '600', color: COLORS.brand700 },
  toggleTextActive: { color: COLORS.white },
  input: { borderWidth: 1, borderColor: COLORS.brand100, borderRadius: 12, paddingHorizontal: 12, paddingVertical: 10, fontSize: 15, color: COLORS.slate800 },
  error: { fontSize: 13, color: COLORS.red600 },
  submit: { borderRadius: 12, backgroundColor: COLORS.brand500, paddingVertical: 12, alignItems: 'center' },
  submitText: { fontWeight: '700', color: COLORS.white },
  disabled: { opacity: 0.5 },
  divider: { flexDirection: 'row', alignItems: 'center', gap: 8, marginVertical: 4 },
  dividerLine: { flex: 1, height: 1, backgroundColor: COLORS.brand100 },
  dividerText: { fontSize: 12, color: COLORS.slate400 },
  keycloak: { borderWidth: 1, borderColor: COLORS.brand100, borderRadius: 12, paddingVertical: 12, alignItems: 'center' },
  keycloakText: { fontWeight: '600', color: COLORS.brand700 },
  disclaimer: { maxWidth: 320, textAlign: 'center', fontSize: 11, color: COLORS.slate400 },
})
