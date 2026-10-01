import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react'
import type { Dashboard, Preferences } from '@dinks/shared'
import { client } from './api'

type DashboardContextValue = {
  data: Dashboard | undefined
  loading: boolean
  displayName: string
  /** Member settings; drives the default flow and the custom indicator list. */
  preferences: Preferences
  refresh: () => Promise<void>
  refreshPreferences: () => Promise<void>
}

const DashboardContext = createContext<DashboardContextValue | null>(null)

export const EMPTY_PREFERENCES: Preferences = { default_flow: 'medium', reminder_enabled: true, reminder_lead_days: 7, trackers: [] }

export function DashboardProvider({ displayName, children }: { displayName: string; children: ReactNode }) {
  const [data, setData] = useState<Dashboard>()
  const [preferences, setPreferences] = useState<Preferences>(EMPTY_PREFERENCES)
  const [loading, setLoading] = useState(true)

  const refresh = useCallback(async () => {
    const next = await client.dashboard()
    setData(next)
  }, [])

  const refreshPreferences = useCallback(async () => {
    setPreferences(await client.preferences())
  }, [])

  useEffect(() => {
    Promise.all([refresh(), refreshPreferences()])
      .catch(() => undefined)
      .finally(() => setLoading(false))
  }, [refresh, refreshPreferences])

  return (
    <DashboardContext.Provider value={{ data, loading, displayName, preferences, refresh, refreshPreferences }}>
      {children}
    </DashboardContext.Provider>
  )
}

export function useDashboard() {
  const ctx = useContext(DashboardContext)
  if (!ctx) throw new Error('useDashboard must be used within DashboardProvider')
  return ctx
}
