#!/usr/bin/env python3
"""Acquire the historical pickup baseline from the Chicago TNP trips archive.

One small SoQL aggregation per (date, hour) against dataset m6dm-c72p, because
the portal times out on wider grouped queries. Windows are half-open
[HH:00, HH+1:00): the publisher rounds timestamps to 15 minutes, so an
inclusive `between` would count every HH:00 trip in two hours.

Responses are cached under data/raw/, in a folder named after the query text,
so a rerun resumes where the last one stopped and a changed query never reuses
old answers. Nothing is published until every window is present and a sample
of adjacent hours reconciles against an independent live count.
"""
from __future__ import annotations

import datetime as dt
import hashlib
import json
import math
import subprocess
import sys
import time
import urllib.parse
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SEED = ROOT / "data" / "seed.json"
MANIFEST = ROOT / "data" / "manifest.json"

DATASET_ID = "m6dm-c72p"
BASE = f"https://data.cityofchicago.org/resource/{DATASET_ID}.json"
BOUNDARIES = "https://data.cityofchicago.org/resource/igwz-8jzy.json"

# Four consecutive weeks with no US federal holiday (Labor Day is 5 Sep,
# Columbus Day 10 Oct). The Chicago Marathon falls on the last Sunday.
PERIOD_START = dt.date(2022, 9, 12)  # a Monday
WEEKS = 4

HOUR_QUERY = (
    "SELECT pickup_community_area AS area_id, count(*) AS trips "
    "WHERE trip_start_timestamp >= '{start}' AND trip_start_timestamp < '{end}' "
    "GROUP BY pickup_community_area LIMIT 200"
)
COUNT_QUERY = (
    "SELECT count(*) AS trips "
    "WHERE trip_start_timestamp >= '{start}' AND trip_start_timestamp < '{end}'"
)

# A cached answer is only valid for the query that produced it.
CACHE = ROOT / "data" / "raw" / ("tnp-hourly-" + hashlib.sha256(HOUR_QUERY.encode()).hexdigest()[:8])

REQUEST_TIMEOUT = 30  # seconds per HTTP request
REQUEST_TRIES = 4  # the portal is intermittently slow
WORKERS = 8
MAX_FAILED = 24  # stop early when the portal is clearly down
# Window indexes whose pair (i, i+1) is checked against one live two-hour
# count: a morning pair, a pair across midnight, and the final pair.
RECONCILE_PAIRS = (8, 23, -2)

NAME_FIXES = {"Ohare": "O'Hare", "Mckinley Park": "McKinley Park"}


class SourceUnavailable(RuntimeError):
    """The portal did not return usable data within the retry budget."""


def integer(value) -> int:
    """Parse a canonical non-negative integer; the portal sends counts as strings.

    Values above 2**53 - 1 are rejected.
    """
    if not isinstance(value, (str, int)) or isinstance(value, bool):
        raise ValueError("noninteger count/code")
    result = int(value)
    if str(result) != str(value) or result < 0 or result > 9007199254740991:
        raise ValueError("invalid integer")
    return result


def coordinates(value):
    """Yield every [lon, lat] pair in a GeoJSON coordinate tree."""
    if isinstance(value, list) and len(value) >= 2 and all(isinstance(x, (int, float)) for x in value[:2]):
        yield value[:2]
    elif isinstance(value, list):
        for child in value:
            yield from coordinates(child)


def windows(start: dt.date, weeks: int) -> list[tuple[str, int, str, str]]:
    """Half-open hourly windows (date, hour, start, end) covering whole weeks."""
    out = []
    for day in range(weeks * 7):
        date = start + dt.timedelta(days=day)
        for hour in range(24):
            begin = dt.datetime.combine(date, dt.time(hour))
            end = begin + dt.timedelta(hours=1)
            out.append((date.isoformat(), hour, begin.isoformat(), end.isoformat()))
    return out


