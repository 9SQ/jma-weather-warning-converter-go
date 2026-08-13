package exportjson

import (
	"reflect"
	"testing"
	"time"

	"github.com/9SQ/jma-weather-warning-converter-go/internal/storage"
	"github.com/9SQ/jma-weather-warning-converter-go/internal/tables"
)

func TestBuildMergesWeatherAndExpandedFloodForecast(t *testing.T) {
	output := Build(
		[]storage.WarningRecord{
			{AreaCode: "0120601", AreaName: "釧路市釧路", KindCode: "03"},
			{AreaCode: "0120601", AreaName: "釧路市釧路", KindCode: "00"},
			{AreaCode: "0120601", AreaName: "釧路市釧路", KindCode: "03"},
			{AreaCode: "9999999", AreaName: "出力しない区域", KindCode: "00"},
		},
		[]storage.FloodRecord{
			{RiverAreaCode: "river-1", RiverAreaName: "河川A", KindCode: "20"},
			{RiverAreaCode: "river-1", RiverAreaName: "河川A", KindCode: "10"},
			{RiverAreaCode: "river-2", RiverAreaName: "河川B", KindCode: "31"},
		},
		&tables.Mappings{
			RiverToCities: map[string]tables.RiverArea{
				"river-1": {
					Name: "河川A",
					Cities: []tables.CodeName{
						{Code: "0110000", Name: "札幌市"},
						{Code: "0120300", Name: "小樽市"},
					},
				},
				"river-2": {
					Name: "河川B",
					Cities: []tables.CodeName{
						{Code: "0120600", Name: "釧路市"},
					},
				},
			},
			CityToLV2Areas: map[string][]tables.CodeName{
				"0110000": {
					{Code: "0110001", Name: "札幌市中央区"},
					{Code: "0110002", Name: "札幌市北区"},
				},
				"0120600": {
					{Code: "0120601", Name: "釧路市釧路"},
				},
			},
		},
		time.Date(2026, 7, 26, 1, 2, 3, 0, time.FixedZone("JST", 9*60*60)),
		time.Date(2026, 7, 26, 1, 3, 4, 0, time.FixedZone("JST", 9*60*60)),
	)

	if output.Updated != "2026-07-26T01:02:03+09:00" {
		t.Fatalf("updated = %q", output.Updated)
	}
	if output.Exported != "2026-07-26T01:03:04+09:00" {
		t.Fatalf("exported = %q", output.Exported)
	}

	want := []OutputArea{
		{Code: "0110001", Name: "札幌市中央区", Kind: []string{"F18"}},
		{Code: "0110002", Name: "札幌市北区", Kind: []string{"F18"}},
		{Code: "0120300", Name: "小樽市", Kind: []string{"F18"}},
		{Code: "0120601", Name: "釧路市釧路", Kind: []string{"03", "F04"}},
	}
	if !reflect.DeepEqual(output.Areas, want) {
		t.Fatalf("areas = %#v, want %#v", output.Areas, want)
	}
}
