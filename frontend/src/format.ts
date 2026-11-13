// Every operational time is shown in Chicago time: the market being operated.

const CHICAGO = 'America/Chicago'

export function chicagoClock(iso: string | null): string {
  if (!iso) return '—'
  return new Intl.DateTimeFormat('en-US', {
    hour: '2-digit',
    minute: '2-digit',
    hourCycle: 'h23',
    timeZone: CHICAGO,
    timeZoneName: 'short',
  }).format(new Date(iso))
}

// The weekday-hour slot an assessment evaluates, e.g. Thursday / 14:00–14:59.
export function chicagoSlot(iso: string): { weekday: string; hours: string } {
  const at = new Date(iso)
  const weekday = new Intl.DateTimeFormat('en-US', { weekday: 'long', timeZone: CHICAGO }).format(at)
  const hour = new Intl.DateTimeFormat('en-US', { hour: '2-digit', hourCycle: 'h23', timeZone: CHICAGO }).format(at)
  return { weekday, hours: `${hour}:00–${hour}:59` }
}

// A date-only period such as 12 Sep – 9 Oct 2022.
export function period(start: string, end: string): string {
  const day = (iso: string, withYear: boolean) =>
    new Intl.DateTimeFormat('en-GB', {
      day: 'numeric',
      month: 'short',
      year: withYear ? 'numeric' : undefined,
      timeZone: 'UTC',
    }).format(new Date(`${iso}T00:00:00Z`))
  return `${day(start, false)} – ${day(end, true)}`
}

export function daysInclusive(start: string, end: string): number {
  return Math.round((Date.parse(`${end}T00:00:00Z`) - Date.parse(`${start}T00:00:00Z`)) / 86_400_000) + 1
}

// Means below 10 keep a decimal, so 0.4 pickups does not read as 0.
export const pickups = (n: number): string =>
  n < 10 ? n.toFixed(1) : Math.round(n).toLocaleString('en-US')
