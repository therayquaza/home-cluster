import { createContext, useContext, useMemo, useState, type ReactNode } from 'react'
import { todayISO } from './lib/dates'

export { todayISO }

type SelectedDateContextValue = { selectedDate: string; setSelectedDate: (iso: string) => void }

const SelectedDateContext = createContext<SelectedDateContextValue | null>(null)

export function SelectedDateProvider({ children }: { children: ReactNode }) {
  const [selectedDate, setSelectedDate] = useState(todayISO())
  const value = useMemo(() => ({ selectedDate, setSelectedDate }), [selectedDate])
  return <SelectedDateContext.Provider value={value}>{children}</SelectedDateContext.Provider>
}

export function useSelectedDate() {
  const ctx = useContext(SelectedDateContext)
  if (!ctx) throw new Error('useSelectedDate must be used within SelectedDateProvider')
  return ctx
}
