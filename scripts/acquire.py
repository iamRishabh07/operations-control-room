#!/usr/bin/env python3
"""Bounded, explicit acquisition of the approved 2025 published aggregates."""
import argparse
import datetime as dt
import json
import math
from pathlib import Path
import subprocess
import time
import urllib.parse

ROOT = Path(__file__).resolve().parents[1]
BASE = "https://data.cityofchicago.org/resource/6dvr-xwnh.json"
BOUNDARIES = "https://data.cityofchicago.org/resource/jcxq-k9xf.json"
PERIOD = "trip_start_timestamp >= '2025-01-01T00:00:00' AND trip_start_timestamp < '2026-01-01T00:00:00'"
VALID = "pickup_community_area between 1 and 77"
SELECT = "pickup_community_area as area_id,date_extract_dow(trip_start_timestamp) as dow,date_extract_hh(trip_start_timestamp) as hour,count(*) as trips"
GROUP = "pickup_community_area,date_extract_dow(trip_start_timestamp),date_extract_hh(trip_start_timestamp)"

def integer(value):
    if not isinstance(value, (str, int)) or isinstance(value, bool):
        raise ValueError("noninteger count/code")
    result = int(value)
    if str(result) != str(value) or result < 0 or result > 9007199254740991:
        raise ValueError("invalid integer")
    return result

class Fetch:
    def __init__(self):
        self.start = time.monotonic()
        self.size = 0
        self.queries = []
    def __call__(self, url, params=None):
        if params:
            url += "?" + urllib.parse.urlencode(params)
        remaining = 60 - (time.monotonic() - self.start)
        if remaining <= 0:
            raise RuntimeError("60-second acquisition budget exhausted")
        self.queries.append(url)
        print("Fetching", url, flush=True)
        result = subprocess.run(["curl", "--fail", "--silent", "--show-error", "--max-time", str(remaining), "--max-filesize", str(10_000_000-self.size), url], capture_output=True, timeout=remaining+1)
        if result.returncode:
            raise RuntimeError(result.stderr.decode().strip())
        self.size += len(result.stdout)
        if self.size > 10_000_000:
            raise RuntimeError("10 MB acquisition budget exceeded")
        return json.loads(result.stdout)

def coordinates(value):
    if isinstance(value, list) and len(value) >= 2 and all(isinstance(x, (float,int)) for x in value[:2]):
        yield value[:2]
    elif isinstance(value, list):
        for child in value:
            yield from coordinates(child)

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--monthly", action="store_true", help="Separate explicit monthly attempt, same total budget")
    args = parser.parse_args()
    get = Fetch()
    meta = get("https://data.cityofchicago.org/api/views/6dvr-xwnh/columns.json")
    columns = {c["fieldName"]: c for c in meta}
    assert columns["trip_start_timestamp"]["dataTypeName"] == "calendar_date", "timestamp semantics changed"
    assert columns["pickup_community_area"]["dataTypeName"] == "number", "area schema changed"
    known = get(BASE, {"$select":"date_extract_dow('2025-01-05T12:00:00'::floating_timestamp) as dow,date_extract_hh('2025-01-05T12:00:00'::floating_timestamp) as hour", "$limit":1})
    assert integer(known[0]["dow"]) == 0 and integer(known[0]["hour"]) == 12, "weekday/hour extraction mismatch"
    periods = [PERIOD]
    if args.monthly:
        periods = []
        for month in range(1,13):
            start = dt.date(2025,month,1)
            end = dt.date(2025,month+1,1) if month < 12 else dt.date(2026,1,1)
            periods.append(f"trip_start_timestamp >= '{start}T00:00:00' AND trip_start_timestamp < '{end}T00:00:00'")
    totals = {}
    for period in periods:
        rows = get(BASE, {"$select": SELECT, "$where":period+" AND "+VALID, "$group":GROUP, "$order":"area_id,dow,hour", "$limit":15000})
        if not isinstance(rows,list) or len(rows)>12936:
            raise ValueError("invalid aggregate size")
        seen = set()
        for row in rows:
            key = tuple(integer(row[k]) for k in ("area_id","dow","hour"))
            if key in seen or not (1<=key[0]<=77 and 0<=key[1]<=6 and 0<=key[2]<=23):
                raise ValueError("invalid or duplicate bucket")
            seen.add(key)
            totals[key] = totals.get(key,0)+integer(row["trips"])
    included = integer(get(BASE,{"$select":"count(*) as n", "$where":PERIOD+" AND "+VALID})[0]["n"])
    excluded = integer(get(BASE,{"$select":"count(*) as n", "$where":PERIOD+" AND (pickup_community_area IS NULL OR pickup_community_area < 1 OR pickup_community_area > 77)"})[0]["n"])
    if sum(totals.values()) != included or included == 0:
        raise ValueError("aggregate/count reconciliation failed or empty source")
    areas = []
    for row in get(BOUNDARIES,{"$select":"area_num_1,community,the_geom", "$limit":100}):
        area_id = integer(row["area_num_1"])
        pts = list(coordinates(row["the_geom"]["coordinates"]))
        if not pts or not all(math.isfinite(x) for p in pts for x in p):
            raise ValueError("invalid geometry")
        lon = (min(p[0] for p in pts)+max(p[0] for p in pts))/2
        lat = (min(p[1] for p in pts)+max(p[1] for p in pts))/2
        if not (41.6 < lat < 42.1 and -88.0 < lon < -87.4):
            raise ValueError("point outside Chicago bounds")
        areas.append(dict(area_id=area_id,name=row["community"].title(),lat=lat,lon=lon,coord_source=BOUNDARIES+"; bounding-box center; approximate"))
    areas.sort(key=lambda a:a["area_id"])
    assert [a["area_id"] for a in areas] == list(range(1,78)), "area coverage mismatch"
    buckets = [dict(area_id=a,dow=d,hour=h,total_trips=totals.get((a,d,h),0)) for a in range(1,78) for d in range(7) for h in range(24)]
    manifest = dict(dataset_id="6dvr-xwnh",period_start="2025-01-01",period_end="2025-12-31",retrieved_at=dt.datetime.now(dt.timezone.utc).isoformat(),query_text=json.dumps(get.queries, separators=(",",":")),rows_loaded=len(buckets),trips_included=included,trips_excluded_invalid_area=excluded)
    seed = dict(areas=areas,buckets=buckets,history=manifest)
    # Publish only after every source and integrity check succeeds.
    temp = ROOT / "data/seed.json.tmp"
    temp.write_text(json.dumps(seed,separators=(",",":"))+"\n")
    temp.replace(ROOT / "data/seed.json")
    (ROOT / "data/manifest.json").write_text(json.dumps(manifest,indent=2)+"\n")
    for area in areas:
        print(f'{area["area_id"]:2} {area["name"]:24} {area["lat"]:.6f} {area["lon"]:.6f}')
    print(f"PASS: {len(buckets)} buckets; {included} included; {excluded} excluded; {get.size} bytes; {time.monotonic()-get.start:.1f}s")

if __name__ == "__main__":
    main()
