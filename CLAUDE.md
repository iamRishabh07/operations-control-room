# Operations Control Room — Claude Code project instructions

## Required reading before coding
- Read `docs/assignment-brief.md` and, when present, `docs/assignment.pdf`.
- Uber final-round full-stack take-home. Scope ~8–10 hours of implementation, not a production platform.
- `docs/assignment-brief.md` is a summary; the source PDF is authoritative if any conflict arises.
- Reference playbook for this setup: `docs/references/The Claude Code Setup Playbook.pdf` (local only, not in the repository; skip it when absent).

## Required product behavior
- Show exactly ONE current operational recommendation; if no intervention is justified, show a useful no-action conclusion with supporting evidence.
- Explain action, location/time, why now, observed/baseline evidence, source timestamps, limitations, and uncertainty.
- Do NOT represent historical Chicago ride counts as live demand, or infer supply/driver availability without data.
- Never present weather as proof of causality. Prefer conditional language and explicit heuristic assumptions.

## Technical constraints
- Go backend, relational SQLite or PostgreSQL with 2–3 core tables and repeatable migrations/schema.
- At least one meaningful SQL aggregation, comparison or ranking; demonstrated in code and explained in README.
- Public mobility data + ONE live external API; keep source provenance and refresh times visible.
- Minimal connected frontend; React + TypeScript preferred.
- 2–3 focused tests: meaningful SQL ranking, recommendation logic, and/or API failure behavior.
- One or two documented local run commands; keep setup simple. No default auth, Kafka, Kubernetes, ML or excess infra.

## Operating method
The first two steps apply to new features and architecture changes. For a small change or a bug fix: read the code, make the change, run `make test`.
- First: produce a requirement-to-test matrix and a small architectural options table. STOP before implementation.
- Second: write `docs/plan.md` with milestones, source-acquisition limits, SQL ranking and no-action scenario. STOP for human approval.
- On approval: implement one vertical slice at a time, with an executable verification command after each slice.
- Ask before adding libraries, changing architecture, accessing external authenticated services, or publishing to GitHub.
- Save notable accepted and rejected AI suggestions in `docs/ai-use-log.md` as they happen. Do not invent entries later.
- Keep `docs/decisions.md` and `README.md` accurate; reconcile with the actual implementation.

