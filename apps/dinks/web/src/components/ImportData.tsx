import { useState, type ChangeEvent } from 'react'
import type { DataFile, ImportMode, ImportResult } from '@dinks/shared'
import { client } from '../api'
import { useToast } from '../useToast'

type Props = {
  onImported: () => void
  onClose: () => void
}

type Preview = { file: DataFile; periods: number; checkins: number; from?: string; to?: string; problem?: string }

/**
 * Reads a dinks data file off disk and shows what it holds before anything is
 * written. The read happens here so a mistyped or unrelated file is caught with
 * a plain explanation instead of a server error, and so the user can see the
 * size of what they are about to load.
 */
function previewFile(text: string): Preview {
  let parsed: unknown
  try {
    parsed = JSON.parse(text)
  } catch {
    return { file: { periods: [], symptoms: [] }, periods: 0, checkins: 0, problem: 'That file is not valid JSON.' }
  }
  const file = parsed as Partial<DataFile>
  if (typeof file !== 'object' || file === null || !Array.isArray(file.periods) || !Array.isArray(file.symptoms)) {
    return { file: { periods: [], symptoms: [] }, periods: 0, checkins: 0, problem: 'That file has no periods and symptoms arrays, so it is not a dinks data file.' }
  }
  if (file.format !== undefined && file.format !== 'dinks-import') {
    return { file: { periods: [], symptoms: [] }, periods: 0, checkins: 0, problem: `That file declares format "${file.format}", which this version of dinks does not read.` }
  }

  const days = [...file.periods.map((p) => p.started_on), ...file.symptoms.map((s) => s.recorded_on)].filter(Boolean).sort()
  return {
    file: { periods: file.periods, symptoms: file.symptoms, format: file.format, version: file.version, source: file.source, exported_at: file.exported_at },
    periods: file.periods.length,
    checkins: file.symptoms.length,
    from: days[0],
    to: days[days.length - 1],
  }
}

export default function ImportData({ onImported, onClose }: Props) {
  const { push } = useToast()
  const [preview, setPreview] = useState<Preview>()
  const [mode, setMode] = useState<ImportMode>('merge')
  const [busy, setBusy] = useState(false)

  async function onPick(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    if (!file) return
    setPreview(previewFile(await file.text()))
  }

  async function submit() {
    if (!preview || preview.problem) return
    setBusy(true)
    try {
      const result = await client.importData(preview.file, mode)
      push(summarise(result), 'success')
      onImported()
      onClose()
    } catch (err) {
      push(`Import failed: ${err instanceof Error ? err.message : 'Something went wrong'}`, 'error')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-4" onClick={onClose}>
      <div className="flex max-h-[85vh] w-full max-w-lg flex-col overflow-y-auto rounded-3xl bg-white shadow-xl" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between border-b border-slate-100 px-5 py-4">
          <h2 className="text-lg font-bold text-slate-800">Import data</h2>
          <button className="text-sm font-medium text-slate-400" onClick={onClose}>
            Cancel
          </button>
        </div>

        <div className="flex flex-col gap-5 p-5">
          <section>
            <p className="mb-2 text-sm text-slate-600">Choose a dinks export file, or a file converted from another app.</p>
            <input
              type="file"
              accept="application/json,.json"
              onChange={onPick}
              className="w-full rounded-xl border border-slate-200 p-2 text-sm file:mr-3 file:rounded-lg file:border-0 file:bg-brand-50 file:px-3 file:py-1.5 file:font-semibold file:text-brand-700"
            />
          </section>

          {preview?.problem && <p className="rounded-xl bg-red-50 p-3 text-sm text-red-700">{preview.problem}</p>}

          {preview && !preview.problem && (
            <section className="rounded-2xl bg-slate-50 p-4 text-sm">
              <p className="font-semibold text-slate-700">
                {preview.periods} periods and {preview.checkins} check-ins
              </p>
              {preview.from && (
                <p className="mt-1 text-slate-500">
                  Covering {preview.from} to {preview.to}
                </p>
              )}
              {preview.file.source && <p className="mt-1 text-slate-500">Converted from {preview.file.source}</p>}
            </section>
          )}

          <section className="flex flex-col gap-2">
            <p className="text-sm font-semibold text-slate-700">How should it be applied?</p>
            {(
              [
                ['merge', 'Merge with what I have', 'Adds anything you do not already have. Safe to run more than once — records you already have are skipped.'],
                ['replace', 'Replace everything', 'Deletes all your current periods and check-ins, then writes this file. Use this to load a history from another app.'],
              ] as const
            ).map(([value, title, blurb]) => (
              <label
                key={value}
                className={`flex cursor-pointer gap-3 rounded-2xl p-3 transition ${mode === value ? 'bg-brand-50 ring-1 ring-brand-500' : 'bg-slate-50'}`}
              >
                <input
                  type="radio"
                  name="import-mode"
                  className="mt-0.5 h-4 w-4 shrink-0 accent-brand-500"
                  checked={mode === value}
                  onChange={() => setMode(value)}
                />
                <span>
                  <span className="block text-sm font-semibold text-slate-800">{title}</span>
                  <span className="mt-0.5 block text-xs text-slate-500">{blurb}</span>
                </span>
              </label>
            ))}
          </section>

          {mode === 'replace' && (
            <p className="rounded-xl bg-red-50 p-3 text-sm text-red-700">
              This permanently deletes every period and check-in you have now. Export your data first if you want a copy.
            </p>
          )}

          <button
            className="w-full rounded-xl bg-brand-500 py-2.5 font-semibold text-white disabled:opacity-40"
            disabled={busy || !preview || !!preview.problem}
            onClick={submit}
          >
            {busy ? 'Importing…' : 'Import'}
          </button>
        </div>
      </div>
    </div>
  )
}

function summarise(result: ImportResult): string {
  const parts = [`${result.periods_imported} periods`, `${result.symptoms_imported} check-ins`]
  const skipped = result.periods_skipped + result.symptoms_skipped
  if (skipped > 0) parts.push(`${skipped} already present`)
  return `Imported ${parts.join(', ')}`
}
