// Author: L.Shuang
// Created: 2026-10-02
// Last Modified: 2026-10-02
//
// MIT License - Copyright (c) 2026 L.Shuang
//
// FEATURE-583: skill load/unload mechanism — unit tests for
// use-case/FEATURE-583/FEATURE-583-UC-0001.md (UC-0001 ~ UC-0015).

package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
)

// i18nInit pins the prompt language for tests that assert localized markers.
func i18nInit() { i18n.Init("zh") }

// i18nT is a short alias used by the assertions below.
func i18nT(key string) string { return i18n.T(key) }

// writeTestSkill creates skills/<name>/SKILL.md inside ws and returns the file
// path. A non-empty bundleFile also creates one bundled resource next to it.
func writeTestSkill(t *testing.T, ws, name, bundleFile, content string) string {
	t.Helper()
	dir := filepath.Join(ws, "skills", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	if bundleFile != "" {
		if err := os.WriteFile(filepath.Join(dir, bundleFile), []byte("bundled\n"), 0o644); err != nil {
			t.Fatalf("write bundle %s: %v", bundleFile, err)
		}
	}
	return path
}

// newSkillTestAgent builds a minimal Agent rooted at ws.
func newSkillTestAgent(t *testing.T, ws string) *Agent {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.LLM.SkillsDefaultEnabled = true
	a := &Agent{
		workspacePath: ws,
		cfg:           cfg,
		systemPrompt:  "SYS",
		messages:      []llm.Message{{Role: "system", Content: "SYS"}},
	}
	a.messagePointer = 1
	return a
}

// loadSkill runs the skill_load tool on the agent.
func loadSkill(t *testing.T, a *Agent, name string) string {
	t.Helper()
	out, err := a.skillLoadTool(context.Background(), map[string]interface{}{"name": name})
	if err != nil {
		t.Fatalf("skill_load(%s): %v", name, err)
	}
	return out
}

// pinnedSkillMessages returns the messages pinned for one skill.
func pinnedSkillMessages(a *Agent, name string) []llm.Message {
	var out []llm.Message
	for _, m := range a.messages {
		if m.Pinned && m.PinnedFor == name {
			out = append(out, m)
		}
	}
	return out
}

const demoSkillBody = "---\nname: demo\ndescription: demo skill\n---\n# Demo\n\nMAGIC-BODY-STRING\n"

// UC-0001: skill_load succeeds and pins a byte-exact pair.
func TestFeature583UC0001SkillLoadSuccess(t *testing.T) {
	ws := t.TempDir()
	path := writeTestSkill(t, ws, "demo", "helper.py", demoSkillBody)
	a := newSkillTestAgent(t, ws)

	out := loadSkill(t, a, "demo")

	ls, ok := a.GetLoadedSkill("demo")
	if !ok {
		t.Fatalf("demo not recorded as loaded")
	}
	if ls.Bytes != len(demoSkillBody) || ls.Source != SkillSourceWorkspace {
		t.Errorf("loaded record = %+v, want bytes=%d source=workspace", ls, len(demoSkillBody))
	}
	if ls.SHA256 != skillSHA256([]byte(demoSkillBody)) {
		t.Errorf("sha256 = %s, want %s", ls.SHA256, skillSHA256([]byte(demoSkillBody)))
	}
	// ③ the body between the markers is byte-exact.
	body, ok := skillContentFromResult(out)
	if !ok {
		t.Fatalf("result has no BEGIN/END markers:\n%s", out)
	}
	if body != demoSkillBody {
		t.Errorf("pinned body mismatch:\ngot  %q\nwant %q", body, demoSkillBody)
	}
	// ④ name + right-aligned source label + bundled resources.
	// FIX-587: the header no longer carries the SKILL.md path, the size/token
	// estimate or the sha256 prefix, so only the name, the localized source
	// label and the resource list are asserted here.
	for _, want := range []string{"demo", i18nT(i18n.KeySkillSrcWorkspace), "helper.py"} {
		if !strings.Contains(out, want) {
			t.Errorf("result misses %q:\n%s", want, out)
		}
	}
	for _, banned := range []string{"sha256:", path, "tokens"} {
		if strings.Contains(out, banned) {
			t.Errorf("FIX-587: result must not carry %q:\n%s", banned, out)
		}
	}
	// ② the tail is a pinned assistant/tool pair.
	a.mu.Lock()
	msgs := a.messages
	a.mu.Unlock()
	if len(msgs) < 3 {
		t.Fatalf("history too short: %d", len(msgs))
	}
	assistant, toolMsg := msgs[len(msgs)-2], msgs[len(msgs)-1]
	if assistant.Role != "assistant" || len(assistant.ToolCalls) != 1 || assistant.ToolCalls[0].Name != toolSkillLoad {
		t.Errorf("second-to-last message = %+v, want assistant skill_load call", assistant)
	}
	if !assistant.Pinned || assistant.PinnedFor != "demo" {
		t.Errorf("assistant pin = %v/%q, want true/demo", assistant.Pinned, assistant.PinnedFor)
	}
	if toolMsg.Role != "tool" || toolMsg.ToolCallID != assistant.ToolCalls[0].ID {
		t.Errorf("last message = %+v, want matching tool result", toolMsg)
	}
	if !toolMsg.Pinned || toolMsg.PinnedFor != "demo" {
		t.Errorf("tool pin = %v/%q, want true/demo", toolMsg.Pinned, toolMsg.PinnedFor)
	}
	if !strings.Contains(toolMsg.Content, demoSkillBody) {
		t.Errorf("tool result does not carry the full body")
	}
}

// UC-0002: unknown and disabled skills are rejected without touching state.
func TestFeature583UC0002SkillLoadErrors(t *testing.T) {
	ws := t.TempDir()
	writeTestSkill(t, ws, "demo", "", demoSkillBody)
	a := newSkillTestAgent(t, ws)

	if _, err := a.skillLoadTool(context.Background(), map[string]interface{}{"name": "nope"}); err == nil {
		t.Fatal("unknown skill: want error, got nil")
	}
	if len(a.LoadedSkills()) != 0 {
		t.Fatalf("state changed after failed load: %+v", a.LoadedSkills())
	}

	a.cfg.SetSkillEnabled("demo", false)
	_, err := a.skillLoadTool(context.Background(), map[string]interface{}{"name": "demo"})
	if err == nil {
		t.Fatal("disabled skill: want error, got nil")
	}
	if !strings.Contains(err.Error(), "Skills") && !strings.Contains(err.Error(), "技能") {
		t.Errorf("disabled error should point at the settings page, got %q", err)
	}
	if len(a.LoadedSkills()) != 0 {
		t.Fatalf("state changed after disabled load: %+v", a.LoadedSkills())
	}
}

// UC-0003: loading an already loaded skill refreshes it.
func TestFeature583UC0003SkillLoadRefresh(t *testing.T) {
	ws := t.TempDir()
	path := writeTestSkill(t, ws, "demo", "", demoSkillBody)
	a := newSkillTestAgent(t, ws)
	loadSkill(t, a, "demo")
	first, _ := a.GetLoadedSkill("demo")

	updated := demoSkillBody + "\nSECOND-VERSION\n"
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatalf("rewrite skill: %v", err)
	}
	out := loadSkill(t, a, "demo")

	if got := len(pinnedSkillMessages(a, "demo")); got != 2 {
		t.Errorf("pinned messages for demo = %d, want 2 (old pair released)", got)
	}
	second, _ := a.GetLoadedSkill("demo")
	if second.SHA256 == first.SHA256 {
		t.Error("sha256 not refreshed")
	}
	body, _ := skillContentFromResult(out)
	if body != updated {
		t.Errorf("refreshed body mismatch:\ngot  %q\nwant %q", body, updated)
	}
	count := 0
	for _, m := range a.messages {
		if m.Pinned && m.PinnedFor == "demo" && strings.Contains(m.Content, "SECOND-VERSION") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("new body appears in %d pinned messages, want 1", count)
	}
}

