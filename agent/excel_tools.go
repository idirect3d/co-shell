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

package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/idirect3d/co-shell/xlsx"
)

// excelOpenTool opens an XLSX file and returns a session ID.
func (a *Agent) excelOpenTool(ctx context.Context, args map[string]interface{}) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("path argument is required")
	}

	mode, _ := args["mode"].(string)
	if mode == "" {
		return "", fmt.Errorf("mode argument is required (create/read/copy)")
	}

	// Resolve relative paths
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("cannot get current working directory: %w", err)
		}
		path = filepath.Join(cwd, path)
	}

	sessionID, err := a.excelSessionMgr.openWithMode(path, mode)
	if err != nil {
		return "", err
	}

	s, err := a.excelSessionMgr.get(sessionID)
	if err != nil {
		return "", err
	}

	fileInfo, _ := os.Stat(s.Path)
	fileSize := ""
	if fileInfo != nil {
		fileSize = fmt.Sprintf("%.1f KB", float64(fileInfo.Size())/1024.0)
	}
	if mode == "create" && fileSize == "0.0 KB" {
		fileSize = "(新建)"
	}

	var resultMsg string
	if mode == "copy" {
		resultMsg = fmt.Sprintf("已创建副本: %s (%s)\n会话 ID: %s\n\n此文件为 %s 的副本，后续操作应直接在此副本上修改，严禁以此文件替换原始文件。\n\n请使用 excel_overview 获取文件概览。", s.Path, fileSize, sessionID, filepath.Base(path))
	} else if mode == "read" {
		resultMsg = fmt.Sprintf("已打开 Excel 文件: %s (%s) (只读)\n会话 ID: %s\n\n请使用 excel_overview 获取文件概览。", s.Path, fileSize, sessionID)
	} else {
		resultMsg = fmt.Sprintf("已打开 Excel 文件: %s (%s)\n会话 ID: %s\n\n请使用 excel_overview 获取文件概览。", s.Path, fileSize, sessionID)
	}
	return resultMsg, nil
}

// excelSaveTool saves an Excel session without closing.
func (a *Agent) excelSaveTool(ctx context.Context, args map[string]interface{}) (string, error) {
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("session_id argument is required")
	}

	s, err := a.excelSessionMgr.get(sessionID)
	if err != nil {
		return "", err
	}

	if s.ReadOnly {
		return "", fmt.Errorf("此文件以只读方式打开，无法保存")
	}

	if err := a.excelSessionMgr.save(sessionID); err != nil {
		return "", err
	}

	fileInfo, _ := os.Stat(s.Path)
	fileSize := ""
	if fileInfo != nil {
		fileSize = fmt.Sprintf("%.1f KB", float64(fileInfo.Size())/1024.0)
	}

	return fmt.Sprintf("已保存: %s (%s)", s.Path, fileSize), nil
}

// excelCloseTool closes an Excel session.
func (a *Agent) excelCloseTool(ctx context.Context, args map[string]interface{}) (string, error) {
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("session_id argument is required")
	}

	s, err := a.excelSessionMgr.get(sessionID)
	if err != nil {
		return "", err
	}
	path := s.Path

	if err := a.excelSessionMgr.close(sessionID); err != nil {
		return "", err
	}

	return fmt.Sprintf("已关闭 Excel 文件: %s", path), nil
}

