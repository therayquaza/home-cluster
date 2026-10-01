import { useEffect, useState } from 'react'
import type { Tracker } from '@dinks/shared'
import { client } from '../../api'
import { useDashboard } from '../../useDashboard'
import { useMutationToast } from '../../useToast'
import FlowPicker from '../../components/FlowPicker'
import { FLOW_LEVELS, flowLabel } from '../../lib/emoji'

/**
 * Per-member settings: what flow a new period starts on, whether the
 * approaching-period reminder appears (and how early), and the custom
 * indicators the user records a number for.
 */
export default function Preferences() {
  const run = useMutationToast()
  // Today and Calendar read the default flow and the tracker list from the
  // dashboard context, so a change here has to reach it or those pages keep
  // using what was loaded at start-up.
  const { refreshPreferences } = useDashboard()
  const [defaultFlow, setDefaultFlow] = useState('medium')
  const [reminderEnabled, setReminderEnabled] = useState(true)
  const [reminderLeadDays, setReminderLeadDays] = useState(7)
  const [trackers, setTrackers] = useState<Tracker[]>([])
  const [loaded, setLoaded] = useState(false)
  const [newLabel, setNewLabel] = useState('')
  const [newUnit, setNewUnit] = useState('')

  useEffect(() => {
    client
      .preferences()
      .then((p) => {
        setDefaultFlow(p.default_flow || 'medium')
        setReminderEnabled(p.reminder_enabled)
        setReminderLeadDays(p.reminder_lead_days)
        setTrackers(p.trackers)
      })
      .catch(() => undefined)
      .finally(() => setLoaded(true))
  }, [])

  async function save(input: Parameters<typeof client.updatePreferences>[0], message: string) {
    try {
      const next = await run(() => client.updatePreferences(input), message)
      setDefaultFlow(next.default_flow || 'medium')
      setReminderEnabled(next.reminder_enabled)
      setReminderLeadDays(next.reminder_lead_days)
      setTrackers(next.trackers)
      await refreshPreferences()
    } catch {
      // toast already shown
    }
  }

  function addTracker() {
    const label = newLabel.trim()
    if (!label) return
    const next = [...trackers, { key: '', label, emoji: '📈', unit: newUnit.trim() }]
    setTrackers(next)
    setNewLabel('')
    setNewUnit('')
    // The server derives the key from the label and echoes the stored list back.
    void save({ trackers: next }, `${label} added`)
  }

  function removeTracker(key: string) {
    const next = trackers.filter((t) => t.key !== key)
    setTrackers(next)
    void save({ trackers: next }, 'Tracker removed')
  }

  if (!loaded) {
    return (
      <main className="p-4">
        <p className="text-sm text-slate-400">Loading…</p>
      </main>
    )
  }

  return (
    <main className="flex flex-col gap-4 p-4">
      <div>
        <h1 className="text-xl font-bold text-slate-800">Preferences</h1>
        <p className="text-sm text-slate-400">These settings apply to your account only.</p>
      </div>

      <section className="rounded-3xl bg-white p-5 shadow-sm">
        <h2 className="mb-1 font-semibold text-slate-800">Default flow</h2>
        <p className="mb-3 text-xs text-slate-400">Preselected when you log a new period.</p>
        <FlowPicker
          value={defaultFlow}
          onChange={(next) => {
            setDefaultFlow(next)
            void save({ default_flow: next }, `Default flow set to ${flowLabel(next)}`)
          }}
        />
      </section>

      <section className="rounded-3xl bg-white p-5 shadow-sm">
        <label className="flex items-center justify-between">
          <span className="font-semibold text-slate-800">Period reminder</span>
          <button
            type="button"
            role="switch"
            aria-checked={reminderEnabled}
            onClick={() => {
              setReminderEnabled(!reminderEnabled)
              void save({ reminder_enabled: !reminderEnabled }, !reminderEnabled ? 'Reminder on' : 'Reminder off')
            }}
            className={`relative h-7 w-12 shrink-0 rounded-full transition ${reminderEnabled ? 'bg-brand-500' : 'bg-slate-200'}`}
          >
            <span className={`absolute top-0.5 h-6 w-6 rounded-full bg-white shadow transition ${reminderEnabled ? 'left-[22px]' : 'left-0.5'}`} />
          </button>
        </label>
        {reminderEnabled && (
          <div className="mt-4">
            <label className="mb-1 block text-xs font-medium text-slate-600">Show it how many days ahead?</label>
            <input
              type="number"
              min={0}
              max={30}
              value={reminderLeadDays}
              onChange={(e) => setReminderLeadDays(Number(e.target.value))}
              onBlur={() => void save({ reminder_lead_days: reminderLeadDays }, 'Reminder updated')}
              className="w-24 rounded-xl border border-slate-200 px-3 py-2 text-sm"
            />
            <p className="mt-1 text-xs text-slate-400">Estimates only — not medical advice.</p>
          </div>
        )}
      </section>

      <section className="rounded-3xl bg-white p-5 shadow-sm">
        <h2 className="font-semibold text-slate-800">Custom indicators</h2>
        <p className="mb-3 text-xs text-slate-400">
          Track anything a number helps with — weight, temperature, sleep. Each one adds a field to your daily check-in.
        </p>

        {trackers.length > 0 && (
          <ul className="mb-3 flex flex-col gap-2">
            {trackers.map((t) => (
              <li key={t.key} className="flex items-center justify-between rounded-2xl bg-slate-50 px-3 py-2">
                <span className="text-sm font-medium text-slate-700">
                  {t.emoji} {t.label}
                  {t.unit && <span className="text-slate-400"> ({t.unit})</span>}
                </span>
                <button
                  className="px-2 text-xs font-semibold text-red-500"
                  onClick={() => removeTracker(t.key)}
                  aria-label={`Remove ${t.label}`}
                >
                  Remove
                </button>
              </li>
            ))}
          </ul>
        )}

        <div className="flex gap-2">
          <input
            value={newLabel}
            onChange={(e) => setNewLabel(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && addTracker()}
            placeholder="e.g. Weight"
            aria-label="Indicator name"
            className="flex-1 rounded-xl border border-slate-200 px-3 py-2 text-sm"
          />
          <input
            value={newUnit}
            onChange={(e) => setNewUnit(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && addTracker()}
            placeholder="kg"
            aria-label="Unit"
            className="w-20 rounded-xl border border-slate-200 px-3 py-2 text-sm"
          />
          <button
            className="rounded-xl bg-brand-500 px-4 py-2 font-semibold text-white disabled:opacity-50"
            disabled={!newLabel.trim()}
            onClick={addTracker}
          >
            Add
          </button>
        </div>
        {trackers.length >= 12 && <p className="mt-2 text-xs text-slate-400">Up to 12 indicators.</p>}
      </section>

      <p className="px-1 text-center text-xs text-slate-400">
        {FLOW_LEVELS.length} flow levels available. Settings are private to your account.
      </p>
    </main>
  )
}
