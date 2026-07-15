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
	"strconv"
)

// GetCellValue returns the parsed value of a cell at (col, row) in the given sheet.
// col and row are 0-based. Returns the displayed value (string resolved from shared strings table).
func (wb *Workbook) GetCellValue(sheetIndex int, col, row int) (*CellValue, error) {
	if sheetIndex < 0 || sheetIndex >= len(wb.Sheets) {
		return nil, fmt.Errorf("sheet index %d out of range (0-%d)", sheetIndex, len(wb.Sheets)-1)
	}
	sheet := wb.Sheets[sheetIndex]

	rowCells, ok := sheet.Rows[row]
	if !ok {
		return &CellValue{
			Row: row, Col: col,
			ColRef: FormatCellRef(col, row),
			Type:   CellTypeEmpty,
			Value:  "",
		}, nil
	}

	cell, ok := rowCells[col]
	if !ok {
		return &CellValue{
			Row: row, Col: col,
			ColRef: FormatCellRef(col, row),
			Type:   CellTypeEmpty,
			Value:  "",
		}, nil
	}

	return cellToCellValue(wb, cell, col, row), nil
}

// SetCellValue sets a cell at (col, row) in the given sheet.
// If the value starts with "=", it is stored as a formula.
func (wb *Workbook) SetCellValue(sheetIndex int, col, row int, value string) error {
	if sheetIndex < 0 || sheetIndex >= len(wb.Sheets) {
		return fmt.Errorf("sheet index %d out of range (0-%d)", sheetIndex, len(wb.Sheets)-1)
	}
	sheet := wb.Sheets[sheetIndex]

	cellType := ""
	cellValue := value
	cellFormula := ""

	if len(value) > 0 && value[0] == '=' {
		// Formula
		cellFormula = value[1:]
		cellType = ""
		// For formulas, we also keep the formula text as the display value
		cellValue = ""
	} else {
		// Check if it looks like a number
		if _, err := strconv.ParseFloat(value, 64); err == nil {
			cellType = ""
		} else if value == "TRUE" || value == "FALSE" {
			cellType = "b"
		} else {
			// Store as inline string (not shared string for simplicity)
			cellType = "str"
		}
	}

	if sheet.Rows[row] == nil {
		sheet.Rows[row] = make(map[int]*Cell)
	}

	// Preserve existing StyleID if cell already has one
	styleID := 0
	if existing, ok := sheet.Rows[row][col]; ok {
		styleID = existing.StyleID
	}

	sheet.Rows[row][col] = &Cell{
		ColRef:  FormatCellRef(col, row),
		Col:     col,
		Row:     row,
		Type:    cellType,
		Value:   cellValue,
		Formula: cellFormula,
		StyleID: styleID,
	}

	if row > sheet.MaxRow {
		sheet.MaxRow = row
	}
	if col > sheet.MaxCol {
		sheet.MaxCol = col
	}
	if cellFormula != "" {
		sheet.HasFormulas = true
	}

	return nil
}

// ReadRange reads a rectangular range of cells and returns them as a 2D slice.
// The result is [row][col] where row 0 = start_row, col 0 = start_col.
func (wb *Workbook) ReadRange(sheetIndex int, rng CellRange) ([][]*CellValue, error) {
	if sheetIndex < 0 || sheetIndex >= len(wb.Sheets) {
		return nil, fmt.Errorf("sheet index %d out of range", sheetIndex)
	}

	rows := rng.EndRow - rng.StartRow + 1
	cols := rng.EndCol - rng.StartCol + 1

	if rows <= 0 || cols <= 0 {
		return nil, fmt.Errorf("invalid range: rows=%d cols=%d", rows, cols)
	}

	result := make([][]*CellValue, rows)
	for r := 0; r < rows; r++ {
		result[r] = make([]*CellValue, cols)
		for c := 0; c < cols; c++ {
			cv, err := wb.GetCellValue(sheetIndex, rng.StartCol+c, rng.StartRow+r)
			if err != nil {
				// Fill with empty cell on error
				cv = &CellValue{
					Row:    rng.StartRow + r,
					Col:    rng.StartCol + c,
					ColRef: FormatCellRef(rng.StartCol+c, rng.StartRow+r),
					Type:   CellTypeEmpty,
					Value:  "",
				}
			}
			result[r][c] = cv
		}
	}

	return result, nil
}

