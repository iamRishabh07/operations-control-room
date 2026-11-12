# Operations Control Room

A small web app for a Chicago rideshare marketplace operator. It surfaces **one** current recommendation: *"put area X on watch for the next hour"* — or, honestly, *"no action required"* / *"unavailable"*. The signal is historical Chicago pickup volume (which area tends to be busiest at this weekday/hour) combined with **modeled precipitation right now** from Open-Meteo.

The product deliberately does **not** claim to know current demand, driver supply, surge, or ETAs. It is a conservative heads-up, not an auto-dispatch.

Uber final-round full-stack take-home. Scoped to ~8–10 hours; prioritises a working, defensible slice over breadth.

## Run it

Requires Go 1.25+, Node 22+, Python 3, GNU make. Two commands:

```sh
make setup   # fetch deps, build frontend, generate seed if missing, init SQLite
make run     # serve API + frontend on http://127.0.0.1:8080
```

Then open <http://127.0.0.1:8080>.

Other targets: `make test`, `make lint`, `make build`, `make clean`.

## What you will see

- A single banner near the top of the page: **"Development baseline"** — the historical pickup counts are a labelled, deterministic synthetic profile (see [Data provenance](#data-provenance) below).
- A single conclusion: *action / no_action / unavailable*, with reason, expiry and historical baseline evidence.
- Top 5 historically-busy community areas for the current Chicago weekday+hour, each with its live modeled precipitation.
- Explicit limitations and source timestamps.

Refresh uses the live Open-Meteo API. If every candidate's modeled precipitation is below `0.2 mm` (per 15-minute bucket) the app returns `no_action`. If the highest-ranked area is at or above the trigger, it becomes the one action.

## Architecture

```
frontend/ (React + TypeScript + Vite, strict)
   │
   │ GET /api/recommendation
   ▼
backend/ (Go 1.25, net/http + database/sql + modernc.org/sqlite, pure-Go)
   │
   ├── internal/control/store.go          seed load + ranking SQL
   ├── internal/control/weather.go        Open-Meteo adapter (bounded, strict JSON)
   ├── internal/control/recommendation.go pure decide() with rank-prefix rule
   └── cmd/server/main.go                 compose, listen on 127.0.0.1:8080
```

Request flow: capture UTC assessment time → derive Chicago weekday/hour → SQL ranking (≤ 5 areas) → one bounded Open-Meteo call for those 5 coordinates → normalise + validate per candidate → `Decide(...)` → JSON response with `valid_until`. Client checks `valid_until` before rendering.

Three SQLite tables, enforced by `CHECK` constraints:
- `areas(area_id PK, name, lat, lon, coord_source)` — 77 Chicago community areas.
- `pickup_baseline(area_id, dow, hour, total_trips, PRIMARY KEY(area_id, dow, hour))`.
- `dataset_imports` — append-only provenance row per import.

### The SQL

The ranking query (`backend/internal/control/ranking.sql`) joins `pickup_baseline` with `areas`, filters to the requested weekday+hour, counts the matching calendar days in the historical period with a recursive CTE, and returns the top 5 areas ordered by total pickups with tie-break on `area_id`. Each result includes the mean-per-day calibrated to the recursive day count, so the displayed number is interpretable even though a shared denominator does not change ordering.

At this scope, sorting 77 rows in Go would be trivial. The SQL earns its place by (a) only transferring the top-5 across the SQL/Go boundary, (b) deriving the correct denominator declaratively via the recursive day CTE, and (c) keeping the exclusion rule (`total_trips > 0`) and the tie-break in one place so tests cannot drift from production.

### Rank-prefix decision

Walk candidates in SQL order. If any **higher-ranked** candidate has invalid weather (missing, stale, wrong units, wrong interval) the whole assessment is `unavailable` — we will not announce "put area #3 on watch" while #1 and #2's evidence is in doubt. The first valid candidate at or above `0.2 mm` becomes the action; `valid_until` is clamped to the earliest of (assessedAt + 5 min, the end of the current Chicago hour, the oldest acceptable freshness of every evidence row that influenced the decision). All five valid-and-dry → `no_action`.

## Tests

Three focused groups in `backend/internal/control/`:

- `store_test.go` — real SQLite, real schema, real ranking: basic ordering, tie-break by `area_id`, wrong-hour exclusion, all-zero seed → empty result, negative-trips rejection.
- `recommendation_test.go` — pure `Decide` with fixed clocks: first wet / first dry second wet / all dry / invalid predecessor / no candidates / upstream failure.
- `weather_test.go` — `httptest.Server`: multi-location array, single-location object, server error, stale reading, wrong units, count mismatch.

Run with `make test`.

## Data provenance

The historical pickup baseline is a **synthetic development fixture**, clearly labelled as such in `data/manifest.json` (`synthetic: true`), in the API response (`history.synthetic`), and in the UI banner.

**Why:** the Chicago TNP data portal's grouped-query path (SODA `$group`) timed out repeatedly during submission preparation on every attempted window — full year, single month, single week, single day — against both archive datasets `m6dm-c72p` (2018–2022) and `n26f-ihde` (2023–). Captured evidence is in `docs/decisions.md`. Rather than ship an app that cannot run without a working portal, this submission pivots to a deterministic baseline derived from documented Chicago TNP characteristics (weekday commute peaks, weekend late-night shift, Loop/Near-North/O'Hare dominance).

**What is still real:**
- All 77 Chicago community areas and their coordinates (bounding-box centres of the official boundaries dataset `igwz-8jzy`).
- The live Open-Meteo modeled precipitation call.
- The SQL ranking, the schema, the decision logic, the HTTP service, the UI — every code path a reviewer would evaluate.

**Production path is preserved:**
```sh
make acquire           # bounded 60s / 10MB real annual aggregate
make acquire-monthly   # 12 monthly aggregates, same total budget
```
Once the Chicago data portal recovers for grouped queries, running `make acquire` will replace `data/seed.json` with the real 2022 extract; the app code does not change.

The live weather source is Open-Meteo. One request per assessment (up to 5 coordinates), 3-second client timeout, 1 MiB body cap. Strict JSON decode. Precipitation freshness bounded to ±30 min / −5 min future.

## Limitations

- Historical counts are synthetic; ordering reflects documented patterns, not measured volume.
- Modeled precipitation is a forecast, not a station observation, and can include snow or mixed precipitation.
- Representative coordinates are community-area bounding-box centres; small positional error vs. true centroid is expected (Lake Michigan-adjacent areas disclosed via bounds check).
- The watch recommendation is a 1-hour operator review window. The assessment itself expires at `valid_until` (always ≤ 5 min).
- The app does not track whether the operator actually started a watch.

## AI use disclosure

See [`docs/ai-use-log.md`](docs/ai-use-log.md) for the full record. In short: AI assistance (Claude Code) was used throughout for design discussion, scaffolding and test cases. Several AI suggestions were rejected or materially changed — most importantly, a 28-day historical window (expanded to a full year), single-area weather (expanded to top-5 rank-prefix), driver-repositioning actions (rejected: evidence does not justify supply intervention), and a server-side expired-at-hour-crossing branch (replaced by returning `valid_until` and letting the client enforce). The data-source blocker was observed, not simulated, and the dev-baseline pivot is disclosed truthfully rather than hidden as a "cached" or "fallback" seed.
