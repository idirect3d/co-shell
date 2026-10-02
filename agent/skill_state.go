// Author: L.Shuang
// Created: 2026-10-02
// Last Modified: 2026-10-02
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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/llm"
)

// LoadedSkill records one skill whose full SKILL.md content is currently
// pinned in the conversation (FEATURE-583).
//
// The loaded set (Agent.loadedSkills) is the authoritative index of what the
// model has in context; the pinned messages carry the content itself. Both are
// kept in sync by the skill tools and verified on session restore.
type LoadedSkill struct {
	// Name is the skill name from the SKILLS index (unique).
	Name string `json:"name"`
	// Path is the absolute path of the SKILL.md file.
	Path string `json:"path"`
	// Dir is the absolute path of the skill directory (for listing resources).
	Dir string `json:"dir"`
	// Bytes is the byte length of the SKILL.md snapshot at load time.
	Bytes int `json:"bytes"`
	// SHA256 is the fingerprint of the SKILL.md snapshot; it is compared with
	// the file on disk to warn the user that the skill changed after loading.
	SHA256 string `json:"sha256"`
	// Source is "workspace" or "global".
	Source string `json:"source"`
	// Description is the short summary parsed from the SKILL.md front matter.
	// It is shown to the user (loaded-skills tooltip in the Web UI and the note
	// line of the skill_load result); it is never part of the pinned content
	// (FIX-587).
	Description string `json:"description,omitempty"`
	// LoadedAt is when the content was pinned.
	LoadedAt time.Time `json:"loaded_at"`
}

// Skill source values (LoadedSkill.Source).
const (
	SkillSourceWorkspace = "workspace"
	SkillSourceGlobal    = "global"
)

// Default warn thresholds (bytes) for skill content size (FEATURE-583, D-E).
// Loading is never refused — exceeding a threshold only adds a warning.
const (
	defaultSkillWarnFileBytes  = 32 * 1024
	defaultSkillWarnTotalBytes = 64 * 1024
)

// LoadedSkills returns a copy of the loaded skill list, sorted by name.
func (a *Agent) LoadedSkills() []LoadedSkill {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.loadedSkillsLocked()
}

func (a *Agent) loadedSkillsLocked() []LoadedSkill {
	if len(a.loadedSkills) == 0 {
		return nil
	}
	out := make([]LoadedSkill, 0, len(a.loadedSkills))
	for _, ls := range a.loadedSkills {
		out = append(out, ls)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// LoadedSkillNames returns the loaded skill names in stable (sorted) order.
func (a *Agent) LoadedSkillNames() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.loadedSkills) == 0 {
		return nil
	}
	names := make([]string, 0, len(a.loadedSkills))
	for name := range a.loadedSkills {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetLoadedSkill returns the loaded skill record for name.
func (a *Agent) GetLoadedSkill(name string) (LoadedSkill, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	ls, ok := a.loadedSkills[name]
	return ls, ok
}

// SkillLoaded reports whether name is currently loaded.
func (a *Agent) SkillLoaded(name string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, ok := a.loadedSkills[name]
	return ok
}

// SetLoadedSkill records (or refreshes) a loaded skill.
func (a *Agent) SetLoadedSkill(ls LoadedSkill) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.loadedSkills == nil {
		a.loadedSkills = make(map[string]LoadedSkill)
	}
	a.loadedSkills[ls.Name] = ls
}

// RemoveLoadedSkill drops name from the loaded set and reports whether it was
// present.
func (a *Agent) RemoveLoadedSkill(name string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.loadedSkills[name]; !ok {
		return false
	}
	delete(a.loadedSkills, name)
	return true
}

// ClearLoadedSkills drops every loaded skill record (does not touch the
// pinned messages).
func (a *Agent) ClearLoadedSkills() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.loadedSkills = nil
}

// LoadedSkillBytes returns the total byte size of all loaded skills.
func (a *Agent) LoadedSkillBytes() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	total := 0
	for _, ls := range a.loadedSkills {
		total += ls.Bytes
	}
	return total
}