def parse_hour(rows) -> tuple[dict[int, int], int]:
    """Split one hourly response into {area_id: trips} and trips with no area."""
    if not isinstance(rows, list):
        raise ValueError("expected a list of groups")
    per_area: dict[int, int] = {}
    excluded = None
    for row in rows:
        trips = integer(row["trips"])
        if row.get("area_id") is None:  # pickups outside Chicago or suppressed
            if excluded is not None:
                raise ValueError("duplicate no-area group")
            excluded = trips
            continue
        area_id = integer(row["area_id"])
        if not 1 <= area_id <= 77:
            raise ValueError(f"area id out of range: {area_id}")
        if area_id in per_area:
            raise ValueError(f"duplicate area group: {area_id}")
        per_area[area_id] = trips
    return per_area, excluded or 0


def fetch(url: str, params: dict, tries: int = REQUEST_TRIES, pause=time.sleep):
    """GET JSON through curl with bounded retries."""
    full = url + ("?" + urllib.parse.urlencode(params) if params else "")
    command = ["curl", "--fail-with-body", "--silent", "--show-error", "--max-time", str(REQUEST_TIMEOUT), full]
    problem = ""
    for attempt in range(tries):
        try:
            result = subprocess.run(command, capture_output=True, timeout=REQUEST_TIMEOUT + 5)
        except subprocess.TimeoutExpired:
            problem = "curl did not exit"
        else:
            if result.returncode == 0:
                try:
                    return json.loads(result.stdout)
                except json.JSONDecodeError as exc:
                    problem = f"malformed JSON: {exc}"
            else:
                problem = result.stderr.decode(errors="replace").strip() or f"curl exit {result.returncode}"
        if attempt + 1 < tries:
            pause(1 + attempt)
    raise SourceUnavailable(problem)


def cached(name: str, load):
    """Return the cached response for name, fetching and storing it if absent."""
    path = CACHE / f"{name}.json"
    if path.exists():
        return json.loads(path.read_text())
    data = load()
    CACHE.mkdir(parents=True, exist_ok=True)
    partial = path.with_suffix(".partial")
    partial.write_text(json.dumps(data))
    partial.replace(path)
    return data


def areas_from(rows) -> list[dict]:
    """Community areas with a bounding-box centre as the weather lookup point."""
    areas = []
    for row in rows:
        area_id = integer(row["area_num_1"])
        points = list(coordinates(row["the_geom"]["coordinates"]))
        if not points or not all(math.isfinite(x) for point in points for x in point):
            raise ValueError(f"invalid geometry for area {area_id}")
        lon = (min(p[0] for p in points) + max(p[0] for p in points)) / 2
        lat = (min(p[1] for p in points) + max(p[1] for p in points)) / 2
        if not (41.6 < lat < 42.1 and -88.0 < lon < -87.4):
            raise ValueError(f"area {area_id} centre outside Chicago bounds")
        name = row["community"].title()
        areas.append(
            {
                "area_id": area_id,
                "name": NAME_FIXES.get(name, name),
                "lat": round(lat, 6),
                "lon": round(lon, 6),
                "coord_source": BOUNDARIES + "; bounding-box centre; approximate",
            }
        )
    areas.sort(key=lambda area: area["area_id"])
    if [area["area_id"] for area in areas] != list(range(1, 78)):
        raise ValueError("expected community areas 1..77")
    return areas


def load_hour(window):
    date, hour, start, end = window

    def get():
        rows = fetch(BASE, {"$query": HOUR_QUERY.format(start=start, end=end)})
        per_area, _ = parse_hour(rows)  # never cache a response that does not validate
        if not per_area:
            # Chicago has pickups in every hour; an empty answer is a bad
            # response, and caching it would publish an all-zero hour.
            raise SourceUnavailable("response had no area groups")
        return rows

    return cached(f"{date}T{hour:02d}", get)


