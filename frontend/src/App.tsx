import { useCallback, useEffect, useRef, useState } from 'react'
import { Assessment, AssessmentError, Candidate, fetchAssessment } from './api'

type ViewState =
  | { kind: 'loading' }
  | { kind: 'ready'; assessment: Assessment; expired: boolean }
  | { kind: 'error'; message: string }

const REQUEST_TIMEOUT_MS = 5_000

export function App() {
  const [view, setView] = useState<ViewState>({ kind: 'loading' })
  const abortRef = useRef<AbortController | null>(null)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const refresh = useCallback(async () => {
    abortRef.current?.abort()
    if (timerRef.current) clearTimeout(timerRef.current)
    const ctrl = new AbortController()
    abortRef.current = ctrl
    setView({ kind: 'loading' })
    const timeout = setTimeout(() => ctrl.abort(), REQUEST_TIMEOUT_MS)
    try {
      const assessment = await fetchAssessment(ctrl.signal)
      const now = Date.now()
      const validUntil = assessment.valid_until ? Date.parse(assessment.valid_until) : NaN
      const expired = assessment.status !== 'unavailable' && Number.isFinite(validUntil) && validUntil <= now
      setView({ kind: 'ready', assessment, expired })
      if (!expired && Number.isFinite(validUntil)) {
        timerRef.current = setTimeout(
          () => setView((v) => (v.kind === 'ready' ? { ...v, expired: true } : v)),
          Math.max(0, validUntil - now),
        )
      }
    } catch (err) {
      if (ctrl.signal.aborted) return
      const message = err instanceof AssessmentError ? err.message : 'request failed'
      setView({ kind: 'error', message })
    } finally {
      clearTimeout(timeout)
    }
  }, [])

  useEffect(() => {
    refresh()
    const onVisible = () => {
      if (document.visibilityState !== 'visible') return
      setView((v) => {
        if (v.kind !== 'ready' || v.expired) return v
        const until = v.assessment.valid_until ? Date.parse(v.assessment.valid_until) : NaN
        if (Number.isFinite(until) && until <= Date.now()) return { ...v, expired: true }
        return v
      })
    }
    document.addEventListener('visibilitychange', onVisible)
    return () => {
      document.removeEventListener('visibilitychange', onVisible)
      abortRef.current?.abort()
      if (timerRef.current) clearTimeout(timerRef.current)
    }
  }, [refresh])

  return (
    <main className="page">
      <header>
        <h1>Operations Control Room</h1>
        <p className="subtitle">Chicago community areas · historical pickup baseline × live modeled precipitation</p>
      </header>
      {view.kind === 'ready' && view.assessment.history?.synthetic && (
        <div className="banner" role="note">
          <strong>Development baseline.</strong> {view.assessment.history.synthetic_notice ??
            'Historical counts are synthetic; see README.'}
        </div>
      )}
      <section className="conclusion" aria-live="polite">
        {view.kind === 'loading' && <p>Loading current recommendation…</p>}
        {view.kind === 'error' && (
          <p className="error">The assessment could not be loaded ({view.message}). Try Refresh.</p>
        )}
        {view.kind === 'ready' && <Conclusion v={view} />}
      </section>
      <section className="controls">
        <button type="button" onClick={refresh} disabled={view.kind === 'loading'}>
          Refresh assessment
        </button>
      </section>
      {view.kind === 'ready' && <Evidence a={view.assessment} />}
    </main>
  )
}

function Conclusion({ v }: { v: Extract<ViewState, { kind: 'ready' }> }) {
  const a = v.assessment
  if (v.expired) {
    return <p className="expired">Assessment expired at {fmtTime(a.valid_until)}. Refresh to re-evaluate.</p>
  }
  if (a.status === 'action' && a.area) {
    return (
      <div>
        <p className="action">
          <strong>Put {a.area.name} (Community Area {a.area.area_id}) on watch for the next hour.</strong>
        </p>
        <p className="why">
          {a.reason}. Consult your internal wait and cancellation signals before intervening — this app does not observe
          live rider demand or driver supply.
        </p>
        <p className="meta">Valid until {fmtTime(a.valid_until)} · assessed at {fmtTime(a.assessed_at)}.</p>
      </div>
    )
  }
  if (a.status === 'no_action') {
    return (
      <div>
        <p className="no-action">No action required.</p>
        <p className="why">{a.reason} across the {a.candidates.length} evaluated area(s).</p>
        <p className="meta">Valid until {fmtTime(a.valid_until)} · assessed at {fmtTime(a.assessed_at)}.</p>
      </div>
    )
  }
  return (
    <div>
      <p className="unavailable">Unavailable.</p>
      <p className="why">{a.reason}</p>
    </div>
  )
}

function Evidence({ a }: { a: Assessment }) {
  return (
    <>
      <section className="evidence">
        <h2>Candidates (top {a.candidates.length} by historical pickups)</h2>
        <table>
          <thead>
            <tr>
              <th>#</th>
              <th>Area</th>
              <th>Mean pickups / matching day</th>
              <th>Total (period)</th>
              <th>Sample days</th>
              <th>Modeled precip (mm/15m)</th>
            </tr>
          </thead>
          <tbody>
            {a.candidates.map((c) => (
              <CandidateRow key={c.area_id} c={c} />
            ))}
            {a.candidates.length === 0 && (
              <tr>
                <td colSpan={6}>No historical candidates for this hour.</td>
              </tr>
            )}
          </tbody>
        </table>
      </section>
      <section className="evidence">
        <h2>Data provenance</h2>
        {a.history && (
          <dl>
            <dt>Historical dataset</dt>
            <dd>
              {a.history.dataset_id} · {a.history.period_start} to {a.history.period_end}
              {a.history.synthetic && ' (synthetic)'}
            </dd>
            <dt>Rows loaded</dt>
            <dd>{a.history.rows_loaded.toLocaleString()} buckets</dd>
            <dt>Weather source</dt>
            <dd>
              {a.weather.source || 'none'}{' '}
              {a.weather.fetched_at && <>· fetched {fmtTime(a.weather.fetched_at)}</>}
            </dd>
          </dl>
        )}
      </section>
      <section className="evidence">
        <h2>Limitations</h2>
        <ul>
          {a.limitations.map((l) => (
            <li key={l}>{l}</li>
          ))}
        </ul>
      </section>
    </>
  )
}

function CandidateRow({ c }: { c: Candidate }) {
  const w = c.weather
  const precip = w.state === 'valid' ? `${w.precipitation_mm?.toFixed(2)}` : `— (${w.reason ?? 'invalid'})`
  return (
    <tr>
      <td>{c.rank}</td>
      <td>
        {c.name} <span className="muted">({c.area_id})</span>
      </td>
      <td>{c.mean_pickups_per_day.toFixed(1)}</td>
      <td>{c.total_trips.toLocaleString()}</td>
      <td>{c.sample_days}</td>
      <td>{precip}</td>
    </tr>
  )
}

function fmtTime(iso: string | null): string {
  if (!iso) return '—'
  try {
    return new Date(iso).toLocaleTimeString(undefined, {
      hour: '2-digit',
      minute: '2-digit',
      timeZoneName: 'short',
    })
  } catch {
    return iso
  }
}
