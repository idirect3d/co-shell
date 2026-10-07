// Author: L.Shuang
// Created: 2026-10-06
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

package web

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/log"
)

// dynItem is one row of the dynamic-info panel's timeline (FEATURE-612).
type dynItem struct {
	// ID is the dynamic event id ("ev-<n>").
	ID string `json:"id"`
	// Kind is the DynamicEventKind, used by the frontend to pick a colour and
	// a localized type label.
	Kind string `json:"kind"`
	// Text is the one-line body shown next to the type label.
	Text string `json:"text"`
	// Ts is the event time in Unix milliseconds.
	Ts int64 `json:"ts"`
	// Flag is "pending" (queued, never injected) or "unacked" (injected but the
	// LLM has not confirmed it through meta.ack yet), empty otherwise.
	Flag string `json:"flag,omitempty"`
}

// dynBlock is one <environment_details> element shown below the timeline.
type dynBlock struct {
	// Tag is the element name without angle brackets.
	Tag string `json:"tag"`
	// Text is the raw element text, tags included.
	Text string `json:"text"`
}

// dynCounts summarises the timeline rows that still need attention.
type dynCounts struct {
	Pending int `json:"pending"`
	Unacked int `json:"unacked"`
}

// dynInfo is the whole snapshot pushed to the browser as kind=dyninfo.
type dynInfo struct {
	Items  []dynItem  `json:"items"`
	Env    []dynBlock `json:"env"`
	Counts dynCounts  `json:"counts"`
}

// dynEnvSkip lists the <environment_details> elements the dynamic-info panel
// hides: current_dir and research are far too large to be of any use in a side
// panel, and task_plan is exactly what this panel's other view already shows.
var dynEnvSkip = map[string]bool{
	"current_dir": true,
	"research":    true,
	"task_plan":   true,
}

// dynTagRe matches the tags of an already rendered payload (board notices and
// sub-agent notices carry ready-made tags) so a timeline row stays readable.
var dynTagRe = regexp.MustCompile(`</?[^>]+>`)

// dynAttrValue returns the first non-empty attribute among names, e.g. the
// "from" of a <board_message> tag — the agent name the panel displays.
func dynAttrValue(tag string, names ...string) string {
	for _, name := range names {
		marker := name + `="`
		i := strings.Index(tag, marker)
		if i < 0 {
			continue
		}
		rest := tag[i+len(marker):]
		if j := strings.Index(rest, `"`); j > 0 {
			return rest[:j]
		}
	}
	return ""
}

// dynEntryText renders the body of one timeline row.
func dynEntryText(ev agent.DynEventView) string {
	if ev.Path != "" {
		return ev.Path
	}
	switch ev.Kind {
	case string(agent.DynamicBoardMessage),
		string(agent.DynamicBoardTimeout),
		string(agent.DynamicSubAgentDone):
		// The agent NAME lives in an attribute (from / agent_id / id), so it is
		// pulled out before the tags are stripped and kept as a prefix — losing it
		// was exactly the "missing agent name" the panel showed.
		from := dynAttrValue(ev.Text, "from", "agent_id", "id")
		body := strings.TrimSpace(dynTagRe.ReplaceAllString(ev.Text, " "))
		if from != "" {
			return from + " · " + body
		}
		return body
	}
	return ev.Text
}

// buildDynInfo assembles the dynamic-perception snapshot from the agent's
// event queue and the last <environment_details> it sent to the LLM.
func (s *WebSession) buildDynInfo() dynInfo {
	info := dynInfo{Items: []dynItem{}, Env: []dynBlock{}}
	if s.ag == nil {
		return info
	}
	for _, ev := range s.ag.DynEventsSnapshot() {
		it := dynItem{ID: ev.ID, Kind: ev.Kind, Text: dynEntryText(ev), Ts: ev.Ts}
		if ev.Pending {
			it.Flag = "pending"
			info.Counts.Pending++
		} else if ev.Unacked {
			it.Flag = "unacked"
			info.Counts.Unacked++
		}
		info.Items = append(info.Items, it)
	}
	for _, b := range applyBoardRoster(s.ag.EnvBlocks(), s.ag.BoardRosterBlock()) {
		if dynEnvSkip[b.Tag] {
			continue
		}
		info.Env = append(info.Env, dynBlock{Tag: b.Tag, Text: b.Text})
	}
	return info
}

// applyBoardRoster swaps the compact <board_agents …><unchanged/></board_agents>
// form for the current roster, so the panel can always list who is on the board
// while the LLM keeps receiving the compact form. An empty roster leaves every
// block untouched (FEATURE-612).
func applyBoardRoster(blocks []agent.EnvBlock, roster string) []agent.EnvBlock {
	if roster == "" {
		return blocks
	}
	for i, b := range blocks {
		if b.Tag == "board_agents" && strings.Contains(b.Text, "<unchanged") {
			blocks[i].Text = roster
		}
	}
	return blocks
}

// pushDynInfo broadcasts the current dynamic-perception snapshot to every
// connected client (FEATURE-612). Best effort: a marshal failure is logged and
// nothing else happens.
func (s *WebSession) pushDynInfo() {
	if s == nil || s.srv == nil {
		return
	}
	raw := s.dynInfoJSON()
	if raw == "" {
		return
	}
	s.srv.sendJSON(serverMessage{Kind: "dyninfo", Dyn: json.RawMessage(raw)})
}

// dynInfoJSON renders the dynamic-perception snapshot as a JSON string, i.e.
// the payload of a kind=dyninfo frame (FEATURE-612). It returns "" when the
// snapshot cannot be marshalled.
func (s *WebSession) dynInfoJSON() string {
	if s == nil || s.ag == nil {
		return ""
	}
	raw, err := json.Marshal(s.buildDynInfo())
	if err != nil {
		log.Warn("session: marshal dyninfo: %v", err)
		return ""
	}
	return string(raw)
}
