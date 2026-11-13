import type { ReactNode } from 'react'
import { Assessment } from '../api'
import { chicagoClock, daysInclusive, period } from '../format'

export function Provenance({ a }: { a: Assessment }) {
  const h = a.history
  const all = h.trips_included + h.trips_excluded_invalid_area
  const excluded = all > 0 ? ((100 * h.trips_excluded_invalid_area) / all).toFixed(1) : '0.0'
  // A reading can be several minutes older than its fetch, so both times are shown.
  const stamped = a.candidates.map((c) => c.weather.valid_at).find((at) => at)

  return (
    <section className="panel" aria-labelledby="provenance-title">
      <header className="panel-head">
        <h2 id="provenance-title">Provenance</h2>
        <span className="meta">Chicago TNP trips · {a.weather.simulated ? 'simulated weather' : 'Open-Meteo'}</span>
      </header>
      <div className="chips">
        <Chip label="Dataset">{h.dataset_id}</Chip>
        <Chip label="Baseline">
          {period(h.period_start, h.period_end)} · {daysInclusive(h.period_start, h.period_end)} days
        </Chip>
        <Chip label="Trips counted">{h.trips_included.toLocaleString('en-US')}</Chip>
        <Chip label="No area, excluded">
          {h.trips_excluded_invalid_area.toLocaleString('en-US')} · {excluded}%
        </Chip>
        <Chip label="Weather">{a.weather.simulated ? 'simulated' : 'Open-Meteo forecast API'}</Chip>
        {stamped && <Chip label="Reading stamped">{chicagoClock(stamped)}</Chip>}
        {a.weather.fetched_at && <Chip label="Fetched">{chicagoClock(a.weather.fetched_at)}</Chip>}
      </div>
      <details className="raw">
        <summary>Import record</summary>
        <pre>{JSON.stringify({ history: h, weather: a.weather }, null, 2)}</pre>
      </details>
    </section>
  )
}

function Chip({ label, children }: { label: string; children: ReactNode }) {
  return (
    <span className="chip">
      <span className="chip-label">{label}</span>
      {children}
    </span>
  )
}
