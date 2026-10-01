export type FlowDay = { date: string; flow: string }
export type Period = { id: number; started_on: string; ended_on?: string; flow: string; days?: FlowDay[]; notes?: string }
export type Symptom = { id: number; recorded_on: string; kind: string; severity: number; notes?: string }
export type Dashboard = { periods: Period[]; symptoms: Symptom[]; next_period?: string; reminder?: string }
export type PeriodInput = { started_on: string; ended_on?: string; flow: string; days?: FlowDay[]; notes?: string }
export type SymptomInput = { recorded_on: string; kind: string; severity: number; notes?: string }
export type RegisterInput = { email: string; password: string; display_name: string }
export type LoginInput = { email: string; password: string }
export type Me = { display_name: string }
export type Tracker = { key: string; label: string; emoji: string; unit?: string }
export type Preferences = { default_flow: string; reminder_enabled: boolean; reminder_lead_days: number; trackers: Tracker[] }
/** Partial update — only the fields present are changed. */
export type PreferencesInput = { default_flow?: string; reminder_enabled?: boolean; reminder_lead_days?: number; trackers?: Tracker[] }
export type Fetcher = <T>(method: string, path: string, body?: unknown) => Promise<T>
export type ShareField =
  | 'on_period'
  | 'next_period'
  | 'reminders'
  | 'flow'
  | 'symptoms'
  | 'libido'
  | 'sex'
  | 'notes'
export type Partner = { subject: string; display_name: string; linked_at: string; share: ShareField[] }
export type PartnerStatus = {
  subject: string
  display_name: string
  /** Absent — not merely false — when the owner has not shared it. */
  on_period?: boolean
  next_period?: string
  reminder?: string
}
/** One day of a shared cycle; unshared fields are absent, not zero-valued. */
export type PartnerDay = {
  date: string
  on_period: boolean
  flow?: string
  symptoms?: string[]
  sex?: string[]
  libido?: string[]
  notes?: string
  period_started?: boolean
  period_ended?: boolean
}
export type PartnerView = PartnerStatus & { shared: ShareField[]; days: PartnerDay[] }
export type Invite = { code: string; expires_at: string }
export type Prediction = { predicted_period_start?: string; method: string; disclaimer: string }
export type Stats = { period_count: number; checkin_count: number; average_cycle_days: number; symptom_counts: Record<string, number>; cycles_sampled: number }

/**
 * The file GET /api/export produces and POST /api/import accepts. The record
 * arrays are the input shapes rather than the stored ones — ids are assigned by
 * the server, so a file may carry them (an export does) but they are ignored.
 * `format`/`version`/`source` are optional: an export omits them, tooling that
 * produces import files writes them so a future revision can be recognised.
 */
export type DataFile = {
  format?: string
  version?: number
  source?: string
  exported_at?: string
  periods: PeriodInput[]
  symptoms: SymptomInput[]
}
export type ImportMode = 'merge' | 'replace'
export type ImportResult = {
  mode: ImportMode
  periods_imported: number
  periods_skipped: number
  symptoms_imported: number
  symptoms_skipped: number
}

export type StatsMetric =
  | 'period_count'
  | 'checkin_count'
  | 'cycles_sampled'
  | 'average_cycle_days'
  | 'cycle_length_stddev'
  | 'average_period_length_days'
  | 'symptom_counts'
  | 'average_symptom_severity'
export type StatsGroupBy = '' | 'month' | 'flow' | 'symptom_kind'
export type StatsQuery = { metrics: StatsMetric[]; group_by?: StatsGroupBy; from?: string; to?: string }
export type StatsResult = { group: string; values: Record<string, unknown> }
export type StatsQueryResponse = { results: StatsResult[] }

export * from './phase.ts'
export * from './symptoms.ts'
export * from './dates.ts'
export * from './periods.ts'
export * from './cycleStats.ts'
export * from './emoji.ts'
export * from './predictions.ts'

export function api(fetcher: Fetcher) {
  return {
    dashboard: () => fetcher<Dashboard>('GET', '/api/dashboard'),
    prediction: () => fetcher<Prediction>('GET', '/api/prediction'),
    stats: (metric?: keyof Stats) => fetcher<Stats | { metric: string; value: unknown }>('GET', `/api/stats${metric ? `?metric=${metric}` : ''}`),
    statsQuery: (query: StatsQuery) => fetcher<StatsQueryResponse>('POST', '/api/stats/query', query),
    createPeriod: (input: PeriodInput) => fetcher<void>('POST', '/api/periods', input),
    updatePeriod: (id: number, input: PeriodInput) => fetcher<void>('PATCH', `/api/periods/${id}`, input),
    deletePeriod: (id: number) => fetcher<void>('DELETE', `/api/periods/${id}`),
    createSymptom: (input: SymptomInput) => fetcher<void>('POST', '/api/symptoms', input),
    updateSymptom: (id: number, input: SymptomInput) => fetcher<void>('PATCH', `/api/symptoms/${id}`, input),
    deleteSymptom: (id: number) => fetcher<void>('DELETE', `/api/symptoms/${id}`),
    exportData: () => fetcher<DataFile>('GET', '/api/export'),
    importData: (file: DataFile, mode?: ImportMode) =>
      fetcher<ImportResult>('POST', `/api/import${mode ? `?mode=${mode}` : ''}`, file),
    deleteMe: () => fetcher<void>('DELETE', '/api/me'),
    register: (input: RegisterInput) => fetcher<void>('POST', '/auth/register', input),
    login: (input: LoginInput) => fetcher<void>('POST', '/auth/login', input),
    logout: () => fetcher<void>('POST', '/auth/logout'),
    me: () => fetcher<Me>('GET', '/api/me'),
    preferences: () => fetcher<Preferences>('GET', '/api/preferences'),
    updatePreferences: (input: PreferencesInput) => fetcher<Preferences>('PATCH', '/api/preferences', input),
    createInvite: () => fetcher<Invite>('POST', '/api/partners/invite'),
    redeemInvite: (code: string) => fetcher<void>('POST', '/api/partners/link', { code }),
    listPartners: () => fetcher<Partner[]>('GET', '/api/partners'),
    revokePartner: (subject: string) => fetcher<void>('DELETE', `/api/partners/${subject}`),
    partnerStatuses: () => fetcher<PartnerStatus[]>('GET', '/api/partners/status'),
    updateShare: (subject: string, share: ShareField[]) =>
      fetcher<Partner>('PATCH', `/api/partners/${subject}/share`, { share }),
    partnerView: (subject: string) => fetcher<PartnerView>('GET', `/api/partners/${subject}/view`),
  }
}

export function isISODate(value: string): boolean { return /^\d{4}-\d{2}-\d{2}$/.test(value) && !Number.isNaN(Date.parse(value + 'T00:00:00Z')) }
