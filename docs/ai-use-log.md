# AI-use log

**How to read this.** Entries are kept as written at the time and are not in date order. The build that ships is described in **Correction pass — 2026-11-13**, **State pass — 2026-11-13** and **Submission pass — 2026-11-13**. Every other entry is history: it records what was done and believed then, including approaches that were later replaced (a full-year baseline, a synthetic seed, a recursive-CTE ranking).

## Toolchain switch — 2026-11-12
Switched project AI harness from Codex to Claude Code. Ported `AGENTS.md` → `CLAUDE.md`, `.codex/agents/*.toml` → `.claude/agents/*.md`, `.codex/hooks/` → `.claude/hooks/`, moved `.agents/skills/` → `.claude/skills/`, and rewrote `docs/CODEX_PROMPTS.md` → `docs/CLAUDE_PROMPTS.md`. Project rules, reviewer responsibilities and the pre-tool-use guard behave the same; only the harness changed.

## Implementation turn — 2026-11-13 (superseded by the correction pass below)
Tool: Claude Code (Opus 4.7). Full implementation session following user approval of the plan and the senior-engineer review in `.claude/plans/users-rishabhsrivastava-developer-opera-delegated-aurora.md`.

What was accepted from AI output (and verified in-session):
- Go module structure (`cmd/server`, `internal/control`), SQLite schema with `CHECK` constraints, recursive-CTE ranking query, Open-Meteo adapter with strict JSON and 1 MiB cap, `Decide()` function with rank-prefix.
- React/TS/Vite UI with runtime validation at the fetch boundary (`parseAssessment`), local expiry timer, `visibilitychange` re-check, 5-second `AbortController` request timeout.
- Three focused Go test groups: SQL ranking, recommendation, weather failures. All pass (`go test ./... -count=1`).

What was rejected or materially changed from AI's earlier proposals:
- Full-year baseline instead of 28 days (earlier AI suggestion rejected during planning).
- Top-5 rank-prefix with evidence-pre-validation instead of walk-and-abort (addresses prior review `C4`).
- `CHECK (id = 1)` singleton removed from `dataset_imports` (prior review `H4`).
- Assessment anchor is start-of-current-hour, not next-hour boundary (prior review `H3`).
- No Node `assert-api.mjs`; a Go `httptest.Server` test exercises the real adapter (prior review `H6`).
- `time.LoadLocation("America/Chicago")` + `time/tzdata`; UTC-truncation shortcut deleted (prior review `H2`).
- Driver-repositioning product reframed as operator "watch" because the evidence does not justify supply intervention (rejected in planning).

Source blocker observed, not simulated:
- Chicago data portal grouped queries timed out across every tested window and both archive datasets (`m6dm-c72p`, `n26f-ihde`). Non-grouped probes returned 200. `jcxq-k9xf` returned 503. Boundaries endpoint `igwz-8jzy` returned real data and is used as the real geo source.
- Pivoted to a deterministic synthetic baseline (`scripts/bootstrap_dev_seed.py`), clearly labelled `synthetic=true` in the manifest, the API response, and a UI banner. Real `make acquire` preserved. No synthetic data is passed off as real.

