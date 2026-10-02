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
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
)

// Markers wrapping the byte-exact SKILL.md text inside a skill_load tool
// result. The text between them is always the file content verbatim.
const (
	skillContentBeginMarker = "----- SKILL.md BEGIN -----"
	skillContentEndMarker   = "----- SKILL.md END -----"
)

// toolSkillLoad / toolSkillUnload are the tool names (FEATURE-583, D-A).
const (
	toolSkillLoad   = "skill_load"
	toolSkillUnload = "skill_unload"
)

// SkillLoadContent validates a skill name, checks it against the enable
// configuration, reads SKILL.md from disk and returns its metadata plus the
// raw content. It does NOT change the loaded state — callers decide how the
// content is pinned (LLM round vs. user command).
func (a *Agent) SkillLoadContent(name string) (LoadedSkill, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return LoadedSkill{}, "", errors.New(i18n.T(i18n.KeySkillErrNameRequired))
	}
	ws, home := a.skillScanDirs()
	skills := ScanSkills(ws, home)
	var found *Skill
	for i := range skills {
		if skills[i].Name == name {
			found = &skills[i]
			break
		}
	}
	if found == nil {
		return LoadedSkill{}, "", fmt.Errorf(i18n.T(i18n.KeySkillErrNotFound), name)
	}
	if !a.SkillEnabled(name) {
		return LoadedSkill{}, "", fmt.Errorf(i18n.T(i18n.KeySkillErrDisabled), name)
	}
	data, err := os.ReadFile(found.Path)
	if err != nil {
		return LoadedSkill{}, "", fmt.Errorf(i18n.T(i18n.KeySkillErrRead), found.Path, err)
	}
	ls := LoadedSkill{
		Name:     found.Name,
		Path:     found.Path,
		Dir:      found.Dir,
		Bytes:    len(data),
		SHA256:   skillSHA256(data),
		Source:   skillSourceOf(ws, found.Dir),
		LoadedAt: time.Now(),
	}
	return ls, string(data), nil
}

// skillLoadTool implements the skill_load tool (FEATURE-583, R1).
//
// The tool itself only validates and formats the result. Pinning and the state
// commit are performed by the stream loop when the call happens inside an LLM
// round (the loop owns the message indexes), or immediately by this tool when
// the load is user-initiated.
func (a *Agent) skillLoadTool(_ context.Context, args map[string]interface{}) (string, error) {
	name := strings.TrimSpace(argString(args, "name"))
	ls, content, err := a.SkillLoadContent(name)
	if err != nil {
		return "", err
	}
	result := a.skillLoadResultText(ls, content)

	if a.inLLMToolRound() {
		// Defer to the loop: it pins the assistant/tool message pair of this
		// round and commits the loaded state right after the tool result is
		// appended (see settlePendingSkillPin).
		a.mu.Lock()
		pending := ls
		a.pendingSkillPin = &pending
		a.mu.Unlock()
		return result, nil
	}

	// User-initiated load: append a self-contained pinned carrier at the end
	// of the history (no round is in flight, so appending is always legal).
	a.CommitSkillLoad(ls, result)
	return result, nil
}

// skillUnloadTool implements the skill_unload tool (FEATURE-583, R1). The
// content is not deleted here — only the pin is released, so the text stays
// visible until the next context reorganization or trimming drops it.
func (a *Agent) skillUnloadTool(_ context.Context, args map[string]interface{}) (string, error) {
	name := strings.TrimSpace(argString(args, "name"))
	if name == "" {
		return "", errors.New(i18n.T(i18n.KeySkillErrNameRequired))
	}
	ls, ok := a.SkillUnload(name)
	if !ok {
		return "", fmt.Errorf(i18n.T(i18n.KeySkillErrNotLoaded), name)
	}
	a.rebuildSystemPrompt()
	a.emitSkillState(SkillActionUnload, ls.Name)
	return fmt.Sprintf(i18n.T(i18n.KeySkillUnloadNote), ls.Name), nil
}

// SkillLoadForUser loads a skill on behalf of the user (:skill load, :skill
// reload, settings page): the content is pinned into the conversation and the
// loaded state is updated. It returns the record and the load result text.
func (a *Agent) SkillLoadForUser(name string) (LoadedSkill, string, error) {
	ls, content, err := a.SkillLoadContent(name)
	if err != nil {
		return LoadedSkill{}, "", err
	}
	result := a.skillLoadResultText(ls, content)
	a.CommitSkillLoad(ls, result)
	return ls, result, nil
}

// CommitSkillLoad pins the content as a self-contained message pair appended at
// the end of the history and records the loaded state. Used for user-initiated
// loads (REPL :skill load, settings API) where no LLM round is in flight.
func (a *Agent) CommitSkillLoad(ls LoadedSkill, resultText string) {
	a.mu.Lock()
	a.unpinSkillMessagesLocked(ls.Name)
	a.messages = append(a.messages, pinnedSkillCarrier(ls.Name, resultText, a.isXMLToolCallMode())...)
	if a.loadedSkills == nil {
		a.loadedSkills = make(map[string]LoadedSkill)
	}
	a.loadedSkills[ls.Name] = ls
	a.mu.Unlock()

	a.rebuildSystemPrompt()
	a.emitSkillState(SkillActionLoad, ls.Name)
}

