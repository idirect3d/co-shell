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

package xlsx

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// OpenFile reads an XLSX file from disk and returns a parsed Workbook.
func OpenFile(path string) (*Workbook, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve path %q: %w", path, err)
	}

	r, err := zip.OpenReader(absPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open xlsx file %q: %w", absPath, err)
	}
	defer r.Close()

	files := make(map[string]*zip.File)
	for _, f := range r.File {
		files[filepath.ToSlash(f.Name)] = f
	}

	wb := &Workbook{Path: absPath}

	if err := parseSST(files, wb); err != nil {
		return nil, fmt.Errorf("cannot parse shared strings: %w", err)
	}

	// Parse styles (fonts, fills, borders, cellXfs) so StyleID attributes
	// on cells can be preserved through edit → save cycles without losing
	// cell formatting (FEATURE-120 fix).
	if err := parseStyles(files, wb); err != nil {
		return nil, fmt.Errorf("cannot parse styles: %w", err)
	}

	if err := parseWb(files, wb); err != nil {
		return nil, fmt.Errorf("cannot parse workbook: %w", err)
	}

	sheetPaths, err := parseRels(files, len(wb.Sheets))
	if err != nil {
		return nil, fmt.Errorf("cannot parse relationships: %w", err)
	}

	for i, sheet := range wb.Sheets {
		sp, ok := sheetPaths[i]
		if !ok || sp == "" {
			continue
		}
		if err := parseSheet(files, sp, sheet); err != nil {
			return nil, fmt.Errorf("cannot parse sheet %q: %w", sheet.Name, err)
		}
	}

	return wb, nil
}

// parseSST reads shared strings using xml.Decoder token-by-token.
// Format: <sst><si><t>text</t></si><si><t>text2</t></si></sst>
func parseSST(files map[string]*zip.File, wb *Workbook) error {
	f, ok := files["xl/sharedStrings.xml"]
	if !ok {
		return nil
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	dec := xml.NewDecoder(rc)
	var strs []string
	var currentText strings.Builder
	inT := false

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "t" {
				inT = true
				currentText.Reset()
			}
		case xml.EndElement:
			if t.Name.Local == "t" {
				strs = append(strs, currentText.String())
				inT = false
			}
		case xml.CharData:
			if inT {
				currentText.Write(t)
			}
		}
	}

	wb.SharedStrings = strs
	return nil
}

// parseWb reads workbook.xml
func parseWb(files map[string]*zip.File, wb *Workbook) error {
	f, ok := files["xl/workbook.xml"]
	if !ok {
		return fmt.Errorf("xl/workbook.xml not found")
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	dec := xml.NewDecoder(rc)
	var currentSheetName string
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "sheet" {
				for _, attr := range t.Attr {
					if attr.Name.Local == "name" {
						currentSheetName = attr.Value
					}
				}
			}
		case xml.EndElement:
			if t.Name.Local == "sheet" && currentSheetName != "" {
				wb.Sheets = append(wb.Sheets, &Sheet{
					Index: len(wb.Sheets),
					Name:  currentSheetName,
					Rows:  make(map[int]map[int]*Cell),
				})
				currentSheetName = ""
			}
		}
	}
	if len(wb.Sheets) == 0 {
		return fmt.Errorf("no sheets found in workbook")
	}
	return nil
}

// parseRels reads relationships.
func parseRels(files map[string]*zip.File, sheetCount int) (map[int]string, error) {
	sheetPaths := make(map[int]string)
	for i := 0; i < sheetCount; i++ {
		sheetPaths[i] = fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1)
	}

	f, ok := files["xl/_rels/workbook.xml.rels"]
	if !ok {
		return sheetPaths, nil
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	dec := xml.NewDecoder(rc)
	rels := make(map[string]string)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "Relationship" {
				var id, target string
				for _, attr := range t.Attr {
					if attr.Name.Local == "Id" {
						id = attr.Value
					}
					if attr.Name.Local == "Target" {
						target = attr.Value
					}
				}
				if id != "" && target != "" && strings.HasPrefix(target, "worksheets/") {
					rels[id] = "xl/" + target
				}
			}
		}
	}

	// Re-read workbook to match rID with sheet index
	wbF, ok := files["xl/workbook.xml"]
	if !ok {
		return sheetPaths, nil
	}
	wbRC, err := wbF.Open()
	if err != nil {
		return nil, err
	}
	defer wbRC.Close()

	wbDec := xml.NewDecoder(wbRC)
	idx := 0
	for {
		tok, err := wbDec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "sheet" {
				var rid string
				for _, attr := range t.Attr {
					if attr.Name.Local == "id" {
						rid = attr.Value
					}
				}
				if rid != "" {
					if p, ok := rels[rid]; ok {
						sheetPaths[idx] = p
					}
				}
				idx++
			}
		}
	}

	return sheetPaths, nil
}

