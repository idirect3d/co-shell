// Author: L.Shuang
// Created: 2026-05-01
// Last Modified: 2026-05-01
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
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
)

// readFileTool reads the contents of a file and returns it with line numbers.
func (a *Agent) readFileTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("readFileTool called: args=%v", args)
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path argument is required")
	}

	// Resolve relative paths
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("cannot get current working directory: %w", err)
		}
		path = filepath.Join(cwd, path)
	}

	// start_line and end_line are both required
	startLineRaw, ok := args["start_line"].(float64)
	if !ok {
		return "", fmt.Errorf("start_line is required (1-based line number to start reading from)")
	}
	startLine := int(startLineRaw)

	endLineRaw, ok := args["end_line"].(float64)
	if !ok {
		return "", fmt.Errorf("end_line is required (1-based line number to stop reading at, inclusive)")
	}
	endLine := int(endLineRaw)

	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("cannot read file %q: %w", path, err)
	}

	// Binary file detection: check the first 512 bytes for NULL bytes
	// or excessive non-text control characters.
	// This catches common binary formats (images, PDFs, Office docs, executables, etc.)
	// while allowing text files with normal formatting characters (tabs, newlines, carriage returns).
	checkLen := len(data)
	if checkLen > 512 {
		checkLen = 512
	}
	for i := 0; i < checkLen; i++ {
		b := data[i]
		// NULL byte is a definitive indicator of binary content
		if b == 0 {
			return "", fmt.Errorf(
				"文件 %q 似乎是二进制文件（包含 NULL 字节），无法读取。\n"+
					"此工具只能读取纯文本文件（如 .txt、.md、.go、.py、.js 等）。\n"+
					"如需分析图片文件，请使用 visual_analysis 将图片加载到多模态上下文中。\n"+
					"如需分析 PDF 文档，请使用 bin/pdf2png.py 将 PDF 转为 PNG 图片后处理。\n"+
					"如需读取 Word 文档中的文本内容，请使用 bin/doc2md.py 将文档转换为 Markdown 后再用 read_file 读取。",
				path)
		}
		// Check for non-text control characters (excluding common formatting: \t, \n, \r)
		if b < 0x20 && b != 0x09 && b != 0x0A && b != 0x0D {
			return "", fmt.Errorf(
				"文件 %q 似乎是二进制文件（包含控制字符 0x%02X），无法读取。\n"+
					"此工具只能读取纯文本文件（如 .txt、.md、.go、.py、.js 等）。\n"+
					"如需分析图片文件，请使用 visual_analysis 将图片加载到多模态上下文中。\n"+
					"如需分析 PDF 文档，请使用 bin/pdf2png.py 将 PDF 转为 PNG 图片后处理。\n"+
					"如需读取 Word 文档中的文本内容，请使用 bin/doc2md.py 将文档转换为 Markdown 后再用 read_file 读取。",
				path, b)
		}
	}

	lines := strings.Split(string(data), "\n")
	totalLines := len(lines)

	// Validate start_line
	if startLine < 1 {
		startLine = 1
	}
	if startLine > totalLines {
		return "", fmt.Errorf("start_line %d exceeds file length (%d lines)", startLine, totalLines)
	}

	// Determine end_line
	if endLine <= 0 || endLine > totalLines {
		endLine = totalLines
	}
	if endLine < startLine {
		endLine = startLine
	}

	// Limit output to 1000 lines
	if endLine-startLine+1 > 1000 {
		endLine = startLine + 999
	}

	// Build output with line numbers
	var result strings.Builder
	result.WriteString(fmt.Sprintf("File: %s (%d lines total)\n\n", path, totalLines))
	for i := startLine - 1; i < endLine; i++ {
		result.WriteString(fmt.Sprintf("%d | %s\n", i+1, lines[i]))
	}

	lineTruncated := endLine < totalLines

	// Check output size limit before appending truncation notices
	maxSize := 16384 // 16KB default
	if a.cfg != nil && a.cfg.LLM.ReadFileMaxSize > 0 {
		maxSize = a.cfg.LLM.ReadFileMaxSize
	}
	output := result.String()
	byteTruncated := len(output) > maxSize

	if byteTruncated {
		// Find the last complete line within the byte limit
		truncated := output[:maxSize]
		// Remove the incomplete last line
		lastNewline := strings.LastIndex(truncated, "\n")
		if lastNewline >= 0 {
			truncated = truncated[:lastNewline]
		}
		// Append truncation notice
		displayedLines := strings.Count(truncated, "\n") - 1 // subtract the initial "\n" after header
		notice := fmt.Sprintf(
			"\n⚠️ 文件内容超长，当前仅返回前 %d 字节（总内容 %d 字节，显示 %d 行 / 共 %d 行）。\n你可以使用 read_file 并指定 start_line/end_line 分段读取。\n",
			maxSize, len(output), displayedLines, totalLines)
		truncated += notice
		return truncated, nil
	}

	if lineTruncated {
		result.WriteString(fmt.Sprintf("... (showing %d-%d of %d, %d more lines not shown)\n", startLine, endLine, totalLines, totalLines-endLine))
	}

	return output, nil
}

