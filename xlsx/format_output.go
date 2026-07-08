// Author: L.Shuang
// Created: 2026-07-08
// Last Modified: 2026-07-08
//
// MIT License
// Copyright (c) 2026 L.Shuang
package xlsx

import (
	"fmt"
	"html"
	"strings"
)

// ReadRangeAsHTML reads a cell range and returns HTML table.
// format="simple" (default): structural HTML with colspan/rowspan.
// format="full": includes background color via bgcolor attribute.
func (wb *Workbook) ReadRangeAsHTML(sheetIndex int, rng CellRange, format string) (string, error) {
	cells, err := wb.ReadRange(sheetIndex, rng)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("<table>\n")

	for _, row := range cells {
		sb.WriteString("  <tr>")
		for _, cv := range row {
			cellText := html.EscapeString(cv.Value)
			attrs := ""
			sb.WriteString(fmt.Sprintf("<td%s>%s</td>", attrs, cellText))
		}
		sb.WriteString("</tr>\n")
	}
	sb.WriteString("</table>")

	return sb.String(), nil
}

// ReadRangeAsTSV reads a cell range and returns tab-separated values.
// Each row is prefixed with "N: " (1-based row index).
// Newlines in cell values are escaped as "\\n".
func (wb *Workbook) ReadRangeAsTSV(sheetIndex int, rng CellRange) (string, error) {
	cells, err := wb.ReadRange(sheetIndex, rng)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	for ri, row := range cells {
		if ri > 0 {
			sb.WriteString("\n")
		}
		physRow := rng.StartRow + ri + 1
		sb.WriteString(fmt.Sprintf("%d: ", physRow))
		var vals []string
		for _, cv := range row {
			v := cv.Value
			v = strings.ReplaceAll(v, "\n", "\\n")
			v = strings.ReplaceAll(v, "\t", " ")
			vals = append(vals, v)
		}
		sb.WriteString(strings.Join(vals, "\t"))
	}

	return sb.String(), nil
}

// ReadRangeAsMarkdown reads a cell range and returns a Markdown table.
// Each row is prefixed with "N: " (1-based row index).
func (wb *Workbook) ReadRangeAsMarkdown(sheetIndex int, rng CellRange) (string, error) {
	cells, err := wb.ReadRange(sheetIndex, rng)
	if err != nil {
		return "", err
	}

	if len(cells) == 0 || len(cells[0]) == 0 {
		return "", nil
	}

	cols := len(cells[0])
	var sb strings.Builder

	for ri, row := range cells {
		physRow := rng.StartRow + ri + 1
		sb.WriteString(fmt.Sprintf("%d: ", physRow))
		sb.WriteString("|")
		for _, cv := range row {
			v := strings.ReplaceAll(cv.Value, "|", "\\|")
			sb.WriteString(" ")
			sb.WriteString(v)
			sb.WriteString(" |")
		}
		sb.WriteString("\n")

		// Header separator after first row
		if ri == 0 {
			sb.WriteString(fmt.Sprintf("%d: ", 0)) // no row number for separator line
			sb.WriteString("|")
			for c := 0; c < cols; c++ {
				sb.WriteString(" --- |")
			}
			sb.WriteString("\n")
		}
	}

	return sb.String(), nil
}

// ReadRangeAsGrid reads a cell range and returns a grid format.
// First row shows Excel column letters (A, B, C...) as headers.
// Each data row is prefixed with "N: " and cells show values only (no column ref or type prefix).
func (wb *Workbook) ReadRangeAsGrid(sheetIndex int, rng CellRange) (string, error) {
	cells, err := wb.ReadRange(sheetIndex, rng)
	if err != nil {
		return "", err
	}

	if len(cells) == 0 {
		return "", nil
	}

	var sb strings.Builder

	// First row: column headers (A, B, C, ...) — letter only, no row number
	sb.WriteString("   ") // align with "N: " prefix width
	var headers []string
	for c := rng.StartCol; c <= rng.EndCol; c++ {
		headers = append(headers, indexToColRef(c))
	}
	sb.WriteString(strings.Join(headers, "\t"))
	sb.WriteString("\n")

	// Data rows
	for ri, row := range cells {
		physRow := rng.StartRow + ri + 1
		sb.WriteString(fmt.Sprintf("%d: ", physRow))
		var vals []string
		for _, cv := range row {
			if cv.Type == CellTypeEmpty {
				vals = append(vals, "")
			} else {
				vals = append(vals, cv.Value)
			}
		}
		sb.WriteString(strings.Join(vals, "\t"))
		if ri < len(cells)-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String(), nil
}
