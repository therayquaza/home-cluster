import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import AppShell from './AppShell'
import Login from './pages/Login'
import Today from './pages/Today'
import CalendarPage from './pages/Calendar'
import Stats from './pages/Stats'
import Settings from './pages/Settings'
import Partners from './pages/Partners'
import Preferences from './pages/settings/Preferences'
import PartnerView from './pages/partner/PartnerView'
import SharedWithMe from './pages/partner/SharedWithMe'
import { ToastProvider, ToastHost } from './useToast'
import 'react-day-picker/style.css'
import './index.css'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ToastProvider>
      <ToastHost />
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route path="/" element={<AppShell />}>
            <Route index element={<Navigate to="/today" replace />} />
            <Route path="today" element={<Today />} />
            <Route path="calendar" element={<CalendarPage />} />
            <Route path="stats" element={<Stats />} />
            <Route path="settings" element={<Settings />} />
            <Route path="settings/partners" element={<Partners />} />
            <Route path="settings/preferences" element={<Preferences />} />
            <Route path="partner" element={<SharedWithMe />} />
            <Route path="partner/:subject" element={<PartnerView />} />
          </Route>
          <Route path="*" element={<Navigate to="/today" replace />} />
        </Routes>
      </BrowserRouter>
    </ToastProvider>
  </StrictMode>,
)
