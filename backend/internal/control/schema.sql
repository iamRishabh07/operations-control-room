CREATE TABLE IF NOT EXISTS areas (
  area_id INTEGER PRIMARY KEY CHECK (area_id BETWEEN 1 AND 77),
  name TEXT NOT NULL CHECK (length(name) > 0),
  lat REAL NOT NULL CHECK (lat BETWEEN 41.6 AND 42.1),
  lon REAL NOT NULL CHECK (lon BETWEEN -88.0 AND -87.4),
  coord_source TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS pickup_baseline (
  area_id INTEGER NOT NULL REFERENCES areas(area_id),
  dow INTEGER NOT NULL CHECK (dow BETWEEN 0 AND 6),
  hour INTEGER NOT NULL CHECK (hour BETWEEN 0 AND 23),
  total_trips INTEGER NOT NULL CHECK (typeof(total_trips) = 'integer' AND total_trips >= 0),
  PRIMARY KEY (area_id, dow, hour)
);

CREATE INDEX IF NOT EXISTS pickup_baseline_time ON pickup_baseline (dow, hour, area_id);

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
