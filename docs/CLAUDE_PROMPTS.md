# Starter prompts for Claude Code — use in order

Use these from the main Claude Code session. Enter plan mode (`Shift+Tab`) for P0–P2 so no files change until approval. Reviewers in `.claude/agents/` can be addressed by `@agent-<name>` or launched with the `Agent` tool.

## P0 — load and inspect WITHOUT coding
Read `CLAUDE.md`, `docs/assignment-brief.md` and `docs/assignment.pdf`. Do not edit files. Build a traceable mandatory-vs-optional requirements table. Identify the 3 biggest ways an otherwise polished submission could fail evaluation. Recommend the smallest technically sound Go + relational DB + React/TypeScript architecture.

## P1 — product decision and alternatives WITHOUT coding
Propose three operational decision heuristics supported by a historical public mobility source and ONE live API. For each, explain actionability, decision rule, baseline SQL, uncertainty, unavailable information, and what would make you display "no action". Rank them for credibility and fit in 8–10 hours. Do not assert historical trips are live demand. Stop for my choice.

## P2 — write approval plan WITHOUT coding
Using my chosen heuristic, write `docs/plan.md` with ≤5 vertical slices, a 2–3 table schema, one real SQL aggregation/ranking query, Go API contract, React states, 2–3 high-value tests and a README/demo checklist. Estimate effort and show scope cuts. Do not implement until I approve.

## P3 — milestone implementation AFTER approval
Leave plan mode. Implement only the first approved milestone from `docs/plan.md`. Show edited files, exact commands run, results, and unresolved risks. Update `docs/decisions.md` and `docs/ai-use-log.md` truthfully. Do not start the next milestone.

## P4 — independent review
Dispatch read-only `product-reviewer`, `go-reviewer`, `sql-reviewer` and `ui-reviewer` subagents in parallel (one message, four `Agent` calls). Wait for all; deduplicate concrete findings with `file:line` and severity. Do not edit files during review.

## P5 — readiness and interview prep
Run the `submission-audit` skill to verify exact clone/setup/run commands, schema/migration, SQL behavior, test results, single-recommendation and no-action states, screenshots, architecture/AI disclosure and secrecy of files. Then run the `interview-defense` skill to quiz me on one realistic change.
