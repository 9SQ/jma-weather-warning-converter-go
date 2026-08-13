# jma-weather-warning-converter-go

気象庁防災情報XMLのAtomフィードから気象警報・注意報のXML電文群を取得・統合し、2026年5月29日より運用開始された「新たな防災気象情報」に対応した `weather_warning.json` を生成します。

本リポジトリでは [防災情報処理 2026 夏](https://quitsq.com/projects/dpip1/) に掲載した `「新たな防災気象情報」を地図に描きたい！ ～気象警報・注意報（R06）編～` で使用しているコードを公開します。

## 実行手順

### 1. river_to_cities.json を生成する (資料更新時のみ)

2026年7月7日時点のテーブルを `table/river_to_cities.json` に入れているため、これ以降に更新があった場合のみ実施してください。

指定河川の洪水予報区域と浸水想定区域を含む市区町村の対応表を元に、変換テーブルを生成します。

1. [気象庁防災情報XMLフォーマット｜技術資料](https://xml.kishou.go.jp/tec_material.html) のページにある **電文毎の解説資料** 解説資料セットをダウンロードして展開し、 `指定河川洪水予報（氾濫警報・注意報）_解説資料_別紙.xlsx` を配置してください。
2. `go run ./cmd/river2cities-table` を実行すると、 `river_to_cities.json` が生成されます。

#### 解説資料セットのzipファイルが正常に展開されない場合

文字コードの影響により一部のOS(macOS等)で文字化けして展開されることがあります。

`unzip -O cp932 "jmaxml_20260707_Manual(pdf).zip" -d manual` のようにすると正常に展開されます。

### 2. weather_warning.json を生成する

```sh
go run ./cmd/wwxml-fetch
go run ./cmd/wwxml-import
go run ./cmd/wwjson-export
```

実行すると `weather_warning.json` が生成されます。

初回実行時は、長期フィードから取得することで全国分のデータを(ほぼ)取りこぼさずに収集できます。

```sh
go run ./cmd/wwxml-fetch -feed "https://www.data.jma.go.jp/developer/xml/feed/extra_l.xml"
```

`weather_warning.json` の仕様は以下の通りです。

## JSON仕様

```json
{
  "updated": "2026-07-26T03:52:00+09:00",
  "exported": "2026-07-26T03:57:34+09:00",
  "areas": [
    {
      "code": "0421501",
      "name": "大崎市東部",
      "kind": ["10", "14", "29", "F18"]
      },
    ... ＜省略＞...
  ]
}
```

| key | description |
| - | - | 
| `updated` | 一番最新のXML電文の `ReportDateTime` |
| `exported` | 出力したJSONファイルの生成日時 |
| `areas` | 二次細分区域ごとの配列 |

#### areas

| key | description |
| - | - |
| `code` | 二次細分区域コード |
| `name` | 二次細分区域名 |
| `kind` | 発表されている気象警報・注意報のコード |

#### kind table

|code|name|
|:----|:----|
|00|解除|
|02|暴風雪警報|
|03|レベル３大雨警報|
|05|暴風警報|
|06|大雪警報|
|07|波浪警報|
|08|レベル３高潮警報|
|09|レベル３土砂災害警報|
|10|レベル２大雨注意報|
|12|大雪注意報|
|13|風雪注意報|
|14|雷注意報|
|15|強風注意報|
|16|波浪注意報|
|17|融雪注意報|
|19|レベル２高潮注意報|
|20|濃霧注意報|
|21|乾燥注意報|
|22|なだれ注意報|
|23|低温注意報|
|24|霜注意報|
|25|着氷注意報|
|26|着雪注意報|
|27|その他の注意報|
|29|レベル２土砂災害注意報|
|32|暴風雪特別警報|
|33|レベル５大雨特別警報|
|35|暴風特別警報|
|36|大雪特別警報|
|37|波浪特別警報|
|38|レベル５高潮特別警報|
|39|レベル５土砂災害特別警報|
|43|レベル４大雨危険警報|
|48|レベル４高潮危険警報|
|49|レベル４土砂災害危険警報|
|F04|レベル３氾濫警報|
|F18|レベル２氾濫注意報|
|F34|レベル５氾濫特別警報|
|F44|レベル４氾濫危険警報|

## コマンドリファレンス

### wwxml-fetch

Atomフィードを取得し、対象のXML電文だけを保存します。

```sh
go run ./cmd/wwxml-fetch \
  -db "weather_warning.db" \
  -feed "https://www.data.jma.go.jp/developer/xml/feed/extra.xml" \
  -out "data/xml"
  -xml-retention 7
```

引数で指定しない場合、以下をデフォルト値として扱います。

- db: `weather_warning.db`
- feed: `https://www.data.jma.go.jp/developer/xml/feed/extra.xml`
- out: `data/xml`
- xml-retention: `7`

デフォルトでは直近7日分のXML電文を保持し、7日を超えた過去分は削除されます。  
`xml-retention` で `0` を指定すると無制限に保持します。任意の数値を指定するとその日数分保持します。

### wwxml-import

保存済みのXML電文を解析し、発表されている気象警報・注意報をSQLiteに保存します。

```sh
go run ./cmd/wwxml-import -db "weather_warning.db"
```

引数で指定しない場合、以下をデフォルト値として扱います。

- db: `weather_warning.db`

### wwjson-export

SQLiteに保存したデータから `weather_warning.json` を生成します。

```sh
go run ./cmd/wwjson-export \
  -db "weather_warning.db" \
  -river-table "table/river_to_cities.json" \
  -city-table "table/city_to_lv2areas.json" \
  -out "weather_warning.json"
```

引数で指定しない場合、以下をデフォルト値として扱います。

- db: `weather_warning.db`
- river-table: `table/river_to_cities.json`
- city-table: `table/city_to_lv2areas.json`
- out: `weather_warning.json`

### river2cities-table

`指定河川洪水予報（氾濫警報・注意報）_解説資料_別紙.xlsx` から `river_to_cities.json` を生成します。

```sh
go run ./cmd/river2cities-table \
  -in "指定河川洪水予報（氾濫警報・注意報）_解説資料_別紙.xlsx" \
  -out "river_to_cities.json"
```

引数で指定しない場合、以下をデフォルト値として扱います。

- in: `指定河川洪水予報（氾濫警報・注意報）_解説資料_別紙.xlsx`
- out: `river_to_cities.json`
