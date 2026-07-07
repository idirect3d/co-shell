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

import "fmt"

// colRefToIndex converts column reference like "A", "Z", "AA" to 0-based index.
func colRefToIndex(ref string) (int, error) {
	idx := 0
	for i := 0; i < len(ref); i++ {
		c := ref[i]
		if c < 'A' || c > 'Z' {
			return 0, fmt.Errorf("invalid column reference: %q", ref)
		}
		idx = idx*26 + int(c-'A'+1)
	}
	return idx - 1, nil
}

// indexToColRef converts 0-based column index to column reference like "A", "Z", "AA".
func indexToColRef(idx int) string {
	ref := ""
	for idx >= 0 {
		ref = string(rune('A'+idx%26)) + ref
		idx = idx/26 - 1
	}
	return ref
}

// ParseCellRef parses a cell reference like "A1" or "C5" into column index and row index.
// Returns (col, row, error) where both are 0-based.
func ParseCellRef(ref string) (int, int, error) {
	if len(ref) < 2 {
		return 0, 0, fmt.Errorf("invalid cell reference: %q", ref)
	}

	// Split into column letters and row number
	colEnd := 0
	for i := 0; i < len(ref); i++ {
		if ref[i] >= 'A' && ref[i] <= 'Z' {
			colEnd = i + 1
		} else {
			break
		}
	}

	if colEnd == 0 {
		return 0, 0, fmt.Errorf("invalid cell reference: %q", ref)
	}

	colStr := ref[:colEnd]
	rowStr := ref[colEnd:]

	if len(rowStr) == 0 {
		return 0, 0, fmt.Errorf("invalid cell reference (missing row): %q", ref)
	}

	col, err := colRefToIndex(colStr)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid cell reference: %q: %w", ref, err)
	}

	row := 0
	for _, c := range rowStr {
		if c < '0' || c > '9' {
			return 0, 0, fmt.Errorf("invalid cell reference: %q", ref)
		}
		row = row*10 + int(c-'0')
	}

	return col, row - 1, nil // convert to 0-based
}

// FormatCellRef formats column index (0-based) and row index (0-based) into a cell reference.
func FormatCellRef(col, row int) string {
	return indexToColRef(col) + fmt.Sprintf("%d", row+1)
}

// FormatRange formats a cell range as "A1:B2".
func FormatRange(rng CellRange) string {
	return FormatCellRef(rng.StartCol, rng.StartRow) + ":" + FormatCellRef(rng.EndCol, rng.EndRow)
}
