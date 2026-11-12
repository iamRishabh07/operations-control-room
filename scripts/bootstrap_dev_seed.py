#!/usr/bin/env python3
"""Bootstrap a labelled development seed.

The Chicago TNP data portal's grouped-query path repeatedly timed out during
submission preparation (see docs/decisions.md). This script produces a seed
that lets the full application be demonstrated end-to-end:

- `areas`: real Chicago community areas pulled from the official boundaries
  endpoint (jcxq-k9xf). Representative point = bbox centre. If the boundaries
  endpoint is also unavailable, the committed fixture at
  data/areas_fallback.json is used; both paths produce the same 77-area shape.
- `buckets`: deterministic weekday/hour pickup profile derived from publicly
  documented Chicago TNP characteristics (weekday commute peaks, weekend
  night-life shift, downtown-heavy distribution). NOT real 2022 trip counts.
  Clearly labelled `synthetic=true` in the manifest and surfaced in the UI
  banner.
- `history.dataset_id` = `dev-baseline-v1`, period = the documented reference
  window the pattern was shaped around.

The real acquisition path remains `make acquire` (scripts/acquire.py). Run it
once the Chicago data portal recovers to replace this seed with real 2022
aggregates.

Deterministic: running twice produces identical seed and manifest bytes.
"""
from __future__ import annotations

import datetime as dt
import hashlib
import json
import math
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DATA = ROOT / "data"
BOUNDARIES_URL = "https://data.cityofchicago.org/resource/igwz-8jzy.json"
FALLBACK_AREAS = ROOT / "scripts" / "areas_fallback.json"
REFERENCE_PERIOD = ("2022-01-01", "2022-12-31")

# Community-area weights — approximate relative pickup share taken from
# published Chicago TNP descriptive summaries (Loop, Near North and Near West
# dominate; outlying residential areas tail off). These are coarse heuristics,
# not measured values. Any area not named here uses the tail weight.
AREA_WEIGHTS: dict[int, float] = {
    32: 10.0,  # Loop
    8: 8.0,    # Near North Side
    28: 6.0,   # Near West Side
    7: 4.0,    # Lincoln Park
    24: 3.5,   # West Town
    6: 3.0,    # Lake View
    33: 3.0,   # Near South Side
    76: 5.0,   # O'Hare (airport pickups)
    56: 2.0,   # Garfield Ridge (Midway-adjacent)
    22: 2.5,   # Logan Square
    5: 2.0,    # North Center
}
TAIL_WEIGHT = 0.6

# Hourly shape: normalized relative multipliers by (dow, hour).
# Weekday morning commute (06–09), evening commute (16–19), weekend late-night
# (Fri/Sat 21–02) are boosted; 03–05 overnight dip.
def hour_multiplier(dow: int, hour: int) -> float:
    weekday = dow in (1, 2, 3, 4, 5)
    base = 1.0
    if weekday:
        if 6 <= hour <= 9:
            base = 2.3
        elif 16 <= hour <= 19:
            base = 2.6
        elif hour in (10, 11, 12, 13, 14, 15):
            base = 1.4
        elif hour in (20, 21, 22):
            base = 1.2
        elif hour in (0, 23):
            base = 0.6
        else:
            base = 0.25  # 1-5am
    else:
        if dow == 6 or dow == 5:  # Fri/Sat night roll-over handled via dow 5/6
            if hour in (21, 22, 23) or hour in (0, 1, 2):
                base = 2.4
            elif 10 <= hour <= 18:
                base = 1.5
            elif 3 <= hour <= 7:
                base = 0.3
            else:
                base = 1.0
        else:  # Sunday dow=0
            if 11 <= hour <= 18:
                base = 1.4
            elif 0 <= hour <= 4:
                base = 0.5
            elif hour in (19, 20, 21, 22, 23):
                base = 1.0
            else:
                base = 0.7
    # Smooth tiny perturbation keyed on (dow, hour) so areas don't tie everywhere.
    return base


def fetch_areas() -> list[dict]:
    """Fetch real community areas via curl, fall back to committed fixture."""
    try:
        out = subprocess.run(
            [
                "curl",
                "--fail-with-body",
                "--silent",
                "--show-error",
                "--max-time",
                "20",
                BOUNDARIES_URL + "?$select=area_num_1,community,the_geom&$limit=100",
            ],
            capture_output=True,
            timeout=25,
            check=True,
        )
        rows = json.loads(out.stdout)
    except (subprocess.CalledProcessError, subprocess.TimeoutExpired, json.JSONDecodeError) as exc:
        print(f"boundaries fetch failed ({exc}); using fallback fixture", file=sys.stderr)
        rows = json.loads(FALLBACK_AREAS.read_text())
    return rows