// searchFilesTool searches for a regex pattern across files in a directory.
// It returns results with context lines, handles binary files, and enforces
// configurable limits on line length and total result size.
func (a *Agent) searchFilesTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("searchFilesTool called: args=%v", args)
	dirPath, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path argument is required")
	}

	pattern, ok := args["regex"].(string)
	if !ok {
		return "", fmt.Errorf("regex argument is required")
	}

	filePattern, _ := args["file_pattern"].(string)

	// Resolve relative paths
	if !filepath.IsAbs(dirPath) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("cannot get current working directory: %w", err)
		}
		dirPath = filepath.Join(cwd, dirPath)
	}

	// Compile regex
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex %q: %w", pattern, err)
	}

	// Get configurable limits from agent config
	maxLineLength := 8192
	maxResultBytes := 65536
	if a.cfg != nil {
		if a.cfg.LLM.SearchMaxLineLength > 0 {
			maxLineLength = a.cfg.LLM.SearchMaxLineLength
		}
		if a.cfg.LLM.SearchMaxResultBytes > 0 {
			maxResultBytes = a.cfg.LLM.SearchMaxResultBytes
		}
	}

	// Binary file extensions to skip
	binaryExts := map[string]bool{
		".exe": true, ".bin": true, ".o": true, ".a": true, ".so": true, ".dll": true, ".dylib": true,
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true, ".ico": true, ".svg": true, ".webp": true,
		".mp3": true, ".mp4": true, ".avi": true, ".mov": true, ".wav": true, ".flac": true,
		".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".xz": true, ".7z": true, ".rar": true,
		".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true, ".ppt": true, ".pptx": true,
		".ttf": true, ".otf": true, ".woff": true, ".woff2": true,
		".db": true, ".sqlite": true,
		".pyc": true, ".pyo": true, ".class": true, ".jar": true,
	}

	// Walk the directory
	var result strings.Builder
	var matchCount int
	var truncatedLineCount int
	var totalBytes int
	var headerWritten bool

	// Helper to write the header with match count info
	writeHeader := func() {
		if headerWritten {
			return
		}
		headerWritten = true
		if truncatedLineCount > 0 {
			result.WriteString(i18n.TF(i18n.KeySearchResultFoundTrunc, dirPath, matchCount, pattern, truncatedLineCount) + "\n\n")
		} else {
			result.WriteString(i18n.TF(i18n.KeySearchResultFound, dirPath, matchCount, pattern) + "\n\n")
		}
	}

	// Helper to write a line with truncation protection
	writeLine := func(line string) {
		if len(line) > maxLineLength {
			truncatedLineCount++
			line = line[:maxLineLength] + i18n.TF(i18n.KeySearchLineTruncated, len(line)-maxLineLength)
		}
		lineBytes := len(line) + 1 // +1 for newline
		if totalBytes+lineBytes > maxResultBytes {
			return // skip this line, we've hit the limit
		}
		result.WriteString(line + "\n")
		totalBytes += lineBytes
	}

	err = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible files
		}
		if info.IsDir() {
			return nil
		}

		// Skip binary files by extension
		ext := strings.ToLower(filepath.Ext(info.Name()))
		if binaryExts[ext] {
			return nil
		}

		// Check file pattern if specified
		if filePattern != "" {
			matched, err := filepath.Match(filePattern, info.Name())
			if err != nil || !matched {
				return nil
			}
		}

		// Read the file
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}

		// Detect binary content: check for null bytes in first 8KB
		checkLen := len(data)
		if checkLen > 8192 {
			checkLen = 8192
		}
		if bytes.IndexByte(data[:checkLen], 0) >= 0 {
			return nil // skip binary files
		}

		lines := strings.Split(string(data), "\n")
		fileMatched := false
		type matchInfo struct {
			lineNum int
			line    string
		}
		var fileMatches []matchInfo

		for i, line := range lines {
			if re.MatchString(line) {
				fileMatched = true
				fileMatches = append(fileMatches, matchInfo{lineNum: i + 1, line: line})
			}
		}

		if !fileMatched {
			return nil
		}

		// Check if we've hit the max result bytes limit before adding this file
		// Estimate: header + file name + context lines
		relPath, _ := filepath.Rel(dirPath, path)
		estimatedBytes := len(relPath) + 20 + len(fileMatches)*80
		if totalBytes+estimatedBytes > maxResultBytes && headerWritten {
			return filepath.SkipDir
		}

		// Write file header with context range
		writeHeader()
		firstLine := fileMatches[0].lineNum
		lastLine := fileMatches[len(fileMatches)-1].lineNum
		fileHeader := fmt.Sprintf("%s:%d-%d:", relPath, firstLine, lastLine)
		writeLine(fileHeader)

		// Determine context lines from config (default: 5)
		contextLines := 5
		if a.cfg != nil && a.cfg.LLM.SearchContextLines > 0 {
			contextLines = a.cfg.LLM.SearchContextLines
		}
		writtenLines := make(map[int]bool) // track which lines have been written to avoid duplicates
		for _, fm := range fileMatches {
			start := fm.lineNum - 1 - contextLines
			if start < 0 {
				start = 0
			}
			end := fm.lineNum - 1 + contextLines
			if end >= len(lines) {
				end = len(lines) - 1
			}
			for i := start; i <= end; i++ {
				if writtenLines[i] {
					continue
				}
				writtenLines[i] = true
				contextLine := fmt.Sprintf("%d: %s", i+1, lines[i])
				writeLine(contextLine)
			}
		}
		writeLine("") // blank line between files

		matchCount += len(fileMatches)
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	if matchCount == 0 {
		return i18n.TF(i18n.KeySearchResultNone, pattern, dirPath), nil
	}

	// If we didn't write the header (shouldn't happen, but just in case)
	if !headerWritten {
		writeHeader()
	}

	// Check if we hit the byte limit
	if totalBytes >= maxResultBytes {
		// Remove the last incomplete line and add a truncation notice
		finalResult := result.String()
		lastNewline := strings.LastIndex(finalResult, "\n")
		if lastNewline >= 0 {
			finalResult = finalResult[:lastNewline]
		}
		// Find the last blank line separator to cleanly truncate
		lastSep := strings.LastIndex(finalResult, "\n\n")
		if lastSep >= 0 {
			finalResult = finalResult[:lastSep+1]
		}
		finalResult += i18n.TF(i18n.KeySearchResultFoundPartial, dirPath, matchCount, pattern) + "\n"
		return finalResult, nil
	}

	return result.String(), nil
}

