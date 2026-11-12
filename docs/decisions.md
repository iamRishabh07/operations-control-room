# Decisions

## Approval record — 2026-11-12
The user approved implementing the final plan. Core product decisions carried forward from the planning phase:
- Use historical Chicago pickup activity plus live modeled precipitation to prioritise an operator **watch**; reject supply-side driver-repositioning actions because the evidence does not support them.
- Full-year weekday/hour baseline; top-5 rank-prefix evaluation (earlier 28-day / single-area proposals were rejected during planning).
- Rank-prefix tolerates invalid lower-ranked weather after a winner but blocks on any invalid higher-ranked candidate.
- Calendar-day denominator improves interpretability; it does not change ordering among the matching weekday/hour.
- Bounding-box centres for areas with explicit bounds check; geographic precision is a disclosed limitation.
- Pure-Go stack: `net/http`, `database/sql`, `modernc.org/sqlite`, React/TS/Vite, plain CSS. No production infra.
- Contract-only API smoke assertions; fixed-clock Go tests prove specific outcomes.

## Source gate result — 2026-11-13
The Chicago TNP data portal's grouped-query path was unresponsive at submission preparation. Captured evidence:

- `m6dm-c72p` (2018–2022 archive) — annual grouped query times out at 55–60 s; monthly grouped query also times out; single-week grouped query times out; single-day single-group (count by area only) times out. Simple non-aggregated probe `?$limit=1` returns 200.
- `n26f-ihde` (2023+) — single-week grouped query times out identically.
- `jcxq-k9xf` — returned HTTP 503 ("Service unavailable").
- Community-area boundaries endpoint `igwz-8jzy` — returned real data for the full 77 areas (used as the real geo source).

Decision: pivot the historical baseline to a deterministic **development seed** labelled `synthetic: true`, surfaced in the UI banner, documented in README. The real acquisition script `scripts/acquire.py` is preserved verbatim; running `make acquire` once the portal recovers will replace the synthetic seed with the real 2022 extract without changing any application code. No synthetic data is passed off as real: every seed, API response and UI render carries the `synthetic` flag.

## Implementation-time decisions
- `dataset_imports` is append-only; startup reads `ORDER BY id DESC LIMIT 1` (dropped earlier `CHECK (id = 1)` singleton constraint — see prior review `H4`).
- Assessment anchor is the **start of the current Chicago hour**, not "now"; the historical bucket and the UI's displayed time always agree (addresses prior review `H3`).
- Rank-prefix validation is now two explicit steps: (a) every candidate's weather is validated and attached to the response before `Decide` runs; (b) `Decide` applies the rule against the already-validated list. Reviewers get honest `candidates[*].weather.state` even for lower-ranked candidates that did not influence the winner (addresses prior review `C4`).
- `time.LoadLocation("America/Chicago")` with embedded `time/tzdata`; `.Truncate(time.Hour)` in local time. No UTC shortcut (addresses prior review `H2`).
- API integration smoke-test executes through the real Go handler via `net/http/httptest.Server` in unit tests; no separate Node `.mjs` script (addresses prior review `H6`).
