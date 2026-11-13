# Plan and status

Status on 2026-11-13: implemented and running on real data. Earlier versions of this plan (a full-year baseline, then a synthetic seed while the data portal was failing) are recorded in `docs/decisions.md`.

## Product

One recommendation: put the busiest Chicago community area with modeled precipitation at the trigger on watch for one hour, or say that no watch is needed. History says where pickups concentrate for this weekday and hour; the live weather model says whether anything is happening there now. The app makes no claim about live demand, supply, shortage or causality.

## Data and SQL

- Source: Chicago TNP trips archive `m6dm-c72p`, 12 September to 9 October 2022 (28 days, no US federal holiday), acquired one hour at a time with half-open windows and committed as `data/seed.json`.
- Three SQLite tables: `areas`, `pickup_hourly` (one row per area, day and hour), `dataset_imports`.
- The ranking query aggregates the matching days per area (mean, range, sample size), computes each area's share of the city with a window function, and returns the top five by mean pickups.

## Weather and decision

- One Open-Meteo request per assessment for up to five points; 3 s timeout, 1 MiB cap, no retries.
- A reading counts only if it is in mm, covers 900 s, is not negative and is at most 30 minutes old.
- Walk the five in rank order: an invalid reading above the winner makes the result unavailable; the first reading at or above 0.2 mm wins; all dry means hold.
- The result expires after 5 minutes, at the end of the Chicago clock hour, or when a reading it used turns 30 minutes old.

## Interface

One page: the recommendation with its evidence and caveat, the five areas with historical and live columns labelled as such, provenance, known limits. Manual refresh only. Every time is shown in Chicago time. A simulated weather source is labelled wherever a reading appears.

## Milestones

| | Milestone | Status |
|---|---|---|
| 1 | Real historical extract, reconciled against the source | done |
| 2 | Schema, ranking query, Go API | done |
| 3 | Live weather, decision rule, failure handling | done |
| 4 | Interface, expiry, error states, phone width | done |
| 5 | Tests, screenshots, README, decision and AI-use records | done |
| 6 | Explicit states: fail closed on missing evidence, reason codes, refresh in place, server-relative expiry | done |
| 7 | Automatic re-check, frontend parser tests, hosted entry point, Dockerfile, AI-use section | done; deployment itself pending |

## Checks

`make test` (Python acquisition tests, Go tests, vet, frontend lint, type-check and parser tests) and `npm run build`. A clean start is `make setup && make run`; setup always rebuilds the local database. `make demo` shows the action state with simulated weather, and `make run ARGS="-weather-url http://127.0.0.1:9/"` the unavailable state.

## Not done

Deployed at https://operations-control-room.vercel.app from the container image. The action state was first seen on real weather on 12 November 2026 (0.20 mm modeled at the busiest areas); before that it had only been exercised with simulated weather.
