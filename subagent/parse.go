// Author: L.Shuang
// Created: 2026-09-27
// Last Modified: 2026-09-27
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

package subagent

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"unicode/utf8"
)

// streamEvent mirrors one JSON-Lines line written by the sub-agent's
// --output-format json renderer (agent.StreamRenderer, FEATURE-307b). Only the
// fields the manager cares about are decoded.
type streamEvent struct {
	Type  string            `json:"type"`
	Level string            `json:"level"`
	Chan  string            `json:"chan"`
	Text  string            `json:"text"`
	Meta  map[string]string `json:"meta"`
}

// Event types emitted by the sub-agent stream renderer (subset used here).
const (
	eventContentChunk  = "content_chunk"
	eventContent       = "content"
	eventThinkingChunk = "thinking_chunk"
	eventThinking      = "thinking"
	eventToolCall      = "tool_call"
	eventToolCallDiff  = "tool_call_diff"
)

// truncationSuffix marks a result cut off by the result limit.
const truncationSuffix = "\n… [truncated]"

// parseStream extracts the sub-agent's final answer and its tool-call trail
// from a JSON-Lines log.
//
// The final answer is the LAST uninterrupted run of LLM content in the log:
// every thinking segment and every tool call closes the current run, so the
// text collected after the last tool call wins (thinking content is never
// part of it). Logs that cannot be read/parsed simply yield an empty result.
func parseStream(path string, limit int) (string, []string) {
	f, err := os.Open(path)
	if err != nil {
		return "", nil
	}
	defer f.Close()

	var (
		cur       strings.Builder
		final     string
		toolCalls []string
	)
	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			final = s
		}
		cur.Reset()
	}

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var ev streamEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		switch ev.Type {
		case eventContentChunk, eventContent:
			cur.WriteString(ev.Text)
		case eventThinkingChunk, eventThinking:
			flush()
		case eventToolCall:
			flush()
			if name := toolName(ev); name != "" {
				toolCalls = append(toolCalls, name)
			}
		case eventToolCallDiff:
			flush()
		}
	}
	flush()

	return truncateResult(final, limit), toolCalls
}

// toolName picks the tool name reported by a tool_call event: the structured
// Meta["name"] when present, otherwise the plain Text payload.
func toolName(ev streamEvent) string {
	if name := strings.TrimSpace(ev.Meta["name"]); name != "" {
		return name
	}
	return strings.TrimSpace(ev.Text)
}

// truncateResult caps s at limit bytes without splitting a UTF-8 rune and
// appends a visible marker so the parent LLM knows the tail was dropped.
func truncateResult(s string, limit int) string {
	if limit <= 0 || len(s) <= limit {
		return s
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + truncationSuffix
}

// tailFile returns the last maxBytes bytes of a text file (used for the log
// tail returned by sub_agent_status). Missing files yield an empty string.
func tailFile(path string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return ""
	}
	size := info.Size()
	start := int64(0)
	if size > int64(maxBytes) {
		start = size - int64(maxBytes)
	}
	buf := make([]byte, size-start)
	n, err := f.ReadAt(buf, start)
	if n <= 0 {
		return ""
	}
	if err != nil && n < len(buf) {
		buf = buf[:n]
	}
	s := string(buf)
	// Trim a leading partial rune/line so the tail starts cleanly.
	if start > 0 {
		if idx := strings.IndexByte(s, '\n'); idx >= 0 {
			s = s[idx+1:]
		}
	}
	return strings.TrimRight(s, "\n")
}
