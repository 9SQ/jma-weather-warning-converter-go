package atom

import (
	"strings"
	"testing"

	"github.com/9SQ/jma-weather-warning-converter-go/internal/domain"
)

func TestEligibleEntriesFiltersAndSortsByUpdated(t *testing.T) {
	feed, err := Parse(strings.NewReader(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <updated>2026-07-26T00:10:00+09:00</updated>
  <entry>
    <id>newer</id>
    <title>気象警報・注意報（Ｒ０６）（大雨）</title>
    <updated>2026-07-26T00:05:00+09:00</updated>
    <link href="newer.xml" />
  </entry>
  <entry>
    <id>ignored</id>
    <title>気象警報・注意報（Ｒ０６）（集約通報）</title>
    <updated>2026-07-26T00:01:00+09:00</updated>
    <link href="ignored.xml" />
  </entry>
  <entry>
    <id>older</id>
    <title>指定河川洪水予報</title>
    <updated>2026-07-26T00:03:00+09:00</updated>
    <link href="older.xml" />
  </entry>
</feed>`))
	if err != nil {
		t.Fatal(err)
	}

	entries, err := EligibleEntries(feed, domain.TargetTitles)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[0].ID != "older" || entries[1].ID != "newer" {
		t.Fatalf("entry order = %q, %q; want older, newer", entries[0].ID, entries[1].ID)
	}

	href, err := entries[0].LinkHref("https://www.data.jma.go.jp/developer/xml/feed/extra.xml")
	if err != nil {
		t.Fatal(err)
	}
	if href != "https://www.data.jma.go.jp/developer/xml/feed/older.xml" {
		t.Fatalf("href = %q", href)
	}
}
