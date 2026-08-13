package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanupOldXMLDeletesOnlyExpiredXML(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	root := t.TempDir()

	oldDir := filepath.Join(root, "20260801")
	newDir := filepath.Join(root, "20260813")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}

	oldXML := filepath.Join(oldDir, "old.xml")
	newXML := filepath.Join(newDir, "new.xml")
	oldText := filepath.Join(oldDir, "old.txt")
	for _, path := range []string{oldXML, newXML, oldText} {
		if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	oldTime := now.Add(-8 * 24 * time.Hour)
	newTime := now.Add(-2 * time.Hour)
	for _, path := range []string{oldXML, oldText} {
		if err := os.Chtimes(path, oldTime, oldTime); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(newXML, newTime, newTime); err != nil {
		t.Fatal(err)
	}

	deleted, err := cleanupOldXML(root, 7*24*time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	if _, err := os.Stat(oldXML); !os.IsNotExist(err) {
		t.Fatalf("old XML still exists or unexpected error: %v", err)
	}
	if _, err := os.Stat(newXML); err != nil {
		t.Fatalf("new XML should remain: %v", err)
	}
	if _, err := os.Stat(oldText); err != nil {
		t.Fatalf("non-XML file should remain: %v", err)
	}
}

func TestCleanupOldXMLUsesDateDirectoryBeforeFileModTime(t *testing.T) {
	location := time.FixedZone("JST", 9*60*60)
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, location)
	root := t.TempDir()

	oldDir := filepath.Join(root, "20260806")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}

	oldXML := filepath.Join(oldDir, "old.xml")
	if err := os.WriteFile(oldXML, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	recentTime := now.Add(-1 * time.Hour)
	if err := os.Chtimes(oldXML, recentTime, recentTime); err != nil {
		t.Fatal(err)
	}

	deleted, err := cleanupOldXML(root, 120*time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	if _, err := os.Stat(oldXML); !os.IsNotExist(err) {
		t.Fatalf("old XML still exists or unexpected error: %v", err)
	}
}

func TestCleanupOldXMLIgnoresFileModTimeWhenDateDirectoryIsInvalid(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	root := t.TempDir()

	invalidDir := filepath.Join(root, "misc")
	if err := os.MkdirAll(invalidDir, 0o755); err != nil {
		t.Fatal(err)
	}

	oldXML := filepath.Join(invalidDir, "old.xml")
	if err := os.WriteFile(oldXML, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := now.Add(-30 * 24 * time.Hour)
	if err := os.Chtimes(oldXML, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	deleted, err := cleanupOldXML(root, 7*24*time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 0 {
		t.Fatalf("deleted = %d, want 0", deleted)
	}
	if _, err := os.Stat(oldXML); err != nil {
		t.Fatalf("XML in invalid date directory should remain: %v", err)
	}
}

func TestCleanupOldXMLCanBeDisabled(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	root := t.TempDir()
	oldXML := filepath.Join(root, "old.xml")
	if err := os.WriteFile(oldXML, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := now.Add(-8 * 24 * time.Hour)
	if err := os.Chtimes(oldXML, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	deleted, err := cleanupOldXML(root, 0, now)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 0 {
		t.Fatalf("deleted = %d, want 0", deleted)
	}
	if _, err := os.Stat(oldXML); err != nil {
		t.Fatalf("old XML should remain when cleanup is disabled: %v", err)
	}
}