// listFilesForPromptResult holds the result of listFilesForPrompt.
type listFilesForPromptResult struct {
	listing   string // the file listing text
	truncated bool   // true if the listing was truncated due to maxEntries
}

// listFilesForPrompt lists files in a directory and returns the listing.
// depth controls recursion: 0 = top-level only, 1 = one level deep, etc.
// -1 means unlimited recursion. maxEntries limits the number of items returned.
// When maxEntries <= 0, no truncation is applied (returns all entries).
func listFilesForPrompt(dirPath string, depth int, maxEntries int) listFilesForPromptResult {
	var result strings.Builder
	var count int
	unlimited := maxEntries <= 0

	if depth < 0 || depth > 0 {
		filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !unlimited && count >= maxEntries {
				return filepath.SkipDir
			}
			relPath, _ := filepath.Rel(dirPath, path)
			if relPath == "." {
				return nil
			}
			// Check depth: count path separators
			if depth >= 0 {
				d := strings.Count(relPath, string(filepath.Separator))
				if d > depth {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
			if info.IsDir() {
				result.WriteString(relPath + "/\n")
			} else {
				result.WriteString(relPath + "\n")
			}
			count++
			return nil
		})
	} else {
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			return listFilesForPromptResult{}
		}
		total := len(entries)
		if !unlimited && total > maxEntries {
			entries = entries[:maxEntries]
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() {
				name += "/"
			}
			result.WriteString(name + "\n")
			count++
		}
		if count == 0 {
			result.WriteString("(empty directory)\n")
		}
	}

	truncated := !unlimited && count >= maxEntries
	if truncated {
		result.WriteString("(File list truncated. Use list_files on specific subdirectories if you need to explore further.)\n")
	}
	return listFilesForPromptResult{
		listing:   result.String(),
		truncated: truncated,
	}
}

