-- Expected pickup exposure per community area for one weekday-hour slot.
-- Aggregates the matching days per area, compares each area with the citywide
-- total for the slot, and ranks by mean pickups. Volume is the ranking key on
-- purpose: a ratio over small counts would promote areas with a handful of trips.
WITH slot AS (
  SELECT
    area_id,
    COUNT(*)   AS sample_days,
    AVG(trips) AS mean_trips,
    MIN(trips) AS min_trips,
    MAX(trips) AS max_trips
  FROM pickup_hourly
  WHERE hour = :hour
    AND CAST(strftime('%w', service_date) AS INTEGER) = :dow
  GROUP BY area_id
)
SELECT
  s.area_id,
  a.name,
  a.lat,
  a.lon,
  s.sample_days,
  s.mean_trips,
  s.min_trips,
  s.max_trips,
  s.mean_trips / SUM(s.mean_trips) OVER () AS city_share
FROM slot s
JOIN areas a USING (area_id)
WHERE s.mean_trips > 0
ORDER BY s.mean_trips DESC, s.area_id ASC
LIMIT 5;
