---
name: sql-proof
description: Design or review meaningful SQL aggregation, rankings, migrations and fixture tests for a small SQLite/Postgres application. Use when working on ranking SQL.
---

1. Confirm raw vs derived data, grouping granularity, timezone and provenance.
2. Keep a 2-3 table relational schema unless evidence demands more.
3. Put at least one meaningful grouping/aggregation/ranking in SQL.
4. Parameterize external input and test nulls, ties, empty data and timestamp buckets.
5. Run fixture-based SQL checks and inspect query plan when practical.
6. Record what the query measures and does NOT prove.
