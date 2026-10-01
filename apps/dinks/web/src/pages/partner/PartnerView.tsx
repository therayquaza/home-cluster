import { useEffect, useState, type ReactNode } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import type { PartnerView as PartnerViewData } from '@dinks/shared'
import { client } from '../../api'
import { flowLabel, shortKind } from '../../lib/emoji'

/**
 * The dedicated partner screen: read-only, and it only ever shows what the
 * owner chose to share. A field absent from the response is not rendered — the
 * UI never fills in a default for data it was not given.
 */
export default function PartnerView() {
  const { subject = '' } = useParams()
  const navigate = useNavigate()
  const [view, setView] = useState<PartnerViewData>()
  const [error, setError] = useState<string>()

  useEffect(() => {
    let live = true
    setView(undefined)
    setError(undefined)
    client
      .partnerView(subject)
      .then((v) => live && setView(v))
      .catch((e) => live && setError(e instanceof Error ? e.message : 'Unable to load'))
    return () => {
      live = false
    }
  }, [subject])

  if (error) {
    return (
      <Shell onBack={() => navigate(-1)}>
        <p className="rounded-2xl bg-white p-5 text-center text-sm text-slate-500 shadow-sm">
          Nothing is shared with you here.
        </p>
      </Shell>
    )
  }
  if (!view) {
    return (
      <Shell onBack={() => navigate(-1)}>
        <p className="p-5 text-center text-sm text-slate-400">Loading…</p>
      </Shell>
    )
  }

  const canSee = (f: string) => view.shared.includes(f as never)
  // Days are returned oldest-first; the most recent matter most on a phone.
  const days = [...view.days].reverse()

  return (
    <Shell onBack={() => navigate(-1)}>
      <header className="mb-4">
        <h1 className="text-xl font-bold text-slate-800">{view.display_name}</h1>
        <p className="text-sm text-slate-400">Shared with you</p>
      </header>

      {canSee('on_period') && (
        <section className={`mb-4 rounded-3xl p-5 shadow-sm ${view.on_period ? 'bg-brand-500' : 'bg-white'}`}>
          <p className={`text-sm ${view.on_period ? 'text-brand-50' : 'text-slate-400'}`}>Today</p>
          <p className={`text-lg font-bold ${view.on_period ? 'text-white' : 'text-slate-700'}`}>
            {view.on_period ? 'On their period' : 'Not on their period'}
          </p>
          {canSee('reminders') && view.reminder && (
            <p className="mt-1 text-xs text-brand-100">{view.reminder}</p>
          )}
          {canSee('next_period') && view.next_period && (
            <p className={`mt-1 text-xs ${view.on_period ? 'text-brand-100' : 'text-slate-400'}`}>
              Estimated next start: {view.next_period}
            </p>
          )}
        </section>
      )}

      <p className="mb-2 text-xs uppercase tracking-wide text-slate-400">Recent days</p>
      {days.length === 0 ? (
        <p className="rounded-2xl bg-white p-5 text-center text-sm text-slate-400 shadow-sm">Nothing recent to show.</p>
      ) : (
        <ul className="flex flex-col gap-2">
          {days.map((d) => (
            <li key={d.date} className="rounded-2xl bg-white p-4 shadow-sm">
              <div className="flex items-baseline justify-between">
                <span className="font-semibold text-slate-700">{d.date}</span>
                <span className="text-xs text-slate-400">
                  {d.period_started ? 'Period started' : d.period_ended ? 'Period ended' : ''}
                </span>
              </div>
              <div className="mt-2 flex flex-wrap gap-2 text-sm">
                {d.on_period && <span className="rounded-full bg-brand-100 px-2 py-0.5 text-brand-700">Period day</span>}
                {canSee('flow') && d.flow && (
                  <span className="rounded-full bg-brand-50 px-2 py-0.5 text-brand-700">{flowLabel(d.flow)}</span>
                )}
                {canSee('symptoms') && d.symptoms?.map((s) => (
                  <span key={s} className="rounded-full bg-slate-100 px-2 py-0.5 text-slate-600">{shortKind(s)}</span>
                ))}
                {canSee('sex') && d.sex?.map((s) => (
                  <span key={s} className="rounded-full bg-amber-100 px-2 py-0.5 text-amber-800">{shortKind(s)}</span>
                ))}
                {canSee('libido') && d.libido?.map((s) => (
                  <span key={s} className="rounded-full bg-amber-100 px-2 py-0.5 text-amber-800">{shortKind(s)}</span>
                ))}
              </div>
              {canSee('notes') && d.notes && <p className="mt-2 text-sm text-slate-500">{d.notes}</p>}
            </li>
          ))}
        </ul>
      )}

      <p className="mt-4 text-center text-xs text-slate-400">
        You see only what {view.display_name} chose to share. Read-only — estimates are not medical advice.
      </p>
    </Shell>
  )
}

function Shell({ children, onBack }: { children: ReactNode; onBack: () => void }) {
  return (
    <main className="mx-auto max-w-lg p-4">
      <button className="mb-3 px-1 text-sm font-medium text-slate-400" onClick={onBack}>
        ‹ Back
      </button>
      {children}
    </main>
  )
}