// listFilesTool lists files and directories within the specified directory.
// depth controls recursion: 0 = top-level only (default), 1 = one level deep, etc.
// -1 means unlimited recursion. Returns at most maxItems items.
// maxItems is configurable via config (default: 256).
func (a *Agent) listFilesTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("listFilesTool called: args=%v", args)
	dirPath, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path argument is required")
	}

	// Resolve relative paths
	if !filepath.IsAbs(dirPath) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("cannot get current working directory: %w", err)
		}
		dirPath = filepath.Join(cwd, dirPath)
	}

	// Check recursive flag: integer depth
	depth := 0 // default: top-level only
	if r, ok := args["recursive"].(float64); ok {
		depth = int(r)
	}

	maxEntries := 100
	if a.cfg != nil && a.cfg.LLM.ListMaxItems > 0 {
		maxEntries = a.cfg.LLM.ListMaxItems
	}

	res := listFilesForPrompt(dirPath, depth, maxEntries)
	if res.listing == "" {
		return "", fmt.Errorf("cannot read directory %q", dirPath)
	}

	if res.truncated {
		// Save the full listing to a .md file under workspace tmp/
		fullRes := listFilesForPrompt(dirPath, depth, -1) // unlimited
		dirName := filepath.Base(dirPath)
		if dirName == "." || dirName == "" {
			dirName = "root"
		}
		mdDir := filepath.Join("tmp", "file-list")
		mdPath := filepath.Join(mdDir, dirName+".md")
		if err := os.MkdirAll(mdDir, 0755); err != nil {
			return "", fmt.Errorf("cannot create temp directory: %w", err)
		}
		mdContent := fmt.Sprintf("# 文件列表: %s\n\n", dirPath)
		mdContent += fullRes.listing
		if err := os.WriteFile(mdPath, []byte(mdContent), 0644); err != nil {
			return "", fmt.Errorf("cannot write file list to %s: %w", mdPath, err)
		}

		totalItems := strings.Count(fullRes.listing, "\n")
		var result strings.Builder
		result.WriteString(fmt.Sprintf("Directory: %s\n\n", dirPath))
		result.WriteString(res.listing)
		result.WriteString(fmt.Sprintf("\n\n✅ 完整文件列表已保存到: %s（共 %d 条目，超过限制 %d）。可用 read_file 读取查看完整内容。\n", mdPath, totalItems, maxEntries))
		return result.String(), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Directory: %s\n\n", dirPath))
	result.WriteString(res.listing)
	return result.String(), nil
}

// listCodeDefinitionNamesTool lists definition names in source code files at the top level of a directory.
func (a *Agent) listCodeDefinitionNamesTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("listCodeDefinitionNamesTool called: args=%v", args)
	dirPath, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path argument is required")
	}

	// Resolve relative paths
	if !filepath.IsAbs(dirPath) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("cannot get current working directory: %w", err)
		}
		dirPath = filepath.Join(cwd, dirPath)
	}

	// Read directory entries
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return "", fmt.Errorf("cannot read directory %q: %w", dirPath, err)
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Definitions in %s:\n\n", dirPath))

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process source code files
		ext := filepath.Ext(entry.Name())
		switch ext {
		case ".go", ".py", ".js", ".ts", ".java", ".c", ".h", ".cpp", ".hpp", ".rs", ".rb", ".php":
			// supported
		default:
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())
		data, readErr := os.ReadFile(filePath)
		if readErr != nil {
			continue
		}

		content := string(data)
		var definitions []string

		switch ext {
		case ".go":
			// Match Go function/method/type definitions
			goRe := regexp.MustCompile(`(?:^|\n)\s*(?:func\s+(?:\([^)]*\)\s*)?(\w+)|type\s+(\w+)\s+(?:struct|interface|func))`)
			matches := goRe.FindAllStringSubmatch(content, -1)
			for _, m := range matches {
				name := m[1]
				if name == "" {
					name = m[2]
				}
				if name != "" {
					definitions = append(definitions, fmt.Sprintf("  func/type: %s", name))
				}
			}
		case ".py":
			pyRe := regexp.MustCompile(`(?:^|\n)\s*(?:def\s+(\w+)|class\s+(\w+))`)
			matches := pyRe.FindAllStringSubmatch(content, -1)
			for _, m := range matches {
				name := m[1]
				if name == "" {
					name = m[2]
				}
				definitions = append(definitions, fmt.Sprintf("  def/class: %s", name))
			}
		case ".js", ".ts":
			jsRe := regexp.MustCompile(`(?:^|\n)\s*(?:function\s+(\w+)|(?:export\s+)?(?:const|let|var)\s+(\w+)\s*[:=]\s*(?:function|\(|=>)|class\s+(\w+))`)
			matches := jsRe.FindAllStringSubmatch(content, -1)
			for _, m := range matches {
				name := m[1]
				if name == "" {
					name = m[2]
				}
				if name == "" {
					name = m[3]
				}
				if name != "" {
					definitions = append(definitions, fmt.Sprintf("  func/class: %s", name))
				}
			}
		case ".java":
			javaRe := regexp.MustCompile(`(?:^|\n)\s*(?:public|private|protected)?\s*(?:static\s+)?(?:class|interface|enum)\s+(\w+)`)
			matches := javaRe.FindAllStringSubmatch(content, -1)
			for _, m := range matches {
				definitions = append(definitions, fmt.Sprintf("  class: %s", m[1]))
			}
		default:
			// Generic: look for function/class definitions
			genericRe := regexp.MustCompile(`(?:^|\n)\s*(?:function|def|class|type|struct)\s+(\w+)`)
			matches := genericRe.FindAllStringSubmatch(content, -1)
			for _, m := range matches {
				definitions = append(definitions, fmt.Sprintf("  def: %s", m[1]))
			}
		}

		if len(definitions) > 0 {
			result.WriteString(fmt.Sprintf("%s:\n", entry.Name()))
			for _, d := range definitions {
				result.WriteString(d + "\n")
			}
			result.WriteString("\n")
		}
	}

	return result.String(), nil
}

