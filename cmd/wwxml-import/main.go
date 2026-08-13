package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/9SQ/jma-weather-warning-converter-go/internal/domain"
	"github.com/9SQ/jma-weather-warning-converter-go/internal/importer"
	"github.com/9SQ/jma-weather-warning-converter-go/internal/jmaxml"
	"github.com/9SQ/jma-weather-warning-converter-go/internal/storage"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	dbPath := flag.String("db", "weather_warning.db", "SQLite database path")
	flag.Parse()

	ctx := context.Background()
	db, err := storage.Open(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		return err
	}

	entries, err := db.PendingEntries(ctx)
	if err != nil {
		return err
	}

	imported := 0
	backfilled := 0
	failed := 0
	for _, entry := range entries {
		if err := importEntry(ctx, db, entry); err != nil {
			failed++
			if markErr := db.MarkEntryError(ctx, entry.EntryID, err.Error()); markErr != nil {
				return markErr
			}
			fmt.Fprintf(os.Stderr, "import failed: entry_id=%s error=%v\n", entry.EntryID, err)
			continue
		}
		imported++
	}
	if failed == 0 {
		backfillEntries, err := db.ImportedEntriesMissingReportDateTime(ctx)
		if err != nil {
			return err
		}
		for _, entry := range backfillEntries {
			reportDateTime, err := readReportDateTime(entry.XMLPath)
			if err != nil {
				failed++
				if markErr := db.SetEntryLastError(ctx, entry.EntryID, err.Error()); markErr != nil {
					return markErr
				}
				fmt.Fprintf(os.Stderr, "backfill failed: entry_id=%s error=%v\n", entry.EntryID, err)
				continue
			}
			if err := db.SetEntryReportDateTime(ctx, entry.EntryID, reportDateTime); err != nil {
				return err
			}
			backfilled++
		}
	}

	if failed > 0 {
		return fmt.Errorf("import complete with errors: imported=%d backfilled=%d failed=%d", imported, backfilled, failed)
	}
	fmt.Printf("import complete: imported=%d backfilled=%d failed=%d\n", imported, backfilled, failed)
	return nil
}

func importEntry(ctx context.Context, db *storage.DB, entry storage.EntryRecord) error {
	file, err := os.Open(entry.XMLPath)
	if err != nil {
		return fmt.Errorf("open xml %s: %w", entry.XMLPath, err)
	}
	defer file.Close()

	report, err := jmaxml.Parse(file)
	if err != nil {
		return err
	}
	if report.ReportDateTime == "" {
		return fmt.Errorf("missing ReportDateTime in %s", entry.XMLPath)
	}

	if category, ok := domain.CategoryForTitle(entry.Title); ok {
		items := report.HeadlineItems(domain.WeatherInfoType)
		areaKinds := importer.AggregateHeadlineItems(items)
		if err := db.ApplyWeatherWarning(ctx, entry.EntryID, category, report.ReportDateTime, areaKinds); err != nil {
			return err
		}
		return db.MarkImported(ctx, entry.EntryID, time.Now().Format(time.RFC3339), report.ReportDateTime)
	}

	if domain.IsFloodTitle(entry.Title) {
		items := report.HeadlineItems(domain.FloodInfoType)
		riverAreaKinds := importer.AggregateHeadlineItems(items)
		if err := db.ApplyFloodForecast(ctx, entry.EntryID, report.ReportDateTime, riverAreaKinds); err != nil {
			return err
		}
		return db.MarkImported(ctx, entry.EntryID, time.Now().Format(time.RFC3339), report.ReportDateTime)
	}

	return fmt.Errorf("unsupported title %q", entry.Title)
}

func readReportDateTime(xmlPath string) (string, error) {
	file, err := os.Open(xmlPath)
	if err != nil {
		return "", fmt.Errorf("open xml %s: %w", xmlPath, err)
	}
	defer file.Close()

	report, err := jmaxml.Parse(file)
	if err != nil {
		return "", err
	}
	if report.ReportDateTime == "" {
		return "", fmt.Errorf("missing ReportDateTime in %s", xmlPath)
	}
	return report.ReportDateTime, nil
}
