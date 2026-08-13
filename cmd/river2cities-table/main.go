package main

import (
	"archive/zip"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
)

const (
	defaultInput  = "指定河川洪水予報（氾濫警報・注意報）_解説資料_別紙.xlsx"
	defaultOutput = "river_to_cities.json"
)

type City struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type Area struct {
	Name   string `json:"name"`
	Cities []City `json:"cities"`
}

type areaBuilder struct {
	Name   string
	Cities []City
	seen   map[string]struct{}
}

type workbookSheet struct {
	Name string
	RID  string
}

func (s *workbookSheet) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "name":
			s.Name = attr.Value
		case "id":
			s.RID = attr.Value
		}
	}
	return dec.Skip()
}

type workbookXML struct {
	Sheets []workbookSheet `xml:"sheets>sheet"`
}

type relationshipXML struct {
	ID     string `xml:"Id,attr"`
	Type   string `xml:"Type,attr"`
	Target string `xml:"Target,attr"`
}

type relationshipsXML struct {
	Relationships []relationshipXML `xml:"Relationship"`
}

type sharedStringItem struct {
	Text string
}

func (s *sharedStringItem) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	text, err := readRichText(dec, start)
	if err != nil {
		return err
	}
	s.Text = text
	return nil
}

type sharedStringsXML struct {
	Items []sharedStringItem `xml:"si"`
}

type inlineString struct {
	Text string
}

func (s *inlineString) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	text, err := readRichText(dec, start)
	if err != nil {
		return err
	}
	s.Text = text
	return nil
}

type cellXML struct {
	Ref          string       `xml:"r,attr"`
	Type         string       `xml:"t,attr"`
	Value        string       `xml:"v"`
	InlineString inlineString `xml:"is"`
}

type rowXML struct {
	Index int       `xml:"r,attr"`
	Cells []cellXML `xml:"c"`
}

type mergeCellXML struct {
	Ref string `xml:"ref,attr"`
}

type worksheetXML struct {
	Rows       []rowXML       `xml:"sheetData>row"`
	MergeCells []mergeCellXML `xml:"mergeCells>mergeCell"`
}

type sheetData struct {
	Rows   map[int]map[int]string
	Merges []cellRange
	MaxRow int
	MaxCol int
}

type cellRange struct {
	StartRow int
	StartCol int
	EndRow   int
	EndCol   int
}

type sheetRef struct {
	Name string
	Path string
}

var digitsOnly = regexp.MustCompile(`^\d+$`)

