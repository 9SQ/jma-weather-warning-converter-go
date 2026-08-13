package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/9SQ/jma-weather-warning-converter-go/internal/exportjson"
	"github.com/9SQ/jma-weather-warning-converter-go/internal/storage"
	"github.com/9SQ/jma-weather-warning-converter-go/internal/tables"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	dbPath := flag.String("db", "weather_warning.db", "SQLite database path")
	riverTablePath := flag.String("river-table", "table/river_to_cities.json", "river area to cities mapping JSON")
	cityTablePath := flag.String("city-table", "table/city_to_lv2areas.json", "city to secondary area mapping JSON")
	outPath := flag.String("out", "weather_warning.json", "output JSON path")
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

	weather, err := db.WeatherWarnings(ctx)
	if err != nil {
		return err
	}
	floods, err := db.FloodForecasts(ctx)
	if err != nil {
		return err
	}
	mappings, err := tables.Load(*riverTablePath, *cityTablePath)
	if err != nil {
		return err
	}

	latestReportDateTime, ok, err := db.LatestReportDateTime(ctx)
	if err != nil {
		return err
	}
	if !ok {
		if len(weather) > 0 || len(floods) > 0 {
			return fmt.Errorf("ReportDateTime is missing; run wwxml-import before export")
		}
		latestReportDateTime = time.Now()
	}

	output := exportjson.Build(weather, floods, mappings, latestReportDateTime, time.Now())
	if err := exportjson.WriteFile(*outPath, output); err != nil {
		return err
	}
	fmt.Printf("export complete: areas=%d out=%s\n", len(output.Areas), *outPath)
	return nil
}
