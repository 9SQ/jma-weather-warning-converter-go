package storage

import (
	"context"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestApplyWeatherWarningOverwritesByAreaAndCategory(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	area := Area{Code: "1111111", Name: "区域A"}
	if err := db.ApplyWeatherWarning(ctx, "entry-1", "大雨", "2026-07-26T00:00:00+09:00", map[Area][]string{
		area: {"03", "03", "29"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.ApplyWeatherWarning(ctx, "entry-2", "大雨", "2026-07-26T00:01:00+09:00", map[Area][]string{
		area: {"00"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.ApplyWeatherWarning(ctx, "entry-3", "その他注意報", "2026-07-26T00:02:00+09:00", map[Area][]string{
		area: {"14"},
	}); err != nil {
		t.Fatal(err)
	}

	records, err := db.WeatherWarnings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(records))
	for _, record := range records {
		got = append(got, record.KindCode)
	}
	sort.Strings(got)

	want := []string{"14"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("kind codes = %#v, want %#v", got, want)
	}
}

func TestWeatherWarningsSkipsLegacyNoWarningCode(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	_, err = db.sql.ExecContext(ctx, `
INSERT INTO weather_warning(area_code, area_name, category, kind_code, source_entry_id, updated_at)
VALUES
  ('1111111', '区域A', '大雨', '00', 'entry-1', '2026-07-26T00:00:00+09:00'),
  ('1111111', '区域A', 'その他注意報', '14', 'entry-2', '2026-07-26T00:01:00+09:00')
`)
	if err != nil {
		t.Fatal(err)
	}

	records, err := db.WeatherWarnings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].KindCode != "14" {
		t.Fatalf("records = %#v, want one 14 record", records)
	}
}

func TestApplyFloodForecastOverwritesByRiverArea(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	area := Area{Code: "river-1", Name: "河川A"}
	if err := db.ApplyFloodForecast(ctx, "entry-1", "2026-07-26T00:00:00+09:00", map[Area][]string{
		area: {"20", "30"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.ApplyFloodForecast(ctx, "entry-2", "2026-07-26T00:01:00+09:00", map[Area][]string{
		area: {"10"},
	}); err != nil {
		t.Fatal(err)
	}

	records, err := db.FloodForecasts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].KindCode != "10" {
		t.Fatalf("records = %#v, want one 10 record", records)
	}
}

func TestLatestReportDateTime(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	if err := db.SaveFetchedEntry(ctx, FetchedEntry{
		EntryID:      "entry-1",
		FeedURL:      "feed",
		Title:        "指定河川洪水予報",
		EntryUpdated: "2026-07-26T00:01:00+09:00",
		Link:         "https://example.test/1.xml",
		XMLPath:      "1.xml",
		FetchedAt:    "2026-07-26T00:01:00+09:00",
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveFetchedEntry(ctx, FetchedEntry{
		EntryID:      "entry-2",
		FeedURL:      "feed",
		Title:        "指定河川洪水予報",
		EntryUpdated: "2026-07-26T00:02:00+09:00",
		Link:         "https://example.test/2.xml",
		XMLPath:      "2.xml",
		FetchedAt:    "2026-07-26T00:02:00+09:00",
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.MarkImported(ctx, "entry-1", "2026-07-26T00:03:00+09:00", "2026-07-26T00:10:00+09:00"); err != nil {
		t.Fatal(err)
	}
	if err := db.MarkImported(ctx, "entry-2", "2026-07-26T00:04:00+09:00", "2026-07-26T00:20:00+09:00"); err != nil {
		t.Fatal(err)
	}

	latest, ok, err := db.LatestReportDateTime(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("LatestReportDateTime ok = false")
	}
	want := time.Date(2026, 7, 26, 0, 20, 0, 0, time.FixedZone("", 9*60*60))
	if !latest.Equal(want) {
		t.Fatalf("latest = %s, want %s", latest.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}
