// Author: L.Shuang
// Created: 2026-07-07
// Last Modified: 2026-07-07
//
// MIT License
//
// Copyright (c) 2026 L.Shuang
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO
// EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES
// OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
// ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
// DEALINGS IN THE SOFTWARE.

// Package xlsx provides pure Go XLSX file parsing and writing.
// It implements the Office Open XML Spreadsheet format using only
// the Go standard library (archive/zip, encoding/xml).
package xlsx

// CellValueType represents the type of a cell's value.
type CellValueType string

const (
	CellTypeNumber  CellValueType = "N"
	CellTypeString  CellValueType = "S"
	CellTypeFormula CellValueType = "F"
	CellTypeBoolean CellValueType = "B"
	CellTypeDate    CellValueType = "D"
	CellTypeEmpty   CellValueType = "E"
	CellTypeError   CellValueType = "R"
)

// CellValue represents the value and metadata of a single cell.
type CellValue struct {
	Row     int           `json:"row"`
	Col     int           `json:"col"`
	ColRef  string        `json:"col_ref"` // e.g. "A", "C5"
	Type    CellValueType `json:"type"`
	Value   string        `json:"value"`             // displayed/parsed value
	Raw     string        `json:"raw,omitempty"`     // raw XML value (<v> tag)
	Formula string        `json:"formula,omitempty"` // formula text if cell type is formula
}

// CellRange represents a rectangular region of cells.
type CellRange struct {
	StartRow int `json:"start_row"`
	EndRow   int `json:"end_row"`
	StartCol int `json:"start_col"`
	EndCol   int `json:"end_col"`
}

// SheetMeta contains metadata about a worksheet.
type SheetMeta struct {
	Index       int      `json:"index"`
	Name        string   `json:"name"`
	Rows        int      `json:"rows"`
	Cols        int      `json:"cols"`
	UsedRange   string   `json:"used_range"` // e.g. "A1:L500"
	Headers     []string `json:"headers,omitempty"`
	HasFormulas bool     `json:"has_formulas"`
}

// Workbook represents an in-memory XLSX workbook.
type Workbook struct {
	Sheets        []*Sheet
	SharedStrings []string
	Path          string
	styles        *styleManager // lazy-initialized style manager
}

// Sheet represents a single worksheet.
type Sheet struct {
	Index       int
	Name        string
	Rows        map[int]map[int]*Cell // row -> col -> cell
	MaxRow      int
	MaxCol      int
	HasFormulas bool
	MergeCells  []MergeCell // merged cell ranges
	RowHeights  []RowHeight // custom row heights
	ColInfos    []ColInfo   // column width definitions
}

// Cell represents an XML cell element.
type Cell struct {
	ColRef  string // e.g. "A1"
	Col     int    // 0-based column index
	Row     int    // 0-based row index
	Type    string // "s"=shared string, "str"=inline, "b"=boolean, "e"=error, "n"=number
	Value   string // raw <v> value
	Formula string // <f> tag content, may be empty
	StyleID int    // index into styles.xml xf table (-1 = default style)
}

// XLSX namespace
const nsSpreadsheetML = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
const nsPackageRel = "http://schemas.openxmlformats.org/package/2006/relationships"

// Reader XML structs (with namespace support for encoding/xml)

type xlsxWorkbook struct {
	XMLName struct{}   `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main workbook"`
	Sheets  xlsxSheets `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main sheets"`
}

type xlsxSheets struct {
	Sheet []xlsxSheet `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main sheet"`
}

type xlsxSheet struct {
	Name string `xml:"name,attr"`
	ID   int    `xml:"sheetId,attr"`
	RID  string `xml:"r:id,attr"`
}

type xlsxRelationships struct {
	XMLName struct{}           `xml:"http://schemas.openxmlformats.org/package/2006/relationships Relationships"`
	Rel     []xlsxRelationship `xml:"http://schemas.openxmlformats.org/package/2006/relationships Relationship"`
}

type xlsxRelationship struct {
	ID     string `xml:"Id,attr"`
	Type   string `xml:"Type,attr"`
	Target string `xml:"Target,attr"`
}

type xlsxSST struct {
	XMLName struct{} `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main sst"`
	SI      []xlsxSI `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main si"`
	Count   int      `xml:"count,attr,omitempty"`
}

type xlsxSI struct {
	T string `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main t"` // simple text
}

type xlsxWorksheet struct {
	XMLName   struct{}      `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main worksheet"`
	SheetData xlsxSheetData `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main sheetData"`
}

type xlsxSheetData struct {
	Row []xlsxRow `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main row"`
}

type xlsxRow struct {
	R int     `xml:"r,attr"`
	C []xlsxC `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main c"`
}

// xlsxC note: for parseSheet(), the V and F fields use plain xml:"v" / xml:"f"
// because the worksheet XML's default namespace is stripped before parsing.
// For parseSST() and parseWb(), these fields are not used.
type xlsxC struct {
	R string `xml:"r,attr"` // cell reference, e.g. "A1"
	T string `xml:"t,attr"` // type: "s"=shared string, "str"=inline string, "b"=boolean, "e"=error, "n"=number
	V string `xml:"v"`      // value
	F string `xml:"f"`      // formula (optional)
}