def flatten(coords):
    if isinstance(coords, list) and len(coords) >= 2 and all(isinstance(x, (int, float)) for x in coords[:2]):
        yield coords[:2]
    elif isinstance(coords, list):
        for child in coords:
            yield from flatten(child)


def area_from_row(row: dict) -> dict:
    area_id = int(row["area_num_1"])
    name = row["community"].title()
    pts = list(flatten(row["the_geom"]["coordinates"]))
    if not pts or not all(math.isfinite(x) for p in pts for x in p):
        raise ValueError(f"invalid geometry for area {area_id}")
    lon = (min(p[0] for p in pts) + max(p[0] for p in pts)) / 2
    lat = (min(p[1] for p in pts) + max(p[1] for p in pts)) / 2
    if not (41.6 < lat < 42.1 and -88.0 < lon < -87.4):
        raise ValueError(f"area {area_id} centre outside Chicago bounds: ({lat}, {lon})")
    return {
        "area_id": area_id,
        "name": name,
        "lat": round(lat, 6),
        "lon": round(lon, 6),
        "coord_source": f"{BOUNDARIES_URL}; bounding-box center; approximate",
    }


def build_seed() -> dict:
    rows = fetch_areas()
    areas = sorted((area_from_row(r) for r in rows), key=lambda a: a["area_id"])
    if [a["area_id"] for a in areas] != list(range(1, 78)):
        raise ValueError("expected exactly 77 Chicago community areas, 1..77")

    # Reference-year day counts per dow (2022-01-01 is Saturday → dow=6).
    period_start = dt.date.fromisoformat(REFERENCE_PERIOD[0])
    period_end = dt.date.fromisoformat(REFERENCE_PERIOD[1])
    dow_days = [0] * 7
    d = period_start
    while d <= period_end:
        dow_days[d.weekday() + 1 if d.weekday() < 6 else 0] += 1  # python Mon=0, our Sun=0
        d += dt.timedelta(days=1)

    # Base daily volume shared across all buckets of a given weekday.
    # 100k Loop-equivalent pickups per weekday distributed over hours/areas
    # produces totals in the 5-figure range per bucket, a reasonable look.
    DAILY_VOLUME = 100_000

    buckets = []
    total_sum = 0
    for a in areas:
        w = AREA_WEIGHTS.get(a["area_id"], TAIL_WEIGHT)
        for dow in range(7):
            n_days = dow_days[dow]
            for hour in range(24):
                mult = hour_multiplier(dow, hour)
                # Area share * hour share * day count; normalize hour share to keep
                # weekday row sum roughly comparable to DAILY_VOLUME.
                share = w / 100.0
                hourly = DAILY_VOLUME * share * mult / 24.0
                trips = int(round(hourly * n_days))
                buckets.append({"area_id": a["area_id"], "dow": dow, "hour": hour, "total_trips": trips})
                total_sum += trips

    query_hash = hashlib.sha256(json.dumps(buckets, sort_keys=True).encode()).hexdigest()[:16]
    history = {
        "dataset_id": "dev-baseline-v1",
        "period_start": REFERENCE_PERIOD[0],
        "period_end": REFERENCE_PERIOD[1],
        "retrieved_at": "1970-01-01T00:00:00+00:00",  # deterministic — not a real time
        "query_text": f"synthetic weekday/hour profile; hash={query_hash}",
        "rows_loaded": len(buckets),
        "trips_included": total_sum,
        "trips_excluded_invalid_area": 0,
        "synthetic": True,
        "synthetic_notice": (
            "Deterministic development baseline. Chicago TNP portal grouped-query path "
            "was unresponsive at submission preparation; see docs/decisions.md. "
            "Replace by running `make acquire` once the portal recovers."
        ),
    }
    return {"areas": areas, "buckets": buckets, "history": history}


def main() -> int:
    seed = build_seed()
    DATA.mkdir(parents=True, exist_ok=True)
    temp = DATA / "seed.json.tmp"
    temp.write_text(json.dumps(seed, separators=(",", ":"), sort_keys=True) + "\n")
    temp.replace(DATA / "seed.json")
    manifest = dict(seed["history"])
    (DATA / "manifest.json").write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n")
    hist = seed["history"]
    print(
        f"wrote {len(seed['buckets'])} buckets across {len(seed['areas'])} areas; "
        f"synthetic total trips={hist['trips_included']}"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