// WriteRange writes a 2D slice of values starting at (startCol, startRow).
func (wb *Workbook) WriteRange(sheetIndex int, startCol, startRow int, values [][]string) error {
	for r, row := range values {
		for c, val := range row {
			if err := wb.SetCellValue(sheetIndex, startCol+c, startRow+r, val); err != nil {
				return fmt.Errorf("error writing cell [%d,%d]: %w", startRow+r, startCol+c, err)
			}
		}
	}
	return nil
}

// WriteRangeCellValues writes a 2D slice of CellValue starting at (startCol, startRow).
func (wb *Workbook) WriteRangeCellValues(sheetIndex int, startCol, startRow int, values [][]*CellValue) error {
	for r, row := range values {
		for c, cv := range row {
			if cv != nil {
				if err := wb.SetCellValue(sheetIndex, startCol+c, startRow+r, cv.Value); err != nil {
					return fmt.Errorf("error writing cell [%d,%d]: %w", startRow+r, startCol+c, err)
				}
			}
		}
	}
	return nil
}

// InsertRows inserts `count` empty rows at `position` (0-based):
// rows at position and below are shifted down.
func (wb *Workbook) InsertRows(sheetIndex, position, count int) error {
	if sheetIndex < 0 || sheetIndex >= len(wb.Sheets) {
		return fmt.Errorf("sheet index %d out of range", sheetIndex)
	}
	sheet := wb.Sheets[sheetIndex]

	if position < 0 {
		position = 0
	}

	// Collect all row keys >= position, sorted descending
	rowKeys := make([]int, 0)
	for r := range sheet.Rows {
		if r >= position {
			rowKeys = append(rowKeys, r)
		}
	}
	// Sort descending so we don't overwrite while shifting
	for i := 0; i < len(rowKeys); i++ {
		for j := i + 1; j < len(rowKeys); j++ {
			if rowKeys[j] > rowKeys[i] {
				rowKeys[i], rowKeys[j] = rowKeys[j], rowKeys[i]
			}
		}
	}

	for _, r := range rowKeys {
		sheet.Rows[r+count] = sheet.Rows[r]
		// Update Row and ColRef on all cells in the shifted row
		if cells, ok := sheet.Rows[r+count]; ok {
			for _, cell := range cells {
				cell.Row += count
				cell.ColRef = FormatCellRef(cell.Col, cell.Row)
			}
		}
		delete(sheet.Rows, r)
	}

	sheet.MaxRow += count
	return nil
}

// DeleteRows removes `count` rows starting from `position` (0-based).
// Rows below are shifted up.
func (wb *Workbook) DeleteRows(sheetIndex, position, count int) error {
	if sheetIndex < 0 || sheetIndex >= len(wb.Sheets) {
		return fmt.Errorf("sheet index %d out of range", sheetIndex)
	}
	sheet := wb.Sheets[sheetIndex]

	if position < 0 {
		position = 0
	}

	// Delete rows in range [position, position+count)
	for r := position; r < position+count; r++ {
		delete(sheet.Rows, r)
	}

	// Shift remaining rows up
	rowKeys := make([]int, 0)
	for r := range sheet.Rows {
		if r >= position+count {
			rowKeys = append(rowKeys, r)
		}
	}
	// Sort ascending
	for i := 0; i < len(rowKeys); i++ {
		for j := i + 1; j < len(rowKeys); j++ {
			if rowKeys[j] < rowKeys[i] {
				rowKeys[i], rowKeys[j] = rowKeys[j], rowKeys[i]
			}
		}
	}

	for _, r := range rowKeys {
		sheet.Rows[r-count] = sheet.Rows[r]
		delete(sheet.Rows, r)
	}

	sheet.MaxRow -= count
	if sheet.MaxRow < 0 {
		sheet.MaxRow = 0
	}
	return nil
}

