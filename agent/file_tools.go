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

	// Determine start and end lines
	startLine := 1
	endLine := 0 // 0 means read to end
	if s, ok := args["start_line"].(float64); ok {
		startLine = int(s)
	}
	if e, ok := args["end_line"].(float64); ok {
		endLine = int(e)
	}

	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("cannot read file %q: %w", path, err)
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
	result.WriteString(fmt.Sprintf("File: %s (%d lines total, showing %d-%d)\n\n", path, totalLines, startLine, endLine))
	for i := startLine - 1; i < endLine; i++ {
		result.WriteString(fmt.Sprintf("%d | %s\n", i+1, lines[i]))
	}

	if endLine < totalLines {
		result.WriteString(fmt.Sprintf("... (%d more lines)\n", totalLines-endLine))
	}

	return result.String(), nil
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
func (a *Agent) replaceInFileTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("replaceInFileTool called: args=%v", args)
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path argument is required")
	}

	search, ok := args["search"].(string)
	if !ok {
		return "", fmt.Errorf("search argument is required")
	}

	replace, ok := args["replace"].(string)
	if !ok {
		return "", fmt.Errorf("replace argument is required")
	}

	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("cannot read file %q: %w", path, err)
	}

	content := string(data)

	// Find the search string
	idx := strings.Index(content, search)
	if idx < 0 {
		return "", fmt.Errorf("search content not found in file %q. The SEARCH content must match the file exactly (including whitespace and indentation)", path)
	}

	// Replace only the first occurrence
	newContent := content[:idx] + replace + content[idx+len(search):]

	// Write back
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("cannot write file %q: %w", path, err)
	}

	return fmt.Sprintf("Successfully replaced content in %s", path), nil
}

// writeToFileTool writes content to a file, creating directories as needed.
func (a *Agent) writeToFileTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("writeToFileTool called: args=%v", args)
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path argument is required")
	}

	content, ok := args["content"].(string)
	if !ok {
		return "", fmt.Errorf("content argument is required")
	}

	// Create parent directories if they don't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("cannot create directories for %q: %w", path, err)
	}

	// Write the file
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("cannot write file %q: %w", path, err)
	}

	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path), nil
}
