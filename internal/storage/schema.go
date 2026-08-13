package storage

const schemaSQL = `
CREATE TABLE IF NOT EXISTS feed_state (
  feed_url TEXT PRIMARY KEY,
  updated TEXT NOT NULL,
  checked_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS processed_entries (
  entry_id TEXT PRIMARY KEY,
  feed_url TEXT NOT NULL,
  title TEXT NOT NULL,
  entry_updated TEXT NOT NULL,
  link TEXT NOT NULL,
  xml_path TEXT NOT NULL,
  xml_sha256 TEXT,
  report_datetime TEXT,
  fetched_at TEXT NOT NULL,
  imported_at TEXT,
  status TEXT NOT NULL,
  last_error TEXT
);

CREATE TABLE IF NOT EXISTS weather_warning (
  area_code TEXT NOT NULL,
  area_name TEXT NOT NULL,
  category TEXT NOT NULL,
  kind_code TEXT NOT NULL,
  source_entry_id TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (area_code, category, kind_code)
);

CREATE TABLE IF NOT EXISTS flood_forecast (
  river_area_code TEXT NOT NULL,
  river_area_name TEXT NOT NULL,
  kind_code TEXT NOT NULL,
  source_entry_id TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (river_area_code, kind_code)
);

CREATE INDEX IF NOT EXISTS idx_processed_entries_status_updated
  ON processed_entries(status, entry_updated);

CREATE INDEX IF NOT EXISTS idx_weather_warning_area
  ON weather_warning(area_code);

CREATE INDEX IF NOT EXISTS idx_flood_forecast_river_area
  ON flood_forecast(river_area_code);
`
