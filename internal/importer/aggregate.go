package importer

import (
	"github.com/9SQ/jma-weather-warning-converter-go/internal/jmaxml"
	"github.com/9SQ/jma-weather-warning-converter-go/internal/storage"
)

func AggregateHeadlineItems(items []jmaxml.HeadlineItem) map[storage.Area][]string {
	byAreaCode := make(map[string]storage.Area)
	kindCodes := make(map[string][]string)

	for _, item := range items {
		area := storage.Area{
			Code: item.AreaCode,
			Name: item.AreaName,
		}
		if existing, ok := byAreaCode[area.Code]; ok && existing.Name != "" {
			area.Name = existing.Name
		}
		byAreaCode[area.Code] = area
		kindCodes[area.Code] = append(kindCodes[area.Code], item.KindCodes...)
	}

	out := make(map[storage.Area][]string, len(byAreaCode))
	for code, area := range byAreaCode {
		out[area] = kindCodes[code]
	}
	return out
}
