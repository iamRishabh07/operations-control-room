WITH matching_days AS (
  SELECT COUNT(*) AS n_days
  FROM (
    WITH RECURSIVE days(d) AS (
      SELECT :period_start
      UNION ALL SELECT date(d, '+1 day') FROM days WHERE d < :period_end
    )
    SELECT d FROM days WHERE CAST(strftime('%w', d) AS INTEGER) = :dow
  )
)
SELECT
  b.area_id,
  a.name,
  a.lat,
  a.lon,
  b.total_trips,
  (SELECT n_days FROM matching_days) AS sample_days,
  b.total_trips * 1.0 / (SELECT n_days FROM matching_days) AS mean_pickups_per_day
FROM pickup_baseline b
JOIN areas a USING (area_id)
WHERE b.dow = :dow
  AND b.hour = :hour
  AND b.total_trips > 0
  AND (SELECT n_days FROM matching_days) > 0
ORDER BY b.total_trips DESC, b.area_id ASC
LIMIT 5;