## Build checks
- Backend: from the repo root, `go test ./api/... ./backend/... ./data/...` and `go vet` on the same; `gofmt -w` only on files you change. Never `./...` from the root: it also matches a Go package inside `frontend/node_modules`.
- Frontend: `npm run lint`, `npm run typecheck`, `npm test` (Node's built-in runner, Node 22.18+, no test library), `npm run build`.
- All of the above except the build: `make test`.
- Database: migrate from empty local DB; verify representative SQL with fixtures and `EXPLAIN QUERY PLAN` on SQLite.
- UI: inspect one recommendation state, no-action state, loading, external API error, and a small viewport.

## Code review rules
- Flag false claims of live demand, data leakage, missing SQL aggregation, unbounded data ingestion, missing timeouts, unsafe SQL concatenation, unstated assumptions, and confusing uncertainty.
- Flag broken setup commands, missing migration steps, untracked output artifacts, secrets and generated datasets too large for Git.
- Report findings with file paths, severity, reproducible steps, and targeted fixes. Do not flag stylistic preferences as defects.

## Engineering discipline
For nontrivial code changes:
1. **Think before coding** — surface assumptions, ambiguity, and meaningful tradeoffs. Do not silently guess.
2. **Simplicity first** — implement the smallest correct solution. Avoid speculative features and abstractions.
3. **Surgical changes** — modify only what the task requires. Preserve unrelated code and behavior.
4. **Goal-driven execution** — define verifiable success criteria. Run relevant checks and review the diff.

Trivial changes stay lightweight.

## Orchestration protocol
The main Claude Code session is the project orchestrator. This protocol is for new features and architecture changes, not for a small change or a bug fix. Before nontrivial implementation:
1. Read `docs/assignment.pdf` as the authoritative brief (and `docs/assignment-brief.md`).
2. Read `docs/references/The Claude Code Setup Playbook.pdf` if it is present (local only).
3. Read `docs/decisions.md` and `docs/plan.md`.
4. Confirm the referenced subagents and skills exist under `.claude/agents/` and `.claude/skills/`.
5. Identify assumptions, risks and acceptance criteria.
6. Delegate bounded analysis to specialist subagents (see below) via `@agent-<name>` or the `Agent` tool.
7. Wait for the agents and reconcile findings.
8. Produce one implementation plan and STOP for approval.

Specialist subagents (in `.claude/agents/`):
- `product-reviewer` — requirements, decision usefulness, product scope, evaluation criteria.
- `sql-reviewer` — data provenance, schema, SQL ranking, data quality, deterministic fixtures.
- `go-reviewer` — API design, error handling, external integrations, concurrency, testability.
- `ui-reviewer` — one-action UX, accessibility, responsive behavior, loading/error/empty states.

Subagent rules:
- Give each agent the assignment context and its exact task — subagents start with no conversation history.
- Delegate independent analysis in parallel (one message, multiple `Agent` calls).
- Avoid overlapping source-file edits.
- Require evidence and explicit uncertainty.
- The main session resolves conflicts and owns integration.
- Do not invent test results or completed work.

Do not implement until the user approves the plan. Do not modify unrelated files. Maintain an accurate AI-use log.

## Workflow conventions
- Use plan mode (`Shift+Tab`) for exploration before edits.
- Prefer the `code-review-graph` MCP tools before Grep/Glob/Read (see `~/CLAUDE.md`).
- Run `make test` and `cd frontend && npm run build` before claiming a slice complete.
- Conventional commits; one logical change per commit.

## Gotchas
- `data/seed.json` is committed real data. `make acquire` rebuilds it with one small query per (date, hour): 30 s per request, 4 tries, 8 workers, resumable cache under `data/raw/` keyed by the query text. It publishes only a complete, reconciled extract and exits 1 otherwise. This replaced the earlier 60 s / 10 MB single-query budget, which the portal could not meet (see `docs/decisions.md`).
- Hourly windows are half-open (`>= start AND < end`). An inclusive `between` double-counts every trip stamped HH:00, because the publisher rounds to 15 minutes.
- After the seed changes, rerun `make setup`: it always rebuilds `data/control.db`. Startup refuses a DB that does not match the seed.
- `make run` serves the built `frontend/dist`. After a `.tsx` change run `npm run build`, or use `npm run dev` (port 5173, `/api` proxied to 8080).
- Only a `valid` weather reading may be read as wet or dry. `Decide` returns unavailable for `invalid` and `missing`; keep it that way.
- State sets are defined once per side. Backend: status, reason-code and evidence-state constants in `backend/internal/control/types.go`. Frontend: the `Assessment` union in `frontend/src/api.ts`, whose parser enforces what each status guarantees, and `ViewState` in `frontend/src/App.tsx` for the page.
- Adding a status touches, in order: the constant, `Decide`, the `Assessment` union and `parseAssessment`, `LABELS` and `explain` in `Hero.tsx` (type-checking fails until both are done), a `[data-status]` colour in `styles.css`, a case in `frontend/test/api.test.mjs`, and the States table in `README.md`. A new `reason_code` needs only the constant, `Decide` and the README table.
- Live and simulated wording sits in four components and `handler.go`. When changing either, run `grep -rn simulated frontend/src backend/internal` and keep both sides in step.
- Rank areas by mean pickups, not by a ratio. Ratios over small counts put areas with a handful of trips on top.
- `make demo` points the weather adapter at `scripts/sim_weather.py`. Any weather source other than Open-Meteo is labelled simulated in the API and the UI; never present those readings as live.
- Production is the `Dockerfile` image on Vercel's container runtime (`vercel.json`, project env `PORT=8080`); every push to `main` redeploys. `backend/cmd/server` is the only entry point: keep it thin and put behaviour in `backend/internal/control`.
- The page re-checks by itself (`RECHECK_*` constants in `frontend/src/App.tsx`). Automatic requests pass `background = true` and must not change what is on screen until they finish.
- When testing page timing with a fake clock, let in-flight requests finish before advancing it; otherwise a real 200 ms response looks like a 5 s timeout.
