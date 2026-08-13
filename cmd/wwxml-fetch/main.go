package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/9SQ/jma-weather-warning-converter-go/internal/atom"
	"github.com/9SQ/jma-weather-warning-converter-go/internal/domain"
	"github.com/9SQ/jma-weather-warning-converter-go/internal/storage"
)

const maxXMLBytes = 50 << 20

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	dbPath := flag.String("db", "weather_warning.db", "SQLite database path")
	feedURL := flag.String("feed", domain.DefaultFeedURL, "Atom feed URL")
	outDir := flag.String("out", "data/xml", "downloaded XML output directory")
	timeout := flag.Duration("timeout", 20*time.Second, "HTTP timeout")
	xmlRetentionDays := flag.Int("xml-retention", 7, "downloaded XML retention days based only on XML date directory; set 0 to disable cleanup")
	flag.Parse()
	if *xmlRetentionDays < 0 {
		return fmt.Errorf("xml-retention must be greater than or equal to 0")
	}

	ctx := context.Background()
	db, err := storage.Open(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		return err
	}
	deletedXML, err := cleanupOldXML(*outDir, time.Duration(*xmlRetentionDays)*24*time.Hour, time.Now())
	if err != nil {
		return err
	}
	if deletedXML > 0 {
		fmt.Printf("xml cleanup complete: deleted=%d retention_days=%d\n", deletedXML, *xmlRetentionDays)
	}

	client := &http.Client{Timeout: *timeout}
	feedBytes, err := getBytes(client, *feedURL)
	if err != nil {
		return err
	}

	feed, err := atom.Parse(bytes.NewReader(feedBytes))
	if err != nil {
		return err
	}
	feedUpdated, err := feed.UpdatedTime()
	if err != nil {
		return fmt.Errorf("parse feed updated: %w", err)
	}

	if lastUpdated, ok, err := db.FeedUpdated(ctx, *feedURL); err != nil {
		return err
	} else if ok {
		lastUpdatedAt, err := atom.ParseTime(lastUpdated)
		if err != nil {
			return fmt.Errorf("parse stored feed updated: %w", err)
		}
		if !feedUpdated.After(lastUpdatedAt) {
			fmt.Printf("feed unchanged: %s\n", feed.Updated)
			return nil
		}
	}

	entries, err := atom.EligibleEntries(feed, domain.TargetTitles)
	if err != nil {
		return fmt.Errorf("filter feed entries: %w", err)
	}

	now := time.Now().Format(time.RFC3339)
	saved := 0
	skipped := 0
	for _, entry := range entries {
		if entry.ID == "" {
			return fmt.Errorf("entry with title %q has empty id", entry.Title)
		}
		exists, err := db.EntryExists(ctx, entry.ID)
		if err != nil {
			return err
		}
		if exists {
			skipped++
			continue
		}

		link, err := entry.LinkHref(*feedURL)
		if err != nil {
			return err
		}
		xmlBytes, err := getBytes(client, link)
		if err != nil {
			return fmt.Errorf("download entry %q: %w", entry.ID, err)
		}
		xmlPath, xmlHash, err := saveXML(*outDir, entry, xmlBytes)
		if err != nil {
			return err
		}
		if err := db.SaveFetchedEntry(ctx, storage.FetchedEntry{
			EntryID:      entry.ID,
			FeedURL:      *feedURL,
			Title:        entry.Title,
			EntryUpdated: entry.Updated,
			Link:         link,
			XMLPath:      xmlPath,
			XMLSHA256:    xmlHash,
			FetchedAt:    now,
		}); err != nil {
			return err
		}
		saved++
	}

	if err := db.UpsertFeedState(ctx, *feedURL, feed.Updated, now); err != nil {
		return err
	}
	fmt.Printf("fetch complete: saved=%d skipped=%d feed_updated=%s\n", saved, skipped, feed.Updated)
	return nil
}

func getBytes(client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request %s: %w", url, err)
	}
	req.Header.Set("User-Agent", "jma-weather-warning-converter-go/0.1")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("get %s: status %s", url, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxXMLBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", url, err)
	}
	if len(body) > maxXMLBytes {
		return nil, fmt.Errorf("response too large: %s", url)
	}
	return body, nil
}

func saveXML(outDir string, entry atom.Entry, data []byte) (string, string, error) {
	updatedAt, err := entry.UpdatedTime()
	if err != nil {
		return "", "", fmt.Errorf("parse entry updated for %q: %w", entry.ID, err)
	}
	entryIDHash := sha256.Sum256([]byte(entry.ID))
	contentHash := sha256.Sum256(data)
	dir := filepath.Join(outDir, updatedAt.Format("20060102"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", fmt.Errorf("create xml directory: %w", err)
	}
	path := filepath.Join(dir, hex.EncodeToString(entryIDHash[:])+".xml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", "", fmt.Errorf("write xml %s: %w", path, err)
	}
	return path, hex.EncodeToString(contentHash[:]), nil
}

func cleanupOldXML(outDir string, retention time.Duration, now time.Time) (int, error) {
	if retention <= 0 {
		return 0, nil
	}

	info, err := os.Stat(outDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("stat xml output directory %s: %w", outDir, err)
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("xml output path is not a directory: %s", outDir)
	}

	deleted := 0
	var dirs []string
	err = filepath.WalkDir(outDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != outDir {
				dirs = append(dirs, path)
			}
			return nil
		}
		if filepath.Ext(path) != ".xml" {
			return nil
		}
		referenceAt, ok := xmlRetentionReferenceTime(path, now.Location())
		if !ok {
			return nil
		}
		if !now.After(referenceAt.Add(retention)) {
			return nil
		}
		if err := os.Remove(path); err != nil {
			return err
		}
		deleted++
		return nil
	})
	if err != nil {
		return deleted, fmt.Errorf("cleanup old xml in %s: %w", outDir, err)
	}

	sort.Slice(dirs, func(i, j int) bool {
		return len(dirs[i]) > len(dirs[j])
	})
	for _, dir := range dirs {
		if err := os.Remove(dir); err != nil && !os.IsNotExist(err) && !isDirectoryNotEmpty(err) {
			return deleted, fmt.Errorf("remove empty xml directory %s: %w", dir, err)
		}
	}

	return deleted, nil
}

func isDirectoryNotEmpty(err error) bool {
	return errors.Is(err, syscall.ENOTEMPTY) || errors.Is(err, syscall.EEXIST)
}

func xmlRetentionReferenceTime(path string, location *time.Location) (time.Time, bool) {
	dirDate := filepath.Base(filepath.Dir(path))
	parsed, err := time.Parse("20060102", dirDate)
	if err != nil {
		return time.Time{}, false
	}
	if location == nil {
		location = time.Local
	}
	endOfDay := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), location)
	return endOfDay, true
}