// InsertCols inserts `count` empty columns at `position` (0-based):
// columns at position and right are shifted right.
func (wb *Workbook) InsertCols(sheetIndex, position, count int) error {
	if sheetIndex < 0 || sheetIndex >= len(wb.Sheets) {
		return fmt.Errorf("sheet index %d out of range", sheetIndex)
	}
	sheet := wb.Sheets[sheetIndex]

	if position < 0 {
		position = 0
	}

	for _, rowCells := range sheet.Rows {
		colKeys := make([]int, 0)
		for c := range rowCells {
			if c >= position {
				colKeys = append(colKeys, c)
			}
		}
		// Sort descending
		for i := 0; i < len(colKeys); i++ {
			for j := i + 1; j < len(colKeys); j++ {
				if colKeys[j] > colKeys[i] {
					colKeys[i], colKeys[j] = colKeys[j], colKeys[i]
				}
			}
		}
		for _, c := range colKeys {
			cell := rowCells[c]
			cell.Col += count
			cell.ColRef = FormatCellRef(cell.Col, cell.Row)
			rowCells[c+count] = cell
			delete(rowCells, c)
		}
	}

	sheet.MaxCol += count
	return nil
}

// DeleteCols removes `count` columns starting from `position` (0-based).
// Columns to the right are shifted left.
func (wb *Workbook) DeleteCols(sheetIndex, position, count int) error {
	if sheetIndex < 0 || sheetIndex >= len(wb.Sheets) {
		return fmt.Errorf("sheet index %d out of range", sheetIndex)
	}
	sheet := wb.Sheets[sheetIndex]

	if position < 0 {
		position = 0
	}

	for _, rowCells := range sheet.Rows {
		// Delete cells in range
		for c := position; c < position+count; c++ {
			delete(rowCells, c)
		}
		// Shift remaining cells left
		colKeys := make([]int, 0)
		for c := range rowCells {
			if c >= position+count {
				colKeys = append(colKeys, c)
			}
		}
		// Sort ascending
		for i := 0; i < len(colKeys); i++ {
			for j := i + 1; j < len(colKeys); j++ {
				if colKeys[j] < colKeys[i] {
					colKeys[i], colKeys[j] = colKeys[j], colKeys[i]
				}
			}
		}
		for _, c := range colKeys {
			cell := rowCells[c]
			cell.Col -= count
			cell.ColRef = FormatCellRef(cell.Col, cell.Row)
			rowCells[c-count] = cell
			delete(rowCells, c)
		}
	}

	sheet.MaxCol -= count
	if sheet.MaxCol < 0 {
		sheet.MaxCol = 0
	}
	return nil
}

// GetSheetMeta returns metadata about a sheet without returning cell data.
func (wb *Workbook) GetSheetMeta(sheetIndex int) (*SheetMeta, error) {
	if sheetIndex < 0 || sheetIndex >= len(wb.Sheets) {
		return nil, fmt.Errorf("sheet index %d out of range", sheetIndex)
	}
	sheet := wb.Sheets[sheetIndex]

	meta := &SheetMeta{
		Index:       sheetIndex,
		Name:        sheet.Name,
		Rows:        sheet.MaxRow + 1,
		Cols:        sheet.MaxCol + 1,
		HasFormulas: sheet.HasFormulas,
	}
	if sheet.MaxRow >= 0 && sheet.MaxCol >= 0 {
		meta.UsedRange = FormatCellRef(0, 0) + ":" + FormatCellRef(sheet.MaxCol, sheet.MaxRow)
	}

	// Try to detect headers: look at first few rows and find the first with content
	if sheet.MaxRow >= 0 {
		headers := make([]string, 0)
		for c := 0; c <= sheet.MaxCol; c++ {
			cv, err := wb.GetCellValue(sheetIndex, c, 0)
			if err == nil && cv.Type != CellTypeEmpty && cv.Value != "" {
				headers = append(headers, cv.Value)
			} else {
				headers = append(headers, "")
			}
		}
		meta.Headers = headers
	}

	return meta, nil
}

