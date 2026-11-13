// Tests for the API boundary. Run with `npm test`: Node's built-in test runner
// loads ../src/api.ts directly, so there is no test dependency (Node 22.18+).

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { AssessmentError, deadlineFor, parseAssessment } from '../src/api.ts'

const reading = (mm) => ({ state: 'valid', precipitation_mm: mm, valid_at: '2026-11-13T02:45:00Z' })
const missing = () => ({ state: 'missing', precipitation_mm: null })

const candidate = (rank, area_id, name, mm) => ({
  rank,
  area_id,
  name,
  sample_days: 4,
  mean_trips: 1800 / rank,
  min_trips: 100,
  max_trips: 2000,
  city_share: 0.2 / rank,
  weather: reading(mm),
})

// Rank 1 is dry, rank 2 is wet: the server would recommend rank 2.
const base = () => ({
  reason: 'as sent by the server',
  slot_start: '2026-11-13T02:00:00Z',
  assessed_at: '2026-11-13T02:47:00Z',
  trigger_mm: 0.2,
  candidates: [candidate(1, 8, 'Near North Side', 0), candidate(2, 28, 'Near West Side', 0.3)],
  history: {
    dataset_id: 'm6dm-c72p',
    period_start: '2022-09-12',
    period_end: '2022-10-09',
    retrieved_at: '2026-11-12T21:50:09+00:00',
    rows_loaded: 51744,
    trips_included: 5068366,
    trips_excluded_invalid_area: 496856,
  },
  weather: {
    source: 'https://api.open-meteo.com/v1/forecast',
    simulated: false,
    fetched_at: '2026-11-13T02:47:01Z',
  },
  limitations: ['Pickup counts are historical.'],
})

const action = () => ({
  ...base(),
  status: 'action',
  area: { area_id: 28, name: 'Near West Side' },
  valid_until: '2026-11-13T02:52:00Z',
})

const hold = () => {
  const a = { ...base(), status: 'no_action', area: null, valid_until: '2026-11-13T02:52:00Z' }
  a.candidates[1].weather = reading(0.1)
  return a
}

const unavailable = () => {
  const a = { ...base(), status: 'unavailable', reason_code: 'weather_unavailable', area: null, valid_until: null }
  a.candidates.forEach((c) => (c.weather = missing()))
  return a
}

const change = (make, edit) => {
  const a = make()
  edit(a)
  return a
}

test('each status parses, and action carries the winning candidate', () => {
  const a = parseAssessment(action())
  assert.equal(a.status, 'action')
  assert.equal(a.winner.area_id, 28)
  assert.equal(a.winner.weather.precipitation_mm, 0.3)

  assert.equal(parseAssessment(hold()).status, 'no_action')

  const u = parseAssessment(unavailable())
  assert.equal(u.status, 'unavailable')
  assert.equal(u.valid_until, null)
  assert.equal(u.reason_code, 'weather_unavailable')
})

test('a reason code the page has never seen is accepted', () => {
  const u = parseAssessment(change(unavailable, (a) => (a.reason_code = 'added_later')))
  assert.equal(u.reason_code, 'added_later')
})

// Responses the server never sends. Each one would crash a component or
// mislead the operator if it were rendered, so each must be rejected.
const rejected = [
  ['hold with no candidates', change(hold, (a) => (a.candidates = [])), /valid reading for every area/],
  ['hold with a missing reading', change(hold, (a) => (a.candidates[1].weather = missing())), /valid reading for every area/],
  ['action on an area that is not a candidate', change(action, (a) => (a.area.area_id = 999)), /without a valid reading/],
  [
    'action on an area whose reading is invalid',
    change(action, (a) => (a.candidates[1].weather = { state: 'invalid', reason: 'unexpected units', precipitation_mm: null })),
    /without a valid reading/,
  ],
  ['action without an expiry', change(action, (a) => (a.valid_until = null)), /without an expiry/],
  ['an expiry that is not a time', change(hold, (a) => (a.valid_until = 'soon')), /without an expiry/],
  ['unavailable with an expiry', change(unavailable, (a) => (a.valid_until = '2026-11-13T02:52:00Z')), /with an expiry/],
  ['unavailable without a reason code', change(unavailable, (a) => delete a.reason_code), /without a reason code/],
  // A missing flag would show a simulated reading as live.
  ['no simulated flag', change(hold, (a) => delete a.weather.simulated), /malformed weather source/],
  ['the response shape before slot_start existed', change(hold, (a) => delete a.slot_start), /missing required fields/],
  ['a reading stamp that is not a time', change(hold, (a) => (a.candidates[0].weather.valid_at = 'x')), /malformed candidates/],
  ['a valid reading without a value', change(hold, (a) => (a.candidates[0].weather.precipitation_mm = null)), /malformed candidates/],
  ['an evidence state that does not exist', change(hold, (a) => (a.candidates[0].weather.state = '')), /malformed candidates/],
  ['a baseline period that is not a date', change(hold, (a) => (a.history.period_start = 'autumn')), /malformed history/],
  ['a status that does not exist', change(hold, (a) => (a.status = 'maybe')), /unknown status/],
  ['a body that is not an object', 'ok', /malformed response/],
]

for (const [name, body, message] of rejected) {
  test(`rejects ${name}`, () => {
    assert.throws(() => parseAssessment(body), (err) => err instanceof AssessmentError && message.test(err.message))
  })
}

test('the deadline is the server-stated validity counted from the request', () => {
  // 02:47:00 to 02:52:00 is five minutes, whatever this machine's clock says.
  const a = parseAssessment(hold())
  assert.equal(deadlineFor(a, 1_000), 1_000 + 5 * 60 * 1000)
  assert.equal(deadlineFor(a, 9_999_999_999_999), 9_999_999_999_999 + 5 * 60 * 1000)
})

test('an unavailable assessment has no deadline', () => {
  assert.equal(deadlineFor(parseAssessment(unavailable()), 1_000), null)
})
