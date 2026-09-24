import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import type { Invite, Partner, PartnerStatus } from '@dinks/shared'
import { client } from '../api'
import { useMutationToast } from '../useToast'

export default function Partners() {
  const navigate = useNavigate()
  const run = useMutationToast()
  const [partners, setPartners] = useState<Partner[]>([])
  const [statuses, setStatuses] = useState<PartnerStatus[]>([])
  const [invite, setInvite] = useState<Invite | undefined>()
  const [code, setCode] = useState('')
  const [redeemError, setRedeemError] = useState('')
  const [busy, setBusy] = useState(false)
  const [loading, setLoading] = useState(true)

  async function loadAll() {
    const [p, s] = await Promise.all([client.listPartners(), client.partnerStatuses()])
    setPartners(p)
    setStatuses(s)
  }

  useEffect(() => {
    loadAll().finally(() => setLoading(false))
  }, [])

  async function generateInvite() {
    setBusy(true)
    try {
      const result = await run(() => client.createInvite(), 'Invite code generated')
      setInvite(result)
    } catch {
      // toast already shown
    } finally {
      setBusy(false)
    }
  }

  async function revoke(subject: string, displayName: string) {
    if (!confirm(`Revoke ${displayName}'s access to your status?`)) return
    setBusy(true)
    try {
      await run(() => client.revokePartner(subject), 'Partner access revoked')
      await loadAll()
    } catch {
      // toast already shown
    } finally {
      setBusy(false)
    }
  }

  async function redeem() {
    setRedeemError('')
    if (!code.trim()) return
    setBusy(true)
    try {
      await run(() => client.redeemInvite(code.trim()), 'Linked successfully')
      setCode('')
      await loadAll()
    } catch (err) {
      setRedeemError(err instanceof Error ? err.message : 'Could not link with that code')
    } finally {
      setBusy(false)
    }
  }

  const expiresAt = invite ? new Date(invite.expires_at) : undefined

  return (
    <main className="flex flex-col gap-4 p-4">
      <header className="flex items-center gap-3">
        <button className="text-sm font-semibold text-brand-700" onClick={() => navigate(-1)}>
          ← Back
        </button>
        <h1 className="text-xl font-bold text-slate-800">Partner mode</h1>
      </header>

      <section className="rounded-3xl bg-white p-5 shadow-sm">
        <h2 className="mb-1 font-semibold text-slate-800">Share my status</h2>
        <p className="mb-3 text-xs text-slate-500">Generate a code and hand it to someone you want to share your cycle status with.</p>
        <button className="w-full rounded-xl bg-brand-500 py-2.5 font-semibold text-white disabled:opacity-50" disabled={busy} onClick={generateInvite}>
          Generate invite code
        </button>
        {invite && (
          <div className="mt-3 rounded-2xl bg-brand-50 p-4 text-center">
            <p className="text-3xl font-extrabold tracking-[0.3em] text-brand-700">{invite.code}</p>
            {expiresAt && <p className="mt-1 text-xs text-brand-700">Expires at {expiresAt.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</p>}
          </div>
        )}

        <h3 className="mb-2 mt-5 text-sm font-semibold text-slate-700">People who can see my status</h3>
        {loading && <p className="text-sm text-slate-400">Loading…</p>}
        {!loading && partners.length === 0 && <p className="text-sm text-slate-400">No one yet.</p>}
        <ul className="flex flex-col gap-2">
          {partners.map((p) => (
            <li key={p.subject} className="flex items-center justify-between rounded-xl bg-slate-50 px-3 py-2 text-sm">
              <div>
                <p className="font-medium text-slate-800">{p.display_name}</p>
                <p className="text-xs text-slate-400">Linked {new Date(p.linked_at).toLocaleDateString()}</p>
              </div>
              <button className="font-semibold text-red-600" disabled={busy} onClick={() => revoke(p.subject, p.display_name)}>
                Revoke
              </button>
            </li>
          ))}
        </ul>
      </section>

      <section className="rounded-3xl bg-white p-5 shadow-sm">
        <h2 className="mb-1 font-semibold text-slate-800">Shared with me</h2>
        <p className="mb-3 text-xs text-slate-500">Enter a code someone shared with you to see their status.</p>
        <div className="flex gap-2">
          <input
            className="flex-1 rounded-xl border border-brand-100 px-3 py-2 text-sm tracking-widest"
            placeholder="6-digit code"
            value={code}
            maxLength={6}
            onChange={(e) => setCode(e.target.value.replace(/\D/g, ''))}
          />
          <button className="rounded-xl bg-brand-500 px-4 py-2 font-semibold text-white disabled:opacity-50" disabled={busy || !code.trim()} onClick={redeem}>
            Link
          </button>
        </div>
        {redeemError && <p className="mt-2 text-sm text-red-500">{redeemError}</p>}

        <div className="mt-4 flex flex-col gap-3">
          {statuses.length === 0 && !loading && <p className="text-sm text-slate-400">No one has shared their status with you yet.</p>}
          {statuses.map((s) => (
            <div key={s.subject} className="rounded-2xl border border-dashed border-slate-200 bg-slate-50 p-4">
              <p className="text-xs uppercase tracking-wide text-slate-400">Read-only</p>
              <p className="mt-1 font-semibold text-slate-800">{s.display_name}</p>
              <p className="mt-1 text-sm text-slate-600">{s.on_period ? 'Currently on their period' : 'Not currently on their period'}</p>
              {s.next_period && <p className="mt-1 text-xs text-slate-500">Next period estimated around {s.next_period}.</p>}
              {s.reminder && <p className="mt-1 text-xs text-slate-500">{s.reminder}</p>}
            </div>
          ))}
        </div>
        {statuses.length > 0 && <p className="mt-3 text-center text-xs text-slate-400">Estimates are not medical advice.</p>}
      </section>
    </main>
  )
}
