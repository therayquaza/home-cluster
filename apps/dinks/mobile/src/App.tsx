import { useEffect, useState } from 'react'
import { Pressable, StyleSheet, Text, View } from 'react-native'
import { SafeAreaProvider, useSafeAreaInsets } from 'react-native-safe-area-context'
import type { Dashboard } from '@dinks/shared'
import { todayISO } from '@dinks/shared'
import { client, setUnauthorizedHandler } from './api'
import { signOut } from './auth'
import { COLORS } from './lib/theme'
import TopBar from './components/TopBar'
import Login from './screens/Login'
import Today from './screens/Today'
import CalendarScreen from './screens/CalendarScreen'
import Stats from './screens/Stats'
import Settings from './screens/Settings'

const TABS = ['Today', 'Calendar', 'Stats', 'Settings'] as const
type Tab = (typeof TABS)[number]
type AuthState = 'loading' | 'authed' | 'anon'

// Mirrors web/src/AppShell.tsx: an auth gate around the same four tabs, the same
// TopBar on Today/Calendar/Stats, and the same bottom navigation. The one mobile
// difference is that navigation is tab state rather than a URL router.
export default function App() {
  return (
    <SafeAreaProvider>
      <AppShell />
    </SafeAreaProvider>
  )
}

function AppShell() {
  const insets = useSafeAreaInsets()
  const [auth, setAuth] = useState<AuthState>('loading')
  const [displayName, setDisplayName] = useState('')
  const [data, setData] = useState<Dashboard>()
  const [tab, setTab] = useState<Tab>('Today')
  const [selectedDate, setSelectedDate] = useState(todayISO())

  const refresh = async () => {
    setData(await client.dashboard())
  }

  async function load() {
    try {
      const me = await client.me()
      setDisplayName(me.display_name)
      await refresh()
      setAuth('authed')
    } catch {
      setAuth('anon')
    }
  }

  useEffect(() => {
    setUnauthorizedHandler(() => setAuth('anon'))
    void load()
  }, [])

  function select(iso: string) {
    setSelectedDate(iso)
    setTab('Today')
  }

  async function handleSignOut() {
    await signOut()
    await client.logout().catch(() => undefined)
    setDisplayName('')
    setData(undefined)
    setAuth('anon')
  }

  if (auth === 'loading') {
    return (
      <View style={[styles.loading, { paddingTop: insets.top }]}>
        <Text style={styles.loadingText}>Loading…</Text>
      </View>
    )
  }

  if (auth === 'anon') {
    return <Login onSignedIn={load} />
  }

  return (
    <View style={[styles.app, { paddingTop: insets.top, paddingLeft: insets.left, paddingRight: insets.right }]}>
      {tab !== 'Settings' && (
        <TopBar
          data={data}
          displayName={displayName}
          selectedDate={selectedDate}
          onSelect={select}
          onOpenSettings={() => setTab('Settings')}
          onOpenCalendar={() => setTab('Calendar')}
        />
      )}
      <View style={styles.content}>
        {tab === 'Today' && <Today client={client} data={data} refresh={refresh} selectedDate={selectedDate} />}
        {tab === 'Calendar' && <CalendarScreen client={client} data={data} refresh={refresh} selectedDate={selectedDate} onSelect={select} />}
        {tab === 'Stats' && <Stats client={client} data={data} />}
        {tab === 'Settings' && <Settings client={client} displayName={displayName} onSignedOut={handleSignOut} />}
      </View>
      <View style={[styles.tabBar, { paddingBottom: Math.max(insets.bottom, 8) }]}>
        {TABS.map((t) => (
          <Pressable key={t} style={styles.tabItem} onPress={() => setTab(t)} accessibilityRole="tab" accessibilityState={{ selected: tab === t }}>
            <Text style={[styles.tabLabel, tab === t && styles.tabLabelActive]}>{t}</Text>
          </Pressable>
        ))}
      </View>
    </View>
  )
}

const styles = StyleSheet.create({
  loading: { flex: 1, alignItems: 'center', justifyContent: 'center', backgroundColor: COLORS.bg },
  loadingText: { color: COLORS.brand500, fontWeight: '600' },
  app: { flex: 1, backgroundColor: COLORS.bg },
  content: { flex: 1 },
  tabBar: { flexDirection: 'row', borderTopWidth: 1, borderTopColor: COLORS.brand100, backgroundColor: COLORS.white },
  tabItem: { flex: 1, paddingVertical: 12, alignItems: 'center' },
  tabLabel: { fontSize: 12, fontWeight: '500', color: COLORS.slate400 },
  tabLabelActive: { color: COLORS.brand500 },
})