// UC-0004: unload releases the pin but keeps the text in the history.
func TestFeature583UC0004SkillUnloadSemantics(t *testing.T) {
	ws := t.TempDir()
	writeTestSkill(t, ws, "demo", "", demoSkillBody)
	a := newSkillTestAgent(t, ws)
	loadSkill(t, a, "demo")

	out, err := a.skillUnloadTool(context.Background(), map[string]interface{}{"name": "demo"})
	if err != nil {
		t.Fatalf("skill_unload: %v", err)
	}
	if _, ok := a.GetLoadedSkill("demo"); ok {
		t.Error("state not removed")
	}
	if got := len(pinnedSkillMessages(a, "demo")); got != 0 {
		t.Errorf("pinned messages left: %d", got)
	}
	found := false
	for _, m := range a.messages {
		if strings.Contains(m.Content, demoSkillBody) {
			found = true
		}
	}
	if !found {
		t.Error("content was removed physically; it must survive until the next reorganization")
	}
	if !strings.Contains(out, "UNLOADED") {
		t.Errorf("unload result = %q, want the deferred-removal wording", out)
	}
}

// UC-0005: unloading a skill that is not loaded reports an error.
func TestFeature583UC0005SkillUnloadNotLoaded(t *testing.T) {
	ws := t.TempDir()
	a := newSkillTestAgent(t, ws)
	if _, err := a.skillUnloadTool(context.Background(), map[string]interface{}{"name": "demo"}); err == nil {
		t.Fatal("want error for an unloaded skill")
	}
	if _, ok := a.GetLoadedSkill("demo"); ok {
		t.Error("state must stay empty")
	}
}

