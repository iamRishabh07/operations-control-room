// Typed boundary for GET /api/recommendation. Every field the UI formats is
// checked here, and so are the rules that tie fields to a status. A malformed
// or self-contradicting response becomes an error state instead of a render
// crash or a misleading page.

export interface WeatherEvidence {
  state: 'valid' | 'invalid' | 'missing'
  reason?: string
  precipitation_mm: number | null // a number exactly when state is valid
  valid_at?: string | null
}

export interface Candidate {
  rank: number
  area_id: number
  name: string
  sample_days: number
  mean_trips: number
  min_trips: number
  max_trips: number
  city_share: number
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
}

interface Common {
  reason: string
  slot_start: string // start of the Chicago clock hour being evaluated
  assessed_at: string // when the server made the assessment; valid_until counts from it
  trigger_mm: number
  candidates: Candidate[]
  history: HistoryEvidence
  weather: { source: string; simulated: boolean; fetched_at: string | null }
  limitations: string[]
}

// What each status guarantees. parseAssessment enforces every line, so the
// components can rely on it.
export type Assessment = Common &
  (
    | { status: 'action'; winner: Candidate; valid_until: string } // winner: the named area, with a valid reading
    | { status: 'no_action'; valid_until: string } // at least one candidate, every reading valid
    | { status: 'unavailable'; valid_until: null; reason_code: string } // reason_code says why
  )

export type Status = Assessment['status']

// When an assessment stops being current, on this machine's clock. The
// server's two timestamps give the length of validity; counting it from the
// moment the request started keeps a wrong local clock out of the result and
// errs on the early side. Unavailable has no validity to count.
export function deadlineFor(a: Assessment, requestedAt: number): number | null {
  if (a.status === 'unavailable') return null
  return requestedAt + (Date.parse(a.valid_until) - Date.parse(a.assessed_at))
}

export class AssessmentError extends Error {}

export async function fetchAssessment(signal?: AbortSignal): Promise<Assessment> {
  const res = await fetch('/api/recommendation', { signal, headers: { accept: 'application/json' } })
  if (!res.ok) {
    throw new AssessmentError(`the server answered ${res.status}`)
  }
  const body: unknown = await res.json().catch(() => {
    throw new AssessmentError('the body was not JSON')
  })
  return parseAssessment(body)
}

type Fields = Record<string, unknown>

const isObject = (v: unknown): v is Fields => typeof v === 'object' && v !== null
const isNumber = (v: unknown): v is number => typeof v === 'number' && Number.isFinite(v)
const isString = (v: unknown): v is string => typeof v === 'string'
const isTime = (v: unknown): v is string => isString(v) && Number.isFinite(Date.parse(v))
const isDay = (v: unknown): v is string => isString(v) && Number.isFinite(Date.parse(`${v}T00:00:00Z`))

function isCandidate(v: unknown): v is Candidate {
  if (!isObject(v) || !isObject(v.weather) || !isString(v.name)) return false
  const numeric = ['rank', 'area_id', 'sample_days', 'mean_trips', 'min_trips', 'max_trips', 'city_share']
  if (!numeric.every((key) => isNumber(v[key]))) return false
  const { state, precipitation_mm, valid_at } = v.weather
  if (valid_at != null && !isTime(valid_at)) return false
  if (state === 'valid') return isNumber(precipitation_mm)
  return state === 'invalid' || state === 'missing'
}

function isHistory(v: unknown): v is HistoryEvidence {
  return (
    isObject(v) &&
    isString(v.dataset_id) &&
    isDay(v.period_start) &&
    isDay(v.period_end) &&
    isNumber(v.rows_loaded) &&
    isNumber(v.trips_included) &&
    isNumber(v.trips_excluded_invalid_area)
  )
}

export function parseAssessment(raw: unknown): Assessment {
  if (!isObject(raw)) throw new AssessmentError('malformed response')
  const { status, valid_until, area, weather } = raw
  if (!isString(raw.reason) || !isNumber(raw.trigger_mm) || !isTime(raw.slot_start) || !isTime(raw.assessed_at)) {
    throw new AssessmentError('missing required fields')
  }
  if (!Array.isArray(raw.candidates) || raw.candidates.length > 5 || !raw.candidates.every(isCandidate)) {
    throw new AssessmentError('malformed candidates')
  }
  if (!isHistory(raw.history)) throw new AssessmentError('malformed history')
  // simulated must be stated: a missing flag would show a simulated reading as live.
  if (!isObject(weather) || !isString(weather.source) || typeof weather.simulated !== 'boolean') {
    throw new AssessmentError('malformed weather source')
  }

  const candidates: Candidate[] = raw.candidates
  const common: Common = {
    reason: raw.reason,
    slot_start: raw.slot_start,
    assessed_at: raw.assessed_at,
    trigger_mm: raw.trigger_mm,
    candidates,
    history: raw.history,
    weather: {
      source: weather.source,
      simulated: weather.simulated,
      fetched_at: isTime(weather.fetched_at) ? weather.fetched_at : null,
    },
    limitations: Array.isArray(raw.limitations) ? raw.limitations.filter(isString) : [],
  }

  switch (status) {
    case 'action': {
      if (!isTime(valid_until)) throw new AssessmentError('action without an expiry')
      const id = isObject(area) ? area.area_id : undefined
      const winner = candidates.find((c) => c.area_id === id && c.weather.state === 'valid')
      if (!winner) throw new AssessmentError('action on an area without a valid reading')
      return { ...common, status, winner, valid_until }
    }
    case 'no_action':
      if (!isTime(valid_until)) throw new AssessmentError('no_action without an expiry')
      if (candidates.length === 0 || !candidates.every((c) => c.weather.state === 'valid')) {
        throw new AssessmentError('no_action without a valid reading for every area')
      }
      return { ...common, status, valid_until }
    case 'unavailable':
      if (valid_until !== null) throw new AssessmentError('unavailable with an expiry')
      if (!isString(raw.reason_code)) throw new AssessmentError('unavailable without a reason code')
      return { ...common, status, valid_until, reason_code: raw.reason_code }
    default:
      throw new AssessmentError('unknown status')
  }
}
