# Implementation plan v4.2 — approved for implementation

Approved in this conversation by the request “Implement the plan.” Implementation status is tracked below; no unrun check is a success.

## Product
One recommendation: put the highest-ranked eligible historical Chicago pickup area on watch for one hour when current modeled precipitation meets an unvalidated threshold. History selects where to look, not live demand. The operator is assumed to have internal wait/cancellation signals. No shortage, causality, supply or validated-benefit claim.

## Data and SQL
Pin Chicago TNP 6dvr-xwnh, calendar 2025 after verification. SoQL groups valid area/weekday/hour with limit 15000. Validate local floating timestamps and Sunday=0 convention against a known date. Reject invalid/duplicate groups and noninteger counts; reconcile independent count before filling all 12936 buckets. Excluded trips are counted separately. Source agreement does not prove reporting completeness.
Use official community boundaries jcxq-k9xf to calculate approximate bounding-box centers; disclose possible geographic mismatch and inspect suitability. Acquisition has a 60-second/10-MB budget per attempt. Annual failure stops; an explicit separate monthly attempt uses the same budget, validates per-month uniqueness, then sums disjoint months. No automatic source substitution.
Commit a JSON seed and manifest; normal startup never calls Chicago. Three tables: areas, pickup_baseline, dataset_imports. Foreign keys, CHECK constraints, schema version, transactional initialization; compare provenance and totals without claiming arbitrary-corruption detection.
SQL joins area metadata, computes a mean using matching calendar-day counts, and orders positive areas by mean DESC, area_id ASC LIMIT 5. The shared denominator changes displayed means, not ordering. DST repeated/absent local hours remain a documented limitation.

## API and decision
GET /api/recommendation returns action/no_action/unavailable with nullable unavailable evidence. Capture assessment time and next Chicago hour boundary once. Open-Meteo receives up to five coordinates; validate object/array shape and count. Weather must have nonnegative finite precipitation in mm over 900 seconds and valid time within -30/+5 minutes.
Rank-prefix: first wet candidate (>=0.2 mm) wins only if every predecessor is valid and dry; invalid predecessor blocks; invalid successors do not. All valid dry candidates -> no_action; no history -> unavailable.
valid_until is min(assessment+5m, captured hour boundary, relevant-prefix weather expiry); unavailable has null expiry. Consumers must check expiry before using guidance. HTTP 200 for expected evidence failures, safe 503 for internal failures. Contexts, 3s client timeout, 5s handler deadline, 1-MiB body cap, server timeouts; no retries/cache/jobs.

## UI
React/strict TypeScript/Vite/plain CSS. One conclusion plus historical and modeled-weather evidence, visible limitations, details methodology. Manual refresh only, bounded single active request. Expiry timer and focus/response checks; expired response never renders an active conclusion. Keyboard access, status announcements and 375px layout. One-hour review duration differs from <=5-minute assessment validity.

## Milestones and gates
1. Source feasibility (2h): make acquire; validate aggregation, reconciliation, geography, real weather payload. STOP if source contracts fail.
2. Connected slice (2h): make setup; make run; node scripts/assert-api.mjs in second terminal.
3. Live decisions (2h): cd backend && go test ./... -count=1; successful real-weather smoke check separate from contract validation.
4. UI (1.5h): cd frontend && npm run lint && npm run build; controlled states, expiry and mobile browser walkthrough.
5. Submission (1.5h): clean setup/run; Go tests/vet, frontend lint/build, git diff --check; screenshots, README, truthful decisions and AI disclosure.
One hour contingency: cut polish, redundant screenshots, then redundant test variants, never distinct correctness invariants.

## Tests and delivery
Three focused Go test groups: SQL/import integrity including EXPLAIN; fixed-clock recommendation/rank-prefix/expiry; HTTP/payload failures. Assertion script checks response invariants, not a predetermined live status. UI controlled responses are labeled simulated.
Use Go net/http/database/sql/modernc.org/sqlite and React/TypeScript/Vite/plain CSS. Pinned dependencies. make setup installs/builds/initializes without replacing incompatible existing DB; make run serves on :8080. No publication without separate authorization.
No auth, maps, ML, Docker, caching, automatic refresh, geometry library, fallback adapters, action tracking, seed checksum or multi-import management.

## Current status
- Approval received. M1 BLOCKED: annual acquisition returned HTTP 503 at the grouped query. The explicit monthly attempt returned HTTP 503 at the known-date prerequisite before any monthly aggregation.
- Metadata verification and one annual-attempt known-date assertion passed. Single/multiple-coordinate weather requests succeeded; a real multi-location payload is saved in docs/evidence/open-meteo-multiple.json. This is historical contract evidence, never runtime fallback weather.
- No seed was published. Representative points, reconciliation, schema, backend, frontend and all downstream milestones remain unimplemented/unverified.
- make acquire and make acquire-monthly are available to retry the same approved source. No alternate dataset or relaxed budget was used.
- setup/run targets remain planned interfaces, deliberately not exposed as working commands before implementation.
