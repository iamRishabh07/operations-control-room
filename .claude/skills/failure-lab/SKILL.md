---
name: failure-lab
description: Explore and reproduce edge cases for external API, empty/stale datasets, SQL and UI without speculative fixes. Use when debugging.
---

1. Capture exact reproduction.
2. List 2-3 concrete hypotheses and identify the smallest test.
3. Reproduce a failing test or observable behavior.
4. Apply the smallest fix and rerun checks.
5. Verify error messages and graceful degraded/no-action behavior.
6. Report root cause, fix, tests and residual risks.