// parseStyles opens xl/styles.xml and populates the workbook's style manager.
func parseStyles(files map[string]*zip.File, wb *Workbook) error {
	f, ok := files["xl/styles.xml"]
	if !ok {
		return nil // styles.xml is optional
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	sm := wb.StyleManager()
	return sm.parseStylesReader(rc)
}

// parseSheet reads a worksheet using xml.Decoder token-by-token.
func parseSheet(files map[string]*zip.File, path string, sheet *Sheet) error {
	f, ok := files[path]
	if !ok {
		return fmt.Errorf("worksheet file %q not found", path)
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	dec := xml.NewDecoder(rc)
	var currentRow int
	var currentCell *Cell
	inRow := false
	inCell := false
	inValue := false
	inFormula := false
	inCols := false
	var currentColInfo ColInfo
	var cellText strings.Builder

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "cols":
				inCols = true

			case "col":
				if inCols {
					currentColInfo = ColInfo{Min: 1, Max: 1, Width: 9.0}
					for _, attr := range t.Attr {
						switch attr.Name.Local {
						case "min":
							fmt.Sscanf(attr.Value, "%d", &currentColInfo.Min)
						case "max":
							fmt.Sscanf(attr.Value, "%d", &currentColInfo.Max)
						case "width":
							fmt.Sscanf(attr.Value, "%f", &currentColInfo.Width)
						}
					}
				}

			case "mergeCell":
				for _, attr := range t.Attr {
					if attr.Name.Local == "ref" {
						parts := strings.Split(attr.Value, ":")
						if len(parts) == 2 {
							sc, sr, _ := ParseCellRef(strings.TrimSpace(parts[0]))
							ec, er, _ := ParseCellRef(strings.TrimSpace(parts[1]))
							sheet.MergeCells = append(sheet.MergeCells, MergeCell{
								StartCol: sc, StartRow: sr,
								EndCol: ec, EndRow: er,
							})
						}
					}
				}

			case "row":
				currentRow = 0
				rh := float64(0)
				for _, attr := range t.Attr {
					if attr.Name.Local == "r" {
						fmt.Sscanf(attr.Value, "%d", &currentRow)
					}
					if attr.Name.Local == "ht" {
						fmt.Sscanf(attr.Value, "%f", &rh)
					}
				}
				currentRow-- // convert to 0-based
				if currentRow < 0 {
					currentRow = 0
				}
				if rh > 0 {
					sheet.RowHeights = append(sheet.RowHeights, RowHeight{Row: currentRow, Height: rh})
				}
				inRow = true

			case "c":
				if !inRow {
					break
				}
				currentCell = &Cell{
					Row:  currentRow,
					Col:  0,
					Type: "",
				}
				for _, attr := range t.Attr {
					if attr.Name.Local == "r" {
						currentCell.ColRef = attr.Value
						col, _, err := ParseCellRef(attr.Value)
						if err == nil {
							currentCell.Col = col
						}
					}
					if attr.Name.Local == "t" {
						currentCell.Type = attr.Value
					}
					if attr.Name.Local == "s" {
						fmt.Sscanf(attr.Value, "%d", &currentCell.StyleID)
					}
				}
				inCell = true
				inValue = false
				inFormula = false
				cellText.Reset()

			case "v":
				if inCell {
					inValue = true
					cellText.Reset()
				}

			case "f":
				if inCell {
					inFormula = true
					cellText.Reset()
				}
			}

		case xml.EndElement:
			switch t.Name.Local {
			case "cols":
				inCols = false
			case "col":
				if inCols {
					sheet.ColInfos = append(sheet.ColInfos, currentColInfo)
				}
			case "row":
				inRow = false
			case "mergeCell":
				// Handled in StartElement, just pass through
			case "mergeCells":
				// End of merge cells section
			case "c":
				if inCell && currentCell != nil {
					if sheet.Rows[currentRow] == nil {
						sheet.Rows[currentRow] = make(map[int]*Cell)
					}
					sheet.Rows[currentRow][currentCell.Col] = currentCell
					if currentCell.Col > sheet.MaxCol {
						sheet.MaxCol = currentCell.Col
					}
					if currentCell.Formula != "" {
						sheet.HasFormulas = true
					}
				}
				inCell = false
				currentCell = nil
			case "v":
				if inValue {
					if currentCell != nil {
						currentCell.Value = cellText.String()
					}
					inValue = false
				}
			case "f":
				if inFormula {
					if currentCell != nil {
						currentCell.Formula = cellText.String()
					}
					inFormula = false
				}
			}

		case xml.CharData:
			if inValue || inFormula {
				cellText.Write(t)
			}
		}
	}

	// Update MaxRow
	for r := range sheet.Rows {
		if r > sheet.MaxRow {
			sheet.MaxRow = r
		}
	}

	return nil
}