// excelOverviewTool returns metadata about all sheets without cell data.
func (a *Agent) excelOverviewTool(ctx context.Context, args map[string]interface{}) (string, error) {
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("session_id argument is required")
	}

	s, err := a.excelSessionMgr.get(sessionID)
	if err != nil {
		return "", err
	}

	wb := s.Workbook
	if len(wb.Sheets) == 0 {
		return "工作簿中没有 Sheet 页。", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("文件名: %s\n", filepath.Base(s.Path)))

	if len(wb.Sheets) == 1 {
		sb.WriteString("Sheet 页: 1 个\n\n")
	} else {
		sb.WriteString(fmt.Sprintf("Sheet 页: %d 个\n\n", len(wb.Sheets)))
	}

	for i, sheet := range wb.Sheets {
		meta, _ := wb.GetSheetMeta(i)
		sb.WriteString(fmt.Sprintf("--- Sheet %d: %s ---\n", i+1, sheet.Name))
		if meta != nil {
			sb.WriteString(fmt.Sprintf("  数据范围: %s\n", meta.UsedRange))
			sb.WriteString(fmt.Sprintf("  总行数: %d\n", meta.Rows))
			sb.WriteString(fmt.Sprintf("  总列数: %d\n", meta.Cols))
			if meta.HasFormulas {
				sb.WriteString("  包含公式: 是\n")
			}
			if len(meta.Headers) > 0 {
				nonEmpty := 0
				for _, h := range meta.Headers {
					if h != "" {
						nonEmpty++
					}
				}
				if nonEmpty > 0 {
					sb.WriteString(fmt.Sprintf("  列标题 (第1行): %s\n", strings.Join(meta.Headers, " | ")))
				}
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("提示: 使用 excel_read <session_id> <sheet名称> <start_row> <end_row> <start_col> <end_col> 读取具体数据。")

	return sb.String(), nil
}

// excelReadTool reads cell data from a range with multiple output formats.
func (a *Agent) excelReadTool(ctx context.Context, args map[string]interface{}) (string, error) {
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("session_id argument is required")
	}

	sheetName, _ := args["sheet"].(string)
	if sheetName == "" {
		return "", fmt.Errorf("sheet argument is required")
	}

	startRow := getIntArg(args, "start_row", 1) - 1
	endRow := getIntArg(args, "end_row", 1) - 1
	startCol := getIntArg(args, "start_col", 1) - 1
	endCol := getIntArg(args, "end_col", 1) - 1
	maxCells := getIntArg(args, "max_cells", 2000)
	format, _ := args["format"].(string)
	if format == "" {
		format = "html"
	}

	if endRow < startRow {
		return "", fmt.Errorf("end_row (%d) must be >= start_row (%d)", endRow+1, startRow+1)
	}
	if endCol < startCol {
		return "", fmt.Errorf("end_col (%d) must be >= start_col (%d)", endCol+1, startCol+1)
	}

	s, err := a.excelSessionMgr.get(sessionID)
	if err != nil {
		return "", err
	}

	sheetIndex := s.Workbook.GetSheetIndexByName(sheetName)
	if sheetIndex < 0 {
		if idx, err := strconv.Atoi(sheetName); err == nil {
			sheetIndex = idx - 1
		}
	}
	if sheetIndex < 0 || sheetIndex >= len(s.Workbook.Sheets) {
		names := make([]string, len(s.Workbook.Sheets))
		for i, sht := range s.Workbook.Sheets {
			names[i] = fmt.Sprintf("%q", sht.Name)
		}
		return "", fmt.Errorf("sheet %q not found. Available sheets: %s", sheetName, strings.Join(names, ", "))
	}

	rng := xlsx.CellRange{
		StartRow: startRow,
		EndRow:   endRow,
		StartCol: startCol,
		EndCol:   endCol,
	}

	totalCells := (endRow - startRow + 1) * (endCol - startCol + 1)
	if totalCells > maxCells {
		return "", fmt.Errorf("请求读取 %d 个单元格，超过了上限 %d 个。请缩小范围后重试", totalCells, maxCells)
	}

	header := fmt.Sprintf("Sheet: %s, 范围 %s\n", sheetName, xlsx.FormatRange(rng))

	var result string

	switch format {
	case "html":
		result, err = s.Workbook.ReadRangeAsHTML(sheetIndex, rng, "simple")
		if err != nil {
			return "", err
		}
	case "full":
		result, err = s.Workbook.ReadRangeAsHTML(sheetIndex, rng, "full")
		if err != nil {
			return "", err
		}
	case "text":
		result, err = s.Workbook.ReadRangeAsTSV(sheetIndex, rng)
		if err != nil {
			return "", err
		}
	case "md":
		result, err = s.Workbook.ReadRangeAsMarkdown(sheetIndex, rng)
		if err != nil {
			return "", err
		}
	case "grid":
		result, err = s.Workbook.ReadRangeAsGrid(sheetIndex, rng)
		if err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unsupported format: %s (supported: html, full, text, md, grid)", format)
	}

	s.Clipboard = nil
	return header + result, nil
}

// excelEditTool writes values to cells.
func (a *Agent) excelEditTool(ctx context.Context, args map[string]interface{}) (string, error) {
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("session_id argument is required")
	}

	sheetName, _ := args["sheet"].(string)
	if sheetName == "" {
		return "", fmt.Errorf("sheet argument is required")
	}

	startCell, _ := args["start_cell"].(string)
	if startCell == "" {
		return "", fmt.Errorf("start_cell argument is required (e.g. 'A1')")
	}

	valuesRaw, ok := args["values"].([]interface{})
	if !ok || len(valuesRaw) == 0 {
		return "", fmt.Errorf("values argument is required (2D array of strings)")
	}

	s, err := a.excelSessionMgr.get(sessionID)
	if err != nil {
		return "", err
	}

	sheetIndex := s.Workbook.GetSheetIndexByName(sheetName)
	if sheetIndex < 0 {
		if idx, err := strconv.Atoi(sheetName); err == nil {
			sheetIndex = idx - 1
		}
	}
	if sheetIndex < 0 || sheetIndex >= len(s.Workbook.Sheets) {
		return "", fmt.Errorf("sheet %q not found", sheetName)
	}

	startCol, startRow, err := xlsx.ParseCellRef(startCell)
	if err != nil {
		return "", fmt.Errorf("invalid start_cell %q: %w", startCell, err)
	}

	// Parse rows: support both []interface{} (OpenAI mode) and tab-separated string (XML mode)
	parsedRows := make([][]string, 0)
	var warnings []string
	for ri, rowRaw := range valuesRaw {
		rowArr, ok := rowRaw.([]interface{})
		if !ok {
			// XML mode: row values are tab-separated text (TSV format, like Excel copy)
			rowStr, ok := rowRaw.(string)
			if !ok {
				continue
			}
			// Split by tab (TSV — recommended, direct from Excel copy)
			parts := strings.Split(rowStr, "\t")
			if len(parts) >= 2 {
				rowArr = make([]interface{}, len(parts))
				for i, p := range parts {
					rowArr[i] = p
				}
			} else {
				// Fallback: try comma-separated
				parts = strings.Split(rowStr, ",")
				if len(parts) >= 2 {
					warnings = append(warnings, fmt.Sprintf("Row %d: used comma as separator (TSV preferred)", ri+1))
					rowArr = make([]interface{}, len(parts))
					for i, p := range parts {
						rowArr[i] = strings.TrimSpace(p)
					}
				} else {
					warnings = append(warnings, fmt.Sprintf("Row %d skipped: single value, not a TSV row", ri+1))
					continue
				}
			}
		}
		rowVals := make([]string, 0, len(rowArr))
		for _, valRaw := range rowArr {
			rowVals = append(rowVals, fmt.Sprintf("%v", valRaw))
		}
		parsedRows = append(parsedRows, rowVals)
	}

	if len(parsedRows) == 0 {
		return "", fmt.Errorf("no valid rows found in values")
	}

	// Count max columns
	colCount := 0
	for _, row := range parsedRows {
		if len(row) > colCount {
			colCount = len(row)
		}
	}

	for ri, row := range parsedRows {
		for ci, val := range row {
			if err := s.Workbook.SetCellValue(sheetIndex, startCol+ci, startRow+ri, val); err != nil {
				return "", fmt.Errorf("error writing cell [%d,%d]: %w", startRow+ri+1, startCol+ci+1, err)
			}
		}
	}

	s.Dirty = true

	endCell := xlsx.FormatCellRef(startCol+colCount-1, startRow+len(parsedRows)-1)
	result := fmt.Sprintf("已写入 %d 行 × %d 列到 %s!%s:%s\n(未保存，请调用 excel_save 持久化)", len(parsedRows), colCount, sheetName, startCell, endCell)
	if len(warnings) > 0 {
		result += "\n⚠ " + strings.Join(warnings, "\n⚠ ")
	}
	return result, nil
}

