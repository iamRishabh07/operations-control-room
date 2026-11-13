import type { CSSProperties } from 'react'
import { Assessment, Candidate } from '../api'
import { chicagoSlot, period, pickups } from '../format'

export function CandidatesBoard({ a }: { a: Assessment }) {
  const slot = chicagoSlot(a.slot_start)
  const topShare = Math.max(...a.candidates.map((c) => c.city_share), 0)

  return (
    <section className="panel" aria-labelledby="evidence-title">
      <header className="panel-head">
        <h2 id="evidence-title">
          Historically busiest areas · {slot.weekday}s {slot.hours}
        </h2>
        <span className="meta">baseline {period(a.history.period_start, a.history.period_end)}</span>
      </header>
      <div className="panel-body">
        <table className="candidates">
          <thead>
            <tr>
              <th className="rank" scope="col">#</th>
              <th scope="col">Area</th>
              <th className="num" scope="col">
                Mean pickups <span className="sub">historical</span>
              </th>
              <th className="num" scope="col">
                Range <span className="sub">across days</span>
              </th>
              <th scope="col">
                Share of city <span className="sub">historical</span>
              </th>
              <th className="num" scope="col">
                Precip, mm / 15 min <span className="sub">{a.weather.simulated ? 'simulated' : 'live model'}</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {a.candidates.length === 0 && (
              <tr>
                <td colSpan={6} className="empty">
                  No pickups recorded for this weekday &amp; hour in the baseline.
                </td>
              </tr>
            )}
            {a.candidates.map((c) => (
              <Row key={c.area_id} c={c} topShare={topShare} trigger={a.trigger_mm} />
            ))}
          </tbody>
        </table>
      </div>
      <p className="panel-footnote">
        Mean and range are pickups in this clock hour on the matching weekday across the baseline. Share is of all
        pickups with a community area in the same slot. Precipitation is{' '}
        {a.weather.simulated
          ? 'from the local simulator, not a live reading'
          : "Open-Meteo's modeled value for the current 15 minutes"}
        ; {a.trigger_mm} mm or more puts the busiest such area on watch.
      </p>
    </section>
  )
}

function Row({ c, topShare, trigger }: { c: Candidate; topShare: number; trigger: number }) {
  const reading = c.weather.state === 'valid' ? c.weather.precipitation_mm : null
  const width = topShare > 0 ? `${(c.city_share / topShare) * 100}%` : '0%'

  return (
    <tr>
      <td className="rank">{c.rank}</td>
      <td>
        <span className="area-cell">
          <span>{c.name}</span>
          <span className="code">#{c.area_id}</span>
        </span>
      </td>
      <td className="num">{pickups(c.mean_trips)}</td>
      <td className="num">
        {c.min_trips.toLocaleString('en-US')}–{c.max_trips.toLocaleString('en-US')}
        <span className="code"> n={c.sample_days}</span>
      </td>
      <td>
        <div className="share">
          <div className="share-bar" style={{ '--w': width } as CSSProperties} />
          <span className="share-value">{(c.city_share * 100).toFixed(1)}%</span>
        </div>
      </td>
      <td className="num">
        <span className="precip" data-wet={reading !== null && reading >= trigger} data-invalid={reading === null}>
          {reading !== null ? reading.toFixed(2) : c.weather.state === 'invalid' ? 'invalid' : '—'}
        </span>
      </td>
    </tr>
  )
}
