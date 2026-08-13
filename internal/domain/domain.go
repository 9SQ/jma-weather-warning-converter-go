package domain

const (
	DefaultFeedURL = "https://www.data.jma.go.jp/developer/xml/feed/extra.xml"

	WeatherInfoType = "気象警報・注意報（市町村等）"
	FloodInfoType   = "指定河川洪水予報（予報区域）"

	FloodTitle               = "指定河川洪水予報"
	WeatherNoWarningKindCode = "00"
)

var WeatherTitleToCategory = map[string]string{
	"気象警報・注意報（Ｒ０６）（大雨）":     "大雨",
	"気象警報・注意報（Ｒ０６）（土砂）":     "土砂",
	"気象警報・注意報（Ｒ０６）（高潮）":     "高潮",
	"気象警報・注意報（Ｒ０６）（暴風）":     "暴風",
	"気象警報・注意報（Ｒ０６）（波浪）":     "波浪",
	"気象警報・注意報（Ｒ０６）（大雪）":     "大雪",
	"気象警報・注意報（Ｒ０６）（その他注意報）": "その他注意報",
}

var TargetTitles = func() map[string]struct{} {
	titles := map[string]struct{}{
		FloodTitle: {},
	}
	for title := range WeatherTitleToCategory {
		titles[title] = struct{}{}
	}
	return titles
}()

var FloodKindToWeatherKind = map[string]string{
	"10": "",
	"20": "F18",
	"21": "F18",
	"22": "",
	"30": "F04",
	"31": "F04",
	"40": "F44",
	"41": "F44",
	"51": "F34",
	"53": "F34",
}

func CategoryForTitle(title string) (string, bool) {
	category, ok := WeatherTitleToCategory[title]
	return category, ok
}

func IsFloodTitle(title string) bool {
	return title == FloodTitle
}

func IsTargetTitle(title string) bool {
	_, ok := TargetTitles[title]
	return ok
}

func ConvertFloodKind(code string) (string, bool) {
	converted, ok := FloodKindToWeatherKind[code]
	return converted, ok
}

func IsWeatherNoWarningKind(code string) bool {
	return code == WeatherNoWarningKindCode
}