// excelCopyTool copies a range to the clipboard.
func (a *Agent) excelCopyTool(ctx context.Context, args map[string]interface{}) (string, error) {
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("session_id argument is required")
	}

	sheetName, _ := args["sheet"].(string)
	if sheetName == "" {
		return "", fmt.Errorf("sheet argument is required")
	}

	startRow := getIntArg(args, "start_row", 1) - 1
	endRow := getIntArg(args, "end_row", 1) - 1
	startCol := getIntArg(args, "start_col", 1) - 1
	endCol := getIntArg(args, "end_col", 1) - 1
	cutMode := getBoolArg(args, "cut", false)

	s, err := a.excelSessionMgr.get(sessionID)
	if err != nil {
		return "", err
	}

	sheetIndex := s.Workbook.GetSheetIndexByName(sheetName)
	if sheetIndex < 0 {
		return "", fmt.Errorf("sheet %q not found", sheetName)
	}

	rng := xlsx.CellRange{StartRow: startRow, EndRow: endRow, StartCol: startCol, EndCol: endCol}
	cells, err := s.Workbook.ReadRange(sheetIndex, rng)
	if err != nil {
		return "", err
	}

	var cutRange *xlsx.CellRange
	var cutSheet string
	if cutMode {
		cutRange = &rng
		cutSheet = sheetName
	}

	if err := a.excelSessionMgr.setClipboard(sessionID, cells, cutMode, cutSheet, cutRange); err != nil {
		return "", err
	}

	rows := endRow - startRow + 1
	cols := endCol - startCol + 1

	var preview strings.Builder
	if cutMode {
		preview.WriteString(fmt.Sprintf("已剪切 %d 行 × %d 列 (从 %s!%s)\n", rows, cols, sheetName, xlsx.FormatRange(rng)))
		preview.WriteString("剪切模式下，粘贴后将自动清除源区域。\n")
	} else {
		preview.WriteString(fmt.Sprintf("已复制 %d 行 × %d 列 (从 %s!%s)\n", rows, cols, sheetName, xlsx.FormatRange(rng)))
	}
	preview.WriteString("预览 (前3行):\n")
	for r := 0; r < rows && r < 3; r++ {
		vals := make([]string, 0)
		for c := 0; c < cols; c++ {
			v := cells[r][c]
			if v.Type != xlsx.CellTypeEmpty {
				vals = append(vals, v.Value)
			} else {
				vals = append(vals, "")
			}
		}
		preview.WriteString(fmt.Sprintf("  %s\n", strings.Join(vals, "\t")))
	}

	return preview.String(), nil
}

