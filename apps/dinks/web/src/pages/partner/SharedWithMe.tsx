import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import type { PartnerStatus } from '@dinks/shared'
import { client } from '../../api'

/**
 * Who shares with you. Each entry links to the read-only view for that person;
 * nothing here is editable, because this side of the link belongs to them.
 */
export default function SharedWithMe() {
  const [people, setPeople] = useState<PartnerStatus[]>()
  const [error, setError] = useState<string>()

  useEffect(() => {
    let live = true
    client
      .partnerStatuses()
      .then((p) => live && setPeople(p))
      .catch(() => live && setError('Unable to load'))
    return () => {
      live = false
    }
  }, [])

  if (error) {
    return <p className="p-6 text-center text-sm text-slate-400">Unable to load.</p>
  }
  if (!people) {
    return <p className="p-6 text-center text-sm text-slate-400">Loading…</p>
  }

  return (
    <main className="flex flex-col gap-4 p-4">
      <header>
        <h1 className="text-xl font-bold text-slate-800">Shared with you</h1>
        <p className="text-sm text-slate-400">Read-only. Each person decides what you can see.</p>
      </header>

      {people.length === 0 ? (
        <p className="rounded-3xl bg-white p-6 text-center text-sm text-slate-400 shadow-sm">
          Nobody shares with you yet. Share your invite code from your settings.
        </p>
      ) : (
        <ul className="flex flex-col gap-2">
          {people.map((p) => (
            <li key={p.subject}>
              <Link
                to={`/partner/${p.subject}`}
                className={`flex items-center justify-between rounded-3xl p-5 shadow-sm ${
                  p.on_period ? 'bg-brand-500' : 'bg-white'
                }`}
              >
                <span>
                  <span className={`block font-semibold ${p.on_period ? 'text-white' : 'text-slate-700'}`}>
                    {p.display_name}
                  </span>
                  <span className={`block text-xs ${p.on_period ? 'text-brand-100' : 'text-slate-400'}`}>
                    {p.on_period === undefined
                      ? 'Status not shared'
                      : p.on_period
                        ? 'On their period'
                        : 'Not on their period'}
                  </span>
                </span>
                <span className={`text-slate-300 ${p.on_period ? 'text-brand-200' : ''}`}>›</span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </main>
  )
}
