// Package agent - three-level degradation of historical <environment_details>
// blocks (FEATURE-579, plan A).
//
// Every user/tool message carries a frozen <environment_details> envelope, so a
// long session re-sends dozens of full envelopes (each ~5KB with the file/tool
// listings) on every LLM call. pruneEnvContext rewrites the envelopes of the
// *copy* handed to the provider:
//
//	L0 full   → the newest env_history_full_keep messages keep everything
//	L1 lite   → the next env_history_lite_keep messages keep the dynamic parts
//	L2 minimal→ everything older keeps only <time> and <message_no>
//
// 0 means "unlimited": full_keep=0 keeps every message at L0 (degradation off),
// lite_keep=0 turns every non-L0 message into L1 (no L2 level).
//
// The persisted history (a.messages) must never be touched: the slice is cloned
// before any ContentPart is replaced (same pattern as pruneUIContext).
//
// Author: L.Shuang
// Created: 2026-10-02
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"strings"
	"time"

	"github.com/idirect3d/co-shell/llm"
)

// Environment degradation levels.
const (
	envLevelFull    = 0 // L0: unchanged envelope
	envLevelLite    = 1 // L1: dynamic parts only
	envLevelMinimal = 2 // L2: <time> + <message_no> only
)

// envDirForceFullEvery forces a full <current_dir>/<tools>/<research> block
// every N envelopes so the LLM can always recover the file listing (plan C).
const envDirForceFullEvery = 20

// envLiteTags are kept at L1; envMinimalTags are kept at L2. Anything else
// (current_dir, tools, research, board_agents, runtime_info, user_dynamic_events)
// is dropped because it is either static, bulky or one-shot.
var (
	envLiteTags = map[string]bool{
		"time":             true,
		"message_no":       true,
		"context_window":   true,
		"cwd":              true,
		"current_mode":     true,
		"result_mode":      true,
		"no_tool_action":   true,
		"session_title":    true,
		"task_plan":        true,
		"opened_resources": true,
		"sub_agents":       true,
		"board_replies":    true,
	}
	envMinimalTags = map[string]bool{
		"time":       true,
		"message_no": true,
	}
)

// envLevelOf returns the degradation level for a message whose distance from
// the newest message is dist (0 = newest).
func envLevelOf(dist, fullKeep, liteKeep int) int {
	if dist <= 0 {
		return envLevelFull // the last message is never degraded
	}
	if fullKeep == 0 {
		return envLevelFull // 0 = unlimited: everything stays L0
	}
	if dist < fullKeep {
		return envLevelFull
	}
	if liteKeep == 0 {
		return envLevelLite // 0 = unlimited: no L2 level
	}
	if dist < fullKeep+liteKeep {
		return envLevelLite
	}
	return envLevelMinimal
}

// envPartIndex returns the index of the ContentPart carrying the
// <environment_details> block, scanning from the end (images may be appended
// after the envelope), or -1 when the message has no envelope.
func envPartIndex(msg *llm.Message) int {
	if msg == nil {
		return -1
	}
	for i := len(msg.ContentParts) - 1; i >= 0; i-- {
		if strings.Contains(msg.ContentParts[i].Text, "<environment_details>") {
			return i
		}
	}
	return -1
}

// envTextAt returns the envelope text of msg, or "" when absent.
func envTextAt(msg *llm.Message) string {
	i := envPartIndex(msg)
	if i < 0 {
		return ""
	}
	return msg.ContentParts[i].Text
}

// envBlock is one top-level element of an <environment_details> envelope.
type envBlock struct {
	name string
	raw  string
}

// tagNameOf returns the element name of a trimmed "<foo>…" line, or "".
func tagNameOf(line string) string {
	if !strings.HasPrefix(line, "<") || strings.HasPrefix(line, "</") {
		return ""
	}
	end := strings.Index(line, ">")
	if end <= 1 {
		return ""
	}
	return line[1:end]
}

// splitEnvBlocks splits an envelope into its top-level elements, preserving
// order. Nested elements (e.g. <browser> inside <opened_resources>) stay inside
// their parent block.
func splitEnvBlocks(envText string) []envBlock {
	body := envText
	if i := strings.Index(body, "<environment_details>"); i >= 0 {
		body = body[i+len("<environment_details>"):]
	}
	if i := strings.LastIndex(body, "</environment_details>"); i >= 0 {
		body = body[:i]
	}
	lines := strings.Split(body, "\n")
	var blocks []envBlock
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		name := tagNameOf(trimmed)
		if name == "" {
			continue
		}
		raw := lines[i]
		if !strings.Contains(trimmed, "</"+name+">") {
			for j := i + 1; j < len(lines); j++ {
				raw += "\n" + lines[j]
				if strings.Contains(lines[j], "</"+name+">") {
					i = j
					break
				}
			}
		}
		blocks = append(blocks, envBlock{name: name, raw: raw})
	}
	return blocks
}