def reconcile(wins, hours) -> list[str]:
    """Check adjacent hourly totals against one independent two-hour count."""
    problems = []
    for index in RECONCILE_PAIRS:
        first, second = wins[index], wins[index + 1]
        expected = 0
        for window in (first, second):
            per_area, no_area = parse_hour(hours[window[:2]])
            expected += sum(per_area.values()) + no_area
        rows = fetch(BASE, {"$query": COUNT_QUERY.format(start=first[2], end=second[3])})
        live = integer(rows[0]["trips"])
        if live != expected:
            problems.append(f"{first[2]}..{second[3]}: hourly sum {expected} != live count {live}")
    return problems


def build_seed(areas, wins, hours, retrieved_at: str) -> dict:
    """Dense seed: one row per (date, hour); trips[i] belongs to area i + 1."""
    rows, included, excluded = [], 0, 0
    for date, hour, _, _ in wins:
        per_area, no_area = parse_hour(hours[(date, hour)])
        counts = [per_area.get(area_id, 0) for area_id in range(1, 78)]
        rows.append({"date": date, "hour": hour, "trips": counts})
        included += sum(counts)
        excluded += no_area
    history = {
        "dataset_id": DATASET_ID,
        "period_start": wins[0][0],
        "period_end": wins[-1][0],
        "retrieved_at": retrieved_at,
        "query_text": HOUR_QUERY,
        "rows_loaded": len(rows) * 77,
        "trips_included": included,
        "trips_excluded_invalid_area": excluded,
    }
    return {"areas": areas, "hours": rows, "history": history}


def main() -> int:
    wins = windows(PERIOD_START, WEEKS)
    started = time.monotonic()
    try:
        areas = areas_from(
            cached("boundaries", lambda: fetch(BOUNDARIES, {"$select": "area_num_1,community,the_geom", "$limit": 100}))
        )
    except SourceUnavailable as exc:
        print(f"boundaries unavailable ({exc}); nothing published, rerun to retry")
        return 1

    hours, failed = {}, 0
    print(f"{len(wins)} hourly windows, {WORKERS} workers, cache {CACHE}", flush=True)
    with ThreadPoolExecutor(max_workers=WORKERS) as pool:
        futures = {pool.submit(load_hour, window): window for window in wins}
        for done, future in enumerate(as_completed(futures), start=1):
            window = futures[future]
            try:
                hours[window[:2]] = future.result()
            except SourceUnavailable as exc:
                failed += 1
                print(f"  failed {window[2]}: {str(exc).splitlines()[0] if str(exc) else 'no detail'}", flush=True)
                if failed > MAX_FAILED:
                    pool.shutdown(wait=False, cancel_futures=True)
                    break
            if done % 48 == 0:
                print(f"  {done}/{len(wins)} ({time.monotonic() - started:.0f}s, {failed} failed)", flush=True)

    if len(hours) < len(wins):
        print(f"{len(hours)}/{len(wins)} windows cached; nothing published. Rerun `make acquire` to resume.")
        return 1

    try:
        problems = reconcile(wins, hours)
    except SourceUnavailable as exc:
        print(f"reconciliation could not run ({exc}); nothing published. Rerun `make acquire`.")
        return 1
    if problems:
        print("reconciliation failed; nothing published:")
        for problem in problems:
            print("  " + problem)
        return 1

    seed = build_seed(areas, wins, hours, dt.datetime.now(dt.timezone.utc).isoformat(timespec="seconds"))
    manifest = dict(seed["history"], source_endpoint=BASE, windows=len(wins), reconciled_pairs=len(RECONCILE_PAIRS))
    SEED.parent.mkdir(parents=True, exist_ok=True)
    partial = SEED.with_suffix(".partial")
    partial.write_text(json.dumps(seed, separators=(",", ":")) + "\n")
    partial.replace(SEED)
    MANIFEST.write_text(json.dumps(manifest, indent=2) + "\n")
    history = seed["history"]
    print(
        f"PASS: {history['period_start']}..{history['period_end']}, {history['rows_loaded']} rows, "
        f"{history['trips_included']} trips, {history['trips_excluded_invalid_area']} without an area, "
        f"{time.monotonic() - started:.0f}s"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
