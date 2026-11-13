import { Assessment, Status } from '../api'
import { chicagoClock, chicagoSlot, period, pickups } from '../format'

interface Props {
  assessment: Assessment
  expired: boolean
}

// What the hero presents: the assessment's status, or "expired" once its time has passed.
type Shown = Status | 'expired'

const LABELS: Record<Shown, string> = {
  action: 'Action',
  no_action: 'Hold',
  unavailable: 'Unavailable',
  expired: 'Expired',
}

interface Copy {
  headline: string
  why: string
  caveat: string | null
}

export function Hero({ assessment: a, expired }: Props) {
  const status: Shown = expired ? 'expired' : a.status
  const slot = chicagoSlot(a.slot_start)
  const { headline, why, caveat } = explain(a, expired, slot)

  return (
    <section
      className="hero"
      data-expired={expired}
      data-reason={a.status === 'unavailable' ? a.reason_code : undefined}
    >
      <p className="status-row" data-status={status}>
        <span className="status-dot" aria-hidden="true" />
        <span>{LABELS[status]}</span>
      </p>
      {/* Only the headline is announced: the hero stays mounted across refreshes. */}
      <h1 className="hero-headline" aria-live="polite">
        {headline}
      </h1>
      <p className="hero-why">{why}</p>
      {caveat && <p className="hero-caveat">{caveat}</p>}

      <dl className="hero-meta">
        <div>
          <dt>Evaluating</dt>
          <dd>
            {slot.weekday.slice(0, 3)} {slot.hours} Chicago
          </dd>
        </div>
        {a.valid_until && (
          <div>
            <dt>{expired ? 'Expired at' : 'Valid until'}</dt>
            <dd>{chicagoClock(a.valid_until)}</dd>
          </div>
        )}
        <div>
          <dt>Trigger</dt>
          <dd>≥ {a.trigger_mm} mm / 15 min</dd>
        </div>
      </dl>
    </section>
  )
}

// One block of copy per status. The declared return type makes a new status
// fail type-checking until it has copy here.
function explain(a: Assessment, expired: boolean, slot: { weekday: string; hours: string }): Copy {
  if (expired) {
    return {
      headline: 'This assessment has expired.',
      why: 'Refresh to re-evaluate. The evidence below belongs to the expired assessment.',
      caveat: null,
    }
  }

  const when = `${slot.weekday}s ${slot.hours}`
  const baseline = period(a.history.period_start, a.history.period_end)
  const simulated = a.weather.simulated

  switch (a.status) {
    case 'action': {
      const winner = a.winner
      return {
        headline: `Put ${winner.name} on watch for the next hour.`,
        why:
          `It is the historically busiest area where ${simulated ? 'the simulated feed' : 'the weather model'} ` +
          `shows precipitation at the trigger. It averaged ${pickups(winner.mean_trips)} pickups on ${when} ` +
          `(${(winner.city_share * 100).toFixed(1)}% of the city, ${winner.sample_days} days, ${baseline}), and ` +
          `${simulated ? 'the simulated weather feed reports' : 'Open-Meteo models'} ` +
          `${winner.weather.precipitation_mm?.toFixed(2)} mm there in the current 15 minutes.`,
        caveat:
          `This is a prompt to look, not a measured problem. It assumes rain shifts demand or driver supply, which ` +
          `the app cannot test: it sees no live rides or drivers. Check wait times and cancellations in ` +
          `${winner.name} before you intervene.`,
      }
    }
    case 'no_action': {
      const wettest = a.candidates.reduce((top, c) =>
        (c.weather.precipitation_mm ?? 0) > (top.weather.precipitation_mm ?? 0) ? c : top,
      )
      return {
        headline: `Hold. None of the ${a.candidates.length} historically busiest areas is at the precipitation trigger.`,
        why:
          `${simulated ? 'Simulated' : 'Modeled'} precipitation is below ${a.trigger_mm} mm / 15 min in every area ` +
          `with the most historical pickups for ${when}. Highest reading: ` +
          `${(wettest.weather.precipitation_mm ?? 0).toFixed(2)} mm (${wettest.name}).`,
        caveat:
          'This says the weather trigger is quiet, not that operations are normal. The app sees no live demand or supply.',
      }
    }
    case 'unavailable':
      return {
        headline: 'No recommendation right now.',
        why: `Reason: ${a.reason}.`,
        caveat: 'Refresh to retry. Until this recovers, decide without this signal.',
      }
  }
}
