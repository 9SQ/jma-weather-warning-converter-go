package tables

import (
	"encoding/json"
	"fmt"
	"os"
)

type CodeName struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type RiverArea struct {
	Name   string     `json:"name"`
	Cities []CodeName `json:"cities"`
}

type Mappings struct {
	RiverToCities  map[string]RiverArea
	CityToLV2Areas map[string][]CodeName
}

func Load(riverToCitiesPath, cityToLV2AreasPath string) (*Mappings, error) {
	riverToCities := make(map[string]RiverArea)
	if err := readJSONFile(riverToCitiesPath, &riverToCities); err != nil {
		return nil, err
	}

	cityToLV2Areas := make(map[string][]CodeName)
	if err := readJSONFile(cityToLV2AreasPath, &cityToLV2Areas); err != nil {
		return nil, err
	}

	return &Mappings{
		RiverToCities:  riverToCities,
		CityToLV2Areas: cityToLV2Areas,
	}, nil
}

func (m *Mappings) ExpandRiverArea(riverAreaCode string) []CodeName {
	riverArea, ok := m.RiverToCities[riverAreaCode]
	if !ok {
		return nil
	}

	seen := map[string]struct{}{}
	areas := make([]CodeName, 0, len(riverArea.Cities))
	for _, city := range riverArea.Cities {
		lv2Areas, ok := m.CityToLV2Areas[city.Code]
		if !ok {
			if _, exists := seen[city.Code]; exists {
				continue
			}
			seen[city.Code] = struct{}{}
			areas = append(areas, city)
			continue
		}
		for _, lv2Area := range lv2Areas {
			if _, exists := seen[lv2Area.Code]; exists {
				continue
			}
			seen[lv2Area.Code] = struct{}{}
			areas = append(areas, lv2Area)
		}
	}
	return areas
}

func readJSONFile(path string, out any) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}
