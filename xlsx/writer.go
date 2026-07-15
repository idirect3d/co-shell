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
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
)

// Save writes the workbook back to its original path.
func (wb *Workbook) Save() error {
	if wb.Path == "" {
		return fmt.Errorf("workbook path is empty")
	}
	return wb.saveTo(wb.Path)
}

// SaveAs writes the workbook to the specified path.
func (wb *Workbook) SaveAs(path string) error {
	return wb.saveTo(path)
}

// StyleManager returns the style manager for this workbook, creating it if needed.
func (wb *Workbook) StyleManager() *styleManager {
	if wb.styles == nil {
		wb.styles = newStyleManager()
	}
	return wb.styles
}

func (wb *Workbook) saveTo(path string) error {
	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("cannot create temp file: %w", err)
	}

	zw := zip.NewWriter(f)

	// Determine whether we have an original file to preserve.
	hasOrig := false
	if zx := wb.Path; zx != "" {
		if fi, err := os.Stat(zx); err == nil && fi.Size() > 0 {
			hasOrig = true
		}
	}

	buildSheetName := func(i int) string {
		return fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1)
	}
	skip := make(map[string]bool)
	for i := range wb.Sheets {
		skip[buildSheetName(i)] = true
	}

	if hasOrig {
		// Copy original files, EXCEPT sheet XML files (we'll regenerate them).
		if zr, err := zip.OpenReader(wb.Path); err == nil {
			for _, zf := range zr.File {
				if skip[zf.Name] {
					continue
				}
				if rc, err := zf.Open(); err == nil {
					var buf bytes.Buffer
					io.Copy(&buf, rc)
					rc.Close()
					if w, err := zw.Create(zf.Name); err == nil {
						w.Write(buf.Bytes())
					}
				}
			}
			zr.Close()
		}
	} else {
		// New workbook: write all regenerated files.
		writeContentTypes(zw, wb)
		writeRels(zw)
		writeWorkbook(zw, wb)
		writeWorkbookRels(zw, wb)
		writeSharedStrings(zw, wb)
		writeStylesXML(zw, wb)
	}

	// Write regenerated sheet XML files.
	for i, sheet := range wb.Sheets {
		writeSheetXML(zw, i+1, sheet, wb)
	}

	if err := zw.Close(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("cannot replace file %q with temp: %w", path, err)
	}
	return nil
}

func writeContentTypes(zw *zip.Writer, wb *Workbook) error {
	ct := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>`
	for i := range wb.Sheets {
		ct += fmt.Sprintf("\n  <Override PartName=\"/xl/worksheets/sheet%d.xml\" ContentType=\"application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml\"/>", i+1)
	}
	if len(wb.SharedStrings) > 0 {
		ct += "\n  <Override PartName=\"/xl/sharedStrings.xml\" ContentType=\"application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml\"/>"
	}
	ct += "\n</Types>"
	return writeRawFile(zw, "[Content_Types].xml", ct)
}

func writeRels(zw *zip.Writer) error {
	rels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`
	return writeRawFile(zw, "_rels/.rels", rels)
}

func writeWorkbook(zw *zip.Writer, wb *Workbook) error {
	s := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets>`
	for i := range wb.Sheets {
		s += fmt.Sprintf("\n    <sheet name=\"%s\" sheetId=\"%d\" r:id=\"rId%d\"/>",
			xmlEscape(wb.Sheets[i].Name), i+1, i+1)
	}
	s += "\n  </sheets>\n</workbook>"
	return writeRawFile(zw, "xl/workbook.xml", s)
}

func writeWorkbookRels(zw *zip.Writer, wb *Workbook) error {
	s := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`
	for i := range wb.Sheets {
		s += fmt.Sprintf("\n  <Relationship Id=\"rId%d\" Type=\"http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet\" Target=\"worksheets/sheet%d.xml\"/>", i+1, i+1)
	}
	s += fmt.Sprintf("\n  <Relationship Id=\"rId%d\" Type=\"http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles\" Target=\"styles.xml\"/>", len(wb.Sheets)+1)
	s += "\n</Relationships>"
	return writeRawFile(zw, "xl/_rels/workbook.xml.rels", s)
}