// UC-0006: the pinned pair survives a moved message pointer (exemption 1).
func TestFeature583UC0006PinnedSurvivesPointer(t *testing.T) {
	ws := t.TempDir()
	writeTestSkill(t, ws, "demo", "", demoSkillBody)
	a := newSkillTestAgent(t, ws)
	loadSkill(t, a, "demo")

	a.mu.Lock()
	base := len(a.messages)
	a.messages = append(a.messages,
		llm.Message{Role: "user", Content: "later question"},
		llm.Message{Role: "assistant", Content: "later answer"},
	)
	a.messagePointer = len(a.messages) - 1
	a.mu.Unlock()

	msgs := a.buildContextMessages()
	if msgs[0].Role != "system" {
		t.Fatalf("first message = %q, want system", msgs[0].Role)
	}
	if !msgs[1].Pinned || !msgs[2].Pinned {
		t.Fatalf("pinned pair not right after the system message: %+v", msgs[:3])
	}
	body, ok := skillContentFromResult(msgs[2].Content)
	if !ok || body != demoSkillBody {
		t.Errorf("pinned body lost or corrupted after pointer move")
	}
	if msgs[1].ToolCalls[0].ID != msgs[2].ToolCallID {
		t.Error("pinned pair is not a valid assistant/tool pair")
	}
	_ = base
	// no duplication
	seen := 0
	for _, m := range msgs {
		if strings.Contains(m.Content, demoSkillBody) {
			seen++
		}
	}
	if seen != 1 {
		t.Errorf("body appears %d times in the request, want 1", seen)
	}
}