// replaceInFileTool replaces sections of content in an existing file using SEARCH/REPLACE.
// Supports multiple SEARCH/REPLACE blocks, relative path resolution, backup mechanism,
// and returns detailed diff information.
func (a *Agent) replaceInFileTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("replaceInFileTool called: args=%v", args)
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path argument is required")
	}

	// Resolve relative paths (like readFileTool does)
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("cannot get current working directory: %w", err)
		}
		path = filepath.Join(cwd, path)
	}

	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("cannot read file %q: %w", path, err)
	}

	content := string(data)
	originalContent := content

	// Collect all SEARCH/REPLACE blocks from the "replacements" array
	// Each replacement is an object with "search", "replace", and optional "start_line" fields.
	// "start_line" refers to the line number in the original file (before any replacements).
	type replacementBlock struct {
		search    string
		replace   string
		startLine int // 0 means not specified
	}
	var blocks []replacementBlock

	if replacementsRaw, ok := args["replacements"].([]interface{}); ok {
		for i, r := range replacementsRaw {
			rMap, ok := r.(map[string]interface{})
			if !ok {
				return "", fmt.Errorf("replacements[%d] must be an object with 'search' and 'replace' fields", i)
			}
			search, ok := rMap["search"].(string)
			if !ok {
				return "", fmt.Errorf("replacements[%d].search is required and must be a string", i)
			}
			replace, ok := rMap["replace"].(string)
			if !ok {
				return "", fmt.Errorf("replacements[%d].replace is required and must be a string", i)
			}
			block := replacementBlock{
				search:  search,
				replace: replace,
			}
			// Optional start_line (1-based, refers to original file)
			if sl, ok := rMap["start_line"].(float64); ok {
				block.startLine = int(sl)
			}
			blocks = append(blocks, block)
		}
	}

	if len(blocks) == 0 {
		return "", fmt.Errorf("replacements argument is required: an array of objects with 'search' and 'replace' fields")
	}

	// lineDiff represents a single line change in a replacement block
	type lineDiff struct {
		lineNum    int    // line number in current content (0 means no line number, for inserted lines)
		oldLine    string // the original line content (empty for inserted lines)
		newLine    string // the new line content (empty for deleted lines)
		isModified bool   // true if this line was changed
		isInserted bool   // true if this is a newly inserted line
		isDeleted  bool   // true if this line was deleted
	}

	// Track replacements for diff output
	type replacementInfo struct {
		search       string
		replace      string
		startLine    int
		endLine      int
		searchLines  int
		replaceLines int
		success      bool
		err          string
		// Per-line diff details
		lineDiffs []lineDiff
	}
	var replacements []replacementInfo

	// lineOffset tracks the cumulative line count change from previous replacements.
	// Positive means lines were added, negative means lines were removed.
	lineOffset := 0

	// Validate that blocks with start_line are provided in ascending order.
	// Blocks without start_line are excluded from this check.
	var lastStartLine int
	hasStartLineBlock := false
	for i, block := range blocks {
		if block.startLine == 0 {
			continue
		}
		if hasStartLineBlock && block.startLine < lastStartLine {
			return "", fmt.Errorf("replacements[%d]: start_line %d is less than previous start_line %d. Blocks with start_line must be ordered by ascending line number.", i, block.startLine, lastStartLine)
		}
		lastStartLine = block.startLine
		hasStartLineBlock = true
	}

	// Perform all replacements sequentially
	for i := 0; i < len(blocks); i++ {
		block := blocks[i]
		search := block.search
		replace := block.replace

		// Determine the search position in the current (modified) content
		var idx int
		found := false

		if block.startLine > 0 {
			// Use start_line for precise positioning.
			// Adjust for line offset from previous replacements:
			// adjustedLine = originalStartLine + lineOffset
			adjustedLine := block.startLine + lineOffset
			if adjustedLine < 1 {
				adjustedLine = 1
			}

			// Convert adjusted line number to byte position in current content
			contentLines := strings.Split(content, "\n")
			if adjustedLine > len(contentLines) {
				errMsg := fmt.Sprintf("SEARCH block %d: start_line %d (adjusted to %d after previous replacements) exceeds file length (%d lines).", i+1, block.startLine, adjustedLine, len(contentLines))
				replacements = append(replacements, replacementInfo{
					search:  search,
					replace: replace,
					success: false,
					err:     errMsg,
				})
				continue
			}

			// Calculate byte offset for the adjusted line (0-based line index)
			lineIdx := adjustedLine - 1
			byteOffset := 0
			for j := 0; j < lineIdx; j++ {
				byteOffset += len(contentLines[j]) + 1 // +1 for newline
			}

			// Search for the content starting from this byte offset
			searchIdx := strings.Index(content[byteOffset:], search)
			if searchIdx >= 0 {
				idx = byteOffset + searchIdx
				found = true
			} else {
				// Fallback: try whitespace-tolerant match near the target line
				searchLines := strings.Split(search, "\n")
				maxCheckLines := len(contentLines) - lineIdx
				if len(searchLines) < maxCheckLines {
					maxCheckLines = len(searchLines)
				}
				for checkIdx := lineIdx; checkIdx <= lineIdx+maxCheckLines && checkIdx+len(searchLines) <= len(contentLines); checkIdx++ {
					match := true
					for j, sLine := range searchLines {
						cLine := contentLines[checkIdx+j]
						if strings.TrimRight(sLine, " \t\r") != strings.TrimRight(cLine, " \t\r") {
							match = false
							break
						}
					}
					if match {
						matchedContent := strings.Join(contentLines[checkIdx:checkIdx+len(searchLines)], "\n")
						searchIdx = strings.Index(content, matchedContent)
						if searchIdx >= 0 {
							idx = searchIdx
							found = true
							break
						}
					}
				}
			}
		} else {
			// No start_line: search the entire content (exact match first)
			idx = strings.Index(content, search)
			if idx >= 0 {
				found = true
			}
		}

		if !found {
			// Try full-content fuzzy match as last resort
			searchLines := strings.Split(search, "\n")
			contentLines := strings.Split(content, "\n")
			for lineIdx := 0; lineIdx <= len(contentLines)-len(searchLines); lineIdx++ {
				match := true
				for j, sLine := range searchLines {
					cLine := contentLines[lineIdx+j]
					if strings.TrimRight(sLine, " \t\r") != strings.TrimRight(cLine, " \t\r") {
						match = false
						break
					}
				}
				if match {
					matchedContent := strings.Join(contentLines[lineIdx:lineIdx+len(searchLines)], "\n")
					searchIdx := strings.Index(content, matchedContent)
					if searchIdx >= 0 {
						idx = searchIdx
						found = true
						break
					}
				}
			}
		}

		if !found {
			// Provide helpful error with closest match info
			closestLine := findClosestMatch(content, search)
			errMsg := fmt.Sprintf("SEARCH block %d not found in file %q.", i+1, path)
			if block.startLine > 0 {
				errMsg += fmt.Sprintf(" Specified start_line=%d", block.startLine)
			}
			if closestLine > 0 {
				errMsg += fmt.Sprintf(" Closest match found near line %d. The SEARCH content must match the file exactly (including whitespace and indentation).", closestLine)
			} else {
				errMsg += " The SEARCH content must match the file exactly (including whitespace and indentation)."
			}
			replacements = append(replacements, replacementInfo{
				search:  search,
				replace: replace,
				success: false,
				err:     errMsg,
			})
			continue
		}

		// Verify that the matched content exactly matches the search string
		matchedText := content[idx : idx+len(search)]
		if matchedText != search {
			// Show the difference for debugging
			errMsg := fmt.Sprintf("SEARCH block %d: matched content does not match search exactly. Expected %d bytes but got different content. This may be due to whitespace differences.", i+1, len(search))
			replacements = append(replacements, replacementInfo{
				search:  search,
				replace: replace,
				success: false,
				err:     errMsg,
			})
			continue
		}

		// Calculate line numbers for the matched content (in current content)
		beforeMatch := content[:idx]
		currentStartLine := strings.Count(beforeMatch, "\n") + 1
		currentEndLine := currentStartLine + strings.Count(search, "\n")
		searchLineCount := strings.Count(search, "\n") + 1
		replaceLineCount := strings.Count(replace, "\n") + 1

		// Build per-line diff information
		searchLines := strings.Split(search, "\n")
		replaceLines := strings.Split(replace, "\n")
		var lineDiffs []lineDiff

		// Compare search lines vs replace lines line by line
		maxLines := searchLineCount
		if replaceLineCount > maxLines {
			maxLines = replaceLineCount
		}
		for lineIdx := 0; lineIdx < maxLines; lineIdx++ {
			var oldLine, newLine string
			hasOld := lineIdx < searchLineCount
			hasNew := lineIdx < replaceLineCount

			if hasOld {
				oldLine = searchLines[lineIdx]
			}
			if hasNew {
				newLine = replaceLines[lineIdx]
			}

			if hasOld && hasNew {
				if oldLine == newLine {
					// Unchanged line - skip (don't add to diffs)
					continue
				}
				// Modified line
				lineDiffs = append(lineDiffs, lineDiff{
					lineNum:    currentStartLine + lineIdx,
					oldLine:    oldLine,
					newLine:    newLine,
					isModified: true,
				})
			} else if hasOld && !hasNew {
				// Deleted line
				lineDiffs = append(lineDiffs, lineDiff{
					lineNum:   currentStartLine + lineIdx,
					oldLine:   oldLine,
					isDeleted: true,
				})
			} else if !hasOld && hasNew {
				// Inserted line (no corresponding line number)
				lineDiffs = append(lineDiffs, lineDiff{
					newLine:    newLine,
					isInserted: true,
				})
			}
		}

		// Update lineOffset: how many lines this replacement adds/removes
		lineOffset += replaceLineCount - searchLineCount

		// Replace only the first occurrence
		content = content[:idx] + replace + content[idx+len(search):]

		replacements = append(replacements, replacementInfo{
			search:       search,
			replace:      replace,
			startLine:    currentStartLine,
			endLine:      currentEndLine,
			searchLines:  searchLineCount,
			replaceLines: replaceLineCount,
			success:      true,
			lineDiffs:    lineDiffs,
		})
	}

	// Check if any replacements failed
	failedCount := 0
	for _, r := range replacements {
		if !r.success {
			failedCount++
		}
	}

	if failedCount == len(replacements) {
		// All failed, return the first error
		for _, r := range replacements {
			if !r.success {
				return "", fmt.Errorf("%s", r.err)
			}
		}
	}

	// Create backup before writing
	backupPath := ""
	if a.cfg != nil && a.cfg.LLM.ToolTimeout >= 0 {
		// Use workspace tmp directory for backup
		tmpDir := filepath.Join(filepath.Dir(path), "..", "tmp")
		if absTmp, err := filepath.Abs(tmpDir); err == nil {
			if info, err := os.Stat(absTmp); err == nil && info.IsDir() {
				backupDir := absTmp
				baseName := filepath.Base(path)
				backupName := fmt.Sprintf("%s.bak.%d", baseName, time.Now().UnixNano())
				backupPath = filepath.Join(backupDir, backupName)
				if err := os.WriteFile(backupPath, []byte(originalContent), 0644); err != nil {
					backupPath = "" // backup failed, continue anyway
				}
			}
		}
	}

	// Write back
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("cannot write file %q: %w", path, err)
	}

	// Build result with diff information
	var result strings.Builder
	totalSuccess := 0
	totalFailed := 0
	for _, r := range replacements {
		if r.success {
			totalSuccess++
		} else {
			totalFailed++
		}
	}

	result.WriteString(fmt.Sprintf("File: %s\n", path))
	if totalFailed > 0 {
		result.WriteString(fmt.Sprintf("⚠️  %d of %d replacements succeeded, %d failed:\n\n", totalSuccess, len(replacements), totalFailed))
	} else {
		result.WriteString(fmt.Sprintf("✅  All %d replacements succeeded:\n\n", len(replacements)))
	}

	for i, r := range replacements {
		if r.success {
			lineRange := fmt.Sprintf("L%d", r.startLine)
			if r.endLine > r.startLine {
				lineRange = fmt.Sprintf("L%d-L%d", r.startLine, r.endLine)
			}
			result.WriteString(fmt.Sprintf("  [%d/%d] %s (%d lines → %d lines)\n", i+1, len(replacements), lineRange, r.searchLines, r.replaceLines))

			// Output per-line diff details
			for _, ld := range r.lineDiffs {
				switch {
				case ld.isModified:
					// Modified line: old -----> new
					result.WriteString(fmt.Sprintf("    %d*: %s -----> %s\n", ld.lineNum, ld.oldLine, ld.newLine))
				case ld.isDeleted:
					// Deleted line: show with line number and asterisk
					result.WriteString(fmt.Sprintf("    %d*: %s\n", ld.lineNum, ld.oldLine))
				case ld.isInserted:
					// Inserted line: no line number, just asterisk
					result.WriteString(fmt.Sprintf("    *: %s\n", ld.newLine))
				}
			}
		} else {
			result.WriteString(fmt.Sprintf("  [%d/%d] ❌ FAILED: %s\n", i+1, len(replacements), r.err))
		}
	}

	if backupPath != "" {
		result.WriteString(fmt.Sprintf("\n📦 Backup saved to: %s", backupPath))
	}

	return result.String(), nil
}

