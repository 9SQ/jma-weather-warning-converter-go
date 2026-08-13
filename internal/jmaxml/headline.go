package jmaxml

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type HeadlineItem struct {
	AreaCode  string
	AreaName  string
	KindCodes []string
}

type Report struct {
	ReportDateTime string
	headline       headline
}

type message struct {
	Head head `xml:"Head"`
}

type head struct {
	ReportDateTime string   `xml:"ReportDateTime"`
	Headline       headline `xml:"Headline"`
}

type headline struct {
	Information []information `xml:"Information"`
}

type information struct {
	Type  string `xml:"type,attr"`
	Items []item `xml:"Item"`
}

type item struct {
	Kinds []kind  `xml:"Kind"`
	Areas []areas `xml:"Areas"`
}

type kind struct {
	Name string `xml:"Name"`
	Code string `xml:"Code"`
}

type areas struct {
	Areas []area `xml:"Area"`
}

type area struct {
	Name string `xml:"Name"`
	Code string `xml:"Code"`
}

func Parse(r io.Reader) (*Report, error) {
	var msg message
	decoder := xml.NewDecoder(r)
	if err := decoder.Decode(&msg); err != nil {
		return nil, fmt.Errorf("parse jma xml: %w", err)
	}
	return &Report{
		ReportDateTime: strings.TrimSpace(msg.Head.ReportDateTime),
		headline:       msg.Head.Headline,
	}, nil
}

func ExtractHeadlineItems(r io.Reader, informationType string) ([]HeadlineItem, error) {
	report, err := Parse(r)
	if err != nil {
		return nil, err
	}
	return report.HeadlineItems(informationType), nil
}

func (r *Report) HeadlineItems(informationType string) []HeadlineItem {
	var out []HeadlineItem
	for _, info := range r.headline.Information {
		if strings.TrimSpace(info.Type) != informationType {
			continue
		}
		for _, item := range info.Items {
			kindCodes := uniqueNonEmptyKindCodes(item.Kinds)
			if len(kindCodes) == 0 {
				continue
			}
			for _, areasBlock := range item.Areas {
				for _, area := range areasBlock.Areas {
					code := strings.TrimSpace(area.Code)
					if code == "" {
						continue
					}
					out = append(out, HeadlineItem{
						AreaCode:  code,
						AreaName:  strings.TrimSpace(area.Name),
						KindCodes: kindCodes,
					})
				}
			}
		}
	}
	return out
}

func uniqueNonEmptyKindCodes(kinds []kind) []string {
	seen := make(map[string]struct{}, len(kinds))
	codes := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		code := strings.TrimSpace(kind.Code)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		codes = append(codes, code)
	}
	return codes
}