// hasEnvTag reports whether the envelope contains the given top-level element.
func hasEnvTag(envText, name string) bool {
	for _, b := range splitEnvBlocks(envText) {
		if b.name == name {
			return true
		}
	}
	return false
}

// buildEnvBlock rebuilds an envelope keeping only the elements allowed at the
// given level. When nothing but the time tag is kept the block stays valid.
func buildEnvBlock(envText string, level int) string {
	keep := envMinimalTags
	if level == envLevelLite {
		keep = envLiteTags
	}
	var sb strings.Builder
	sb.WriteString("<environment_details>")
	for _, b := range splitEnvBlocks(envText) {
		if !keep[b.name] {
			continue
		}
		sb.WriteString("\n")
		sb.WriteString(b.raw)
	}
	sb.WriteString("\n</environment_details>")
	return sb.String()
}

// minimalEnvBlock builds a standalone L2 envelope (time + message_no) without
// draining any dynamic queue — used by plan B for the non-final tool results of
// one iteration.
func minimalEnvBlock(now string, messageNo int) string {
	return "<environment_details>\n<time>" + now + "</time>\n<message_no>" + itoa(messageNo) + "</message_no>\n</environment_details>"
}

// itoa is a tiny wrapper to keep env_prune.go free of strconv noise.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// pruneEnvContext degrades the envelopes of historical messages in msgs (the
// per-call copy). It returns msgs unchanged when the switch is off.
func (a *Agent) pruneEnvContext(msgs []llm.Message) []llm.Message {
	if a.cfg == nil || !a.cfg.LLM.EnvHistoryPrune {
		return msgs
	}
	n := len(msgs)
	if n == 0 {
		return msgs
	}
	fullKeep := a.cfg.LLM.EnvHistoryFullKeep
	liteKeep := a.cfg.LLM.EnvHistoryLiteKeep

	levels := make([]int, n)
	for i := 0; i < n; i++ {
		levels[i] = envLevelOf(n-1-i, fullKeep, liteKeep)
	}

	// Fallback: keep <task_plan> visible. When the newest message has no plan
	// (skipTaskPlan for track_task_progress/view_task_plan/attempt_completion),
	// the most recent message that carries one stays at least at L1.
	if !hasEnvTag(envTextAt(&msgs[n-1]), "task_plan") {
		for i := n - 1; i >= 0; i-- {
			if hasEnvTag(envTextAt(&msgs[i]), "task_plan") {
				if levels[i] > envLevelLite {
					levels[i] = envLevelLite
				}
				break
			}
		}
	}

	for i := 0; i < n; i++ {
		if levels[i] == envLevelFull {
			continue
		}
		idx := envPartIndex(&msgs[i])
		if idx < 0 {
			continue
		}
		pruned := buildEnvBlock(msgs[i].ContentParts[idx].Text, levels[i])
		// Clone before writing so the persistent history keeps its envelope
		// (the elements share the ContentParts backing array with a.messages).
		parts := make([]llm.ContentPart, len(msgs[i].ContentParts))
		copy(parts, msgs[i].ContentParts)
		parts[idx].Text = pruned
		msgs[i].ContentParts = parts
	}
	return msgs
}

// injectMinimalEnvToLast appends an L2 envelope (<time> + <message_no>) to the
// last persisted message. Plan B uses it for the non-final tool results of one
// iteration: it never builds the full envelope, so the dynamic-event queue is
// drained exactly once per iteration — by the final tool result, which takes
// the full envelope path.
func (a *Agent) injectMinimalEnvToLast() {
	if len(a.messages) == 0 {
		return
	}
	lastIdx := len(a.messages) - 1
	msg := &a.messages[lastIdx]
	if msg.Role != "user" && msg.Role != "tool" {
		return
	}
	envText := minimalEnvBlock(time.Now().Format("2006-01-02 15:04:05 Monday"), lastIdx)
	if len(msg.ContentParts) == 0 {
		msg.ContentParts = []llm.ContentPart{
			{Type: llm.ContentPartText, Text: msg.Content},
		}
		msg.Content = ""
	}
	msg.AppendTextPart(envText)
}