// excelPasteTool pastes clipboard content to target cell.
func (a *Agent) excelPasteTool(ctx context.Context, args map[string]interface{}) (string, error) {
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("session_id argument is required")
	}

	sheetName, _ := args["sheet"].(string)
	if sheetName == "" {
		return "", fmt.Errorf("sheet argument is required")
	}

	targetCell, _ := args["target_cell"].(string)
	if targetCell == "" {
		return "", fmt.Errorf("target_cell argument is required (e.g. 'F2')")
	}

	clipboard, err := a.excelSessionMgr.getClipboard(sessionID)
	if err != nil {
		return "", err
	}

	s, err := a.excelSessionMgr.get(sessionID)
	if err != nil {
		return "", err
	}

	sheetIndex := s.Workbook.GetSheetIndexByName(sheetName)
	if sheetIndex < 0 {
		return "", fmt.Errorf("sheet %q not found", sheetName)
	}

	targetCol, targetRow, err := xlsx.ParseCellRef(targetCell)
	if err != nil {
		return "", fmt.Errorf("invalid target_cell %q: %w", targetCell, err)
	}

	// Write clipboard content
	for r, row := range clipboard.Values {
		for c, cv := range row {
			if cv != nil && cv.Type != xlsx.CellTypeEmpty {
				if err := s.Workbook.SetCellValue(sheetIndex, targetCol+c, targetRow+r, cv.Value); err != nil {
					return "", fmt.Errorf("error pasting cell [%d,%d]: %w", targetRow+r+1, targetCol+c+1, err)
				}
			}
		}
	}

	s.Dirty = true

	clonedRows := len(clipboard.Values)
	clonedCols := 0
	if clonedRows > 0 {
		clonedCols = len(clipboard.Values[0])
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("已粘贴 %d 行 × %d 列到 %s!%s", clonedRows, clonedCols, sheetName, targetCell))

	// Handle cut mode: clear source area
	if clipboard.CutMode && clipboard.CutSheet != "" && clipboard.CutRange != nil {
		cutSheetIndex := s.Workbook.GetSheetIndexByName(clipboard.CutSheet)
		if cutSheetIndex >= 0 {
			cr := clipboard.CutRange
			for r := cr.StartRow; r <= cr.EndRow; r++ {
				for c := cr.StartCol; c <= cr.EndCol; c++ {
					if rowCells, ok := s.Workbook.Sheets[cutSheetIndex].Rows[r]; ok {
						delete(rowCells, c)
					}
				}
			}
		}
		result.WriteString(fmt.Sprintf("\n已清除源区域: %s!%s", clipboard.CutSheet, xlsx.FormatRange(*clipboard.CutRange)))
	}

	a.excelSessionMgr.setClipboard(sessionID, nil, false, "", nil) // clear clipboard

	result.WriteString("\n(未保存，请调用 excel_save 持久化)")
	return result.String(), nil
}

