// Author: L.Shuang
// Created: 2026-09-08
// Last Modified: 2026-09-08
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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
)

// TestLimitToolResult_NoLimit verifies that when ToolResultMaxSize is 0
// (unlimited), the result passes through unchanged (UC-0006).
func TestLimitToolResult_NoLimit(t *testing.T) {
	a := &Agent{cfg: config.DefaultConfig()}
	a.cfg.LLM.ToolResultMaxSize = 0
	content := strings.Repeat("line\n", 10000) // ~50KB
	got := a.limitToolResult("execute_command", content)
	if got != content {
		t.Fatalf("unlimited mode should return content unchanged, got %d bytes want %d", len(got), len(content))
	}
}

// TestLimitToolResult_UnderLimit verifies that content within the limit is
// returned unchanged (UC-0002).
func TestLimitToolResult_UnderLimit(t *testing.T) {
	a := &Agent{cfg: config.DefaultConfig()}
	a.cfg.LLM.ToolResultMaxSize = 1024
	content := "short result"
	got := a.limitToolResult("execute_command", content)
	if got != content {
		t.Fatalf("under-limit content should be unchanged, got %q", got)
	}
}

// TestLimitToolResult_OverLimit verifies that oversized content is truncated
// to the head, a notice with file path / size / lines is appended, and the
// full content is saved to tmp/tool-result/ (UC-0003, UC-0007).
func TestLimitToolResult_OverLimit(t *testing.T) {
	a := &Agent{cfg: config.DefaultConfig()}
	a.cfg.LLM.ToolResultMaxSize = 1024

	// Build content with multiple lines, total > 1024 bytes.
	var sb strings.Builder
	for i := 0; i < 200; i++ {
		sb.WriteString("line-")
		sb.WriteString(strings.Repeat("x", 20))
		sb.WriteString("\n")
	}
	content := sb.String() // ~200 * 26 = 5200 bytes

	got := a.limitToolResult("execute_command", content)

	// The result must be shorter than the original.
	if len(got) >= len(content) {
		t.Fatalf("truncated result should be shorter than original: got %d >= want < %d", len(got), len(content))
	}

	// The notice must contain the file path, total size, and line count.
	if !strings.Contains(got, "tmp/tool-result") {
		t.Errorf("notice should mention tmp/tool-result, got: %s", got)
	}
	if !strings.Contains(got, "5200") {
		t.Errorf("notice should mention total size 5200, got: %s", got)
	}
	if !strings.Contains(got, "200") {
		t.Errorf("notice should mention line count 200, got: %s", got)
	}

	// The saved file must exist and contain the full content.
	path := extractSavedPath(got)
	if path == "" {
		t.Fatalf("could not extract saved file path from notice: %s", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("saved file %s should exist: %v", path, err)
	}
	if string(data) != content {
		t.Fatalf("saved file content mismatch: got %d bytes want %d", len(data), len(content))
	}
}

// TestLimitToolResult_HeadKept verifies that the head of the content is kept
// (the beginning of the result is preserved) (UC-0003).
func TestLimitToolResult_HeadKept(t *testing.T) {
	a := &Agent{cfg: config.DefaultConfig()}
	a.cfg.LLM.ToolResultMaxSize = 1024

	content := "HEAD-MARKER\n" + strings.Repeat("y", 5000)
	got := a.limitToolResult("execute_command", content)

	if !strings.HasPrefix(got, "HEAD-MARKER") {
		t.Errorf("truncated result should keep the head, got prefix: %.40q", got)
	}
}

// TestLimitToolResult_SanitizeToolName verifies that tool names with special
// characters are sanitized for the file name (UC-0007).
func TestLimitToolResult_SanitizeToolName(t *testing.T) {
	if got := sanitizeToolName("execute_command"); got != "execute_command" {
		t.Errorf("sanitizeToolName(execute_command) = %q, want execute_command", got)
	}
	if got := sanitizeToolName("a/b:c"); got != "a_b_c" {
		t.Errorf("sanitizeToolName(a/b:c) = %q, want a_b_c", got)
	}
	if got := sanitizeToolName(""); got != "tool" {
		t.Errorf("sanitizeToolName(empty) = %q, want tool", got)
	}
}

// TestCountLines verifies line counting.
func TestCountLines(t *testing.T) {
	if got := countLines(""); got != 0 {
		t.Errorf("countLines(empty) = %d, want 0", got)
	}
	if got := countLines("a\nb\nc"); got != 3 {
		t.Errorf("countLines(a\\nb\\nc) = %d, want 3", got)
	}
	if got := countLines("a\nb\nc\n"); got != 4 {
		t.Errorf("countLines(a\\nb\\nc\\n) = %d, want 4", got)
	}
}

// TestTruncateToolResultHead verifies head truncation cuts at a line boundary.
func TestTruncateToolResultHead(t *testing.T) {
	content := "line1\nline2\nline3\nline4\n"
	got := truncateToolResultHead(content, 12) // "line1\nline2\nl" -> cut at last newline -> "line1\nline2"
	if got != "line1\nline2" {
		t.Errorf("truncateToolResultHead = %q, want %q", got, "line1\nline2")
	}
}

// extractSavedPath pulls the saved file path out of the truncation notice.
// The notice format is: "... saved to: <path> ..." (zh) or
// "... saved to: <path> ..." (en).
func extractSavedPath(notice string) string {
	// The path is the last token that starts with "tmp/tool-result".
	idx := strings.Index(notice, "tmp/tool-result")
	if idx < 0 {
		return ""
	}
	rest := notice[idx:]
	// Path ends at whitespace or end of string.
	end := strings.IndexAny(rest, " \n")
	if end < 0 {
		return rest
	}
	return rest[:end]
}

// TestLimitToolResult_ConfigNil verifies that a nil config is safe.
func TestLimitToolResult_ConfigNil(t *testing.T) {
	a := &Agent{}
	content := strings.Repeat("z", 10000)
	got := a.limitToolResult("execute_command", content)
	if got != content {
		t.Fatalf("nil config should return content unchanged")
	}
}

// TestToolResultTruncatedI18n verifies the i18n key resolves (UC-0001).
func TestToolResultTruncatedI18n(t *testing.T) {
	msg := i18n.TF(i18n.KeyToolResultTruncated, 1024, 5000, 200, 3976, "tmp/tool-result/x.txt")
	if !strings.Contains(msg, "tmp/tool-result/x.txt") {
		t.Errorf("i18n message should contain the file path, got: %s", msg)
	}
	if !strings.Contains(msg, "1024") {
		t.Errorf("i18n message should contain the limit, got: %s", msg)
	}
}

// TestSaveToolResultFile verifies the file is written under tmp/tool-result/
// with a .txt extension (UC-0007).
func TestSaveToolResultFile(t *testing.T) {
	path, err := saveToolResultFile("execute_command", "hello world")
	if err != nil {
		t.Fatalf("saveToolResultFile failed: %v", err)
	}
	defer os.Remove(path)

	if !strings.HasPrefix(path, filepath.Join("tmp", "tool-result")) {
		t.Errorf("saved path should be under tmp/tool-result, got: %s", path)
	}
	if !strings.HasSuffix(path, ".txt") {
		t.Errorf("saved path should end with .txt, got: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read saved file: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("saved content mismatch: got %q", string(data))
	}
}
