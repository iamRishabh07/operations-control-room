import { useCallback, useEffect, useRef, useState } from 'react'
import { Assessment, AssessmentError, deadlineFor, fetchAssessment } from './api'
import { chicagoClock } from './format'
import { Hero } from './components/Hero'
import { CandidatesBoard } from './components/CandidatesBoard'
import { Provenance } from './components/Provenance'
import { Limits } from './components/Limits'

// What the latest refresh did while an assessment stays on screen.
type Refresh = { kind: 'idle' } | { kind: 'pending' } | { kind: 'failed'; message: string }

// loading and error mean there is no assessment to show. Once one has arrived
// it stays on screen until a newer one replaces it: a refresh runs, or fails,
// beside it. deadline is when it stops being current, on this browser's clock.
type ViewState =
  | { kind: 'loading' }
  | { kind: 'error'; message: string }
  | { kind: 'ready'; assessment: Assessment; deadline: number | null; expired: boolean; refresh: Refresh }

const REQUEST_TIMEOUT_MS = 5_000

// The page re-checks by itself: shortly before an assessment expires, and a
// minute after an unavailable answer or a failed request. The minimum gap
// keeps a response with almost no validity left from causing rapid requests.
const RECHECK_LEAD_MS = 10_000
const RECHECK_RETRY_MS = 60_000
const RECHECK_MIN_GAP_MS = 5_000

const isPast = (deadline: number | null) => deadline !== null && Date.now() >= deadline

