package jmaxml

import (
	"reflect"
	"strings"
	"testing"
)

func TestExtractHeadlineItems(t *testing.T) {
	items, err := ExtractHeadlineItems(strings.NewReader(`<?xml version="1.0" encoding="UTF-8"?>
<Report xmlns="http://xml.kishou.go.jp/jmaxml1/">
  <Head>
    <ReportDateTime>2026-07-26T01:02:03+09:00</ReportDateTime>
    <Headline>
      <Information type="府県予報区等">
        <Item>
          <Kind><Code>XX</Code></Kind>
          <Areas><Area><Name>対象外</Name><Code>0000000</Code></Area></Areas>
        </Item>
      </Information>
      <Information type="気象警報・注意報（市町村等）">
        <Item>
          <Kind><Code>03</Code></Kind>
          <Kind><Code>03</Code></Kind>
          <Kind><Code>29</Code></Kind>
          <Areas>
            <Area><Name>区域A</Name><Code>1111111</Code></Area>
            <Area><Name>区域B</Name><Code>2222222</Code></Area>
          </Areas>
        </Item>
      </Information>
    </Headline>
  </Head>
</Report>`), "気象警報・注意報（市町村等）")
	if err != nil {
		t.Fatal(err)
	}

	want := []HeadlineItem{
		{AreaCode: "1111111", AreaName: "区域A", KindCodes: []string{"03", "29"}},
		{AreaCode: "2222222", AreaName: "区域B", KindCodes: []string{"03", "29"}},
	}
	if !reflect.DeepEqual(items, want) {
		t.Fatalf("items = %#v, want %#v", items, want)
	}
}

func TestParseReportDateTime(t *testing.T) {
	report, err := Parse(strings.NewReader(`<?xml version="1.0" encoding="UTF-8"?>
<Report xmlns="http://xml.kishou.go.jp/jmaxml1/">
  <Head>
    <ReportDateTime>2026-07-26T01:02:03+09:00</ReportDateTime>
  </Head>
</Report>`))
	if err != nil {
		t.Fatal(err)
	}
	if report.ReportDateTime != "2026-07-26T01:02:03+09:00" {
		t.Fatalf("ReportDateTime = %q", report.ReportDateTime)
	}
}
