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

// Meta keys of a tool_call event emitted by the stream renderer. The subagent
// package cannot import the agent package (agent imports subagent), so the two
// keys it needs are duplicated here.
const (
	metaKeyPhase       = "phase"
	metaKeyToolSummary = "tool_summary"
)

// Tool-call phases reported in Meta["phase"].
const (
	phaseResult = "result"
)

// completionTool is the terminal tool of the agent loop: the iteration that
// runs it ends immediately (no further LLM call), so its result — not a
// content event — carries the sub-agent's final answer.
const completionTool = "attempt_completion"

// maxToolLabelLen bounds the label derived from a tool_call event text when the
// event carries no structured tool name; longer text is dropped so command
// output never leaks into the tool-call trail.
const maxToolLabelLen = 64

// parseStream extracts the sub-agent's final answer and its tool-call trail
// from a JSON-Lines log.
//
// The final answer is the LAST thing the agent produced, resolved by a fallback
// chain:
//
//  1. the last uninterrupted run of LLM content in the log (every thinking
//     segment and every tool call closes the current run, so reasoning text is
//     never part of it);
//  2. when that run is empty — the common case, because co-shell finishes a
//     task by calling attempt_completion, whose text is echoed as a tool
//     result rather than as content — the result of the last
//     attempt_completion tool call.
//
// Both sources are ordered: whichever of the two comes last in the log wins, so
// a sub-agent that keeps talking after attempt_completion is still reported by
// its newest content. Logs that cannot be read/parsed simply yield an empty
// result.
func parseStream(path string, limit int) (string, []string) {
	f, err := os.Open(path)
	if err != nil {
		return "", nil
	}
	defer f.Close()

	var (
		cur        strings.Builder
		final      string
		completion string // result of the last attempt_completion call
		pending    bool   // the next tool_call event belongs to attempt_completion
		toolCalls  []string
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
			if pending {
				// The event right after an attempt_completion call is its result
				// echo: take it as the answer and keep it out of the call trail.
				if ev.Meta[metaKeyPhase] == phaseResult {
					if txt := cleanToolResult(ev.Text); txt != "" {
						completion = txt
						final = "" // the completion result is the later segment
					}
				}
				pending = false
				continue
			}
			name := toolName(ev)
			if name == completionTool {
				pending = true
				if r := completionResult(ev); r != "" {
					completion = r
					final = ""
				}
			}
			if name != "" {
				toolCalls = append(toolCalls, name)
			}
		case eventToolCallDiff:
			flush()
		}
	}
	flush()

	if final == "" {
		final = completion
	}
	return truncateResult(final, limit), toolCalls
}

// toolName picks the tool name reported by a tool_call event: the structured
// Meta["name"], the tool_name of the structured tool summary, otherwise the
// renderer's short bracketed marker. Raw event text is never used as a name —
// it can carry a whole command output (see shortLabel).
func toolName(ev streamEvent) string {
	if name := strings.TrimSpace(ev.Meta["name"]); name != "" {
		return name
	}
	if s := parseToolSummary(ev.Meta[metaKeyToolSummary]); s.ToolName != "" {
		return s.ToolName
	}
	return shortLabel(ev.Text)
}

// toolSummary mirrors the subset of agent.ToolSummary carried in
// Meta["tool_summary"] (FEATURE-388) that the parser needs.
type toolSummary struct {
	ToolName string             `json:"tool_name"`
	Params   []toolSummaryParam `json:"params"`
}

// toolSummaryParam is one key parameter of a tool summary.
type toolSummaryParam struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// parseToolSummary decodes Meta["tool_summary"]; malformed JSON yields a zero
// summary so parsing never fails on an unexpected payload.
func parseToolSummary(raw string) toolSummary {
	var s toolSummary
	if strings.TrimSpace(raw) == "" {
		return s
	}
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return toolSummary{}
	}
	return s
}

// paramValue returns the value of the named summary parameter, trimmed.
func (s toolSummary) paramValue(name string) string {
	for _, p := range s.Params {
		if p.Name == name {
			return strings.TrimSpace(p.Value)
		}
	}
	return ""
}

// completionResult returns the result text carried by an attempt_completion
// tool call itself (Meta["tool_summary"] → params.result). The renderer
// currently reports no params for this tool, so the caller falls back to the
// result echo event; the path is kept for payloads that do carry it.
func completionResult(ev streamEvent) string {
	return parseToolSummary(ev.Meta[metaKeyToolSummary]).paramValue("result")
}

// shortLabel extracts a compact label from a tool_call event text: the leading
// bracketed marker ("[🔧 read_file] …") when present, otherwise the first line
// when it is short. Longer text yields "" so tool output cannot leak into the
// trail.
func shortLabel(text string) string {
	s := strings.TrimSpace(text)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "[") {
		if i := strings.IndexByte(s, ']'); i >= 0 && i+1 <= maxToolLabelLen {
			return s[:i+1]
		}
	}
	line := s
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	line = strings.TrimSpace(line)
	if len(line) > maxToolLabelLen {
		return ""
	}
	return line
}

// cleanToolResult strips the renderer decorations around a tool result text: a
// leading bracketed marker ("[返回结果]") and a trailing "[意图] …" paragraph.
// The short status line the renderer keeps in between (e.g. "✅ 任务完成") is
// left in place: it is informational and language-dependent.
func cleanToolResult(text string) string {
	s := strings.TrimSpace(text)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "[") {
		if i := strings.IndexByte(s, ']'); i >= 0 && i+1 <= maxToolLabelLen {
			s = strings.TrimSpace(s[i+1:])
		}
	}
	if i := strings.LastIndex(s, "\n\n["); i >= 0 {
		tail := s[i+2:]
		if j := strings.IndexByte(tail, ']'); j >= 0 && j+1 <= maxToolLabelLen && !strings.Contains(tail, "\n") {
			s = strings.TrimSpace(s[:i])
		}
	}
	return s
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
