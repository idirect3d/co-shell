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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
)

// toolResultDir is the directory (relative to the workspace) where full tool
// results that exceed the context limit are saved (FEATURE-491).
const toolResultDir = "tmp/tool-result"

// limitToolResult applies the FEATURE-491 context length limit to a tool
// result before it enters the LLM context. When the result exceeds
// cfg.LLM.ToolResultMaxSize (0 = unlimited), only the head (up to the limit)
// is kept, the full content is saved to tmp/tool-result/, and a notice with
// the file path / total size / total lines / truncated bytes is appended so
// the LLM can decide whether to read more.
func (a *Agent) limitToolResult(toolName, result string) string {
	if a.cfg == nil || a.cfg.LLM.ToolResultMaxSize <= 0 {
		return result
	}
	maxSize := a.cfg.LLM.ToolResultMaxSize
	if len(result) <= maxSize {
		return result
	}

	// Save the full content to a text file under tmp/tool-result/.
	path, err := saveToolResultFile(toolName, result)
	if err != nil {
		// If saving fails, still truncate but note the failure.
		log.Warn("limitToolResult: failed to save full tool result for %s: %v", toolName, err)
		return truncateToolResultHead(result, maxSize) +
			fmt.Sprintf("\n\n⚠️ %s", i18n.TF(i18n.KeyToolResultTruncated, maxSize, len(result), countLines(result), len(result)-maxSize, "(save failed)"))
	}

	truncated := truncateToolResultHead(result, maxSize)
	notice := i18n.TF(i18n.KeyToolResultTruncated, maxSize, len(result), countLines(result), len(result)-maxSize, path)
	return truncated + notice
}

// truncateToolResultHead keeps only the head of the content up to maxSize
// bytes, cutting at the last complete line boundary to avoid a dangling
// partial line.
func truncateToolResultHead(content string, maxSize int) string {
	if len(content) <= maxSize {
		return content
	}
	head := content[:maxSize]
	// Cut at the last newline within the head so the truncated content ends
	// on a complete line.
	if idx := strings.LastIndex(head, "\n"); idx >= 0 {
		head = head[:idx]
	}
	return head
}

// countLines returns the number of lines in the content (newline-separated).
func countLines(content string) int {
	if content == "" {
		return 0
	}
	return strings.Count(content, "\n") + 1
}

// saveToolResultFile writes the full tool result to tmp/tool-result/ with a
// name of the form {toolName}-{timestamp}.txt and returns the file path.
func saveToolResultFile(toolName, content string) (string, error) {
	dir := toolResultDir
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("cannot create tool result directory: %w", err)
	}
	name := fmt.Sprintf("%s-%s.txt", sanitizeToolName(toolName), time.Now().Format("20060102-150405"))
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("cannot write tool result to %s: %w", path, err)
	}
	return path, nil
}

// sanitizeToolName makes a tool name safe for use in a file name.
func sanitizeToolName(name string) string {
	if name == "" {
		return "tool"
	}
	var sb strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}
	return sb.String()
}
