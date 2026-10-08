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
// FEATURE-622: these slimmers emit cross-message references, which only stay
// resolvable while every historical envelope is kept in the request. With the
// history trim on (env_history_prune) all older envelopes collapse to <time> +
// <message_no>, so the three sub-switches are bypassed and the newest envelope
// is rendered self-contained (full listings, brief roster, runtime_info).
//
// Author: L.Shuang
// Created: 2026-10-02
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"hash/fnv"
	"os"
	"path/filepath"
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

// envHistoryTrimOn reports whether historical envelopes are trimmed
// (FEATURE-622). While it is on, the newest envelope is the only complete one,
// so the slimmers below must not emit cross-message references at all.
func (a *Agent) envHistoryTrimOn() bool {
	return a.cfg != nil && a.cfg.LLM.EnvHistoryPrune
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

// envToolsMaxEntries caps how many entries each tool directory contributes to
// the <tools> block (the same limit the workspace listing used before).
const envToolsMaxEntries = 64

// toolsGlobalDir returns the absolute path of the global tools directory
// (~/.co-shell/bin), or "" when the home directory cannot be resolved.
func toolsGlobalDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".co-shell", "bin")
}

// buildToolsBlock renders the body of the <tools> block as two sections: the
// global directory first (~/.co-shell/bin), then the workspace one (<cwd>/bin).
// Each section prints its absolute path exactly once on its own line and lists
// every entry below it as "name time size" with no path prefix (FEATURE-623).
// A section whose directory is missing keeps its path line so the reader can
// tell the directory is empty; the global section is dropped entirely when the
// home directory cannot be resolved.
func buildToolsBlock(cwd string) string {
	var sb strings.Builder
	if dir := toolsGlobalDir(); dir != "" {
		sb.WriteString("<global>\n")
		sb.WriteString(dir)
		sb.WriteString("\n")
		sb.WriteString(listFilesWithMeta(dir, envToolsMaxEntries, ""))
		sb.WriteString("</global>\n")
	}
	wsDir := filepath.Join(cwd, "bin")
	sb.WriteString("<workspace>\n")
	sb.WriteString(wsDir)
	sb.WriteString("\n")
	sb.WriteString(listFilesWithMeta(wsDir, envToolsMaxEntries, ""))
	sb.WriteString("</workspace>")
	return sb.String()
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

	if a.cfg == nil || !a.cfg.LLM.EnvDirChangeDetect || !incDir || a.envHistoryTrimOn() {
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
	// FEATURE-622: with the history trim on the newest envelope must stay
	// self-contained, so the roster is rendered every time — still in its brief
	// form (id | name | status), which is what this switch asks for anyway.
	if a.envHistoryTrimOn() {
		return briefBoardRoster(full)
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
	if a.cfg == nil || !a.cfg.LLM.EnvRuntimeInfoOnce || a.envHistoryTrimOn() {
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
