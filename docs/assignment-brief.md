# Original take-home requirements: Operations Control Room (Full Stack)

Source: provided three-page Uber final-round take-home exercise. This is a faithful working summary; consult `assignment.pdf` for the original wording.

## Objective
Build a small working web application from public mobility data and ONE live external API. The product tells a marketplace operator the SINGLE most important action to take right now, why, evidence, and what could make it wrong. Historical data may provide baselines; the live API provides current context.

## Scope
The exercise says a polished solution could take 12-20 hours but asks the candidate to scope to approximately 8-10 hours. Thoughtful product decisions and trade-offs matter more than feature count; reasonable heuristics are accepted.

## Technical acceptance criteria
- Backend in Go; standard library or lightweight Go router/framework.
- Simple relational database (2-3 tables), SQLite or PostgreSQL; include schema or migrations.
- Meaningful SQL aggregation/comparison/ranking, NOT entirely in Go.
- A public mobility dataset + one live external API; Chicago Transportation Network Providers trips and Open-Meteo are suggested but not mandatory.
- Minimal usable frontend connected to the Go backend; React + TypeScript recommended.
- 2-3 focused automated tests of important business logic, SQL or external-API failure handling; test volume is not graded.

## Product acceptance criteria
- Exactly ONE current recommendation, not a general dashboard of unrelated insight cards.
- If intervention is unnecessary, explain why and show evidence.
- Include limitations, failure modes and uncertainty in the app and/or README; useful in-product uncertainty may earn consideration.
- Make action understandable and actionable to an operator.
- Creativity is evaluated on usefulness, reasoning, clarity, not raw feature count.
- Extra dashboards, ML, auth and infrastructure are NOT inherently better.

## Submission requirements
- Accessible repository and README with local setup/run steps.
- Schema/migrations, dependencies/config examples, screenshots or brief demo, short architecture overview, database and SQL explanation, tests.
- Document assumptions, trade-offs, known limitations.
- Reviewer must run locally with one or two documented commands; Docker is optional; deployment optional.
- AI-use disclosure: tools used and how, accepted outputs, at least one suggestion rejected/substantially changed and why; validation of generated code, incorrect assumptions or risks, and hardest technical parts.

## Follow-up interview
- Demo working product.
- Explain product decision, architecture, Go, data model and SQL.
- Discuss trade-offs and use of AI.
- Work through a small code change, extension or debugging task, using normal development tools including AI.

## Where each requirement is met

Paths are from the repository root. "README" means a section of `README.md`.

| Requirement | Where |
|---|---|
| Backend in Go | `backend/` (standard library `net/http`); `go.mod` |
| Relational database, 2–3 tables | SQLite, three tables: `backend/internal/control/schema.sql` |
| Schema or migrations included | `schema.sql`, applied by `make setup`; README "Database and SQL" |
| Aggregation, comparison or ranking in SQL | `backend/internal/control/ranking.sql`: `GROUP BY` aggregation, window share, ordered top five; README "Database and SQL" |
| Public mobility dataset | Chicago TNP trips, dataset `m6dm-c72p`: `data/seed.json`, `scripts/acquire.py`; README "Data" |
| One live external API | Open-Meteo: `backend/internal/control/weather.go` |
| Frontend connected to the Go API | React + TypeScript: `frontend/src/` |
| Two or three focused tests | README "Tests" names the three to read first; `make test` runs all |
| One current recommendation only | `frontend/src/components/Hero.tsx`; README "How the recommendation is made" |
| No intervention: explain why, show evidence | The "Hold" state: `Hero.tsx`, `CandidatesBoard.tsx`; screenshot `docs/evidence/screenshots/hold-live-weather.png` |
| What could make it wrong, in the product | The caveat under each recommendation, the "Known limits" list, the unavailable and expired states; README "States" |
| Understandable and actionable for an operator | Headline names the area and the action; the caveat says what to check before intervening |
| README with setup and run | README "Run it" (two commands, or Docker) |
| Schema, dependencies, configuration examples | `schema.sql`, `go.mod`, `frontend/package.json`, `vercel.json`, `Dockerfile`; README "Configuration" |
| Screenshots or demo | `docs/evidence/screenshots/`; README |
| Architecture overview | README "Architecture" |
| Database design and SQL explained | README "Database and SQL" |
| Assumptions, trade-offs, limitations | README "Assumptions, trade-offs and limits"; `docs/decisions.md` |
| AI-use disclosure: tools, accepted, rejected, validation, wrong assumptions, hardest parts | README "AI use", one heading per question; `docs/ai-use-log.md` |
| Runs locally with one or two commands | `make setup`, `make run` |
| Deployment (optional) | https://operations-control-room.onrender.com (mirror on Vercel), both from `Dockerfile`; README "Deploying" |
