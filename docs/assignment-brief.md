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