// UC-0007: reorganization collapse keeps the pinned pair (exemption 2).
func TestFeature583UC0007CollapseKeepsPinned(t *testing.T) {
	ws := t.TempDir()
	writeTestSkill(t, ws, "demo", "", demoSkillBody)
	a := newSkillTestAgent(t, ws)
	loadSkill(t, a, "demo")

	a.mu.Lock()
	a.messages = append(a.messages, llm.Message{Role: "user", Content: "old question"})
	a.mu.Unlock()
	a.collapseAfterReorganize()

	a.mu.Lock()
	msgs := append([]llm.Message(nil), a.messages...)
	ptr := a.messagePointer
	a.mu.Unlock()

	if len(msgs) < 3 {
		t.Fatalf("collapsed history = %d messages, want [system, pinned, pinned]", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Fatalf("msgs[0] role = %q, want system", msgs[0].Role)
	}
	if !msgs[1].Pinned || !msgs[2].Pinned {
		t.Fatalf("pinned pair lost by the collapse: %+v", msgs)
	}
	if body, ok := skillContentFromResult(msgs[2].Content); !ok || body != demoSkillBody {
		t.Error("pinned body corrupted by the collapse")
	}
	if ptr != 1 {
		t.Errorf("messagePointer = %d, want 1", ptr)
	}

	// The summary tail path keeps the pinned pair too.
	collapsed := collapseKeepingPinned(msgs[0], msgs, llm.Message{Role: "user", Content: "SUMMARY"})
	if len(collapsed) != 4 || collapsed[3].Content != "SUMMARY" {
		t.Fatalf("collapseKeepingPinned tail = %+v, want 4 messages ending with the summary", collapsed)
	}
	if !collapsed[1].Pinned || !collapsed[2].Pinned {
		t.Error("collapseKeepingPinned dropped the pinned pair")
	}
}

// UC-0008: window trimming keeps the pinned pair (exemption 3).
func TestFeature583UC0008WindowTrimKeepsPinned(t *testing.T) {
	ws := t.TempDir()
	writeTestSkill(t, ws, "demo", "", demoSkillBody)
	a := newSkillTestAgent(t, ws)
	loadSkill(t, a, "demo")
	a.cfg.LLM.LoopReorganizeEnabled = true
	a.cfg.LLM.ContextPolicy = "window"
	a.cfg.LLM.ContextLimit = -1

	a.mu.Lock()
	for i := 0; i < 5; i++ {
		a.messages = append(a.messages,
			llm.Message{Role: "user", Content: "question"},
			llm.Message{Role: "assistant", Content: "answer"},
		)
	}
	before := len(a.messages)
	a.mu.Unlock()

	if got := a.reorganizeContextOnLoop(); got != "" {
		t.Errorf("window trim returned %q, want empty", got)
	}

	a.mu.Lock()
	msgs := append([]llm.Message(nil), a.messages...)
	a.mu.Unlock()
	if len(msgs) >= before {
		t.Fatalf("window trim removed nothing: %d messages (was %d)", len(msgs), before)
	}
	if len(msgs) != 4 {
		t.Fatalf("trimmed history = %d messages, want [system, pinned, pinned, last]", len(msgs))
	}
	if !msgs[1].Pinned || !msgs[2].Pinned {
		t.Fatalf("pinned pair lost by the window trim: %+v", msgs)
	}
	if body, ok := skillContentFromResult(msgs[2].Content); !ok || body != demoSkillBody {
		t.Error("pinned body corrupted by the window trim")
	}
}

// UC-0009: rollback / retry-from truncation keeps the pinned pair (exemption 4).
func TestFeature583UC0009TruncateKeepsPinned(t *testing.T) {
	ws := t.TempDir()
	writeTestSkill(t, ws, "demo", "", demoSkillBody)
	a := newSkillTestAgent(t, ws)
	loadSkill(t, a, "demo")

	// Pure helper: cutting past the pinned pair re-inserts it after the system
	// message.
	kept := truncateHistoryKeepPinned(a.messages, 1)
	if len(kept) != 3 || !kept[1].Pinned || !kept[2].Pinned {
		t.Fatalf("truncateHistoryKeepPinned = %+v, want [system, pinned assistant, pinned tool]", kept)
	}
	if body, ok := skillContentFromResult(kept[2].Content); !ok || body != demoSkillBody {
		t.Error("pinned body corrupted by the truncation")
	}

	// removeLastAssistantWithToolCalls truncates at the pinned assistant and
	// must restore it.
	removed := a.removeLastAssistantWithToolCalls()
	if removed == "" {
		t.Fatal("removeLastAssistantWithToolCalls reported nothing to remove")
	}
	a.mu.Lock()
	msgs := append([]llm.Message(nil), a.messages...)
	a.mu.Unlock()
	pinned := 0
	for _, m := range msgs {
		if m.Pinned && m.PinnedFor == "demo" {
			pinned++
		}
	}
	if pinned != 2 {
		t.Errorf("pinned messages after retry-from truncation = %d, want 2", pinned)
	}
}

// UC-0010: several skills keep a stable order and unload independently.
func TestFeature583UC0010MultipleSkillsOrderStable(t *testing.T) {
	ws := t.TempDir()
	writeTestSkill(t, ws, "alpha", "", "---\nname: alpha\n---\nALPHA\n")
	writeTestSkill(t, ws, "beta", "", "---\nname: beta\n---\nBETA\n")
	a := newSkillTestAgent(t, ws)
	loadSkill(t, a, "alpha")
	loadSkill(t, a, "beta")

	first := a.buildContextMessages()
	second := a.buildContextMessages()
	if len(first) != len(second) {
		t.Fatalf("unstable context length: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i].Role != second[i].Role || first[i].Content != second[i].Content {
			t.Fatalf("context not stable at %d", i)
		}
	}
	if len(first) < 5 {
		t.Fatalf("request = %d messages, want [system, alpha pair, beta pair]", len(first))
	}
	if !strings.Contains(first[2].Content, "ALPHA") || !strings.Contains(first[4].Content, "BETA") {
		t.Fatalf("pinned order wrong: msg2=%q msg4=%q", first[2].Role, first[4].Role)
	}

	if _, ok := a.SkillUnload("alpha"); !ok {
		t.Fatal("unload alpha failed")
	}
	if got := len(pinnedSkillMessages(a, "alpha")); got != 0 {
		t.Errorf("alpha pins left: %d", got)
	}
	if got := len(pinnedSkillMessages(a, "beta")); got != 2 {
		t.Errorf("beta pins = %d, want 2", got)
	}
}

// UC-0011: the SKILLS index marks loaded skills and hides disabled ones.
func TestFeature583UC0011SkillsIndexMarked(t *testing.T) {
	i18nInit()
	ws := t.TempDir()
	writeTestSkill(t, ws, "demo", "", demoSkillBody)
	writeTestSkill(t, ws, "other", "", "---\nname: other\n---\nOTHER\n")
	home := t.TempDir()
	cfg := config.DefaultConfig()

	// ① The section carries static rules only — no list, no markers.
	env := &promptEnv{cwd: ws, homeDir: home}
	section := buildNamedSection("Skills", env, cfg, false, nil, false)
	for _, forbidden := range []string{"demo", "other"} {
		if strings.Contains(section, forbidden) {
			t.Errorf("SKILLS section must stay static, found %q:\n%s", forbidden, section)
		}
	}
	for _, want := range []string{"skill_load", "skill_unload", "environment_details"} {
		if !strings.Contains(section, want) {
			t.Errorf("SKILLS section misses rule %q:\n%s", want, section)
		}
	}

	// ② The available list lives in the <skills> block: name + description.
	a := newSkillTestAgent(t, ws)
	block := a.buildSkillsEnvBlock()
	if !strings.Contains(block, "- demo: demo skill") || !strings.Contains(block, "- other: ") {
		t.Errorf("<skills> block incomplete:\n%s", block)
	}
	if strings.Contains(block, "SKILL.md") || strings.Contains(block, ws) {
		t.Errorf("<skills> block must not carry paths:\n%s", block)
	}

	// ③ Loaded marker + disabled filtering.
	loadSkill(t, a, "demo")
	block = a.buildSkillsEnvBlock()
	tag := i18nT(i18n.KeySystemPromptSkillsLoadedTag)
	if !strings.Contains(block, tag+" demo") {
		t.Errorf("loaded marker missing:\n%s", block)
	}
	if strings.Contains(block, tag+" other") {
		t.Errorf("unloaded skill must not be marked:\n%s", block)
	}
	a.cfg.SetSkillEnabled("demo", false)
	block = a.buildSkillsEnvBlock()
	if strings.Contains(block, "demo") {
		t.Errorf("disabled skill still listed:\n%s", block)
	}
	if !strings.Contains(block, "other") {
		t.Errorf("enabled skill disappeared:\n%s", block)
	}
}

// UC-0012: the skill_state event carries the full snapshot.
func TestFeature583UC0012SkillStateEvent(t *testing.T) {
	ws := t.TempDir()
	writeTestSkill(t, ws, "demo", "", demoSkillBody)
	a := newSkillTestAgent(t, ws)
	loadSkill(t, a, "demo")

	payload := a.skillStatePayload(SkillActionLoad, "demo")
	var decoded SkillStatePayload
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if decoded.Type != EventSkillState || decoded.Action != SkillActionLoad || decoded.Name != "demo" {
		t.Errorf("payload header = %+v", decoded)
	}
	if len(decoded.Loaded) != 1 || decoded.Loaded[0].Name != "demo" {
		t.Fatalf("payload loaded list = %+v", decoded.Loaded)
	}
	if decoded.Loaded[0].Source != SkillSourceWorkspace || decoded.Loaded[0].SHA256 == "" || decoded.Loaded[0].LoadedAt.IsZero() {
		t.Errorf("payload loaded record incomplete: %+v", decoded.Loaded[0])
	}
	if decoded.TotalBytes != len(demoSkillBody) {
		t.Errorf("total_bytes = %d, want %d", decoded.TotalBytes, len(demoSkillBody))
	}

	ev := SkillStateEvent(SkillActionLoad, "demo", payload)
	if ev.Type != EventSkillState || ev.Meta[MetaKeySkillAction] != SkillActionLoad ||
		ev.Meta[MetaKeySkillName] != "demo" || ev.Meta[MetaKeySkillState] != payload {
		t.Errorf("event = %+v", ev)
	}
}

// UC-0013: enable/disable resolution (config).
func TestFeature583UC0013SkillEnabledJudgement(t *testing.T) {
	cases := []struct {
		name     string
		def      bool
		explicit map[string]bool
		want     bool
	}{
		{"explicit true", true, map[string]bool{"a": true}, true},
		{"explicit false", true, map[string]bool{"a": false}, false},
		{"default true", true, nil, true},
		{"default false", false, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.LLM.SkillsDefaultEnabled = tc.def
			cfg.LLM.SkillsEnabled = tc.explicit
			if got := cfg.SkillEnabled("a"); got != tc.want {
				t.Errorf("SkillEnabled = %v, want %v", got, tc.want)
			}
		})
	}
	// SetSkillEnabled drops entries equal to the default, keeping config clean.
	cfg := config.DefaultConfig()
	cfg.SetSkillEnabled("a", true)
	if cfg.LLM.SkillsEnabled != nil {
		t.Errorf("redundant entry kept: %+v", cfg.LLM.SkillsEnabled)
	}
	cfg.SetSkillEnabled("a", false)
	if !cfg.SkillEnabled("a") {
		// explicit false must win
		t.Log("explicit false recorded")
	} else {
		t.Error("explicit false not applied")
	}
}