// excelInsertTool inserts rows or columns.
func (a *Agent) excelInsertTool(ctx context.Context, args map[string]interface{}) (string, error) {
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("session_id argument is required")
	}

	sheetName, _ := args["sheet"].(string)
	if sheetName == "" {
		return "", fmt.Errorf("sheet argument is required")
	}

	what, _ := args["what"].(string)
	if what != "rows" && what != "cols" {
		return "", fmt.Errorf("what argument must be 'rows' or 'cols'")
	}

	position := getIntArg(args, "position", 1) - 1
	count := getIntArg(args, "count", 1)

	s, err := a.excelSessionMgr.get(sessionID)
	if err != nil {
		return "", err
	}

	sheetIndex := s.Workbook.GetSheetIndexByName(sheetName)
	if sheetIndex < 0 {
		return "", fmt.Errorf("sheet %q not found", sheetName)
	}

	if what == "rows" {
		if err := s.Workbook.InsertRows(sheetIndex, position, count); err != nil {
			return "", err
		}
	} else {
		if err := s.Workbook.InsertCols(sheetIndex, position, count); err != nil {
			return "", err
		}
	}

	s.Dirty = true

	posLabel := position + 1
	if what == "rows" {
		return fmt.Sprintf("已在第 %d 行前插入 %d 行\n(未保存，请调用 excel_save 持久化)", posLabel, count), nil
	}
	return fmt.Sprintf("已在第 %d 列前插入 %d 列\n(未保存，请调用 excel_save 持久化)", posLabel, count), nil
}

// excelDeleteTool deletes rows, columns, or cell content.
func (a *Agent) excelDeleteTool(ctx context.Context, args map[string]interface{}) (string, error) {
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("session_id argument is required")
	}

	sheetName, _ := args["sheet"].(string)
	if sheetName == "" {
		return "", fmt.Errorf("sheet argument is required")
	}

	what, _ := args["what"].(string)
	if what != "rows" && what != "cols" && what != "cells" {
		return "", fmt.Errorf("what argument must be 'rows', 'cols', or 'cells'")
	}

	s, err := a.excelSessionMgr.get(sessionID)
	if err != nil {
		return "", err
	}

	sheetIndex := s.Workbook.GetSheetIndexByName(sheetName)
	if sheetIndex < 0 {
		return "", fmt.Errorf("sheet %q not found", sheetName)
	}

	if what == "rows" {
		position := getIntArg(args, "position", 1) - 1
		count := getIntArg(args, "count", 1)
		if err := s.Workbook.DeleteRows(sheetIndex, position, count); err != nil {
			return "", err
		}
		s.Dirty = true
		return fmt.Sprintf("已删除第 %d-%d 行\n(未保存，请调用 excel_save 持久化)", position+1, position+count), nil
	} else if what == "cols" {
		position := getIntArg(args, "position", 1) - 1
		count := getIntArg(args, "count", 1)
		if err := s.Workbook.DeleteCols(sheetIndex, position, count); err != nil {
			return "", err
		}
		s.Dirty = true
		return fmt.Sprintf("已删除第 %d-%d 列\n(未保存，请调用 excel_save 持久化)", position+1, position+count), nil
	} else {
		// cells: clear content
		startRow := getIntArg(args, "start_row", 1) - 1
		endRow := getIntArg(args, "end_row", 1) - 1
		startCol := getIntArg(args, "start_col", 1) - 1
		endCol := getIntArg(args, "end_col", 1) - 1

		for r := startRow; r <= endRow; r++ {
			if rowCells, ok := s.Workbook.Sheets[sheetIndex].Rows[r]; ok {
				for c := startCol; c <= endCol; c++ {
					delete(rowCells, c)
				}
			}
		}
		s.Dirty = true
		return fmt.Sprintf("已清空 %s 的单元格范围 %s:%s\n(未保存，请调用 excel_save 持久化)", sheetName,
			xlsx.FormatCellRef(startCol, startRow), xlsx.FormatCellRef(endCol, endRow)), nil
	}
}

