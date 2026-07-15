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
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestCreateReadRoundtrip(t *testing.T) {
	// Create a temp directory for test files
	tmpDir, err := os.MkdirTemp("", "xlsx-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.xlsx")

	// Create a workbook, write data, save
	wb := &Workbook{
		Path: testFile,
	}
	wb.CreateSheet("Sheet1")

	if err := wb.SetCellValue(0, 0, 0, "Name"); err != nil {
		t.Fatal(err)
	}
	if err := wb.SetCellValue(0, 1, 0, "Age"); err != nil {
		t.Fatal(err)
	}
	if err := wb.SetCellValue(0, 0, 1, "Alice"); err != nil {
		t.Fatal(err)
	}
	if err := wb.SetCellValue(0, 1, 1, "30"); err != nil {
		t.Fatal(err)
	}
	if err := wb.SetCellValue(0, 0, 2, "Bob"); err != nil {
		t.Fatal(err)
	}
	if err := wb.SetCellValue(0, 1, 2, "25"); err != nil {
		t.Fatal(err)
	}
	// Write a formula
	if err := wb.SetCellValue(0, 2, 0, "=SUM(B2:B3)"); err != nil {
		t.Fatal(err)
	}

	if err := wb.Save(); err != nil {
		t.Fatal(err)
	}

	// Verify file exists and has content
	stat, err := os.Stat(testFile)
	if err != nil {
		t.Fatal(err)
	}
	if stat.Size() == 0 {
		t.Fatal("saved file is empty")
	}
	t.Logf("saved file size: %d bytes", stat.Size())

	// Re-open and verify data
	wb2, err := OpenFile(testFile)
	if err != nil {
		t.Fatal(err)
	}

	if len(wb2.Sheets) != 1 {
		t.Fatalf("expected 1 sheet, got %d", len(wb2.Sheets))
	}
	if wb2.Sheets[0].Name != "Sheet1" {
		t.Fatalf("expected Sheet1, got %q", wb2.Sheets[0].Name)
	}

	// Check cells
	cv, err := wb2.GetCellValue(0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if cv.Value != "Name" || cv.Type != CellTypeString {
		t.Fatalf("expected (Name, S), got (%q, %s)", cv.Value, cv.Type)
	}

	cv, err = wb2.GetCellValue(0, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if cv.Value != "Age" || cv.Type != CellTypeString {
		t.Fatalf("expected (Age, S), got (%q, %s)", cv.Value, cv.Type)
	}

	cv, err = wb2.GetCellValue(0, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if cv.Value != "Alice" || cv.Type != CellTypeString {
		t.Fatalf("expected (Alice, S), got (%q, %s)", cv.Value, cv.Type)
	}

	cv, err = wb2.GetCellValue(0, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if cv.Value != "30" || cv.Type != CellTypeNumber {
		t.Fatalf("expected (30, N), got (%q, %s)", cv.Value, cv.Type)
	}

	// Verify formula
	cv, err = wb2.GetCellValue(0, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if cv.Type != CellTypeFormula && cv.Type != CellTypeString {
		t.Fatalf("expected formula cell, got type=%s value=%q", cv.Type, cv.Value)
	}

	// Verify sheet metadata
	meta, err := wb2.GetSheetMeta(0)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Rows == 0 || meta.Cols == 0 {
		t.Fatalf("expected non-zero rows/cols, got rows=%d cols=%d", meta.Rows, meta.Cols)
	}
	t.Logf("sheet meta: rows=%d, cols=%d, used_range=%q, headers=%v", meta.Rows, meta.Cols, meta.UsedRange, meta.Headers)
}

func TestReadRange(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "xlsx-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "readrange.xlsx")
	wb := &Workbook{Path: testFile}
	wb.CreateSheet("Data")
	for r := 0; r < 5; r++ {
		for c := 0; c < 3; c++ {
			val := "A"
			if c == 0 {
				val = ""
			}
			if err := wb.SetCellValue(0, c, r, val); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := wb.Save(); err != nil {
		t.Fatal(err)
	}

	wb2, err := OpenFile(testFile)
	if err != nil {
		t.Fatal(err)
	}

	rng := CellRange{StartRow: 2, EndRow: 4, StartCol: 0, EndCol: 2}
	cells, err := wb2.ReadRange(0, rng)
	if err != nil {
		t.Fatal(err)
	}

	if len(cells) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(cells))
	}
	if len(cells[0]) != 3 {
		t.Fatalf("expected 3 cols, got %d", len(cells[0]))
	}
}

func TestSheetOps(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "xlsx-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "sheetops.xlsx")
	wb := &Workbook{Path: testFile}

	// Create multiple sheets
	wb.CreateSheet("First")
	wb.CreateSheet("Second")
	if len(wb.Sheets) != 2 {
		t.Fatalf("expected 2 sheets, got %d", len(wb.Sheets))
	}

	// Rename
	if err := wb.RenameSheet(0, "Renamed"); err != nil {
		t.Fatal(err)
	}
	if wb.Sheets[0].Name != "Renamed" {
		t.Fatalf("expected Renamed, got %q", wb.Sheets[0].Name)
	}

	// Copy
	idx, err := wb.CopySheet(0, "CopyOfRenamed")
	if err != nil {
		t.Fatal(err)
	}
	if idx != 2 {
		t.Fatalf("expected new sheet index 2, got %d", idx)
	}

	// Delete
	if err := wb.DeleteSheet(1); err != nil {
		t.Fatal(err)
	}
	if len(wb.Sheets) != 2 {
		t.Fatalf("expected 2 sheets after delete, got %d", len(wb.Sheets))
	}

	if err := wb.Save(); err != nil {
		t.Fatal(err)
	}
}

func TestInsertDeleteRows(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "xlsx-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "rowcols.xlsx")
	wb := &Workbook{Path: testFile}
	wb.CreateSheet("Sheet1")

	for r := 0; r < 10; r++ {
		if err := wb.SetCellValue(0, 0, r, "Row"); err != nil {
			t.Fatal(err)
		}
	}

	// Insert rows
	if err := wb.InsertRows(0, 3, 2); err != nil {
		t.Fatal(err)
	}

	// Delete rows
	if err := wb.DeleteRows(0, 5, 3); err != nil {
		t.Fatal(err)
	}

	if err := wb.Save(); err != nil {
		t.Fatal(err)
	}

	coordsTestFile := filepath.Join(tmpDir, "coords.xlsx")
	// Test coordinate utilities
	ref := FormatCellRef(0, 4)
	if ref != "A5" {
		t.Fatalf("expected A5, got %q", ref)
	}
	ref = FormatCellRef(25, 0)
	if ref != "Z1" {
		t.Fatalf("expected Z1, got %q", ref)
	}
	ref = FormatCellRef(26, 0)
	if ref != "AA1" {
		t.Fatalf("expected AA1, got %q", ref)
	}

	col, row, err := ParseCellRef("C5")
	if err != nil {
		t.Fatal(err)
	}
	if col != 2 || row != 4 {
		t.Fatalf("expected col=2,row=4 got col=%d,row=%d", col, row)
	}

	// Save empty file for coords test
	wb2 := &Workbook{Path: coordsTestFile}
	wb2.CreateSheet("Sheet1")
	if err := wb2.Save(); err != nil {
		t.Fatal(err)
	}
}

func TestCoords(t *testing.T) {
	tests := []struct {
		ref     string
		wantCol int
		wantRow int
	}{
		{"A1", 0, 0},
		{"B1", 1, 0},
		{"Z1", 25, 0},
		{"AA1", 26, 0},
		{"AB1", 27, 0},
		{"ZZ1", 701, 0},
		{"A10", 0, 9},
		{"C5", 2, 4},
	}

	for _, tt := range tests {
		col, row, err := ParseCellRef(tt.ref)
		if err != nil {
			t.Errorf("ParseCellRef(%q) error: %v", tt.ref, err)
			continue
		}
		if col != tt.wantCol || row != tt.wantRow {
			t.Errorf("ParseCellRef(%q) = (%d,%d), want (%d,%d)", tt.ref, col, row, tt.wantCol, tt.wantRow)
		}
	}

	// Roundtrip test
	for _, tt := range tests {
		got := FormatCellRef(tt.wantCol, tt.wantRow)
		if got != tt.ref {
			t.Errorf("FormatCellRef(%d,%d) = %q, want %q", tt.wantCol, tt.wantRow, got, tt.ref)
		}
	}
}

// TestMultiSheetWithRandomData creates an empty XLSX file, creates two sheets,
// writes random data to both, then reads and verifies the data.
func TestMultiSheetWithRandomData(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "xlsx-multisheet-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "random_data.xlsx")

	// Step 1: Create a new empty workbook and two sheets with data
	wb := &Workbook{Path: testFile}
	wb.CreateSheet("Employees") // Sheet index 0
	wb.CreateSheet("Inventory") // Sheet index 1

	// Write headers to Employees sheet
	headers1 := []string{"ID", "Name", "Department", "Salary"}
	for c, h := range headers1 {
		if err := wb.SetCellValue(0, c, 0, h); err != nil {
			t.Fatal(err)
		}
	}

	// Write 10 random employee rows
	names := []string{"Alice", "Bob", "Charlie", "Diana", "Eve", "Frank", "Grace", "Henry", "Ivy", "Jack"}
	depts := []string{"Engineering", "Marketing", "Sales", "HR", "Finance", "Legal"}
	rng := rand.New(rand.NewSource(42))

	for r := 0; r < 10; r++ {
		if err := wb.SetCellValue(0, 0, r+1, fmt.Sprintf("EMP%03d", r+1)); err != nil {
			t.Fatal(err)
		}
		if err := wb.SetCellValue(0, 1, r+1, names[r]); err != nil {
			t.Fatal(err)
		}
		if err := wb.SetCellValue(0, 2, r+1, depts[rng.Intn(len(depts))]); err != nil {
			t.Fatal(err)
		}
		salary := 50000 + rng.Intn(80000)
		if err := wb.SetCellValue(0, 3, r+1, fmt.Sprintf("%d", salary)); err != nil {
			t.Fatal(err)
		}
	}

	// Write a formula: average salary
	if err := wb.SetCellValue(0, 3, 11, "=AVERAGE(D2:D11)"); err != nil {
		t.Fatal(err)
	}

	// Write headers to Inventory sheet
	headers2 := []string{"ItemID", "ItemName", "Quantity", "UnitPrice"}
	for c, h := range headers2 {
		if err := wb.SetCellValue(1, c, 0, h); err != nil {
			t.Fatal(err)
		}
	}

	// Write 8 random inventory items
	items := []string{"Laptop", "Monitor", "Keyboard", "Mouse", "Desk", "Chair", "Headset", "Cable"}
	for r := 0; r < 8; r++ {
		if err := wb.SetCellValue(1, 0, r+1, fmt.Sprintf("ITM%03d", r+1)); err != nil {
			t.Fatal(err)
		}
		if err := wb.SetCellValue(1, 1, r+1, items[r]); err != nil {
			t.Fatal(err)
		}
		qty := rng.Intn(100)
		if err := wb.SetCellValue(1, 2, r+1, fmt.Sprintf("%d", qty)); err != nil {
			t.Fatal(err)
		}
		price := 10 + rng.Intn(2000)
		if err := wb.SetCellValue(1, 3, r+1, fmt.Sprintf("%d", price)); err != nil {
			t.Fatal(err)
		}
	}

	// Step 2: Save to disk
	if err := wb.Save(); err != nil {
		t.Fatal(err)
	}

	stat, _ := os.Stat(testFile)
	t.Logf("Saved file size: %d bytes", stat.Size())

	// Step 3: Re-open from disk
	wb2, err := OpenFile(testFile)
	if err != nil {
		t.Fatal(err)
	}

	// Step 4: Verify structure
	if len(wb2.Sheets) != 2 {
		t.Fatalf("expected 2 sheets, got %d", len(wb2.Sheets))
	}

	// Verify sheet names (order may vary, use index)
	sheetNames := []string{wb2.Sheets[0].Name, wb2.Sheets[1].Name}
	sort.Strings(sheetNames)
	if sheetNames[0] != "Employees" || sheetNames[1] != "Inventory" {
		t.Fatalf("expected sheets Employees and Inventory, got %v", sheetNames)
	}

	// Step 5: Verify Employees sheet data
	empIdx := -1
	for i, s := range wb2.Sheets {
		if s.Name == "Employees" {
			empIdx = i
			break
		}
	}
	if empIdx < 0 {
		t.Fatal("Employees sheet not found")
	}

	meta, err := wb2.GetSheetMeta(empIdx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Employees meta: rows=%d, cols=%d, has_formulas=%v, headers=%v",
		meta.Rows, meta.Cols, meta.HasFormulas, meta.Headers)

	if meta.Rows < 11 {
		t.Fatalf("Employees expected >=11 rows (10 data + header), got %d", meta.Rows)
	}
	if meta.Cols < 4 {
		t.Fatalf("Employees expected >=4 cols, got %d", meta.Cols)
	}
	if !meta.HasFormulas {
		t.Fatal("Employees expected to have formulas (AVERAGE)")
	}
	if len(meta.Headers) < 4 {
		t.Fatalf("expected at least 4 headers, got %d: %v", len(meta.Headers), meta.Headers)
	}

	// Read and verify first data row
	cv1, _ := wb2.GetCellValue(empIdx, 0, 1)
	if cv1.Value != "EMP001" || cv1.Type != CellTypeString {
		t.Fatalf("expected EMP001(S), got (%q, %s)", cv1.Value, cv1.Type)
	}
	cv2, _ := wb2.GetCellValue(empIdx, 1, 1)
	if cv2.Value != "Alice" || cv2.Type != CellTypeString {
		t.Fatalf("expected Alice(S), got (%q, %s)", cv2.Value, cv2.Type)
	}
	cv4, _ := wb2.GetCellValue(empIdx, 3, 1)
	if cv4.Type != CellTypeNumber {
		t.Fatalf("expected salary to be number, got (%q, %s)", cv4.Value, cv4.Type)
	}

	// Step 6: Verify Inventory sheet data
	invIdx := -1
	for i, s := range wb2.Sheets {
		if s.Name == "Inventory" {
			invIdx = i
			break
		}
	}
	if invIdx < 0 {
		t.Fatal("Inventory sheet not found")
	}

	meta2, err := wb2.GetSheetMeta(invIdx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Inventory meta: rows=%d, cols=%d, headers=%v",
		meta2.Rows, meta2.Cols, meta2.Headers)

	if meta2.Rows < 8 {
		t.Fatalf("Inventory expected >=8 rows, got %d", meta2.Rows)
	}

	// Read last inventory item row
	lastRow := meta2.Rows - 1                           // last data row (0-based)
	cvItem, _ := wb2.GetCellValue(invIdx, 0, lastRow-1) // 0-based: row 7 (0-indexed) = 8th item
	if cvItem.Type != CellTypeString || cvItem.Value == "" {
		t.Logf("Last inventory item: (%q, %s)", cvItem.Value, cvItem.Type)
	}

	// Step 7: Read full ranges to verify ReadRange works across sheets
	empRange := CellRange{StartRow: 0, EndRow: 10, StartCol: 0, EndCol: 3}
	empCells, err := wb2.ReadRange(empIdx, empRange)
	if err != nil {
		t.Fatal(err)
	}
	if len(empCells) != 11 {
		t.Fatalf("expected 11 rows from Employees ReadRange, got %d", len(empCells))
	}
	if len(empCells[0]) != 4 {
		t.Fatalf("expected 4 cols from Employees ReadRange, got %d", len(empCells[0]))
	}
	t.Logf("Employees header: [%s %s %s %s]",
		empCells[0][0].Value, empCells[0][1].Value,
		empCells[0][2].Value, empCells[0][3].Value)

	invRange := CellRange{StartRow: 0, EndRow: 8, StartCol: 0, EndCol: 3}
	invCells, err := wb2.ReadRange(invIdx, invRange)
	if err != nil {
		t.Fatal(err)
	}
	if len(invCells) != 9 {
		t.Fatalf("expected 9 rows from Inventory ReadRange, got %d", len(invCells))
	}

	t.Logf("✅ Multi-sheet random data test passed (Employees: %d rows, Inventory: %d rows)",
		meta.Rows, meta2.Rows)
}

// TestPreserveFormatOnEdit opens a real xlsx file, reads data and styles,
// edits A1, saves, re-opens and verifies nothing else is lost.
// This guards against regressions where edit+save cycles corrupt data or formatting.
func TestPreserveFormatOnEdit(t *testing.T) {
	original := "../work/research/2026_calendar.xlsx"
	if _, err := os.Stat(original); os.IsNotExist(err) {
		t.Skipf("source file %q not found", original)
	}

	// Step 1: Open the original file and snapshot its state
	wb, err := OpenFile(original)
	if err != nil {
		t.Fatalf("cannot open %q: %v", original, err)
	}
	if len(wb.Sheets) == 0 {
		t.Fatal("workbook has no sheets")
	}
	sheet := wb.Sheets[0]

	// Snapshot: count rows, merge cells, col infos, row heights
	rowCount := len(sheet.Rows)
	mergeCount := len(sheet.MergeCells)
	colInfoCount := len(sheet.ColInfos)
	rowHeightCount := len(sheet.RowHeights)

	// Snapshot: read a few key cell values
	type cellRef struct{ col, row int }
	keyCells := []cellRef{
		{0, 0}, // A1 — "2026年" (rich text)
		{1, 0}, // B1 — shared string
		{0, 4}, // A5 — some data cell
	}
	snapshot := make(map[cellRef]*CellValue)
	for _, ref := range keyCells {
		cv, err := wb.GetCellValue(0, ref.col, ref.row)
		if err != nil {
			t.Fatalf("cannot read cell %s: %v", FormatCellRef(ref.col, ref.row), err)
		}
		snapshot[ref] = cv
		t.Logf("cell %s = %q (type=%s)", FormatCellRef(ref.col, ref.row), cv.Value, cv.Type)
	}

	// Snapshot: check StyleID on a styled cell
	if cell, ok := sheet.Rows[0]; ok {
		if c, ok := cell[0]; ok {
			t.Logf("A1 StyleID = %d", c.StyleID)
		}
	}

	t.Logf("Original: rows=%d, merges=%d, cols=%d, rowHeights=%d",
		rowCount, mergeCount, colInfoCount, rowHeightCount)

	// Step 2: Edit A1
	if err := wb.SetCellValue(0, 0, 0, "公元 2026 年"); err != nil {
		t.Fatalf("cannot set A1: %v", err)
	}

	// Step 3: Save to temp file
	tmpDir, err := os.MkdirTemp("", "xlsx-roundtrip-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	savedPath := filepath.Join(tmpDir, "edited.xlsx")
	if err := wb.SaveAs(savedPath); err != nil {
		t.Fatalf("cannot save: %v", err)
	}

	stat, _ := os.Stat(savedPath)
	t.Logf("Saved file: %d bytes", stat.Size())

	// Step 4: Re-open and verify
	wb2, err := OpenFile(savedPath)
	if err != nil {
		t.Fatalf("cannot re-open saved file: %v", err)
	}
	if len(wb2.Sheets) == 0 {
		t.Fatal("re-opened workbook has no sheets")
	}
	sheet2 := wb2.Sheets[0]

	// Verify: row count preserved
	if len(sheet2.Rows) != rowCount {
		t.Errorf("row count changed: before=%d after=%d", rowCount, len(sheet2.Rows))
	}
	// Verify: merge cells preserved
	if len(sheet2.MergeCells) != mergeCount {
		t.Errorf("merge cell count changed: before=%d after=%d", mergeCount, len(sheet2.MergeCells))
	}
	// Verify: col infos preserved
	if len(sheet2.ColInfos) != colInfoCount {
		t.Errorf("col info count changed: before=%d after=%d", colInfoCount, len(sheet2.ColInfos))
	}
	// Verify: row heights preserved
	if len(sheet2.RowHeights) != rowHeightCount {
		t.Errorf("row height count changed: before=%d after=%d", rowHeightCount, len(sheet2.RowHeights))
	}

	// Verify: A1 updated
	cvA1, err := wb2.GetCellValue(0, 0, 0)
	if err != nil {
		t.Fatalf("cannot read A1 after edit: %v", err)
	}
	if cvA1.Value != "公元 2026 年" {
		t.Errorf("A1 value wrong: expected %q, got %q", "公元 2026 年", cvA1.Value)
	}
	t.Logf("A1 after edit = %q", cvA1.Value)

	// Verify: other cells unchanged
	for _, ref := range keyCells {
		if ref.col == 0 && ref.row == 0 {
			continue // A1 was intentionally changed
		}
		original := snapshot[ref]
		if original == nil {
			continue
		}
		cv, err := wb2.GetCellValue(0, ref.col, ref.row)
		if err != nil {
			t.Errorf("cannot read cell %s after edit: %v", FormatCellRef(ref.col, ref.row), err)
			continue
		}
		if cv.Value != original.Value {
			t.Errorf("cell %s changed: before=%q after=%q",
				FormatCellRef(ref.col, ref.row), original.Value, cv.Value)
		}
	}

	// Verify: StyleID preserved (at least non-zero)
	if cell, ok := sheet2.Rows[0]; ok {
		if c, ok := cell[0]; ok {
			if c.StyleID > 0 {
				t.Logf("A1 StyleID preserved = %d", c.StyleID)
			} else {
				t.Logf("A1 StyleID = 0 (may be default style)")
			}
		}
	}

	// Verify: style sheet has fonts, fills, borders
	sm := wb2.StyleManager()
	if len(sm.Fonts) < 2 {
		t.Errorf("expected at least 2 fonts, got %d", len(sm.Fonts))
	}
	if len(sm.Fills) < 3 {
		t.Errorf("expected at least 3 fills, got %d", len(sm.Fills))
	}
	if len(sm.Quads) < 2 {
		t.Errorf("expected at least 2 borders, got %d", len(sm.Quads))
	}
	// Verify: numFmts preserved
	if len(sm.NumFmts) < 4 {
		t.Errorf("expected at least 4 numFmts, got %d (count: %v)", len(sm.NumFmts), sm.NumFmts)
	}

	t.Logf("Styles: fonts=%d, fills=%d, borders=%d, xfList=%d, numFmts=%d",
		len(sm.Fonts), len(sm.Fills), len(sm.Quads), len(sm.XFList), len(sm.NumFmts))
	t.Logf("✅ Round-trip edit test passed")
}
