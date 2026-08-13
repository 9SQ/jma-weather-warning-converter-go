package atom

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
	"time"
)

type Feed struct {
	ID      string  `xml:"id"`
	Title   string  `xml:"title"`
	Updated string  `xml:"updated"`
	Entries []Entry `xml:"entry"`
}

type Entry struct {
	ID      string `xml:"id"`
	Title   string `xml:"title"`
	Updated string `xml:"updated"`
	Links   []Link `xml:"link"`
}

type Link struct {
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
	Href string `xml:"href,attr"`
}

func Parse(r io.Reader) (*Feed, error) {
	var feed Feed
	decoder := xml.NewDecoder(r)
	if err := decoder.Decode(&feed); err != nil {
		return nil, fmt.Errorf("parse atom feed: %w", err)
	}
	return &feed, nil
}

func ParseTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unsupported time format %q", value)
}

func (f *Feed) UpdatedTime() (time.Time, error) {
	return ParseTime(f.Updated)
}

func (e Entry) UpdatedTime() (time.Time, error) {
	return ParseTime(e.Updated)
}

func (e Entry) LinkHref(baseURL string) (string, error) {
	for _, link := range e.Links {
		if strings.TrimSpace(link.Href) == "" {
			continue
		}
		href := strings.TrimSpace(link.Href)
		parsedHref, err := url.Parse(href)
		if err != nil {
			return "", fmt.Errorf("parse entry link %q: %w", href, err)
		}
		if parsedHref.IsAbs() {
			return parsedHref.String(), nil
		}
		base, err := url.Parse(baseURL)
		if err != nil {
			return "", fmt.Errorf("parse feed url %q: %w", baseURL, err)
		}
		return base.ResolveReference(parsedHref).String(), nil
	}
	return "", fmt.Errorf("entry %q has no usable link", e.ID)
}

func EligibleEntries(feed *Feed, targetTitles map[string]struct{}) ([]Entry, error) {
	entries := make([]Entry, 0, len(feed.Entries))
	for _, entry := range feed.Entries {
		title := strings.TrimSpace(entry.Title)
		if _, ok := targetTitles[title]; !ok {
			continue
		}
		entry.Title = title
		entry.ID = strings.TrimSpace(entry.ID)
		entry.Updated = strings.TrimSpace(entry.Updated)
		entries = append(entries, entry)
	}

	var parseErr error
	sort.SliceStable(entries, func(i, j int) bool {
		left, err := entries[i].UpdatedTime()
		if err != nil && parseErr == nil {
			parseErr = err
		}
		right, err := entries[j].UpdatedTime()
		if err != nil && parseErr == nil {
			parseErr = err
		}
		return left.Before(right)
	})
	if parseErr != nil {
		return nil, parseErr
	}
	return entries, nil
}