// excelSheetTool manages sheets.
func (a *Agent) excelSheetTool(ctx context.Context, args map[string]interface{}) (string, error) {
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("session_id argument is required")
	}

	action, _ := args["action"].(string)
	if action == "" {
		return "", fmt.Errorf("action argument is required")
	}

	s, err := a.excelSessionMgr.get(sessionID)
	if err != nil {
		return "", err
	}

	wb := s.Workbook

	switch action {
	case "create":
		name, _ := args["name"].(string)
		if name == "" {
			return "", fmt.Errorf("name argument is required for create action")
		}
		idx := wb.CreateSheet(name)
		s.Dirty = true
		return fmt.Sprintf("已创建 Sheet %q (索引 %d)", name, idx+1), nil

	case "delete":
		name, _ := args["name"].(string)
		if name == "" {
			return "", fmt.Errorf("name argument is required for delete action")
		}
		idx := wb.GetSheetIndexByName(name)
		if idx < 0 {
			return "", fmt.Errorf("sheet %q not found", name)
		}
		if err := wb.DeleteSheet(idx); err != nil {
			return "", err
		}
		s.Dirty = true
		return fmt.Sprintf("已删除 Sheet %q", name), nil

	case "rename":
		name, _ := args["name"].(string)
		newName, _ := args["new_name"].(string)
		if name == "" || newName == "" {
			return "", fmt.Errorf("name and new_name arguments are required for rename action")
		}
		idx := wb.GetSheetIndexByName(name)
		if idx < 0 {
			return "", fmt.Errorf("sheet %q not found", name)
		}
		if err := wb.RenameSheet(idx, newName); err != nil {
			return "", err
		}
		s.Dirty = true
		return fmt.Sprintf("已重命名 Sheet %q → %q", name, newName), nil

	case "copy":
		name, _ := args["name"].(string)
		newName, _ := args["new_name"].(string)
		if name == "" || newName == "" {
			return "", fmt.Errorf("name and new_name arguments are required for copy action")
		}
		idx := wb.GetSheetIndexByName(name)
		if idx < 0 {
			return "", fmt.Errorf("sheet %q not found", name)
		}
		newIdx, err := wb.CopySheet(idx, newName)
		if err != nil {
			return "", err
		}
		s.Dirty = true
		return fmt.Sprintf("已复制 Sheet %q → %q (新索引 %d)", name, newName, newIdx+1), nil

	case "list":
		var sb strings.Builder
		for i, sheet := range wb.Sheets {
			meta, _ := wb.GetSheetMeta(i)
			sb.WriteString(fmt.Sprintf("%d. %s", i+1, sheet.Name))
			if meta != nil {
				sb.WriteString(fmt.Sprintf(" (%s, %d rows)", meta.UsedRange, meta.Rows))
			}
			sb.WriteString("\n")
		}
		return sb.String(), nil

	default:
		return "", fmt.Errorf("unsupported action: %s (supported: create, delete, rename, copy, list)", action)
	}
}

