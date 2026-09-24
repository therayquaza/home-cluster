import { useEffect, useState } from 'react'
import { Button, SafeAreaView, StyleSheet, Text, View, Pressable } from 'react-native'
import type { Dashboard, Me } from '@dinks/shared'
import { mobileAPI } from './api'
import { signIn, signOut, token } from './auth'
import { todayISO } from './lib/dates'
import { COLORS } from './lib/theme'
import DayStrip from './components/DayStrip'
import Today from './screens/Today'
import CalendarScreen from './screens/CalendarScreen'
import Stats from './screens/Stats'
import Settings from './screens/Settings'

const client = mobileAPI(token)
const TABS = ['Today', 'Calendar', 'Stats', 'Settings'] as const
type Tab = (typeof TABS)[number]

export default function App() {
  const [signedIn, setSignedIn] = useState(false)
  const [data, setData] = useState<Dashboard>()
  const [me, setMe] = useState<Me>()
  const [tab, setTab] = useState<Tab>('Today')
  const [selectedDate, setSelectedDate] = useState(todayISO())

  const refresh = () =>
    client
      .dashboard()
      .then((d) => {
        setData(d)
        setSignedIn(true)
      })
      .catch(() => setSignedIn(false))

  useEffect(() => {
    void refresh()
    client.me().then(setMe).catch(() => undefined)
  }, [])

  function select(iso: string) {
    setSelectedDate(iso)
    setTab('Today')
  }

  if (!signedIn) {
    return (
      <SafeAreaView style={styles.landing}>
        <Text style={styles.landingTitle}>Dinks</Text>
        <Text>Your private cycle records.</Text>
        <View style={{ marginTop: 16 }}>
          <Button
            title="Sign in"
            onPress={() =>
              signIn()
                .then(() => refresh().then(() => client.me().then(setMe).catch(() => undefined)))
                .catch((e) => alert(`Sign in failed: ${e.message}`))
            }
          />
        </View>
      </SafeAreaView>
    )
  }

  return (
    <SafeAreaView style={styles.app}>
      {(tab === 'Today' || tab === 'Calendar' || tab === 'Stats') && <DayStrip data={data} selectedDate={selectedDate} onSelect={setSelectedDate} />}
      <View style={{ flex: 1 }}>
        {tab === 'Today' && <Today client={client} data={data} refresh={refresh} selectedDate={selectedDate} />}
        {tab === 'Calendar' && <CalendarScreen client={client} data={data} refresh={refresh} selectedDate={selectedDate} onSelect={select} />}
        {tab === 'Stats' && <Stats client={client} data={data} />}
        {tab === 'Settings' && (
          <Settings
            client={client}
            displayName={me?.display_name ?? ''}
            onSignedOut={() => signOut().then(() => setSignedIn(false))}
          />
        )}
      </View>
      <View style={styles.tabBar}>
        {TABS.map((t) => (
          <Pressable key={t} style={styles.tabItem} onPress={() => setTab(t)}>
            <Text style={[styles.tabLabel, tab === t && styles.tabLabelActive]}>{t}</Text>
          </Pressable>
        ))}
      </View>
    </SafeAreaView>
  )
}

const styles = StyleSheet.create({
  landing: { flex: 1, padding: 20, justifyContent: 'center', backgroundColor: COLORS.bg },
  landingTitle: { fontSize: 30, fontWeight: '700', marginBottom: 8, color: COLORS.slate800 },
  app: { flex: 1, backgroundColor: COLORS.bg },
  tabBar: { flexDirection: 'row', borderTopWidth: 1, borderTopColor: COLORS.brand100, backgroundColor: COLORS.white },
  tabItem: { flex: 1, paddingVertical: 12, alignItems: 'center' },
  tabLabel: { fontSize: 12, fontWeight: '600', color: COLORS.slate400 },
  tabLabelActive: { color: COLORS.brand500 },
})
