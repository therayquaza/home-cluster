import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react'
import type { Dashboard } from '@dinks/shared'
import { client } from './api'

type DashboardContextValue = {
  data: Dashboard | undefined
  loading: boolean
  displayName: string
  refresh: () => Promise<void>
}

const DashboardContext = createContext<DashboardContextValue | null>(null)

export function DashboardProvider({ displayName, children }: { displayName: string; children: ReactNode }) {
  const [data, setData] = useState<Dashboard>()
  const [loading, setLoading] = useState(true)

  const refresh = useCallback(async () => {
    const next = await client.dashboard()
    setData(next)
  }, [])

  useEffect(() => {
    refresh().finally(() => setLoading(false))
  }, [refresh])

  return <DashboardContext.Provider value={{ data, loading, displayName, refresh }}>{children}</DashboardContext.Provider>
}

export function useDashboard() {
  const ctx = useContext(DashboardContext)
  if (!ctx) throw new Error('useDashboard must be used within DashboardProvider')
  return ctx
}