// excelFormatTool formats a range of cells with style properties.
// mode="reset" (default): replaces all style properties with the specified ones.
// mode="merge": only updates the properties in what[], preserving existing styles.
func (a *Agent) excelFormatTool(ctx context.Context, args map[string]interface{}) (string, error) {
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("session_id argument is required")
	}
	s, err := a.excelSessionMgr.get(sessionID)
	if err != nil {
		return "", err
	}

	sheetName, _ := args["sheet"].(string)
	if sheetName == "" {
		return "", fmt.Errorf("sheet argument is required")
	}
	sheetIndex := s.Workbook.GetSheetIndexByName(sheetName)
	if sheetIndex < 0 {
		return "", fmt.Errorf("sheet %q not found", sheetName)
	}
	sheet := s.Workbook.Sheets[sheetIndex]

	// Get what operations to perform
	whatRaw, _ := args["what"].([]interface{})
	whatSet := make(map[string]bool)
	for _, w := range whatRaw {
		if ws, ok := w.(string); ok {
			whatSet[ws] = true
		}
	}
	if len(whatSet) == 0 {
		return "", fmt.Errorf("what argument is required (array of operations)")
	}

	// Parse mode: "merge" (default) or "reset"
	mode, _ := args["mode"].(string)
	if mode == "" {
		mode = "merge"
	}

	startRow := getIntArg(args, "start_row", 1) - 1
	endRow := getIntArg(args, "end_row", startRow+1) - 1
	startCol := getIntArg(args, "start_col", 1) - 1
	endCol := getIntArg(args, "end_col", startCol+1) - 1
	rng := xlsx.CellRange{StartRow: startRow, EndRow: endRow, StartCol: startCol, EndCol: endCol}

	var results []string

	// Merge/unmerge/row_height/col_width operations (no mode dependency)
	if whatSet["merge"] {
		sheet.AddMergeCell(rng)
		results = append(results, fmt.Sprintf("合并 %s", xlsx.FormatRange(rng)))
	}
	if whatSet["unmerge"] {
		sheet.RemoveMergeCell(rng)
		results = append(results, fmt.Sprintf("拆分 %s", xlsx.FormatRange(rng)))
	}
	if whatSet["row_height"] {
		h := getFloatArg(args, "row_height", 0)
		if h > 0 {
			sheet.SetRowHeight(startRow, h)
			results = append(results, fmt.Sprintf("行高 %.1f", h))
		}
	}
	if whatSet["col_width"] {
		w := getFloatArg(args, "col_width", 0)
		if w > 0 {
			sheet.SetColWidth(startCol, w)
			results = append(results, fmt.Sprintf("列宽 %.1f", w))
		}
	}

	// Build a CellStyle for font/fill/border/alignment/number_format
	if whatSet["font"] || whatSet["fill"] || whatSet["border"] || whatSet["alignment"] || whatSet["number_format"] {
		sm := s.Workbook.StyleManager()

		if mode == "reset" {
			// RESET mode: build a fresh CellStyle from args only
			cs := buildCellStyleFromArgs(args, whatSet)
			if _, err := s.Workbook.ApplyCellStyle(sheetIndex, rng, cs); err != nil {
				return "", fmt.Errorf("error applying style: %w", err)
			}
			results = append(results, fmt.Sprintf("[reset] %s", describeCellStyle(cs)))
		} else {
			// MERGE mode: per-cell, merge new props onto existing
			for r := rng.StartRow; r <= rng.EndRow; r++ {
				for c := rng.StartCol; c <= rng.EndCol; c++ {
					// Default styleID = 0 (default xf)
					styleID := 0
					if rowCells, ok := sheet.Rows[r]; ok {
						if cell, ok := rowCells[c]; ok {
							styleID = cell.StyleID
						}
					}
					// Reconstruct existing style, merge new, register
					existing := sm.StyleFromXF(styleID)
					merged := mergeCellStyle(existing, args, whatSet)
					newStyleID := s.Workbook.RegisterStyle(merged)
					// Ensure cell exists before setting style
					if sheet.Rows[r] == nil {
						sheet.Rows[r] = make(map[int]*xlsx.Cell)
					}
					if _, ok := sheet.Rows[r][c]; !ok {
						sheet.Rows[r][c] = &xlsx.Cell{
							ColRef: xlsx.FormatCellRef(c, r),
							Col:    c, Row: r,
						}
					}
					sheet.Rows[r][c].StyleID = newStyleID
				}
			}
			results = append(results, fmt.Sprintf("[merge] %s", describeCellStyle(buildCellStyleFromArgs(args, whatSet))))
		}
	}

	s.Dirty = true

	if len(results) == 0 {
		return "未执行任何格式操作", nil
	}

	return fmt.Sprintf("格式已应用到 %s:\n  %s\n(未保存，请调用 excel_save 持久化)", xlsx.FormatRange(rng), strings.Join(results, "\n  ")), nil
}

// buildCellStyleFromArgs constructs a CellStyle purely from tool arguments.
func buildCellStyleFromArgs(args map[string]interface{}, whatSet map[string]bool) xlsx.CellStyle {
	cs := xlsx.CellStyle{}

	if whatSet["font"] {
		fs := &xlsx.FontStyle{}
		fs.Name, _ = args["font_name"].(string)
		fs.Size = getIntArg(args, "font_size", 0)
		fs.Bold = getBoolArg(args, "font_bold", false)
		fs.Italic = getBoolArg(args, "font_italic", false)
		fs.Underline = getBoolArg(args, "font_underline", false)
		fs.Color, _ = args["font_color"].(string)
		cs.Font = fs
	}
	if whatSet["fill"] {
		if fc, _ := args["fill_color"].(string); fc != "" {
			cs.Fill = &xlsx.FillStyle{FgColor: fc, PatternType: "solid"}
		}
	}
	if whatSet["border"] {
		bs := &xlsx.BorderStyle{}
		bsStyle, _ := args["border_style"].(string)
		if bsStyle == "" {
			bsStyle = "thin"
		}
		bsColor, _ := args["border_color"].(string)
		if getBoolArg(args, "border_top", true) || !hasKey(args, "border_top") {
			bs.Top = &xlsx.BorderEdge{Style: bsStyle, Color: bsColor}
		}
		if getBoolArg(args, "border_bottom", true) || !hasKey(args, "border_bottom") {
			bs.Bottom = &xlsx.BorderEdge{Style: bsStyle, Color: bsColor}
		}
		if getBoolArg(args, "border_left", true) || !hasKey(args, "border_left") {
			bs.Left = &xlsx.BorderEdge{Style: bsStyle, Color: bsColor}
		}
		if getBoolArg(args, "border_right", true) || !hasKey(args, "border_right") {
			bs.Right = &xlsx.BorderEdge{Style: bsStyle, Color: bsColor}
		}
		cs.Border = bs
	}
	if whatSet["alignment"] {
		cs.Alignment = &xlsx.AlignmentStyle{}
		cs.Alignment.Horizontal, _ = args["h_align"].(string)
		cs.Alignment.Vertical, _ = args["v_align"].(string)
		cs.Alignment.WrapText = getBoolArg(args, "wrap_text", false)
	}
	if whatSet["number_format"] {
		cs.NumberFormat, _ = args["number_format"].(string)
	}
	return cs
}

