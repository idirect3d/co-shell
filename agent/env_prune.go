// Package agent - trimming of historical <environment_details> envelopes
// (FEATURE-622, superseding the three-level degradation of FEATURE-579 plan A).
//
// Every user/tool message carries a frozen <environment_details> envelope, so a
// long session re-sends dozens of full envelopes (each ~5KB with the file/tool
// listings) on every LLM call. With env_history_prune on, pruneEnvContext
// rewrites the envelopes of the *copy* handed to the provider:
//
//	newest carrier → kept exactly as frozen (the only complete envelope)
//	everything else → <time> + <message_no> only
//
// The newest carrier is the last user/tool message that still carries an
// envelope (not necessarily the last element of the slice). Because the older
// envelopes lose their listings, the generator must never put cross-message
// references (<unchanged since msg#N/> or <unchanged/>) into that newest
// envelope — see env_slim.go, which bypasses the three slim-down sub-switches
// while this switch is on. With env_history_prune off nothing is rewritten and
// the sub-switches apply as before.
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

// envDirForceFullEvery forces a full <current_dir>/<tools>/<research> block
// every N envelopes so the LLM can always recover the file listing (plan C).
// It only matters while env_history_prune is off: with the trim on, the newest
// envelope always carries the full listings (FEATURE-622).
const envDirForceFullEvery = 20

// envMinimalTags are the only elements kept in a trimmed envelope. Everything
// else (cwd, context_window, current_mode, current_dir, tools, research,
// board_agents, runtime_info, task_plan, opened_resources, user_dynamic_events)
// is dropped because it is bulky, static or one-shot — the newest envelope
// always carries the current values (FEATURE-622).
var envMinimalTags = map[string]bool{
	"time":       true,
	"message_no": true,
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

// tagNameOf returns the element name of a trimmed "<foo>…" line, or "". Any
// attribute is dropped, so `<board_agents rev="7">` yields "board_agents".
func tagNameOf(line string) string {
	if !strings.HasPrefix(line, "<") || strings.HasPrefix(line, "</") {
		return ""
	}
	end := strings.Index(line, ">")
	if end <= 1 {
		return ""
	}
	name := line[1:end]
	if i := strings.IndexAny(name, " \t"); i >= 0 {
		name = name[:i]
	}
	return name
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

// buildMinimalEnvBlock rebuilds an envelope keeping only <time> and
// <message_no>, so a long history stays cheap (FEATURE-622).
func buildMinimalEnvBlock(envText string) string {
	var sb strings.Builder
	sb.WriteString("<environment_details>")
	for _, b := range splitEnvBlocks(envText) {
		if !envMinimalTags[b.name] {
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

// pruneEnvContext trims the envelopes of historical messages in msgs (the
// per-call copy): only the newest user/tool message carrying an envelope keeps
// its full envelope, every older one collapses to <time> + <message_no>. It
// returns msgs unchanged when the switch is off (FEATURE-622).
func (a *Agent) pruneEnvContext(msgs []llm.Message) []llm.Message {
	if a.cfg == nil || !a.cfg.LLM.EnvHistoryPrune {
		return msgs
	}
	n := len(msgs)
	if n == 0 {
		return msgs
	}

	// Find the newest envelope carrier (the last user/tool message with an
	// envelope): it is the single full envelope of the request.
	keep := -1
	for i := n - 1; i >= 0; i-- {
		if msgs[i].Role != "user" && msgs[i].Role != "tool" {
			continue
		}
		if envPartIndex(&msgs[i]) >= 0 {
			keep = i
			break
		}
	}
	if keep < 0 {
		return msgs
	}

	for i := 0; i < n; i++ {
		if i == keep {
			continue
		}
		idx := envPartIndex(&msgs[i])
		if idx < 0 {
			continue
		}
		trimmed := buildMinimalEnvBlock(msgs[i].ContentParts[idx].Text)
		// Clone before writing so the persistent history keeps its envelope
		// (the elements share the ContentParts backing array with a.messages).
		parts := make([]llm.ContentPart, len(msgs[i].ContentParts))
		copy(parts, msgs[i].ContentParts)
		parts[idx].Text = trimmed
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
