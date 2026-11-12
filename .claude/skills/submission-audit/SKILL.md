---
name: submission-audit
description: Verify the Uber submission end-to-end without changing architecture or inventing results. Use immediately before sharing a repository.
---

1. Read the take-home requirements and compare every acceptance criterion to actual source/config/docs.
2. From a clean checkout, verify exact documented setup and run commands, migrations, tests, lint and frontend build.
3. Exercise recommendation/no-action/API-failure UI states and capture screenshot/demo evidence.
4. Inspect git status and tracked files for secrets, giant datasets, caches and generated artifacts.
5. Check README architecture, SQL explanation, assumptions, AI-use disclosure, and follow-up demo script.
6. Return PASS/FAIL/UNVERIFIED with commands/results, not wishful green checks.