// findClosestMatch attempts to find the closest matching line in content for the given search text.
// Returns the line number (1-based) of the closest match, or 0 if no reasonable match found.
func findClosestMatch(content, search string) int {
	searchLines := strings.Split(search, "\n")
	if len(searchLines) == 0 {
		return 0
	}

	contentLines := strings.Split(content, "\n")
	firstSearchLine := strings.TrimSpace(searchLines[0])
	if firstSearchLine == "" {
		return 0
	}

	// Find the line in content that best matches the first line of the search
	bestScore := 0
	bestLine := 0
	for i, cLine := range contentLines {
		trimmed := strings.TrimSpace(cLine)
		if trimmed == "" {
			continue
		}
		// Calculate similarity score based on common prefix length
		score := 0
		minLen := len(firstSearchLine)
		if len(trimmed) < minLen {
			minLen = len(trimmed)
		}
		for j := 0; j < minLen; j++ {
			if j < len(firstSearchLine) && j < len(trimmed) && firstSearchLine[j] == trimmed[j] {
				score++
			} else {
				break
			}
		}
		if score > bestScore {
			bestScore = score
			bestLine = i + 1
		}
	}

	if bestScore > 3 { // At least 3 matching characters to be considered a close match
		return bestLine
	}
	return 0
}

// writeToFileTool creates, overwrites, or appends to a file with optional content.
// The 'mode' parameter controls the file operation:
//   - "new" (default): creates a new file. Fails if the file already exists.
//   - "rewrite": overwrites an existing file with new content. Fails if the file doesn't exist.
//   - "append": appends content to an existing file. Fails if the file doesn't exist.
func (a *Agent) writeToFileTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("writeToFileTool called: args=%v", args)
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path argument is required")
	}

	// Determine mode (required)
	mode, ok := args["mode"].(string)
	if !ok {
		return "", fmt.Errorf("mode argument is required: one of: new, rewrite, append")
	}

	// Validate mode value
	validModes := map[string]bool{"new": true, "rewrite": true, "append": true}
	if !validModes[mode] {
		return "", fmt.Errorf("invalid mode %q: must be one of: new, rewrite, append", mode)
	}

	// Check file existence
	fileExists := true
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fileExists = false
	}

	switch mode {
	case "new":
		if fileExists {
			return "", fmt.Errorf("file %q already exists. mode 'new' can only create NEW files. Use mode 'rewrite' to overwrite existing files, or mode 'append' to append content to existing files", path)
		}
	case "rewrite", "append":
		if !fileExists {
			return "", fmt.Errorf("file %q does not exist. mode '%s' requires an existing file. Use mode 'new' to create a new file", path, mode)
		}
	}

	content, _ := args["content"].(string)

	// Create parent directories if they don't exist (only for 'new' mode)
	if mode == "new" {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("cannot create directories for %q: %w", path, err)
		}
	}

	// Write the file
	var writeErr error
	if mode == "append" {
		// Open existing file and append content
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return "", fmt.Errorf("cannot open file %q for append: %w", path, err)
		}
		defer f.Close()
		_, writeErr = f.WriteString(content)
	} else {
		writeErr = os.WriteFile(path, []byte(content), 0644)
	}
	if writeErr != nil {
		return "", fmt.Errorf("cannot write to file %q: %w", path, writeErr)
	}

	if content == "" {
		return fmt.Sprintf("(%s) %s", mode, path), nil
	}
	return fmt.Sprintf("(%s) %d bytes written to %s", mode, len(content), path), nil
}
