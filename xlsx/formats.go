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

// MergeCell represents a range of merged cells.
type MergeCell struct {
	StartCol int
	StartRow int
	EndCol   int
	EndRow   int
}

// ColInfo represents column width.
type ColInfo struct {
	Min   int     // 1-based first column
	Max   int     // 1-based last column
	Width float64 // column width in character units
}

// RowHeight records the height of a specific row.
type RowHeight struct {
	Row    int     // 0-based row index
	Height float64 // height in points
}

// AddMergeCell adds a merge cell range to the sheet.
func (s *Sheet) AddMergeCell(rng CellRange) {
	s.MergeCells = append(s.MergeCells, MergeCell{
		StartCol: rng.StartCol,
		StartRow: rng.StartRow,
		EndCol:   rng.EndCol,
		EndRow:   rng.EndRow,
	})
}

// RemoveMergeCell removes merge cell ranges that intersect with the given range.
func (s *Sheet) RemoveMergeCell(rng CellRange) {
	filtered := s.MergeCells[:0]
	for _, mc := range s.MergeCells {
		// Check overlap
		if mc.StartCol <= rng.EndCol && mc.EndCol >= rng.StartCol &&
			mc.StartRow <= rng.EndRow && mc.EndRow >= rng.StartRow {
			continue // remove overlapping merges
		}
		filtered = append(filtered, mc)
	}
	s.MergeCells = filtered
}

// SetRowHeight sets the height of a specific row.
func (s *Sheet) SetRowHeight(row int, height float64) {
	for i, rh := range s.RowHeights {
		if rh.Row == row {
			s.RowHeights[i].Height = height
			return
		}
	}
	s.RowHeights = append(s.RowHeights, RowHeight{Row: row, Height: height})
}

// SetColWidth sets the width of a column range.
func (s *Sheet) SetColWidth(col int, width float64) {
	ci := ColInfo{Min: col + 1, Max: col + 1, Width: width}
	for i, existing := range s.ColInfos {
		if existing.Min == ci.Min && existing.Max == ci.Max {
			s.ColInfos[i].Width = width
			return
		}
	}
	s.ColInfos = append(s.ColInfos, ci)
}

// SetCellStyle sets the style index for a cell range.
func (s *Sheet) SetCellStyle(rng CellRange, styleID int) {
	for r := rng.StartRow; r <= rng.EndRow; r++ {
		for c := rng.StartCol; c <= rng.EndCol; c++ {
			if s.Rows[r] == nil {
				s.Rows[r] = make(map[int]*Cell)
			}
			if cell, ok := s.Rows[r][c]; ok {
				cell.StyleID = styleID
			} else {
				// Create empty cell with style
				s.Rows[r][c] = &Cell{
					ColRef:  FormatCellRef(c, r),
					Col:     c,
					Row:     r,
					Type:    "",
					Value:   "",
					StyleID: styleID,
				}
			}
		}
	}
}
