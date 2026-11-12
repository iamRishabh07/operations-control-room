# Operations Control Room - Codex project instructions

## Required reading before coding
- Read `docs/assignment-brief.md` and, when present, `docs/assignment.pdf`.
- This is an Uber final-round full-stack take-home. Scope for roughly 8-10 hours of implementation work, not a production platform.
- Treat `docs/assignment-brief.md` as a summary, but the source PDF is authoritative if any conflict arises.

## Required product behavior
- Show exactly ONE current operational recommendation; if no intervention is justified, show a useful no-action conclusion and supporting evidence.
- Explain action, location/time, why now, observed/baseline evidence, source timestamps, limitations, and uncertainty.
- Do not represent historical Chicago ride counts as live demand, or infer supply/driver availability without data.
- Never present weather as proof of causality. Prefer conditional language and explicit heuristic assumptions.

## Technical constraints
- Go backend, relational SQLite or PostgreSQL with 2-3 core tables and repeatable migrations/schema.
- At least one meaningful SQL aggregation, comparison or ranking; it must be demonstrated in code and explained in the README.
- Public mobility data + ONE live external API; keep source provenance and refresh times visible.
- Minimal connected frontend; React + TypeScript is preferred.
- Include 2-3 focused tests: meaningful SQL ranking, recommendation logic, and/or API failure behavior.
- One or two documented local run commands; keep setup simple. No default auth, Kafka, Kubernetes, ML or excessive infrastructure.

## Operating method
- First: produce a requirement-to-test matrix and a small architectural options table. STOP before implementation.
- Second: write `docs/plan.md` with milestones, source acquisition limits, SQL ranking and no-action scenario. STOP for human approval.
- On approval: implement one vertical slice at a time, with an executable verification command after each slice.
- Ask before adding libraries, changing architecture, accessing external authenticated services, or publishing to GitHub.
- Save notable accepted and rejected AI suggestions in `docs/ai-use-log.md` as they happen. Do not invent entries later.
- Keep `docs/decisions.md` and `README.md` accurate; reconcile with actual implementation.

## Build checks (fill in after implementation)
- Backend: `go test ./...`; `go vet ./...`; `gofmt -w` only on files you change.
- Frontend: use `npm run lint`, `npm run build`, and any tests defined in package.json.
- Database: migrate from empty local DB; verify representative SQL with fixtures and EXPLAIN QUERY PLAN when using SQLite.
- UI: inspect one recommendation state, no-action state, loading, external API error, and a small viewport.

## Code Review Rules
- Flag false claims of live demand, data leakage, missing SQL aggregation, unbounded data ingestion, missing timeouts, unsafe SQL concatenation, unstated assumptions, and confusing uncertainty.
- Flag broken setup commands, missing migration steps, untracked output artifacts, secrets and generated datasets too large for Git.
- Report findings with file paths, severity, reproducible steps, and targeted fixes. Do not flag stylistic preferences as defects.


## Engineering discipline

For nontrivial code changes:

1. THINK BEFORE CODING
   Surface assumptions, ambiguity, and meaningful
   tradeoffs. Do not silently guess.

2. SIMPLICITY FIRST
   Implement the smallest correct solution.
   Avoid speculative features and abstractions.

3. SURGICAL CHANGES
   Modify only what the task requires.
   Preserve unrelated code and behavior.

4. GOAL-DRIVEN EXECUTION
   Define verifiable success criteria.
   Run relevant checks and review the diff.

Use the $karpathy-engineering skill for
the full workflow.

Keep trivial changes lightweight.


## Orchestration protocol

The main Codex agent is the project orchestrator.

Before nontrivial implementation:
1. Read docs/assignment.pdf as the authoritative brief.
2. Read docs/assignment-brief.md and
   docs/references/Codex_Setup_Playbook_2026.md.
3. Read docs/decisions.md and docs/plan.md.
4. Check whether the referenced skills and agents exist.
5. Identify assumptions, risks and acceptance criteria.
6. Delegate bounded analysis to specialist subagents
   where it adds value.
7. Wait for the agents and reconcile their findings.
8. Produce one implementation plan and stop for approval.

Specialist responsibilities:
- product_reviewer: requirements, decision usefulness,
  product scope and evaluation criteria.
- sql_reviewer: data provenance, schema, SQL ranking,
  data quality and deterministic fixtures.
- go_reviewer: API design, error handling, external
  integrations, concurrency and testability.
- ui_reviewer: one-action UX, accessibility, responsive
  behavior and loading/error/empty states.

Subagent rules:
- Give each agent the assignment context and exact task.
- Delegate independent analysis in parallel.
- Avoid overlapping source-file edits.
- Require evidence and explicit uncertainty.
- The manager resolves conflicts and owns integration.
- Do not invent test results or completed work.

Before coding, use the karpathy-engineering principles:
think before coding, simplicity first, surgical changes,
and goal-driven verification.

Do not implement until the user approves the plan.
Do not modify unrelated files.
Maintain an accurate AI-use log.

<claude-mem-context>
# Memory Context

# [operations-control-room] recent context, 2026-11-12 5:48pm GMT+5:30

No previous sessions found.
</claude-mem-context>