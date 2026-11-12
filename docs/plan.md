# Implementation plan v5 — implemented 2026-11-13

Status: implemented and runnable end-to-end. Historical baseline shipped as a labelled synthetic development seed because the Chicago data portal's grouped-query path was unresponsive during submission preparation (see `docs/decisions.md`). All code paths a reviewer would evaluate are real.

## Product

One recommendation: put the highest-ranked eligible Chicago community area on watch for one hour when current modeled precipitation meets a documented threshold. History selects where to look, not live demand. Operator is assumed to have internal wait/cancellation signals. No shortage, causality, supply or validated-benefit claim.

## Data and SQL

- Source: synthetic development baseline (`data/seed.json`, `history.synthetic=true`) derived from documented Chicago TNP characteristics (weekday commute peaks, weekend nightlife shift, Loop/Near-North/O'Hare dominance). Real community-area list and coordinates from Chicago boundaries endpoint `igwz-8jzy`. The real acquisition script (`scripts/acquire.py`) is preserved for when the portal recovers.
- Schema: three SQLite tables with `CHECK` constraints — `areas`, `pickup_baseline`, `dataset_imports`. Foreign keys enforced per connection, WAL journal, append-only import log.
- Ranking SQL: recursive CTE counts matching calendar days for `:dow` in the reference period; joins baseline + areas; filters `total_trips > 0`; orders `total_trips DESC, area_id ASC`; `LIMIT 5`. The denominator does not change ordering at fixed `(dow, hour)` but keeps the displayed mean interpretable and co-locates filter + tie-break with the query itself.

## Weather + decision

- Open-Meteo forecast endpoint, one call per assessment with up to 5 comma-separated coordinates, 3 s client timeout, 1 MiB body cap, strict JSON (unknown-field tolerant at top level).
- Per-candidate validity: valid `time`/`interval`/`precipitation`, units `mm`, interval 900 s, age within `-5m..+30m`, value finite + nonnegative.
- Decide: walk candidates in rank order; invalid predecessor → unavailable; first valid wet (≥ 0.2 mm) → action; otherwise no_action. `valid_until = min(assessedAt + 5m, next local hour, oldest evidence freshness in the prefix)`.
- Assessment anchor is the **start of the current Chicago hour**; the historical bucket and the displayed time always agree. Timezone comes from `time.LoadLocation("America/Chicago")` with `time/tzdata` embedded.

## API + UI

- `GET /api/recommendation` → one typed assessment JSON (`action` / `no_action` / `unavailable`), `valid_until` null exactly for `unavailable`, candidates ≤ 5, each with normalised weather evidence.
- Internal DB failure → safe `503` with `{ error, message }`. Weather upstream failure → normal `200` with `unavailable` + reason.
- React/TS/Vite UI: one conclusion, a "development baseline" banner driven by `history.synthetic`, evidence table for candidates, data-provenance + limitations sections. Manual refresh only, bounded by `AbortController` + 5 s timeout. Local timer honours `valid_until` and recomputes expiry on `visibilitychange`.

## Tests

Three focused test groups under `backend/internal/control/`:

1. `store_test.go` — real SQLite + real ranking: basic order, tie-break, wrong-hour exclusion, all-zero → empty, negative-trips rejection.
2. `recommendation_test.go` — pure `Decide`, fixed clocks: wet first / dry-wet / all-dry / invalid predecessor / no candidates / upstream failure.
3. `weather_test.go` — `httptest.Server`: multi-location array, single-location object, 500, stale, wrong units, count mismatch.

Plus `scripts/test_acquire.py` acquisition-level checks (bounded fetch, coordinate traversal, integer validation).

## Delivery

- `make setup` installs deps, builds frontend, ensures seed, initialises SQLite.
- `make run` serves API + built frontend at `127.0.0.1:8080`.
- `make test` runs Go tests + vet + frontend lint.
- Clean checkout → `make setup && make run` rehearsed before submission.

## Milestones (status)

1. Source feasibility — **BLOCKED** on real source; pivoted to labelled synthetic baseline. Real `make acquire` preserved.
2. Connected slice — **done**: SQL ranking + HTTP serving.
3. Live decisions — **done**: Open-Meteo integration + rank-prefix tests.
4. UI — **done**: React/TS strict, expiry handling, mobile-friendly table.
5. Submission — **done**: README, decisions, ai-use-log reconciled; tests pass.
