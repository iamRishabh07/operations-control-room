# Decisions

**How to read this.** Entries are in the order they were made. The last three sections, "Correction pass", "State pass" and "Submission pass", describe what ships; where they disagree with an earlier entry, the earlier one is marked superseded.

## Approval record — 2026-11-12
The user approved implementing the final plan. Core product decisions carried forward from the planning phase:
- Use historical Chicago pickup activity plus live modeled precipitation to prioritise an operator **watch**; reject supply-side driver-repositioning actions because the evidence does not support them.
- Full-year weekday/hour baseline; top-5 rank-prefix evaluation (earlier 28-day / single-area proposals were rejected during planning). *Superseded for the baseline: the portal cannot serve a full year, so four weeks are used. The top-5 rank-prefix rule stands.*
- Rank-prefix tolerates invalid lower-ranked weather after a winner but blocks on any invalid higher-ranked candidate.
- Calendar-day denominator improves interpretability; it does not change ordering among the matching weekday/hour. *Superseded: the table now stores one row per day, and SQL computes the mean directly.*
- Bounding-box centres for areas with explicit bounds check; geographic precision is a disclosed limitation.
- Pure-Go stack: `net/http`, `database/sql`, `modernc.org/sqlite`, React/TS/Vite, plain CSS. No production infra.
- Contract-only API smoke assertions; fixed-clock Go tests prove specific outcomes.

## Source gate result — 2026-11-13
*Superseded the same day: real data was acquired and the synthetic seed removed. See "Correction pass" below.*

The Chicago TNP data portal's grouped-query path was unresponsive at submission preparation. Captured evidence:

- `m6dm-c72p` (2018–2022 archive) — annual grouped query times out at 55–60 s; monthly grouped query also times out; single-week grouped query times out; single-day single-group (count by area only) times out. Simple non-aggregated probe `?$limit=1` returns 200.
- `n26f-ihde` (2023+) — single-week grouped query times out identically.
- `jcxq-k9xf` — returned HTTP 503 ("Service unavailable").
- Community-area boundaries endpoint `igwz-8jzy` — returned real data for the full 77 areas (used as the real geo source).

Decision: pivot the historical baseline to a deterministic **development seed** labelled `synthetic: true`, surfaced in the UI banner, documented in README. The real acquisition script `scripts/acquire.py` is preserved verbatim; running `make acquire` once the portal recovers will replace the synthetic seed with the real 2022 extract without changing any application code. No synthetic data is passed off as real: every seed, API response and UI render carries the `synthetic` flag.

## Implementation-time decisions
- `dataset_imports` is append-only; startup reads `ORDER BY id DESC LIMIT 1` (dropped earlier `CHECK (id = 1)` singleton constraint — see prior review `H4`).
- Assessment anchor is the **start of the current Chicago hour**, not "now"; the historical bucket and the UI's displayed time always agree (addresses prior review `H3`). *Field renamed in the "State pass": the hour start is `slot_start`; `assessed_at` is now the time of assessment.*
- Rank-prefix validation is now two explicit steps: (a) every candidate's weather is validated and attached to the response before `Decide` runs; (b) `Decide` applies the rule against the already-validated list. Reviewers get honest `candidates[*].weather.state` even for lower-ranked candidates that did not influence the winner (addresses prior review `C4`).
- `time.LoadLocation("America/Chicago")` with embedded `time/tzdata`; `.Truncate(time.Hour)` in local time. No UTC shortcut (addresses prior review `H2`). *Superseded: this was backwards, see "Correction pass".*
- API integration smoke-test executes through the real Go handler via `net/http/httptest.Server` in unit tests; no separate Node `.mjs` script (addresses prior review `H6`).

## Correction pass — 2026-11-13
A skeptical review of the running app found defects in the data, the ranking and the labelling. Each decision below replaces an earlier one.

**Real data through narrow queries.** The portal's `$group` parameter times out, but the `$query` form answers a one-hour aggregation in a few seconds. `scripts/acquire.py` now sends one query per date and hour, caches every response under `data/raw/` in a folder keyed by the query text, and can be rerun to resume. The synthetic seed, its generator, the fallback area file and the `synthetic` fields are deleted.