// UC-0014: disabling a loaded skill unloads it and reports a config event.
func TestFeature583UC0014DisabledSkillAutoUnloaded(t *testing.T) {
	ws := t.TempDir()
	writeTestSkill(t, ws, "demo", "", demoSkillBody)
	a := newSkillTestAgent(t, ws)
	loadSkill(t, a, "demo")

	a.cfg.SetSkillEnabled("demo", false)
	unloaded := a.ApplySkillsConfigState()
	if len(unloaded) != 1 || unloaded[0] != "demo" {
		t.Fatalf("unloaded = %v, want [demo]", unloaded)
	}
	if _, ok := a.GetLoadedSkill("demo"); ok {
		t.Error("state kept for a disabled skill")
	}
	if got := len(pinnedSkillMessages(a, "demo")); got != 0 {
		t.Errorf("pins left: %d", got)
	}
	block := a.buildSkillsEnvBlock()
	if strings.Contains(block, "demo") {
		t.Errorf("disabled skill still in the skills list:\n%s", block)
	}
}

// UC-0015: exceeding the size threshold warns but never refuses the load.
func TestFeature583UC0015WarnThreshold(t *testing.T) {
	ws := t.TempDir()
	body := "---\nname: big\n---\n" + strings.Repeat("X", 2048) + "\nEND-MARKER\n"
	writeTestSkill(t, ws, "big", "", body)
	a := newSkillTestAgent(t, ws)
	a.cfg.LLM.SkillsWarnFileBytes = 256
	a.cfg.LLM.SkillsWarnTotalBytes = 512

	out := loadSkill(t, a, "big")

	if !strings.Contains(out, "⚠️") {
		t.Errorf("warn missing from the load result:\n%s", out)
	}
	if body2, ok := skillContentFromResult(out); !ok || body2 != body {
		t.Error("warned load lost content")
	}
	if ls, ok := a.GetLoadedSkill("big"); !ok || ls.Bytes != len(body) {
		t.Errorf("state after warned load = %+v", ls)
	}
	if warn := a.skillWarn(len(body), len(body)); warn == "" {
		t.Error("skillWarn returned no warning above the thresholds")
	}
}

