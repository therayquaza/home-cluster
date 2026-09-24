import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { client } from '../api'
import { useDashboard } from '../useDashboard'
import { useMutationToast } from '../useToast'

export default function Settings() {
  const { displayName } = useDashboard()
  const navigate = useNavigate()
  const run = useMutationToast()
  const [confirmingDelete, setConfirmingDelete] = useState(false)
  const [busy, setBusy] = useState(false)

  async function signOut() {
    try {
      await run(() => client.logout(), 'Signed out')
    } catch {
      // toast already shown; still navigate to login below
    }
    navigate('/login', { replace: true })
  }

  async function exportData() {
    try {
      const payload = await run(() => client.exportData(), 'Export ready — downloading')
      const blob = new Blob([JSON.stringify(payload, null, 2)], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'dinks-export.json'
      a.click()
      URL.revokeObjectURL(url)
    } catch {
      // toast already shown
    }
  }

  async function deleteAccount() {
    setBusy(true)
    try {
      await run(() => client.deleteMe(), 'Account deleted')
      navigate('/login', { replace: true })
    } catch {
      // toast already shown
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="flex flex-col gap-4 p-4">
      <h1 className="text-xl font-bold text-slate-800">Settings</h1>

      <section className="rounded-3xl bg-white p-5 shadow-sm">
        <p className="text-xs uppercase tracking-wide text-slate-400">Signed in as</p>
        <p className="mt-1 text-lg font-semibold text-slate-800">{displayName}</p>
      </section>

      <section className="flex flex-col gap-2 rounded-3xl bg-white p-2 shadow-sm">
        <button className="flex items-center justify-between rounded-2xl px-3 py-3 text-left font-semibold text-slate-700" onClick={() => navigate('/settings/partners')}>
          Partner mode
          <span className="text-slate-300">›</span>
        </button>
        <button className="flex items-center justify-between rounded-2xl px-3 py-3 text-left font-semibold text-slate-700" onClick={() => navigate('/settings/insights')}>
          Cycle insights
          <span className="text-slate-300">›</span>
        </button>
      </section>

      <section className="rounded-3xl bg-white p-5 shadow-sm">
        <button className="w-full rounded-xl bg-brand-50 py-2.5 font-semibold text-brand-700" onClick={exportData}>
          Export my data
        </button>
        <button className="mt-3 w-full rounded-xl bg-slate-100 py-2.5 font-semibold text-slate-700" onClick={signOut}>
          Sign out
        </button>
      </section>

      <section className="rounded-3xl border border-red-200 bg-red-50 p-5">
        <h2 className="mb-2 font-semibold text-red-700">Danger zone</h2>
        {!confirmingDelete ? (
          <button className="w-full rounded-xl bg-red-100 py-2.5 font-semibold text-red-700" onClick={() => setConfirmingDelete(true)}>
            Delete my account
          </button>
        ) : (
          <div className="flex flex-col gap-2">
            <p className="text-sm text-red-700">This permanently deletes your account and all data. This cannot be undone.</p>
            <div className="flex gap-2">
              <button className="flex-1 rounded-xl bg-slate-100 py-2.5 font-semibold text-slate-700" onClick={() => setConfirmingDelete(false)}>
                Cancel
              </button>
              <button className="flex-1 rounded-xl bg-red-600 py-2.5 font-semibold text-white disabled:opacity-50" disabled={busy} onClick={deleteAccount}>
                Confirm delete
              </button>
            </div>
          </div>
        )}
      </section>

      <p className="px-1 text-center text-xs text-slate-400">Estimates are not medical advice.</p>
    </main>
  )
}
