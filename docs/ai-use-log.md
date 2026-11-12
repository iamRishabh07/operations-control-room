# AI-use log

## Toolchain switch — 2026-11-12
Switched project AI harness from Codex to Claude Code. Ported `AGENTS.md` → `CLAUDE.md`, `.codex/agents/*.toml` → `.claude/agents/*.md`, `.codex/hooks/` → `.claude/hooks/`, moved `.agents/skills/` → `.claude/skills/`, and rewrote `docs/CODEX_PROMPTS.md` → `docs/CLAUDE_PROMPTS.md`. Project rules, reviewer responsibilities and the pre-tool-use guard behave the same; only the harness changed.

## Implementation turn — 2026-11-13
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
