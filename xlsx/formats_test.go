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
	"os"
	"path/filepath"
	"testing"
)

func TestStyleFormat(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "xlsx-format-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "styled.xlsx")
	wb := &Workbook{Path: testFile}
	wb.CreateSheet("Sheet1")

	// Write data
	wb.SetCellValue(0, 0, 0, "Header")
	wb.SetCellValue(0, 1, 0, "Value")
	wb.SetCellValue(0, 0, 1, "Item1")
	wb.SetCellValue(0, 1, 1, "100")

	// Apply styles
	sm := wb.StyleManager()

	// Style 1: header with bold, center alignment, yellow fill, thin border
	cs1 := CellStyle{
		Font:      &FontStyle{Bold: true, Name: "Arial", Size: 12, Color: "#FFFFFF"},
		Fill:      &FillStyle{FgColor: "#4472C4", PatternType: "solid"},
		Alignment: &AlignmentStyle{Horizontal: "center", Vertical: "center"},
		Border: &BorderStyle{
			Top:    &BorderEdge{Style: "thin"},
			Bottom: &BorderEdge{Style: "thin"},
			Left:   &BorderEdge{Style: "thin"},
			Right:  &BorderEdge{Style: "thin"},
		},
	}
	s1 := sm.addStyle(cs1)

	// Style 2: data with center alignment, light blue fill
	cs2 := CellStyle{
		Font:      &FontStyle{Size: 11},
		Fill:      &FillStyle{FgColor: "#D6E4F0", PatternType: "solid"},
		Alignment: &AlignmentStyle{Horizontal: "center"},
	}
	s2 := sm.addStyle(cs2)

	// Apply styles to cells
	wb.Sheets[0].SetCellStyle(CellRange{StartRow: 0, EndRow: 0, StartCol: 0, EndCol: 1}, s1)
	wb.Sheets[0].SetCellStyle(CellRange{StartRow: 1, EndRow: 1, StartCol: 0, EndCol: 1}, s2)

	// Set column widths
	wb.Sheets[0].SetColWidth(0, 15)
	wb.Sheets[0].SetColWidth(1, 12)

	// Set row height for header
	wb.Sheets[0].SetRowHeight(0, 25)

	if err := wb.Save(); err != nil {
		t.Fatal(err)
	}

	stat, _ := os.Stat(testFile)
	t.Logf("Styled file size: %d bytes", stat.Size())
}

func TestMergeUnmerge(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "xlsx-merge-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "merged.xlsx")
	wb := &Workbook{Path: testFile}
	wb.CreateSheet("Sheet1")

	wb.SetCellValue(0, 0, 0, "Merged Title")
	wb.Sheets[0].AddMergeCell(CellRange{StartRow: 0, EndRow: 0, StartCol: 0, EndCol: 3})

	wb.SetCellValue(0, 1, 0, "A")
	wb.SetCellValue(0, 2, 0, "B")

	if err := wb.Save(); err != nil {
		t.Fatal(err)
	}

	stat, _ := os.Stat(testFile)
	t.Logf("Merged file size: %d bytes", stat.Size())

	// Re-read and check
	wb2, err := OpenFile(testFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(wb2.Sheets) != 1 {
		t.Fatalf("expected 1 sheet, got %d", len(wb2.Sheets))
	}
	t.Logf("Re-read OK: sheet=%s", wb2.Sheets[0].Name)
}

func TestStyleCache(t *testing.T) {
	sm := newStyleManager()

	cs := CellStyle{
		Font: &FontStyle{Bold: true, Size: 14, Name: "Arial", Color: "#FF0000"},
		Fill: &FillStyle{FgColor: "#FFFF00", PatternType: "solid"},
	}
	s1 := sm.addStyle(cs)
	s2 := sm.addStyle(cs)
	if s1 != s2 {
		t.Fatalf("expected same style index %d, got %d", s1, s2)
	}
	t.Logf("Style cache works: %d == %d", s1, s2)

	// Different style should get different index
	cs2 := CellStyle{
		Fill: &FillStyle{FgColor: "#0000FF", PatternType: "solid"},
	}
	s3 := sm.addStyle(cs2)
	if s3 <= s2 {
		t.Fatalf("expected different index, s3=%d s2=%d", s3, s2)
	}
	t.Logf("Different style gets different index: %d != %d", s3, s2)
}
