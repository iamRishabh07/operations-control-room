# Decisions

## Approval record — 2026-11-12
The user approved implementing the final plan. Prior conversation decisions are summarized here at implementation start, not represented as previously written records.
- Use historical Chicago activity plus live modeled precipitation to prioritize a watch; reject unsupported driver repositioning.
- Use full-year aggregates and five candidates; user rejected the earlier 28-day and single-area proposals.
- Rank-prefix tolerates invalid lower-ranked weather after a winner; strict all-candidate validity was rejected.
- Calendar denominator improves interpretability, not ordering among a fixed weekday/hour.
- Approximate boundary-box centers avoid a geometry dependency; geographic accuracy remains limited.
- Use native Go HTTP, SQLite with modernc.org/sqlite, React/TypeScript/Vite; no production infrastructure.
- Contract-only API smoke assertions; fixed-clock Go tests prove specific outcomes. Expired responses may be returned but UI must never render them as current.

## Source gate result
Stop after both approved acquisition attempts failed with HTTP 503. Preserve the approved source/date/budget and do not create a synthetic production seed. Resume M1 with the existing script when the source is available, or after an explicitly approved source/scope change. No backend or frontend implementation began.
