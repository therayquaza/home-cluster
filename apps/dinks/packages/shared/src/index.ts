export type Period = { id: number; started_on: string; ended_on?: string; flow: string; notes?: string }
export type Symptom = { id: number; recorded_on: string; kind: string; severity: number; notes?: string }
export type Dashboard = { periods: Period[]; symptoms: Symptom[]; next_period?: string; reminder?: string }
export type PeriodInput = { started_on: string; ended_on?: string; flow: string; notes?: string }
export type SymptomInput = { recorded_on: string; kind: string; severity: number; notes?: string }
export type RegisterInput = { email: string; password: string; display_name: string }
export type LoginInput = { email: string; password: string }
export type Me = { display_name: string }
export type Fetcher = <T>(method: string, path: string, body?: unknown) => Promise<T>
export type Partner = { subject: string; display_name: string; linked_at: string }
export type PartnerStatus = { subject: string; display_name: string; on_period: boolean; next_period?: string; reminder?: string }
export type Invite = { code: string; expires_at: string }
export type Prediction = { predicted_period_start?: string; method: string; disclaimer: string }
export type Stats = { period_count: number; checkin_count: number; average_cycle_days: number; symptom_counts: Record<string, number>; cycles_sampled: number }

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

export function api(fetcher: Fetcher) {
  return {
    dashboard: () => fetcher<Dashboard>('GET', '/api/dashboard'),
    prediction: () => fetcher<Prediction>('GET', '/api/prediction'),
    stats: (metric?: keyof Stats) => fetcher<Stats | { metric: string; value: unknown }>('GET', `/api/stats${metric ? `?metric=${metric}` : ''}`),
    statsQuery: (query: StatsQuery) => fetcher<StatsQueryResponse>('POST', '/api/stats/query', query),
    createPeriod: (input: PeriodInput) => fetcher<void>('POST', '/api/periods', input),
    updatePeriod: (id: number, input: PeriodInput) => fetcher<void>('PATCH', `/api/periods/${id}`, input),
    createSymptom: (input: SymptomInput) => fetcher<void>('POST', '/api/symptoms', input),
    updateSymptom: (id: number, input: SymptomInput) => fetcher<void>('PATCH', `/api/symptoms/${id}`, input),
    deleteSymptom: (id: number) => fetcher<void>('DELETE', `/api/symptoms/${id}`),
    exportData: () => fetcher<unknown>('GET', '/api/export'),
    deleteMe: () => fetcher<void>('DELETE', '/api/me'),
    register: (input: RegisterInput) => fetcher<void>('POST', '/auth/register', input),
    login: (input: LoginInput) => fetcher<void>('POST', '/auth/login', input),
    logout: () => fetcher<void>('POST', '/auth/logout'),
    me: () => fetcher<Me>('GET', '/api/me'),
    createInvite: () => fetcher<Invite>('POST', '/api/partners/invite'),
    redeemInvite: (code: string) => fetcher<void>('POST', '/api/partners/link', { code }),
    listPartners: () => fetcher<Partner[]>('GET', '/api/partners'),
    revokePartner: (subject: string) => fetcher<void>('DELETE', `/api/partners/${subject}`),
    partnerStatuses: () => fetcher<PartnerStatus[]>('GET', '/api/partners/status'),
  }
}

export function isISODate(value: string): boolean { return /^\d{4}-\d{2}-\d{2}$/.test(value) && !Number.isNaN(Date.parse(value + 'T00:00:00Z')) }
