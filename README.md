# Operations Control Room

**Implementation incomplete: source-feasibility gate blocked.** This repository currently contains the approved plan and bounded acquisition tooling, not a working web application.

## Implemented
- Approved architecture and milestones: [docs/plan.md](docs/plan.md).
- Historical acquisition: 2025 Chicago TNP dataset `6dvr-xwnh`, grouped by area/weekday/hour, with an explicit limit, independent count reconciliation, zero-fill validation and official boundary-derived representative points.
- A 60-second / 10-MB budget per acquisition attempt; output is published only after every check succeeds.
- Real multi-location Open-Meteo payload captured as historical contract evidence, not a live fallback.

## Current blocker
The annual attempt failed with HTTP 503 at the aggregate query. The separately invoked monthly attempt failed with HTTP 503 at the known-date prerequisite. Neither produced a seed. The approved plan requires stopping here; no synthetic data was substituted.

## Commands available now
Requires Python 3 and curl. From the repository root:

```sh
make test-acquire
make acquire
```

The explicit alternative attempt is `make acquire-monthly` with the same total budget. It does not run automatically. Successful acquisition writes `data/seed.json` and `data/manifest.json`; source reporting completeness remains unknown even after reconciliation.

`make setup` and `make run` remain planned and are not implemented. Go/SQLite API, React UI, live decision logic, application tests and screenshots remain pending.

## Provenance and AI use
See [decision log](docs/decisions.md) and [AI-use log](docs/ai-use-log.md). Codex produced the approved plan and acquisition tooling; the source failures were observed, not simulated. Known uncertainty includes source revisions, geographic approximation, historical representativeness and unvalidated precipitation thresholds.

Validation performed: `make test-acquire` passed three focused checks of numeric input, geometry coordinate traversal, and bounded HTTP failure behavior. Failed acquisitions left no seed or manifest. These are acquisition checks, not the planned application test suite.
