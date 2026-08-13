package exportjson

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/9SQ/jma-weather-warning-converter-go/internal/domain"
	"github.com/9SQ/jma-weather-warning-converter-go/internal/storage"
	"github.com/9SQ/jma-weather-warning-converter-go/internal/tables"
)

type Output struct {
	Updated  string       `json:"updated"`
	Exported string       `json:"exported"`
	Areas    []OutputArea `json:"areas"`
}

type OutputArea struct {
	Code string   `json:"code"`
	Name string   `json:"name"`
	Kind []string `json:"kind"`
}

type areaState struct {
	name string
	kind map[string]struct{}
}

func Build(weather []storage.WarningRecord, floods []storage.FloodRecord, mappings *tables.Mappings, updatedAt, exportedAt time.Time) Output {
	areas := map[string]*areaState{}

	for _, record := range weather {
		if domain.IsWeatherNoWarningKind(record.KindCode) {
			continue
		}
		addKind(areas, record.AreaCode, record.AreaName, record.KindCode)
	}

	for _, record := range floods {
		convertedKind, ok := domain.ConvertFloodKind(record.KindCode)
		if !ok || convertedKind == "" {
			continue
		}
		for _, area := range mappings.ExpandRiverArea(record.RiverAreaCode) {
			addKind(areas, area.Code, area.Name, convertedKind)
		}
	}

	codes := make([]string, 0, len(areas))
	for code := range areas {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	out := Output{
		Updated:  updatedAt.Format(time.RFC3339),
		Exported: exportedAt.Format(time.RFC3339),
		Areas:    make([]OutputArea, 0, len(codes)),
	}
	for _, code := range codes {
		state := areas[code]
		kinds := make([]string, 0, len(state.kind))
		for kindCode := range state.kind {
			kinds = append(kinds, kindCode)
		}
		sort.Strings(kinds)
		out.Areas = append(out.Areas, OutputArea{
			Code: code,
			Name: state.name,
			Kind: kinds,
		})
	}
	return out
}

func WriteFile(path string, output Output) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(path), ".weather_warning_*.json")
	if err != nil {
		return fmt.Errorf("create temp output: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	encoder := json.NewEncoder(tempFile)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		tempFile.Close()
		return fmt.Errorf("write json: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temp output: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace output %s: %w", path, err)
	}
	return nil
}

func addKind(areas map[string]*areaState, code, name, kindCode string) {
	if code == "" || kindCode == "" {
		return
	}
	state, ok := areas[code]
	if !ok {
		state = &areaState{kind: map[string]struct{}{}}
		areas[code] = state
	}
	if state.name == "" {
		state.name = name
	}
	state.kind[kindCode] = struct{}{}
}