// SetLoadedSkillNames restores the loaded-skill index from a persisted session
// (FEATURE-583, D-C). Only names are persisted; the records are rebuilt from
// the pinned messages on restore (see rebuildLoadedSkillsFromMessages).
func (a *Agent) SetLoadedSkillNames(names []string) {
	if len(names) == 0 {
		a.ClearLoadedSkills()
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.loadedSkills == nil {
		a.loadedSkills = make(map[string]LoadedSkill)
	}
	for _, name := range names {
		if name == "" {
			continue
		}
		if _, ok := a.loadedSkills[name]; !ok {
			a.loadedSkills[name] = LoadedSkill{Name: name}
		}
	}
}

// rebuildLoadedSkillsFromMessages rebuilds the loaded-skill index from the
// pinned messages of the restored history (FEATURE-583, D-C). The messages are
// the source of truth for the content; the persisted name list only tells which
// skills are expected to be present.
//
// Invariant: every loaded name has a pinned message carrying its content. A
// name whose pinned messages are missing is dropped from the index (the caller
// may warn about it).
func (a *Agent) rebuildLoadedSkillsFromMessages(msgs []llm.Message) []string {
	present := make(map[string]time.Time)
	for _, m := range msgs {
		if m.Pinned && m.PinnedFor != "" {
			present[m.PinnedFor] = time.Now()
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	var missing []string
	for name := range a.loadedSkills {
		if _, ok := present[name]; !ok {
			delete(a.loadedSkills, name)
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}

// SkillEnabled reports whether a skill is enabled by configuration
// (FEATURE-583, D-D). A skill explicitly listed in skills_enabled wins;
// otherwise skills_default_enabled applies (default true).
func (a *Agent) SkillEnabled(name string) bool {
	if a.cfg == nil {
		return true
	}
	return a.cfg.SkillEnabled(name)
}

// skillWarnThresholds returns the configured (file, total) warning thresholds
// in bytes (FEATURE-583, D-E).
func (a *Agent) skillWarnThresholds() (int, int) {
	file, total := defaultSkillWarnFileBytes, defaultSkillWarnTotalBytes
	if a.cfg != nil {
		if a.cfg.LLM.SkillsWarnFileBytes > 0 {
			file = a.cfg.LLM.SkillsWarnFileBytes
		}
		if a.cfg.LLM.SkillsWarnTotalBytes > 0 {
			total = a.cfg.LLM.SkillsWarnTotalBytes
		}
	}
	return file, total
}

// skillWarn reports a warning text when the skill's own size or the accumulated
// loaded size exceeds the configured thresholds. Loading is never refused.
func (a *Agent) skillWarn(bytes, totalBytes int) string {
	fileLimit, totalLimit := a.skillWarnThresholds()
	switch {
	case bytes > fileLimit && totalBytes > totalLimit:
		return "size-warn"
	case bytes > fileLimit:
		return "file-warn"
	case totalBytes > totalLimit:
		return "total-warn"
	}
	return ""
}

// --- pinned-message helpers ---

// isPinnedMessage reports whether the message must survive history rewrites.
func isPinnedMessage(m llm.Message) bool {
	return m.Pinned || m.PinnedFor != ""
}

// pinnedMessageIndexes returns the indexes of messages that must survive every
// history-rewriting operation (FEATURE-583, R3).
//
// The set is the pinned closure: every pinned message plus every tool result
// that answers a tool_call of a pinned assistant message. The closure is needed
// because an assistant message with tool_calls is rejected by the API unless
// all of its calls are answered by tool messages — so a pinned skill_load pair
// stays valid even when the model batched it with other tools.
func pinnedMessageIndexes(msgs []llm.Message) map[int]bool {
	keep := make(map[int]bool)
	callIDs := make(map[string]bool)
	for i, m := range msgs {
		if !isPinnedMessage(m) {
			continue
		}
		keep[i] = true
		for _, tc := range m.ToolCalls {
			if tc.ID != "" {
				callIDs[tc.ID] = true
			}
		}
	}
	if len(callIDs) > 0 {
		for i, m := range msgs {
			if m.Role == "tool" && m.ToolCallID != "" && callIDs[m.ToolCallID] {
				keep[i] = true
			}
		}
	}
	return keep
}

// pinnedMessages returns the pinned closure in its original relative order.
func pinnedMessages(msgs []llm.Message) []llm.Message {
	idx := pinnedMessageIndexes(msgs)
	if len(idx) == 0 {
		return nil
	}
	out := make([]llm.Message, 0, len(idx))
	for i, m := range msgs {
		if idx[i] {
			out = append(out, m)
		}
	}
	return out
}

// hasPinnedMessages reports whether any message in the slice is pinned or is
// part of the pinned closure.
func hasPinnedMessages(msgs []llm.Message) bool {
	return len(pinnedMessageIndexes(msgs)) > 0
}

// --- file helpers ---

// skillHomeDir returns the user's home directory (same lookup the SKILLS index
// uses when scanning the global skills directory).
func skillHomeDir() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return home
	}
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	return os.Getenv("USERPROFILE")
}

// skillScanDirs returns the (workspace, home) pair used to discover skills,
// matching the source used by the SKILLS prompt section.
func (a *Agent) skillScanDirs() (string, string) {
	ws := a.workspacePath
	if ws == "" {
		if cwd, err := os.Getwd(); err == nil {
			ws = cwd
		}
	}
	return ws, skillHomeDir()
}

// skillSourceOf reports whether the skill directory belongs to the workspace or
// the global skills directory.
func skillSourceOf(workspacePath, skillDir string) string {
	if workspacePath != "" {
		wsSkills := filepath.Join(workspacePath, "skills")
		if rel, err := filepath.Rel(wsSkills, skillDir); err == nil && !strings.HasPrefix(rel, "..") {
			return SkillSourceWorkspace
		}
	}
	return SkillSourceGlobal
}

// skillSHA256 returns the hex sha256 fingerprint of content.
func skillSHA256(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// listSkillResources lists the files bundled inside a skill directory (all
// files except SKILL.md), relative to the directory, sorted. Used to tell the
// model which helper scripts/assets accompany the skill.
func listSkillResources(dir string) []string {
	if dir == "" {
		return nil
	}
	var out []string
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil || rel == "SKILL.md" {
			return nil
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(out)
	return out
}

// --- perception helpers (FEATURE-583, R2) ---

// SkillStatePayload is the JSON snapshot carried by an EventSkillState event.
// Consumers (Web UI, REPL renderer) redraw the whole loaded-skill area from it,
// so one event always describes the complete current state.
type SkillStatePayload struct {
	Type       string        `json:"type"`
	Action     string        `json:"action"`
	Name       string        `json:"name,omitempty"`
	Loaded     []LoadedSkill `json:"loaded"`
	TotalBytes int           `json:"total_bytes"`
	Warn       string        `json:"warn,omitempty"`
}

// inLLMToolRound reports whether the current tool callback runs inside an LLM
// tool-call round (set by executeToolCall).
func (a *Agent) inLLMToolRound() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.llmToolRound
}

// setLLMToolRound marks/unmarks the current tool callback as part of an LLM
// tool-call round.
func (a *Agent) setLLMToolRound(v bool) {
	a.mu.Lock()
	a.llmToolRound = v
	a.mu.Unlock()
}

// isXMLToolCallMode reports whether tool calls are exchanged as XML text
// instead of native tool_call/tool messages.
func (a *Agent) isXMLToolCallMode() bool {
	if a.toolCallModeMgr == nil {
		return false
	}
	if mode := a.toolCallModeMgr.Current(); mode != nil {
		return !mode.SendTools
	}
	return false
}

// liveStreamCallback returns the stream callback RunStream installed for the
// current turn (nil outside a turn, e.g. for REPL commands and unit tests).
func (a *Agent) liveStreamCallback() StreamCallback {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.uiEmit
}

// skillStatePayload serializes the current loaded-skill snapshot.
func (a *Agent) skillStatePayload(action, name string) string {
	loaded := a.LoadedSkills()
	total := 0
	for _, ls := range loaded {
		total += ls.Bytes
	}
	payload := SkillStatePayload{
		Type:       EventSkillState,
		Action:     action,
		Name:       name,
		Loaded:     loaded,
		TotalBytes: total,
	}
	if ls, ok := a.GetLoadedSkill(name); ok {
		payload.Warn = a.skillWarn(ls.Bytes, total)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}

// emitSkillState publishes a skill_state event through the stream callback of
// the current turn (best effort: commands executed outside a turn have none,
// their textual result already reports the change).
func (a *Agent) emitSkillState(action, name string) {
	cb := a.liveStreamCallback()
	if cb == nil {
		return
	}
	cb(SkillStateEvent(action, name, a.skillStatePayload(action, name)))
}

// --- truncation exemption (FEATURE-583, R3 exemption point 4) ---

// truncateHistoryLocked drops the messages from idx onwards while keeping every
// pinned message that the cut would have removed. The survivors are re-inserted
// right after the system message, so a skill loaded in the current round is not
// lost when that round is rolled back (parse failure, retry-from, interrupt).
//
// The caller must hold a.mu.
func (a *Agent) truncateHistoryLocked(idx int) {
	a.messages = truncateHistoryKeepPinned(a.messages, idx)
}

// truncateHistoryKeepPinned returns messages[:idx] with every pinned message of
// the removed tail re-inserted right after the system message (FEATURE-583, R3
// exemption point 4). It is pure, so it works both inside and outside a locked
// section.
func truncateHistoryKeepPinned(messages []llm.Message, idx int) []llm.Message {
	if idx < 0 || idx > len(messages) {
		return messages
	}
	removed := pinnedMessages(messages[idx:])
	head := messages[:idx]
	if len(removed) == 0 {
		return head
	}
	merged := make([]llm.Message, 0, len(head)+len(removed))
	if len(head) > 0 {
		merged = append(merged, head[0])
		merged = append(merged, removed...)
		merged = append(merged, head[1:]...)
	} else {
		merged = append(merged, removed...)
	}
	return merged
}

// truncateHistory is the locking wrapper around truncateHistoryLocked.
func (a *Agent) truncateHistory(idx int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.truncateHistoryLocked(idx)
}

// reconcileLoadedSkills drops loaded skills that no longer have a pinned
// message in the history, keeping the invariant "loaded implies content is
// present" true after an arbitrary history rewrite. It returns the dropped
// names.
func (a *Agent) reconcileLoadedSkills() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.loadedSkills) == 0 {
		return nil
	}
	present := make(map[string]bool)
	for _, m := range a.messages {
		if m.Pinned && m.PinnedFor != "" {
			present[m.PinnedFor] = true
		}
	}
	var dropped []string
	for name := range a.loadedSkills {
		if !present[name] {
			delete(a.loadedSkills, name)
			dropped = append(dropped, name)
		}
	}
	sort.Strings(dropped)
	return dropped
}

// --- collapse helper (FEATURE-583, R3 exemption points 2 and 3) ---

// collapseKeepingPinned rebuilds a history as
// [system] + pinned closure + optional tail messages. It is used wherever the
// history is collapsed to a single point (reorganize_context, window trimming):
// the pinned skill content is never summarized away or trimmed off.
func collapseKeepingPinned(systemMsg llm.Message, all []llm.Message, tail ...llm.Message) []llm.Message {
	pinned := pinnedMessages(all)
	out := make([]llm.Message, 0, 1+len(pinned)+len(tail))
	out = append(out, systemMsg)
	out = append(out, pinned...)
	for _, m := range tail {
		if m.Role == "" || isPinnedMessage(m) {
			continue
		}
		out = append(out, m)
	}
	return out
}

// loadedSkillSet returns the loaded skill names as a set, for the prompt index.
func (a *Agent) loadedSkillSet() map[string]bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.loadedSkills) == 0 {
		return nil
	}
	out := make(map[string]bool, len(a.loadedSkills))
	for name := range a.loadedSkills {
		out[name] = true
	}
	return out
}

// --- persistence (FEATURE-583, D-C) ---

// rebuildLoadedStateLocked rebuilds the loaded-skill index from the pinned
// messages of the current history. Pinned messages are the source of truth for
// what the model still has in context, so the index is derived from them: a
// restored or replaced history can neither claim content it does not carry nor
// lose content it does.
//
// It returns the names that could not be rebuilt (skill deleted or disabled);
// the pins of those messages are released.
//
// The caller must hold a.mu.
func (a *Agent) rebuildLoadedStateLocked() []string {
	names := make(map[string]bool)
	for _, m := range a.messages {
		if m.Pinned && m.PinnedFor != "" {
			names[m.PinnedFor] = true
		}
	}
	a.loadedSkills = nil
	if len(names) == 0 {
		return nil
	}
	ws, home := a.skillScanDirs()
	byName := make(map[string]Skill)
	for _, s := range ScanSkills(ws, home) {
		byName[s.Name] = s
	}
	now := time.Now()
	var dropped []string
	for name := range names {
		s, ok := byName[name]
		if !ok || !a.cfg.SkillEnabled(name) {
			dropped = append(dropped, name)
			continue
		}
		data, err := os.ReadFile(s.Path)
		if err != nil {
			dropped = append(dropped, name)
			continue
		}
		if a.loadedSkills == nil {
			a.loadedSkills = make(map[string]LoadedSkill)
		}
		a.loadedSkills[name] = LoadedSkill{
			Name:        name,
			Path:        s.Path,
			Dir:         s.Dir,
			Bytes:       len(data),
			SHA256:      skillSHA256(data),
			Source:      skillSourceOf(ws, s.Dir),
			Description: s.Description,
			LoadedAt:    now,
		}
	}
	if len(dropped) == 0 {
		return nil
	}
	droppedSet := make(map[string]bool, len(dropped))
	for _, n := range dropped {
		droppedSet[n] = true
	}
	for i := range a.messages {
		if droppedSet[a.messages[i].PinnedFor] {
			a.messages[i].Pinned = false
			a.messages[i].PinnedFor = ""
		}
	}
	sort.Strings(dropped)
	return dropped
}

// --- settings integration (FEATURE-583, D-D) ---

// SkillInfo describes one skill for the Skills settings page and for any other
// consumer that needs the full picture (index metadata + enable state +
// whether the content is currently pinned).
type SkillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
	Source      string `json:"source"`
	// Enabled is the effective state (explicit entry wins over the default).
	Enabled bool `json:"enabled"`
	// Explicit is true when the user configured this skill individually
	// (i.e. skills_enabled holds an entry for it).
	Explicit bool `json:"explicit"`
	// Loaded is true while the skill content is pinned in the conversation.
	Loaded bool `json:"loaded"`
	// Bytes is the SKILL.md size on disk.
	Bytes int `json:"bytes"`
	// Changed is true when the pinned snapshot no longer matches the file on
	// disk (sha256 mismatch) — :skill reload refreshes it.
	Changed bool `json:"changed"`
	// SHA256 is the disk fingerprint ("" when it could not be read).
	SHA256 string `json:"sha256,omitempty"`
}

