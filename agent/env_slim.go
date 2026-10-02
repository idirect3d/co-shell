// Package agent - plan C of FEATURE-579: slim down the parts of
// <environment_details> that are static or one-shot.
//
//	board_agents      → id | name | status (brief), <unchanged/> while rev is stable
//	current_dir/tools/research → change detection with <unchanged since msg#N/>
//	runtime_info      → only when it changed (session first envelope included)
//
// All three sub-switches default to on; when a switch is off the original full
// rendering is produced exactly as before.
//
// Author: L.Shuang
// Created: 2026-10-02
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"hash/fnv"
	"strconv"
	"strings"
)

// envAgentState is the session-level state of the plan C slim-down. It lives on
// the Agent because it must survive across LLM iterations within one session.
type envAgentState struct {
	// board roster (C-1)
	rosterRev  string
	rosterSent bool

	// current_dir/tools/research change detection (C-2)
	dirHash     uint64
	dirMsgNo    int
	dirMisses   int // envelopes emitted without a fresh full listing
	dirSentFull bool

	// runtime_info (C-3)
	runtimeInfo     string
	runtimeInfoSent bool
}

// fnvHash returns a stable hash of the given strings.
func fnvHash(parts ...string) uint64 {
	h := fnv.New64a()
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}
	return h.Sum64()
}

// envStaticBlocks renders the <current_dir>/<tools>/<research> blocks for the
// envelope of message msgNo. With env_dir_change_detect on, unchanged listings
// collapse to a <unchanged since msg#N/> placeholder; a full listing is emitted
// every envDirForceFullEvery envelopes anyway so the LLM can always recover it.
func (a *Agent) envStaticBlocks(msgNo int, files, binFiles, researchFiles string, incDir, incTools, incResearch bool) string {
	full := func() string {
		var sb strings.Builder
		if incDir {
			sb.WriteString("<current_dir>\n")
			sb.WriteString(files)
			sb.WriteString("\n</current_dir>\n")
		}
		if incTools && binFiles != "" {
			sb.WriteString("<tools>\n")
			sb.WriteString(binFiles)
			sb.WriteString("\n</tools>\n")
		}
		if incResearch && researchFiles != "" {
			sb.WriteString("<research>\n")
			sb.WriteString(researchFiles)
			sb.WriteString("\n</research>\n")
		}
		return sb.String()
	}

	if a.cfg == nil || !a.cfg.LLM.EnvDirChangeDetect || !incDir {
		return full()
	}

	hash := fnvHash(files, binFiles, researchFiles)
	st := &a.envState
	stale := !st.dirSentFull || hash != st.dirHash || st.dirMisses >= envDirForceFullEvery
	if stale {
		st.dirHash = hash
		st.dirMsgNo = msgNo
		st.dirMisses = 0
		st.dirSentFull = true
		return full()
	}
	st.dirMisses++

	placeholder := "<unchanged since msg#" + strconv.Itoa(st.dirMsgNo) + "/>"
	var sb strings.Builder
	if incDir {
		sb.WriteString("<current_dir>\n")
		sb.WriteString(placeholder)
		sb.WriteString("\n</current_dir>\n")
	}
	if incTools && binFiles != "" {
		sb.WriteString("<tools>\n")
		sb.WriteString(placeholder)
		sb.WriteString("\n</tools>\n")
	}
	if incResearch && researchFiles != "" {
		sb.WriteString("<research>\n")
		sb.WriteString(placeholder)
		sb.WriteString("\n</research>\n")
	}
	return sb.String()
}

// boardRosterRev extracts the rev attribute of a <board_agents> block.
func boardRosterRev(block string) string {
	const marker = `<board_agents rev="`
	i := strings.Index(block, marker)
	if i < 0 {
		return ""
	}
	rest := block[i+len(marker):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// briefBoardRoster strips the responsibility text from every <agent> element,
// keeping only id/name/status.
func briefBoardRoster(block string) string {
	var sb strings.Builder
	rest := block
	for {
		open := strings.Index(rest, "<agent ")
		if open < 0 {
			sb.WriteString(rest)
			break
		}
		sb.WriteString(rest[:open])
		tail := rest[open:]
		gt := strings.Index(tail, ">")
		if gt < 0 {
			sb.WriteString(tail)
			break
		}
		head := tail[:gt+1]
		// Empty the element body: <agent …></agent>
		sb.WriteString(head)
		rest = tail[gt+1:]
		if close := strings.Index(rest, "</agent>"); close >= 0 {
			rest = rest[close:]
		}
	}
	return sb.String()
}

// boardRosterBlockSlim renders <board_agents> for the current envelope: the
// brief form (id | name | status) with env_board_roster_brief on, collapsing to
// <unchanged/> while the roster revision does not change.
func (a *Agent) boardRosterBlockSlim() string {
	full := a.BoardRosterBlock()
	if full == "" {
		return ""
	}
	if a.cfg == nil || !a.cfg.LLM.EnvBoardRosterBrief {
		return full
	}
	rev := boardRosterRev(full)
	st := &a.envState
	if st.rosterSent && rev != "" && rev == st.rosterRev {
		return `<board_agents rev="` + rev + `">` + "\n<unchanged/>\n</board_agents>"
	}
	st.rosterRev = rev
	st.rosterSent = true
	return briefBoardRoster(full)
}

// runtimeInfoBlockSlim renders <runtime_info> only when the runtime values
// changed since the previous envelope (env_runtime_info_once).
func (a *Agent) runtimeInfoBlockSlim() string {
	ri := a.buildRuntimeInfo()
	if ri == "" {
		return ""
	}
	if a.cfg == nil || !a.cfg.LLM.EnvRuntimeInfoOnce {
		return ri
	}
	st := &a.envState
	if st.runtimeInfoSent && ri == st.runtimeInfo {
		return ""
	}
	st.runtimeInfo = ri
	st.runtimeInfoSent = true
	return ri
}