// settlePendingSkillPin pins the messages of the round that just called
// skill_load and commits the loaded state (FEATURE-583). It is called by the
// stream loop right after the tool result message has been appended.
//
// assistantIdx is the index of the round's assistant message; the tool result
// is the last message in the history at this point.
func (a *Agent) settlePendingSkillPin(assistantIdx int) {
	a.mu.Lock()
	pending := a.pendingSkillPin
	a.pendingSkillPin = nil
	if pending == nil {
		a.mu.Unlock()
		return
	}
	ls := *pending
	a.unpinSkillMessagesLocked(ls.Name)
	if assistantIdx >= 0 && assistantIdx < len(a.messages) {
		a.messages[assistantIdx].Pinned = true
		a.messages[assistantIdx].PinnedFor = ls.Name
	}
	if last := len(a.messages) - 1; last >= 0 {
		a.messages[last].Pinned = true
		a.messages[last].PinnedFor = ls.Name
	}
	if a.loadedSkills == nil {
		a.loadedSkills = make(map[string]LoadedSkill)
	}
	a.loadedSkills[ls.Name] = ls
	a.mu.Unlock()

	a.rebuildSystemPrompt()
	a.emitSkillState(SkillActionLoad, ls.Name)
}

// SkillUnload removes a skill from the loaded set and releases the pin of its
// messages (FEATURE-583, D-B). The content itself is left in the history; it is
// dropped by the next context reorganization or trimming.
func (a *Agent) SkillUnload(name string) (LoadedSkill, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	ls, ok := a.loadedSkills[name]
	if !ok {
		return LoadedSkill{}, false
	}
	delete(a.loadedSkills, name)
	a.unpinSkillMessagesLocked(name)
	return ls, true
}

// unpinSkillMessagesLocked clears the pin of every message that belongs to the
// named skill. The caller must hold a.mu.
func (a *Agent) unpinSkillMessagesLocked(name string) int {
	n := 0
	for i := range a.messages {
		if a.messages[i].PinnedFor != name {
			continue
		}
		a.messages[i].Pinned = false
		a.messages[i].PinnedFor = ""
		n++
	}
	return n
}

// pinnedSkillCarrier builds the pinned message(s) carrying a skill's full text
// for a user-initiated load. In OpenAI tool-call mode it is an assistant
// tool_call plus its matching tool result (a valid, self-contained pair); in
// XML mode a single user message is used because that mode never emits
// tool_call/tool message pairs.
func pinnedSkillCarrier(name, resultText string, xmlMode bool) []llm.Message {
	if xmlMode {
		return []llm.Message{{
			Role:      "user",
			Content:   resultText,
			Pinned:    true,
			PinnedFor: name,
		}}
	}
	callID := fmt.Sprintf("skill-load-%s-%d", name, time.Now().UnixNano())
	call := llm.ToolCall{
		ID:        callID,
		Name:      toolSkillLoad,
		Type:      "function",
		Arguments: fmt.Sprintf(`{"name":%q}`, name),
	}
	return []llm.Message{
		{Role: "assistant", ToolCalls: []llm.ToolCall{call}, Pinned: true, PinnedFor: name},
		{Role: "tool", ToolCallID: callID, Content: resultText, Pinned: true, PinnedFor: name},
	}
}

// skillLoadResultText formats the skill_load result: a short metadata header
// followed by the byte-exact SKILL.md content between explicit markers.
func (a *Agent) skillLoadResultText(ls LoadedSkill, content string) string {
	total := a.LoadedSkillBytes()
	if prev, ok := a.GetLoadedSkill(ls.Name); ok {
		total -= prev.Bytes
	}
	total += ls.Bytes

	var sb strings.Builder
	sb.WriteString(i18n.TF(i18n.KeySkillLoadHeader, ls.Name, ls.Source, ls.Path,
		ls.Bytes, ls.Bytes/4, shortSHA(ls.SHA256)))
	sb.WriteString("\n")
	sb.WriteString(i18n.TF(i18n.KeySkillLoadPinnedNote, ls.Name))
	if resources := listSkillResources(ls.Dir); len(resources) > 0 {
		sb.WriteString("\n")
		sb.WriteString(i18n.TF(i18n.KeySkillLoadResources, strings.Join(resources, ", ")))
	}
	if warn := a.skillWarn(ls.Bytes, total); warn != "" {
		sb.WriteString("\n")
		sb.WriteString(i18n.TF(i18n.KeySkillLoadWarn, ls.Bytes, total))
	}
	sb.WriteString("\n\n")
	sb.WriteString(skillContentBeginMarker)
	sb.WriteString("\n")
	sb.WriteString(content)
	sb.WriteString(skillContentEndMarker)
	return sb.String()
}

// skillContentFromResult extracts the byte-exact SKILL.md text previously
// wrapped by skillLoadResultText. ok is false when the markers are absent.
func skillContentFromResult(result string) (string, bool) {
	start := strings.Index(result, skillContentBeginMarker+"\n")
	if start < 0 {
		return "", false
	}
	body := result[start+len(skillContentBeginMarker)+1:]
	end := strings.Index(body, skillContentEndMarker)
	if end < 0 {
		return "", false
	}
	return body[:end], true
}

// shortSHA returns the first 8 hex characters of a sha256 fingerprint.
func shortSHA(sha string) string {
	if len(sha) <= 8 {
		return sha
	}
	return sha[:8]
}
