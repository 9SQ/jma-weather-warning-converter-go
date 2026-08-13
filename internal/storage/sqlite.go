package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/9SQ/jma-weather-warning-converter-go/internal/domain"

	_ "modernc.org/sqlite"
)

type DB struct {
	sql *sql.DB
}

type FetchedEntry struct {
	EntryID      string
	FeedURL      string
	Title        string
	EntryUpdated string
	Link         string
	XMLPath      string
	XMLSHA256    string
	FetchedAt    string
}

type EntryRecord struct {
	EntryID      string
	FeedURL      string
	Title        string
	EntryUpdated string
	Link         string
	XMLPath      string
}

type Area struct {
	Code string
	Name string
}

type WarningRecord struct {
	AreaCode string
	AreaName string
	KindCode string
}

type FloodRecord struct {
	RiverAreaCode string
	RiverAreaName string
	KindCode      string
}

func Open(path string) (*DB, error) {
	handle, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}
	handle.SetMaxOpenConns(1)
	return &DB{sql: handle}, nil
}

func (db *DB) Close() error {
	return db.sql.Close()
}

func (db *DB) Migrate(ctx context.Context) error {
	if _, err := db.sql.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("migrate sqlite schema: %w", err)
	}
	if err := db.ensureColumn(ctx, "processed_entries", "report_datetime", "TEXT"); err != nil {
		return err
	}
	if _, err := db.sql.ExecContext(ctx, `
CREATE INDEX IF NOT EXISTS idx_processed_entries_report_datetime
  ON processed_entries(report_datetime)
`); err != nil {
		return fmt.Errorf("create report datetime index: %w", err)
	}
	return nil
}

func (db *DB) FeedUpdated(ctx context.Context, feedURL string) (string, bool, error) {
	var updated string
	err := db.sql.QueryRowContext(ctx, `SELECT updated FROM feed_state WHERE feed_url = ?`, feedURL).Scan(&updated)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("select feed state: %w", err)
	}
	return updated, true, nil
}

func (db *DB) UpsertFeedState(ctx context.Context, feedURL, updated, checkedAt string) error {
	_, err := db.sql.ExecContext(ctx, `
INSERT INTO feed_state(feed_url, updated, checked_at)
VALUES(?, ?, ?)
ON CONFLICT(feed_url) DO UPDATE SET
  updated = excluded.updated,
  checked_at = excluded.checked_at
`, feedURL, updated, checkedAt)
	if err != nil {
		return fmt.Errorf("upsert feed state: %w", err)
	}
	return nil
}

func (db *DB) EntryExists(ctx context.Context, entryID string) (bool, error) {
	var exists int
	err := db.sql.QueryRowContext(ctx, `SELECT 1 FROM processed_entries WHERE entry_id = ?`, entryID).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("select processed entry: %w", err)
	}
	return true, nil
}

func (db *DB) SaveFetchedEntry(ctx context.Context, entry FetchedEntry) error {
	_, err := db.sql.ExecContext(ctx, `
INSERT INTO processed_entries(
  entry_id, feed_url, title, entry_updated, link, xml_path, xml_sha256, fetched_at, status
) VALUES(?, ?, ?, ?, ?, ?, ?, ?, 'fetched')
ON CONFLICT(entry_id) DO NOTHING
`, entry.EntryID, entry.FeedURL, entry.Title, entry.EntryUpdated, entry.Link, entry.XMLPath, entry.XMLSHA256, entry.FetchedAt)
	if err != nil {
		return fmt.Errorf("save fetched entry %q: %w", entry.EntryID, err)
	}
	return nil
}