func cellToCellValue(wb *Workbook, cell *Cell, col, row int) *CellValue {
	cv := &CellValue{
		Row:     row,
		Col:     col,
		ColRef:  cell.ColRef,
		Raw:     cell.Value,
		Formula: cell.Formula,
	}

	switch cell.Type {
	case "s":
		// Shared string
		cv.Type = CellTypeString
		idx, err := strconv.Atoi(cell.Value)
		if err == nil && idx >= 0 && idx < len(wb.SharedStrings) {
			cv.Value = wb.SharedStrings[idx]
		}
	case "str":
		// Inline string
		cv.Type = CellTypeString
		cv.Value = cell.Value
	case "b":
		cv.Type = CellTypeBoolean
		if cell.Value == "1" || cell.Value == "TRUE" {
			cv.Value = "TRUE"
		} else {
			cv.Value = "FALSE"
		}
	case "e":
		cv.Type = CellTypeError
		cv.Value = cell.Value
	case "":
		if cell.Formula != "" {
			cv.Type = CellTypeFormula
			cv.Value = cell.Formula
		} else if cell.Value != "" {
			// Try to detect number vs date
			if isNumeric(cell.Value) {
				cv.Type = CellTypeNumber
				cv.Value = cell.Value
			} else {
				cv.Type = CellTypeString
				cv.Value = cell.Value
			}
		} else {
			cv.Type = CellTypeEmpty
		}
	default:
		// Treat as number
		cv.Type = CellTypeNumber
		cv.Value = cell.Value
	}

	return cv
}

func isNumeric(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

// CreateSheet creates a new sheet with the given name.
func (wb *Workbook) CreateSheet(name string) int {
	idx := len(wb.Sheets)
	wb.Sheets = append(wb.Sheets, &Sheet{
		Index: idx,
		Name:  name,
		Rows:  make(map[int]map[int]*Cell),
	})
	return idx
}

// DeleteSheet removes a sheet by index.
func (wb *Workbook) DeleteSheet(index int) error {
	if index < 0 || index >= len(wb.Sheets) {
		return fmt.Errorf("sheet index %d out of range", index)
	}
	wb.Sheets = append(wb.Sheets[:index], wb.Sheets[index+1:]...)
	return nil
}

// RenameSheet renames a sheet.
func (wb *Workbook) RenameSheet(index int, newName string) error {
	if index < 0 || index >= len(wb.Sheets) {
		return fmt.Errorf("sheet index %d out of range", index)
	}
	wb.Sheets[index].Name = newName
	return nil
}

// CopySheet copies a sheet to a new name.
func (wb *Workbook) CopySheet(srcIndex int, newName string) (int, error) {
	if srcIndex < 0 || srcIndex >= len(wb.Sheets) {
		return 0, fmt.Errorf("sheet index %d out of range", srcIndex)
	}

	src := wb.Sheets[srcIndex]
	dst := &Sheet{
		Index: len(wb.Sheets),
		Name:  newName,
		Rows:  make(map[int]map[int]*Cell),
	}

	// Deep copy rows
	for r, rowCells := range src.Rows {
		dstRow := make(map[int]*Cell)
		for c, cell := range rowCells {
			cellCopy := *cell
			dstRow[c] = &cellCopy
		}
		dst.Rows[r] = dstRow
	}
	dst.MaxRow = src.MaxRow
	dst.MaxCol = src.MaxCol
	dst.HasFormulas = src.HasFormulas

	wb.Sheets = append(wb.Sheets, dst)
	return len(wb.Sheets) - 1, nil
}

// ApplyCellStyle registers a CellStyle and applies it to a range of cells.
// Returns the style index for reference.
func (wb *Workbook) ApplyCellStyle(sheetIndex int, rng CellRange, style CellStyle) (int, error) {
	if sheetIndex < 0 || sheetIndex >= len(wb.Sheets) {
		return 0, fmt.Errorf("sheet index %d out of range", sheetIndex)
	}
	sm := wb.StyleManager()
	styleID := sm.addStyle(style)
	wb.Sheets[sheetIndex].SetCellStyle(rng, styleID)
	return styleID, nil
}

// RegisterStyle registers a CellStyle with the style manager and returns its index.
func (wb *Workbook) RegisterStyle(style CellStyle) int {
	return wb.StyleManager().addStyle(style)
}

// GetSheetIndexByName returns the index of a sheet by name, or -1 if not found.
func (wb *Workbook) GetSheetIndexByName(name string) int {
	for i, s := range wb.Sheets {
		if s.Name == name {
			return i
		}
	}
	return -1
}
