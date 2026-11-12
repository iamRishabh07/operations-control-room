// Typed boundary for the /api/recommendation response. Any field the UI uses
// is defensively validated at parse time; missing or malformed responses are
// rejected rather than rendered as a confident conclusion.

export type Status = 'action' | 'no_action' | 'unavailable'

export interface WeatherEvidence {
  state: 'valid' | 'invalid'
  reason?: string
  precipitation_mm: number | null
  interval_seconds?: number | null
  units?: string
  valid_at?: string | null
}

export interface Candidate {
  rank: number
  area_id: number
  name: string
  lat: number
  lon: number
  total_trips: number
  sample_days: number
  mean_pickups_per_day: number
  weather: WeatherEvidence
}

export interface HistoryEvidence {
  dataset_id: string
  period_start: string
  period_end: string
  retrieved_at: string
  rows_loaded: number
  trips_included: number
  trips_excluded_invalid_area: number
  synthetic?: boolean
  synthetic_notice?: string
}

export interface Assessment {
  status: Status
  reason: string
  assessed_at: string
  valid_until: string | null
  area: { area_id: number; name: string } | null
  candidates: Candidate[]
  history: HistoryEvidence | null
  weather: { source: string; fetched_at: string | null }
  limitations: string[]
}

export class AssessmentError extends Error {}

export async function fetchAssessment(signal?: AbortSignal): Promise<Assessment> {
  const res = await fetch('/api/recommendation', { signal, headers: { accept: 'application/json' } })
  if (!res.ok) {
    throw new AssessmentError(`server error ${res.status}`)
  }
  const body = (await res.json()) as unknown
  return parseAssessment(body)
}

function parseAssessment(raw: unknown): Assessment {
  if (!raw || typeof raw !== 'object') throw new AssessmentError('malformed response')
  const r = raw as Record<string, unknown>
  const status = r.status
  if (status !== 'action' && status !== 'no_action' && status !== 'unavailable') {
    throw new AssessmentError('unknown status')
  }
  if (typeof r.assessed_at !== 'string' || typeof r.reason !== 'string') {
    throw new AssessmentError('missing required fields')
  }
  const valid_until = r.valid_until === null || typeof r.valid_until === 'string' ? r.valid_until : null
  if (status === 'unavailable' && valid_until !== null) {
    throw new AssessmentError('unavailable must have null valid_until')
  }
  if (status !== 'unavailable' && typeof valid_until !== 'string') {
    throw new AssessmentError('non-unavailable must have valid_until')
  }
  const candidates = Array.isArray(r.candidates) ? (r.candidates as Candidate[]) : []
  if (candidates.length > 5) throw new AssessmentError('too many candidates')
  const area = r.area && typeof r.area === 'object' ? (r.area as { area_id: number; name: string }) : null
  if (status === 'action' && !area) throw new AssessmentError('action without area')
  return {
    status,
    reason: r.reason,
    assessed_at: r.assessed_at,
    valid_until,
    area,
    candidates,
    history: (r.history as HistoryEvidence | null) ?? null,
    weather: (r.weather as Assessment['weather']) ?? { source: '', fetched_at: null },
    limitations: Array.isArray(r.limitations) ? (r.limitations as string[]) : [],
  }
}