func (db *DB) PendingEntries(ctx context.Context) ([]EntryRecord, error) {
	rows, err := db.sql.QueryContext(ctx, `
SELECT entry_id, feed_url, title, entry_updated, link, xml_path
FROM processed_entries
WHERE status IN ('fetched', 'error') AND imported_at IS NULL
ORDER BY entry_updated ASC, entry_id ASC
`)
	if err != nil {
		return nil, fmt.Errorf("select pending entries: %w", err)
	}
	defer rows.Close()

	var entries []EntryRecord
	for rows.Next() {
		var entry EntryRecord
		if err := rows.Scan(&entry.EntryID, &entry.FeedURL, &entry.Title, &entry.EntryUpdated, &entry.Link, &entry.XMLPath); err != nil {
			return nil, fmt.Errorf("scan pending entry: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending entries: %w", err)
	}
	return entries, nil
}

func (db *DB) ImportedEntriesMissingReportDateTime(ctx context.Context) ([]EntryRecord, error) {
	rows, err := db.sql.QueryContext(ctx, `
SELECT entry_id, feed_url, title, entry_updated, link, xml_path
FROM processed_entries
WHERE status = 'imported' AND imported_at IS NOT NULL AND COALESCE(report_datetime, '') = ''
ORDER BY entry_updated ASC, entry_id ASC
`)
	if err != nil {
		return nil, fmt.Errorf("select imported entries missing report datetime: %w", err)
	}
	defer rows.Close()

	var entries []EntryRecord
	for rows.Next() {
		var entry EntryRecord
		if err := rows.Scan(&entry.EntryID, &entry.FeedURL, &entry.Title, &entry.EntryUpdated, &entry.Link, &entry.XMLPath); err != nil {
			return nil, fmt.Errorf("scan imported entry missing report datetime: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate imported entries missing report datetime: %w", err)
	}
	return entries, nil
}

func (db *DB) MarkImported(ctx context.Context, entryID, importedAt, reportDateTime string) error {
	_, err := db.sql.ExecContext(ctx, `
UPDATE processed_entries
SET status = 'imported', imported_at = ?, report_datetime = ?, last_error = NULL
WHERE entry_id = ?
`, importedAt, reportDateTime, entryID)
	if err != nil {
		return fmt.Errorf("mark imported entry %q: %w", entryID, err)
	}
	return nil
}

func (db *DB) SetEntryReportDateTime(ctx context.Context, entryID, reportDateTime string) error {
	_, err := db.sql.ExecContext(ctx, `
UPDATE processed_entries
SET report_datetime = ?
WHERE entry_id = ?
`, reportDateTime, entryID)
	if err != nil {
		return fmt.Errorf("set report datetime for entry %q: %w", entryID, err)
	}
	return nil
}

func (db *DB) MarkEntryError(ctx context.Context, entryID, message string) error {
	_, err := db.sql.ExecContext(ctx, `
UPDATE processed_entries
SET status = 'error', last_error = ?
WHERE entry_id = ?
`, trimError(message), entryID)
	if err != nil {
		return fmt.Errorf("mark error entry %q: %w", entryID, err)
	}
	return nil
}

func (db *DB) SetEntryLastError(ctx context.Context, entryID, message string) error {
	_, err := db.sql.ExecContext(ctx, `
UPDATE processed_entries
SET last_error = ?
WHERE entry_id = ?
`, trimError(message), entryID)
	if err != nil {
		return fmt.Errorf("set last error for entry %q: %w", entryID, err)
	}
	return nil
}

func (db *DB) ApplyWeatherWarning(ctx context.Context, entryID, category, updatedAt string, areaKinds map[Area][]string) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin weather transaction: %w", err)
	}
	defer tx.Rollback()

	for area, kindCodes := range areaKinds {
		if _, err := tx.ExecContext(ctx, `
DELETE FROM weather_warning
WHERE area_code = ? AND category = ?
`, area.Code, category); err != nil {
			return fmt.Errorf("delete weather warning area %q category %q: %w", area.Code, category, err)
		}

		for _, kindCode := range uniqueNonEmpty(kindCodes) {
			if domain.IsWeatherNoWarningKind(kindCode) {
				continue
			}
			if _, err := tx.ExecContext(ctx, `
INSERT INTO weather_warning(area_code, area_name, category, kind_code, source_entry_id, updated_at)
VALUES(?, ?, ?, ?, ?, ?)
`, area.Code, area.Name, category, kindCode, entryID, updatedAt); err != nil {
				return fmt.Errorf("insert weather warning area %q kind %q: %w", area.Code, kindCode, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit weather transaction: %w", err)
	}
	return nil
}

func (db *DB) ApplyFloodForecast(ctx context.Context, entryID, updatedAt string, riverAreaKinds map[Area][]string) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin flood transaction: %w", err)
	}
	defer tx.Rollback()

	for area, kindCodes := range riverAreaKinds {
		if _, err := tx.ExecContext(ctx, `
DELETE FROM flood_forecast
WHERE river_area_code = ?
`, area.Code); err != nil {
			return fmt.Errorf("delete flood forecast river area %q: %w", area.Code, err)
		}

		for _, kindCode := range uniqueNonEmpty(kindCodes) {
			if _, err := tx.ExecContext(ctx, `
INSERT INTO flood_forecast(river_area_code, river_area_name, kind_code, source_entry_id, updated_at)
VALUES(?, ?, ?, ?, ?)
`, area.Code, area.Name, kindCode, entryID, updatedAt); err != nil {
				return fmt.Errorf("insert flood forecast river area %q kind %q: %w", area.Code, kindCode, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit flood transaction: %w", err)
	}
	return nil
}

func (db *DB) WeatherWarnings(ctx context.Context) ([]WarningRecord, error) {
	rows, err := db.sql.QueryContext(ctx, `
SELECT area_code, area_name, kind_code
FROM weather_warning
WHERE kind_code <> ?
ORDER BY area_code ASC, kind_code ASC
`, domain.WeatherNoWarningKindCode)
	if err != nil {
		return nil, fmt.Errorf("select weather warnings: %w", err)
	}
	defer rows.Close()

	var records []WarningRecord
	for rows.Next() {
		var record WarningRecord
		if err := rows.Scan(&record.AreaCode, &record.AreaName, &record.KindCode); err != nil {
			return nil, fmt.Errorf("scan weather warning: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate weather warnings: %w", err)
	}
	return records, nil
}

func (db *DB) FloodForecasts(ctx context.Context) ([]FloodRecord, error) {
	rows, err := db.sql.QueryContext(ctx, `
SELECT river_area_code, river_area_name, kind_code
FROM flood_forecast
ORDER BY river_area_code ASC, kind_code ASC
`)
	if err != nil {
		return nil, fmt.Errorf("select flood forecasts: %w", err)
	}
	defer rows.Close()

	var records []FloodRecord
	for rows.Next() {
		var record FloodRecord
		if err := rows.Scan(&record.RiverAreaCode, &record.RiverAreaName, &record.KindCode); err != nil {
			return nil, fmt.Errorf("scan flood forecast: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate flood forecasts: %w", err)
	}
	return records, nil
}

func (db *DB) LatestReportDateTime(ctx context.Context) (time.Time, bool, error) {
	rows, err := db.sql.QueryContext(ctx, `
SELECT report_datetime
FROM processed_entries
WHERE status = 'imported' AND COALESCE(report_datetime, '') <> ''
`)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("select report datetimes: %w", err)
	}
	defer rows.Close()

	var latest time.Time
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return time.Time{}, false, fmt.Errorf("scan report datetime: %w", err)
		}
		parsed, err := parseStoredTime(value)
		if err != nil {
			return time.Time{}, false, fmt.Errorf("parse report datetime %q: %w", value, err)
		}
		if latest.IsZero() || parsed.After(latest) {
			latest = parsed
		}
	}
	if err := rows.Err(); err != nil {
		return time.Time{}, false, fmt.Errorf("iterate report datetimes: %w", err)
	}
	if latest.IsZero() {
		return time.Time{}, false, nil
	}
	return latest, true, nil
}

func (db *DB) ensureColumn(ctx context.Context, tableName, columnName, columnType string) error {
	rows, err := db.sql.QueryContext(ctx, `PRAGMA table_info(`+tableName+`)`)
	if err != nil {
		return fmt.Errorf("inspect table %s: %w", tableName, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return fmt.Errorf("scan table info for %s: %w", tableName, err)
		}
		if name == columnName {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate table info for %s: %w", tableName, err)
	}

	if _, err := db.sql.ExecContext(ctx, `ALTER TABLE `+tableName+` ADD COLUMN `+columnName+` `+columnType); err != nil {
		return fmt.Errorf("add column %s.%s: %w", tableName, columnName, err)
	}
	return nil
}

func uniqueNonEmpty(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func trimError(message string) string {
	message = strings.TrimSpace(message)
	if len(message) <= 2000 {
		return message
	}
	return message[:2000]
}

func parseStoredTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, value)
}