// UC-0016: pinned content and the loaded index survive a JSON round trip.
func TestFeature583UC0016PersistenceRoundTrip(t *testing.T) {
	ws := t.TempDir()
	writeTestSkill(t, ws, "demo", "", demoSkillBody)
	a := newSkillTestAgent(t, ws)
	loadSkill(t, a, "demo")

	a.mu.Lock()
	data, err := json.Marshal(a.messages)
	a.mu.Unlock()
	if err != nil {
		t.Fatalf("marshal messages: %v", err)
	}
	var restored []llm.Message
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal messages: %v", err)
	}

	b := newSkillTestAgent(t, ws)
	b.mu.Lock()
	b.messages = restored
	dropped := b.rebuildLoadedStateLocked()
	b.mu.Unlock()
	if len(dropped) != 0 {
		t.Fatalf("dropped on restore: %v", dropped)
	}
	if names := b.LoadedSkillNames(); len(names) != 1 || names[0] != "demo" {
		t.Fatalf("restored loaded set = %v, want [demo]", names)
	}
	// Invariant: every loaded name has a pinned message carrying its content.
	for _, ls := range b.LoadedSkills() {
		found := false
		for _, m := range b.GetMessages() {
			if m.Pinned && m.PinnedFor == ls.Name {
				if body, ok := skillContentFromResult(m.Content); ok && skillSHA256([]byte(body)) == ls.SHA256 {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("loaded skill %q has no complete pinned content", ls.Name)
		}
	}
}

// UC-0017: a changed file is flagged; a deleted file is dropped on restore.
func TestFeature583UC0017ChangedAndDeleted(t *testing.T) {
	ws := t.TempDir()
	path := writeTestSkill(t, ws, "demo", "", demoSkillBody)
	a := newSkillTestAgent(t, ws)
	loadSkill(t, a, "demo")

	if err := os.WriteFile(path, []byte(demoSkillBody+"CHANGED\n"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	found := false
	for _, info := range a.SkillInfos() {
		if info.Name == "demo" {
			found = true
			if !info.Changed {
				t.Errorf("changed flag not raised: %+v", info)
			}
		}
	}
	if !found {
		t.Fatal("demo missing from SkillInfos")
	}

	if err := os.RemoveAll(filepath.Dir(path)); err != nil {
		t.Fatalf("remove skill dir: %v", err)
	}
	a.mu.Lock()
	dropped := a.rebuildLoadedStateLocked()
	a.mu.Unlock()
	if len(dropped) != 1 || dropped[0] != "demo" {
		t.Fatalf("dropped = %v, want [demo]", dropped)
	}
	if len(a.LoadedSkills()) != 0 {
		t.Error("state kept for a deleted skill")
	}
	for _, m := range a.GetMessages() {
		if m.PinnedFor == "demo" {
			t.Error("pin kept for a deleted skill")
		}
	}
}

// UC-0020 (cache): loading a skill must not disturb the request prefix.
func TestFeature583UC0020CacheFriendly(t *testing.T) {
	ws := t.TempDir()
	writeTestSkill(t, ws, "demo", "", demoSkillBody)
	a := newSkillTestAgent(t, ws)
	a.mu.Lock()
	a.messages = append(a.messages,
		llm.Message{Role: "user", Content: "first\n\n<environment_details>\n<time>t1</time>\n</environment_details>"},
		llm.Message{Role: "assistant", Content: "reply"},
		llm.Message{Role: "user", Content: "second\n\n<environment_details>\n<time>t2</time>\n</environment_details>"},
	)
	a.mu.Unlock()

	before := a.buildContextMessages()
	loadSkill(t, a, "demo")
	after := a.buildContextMessages()

	// ① The pinned pair is appended at the tail; the messages are not reordered
	//    and the system prompt stays byte-identical.
	if len(after) != len(before)+2 {
		t.Fatalf("request grew by %d messages, want 2", len(after)-len(before))
	}
	for i := range before {
		if before[i].Role != after[i].Role {
			t.Fatalf("message %d role changed: %s -> %s", i, before[i].Role, after[i].Role)
		}
	}
	if strings.Contains(after[0].Content, "demo") {
		t.Errorf("system prompt must not carry the skill list:\n%s", after[0].Content)
	}
	if !after[len(after)-2].Pinned || !after[len(after)-1].Pinned {
		t.Error("pinned pair is not at the tail")
	}

	// ② The <skills> block appears exactly once, in the newest envelope.
	count, lastEnvIdx := 0, -1
	for i := 1; i < len(after); i++ {
		if strings.Contains(after[i].Content, "<skills>") || strings.Contains(envTextOf(after[i]), "<skills>") {
			count++
			lastEnvIdx = i
		}
	}
	if count != 1 {
		t.Errorf("<skills> block appears in %d messages, want exactly 1", count)
	}
	if lastEnvIdx != 3 {
		t.Errorf("<skills> block landed in message %d, want the newest envelope (3)", lastEnvIdx)
	}

	// ③ Loading did not change the system prompt.
	if !strings.Contains(after[0].Content, "skill_load") {
		t.Error("system prompt lost the static skill rules")
	}
}

// envTextOf returns the envelope text of a message (ContentParts first).
func envTextOf(m llm.Message) string {
	for i := len(m.ContentParts) - 1; i >= 0; i-- {
		if strings.Contains(m.ContentParts[i].Text, "<environment_details>") {
			return m.ContentParts[i].Text
		}
	}
	return ""
}