// mergeCellStyle merges new style properties on top of existing ones.
func mergeCellStyle(existing xlsx.CellStyle, args map[string]interface{}, whatSet map[string]bool) xlsx.CellStyle {
	if whatSet["font"] {
		fs := &xlsx.FontStyle{}
		if existing.Font != nil {
			*fs = *existing.Font
		}
		if n, _ := args["font_name"].(string); n != "" {
			fs.Name = n
		}
		if sz := getIntArg(args, "font_size", 0); sz > 0 {
			fs.Size = sz
		}
		fs.Bold = getBoolArg(args, "font_bold", fs.Bold)
		fs.Italic = getBoolArg(args, "font_italic", fs.Italic)
		fs.Underline = getBoolArg(args, "font_underline", fs.Underline)
		if c, _ := args["font_color"].(string); c != "" {
			fs.Color = c
		}
		existing.Font = fs
	}
	if whatSet["fill"] {
		if fc, _ := args["fill_color"].(string); fc != "" {
			existing.Fill = &xlsx.FillStyle{FgColor: fc, PatternType: "solid"}
		}
	}
	if whatSet["border"] {
		bs := &xlsx.BorderStyle{}
		if existing.Border != nil {
			*bs = *existing.Border
		}
		bsStyle, _ := args["border_style"].(string)
		if bsStyle == "" {
			bsStyle = "thin"
		}
		bsColor, _ := args["border_color"].(string)
		if getBoolArg(args, "border_top", true) || !hasKey(args, "border_top") {
			bs.Top = &xlsx.BorderEdge{Style: bsStyle, Color: bsColor}
		}
		if getBoolArg(args, "border_bottom", true) || !hasKey(args, "border_bottom") {
			bs.Bottom = &xlsx.BorderEdge{Style: bsStyle, Color: bsColor}
		}
		if getBoolArg(args, "border_left", true) || !hasKey(args, "border_left") {
			bs.Left = &xlsx.BorderEdge{Style: bsStyle, Color: bsColor}
		}
		if getBoolArg(args, "border_right", true) || !hasKey(args, "border_right") {
			bs.Right = &xlsx.BorderEdge{Style: bsStyle, Color: bsColor}
		}
		existing.Border = bs
	}
	if whatSet["alignment"] {
		al := &xlsx.AlignmentStyle{}
		if existing.Alignment != nil {
			*al = *existing.Alignment
		}
		if h, _ := args["h_align"].(string); h != "" {
			al.Horizontal = h
		}
		if v, _ := args["v_align"].(string); v != "" {
			al.Vertical = v
		}
		al.WrapText = getBoolArg(args, "wrap_text", al.WrapText)
		existing.Alignment = al
	}
	if whatSet["number_format"] {
		if nf, _ := args["number_format"].(string); nf != "" {
			existing.NumberFormat = nf
		}
	}
	return existing
}

func describeCellStyle(cs xlsx.CellStyle) string {
	var parts []string
	if cs.Font != nil {
		parts = append(parts, "字体")
	}
	if cs.Fill != nil {
		parts = append(parts, "底色")
	}
	if cs.Border != nil {
		parts = append(parts, "边框")
	}
	if cs.Alignment != nil {
		parts = append(parts, "对齐")
	}
	if cs.NumberFormat != "" {
		parts = append(parts, "数字格式")
	}
	if len(parts) == 0 {
		return "无"
	}
	return strings.Join(parts, "+")
}

// hasKey checks if a key exists in the args map.
func hasKey(args map[string]interface{}, key string) bool {
	_, ok := args[key]
	return ok
}

func getIntArg(args map[string]interface{}, key string, defaultVal int) int {
	if v, ok := args[key].(float64); ok {
		return int(v)
	}
	if v, ok := args[key].(string); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}

func getFloatArg(args map[string]interface{}, key string, defaultVal float64) float64 {
	if v, ok := args[key].(float64); ok {
		return v
	}
	if v, ok := args[key].(string); ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return defaultVal
}

func getBoolArg(args map[string]interface{}, key string, defaultVal bool) bool {
	if v, ok := args[key].(bool); ok {
		return v
	}
	return defaultVal
}