func writeSharedStrings(zw *zip.Writer, wb *Workbook) error {
	if len(wb.SharedStrings) == 0 {
		return nil
	}
	s := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="%d" uniqueCount="%d">`,
		len(wb.SharedStrings), len(wb.SharedStrings))
	for _, ss := range wb.SharedStrings {
		s += fmt.Sprintf("\n  <si><t>%s</t></si>", xmlEscape(ss))
	}
	s += "\n</sst>"
	return writeRawFile(zw, "xl/sharedStrings.xml", s)
}

func writeStylesXML(zw *zip.Writer, wb *Workbook) error {
	sm := wb.StyleManager()

	// Fonts
	s := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <fonts count="` + strconv.Itoa(len(sm.Fonts)) + `">`
	for _, f := range sm.Fonts {
		s += "\n    <font>"
		if f.Size > 0 {
			s += fmt.Sprintf("<sz val=\"%d\"/>", f.Size)
		}
		if f.Name != "" {
			s += fmt.Sprintf("<name val=\"%s\"/>", xmlEscape(f.Name))
		}
		if f.Bold {
			s += "<b/>"
		}
		if f.Italic {
			s += "<i/>"
		}
		if f.Underline {
			s += "<u/>"
		}
		if f.Strike {
			s += "<strike/>"
		}
		if f.Color != "" {
			s += fmt.Sprintf("<color rgb=\"%s\"/>", f.Color)
		}
		s += "</font>"
	}
	s += "\n  </fonts>"

	// Fills
	s += fmt.Sprintf("\n  <fills count=\"%d\">", len(sm.Fills))
	for _, fl := range sm.Fills {
		s += "\n    <fill>"
		if fl.Pattern == "none" {
			s += `<patternFill patternType="none"/>`
		} else if fl.Pattern == "gray125" {
			s += `<patternFill patternType="gray125"/>`
		} else {
			if fl.Color != "" {
				s += fmt.Sprintf(`<patternFill patternType="solid"><fgColor rgb="%s"/></patternFill>`, fl.Color)
			} else {
				s += `<patternFill patternType="solid"/>`
			}
		}
		s += "</fill>"
	}
	s += "\n  </fills>"

	// Borders
	numBorders := len(sm.Quads)
	s += fmt.Sprintf("\n  <borders count=\"%d\">", numBorders)
	for _, q := range sm.Quads {
		s += "\n    <border>"
		for _, edge := range q {
			side := "left"
			s += fmt.Sprintf(`      <%s>`, side)
			if edge != nil {
				s += fmt.Sprintf(`<border style="%s"><color auto="1"/>`, edge.Style)
				if edge.Color != "" {
					s += fmt.Sprintf(`<color rgb="%s"/>`, edge.Color)
				}
				s += fmt.Sprintf(`</border>`)
			}
			s += fmt.Sprintf(`</%s>`, side)
		}
		s += "\n    </border>"
	}
	_ = numBorders
	s += fmt.Sprintf("\n  </borders>")

	return writeRawFile(zw, "xl/styles.xml", buildStylesXML(sm))
}

func xlColor(c string) string {
	if c == "" {
		return ""
	}
	result := c
	if result[0] == '#' {
		result = result[1:]
	}
	if len(result) == 6 {
		result = "FF" + result
	}
	return result
}

