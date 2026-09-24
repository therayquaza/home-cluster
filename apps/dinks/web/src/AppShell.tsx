import { useEffect, useState } from 'react'
import { Navigate, NavLink, Outlet, useLocation, type NavLinkRenderProps } from 'react-router-dom'
import { publicClient } from './api'
import { DashboardProvider } from './useDashboard'
import { SelectedDateProvider } from './useSelectedDate'
import TopBar from './components/TopBar'

type AuthState = 'loading' | 'authed' | 'anon'

const tabs = [
  { to: '/today', label: 'Today' },
  { to: '/calendar', label: 'Calendar' },
  { to: '/stats', label: 'Stats' },
  { to: '/settings', label: 'Settings' },
]

const topBarRoutes = ['/today', '/calendar', '/stats']

export default function AppShell() {
  const [auth, setAuth] = useState<AuthState>('loading')
  const [displayName, setDisplayName] = useState('')
  const location = useLocation()
  const showTopBar = topBarRoutes.some((route) => location.pathname.startsWith(route))

  useEffect(() => {
    publicClient
      .me()
      .then((me) => {
        setDisplayName(me.display_name)
        setAuth('authed')
      })
      .catch(() => setAuth('anon'))
  }, [])

  if (auth === 'loading') {
    return <div className="grid h-screen place-items-center text-brand-500">Loading…</div>
  }
  if (auth === 'anon') {
    return <Navigate to="/login" replace />
  }

  return (
    <DashboardProvider displayName={displayName}>
      <SelectedDateProvider>
        <div className="mx-auto flex min-h-screen max-w-lg flex-col bg-brand-bg pb-20">
          {showTopBar && <TopBar />}
          <Outlet />
          <nav className="fixed inset-x-0 bottom-0 mx-auto flex max-w-lg border-t border-brand-100 bg-white/95 backdrop-blur">
            {tabs.map((tab) => (
              <NavLink
                key={tab.to}
                to={tab.to}
                className={({ isActive }: NavLinkRenderProps) => `flex-1 py-3 text-center text-xs font-medium ${isActive ? 'text-brand-500' : 'text-slate-400'}`}
              >
                {tab.label}
              </NavLink>
            ))}
          </nav>
        </div>
      </SelectedDateProvider>
    </DashboardProvider>
  )
}