func main() {
	input := flag.String("in", defaultInput, "input xlsx file")
	output := flag.String("out", defaultOutput, "output json file, or - for stdout")
	flag.Parse()

	areas, err := convertWorkbook(*input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	data, err := json.MarshalIndent(areas, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: marshal json: %v\n", err)
		os.Exit(1)
	}
	data = append(data, '\n')

	if *output == "-" {
		_, _ = os.Stdout.Write(data)
		return
	}
	if err := os.WriteFile(*output, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error: write output: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%d forecast areas)\n", *output, len(areas))
}

func convertWorkbook(filename string) (map[string]Area, error) {
	reader, err := zip.OpenReader(filename)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	files := make(map[string]*zip.File, len(reader.File))
	for _, file := range reader.File {
		files[file.Name] = file
	}

	sharedStrings, err := loadSharedStrings(files)
	if err != nil {
		return nil, err
	}

	sheets, err := loadWorkbookSheets(files)
	if err != nil {
		return nil, err
	}
	if len(sheets) == 0 {
		return nil, errors.New("worksheet not found")
	}

	builders := map[string]*areaBuilder{}
	for _, sheet := range sheets {
		ws, ok := files[sheet.Path]
		if !ok {
			return nil, fmt.Errorf("worksheet %q not found at %s", sheet.Name, sheet.Path)
		}
		data, err := loadWorksheet(ws, sharedStrings)
		if err != nil {
			return nil, fmt.Errorf("read worksheet %q: %w", sheet.Name, err)
		}
		extractAreas(data, builders)
	}

	result := make(map[string]Area, len(builders))
	for code, builder := range builders {
		result[code] = Area{
			Name:   builder.Name,
			Cities: builder.Cities,
		}
	}
	return result, nil
}

func loadWorkbookSheets(files map[string]*zip.File) ([]sheetRef, error) {
	workbookFile, ok := files["xl/workbook.xml"]
	if !ok {
		return nil, errors.New("xl/workbook.xml not found")
	}
	workbookBytes, err := readZipFile(workbookFile)
	if err != nil {
		return nil, err
	}
	var workbook workbookXML
	if err := xml.Unmarshal(workbookBytes, &workbook); err != nil {
		return nil, err
	}

	relsFile, ok := files["xl/_rels/workbook.xml.rels"]
	if !ok {
		return nil, errors.New("xl/_rels/workbook.xml.rels not found")
	}
	relsBytes, err := readZipFile(relsFile)
	if err != nil {
		return nil, err
	}
	var relationships relationshipsXML
	if err := xml.Unmarshal(relsBytes, &relationships); err != nil {
		return nil, err
	}

	targetByID := map[string]string{}
	for _, rel := range relationships.Relationships {
		if !strings.Contains(rel.Type, "/worksheet") {
			continue
		}
		targetByID[rel.ID] = resolveXLSXPath("xl", rel.Target)
	}

	sheets := make([]sheetRef, 0, len(workbook.Sheets))
	for _, sheet := range workbook.Sheets {
		target := targetByID[sheet.RID]
		if target == "" {
			continue
		}
		sheets = append(sheets, sheetRef{
			Name: sheet.Name,
			Path: target,
		})
	}
	return sheets, nil
}

func loadSharedStrings(files map[string]*zip.File) ([]string, error) {
	file, ok := files["xl/sharedStrings.xml"]
	if !ok {
		return nil, nil
	}
	data, err := readZipFile(file)
	if err != nil {
		return nil, err
	}
	var table sharedStringsXML
	if err := xml.Unmarshal(data, &table); err != nil {
		return nil, err
	}

	items := make([]string, len(table.Items))
	for i, item := range table.Items {
		items[i] = cleanText(item.Text)
	}
	return items, nil
}

func loadWorksheet(file *zip.File, sharedStrings []string) (*sheetData, error) {
	data, err := readZipFile(file)
	if err != nil {
		return nil, err
	}

	var sheet worksheetXML
	if err := xml.Unmarshal(data, &sheet); err != nil {
		return nil, err
	}

	rows := map[int]map[int]string{}
	maxRow := 0
	maxCol := 0
	for _, row := range sheet.Rows {
		rowIndex := row.Index
		for _, cell := range row.Cells {
			col, refRow, err := splitCellRef(cell.Ref)
			if err != nil {
				return nil, err
			}
			if rowIndex == 0 {
				rowIndex = refRow
			}
			value, err := cellValue(cell, sharedStrings)
			if err != nil {
				return nil, fmt.Errorf("cell %s: %w", cell.Ref, err)
			}
			value = cleanText(value)
			if value == "" {
				continue
			}
			if rows[refRow] == nil {
				rows[refRow] = map[int]string{}
			}
			rows[refRow][col] = value
			if refRow > maxRow {
				maxRow = refRow
			}
			if col > maxCol {
				maxCol = col
			}
		}
	}

	merges := make([]cellRange, 0, len(sheet.MergeCells))
	for _, merge := range sheet.MergeCells {
		cellRange, err := parseCellRange(merge.Ref)
		if err != nil {
			return nil, fmt.Errorf("merge range %s: %w", merge.Ref, err)
		}
		merges = append(merges, cellRange)
		applyMerge(rows, cellRange)
		if cellRange.EndRow > maxRow {
			maxRow = cellRange.EndRow
		}
		if cellRange.EndCol > maxCol {
			maxCol = cellRange.EndCol
		}
	}

	return &sheetData{
		Rows:   rows,
		Merges: merges,
		MaxRow: maxRow,
		MaxCol: maxCol,
	}, nil
}

func extractAreas(data *sheetData, builders map[string]*areaBuilder) {
	headerRow, areaNameCol, areaCodeCol, cityStartCol, cityEndCol := findHeader(data)
	if headerRow == 0 {
		return
	}

	currentAreaCode := ""
	currentAreaName := ""
	for row := headerRow + 1; row <= data.MaxRow-1; row++ {
		if value := cell(data, row, areaCodeCol); value != "" {
			currentAreaCode = normalizeCode(value, 12)
		}
		if value := cell(data, row, areaNameCol); value != "" {
			currentAreaName = value
		}
		if currentAreaCode == "" || currentAreaName == "" {
			continue
		}
		if !isCityNameRow(data, row, cityStartCol, cityEndCol) {
			continue
		}

		builder := builders[currentAreaCode]
		if builder == nil {
			builder = &areaBuilder{
				Name: currentAreaName,
				seen: map[string]struct{}{},
			}
			builders[currentAreaCode] = builder
		}
		if builder.Name == "" {
			builder.Name = currentAreaName
		}

		for col := cityStartCol; col <= cityEndCol; col++ {
			name := cell(data, row, col)
			code := normalizeCode(cell(data, row+1, col), 7)
			if name == "" && code == "" {
				continue
			}
			if looksLikeCode(name) && !looksLikeCode(code) {
				continue
			}
			if name == "" {
				continue
			}
			key := code + "\x00" + name
			if _, ok := builder.seen[key]; ok {
				continue
			}
			builder.seen[key] = struct{}{}
			builder.Cities = append(builder.Cities, City{
				Code: code,
				Name: name,
			})
		}
	}
}

func findHeader(data *sheetData) (headerRow, areaNameCol, areaCodeCol, cityStartCol, cityEndCol int) {
	for row := 1; row <= data.MaxRow; row++ {
		for col := 1; col <= data.MaxCol; col++ {
			value := cell(data, row, col)
			switch {
			case areaNameCol == 0 && strings.Contains(value, "洪水予報区域名"):
				areaNameCol = col
			case areaCodeCol == 0 && strings.Contains(value, "予報区域コード"):
				areaCodeCol = col
			case cityStartCol == 0 && strings.Contains(value, "洪水浸水想定区域を含む市区町村名"):
				cityStartCol = col
			}
		}
		if areaNameCol != 0 && areaCodeCol != 0 && cityStartCol != 0 {
			headerRow = row
			break
		}
		areaNameCol = 0
		areaCodeCol = 0
		cityStartCol = 0
	}
	if headerRow == 0 {
		return 0, 0, 0, 0, 0
	}

	cityEndCol = data.MaxCol
	for _, merge := range data.Merges {
		if merge.StartRow == headerRow && merge.StartCol == cityStartCol {
			cityEndCol = merge.EndCol
			break
		}
	}
	return headerRow, areaNameCol, areaCodeCol, cityStartCol, cityEndCol
}

func isCityNameRow(data *sheetData, row, startCol, endCol int) bool {
	for col := startCol; col <= endCol; col++ {
		name := cell(data, row, col)
		code := cell(data, row+1, col)
		if name == "" || code == "" {
			continue
		}
		if !looksLikeCode(name) && looksLikeCode(code) {
			return true
		}
	}
	return false
}

func cell(data *sheetData, row, col int) string {
	if data.Rows[row] == nil {
		return ""
	}
	return data.Rows[row][col]
}

func cellValue(cell cellXML, sharedStrings []string) (string, error) {
	switch cell.Type {
	case "s":
		if cell.Value == "" {
			return "", nil
		}
		index, err := strconv.Atoi(cell.Value)
		if err != nil {
			return "", err
		}
		if index < 0 || index >= len(sharedStrings) {
			return "", fmt.Errorf("shared string index %d out of range", index)
		}
		return sharedStrings[index], nil
	case "inlineStr":
		return cell.InlineString.Text, nil
	default:
		return cell.Value, nil
	}
}

func readRichText(dec *xml.Decoder, start xml.StartElement) (string, error) {
	var builder strings.Builder
	skipDepth := 0
	for {
		token, err := dec.Token()
		if err != nil {
			return "", err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "rPh" {
				skipDepth = 1
				continue
			}
			if skipDepth > 0 {
				skipDepth++
				continue
			}
			if t.Name.Local != "t" {
				continue
			}
			var text string
			if err := dec.DecodeElement(&text, &t); err != nil {
				return "", err
			}
			builder.WriteString(text)
		case xml.EndElement:
			if skipDepth > 0 {
				skipDepth--
				continue
			}
			if t.Name == start.Name {
				return builder.String(), nil
			}
		}
	}
}

func applyMerge(rows map[int]map[int]string, cellRange cellRange) {
	value := ""
	if rows[cellRange.StartRow] != nil {
		value = rows[cellRange.StartRow][cellRange.StartCol]
	}
	if value == "" {
		return
	}
	for row := cellRange.StartRow; row <= cellRange.EndRow; row++ {
		if rows[row] == nil {
			rows[row] = map[int]string{}
		}
		for col := cellRange.StartCol; col <= cellRange.EndCol; col++ {
			if rows[row][col] == "" {
				rows[row][col] = value
			}
		}
	}
}

func readZipFile(file *zip.File) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func resolveXLSXPath(baseDir, target string) string {
	if strings.HasPrefix(target, "/") {
		return strings.TrimPrefix(path.Clean(target), "/")
	}
	return path.Clean(path.Join(baseDir, target))
}

func cleanText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.Trim(value, " \t\n\v\f\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000")
}