func buildStylesXML(sm *styleManager) string {
	s := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`

	if len(sm.NumFmts) > 0 {
		s += fmt.Sprintf("\n  <numFmts count=\"%d\">", len(sm.NumFmts))
		for i, nf := range sm.NumFmts {
			fmtID := 41 + i
			s += fmt.Sprintf(`<numFmt numFmtId="%d" formatCode="%s"/>`, fmtID, xmlEscape(nf))
		}
		s += "\n  </numFmts>"
	}

	s += fmt.Sprintf("\n  <fonts count=\"%d\">", len(sm.Fonts))
	for _, f := range sm.Fonts {
		s += "\n    <font>"
		if f.Size > 0 {
			s += fmt.Sprintf("<sz val=\"%d\"/>", f.Size)
		}
		if f.Name != "" {
			s += fmt.Sprintf("<name val=\"%s\"/>", xmlEscape(f.Name))
		}
		if f.Bold {
			s += "<b/>"
		}
		if f.Italic {
			s += "<i/>"
		}
		if f.Underline {
			s += "<u/>"
		}
		if f.Strike {
			s += "<strike/>"
		}
		col := xlColor(f.Color)
		if col != "" {
			s += fmt.Sprintf(`<color rgb="%s"/>`, col)
		}
		s += "</font>"
	}
	s += "\n  </fonts>"

	s += fmt.Sprintf("\n  <fills count=\"%d\">", len(sm.Fills))
	for _, fl := range sm.Fills {
		s += "\n    <fill>"
		if fl.Pattern == "none" {
			s += `<patternFill patternType="none"/>`
		} else if fl.Pattern == "gray125" {
			s += `<patternFill patternType="gray125"/>`
		} else {
			s += `<patternFill patternType="solid">`
			col := xlColor(fl.Color)
			if col != "" {
				s += fmt.Sprintf(`<fgColor rgb="%s"/>`, col)
			} else {
				s += `<fgColor auto="1"/>`
			}
			s += `</patternFill>`
		}
		s += "</fill>"
	}
	s += "\n  </fills>"

	s += fmt.Sprintf("\n  <borders count=\"%d\">", len(sm.Quads))
	for _, q := range sm.Quads {
		s += "\n    <border>"
		sides := []string{"left", "right", "top", "bottom"}
		for j, edge := range q {
			s += fmt.Sprintf("\n      <%s", sides[j])
			if edge != nil {
				s += fmt.Sprintf(` style="%s"`, edge.Style) + ">"
				col := xlColor(edge.Color)
				if col != "" {
					s += fmt.Sprintf(`<color rgb="%s"/>`, col)
				} else {
					s += `<color auto="1"/>`
				}
				s += fmt.Sprintf("</%s>", sides[j])
			} else {
				s += "/>"
			}
		}
		s += "\n    </border>"
	}
	s += "\n  </borders>"

	s += "\n  <cellStyleXfs count=\"1\"><xf numFmtId=\"0\" fontId=\"0\" fillId=\"0\" borderId=\"0\"/></cellStyleXfs>"
	s += fmt.Sprintf("\n  <cellXfs count=\"%d\">", len(sm.XFList))
	for _, xf := range sm.XFList {
		numFmtID := 0
		if xf.NumFmtID >= 0 {
			numFmtID = xf.NumFmtID
		}
		applyFmt := "0"
		if xf.NumFmtID >= 0 {
			applyFmt = "1"
		}
		applyFont := "0"
		if xf.FontID > 0 {
			applyFont = "1"
		}
		applyFill := "0"
		if xf.FillID > 0 {
			applyFill = "1"
		}
		applyBorder := "0"
		if xf.BorderID > 0 {
			applyBorder = "1"
		}
		applyAlign := "0"
		if xf.AlignID >= 0 {
			applyAlign = "1"
		}
		s += fmt.Sprintf(`<xf numFmtId="%d" fontId="%d" fillId="%d" borderId="%d" xfId="0" applyNumberFormat="%s" applyFont="%s" applyFill="%s" applyBorder="%s" applyAlignment="%s"`,
			numFmtID, xf.FontID, xf.FillID, xf.BorderID, applyFmt, applyFont, applyFill, applyBorder, applyAlign)
		if xf.AlignID >= 0 && xf.AlignID < len(sm.Aligns) {
			a := sm.Aligns[xf.AlignID]
			s += ">" + `<alignment`
			if a.Horizontal != "" {
				s += fmt.Sprintf(` horizontal="%s"`, a.Horizontal)
			}
			if a.Vertical != "" {
				s += fmt.Sprintf(` vertical="%s"`, a.Vertical)
			}
			if a.WrapText {
				s += ` wrapText="1"`
			}
			if a.Rotation > 0 {
				s += fmt.Sprintf(` textRotation="%d"`, a.Rotation)
			}
			s += `/>` + `</xf>`
		} else {
			s += `/>`
		}
	}
	s += "\n  </cellXfs>"
	s += "\n  <cellStyles count=\"1\"><cellStyle name=\"Normal\" xfId=\"0\" builtinId=\"0\"/></cellStyles>"
	s += "\n</styleSheet>"
	return s
}

func writeSheetXML(zw *zip.Writer, sheetNum int, sheet *Sheet, wb *Workbook) error {
	s := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`

	if len(sheet.ColInfos) > 0 {
		s += "\n  <cols>"
		for _, ci := range sheet.ColInfos {
			w := ci.Width
			if w == 0 {
				w = 9.0
			}
			s += fmt.Sprintf(`<col min="%d" max="%d" width="%.1f" customWidth="1"/>`, ci.Min, ci.Max, w)
		}
		s += "\n  </cols>"
	}

	s += "\n  <sheetData>"
	rowKeys := make([]int, 0, len(sheet.Rows))
	for r := range sheet.Rows {
		rowKeys = append(rowKeys, r)
	}
	sort.Ints(rowKeys)

	rhMap := make(map[int]float64)
	for _, rh := range sheet.RowHeights {
		rhMap[rh.Row] = rh.Height
	}

	for _, r := range rowKeys {
		rowCells := sheet.Rows[r]
		colKeys := make([]int, 0, len(rowCells))
		for c := range rowCells {
			colKeys = append(colKeys, c)
		}
		sort.Ints(colKeys)

		rowTag := fmt.Sprintf("\n    <row r=\"%d\"", r+1)
		if h, ok := rhMap[r]; ok {
			rowTag += fmt.Sprintf(` ht="%.1f" customHeight="1"`, h)
		}
		rowTag += ">"
		s += rowTag

		for _, c := range colKeys {
			cell := rowCells[c]
			cRef := cell.ColRef
			if cRef == "" {
				cRef = FormatCellRef(c, r)
			}
			if cell.Formula != "" {
				s += fmt.Sprintf("\n      <c r=\"%s\" s=\"%d\"><f>%s</f><v>%s</v></c>", cRef, cell.StyleID, xmlEscape(cell.Formula), xmlEscape(cell.Value))
			} else if cell.Type == "str" {
				s += fmt.Sprintf("\n      <c r=\"%s\" t=\"str\" s=\"%d\"><v>%s</v></c>", cRef, cell.StyleID, xmlEscape(cell.Value))
			} else if cell.Type == "s" {
				s += fmt.Sprintf("\n      <c r=\"%s\" t=\"s\" s=\"%d\"><v>%s</v></c>", cRef, cell.StyleID, xmlEscape(cell.Value))
			} else {
				s += fmt.Sprintf("\n      <c r=\"%s\" s=\"%d\"><v>%s</v></c>", cRef, cell.StyleID, xmlEscape(cell.Value))
			}
		}
		s += "\n    </row>"
	}
	s += "\n  </sheetData>"

	if len(sheet.MergeCells) > 0 {
		s += fmt.Sprintf("\n  <mergeCells count=\"%d\">", len(sheet.MergeCells))
		for _, mc := range sheet.MergeCells {
			ref := FormatCellRef(mc.StartCol, mc.StartRow) + ":" + FormatCellRef(mc.EndCol, mc.EndRow)
			s += fmt.Sprintf(`<mergeCell ref="%s"/>`, ref)
		}
		s += "\n  </mergeCells>"
	}

	s += "\n</worksheet>"
	return writeRawFile(zw, fmt.Sprintf("xl/worksheets/sheet%d.xml", sheetNum), s)
}

func xmlEscape(s string) string {
	var esc []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '&':
			esc = append(esc, '&', 'a', 'm', 'p', ';')
		case '<':
			esc = append(esc, '&', 'l', 't', ';')
		case '>':
			esc = append(esc, '&', 'g', 't', ';')
		case '"':
			esc = append(esc, '&', 'q', 'u', 'o', 't', ';')
		case '\'':
			esc = append(esc, '&', 'a', 'p', 'o', 's', ';')
		default:
			esc = append(esc, c)
		}
	}
	return string(esc)
}

func writeRawFile(zw *zip.Writer, name, content string) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, content)
	return err
}