**Half-open hourly windows.** The first real extract used SoQL `between`, which is inclusive at both ends. Start times are rounded to 15 minutes, so trips stamped HH:00 were counted in two hours. Evidence: seed total for Mon 2022-10-10 08:00 was 10,084; a live half-open count gave 8,025; trips stamped exactly 09:00:00 numbered 2,059; 8,025 + 2,059 = 10,084. Windows are now `>= start AND < end`, a unit test pins that, and the extract was re-acquired.

**Publish only complete, reconciled extracts.** A failed hour is never zero-filled, and an empty answer counts as a failure. The script publishes only when every window is cached and three adjacent-hour pairs equal an independent live two-hour count; otherwise it exits 1 and says how to resume. The first four-week run finished 670 of 672 windows in 991 s (one timeout, one HTTP 503) and published nothing; the rerun fetched the two missing windows in 10 s and passed.

**Acquisition bounds.** The earlier budget of 60 s and 10 MB per attempt assumed a single query and could not be met. New bounds: 30 s per request, 4 tries, 8 workers, stop after 24 failed windows. Total download is about 2 MB.

**Baseline period: 12 September to 9 October 2022.** Four whole weeks from the assignment-linked dataset with no US federal holiday. The earlier single week (10–16 October) contained Columbus Day and gave one observation per slot. The window includes the Chicago Marathon on its last Sunday; the range column exposes that kind of outlier. Result: 51,744 rows, 5,068,366 trips, 496,856 (8.9%) excluded for having no community area. The excluded count is measured from the no-area group in each response; the earlier manifest claimed 0 without measuring.

**Observation-grain table, aggregation in SQL.** `pickup_hourly(area_id, service_date, hour, trips)` replaces the pre-aggregated table. The ranking query does the `GROUP BY`, mean, range, sample size and city share. The weekday is derived from the date in SQL.