// SkillInfos returns every discovered skill with its effective enable state and
// the current loaded state, sorted by name.
func (a *Agent) SkillInfos() []SkillInfo {
	ws, home := a.skillScanDirs()
	skills := ScanSkills(ws, home)
	loaded, _ := a.GetLoadedSkill("") // placeholder to keep the lock scope small
	_ = loaded
	out := make([]SkillInfo, 0, len(skills))
	for _, s := range skills {
		info := SkillInfo{
			Name:        s.Name,
			Description: s.Description,
			Path:        s.Path,
			Source:      skillSourceOf(ws, s.Dir),
			Enabled:     a.SkillEnabled(s.Name),
		}
		if a.cfg != nil {
			_, info.Explicit = a.cfg.LLM.SkillsEnabled[s.Name]
		}
		if data, err := os.ReadFile(s.Path); err == nil {
			info.Bytes = len(data)
			info.SHA256 = skillSHA256(data)
		}
		if ls, ok := a.GetLoadedSkill(s.Name); ok {
			info.Loaded = true
			info.Changed = ls.SHA256 != "" && info.SHA256 != "" && ls.SHA256 != info.SHA256
		}
		out = append(out, info)
	}
	return out
}

// ApplySkillsConfigState re-applies the skills enable/disable configuration
// after it changed: skills that are now disabled are unloaded (their pins
// released), the system prompt is rebuilt and a skill_state config event is
// emitted. It returns the names that were unloaded.
func (a *Agent) ApplySkillsConfigState() []string {
	var unloaded []string
	for _, ls := range a.LoadedSkills() {
		if a.SkillEnabled(ls.Name) {
			continue
		}
		a.SkillUnload(ls.Name)
		unloaded = append(unloaded, ls.Name)
	}
	a.rebuildSystemPrompt()
	a.emitSkillState(SkillActionConfig, "")
	return unloaded
}