func normalizeCode(value string, width int) string {
	value = cleanText(value)
	value = strings.TrimSuffix(value, ".0")
	if width > 0 && digitsOnly.MatchString(value) && len(value) < width {
		return strings.Repeat("0", width-len(value)) + value
	}
	return value
}

func looksLikeCode(value string) bool {
	value = normalizeCode(value, 0)
	return digitsOnly.MatchString(value)
}

func splitCellRef(ref string) (col int, row int, err error) {
	if ref == "" {
		return 0, 0, errors.New("empty cell reference")
	}
	i := 0
	for i < len(ref) && ref[i] >= 'A' && ref[i] <= 'Z' {
		col = col*26 + int(ref[i]-'A'+1)
		i++
	}
	if i == 0 || i == len(ref) {
		return 0, 0, fmt.Errorf("invalid cell reference %q", ref)
	}
	row, err = strconv.Atoi(ref[i:])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid cell reference %q", ref)
	}
	return col, row, nil
}

func parseCellRange(ref string) (cellRange, error) {
	parts := strings.Split(ref, ":")
	if len(parts) == 1 {
		col, row, err := splitCellRef(parts[0])
		if err != nil {
			return cellRange{}, err
		}
		return cellRange{StartRow: row, StartCol: col, EndRow: row, EndCol: col}, nil
	}
	if len(parts) != 2 {
		return cellRange{}, fmt.Errorf("invalid range %q", ref)
	}
	startCol, startRow, err := splitCellRef(parts[0])
	if err != nil {
		return cellRange{}, err
	}
	endCol, endRow, err := splitCellRef(parts[1])
	if err != nil {
		return cellRange{}, err
	}
	if startRow > endRow {
		startRow, endRow = endRow, startRow
	}
	if startCol > endCol {
		startCol, endCol = endCol, startCol
	}
	return cellRange{StartRow: startRow, StartCol: startCol, EndRow: endRow, EndCol: endCol}, nil
}
