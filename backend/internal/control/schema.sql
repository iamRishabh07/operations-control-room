CREATE TABLE IF NOT EXISTS areas (
  area_id INTEGER PRIMARY KEY CHECK (area_id BETWEEN 1 AND 77),
  name TEXT NOT NULL CHECK (length(name) > 0),
  lat REAL NOT NULL CHECK (lat BETWEEN 41.6 AND 42.1),
  lon REAL NOT NULL CHECK (lon BETWEEN -88.0 AND -87.4),
  coord_source TEXT NOT NULL
);

-- One observation per community area, calendar day and clock hour (Chicago
-- local time). The weekday is derived from service_date in the ranking query.
CREATE TABLE IF NOT EXISTS pickup_hourly (
  area_id INTEGER NOT NULL REFERENCES areas(area_id),
  service_date TEXT NOT NULL CHECK (service_date IS date(service_date)),
  hour INTEGER NOT NULL CHECK (hour BETWEEN 0 AND 23),
  trips INTEGER NOT NULL CHECK (typeof(trips) = 'integer' AND trips >= 0),
  PRIMARY KEY (area_id, service_date, hour)
);

CREATE INDEX IF NOT EXISTS pickup_hourly_hour ON pickup_hourly (hour);

CREATE TABLE IF NOT EXISTS dataset_imports (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  dataset_id TEXT NOT NULL,
  period_start TEXT NOT NULL,
  period_end TEXT NOT NULL,
  retrieved_at TEXT NOT NULL,
  query_text TEXT NOT NULL,
  rows_loaded INTEGER NOT NULL CHECK (rows_loaded >= 0),
  trips_included INTEGER NOT NULL CHECK (trips_included >= 0),
  trips_excluded_invalid_area INTEGER NOT NULL CHECK (trips_excluded_invalid_area >= 0),
  imported_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