Validation this turn:
- `make setup` ran clean from empty `data/`.
- `make run` served `/api/recommendation` (status `no_action`, 5 candidates, live Open-Meteo precipitation 0.00 mm at Loop / Near North / Near West / O'Hare / Lincoln Park) and the built React UI (`<title>Operations Control Room</title>` at `/`).
- `go test ./...` passed all three test groups; `go vet` clean; `npm run lint` clean; `npm run build` produced a 48 KB gzipped bundle.
- Hardest technical part: diagnosing that the Chicago portal's `$group` path was broadly unresponsive and deciding honestly to ship a labelled synthetic baseline rather than either (a) silently relaxing the acquisition budget or (b) shipping an app that cannot boot.

## Correction pass — 2026-11-13
Tool: Claude Code. The user rejected the synthetic baseline, asked for a skeptical review of the running app, and approved the correction plan before any code changed. The entry above describes a build that this pass replaced.

Errors the AI had introduced, and what exposed each one:
- **Double-counted trips.** The AI's acquisition rewrite queried each hour with SoQL `between`, inclusive at both ends. Start times are rounded to 15 minutes, so HH:00 trips were counted twice. Exposed by comparing the seed with the live source: seed Mon 2022-10-10 08:00 = 10,084; live half-open count = 8,025; trips stamped exactly 09:00:00 = 2,059; 8,025 + 2,059 = 10,084.
- **Ranking that promoted noise.** An earlier AI review recommended ranking by a "surge ratio". On the live app it put Hegewisch first with 9 trips; Near North Side had 1,562 in the same slot. Replaced by mean pickups.
- **Invented metadata.** The manifest said 0 trips were excluded for lacking an area. Nothing had measured it; one live hour showed 801. It is now measured (8.9% over the period).
- **Untested rewrite.** The AI rewrote `acquire.py` and did not re-run its tests. Reading them showed 2 of 3 would fail, and `make acquire-monthly` passed a flag that no longer existed.
- **A daylight-saving "fix" that was backwards.** An earlier AI review called UTC truncation a bug and replaced it with local-time reconstruction. A new test for the repeated hour failed against that code (hour start 06:00Z for an instant at 07:30Z) and passes with truncation.
- **Historical counts presented as live.** The AI's redesign labelled 2022 counts "This hour" under a "Chicago · live" header.
- **Two UI failure cases.** A request timeout left the page on "Refreshing…" with the button disabled; a malformed candidate could crash rendering.
- **Demo simulator that never listened.** Python's `HTTPServer` resolves its hostname on bind and stalled; found on the first `make demo`, replaced with a plain `TCPServer`.

AI output accepted in this pass, after checking: per-hour acquisition with a resumable cache and a publish gate; the observation-grain table and the `GROUP BY` plus window ranking query; the `-weather-url` seam with simulated labelling; the evidence and caveat sentences in the recommendation.

How it was validated (actual results):
- `make test`: 10 Python tests, 25 Go tests, `go vet`, frontend lint, all passing. `npm run build` passes.
- Two read-only reviewer agents (SQL/data and product claims) read the finished diff. Neither found a critical or high defect in the code. Their points were applied: an empty source answer is now a failed window instead of a zero hour; the cache folder is keyed by the query text; startup compares row count and trip total with the seed, not only metadata; "busiest" is always qualified as historical; the action caveat states the rain assumption; the reading's own timestamp is shown.
- Acquisition: 672 windows. First run 991 s, 670 cached, 2 failed, nothing published. Rerun fetched the remaining 2 in 10 s and three reconciliation pairs matched live counts. A separate spot check: seed total for 2022-09-29 17:00 = 10,744, live half-open count = 10,744.
- Ranking: API top five for the current slot equal a direct `sqlite3` run of `ranking.sql`. Over all 168 slots the weakest top area averages 95.3 pickups and the weakest fifth 33.5.
- Browser (Playwright, Chromium): hold state on live weather; a stalled request reaches the error state in 5.4 s and Refresh recovers; a response with a missing field is rejected with "malformed candidates"; an unreachable weather endpoint gives "No recommendation right now"; no horizontal overflow at 1280 or 390 px and no console errors; `make demo` shows the action state with the simulated-weather banner and no remaining claim that the reading is from Open-Meteo.
- `rm data/control.db && make setup && make run` boots from scratch.

Hardest parts: the portal answers only narrow aggregations and fails intermittently, so acquisition had to be resumable and all-or-nothing; and accepting that the plainer estimator (volume) was right where the cleverer one (ratio) was wrong.

Not verified: behaviour during real precipitation (the action state was exercised only with simulated weather); Open-Meteo's timestamp semantics beyond four observed readings; any browser other than Chromium.

## State pass — 2026-11-13
Tool: Claude Code. The user asked for every implicit state machine in the app to be found and the worthwhile ones made explicit, without breaking anything. The plan was approved before any code changed. Decisions are in `docs/decisions.md`, "State pass".

Errors the AI had introduced earlier, found in this pass:
- **A missing weather reading could be read as dry.** `Decide` blocked only on `invalid`. It was unreachable because the handler passed a separate string, and no test covered it. A test written first failed with `no_action` for a candidate with no reading and for a `valid` one without a value.
- **Two responses the frontend accepted could break the page**: "hold" with no candidates threw in the hero; "action" naming an unknown area showed the label "Action" over "No recommendation". A response without the `simulated` flag was rendered as live.
- **Refresh threw away a valid recommendation.** Seen for real during verification: on the old bundle, a stalled refresh replaced a live action recommendation with the error state.
- **Expiry trusted the browser clock.**
- **`assessed_at` held the hour start**, so validity looked like 28 minutes when the rule is 5.
- **"Expires at the time shown" was sent with unavailable**, where no time is shown.
- **`CLAUDE.md` named a `/verify` command that never existed** and made a file outside the repository required reading.
- **The server sent no cache header for the page.** Found while verifying: after a rebuild the browser still ran the previous bundle (`transferSize: 0` on the navigation), so the first browser check was silently testing old code.

How the design was checked before coding: a read-only planning agent reviewed my inventory and proposal. It confirmed the fail-open path by line, and caught six things I had missed: the `simulated` flag failing open in the parser, `make test` not type-checking, the browser-clock expiry, a live region that would announce a bare time on every refresh, the expiry sentence on unavailable, and `make run` serving a stale build.

Suggestions rejected or changed:
- Mine: a view model of two independent parts (assessment, request). Rejected on the reviewer's point that it allows "nothing shown, nothing in flight"; the refresh state now lives inside `ready`.
- Mine: five reason codes. Cut to three; two only repeated the status.
- Mine: one Hero component for every page state, and a separate state module. Dropped as more code for no behaviour.
- The reviewer's: name the empty string instead of sending `missing`. Not taken; an empty string in the API is the implicit state being removed.
- The reviewer's: a typed lookup table for live and simulated wording. Not taken; the parser fix closes the real risk.
- The reviewer's: commit a baseline first. Not done because the user had said not to commit; a patch and an archive of untracked files were saved outside the repository instead.

How it was validated (actual results):
- `make test`: 10 Python tests, 28 Go tests, `go vet`, lint and type-check pass. `npm run build` passes.
- Mutation check: adding a fourth status to the TypeScript union gave exactly two type errors, the label map and the copy function. Reverted.
- `make setup` twice in a row succeeds. With one row altered, `make run` refuses: "existing DB holds 51744 rows and 5068367 trips, the seed has 51744 and 5068366; rerun `make setup` to rebuild it". `make setup` recovers.
- Steady states unchanged: fed the responses captured before the change, the hold page renders the same text as before (whole page) and so does the action hero and its weather wording.
- Browser (Playwright, Chromium), console clean throughout:
  - a refresh delayed by 2 s keeps the same hero element in the same place, button "Refreshing…";
  - a stalled refresh shows the notice after 5.3 s and keeps the assessment; the next refresh clears it;
  - with the browser clock set 10 minutes slow, the assessment is still current 25 s before its deadline and expired 15 s after it, while that clock is still earlier than `valid_until`; the failed-refresh notice stays;
  - a stalled first load reaches the error state in 5.3 s and recovers;
  - ten malformed responses (hold with no candidates, hold with a missing reading, action on an unknown area, action whose reading is invalid, no `simulated` flag, unavailable without a code, the old response shape, a `valid_until` that is not a time, a reading stamp that is not a time, an HTML body) each end in the error state with a specific message and nothing rendered; an unknown `reason_code` renders normally;
  - `make run ARGS="-weather-url http://127.0.0.1:9/"` gives unavailable / `weather_unavailable`, every reading `missing`, no mention of an expiry;
  - `make demo` gives the action state with the simulated banner;
  - no horizontal overflow at 1280 or 390 px, including with both banners shown;
  - the button keeps keyboard focus through a refresh and a second key press while busy sends nothing;
  - after a rebuild, a plain reload picks up the new bundle.
- **Action on real weather.** During this pass Open-Meteo reported 0.20 mm at the three busiest areas (20:33 CST, 12 November); a direct request to Open-Meteo returned the same value. This is the first time the action state was seen without the simulator. Screenshot: `docs/evidence/screenshots/action-live-weather.png`.
- Two read-only reviewer agents (Go, UI) read the finished change and tried to break it. Neither found a serious defect. Applied from their notes: the Refresh button no longer drops keyboard focus while busy; the failed-refresh notice says "expired" when it is; the action reason reads "at or above the trigger"; the unit string from the upstream response is quoted and capped before it is echoed; a test now asserts the expiry sentence is present when there is an expiry; `weather_invalid` and `no_history` are tested through the handler.

One event not explained: one refresh during the browser pass came back unavailable, straight after a stalled-request test. Eight direct API calls and three repeats of the same sequence all returned the expected action state. The server log was not being captured at that moment. A single slow answer from Open-Meteo (3-second timeout, no retry) fits, but that is not proven.

Not verified: any browser other than Chromium; what a screen reader actually announces (attributes were checked, not listened to; a reviewer noted the first result may not be announced because its live region is created together with its content, which was already true before this pass); the `weather_invalid` and `no_history` states on the page (tested through the handler only).

## Submission pass — 2026-11-13
Tool: Claude Code. The user asked what to do about the items the state pass had skipped, asked for the app to be deployable (Vercel first), and asked for the AI-use section to answer each of the brief's questions with examples. The plan was approved before any code changed. Decisions are in `docs/decisions.md`, "Submission pass".

AI assumptions that turned out wrong in this pass:
- **"Frontend tests need a new dependency."** I had skipped them for that reason. Running the parser under plain Node showed Node 22.19 loads the TypeScript file directly.
- **`node --test test/` runs a folder.** On Node 22 it does not; the first "failing" run failed for that reason, not because of the missing export it was meant to prove. Caught by reading the output; the script now names the files, and a mutation check (one parser rule weakened) made exactly one test fail.
- **My browser checks of the timing, twice.** With a fake clock running far faster than real time, a real 200 ms response looked like a 5 s timeout, and coarse 60 s jumps fired the re-check and the expiry together. Both made the page look broken when it was not. The harness now waits for in-flight requests before advancing the clock.
- **Automatic requests reused the manual-refresh path.** A reviewer agent pointed out that every automatic request switched the page to "pending" or "loading", so during an outage the failure notice would be removed and announced again every minute, and the button would flicker. Automatic requests now change nothing until they finish.
- **A failed start was cached for the life of a hosted instance** (`sync.Once`), and the retry reused an expired context. Found by the Go reviewer agent. Fixed, with a test that fails the start once and then succeeds on the same instance.

Accepted, after checking: the re-check rule (10 s before expiry, 60 s after unavailable or failure, 5 s minimum gap, visible tab only); the parser tests; one Go module at the root; the hosted entry point that reuses `control.Service`; a container image with the database built in.

Rejected or changed: a wording table for live and simulated text (again); a marker on the chosen table row; changing "at the trigger" in the hero copy (it would have invalidated the live-rain screenshots); a `.vercelignore` (a Git import only sees tracked files); a second guard a reviewer suggested after the `await` in `refresh` (it could discard a valid response and leave the page without a next check).

How it was validated (actual results):
- `make test`: 10 Python tests, 31 Go tests, `go vet`, lint, type-check and 20 frontend tests pass. `npm run build` passes. `make setup` from scratch, then `make run`, serves the page and the API.
- Browser, controlled clock, waiting for real requests to finish between steps:
  - 12 minutes in 5 s steps: 3 requests, the page never shows "Expired", the button never leaves "Refresh";
  - a stalled re-check: notice at +5 s, "Expired" at +10 s; during a continued outage the notice stays the same element across three automatic retries;
  - responses with 2 s of validity: 6 requests in 30 s, one per 5 s;
  - tab hidden for 8 minutes: no requests; shown again: one request and recovery;
  - unavailable: no request for 50 s, then one, and recovery without a click;
  - first load failing: the error message stays in place across a retry and the page recovers by itself;
  - manual Refresh still shows "Refreshing…".
- Container: `docker build` succeeds; run with 0.1 CPU, 512 MB and `PORT=10000` it was ready in 0.7 s, served the page and returned a live assessment; 27.6 MB; runs as an unprivileged user.
- Reviewer agents: the UI reviewer could not produce a request burst, a lost re-check, a leaked timer or an expired assessment shown as current; its two UX findings are fixed above. The Go reviewer found the cached start failure; fixed above.

Not verified: **the Vercel build.** It could not be run without the account. It relies on three things Vercel's documentation does not state for its beta Go runtime: Go functions beside a Vite site in one project, imports from elsewhere in the module, and a writable temp directory. Also not verified: Go 1.25 on Vercel; any browser other than Chromium; screen-reader output.

## Deployment — 2026-11-13
Tool: Claude Code. The user imported the repository on Vercel. The dashboard detected the `Dockerfile` and offered a container service; the AI's Go-function setup would not have been built at all once services were configured (the build log said so). The AI read Vercel's documentation on services and the container runtime, replaced `vercel.json` with a single container service, and asked the user to set `PORT=8080`, because Vercel routes to port 80 unless told otherwise. The first deployment succeeded in about two minutes.

Checked from outside (actual results): the page returns 200 with `Cache-Control: no-cache`; `/api/recommendation` returns `no_action` in 0.5 s with a live, non-simulated Open-Meteo reading and `rows_loaded` 51,744; an unknown path returns 404 and a POST returns 405, as locally.

Then removed the unused function route (`api/`, `backend/serverless/`, `data/seed.go`) and pointed the docs at the live URL. Not verified: behaviour after the container sleeps (first request after five idle minutes).

Later the same day the Vercel copy showed "unavailable" on every request while local runs worked. The AI sampled the live API (12 of 12 unavailable, each in about 0.5 s, so an error answer rather than a timeout) and compared a direct Open-Meteo call from the user's machine (200). It suggested the free tier's per-IP limit as the likely cause but did not claim it until the user pasted the Vercel logs: `weather status 429`, including the first request of a fresh instance. The user then deployed the same image on Render; the AI checked it from outside: 8 of 8 requests returned live readings, the page returned 200, an unknown path 404. A server-side weather cache was proposed and set aside, because it cannot fix a limit exhausted by other tenants.

## Implementation start — 2026-11-12
Tool: Codex, using karpathy-engineering and project SQL/Go/UI skills. User requested implementation of the approved plan.
Historical discussion record (recorded now, not backdated): user rejected 28-day history, one-area weather, and strict all-candidate validity; accepted full-year aggregates, top-five evaluation and rank-prefix. The denominator claim was corrected: common denominator does not change ordering. No historical tests are claimed.
Validation so far: public Chicago metadata endpoint returned dataset columns; Open-Meteo two-coordinate response returned ordered results with current precipitation mm and interval 900. Acquisition and application tests remain pending.

## M1 feasibility result — current implementation turn
- Implemented Python standard-library acquisition script using curl, explicit 15000 limit, duplicate/range validation, reconciliation before zero-fill, transactional seed publication after all checks, and a separate monthly attempt. Added bounded-fetch and input-validation unit checks; their results are recorded after execution below.
- Annual attempt: metadata and Sunday/hour assertion succeeded; grouped 2025 query returned HTTP 503. No seed written.
- Explicit monthly attempt: metadata succeeded; known-date query returned HTTP 503 before monthly aggregation. No seed written.
- Saved real Open-Meteo multi-location response as contract evidence. Single-location shape also observed. Full documentation semantics and geographic suitability not yet fully verified.
- Stopped at the user's source-feasibility gate. Rejected continuing with fabricated history, unverified data, or silently relaxed acquisition limits. No app implementation or app test success is claimed.

Validation: make test-acquire passed all three acquisition checks. Saved weather fixture has two entries, mm units and 900-second intervals. Confirmed failed acquisitions published neither seed nor manifest. New implementation files passed a whitespace check. No application tests ran because M1 remains blocked.

## Continued source diagnosis
User supplied the exact assignment links and screenshot identifying m6dm-c72p as the 2018-2022 archive. Following that steering, test calendar 2022 from the assignment-linked source; no claim of current history. A narrow Jan 2, 2022 query returned Sunday=0/hour=12. SQL reviewer identified missing output-directory creation, disabled assertions under python -O, and insufficient HTTP failure diagnostics; accepted these fixes. Prior annual 2025 retry timed out within 60 seconds. This source change is explicit, not a silent fallback.