**Rank by mean pickups, not by a ratio.** A "surge ratio" (this hour against the area's own average hour) was tried and put Hegewisch first with 9 trips while Near North Side had 1,562. A ratio is noise on small counts, and "excess over own average" fails at night when every area is below its average. Volume holds in every slot: across all 168 weekday-hours the weakest top area averages 95 pickups and the weakest fifth 33.5.

**Hour start by truncating the instant.** Rebuilding the hour from local date parts is ambiguous in the repeated hour when daylight saving ends. `now.Truncate(time.Hour)` is exact because Chicago's offset is always a whole number of hours. A test covers both passes of 01:30 on 2026-11-01.

**Simulation seam for demonstration.** `-weather-url` lets the adapter read from another endpoint; `make demo` points it at `scripts/sim_weather.py`. Any source other than Open-Meteo sets `weather.simulated` and a banner. Chosen over a fake-data mode inside the server so that everything except the weather input stays real.

**Left as is.** Weather lead tolerance of 16 minutes (four observed readings were all stamped at the start of the running 15-minute interval, so the slack is harmless). Bounding-box centres (the weather grid is kilometres wide). The rank-prefix rule.

## State pass — 2026-11-13
A review of every place the app is "in one of a few states" found several held as loose values, empty strings or prose. The ones that could mislead an operator were made explicit. The rule, the SQL, the data and the wording of the action and hold states did not change.

**A missing reading blocks the decision, inside `Decide`.** `Decide` used to block only on `invalid`; a candidate with no reading fell through and counted as dry. It was unreachable because the handler passed a separate "upstream failed" string, and no test pinned it. `Decide` now returns unavailable for anything that is not a `valid` reading with a value, and the string parameter is gone. A test written first failed with `no_action` for a candidate with no reading and for a `valid` one without a value.

**`missing` is a named evidence state.** The API used to send `"state": ""` for "no reading". It now sends `missing`. Naming the empty string in Go would have been a smaller change, but an empty string in the response is exactly the kind of implicit state this pass set out to remove.

**Reason codes, for unavailable only.** `no_history`, `weather_unavailable`, `weather_invalid`. Action and hold each have one reason, the rule itself, so a code there would repeat the status. The frontend accepts any string, so a new code cannot break the page.

**`slot_start` and a true `assessed_at`.** `assessed_at` held the start of the clock hour, so `valid_until − assessed_at` looked like 28 minutes when the rule is 5. The hour start is now `slot_start`; `assessed_at` is the moment of assessment. The only consumer is this frontend.

**Expiry is counted from the request, not read off the browser clock.** The page compared the server's `valid_until` with its own clock; a clock running slow kept an expired answer on screen. The page now takes the length of validity from the server (`valid_until − assessed_at`) and counts it from the moment it sent the request, which errs early.

**Refresh keeps a valid assessment on screen.** Pressing Refresh used to blank the page, and a failed request discarded a recommendation that was still inside its own validity. The assessment now stays until a newer one arrives; a failed refresh adds a notice; expiry is not postponed. This narrows the earlier rule "nothing is shown rather than something stale" to the case where nothing valid exists (first load). An assessment inside its `valid_until` is not stale by the backend's own definition.

**The response parser enforces what each status guarantees.** Action must name a candidate with a valid reading; hold needs at least one candidate and a valid reading for each; timestamps must parse; `simulated` must be a boolean, because a missing flag would show a simulated reading as live. The backend sends none of the rejected shapes; two of them used to crash or mislabel the page.

**`make setup` always rebuilds the local database.** It holds only rows loaded from the seed, so there is nothing to preserve. Startup still refuses a database that does not match the seed; the message now names one command.

**The page is served with `Cache-Control: no-cache`.** Found while verifying this pass: after a rebuild the browser kept showing the previous bundle, because the server sent no cache header for `index.html`. Asset names carry a content hash and stay cacheable.

**`make test` type-checks the frontend.** It only linted, so a status without copy would have passed.

**Left as is, on purpose.** Live and simulated wording stays as nine one-use phrases rather than a lookup table (the parser fix closes the real risk). The acquisition script's exit codes, the `-init-only` flag and the request-timeout flag in `App.tsx` are small and tested. No automatic refresh on expiry. No frontend unit tests, which would need a new dependency. *Both superseded in the "Submission pass" below.*

## Submission pass — 2026-11-13

**The page re-checks by itself.** Supersedes "no automatic refresh". Left open, the page turned "Expired" within five minutes every time, and one slow weather answer left "No recommendation" on screen until someone clicked. Once refresh happened in place (State pass) an automatic one became safe. Rule: ask again 10 seconds before an assessment expires, and a minute after an unavailable answer or a failed request; never more than once in 5 seconds, which bounds the request rate whatever the server returns; never while the tab is hidden. Automatic requests change nothing on screen until they finish, so during an outage the failure notice is shown once, not removed and re-announced every minute.

**Frontend tests on Node's built-in runner.** Supersedes "no frontend unit tests". Node 22.18 and later load TypeScript directly, so the response parser is tested with no new dependency. The parser is where the page decides whether a response can be trusted, and the browser checks of it were not repeatable by anyone else. `deadlineFor` moved into `api.ts` to be testable. Cost: `make test` needs Node 22.18+; running the app does not.

**One Go module at the repository root.** Vercel's Go runtime requires `go.mod` at the project root. Package import paths are unchanged. Go commands in the `Makefile` name our packages explicitly, because `./...` from the root also matches a Go package inside `frontend/node_modules`.

**Two entry points, one service.** `backend/cmd/server` for local use and containers; `backend/serverless` for function platforms, called from `api/recommendation.go`. Both build the same `control.Service`, so the SQL, the rule and the weather adapter cannot drift apart. The hosted entry compiles the seed in and rebuilds SQLite in the temp directory when an instance starts. A failed start is not remembered, so the next request tries again; a broken database file is deleted and rebuilt once.

**Two deployment paths.** Vercel is the first choice because it does not sleep. Its Go runtime is in beta and its build could not be run locally, so a `Dockerfile` was added and verified as the fallback: the same server, database built into the image, ready in 0.7 s on 0.1 CPU. The container path also lets a reviewer run the app with only Docker installed.