export function App() {
  const [view, setView] = useState<ViewState>({ kind: 'loading' })
  const abortRef = useRef<AbortController | null>(null)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const recheckTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const recheckAtRef = useRef<number | null>(null) // when the next automatic request is due

  // background is true for the page's own re-checks. They change nothing on
  // screen until they finish, so a notice already shown is not removed and
  // announced again every minute during an outage.
  const refresh = useCallback(async (background = false) => {
    abortRef.current?.abort()
    if (recheckTimerRef.current) clearTimeout(recheckTimerRef.current)
    recheckAtRef.current = null
    const ctrl = new AbortController()
    abortRef.current = ctrl
    if (!background) {
      setView((v) => (v.kind === 'ready' ? { ...v, refresh: { kind: 'pending' } } : { kind: 'loading' }))
    }
    const requestedAt = Date.now()
    // A hidden tab makes no request; the visibility handler catches up when it is shown.
    const recheckAt = (at: number) => {
      if (recheckTimerRef.current) clearTimeout(recheckTimerRef.current)
      recheckAtRef.current = at
      recheckTimerRef.current = setTimeout(
        () => {
          if (document.visibilityState === 'visible') refresh(true)
        },
        Math.max(0, at - Date.now()),
      )
    }
    let timedOut = false
    const timeout = setTimeout(() => {
      timedOut = true
      ctrl.abort()
    }, REQUEST_TIMEOUT_MS)
    try {
      const assessment = await fetchAssessment(ctrl.signal)
      const deadline = deadlineFor(assessment, requestedAt)
      const expired = isPast(deadline)
      // Only a new assessment replaces the expiry timer, so the one on screen
      // keeps counting down through a slow or failed refresh.
      if (timerRef.current) clearTimeout(timerRef.current)
      timerRef.current =
        deadline === null || expired
          ? null
          : setTimeout(
              () => setView((v) => (v.kind === 'ready' ? { ...v, expired: true } : v)),
              deadline - Date.now(),
            )
      setView({ kind: 'ready', assessment, deadline, expired, refresh: { kind: 'idle' } })
      recheckAt(
        deadline === null
          ? requestedAt + RECHECK_RETRY_MS
          : Math.max(deadline - RECHECK_LEAD_MS, requestedAt + RECHECK_MIN_GAP_MS),
      )
    } catch (err) {
      // An abort that is not our timeout means a newer request or an unmount took over.
      if (ctrl.signal.aborted && !timedOut) return
      const message = timedOut
        ? `No response within ${REQUEST_TIMEOUT_MS / 1000} seconds.`
        : err instanceof AssessmentError
          ? `The response was rejected: ${err.message}.`
          : 'The server could not be reached.'
      setView((v) => (v.kind === 'ready' ? { ...v, refresh: { kind: 'failed', message } } : { kind: 'error', message }))
      recheckAt(Date.now() + RECHECK_RETRY_MS)
    } finally {
      clearTimeout(timeout)
    }
  }, [])

  useEffect(() => {
    refresh()
    // Timers do not run while a tab sleeps, so catch up on return: mark an
    // assessment whose time has passed, and make the request that came due.
    const onVisible = () => {
      if (document.visibilityState !== 'visible') return
      setView((v) => (v.kind === 'ready' && !v.expired && isPast(v.deadline) ? { ...v, expired: true } : v))
      if (recheckAtRef.current !== null && Date.now() >= recheckAtRef.current) refresh(true)
    }
    document.addEventListener('visibilitychange', onVisible)
    return () => {
      document.removeEventListener('visibilitychange', onVisible)
      abortRef.current?.abort()
      if (timerRef.current) clearTimeout(timerRef.current)
      if (recheckTimerRef.current) clearTimeout(recheckTimerRef.current)
    }
  }, [refresh])

  const busy = view.kind === 'loading' || (view.kind === 'ready' && view.refresh.kind === 'pending')

  return (
    <main className="page">
      <header className="topbar">
        <span className="wordmark">Operations Control Room</span>
        <span className="breadcrumb">
          Chicago
          {view.kind === 'ready' &&
            ` · historical pickups + ${view.assessment.weather.simulated ? 'simulated weather' : 'live weather model'}`}
        </span>
      </header>

      {view.kind === 'ready' && view.assessment.weather.simulated && (
        <p className="banner" role="note">
          <strong>Simulated weather.</strong> The weather adapter is pointed at {view.assessment.weather.source},
          not Open-Meteo. Precipitation values on this page are not live readings.
        </p>
      )}

      {view.kind === 'ready' && view.refresh.kind === 'failed' && (
        <p className="banner" role="alert">
          <strong>Refresh failed.</strong> {view.refresh.message} Still showing the{view.expired ? ' expired' : ''}{' '}
          assessment made at {chicagoClock(view.assessment.assessed_at)}.
        </p>
      )}

      {view.kind === 'loading' && <LoadingHero />}
      {view.kind === 'error' && <ErrorHero message={view.message} />}
      {view.kind === 'ready' && <Hero assessment={view.assessment} expired={view.expired} />}

      <div className="toolbar">
        <button
          type="button"
          className="btn btn-primary"
          // aria-disabled rather than disabled: a disabled button drops keyboard focus.
          onClick={busy ? undefined : () => refresh()}
          aria-disabled={busy}
        >
          {busy && <span className="spinner" aria-hidden="true" />}
          {busy ? 'Refreshing…' : 'Refresh'}
        </button>
      </div>

      {view.kind === 'ready' && (
        <>
          <div className={view.expired ? 'stale' : undefined}>
            <CandidatesBoard a={view.assessment} />
          </div>
          <Provenance a={view.assessment} />
          <Limits a={view.assessment} />
        </>
      )}
    </main>
  )
}

function LoadingHero() {
  return (
    <section className="hero" aria-busy="true">
      <p className="status-row" data-status="unavailable">
        <span className="status-dot" aria-hidden="true" />
        <span>Loading</span>
      </p>
      <h1 className="hero-headline muted-headline">
        Reading the historical baseline &amp; the live weather model…
      </h1>
    </section>
  )
}

function ErrorHero({ message }: { message: string }) {
  return (
    <section className="hero" role="alert">
      <p className="status-row" data-status="unavailable">
        <span className="status-dot" aria-hidden="true" />
        <span>Error</span>
      </p>
      <h1 className="hero-headline">The assessment could not be loaded.</h1>
      <p className="hero-why">
        {message} Nothing is shown rather than something stale. The page tries again every minute; press Refresh to
        try now.
      </p>
    </section>
  )
}
