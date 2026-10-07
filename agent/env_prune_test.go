// Tests for FEATURE-579 plans A/B/C and FEATURE-622: historical env trimming,
// single-block tool batches and the env slim-down helpers.
//
// Author: L.Shuang
// Created: 2026-10-02
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/llm"
)

// testEnv builds a representative <environment_details> envelope.
func testEnv(msgNo int, withTaskPlan bool) string {
	var sb strings.Builder
	sb.WriteString("<environment_details>\n")
	sb.WriteString("<time>2026-10-02 00:00:00 Friday</time>\n")
	sb.WriteString("<message_no>" + itoa(msgNo) + "</message_no>\n")
	sb.WriteString("<context_window>1K / 512K</context_window>\n")
	sb.WriteString("<cwd>/tmp/demo</cwd>\n")
	sb.WriteString("<current_mode>act</current_mode>\n")
	sb.WriteString("<no_tool_action>prompt</no_tool_action>\n")
	sb.WriteString("<current_dir>\nalpha.txt\nbeta.txt\n</current_dir>\n")
	sb.WriteString("<tools>\nbin/tool.sh\n</tools>\n")
	sb.WriteString("<research>\nresearch/note.md\n</research>\n")
	sb.WriteString("<board_agents rev=\"7\">\n<agent id=\"a1\" name=\"a1\" status=\"online\">responsible for everything</agent>\n</board_agents>\n")
	if withTaskPlan {
		sb.WriteString("<task_plan>\nStep 1: do it\n</task_plan>\n")
	}
	sb.WriteString("<opened_resources>\n<browser>none</browser>\n</opened_resources>\n")
	sb.WriteString("</environment_details>")
	return sb.String()
}

// envMsgs returns n messages that each carry an envelope (1 = oldest).
func envMsgs(n int, taskPlanFrom int) []llm.Message {
	msgs := make([]llm.Message, 0, n)
	for i := 0; i < n; i++ {
		text := testEnv(i+1, i+1 >= taskPlanFrom)
		msgs = append(msgs, llm.Message{
			Role:         "user",
			ContentParts: []llm.ContentPart{{Type: llm.ContentPartText, Text: text}},
		})
	}
	return msgs
}

func testAgent(t *testing.T) *Agent {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.LLM.EnvHistoryPrune = true
	return &Agent{cfg: cfg}
}

// trimmedTags are the blocks that a trimmed envelope must never carry.
var trimmedTags = []string{
	"<cwd>", "<context_window>", "<current_mode>", "<no_tool_action>",
	"<current_dir>", "<tools>", "<research>", "<board_agents", "<task_plan>",
	"<opened_resources>", "<session_title>", "<sub_agents", "<board_replies",
}

// TestEnvPruneOnlyNewestFull is the FEATURE-622 core: the newest envelope
// carrier keeps everything, every older envelope keeps <time>+<message_no>.
func TestEnvPruneOnlyNewestFull(t *testing.T) {
	a := testAgent(t)
	got := a.pruneEnvContext(envMsgs(4, 1))

	newest := envTextAt(&got[3])
	for _, want := range []string{
		"<time>", "<message_no>", "<context_window>", "<cwd>", "<current_mode>",
		"<current_dir>", "<tools>", "<research>", "<board_agents", "<task_plan>",
		"<opened_resources>",
	} {
		if !strings.Contains(newest, want) {
			t.Fatalf("newest envelope must keep %s, got: %q", want, newest)
		}
	}
	for i := 0; i < 3; i++ {
		old := envTextAt(&got[i])
		if !strings.Contains(old, "<time>") || !strings.Contains(old, "<message_no>") {
			t.Fatalf("index %d must keep time+message_no, got: %q", i, old)
		}
		for _, unwanted := range trimmedTags {
			if strings.Contains(old, unwanted) {
				t.Fatalf("index %d must drop %s, got: %q", i, unwanted, old)
			}
		}
	}
	t.Logf("TRIM|newest_bytes=%d|trimmed_bytes=%d,%d,%d", len(newest),
		len(envTextAt(&got[0])), len(envTextAt(&got[1])), len(envTextAt(&got[2])))
}

// TestEnvPruneNewestCarrierFromTail: the full envelope is the last user/tool
// message carrying one, not necessarily the last element of the slice.
func TestEnvPruneNewestCarrierFromTail(t *testing.T) {
	a := testAgent(t)
	msgs := envMsgs(3, 1)
	msgs = append(msgs, llm.Message{Role: "assistant", Content: "thinking"})
	got := a.pruneEnvContext(msgs)

	carrier := envTextAt(&got[2])
	if !strings.Contains(carrier, "<current_dir>") || !strings.Contains(carrier, "<task_plan>") {
		t.Fatalf("the newest carrier (index 2) must stay full, got: %q", carrier)
	}
	for i := 0; i < 2; i++ {
		if strings.Contains(envTextAt(&got[i]), "<current_dir>") {
			t.Fatalf("index %d must be trimmed, got: %q", i, envTextAt(&got[i]))
		}
	}
	t.Logf("CARRIER_FROM_TAIL|full_index=2|last_role=assistant")
}

// TestEnvPruneNoTaskPlanFallback: FEATURE-622 removed the task_plan fallback —
// when the newest envelope skips the plan, no older envelope is promoted.
func TestEnvPruneNoTaskPlanFallback(t *testing.T) {
	a := testAgent(t)
	msgs := envMsgs(3, 1)
	for _, i := range []int{1, 2} {
		msgs[i] = llm.Message{Role: "user", ContentParts: []llm.ContentPart{
			{Type: llm.ContentPartText, Text: testEnv(i+1, false)},
		}}
	}
	got := a.pruneEnvContext(msgs)
	oldest := envTextAt(&got[0])
	if strings.Contains(oldest, "<task_plan>") {
		t.Fatalf("removed fallback must not promote an older envelope, got: %q", oldest)
	}
	t.Logf("NO_TASKPLAN_FALLBACK|oldest_bytes=%d", len(oldest))
}

// TestEnvPruneNoMutatePersistent verifies the persisted history is untouched.
func TestEnvPruneNoMutatePersistent(t *testing.T) {
	a := testAgent(t)
	persistent := envMsgs(4, 1)
	before := make([]string, len(persistent))
	for i := range persistent {
		before[i] = envTextAt(&persistent[i])
	}
	a.messages = persistent

	// buildContextMessages hands a copy to pruneEnvContext — emulate it.
	copied := make([]llm.Message, len(a.messages))
	copy(copied, a.messages)
	pruned := a.pruneEnvContext(copied)

	for i := range a.messages {
		if envTextAt(&a.messages[i]) != before[i] {
			t.Fatalf("persisted history changed at index %d", i)
		}
	}
	// The copy must not share the ContentParts backing array.
	pruned[1].ContentParts[0].Text = "tampered"
	if envTextAt(&a.messages[1]) != before[1] {
		t.Fatalf("pruned copy shares the ContentParts backing array with a.messages")
	}
	t.Logf("NO_MUTATE|persistent_unchanged=true|deep_copy=true")
}

// TestEnvPruneDisabled is the backward-compatibility check (switch off).
func TestEnvPruneDisabled(t *testing.T) {
	a := testAgent(t)
	a.cfg.LLM.EnvHistoryPrune = false
	msgs := envMsgs(4, 1)
	before := make([]string, len(msgs))
	for i := range msgs {
		before[i] = envTextAt(&msgs[i])
	}
	got := a.pruneEnvContext(msgs)
	for i := range got {
		if envTextAt(&got[i]) != before[i] {
			t.Fatalf("switch off must not change any envelope, index %d", i)
		}
	}
	t.Logf("DISABLED|unchanged=true|every_envelope_full=true")
}

// TestMinimalEnvBlock covers the plan B helper used for non-final tool results.
func TestMinimalEnvBlock(t *testing.T) {
	s := minimalEnvBlock("2026-10-02 00:00:00 Friday", 7)
	if !strings.Contains(s, "<time>2026-10-02 00:00:00 Friday</time>") || !strings.Contains(s, "<message_no>7</message_no>") {
		t.Fatalf("minimal env must carry time+message_no, got: %q", s)
	}
	for _, tag := range []string{"<cwd>", "<current_dir>", "<tools>", "<board_agents>", "<task_plan>"} {
		if strings.Contains(s, tag) {
			t.Fatalf("minimal env must not contain %s: %q", tag, s)
		}
	}
	t.Logf("MINIMAL_ENV|bytes=%d", len(s))
}

// TestEnvStaticBlocksChangeDetect covers plan C-2 with the history trim off:
// the placeholder is still produced while the listings stay unchanged.
func TestEnvStaticBlocksChangeDetect(t *testing.T) {
	a := testAgent(t)
	a.cfg.LLM.EnvHistoryPrune = false
	a.cfg.LLM.EnvDirChangeDetect = true
	full := a.envStaticBlocks(1, "alpha.txt\n", "bin/tool.sh\n", "research/note.md\n", true, true, true)
	if !strings.Contains(full, "<current_dir>") || !strings.Contains(full, "<tools>") || !strings.Contains(full, "<research>") {
		t.Fatalf("first envelope must carry the full listings, got: %q", full)
	}
	same := a.envStaticBlocks(2, "alpha.txt\n", "bin/tool.sh\n", "research/note.md\n", true, true, true)
	if !strings.Contains(same, "<unchanged since msg#1/>") || strings.Contains(same, "alpha.txt") {
		t.Fatalf("unchanged listings must collapse to a placeholder, got: %q", same)
	}
	changed := a.envStaticBlocks(3, "alpha.txt\ngamma.txt\n", "bin/tool.sh\n", "research/note.md\n", true, true, true)
	if !strings.Contains(changed, "gamma.txt") {
		t.Fatalf("changed listings must be emitted in full, got: %q", changed)
	}
	t.Logf("DIRDETECT_OFF|full=%d|placeholder=%d|changed=%d", len(full), len(same), len(changed))
}

// TestEnvStaticBlocksSelfContainedWithTrim is the FEATURE-622 counterpart: with
// the history trim on the newest envelope is the only complete one, so it must
// never carry a cross-message reference.
func TestEnvStaticBlocksSelfContainedWithTrim(t *testing.T) {
	a := testAgent(t)
	a.cfg.LLM.EnvDirChangeDetect = true
	first := a.envStaticBlocks(1, "alpha.txt\n", "bin/tool.sh\n", "research/note.md\n", true, true, true)
	second := a.envStaticBlocks(2, "alpha.txt\n", "bin/tool.sh\n", "research/note.md\n", true, true, true)
	for _, env := range []string{first, second} {
		if strings.Contains(env, "<unchanged") {
			t.Fatalf("trim on must never emit a placeholder, got: %q", env)
		}
	}
	for _, want := range []string{"alpha.txt", "bin/tool.sh", "research/note.md"} {
		if !strings.Contains(second, want) {
			t.Fatalf("trim on must re-emit %s, got: %q", want, second)
		}
	}
	t.Logf("DIRDETECT_ON|first_bytes=%d|second_bytes=%d", len(first), len(second))
}

// TestEnvRuntimeInfoSelfContainedWithTrim guards the third slimmer: with the
// trim on runtime_info is emitted every time; with the trim off an unchanged
// value is suppressed as before.
func TestEnvRuntimeInfoSelfContainedWithTrim(t *testing.T) {
	runtime := RuntimeInfo{PID: 4242, Version: "0.105.0", Build: "1262", ServiceMode: "serve"}

	a := testAgent(t)
	a.cfg.LLM.EnvRuntimeInfoOnce = true
	a.runtimeInfo = runtime
	first := a.runtimeInfoBlockSlim()
	second := a.runtimeInfoBlockSlim()
	if first == "" || second == "" {
		t.Fatalf("trim on must emit runtime_info every time: first=%q second=%q", first, second)
	}
	if !strings.Contains(second, "<pid>4242</pid>") {
		t.Fatalf("runtime_info must carry the values, got: %q", second)
	}

	b := testAgent(t)
	b.cfg.LLM.EnvHistoryPrune = false
	b.cfg.LLM.EnvRuntimeInfoOnce = true
	b.runtimeInfo = runtime
	if ri := b.runtimeInfoBlockSlim(); ri == "" {
		t.Fatalf("trim off must still emit runtime_info on the first envelope")
	}
	if ri := b.runtimeInfoBlockSlim(); ri != "" {
		t.Fatalf("trim off must suppress unchanged runtime_info, got: %q", ri)
	}
	t.Logf("RUNTIME_INFO|trim_on_second_bytes=%d|trim_off_second_suppressed=true", len(second))
}

// TestEnvHistoryTrimOnSemantics documents the single switch that drives both
// halves of FEATURE-622.
func TestEnvHistoryTrimOnSemantics(t *testing.T) {
	on := testAgent(t)
	if !on.envHistoryTrimOn() {
		t.Fatalf("envHistoryTrimOn must be true with env_history_prune on")
	}
	off := testAgent(t)
	off.cfg.LLM.EnvHistoryPrune = false
	if off.envHistoryTrimOn() {
		t.Fatalf("envHistoryTrimOn must be false with env_history_prune off")
	}
	if (&Agent{}).envHistoryTrimOn() {
		t.Fatalf("nil config must report the trim as off")
	}
	t.Logf("TRIM_SWITCH|on=true|off=false|nil_cfg=false")
}

// TestBoardRosterBriefCovers plan C-1: rev parsing + brief roster rendering.
func TestBoardRosterBrief(t *testing.T) {
	block := "<board_agents rev=\"450\">\n<agent id=\"admin\" name=\"admin\" status=\"online\">You are a strict agent.</agent>\n<agent id=\"work\" name=\"work\" status=\"offline\">Helper</agent>\n</board_agents>"
	if rev := boardRosterRev(block); rev != "450" {
		t.Fatalf("boardRosterRev = %q, want 450", rev)
	}
	brief := briefBoardRoster(block)
	if strings.Contains(brief, "You are a strict agent.") || strings.Contains(brief, "Helper") {
		t.Fatalf("brief roster must drop the responsibility text, got: %q", brief)
	}
	for _, want := range []string{`id="admin"`, `name="admin"`, `status="online"`, `id="work"`} {
		if !strings.Contains(brief, want) {
			t.Fatalf("brief roster lost %s: %q", want, brief)
		}
	}
	t.Logf("ROSTER|full_bytes=%d|brief_bytes=%d", len(block), len(brief))
}

// TestFIX586BuildContextMessagesPrunesEnv is the WIRING regression test for
// FIX-586, updated to the FEATURE-622 semantics. FEATURE-579 plan A shipped
// pruneEnvContext and wired it into buildContextMessages, but FEATURE-583
// accidentally dropped that single call while inserting the <skills> injection:
// the trimming silently stopped running and every request kept the full
// envelope of every historical message. The older tests all call
// pruneEnvContext directly, so none of them noticed.
func TestFIX586BuildContextMessagesPrunesEnv(t *testing.T) {
	a := testAgent(t)
	a.messages = append([]llm.Message{{Role: "system", Content: "system prompt"}}, envMsgs(4, 1)...)

	before := make([]string, len(a.messages))
	for i := range a.messages {
		before[i] = envTextAt(&a.messages[i])
	}

	got := a.buildContextMessages()
	if len(got) != len(a.messages) {
		t.Fatalf("smart mode must keep every message: got %d, want %d", len(got), len(a.messages))
	}

	// The newest carrier keeps the complete envelope — it is the one envelope
	// the model acts on.
	newest := envTextAt(&got[len(got)-1])
	for _, want := range []string{"<current_dir>", "<tools>", "<task_plan>", "<cwd>"} {
		if !strings.Contains(newest, want) {
			t.Errorf("newest envelope must keep %s, got %q", want, newest)
		}
	}
	// Every older envelope keeps <time> + <message_no> only.
	for i := 1; i < len(got)-1; i++ {
		old := envTextAt(&got[i])
		if !strings.Contains(old, "<time>") || !strings.Contains(old, "<message_no>") {
			t.Errorf("envelope at index %d must keep time+message_no, got %q", i, old)
		}
		for _, unwanted := range trimmedTags {
			if strings.Contains(old, unwanted) {
				t.Errorf("envelope at index %d must drop %s, got %q", i, unwanted, old)
			}
		}
	}

	// The persisted history must keep every full envelope.
	for i := range a.messages {
		if envTextAt(&a.messages[i]) != before[i] {
			t.Errorf("persisted history changed at index %d", i)
		}
	}
	t.Logf("FIX586_WIRING|full=1|trimmed=%d|newest_bytes=%d|history_unchanged=true", len(got)-2, len(newest))
}

// TestFIX586BuildContextMessagesKeepsFullEnvWhenSwitchedOff is the switch-off
// half of the same guard: with the master switch off nothing may be trimmed,
// so the restored call must stay behind the config check.
func TestFIX586BuildContextMessagesKeepsFullEnvWhenSwitchedOff(t *testing.T) {
	a := testAgent(t)
	a.cfg.LLM.EnvHistoryPrune = false
	a.messages = append([]llm.Message{{Role: "system", Content: "system prompt"}}, envMsgs(4, 1)...)

	got := a.buildContextMessages()
	for i := 1; i < len(got); i++ {
		if !strings.Contains(envTextAt(&got[i]), "<current_dir>") {
			t.Errorf("switch off must keep envelope %d full, got %q", i, envTextAt(&got[i]))
		}
	}
	t.Logf("FIX586_WIRING_OFF|every_envelope_full=true")
}

// TestSplitEnvBlocksTagNames is the FEATURE-612 regression guard for the panel's
// environment blocks: a top-level element name must never carry its attributes
// (the panel renders the name as the block title), and a block must stop at its
// own closing tag instead of swallowing everything that follows.
func TestSplitEnvBlocksTagNames(t *testing.T) {
	env := "<environment_details>\n" +
		"<time>2026-10-07 00:00:00 Tuesday</time>\n" +
		"<board_agents rev=\"703\">\n" +
		"  <agent id=\"a1\" name=\"work\" status=\"online\"/>\n" +
		"</board_agents>\n" +
		"<sub_agents max=\"5\">\n" +
		"</sub_agents>\n" +
		"<tools>\nbin/tool.sh 2026-07-30 05:43:15 1.4KB\n</tools>\n" +
		"</environment_details>"

	want := map[string]string{
		"time":         "<time>2026-10-07 00:00:00 Tuesday</time>",
		"board_agents": "<board_agents rev=\"703\">\n  <agent id=\"a1\" name=\"work\" status=\"online\"/>\n</board_agents>",
		"sub_agents":   "<sub_agents max=\"5\">\n</sub_agents>",
		"tools":        "<tools>\nbin/tool.sh 2026-07-30 05:43:15 1.4KB\n</tools>",
	}

	blocks := splitEnvBlocks(env)
	if len(blocks) != len(want) {
		t.Fatalf("got %d blocks, want %d: %+v", len(blocks), len(want), blocks)
	}
	for _, b := range blocks {
		raw, ok := want[b.name]
		if !ok {
			t.Fatalf("unexpected block name %q (attributes must not be part of it)", b.name)
		}
		if b.raw != raw {
			t.Fatalf("block %q raw = %q, want %q", b.name, b.raw, raw)
		}
	}
}

// TestBuildMinimalEnvBlockKeepsOnlyTimeAndMessageNo guards the trimming helper.
func TestBuildMinimalEnvBlockKeepsOnlyTimeAndMessageNo(t *testing.T) {
	env := testEnv(9, true)
	min := buildMinimalEnvBlock(env)
	if !strings.Contains(min, "<time>2026-10-02 00:00:00 Friday</time>") || !strings.Contains(min, "<message_no>9</message_no>") {
		t.Fatalf("trimmed envelope must keep time+message_no, got: %q", min)
	}
	for _, unwanted := range trimmedTags {
		if strings.Contains(min, unwanted) {
			t.Fatalf("trimmed envelope must drop %s, got: %q", unwanted, min)
		}
	}
	if !strings.HasPrefix(min, "<environment_details>") || !strings.HasSuffix(min, "</environment_details>") {
		t.Fatalf("trimmed envelope must stay well formed, got: %q", min)
	}
	t.Logf("BUILD_MINIMAL|in_bytes=%d|out_bytes=%d", len(env), len(min))
}

// TestBoardRosterSelfContainedWithTrim guards the roster slimmer: with the
// history trim on the newest envelope must carry the brief roster every time
// (no <unchanged/> reference); with the trim off the placeholder comes back.
func TestBoardRosterSelfContainedWithTrim(t *testing.T) {
	withRoster := func(t *testing.T, trim bool) *Agent {
		t.Helper()
		a := testAgent(t)
		a.cfg.LLM.EnvHistoryPrune = trim
		a.cfg.LLM.EnvBoardRosterBrief = true
		a.cfg.BoardEnabled = true
		st := a.boardStateFor()
		st.mu.Lock()
		st.rev = 7
		st.agents = []BoardAgentInfo{{ID: "a1", Name: "work", Duty: "helper", Online: true}}
		st.mu.Unlock()
		return a
	}

	a := withRoster(t, true)
	first := a.boardRosterBlockSlim()
	second := a.boardRosterBlockSlim()
	for i, got := range []string{first, second} {
		if got == "" {
			t.Fatalf("trim on must render the roster every time (call %d)", i+1)
		}
		if strings.Contains(got, "<unchanged/>") {
			t.Fatalf("trim on must not emit <unchanged/> (call %d): %q", i+1, got)
		}
		if !strings.Contains(got, `name="work"`) {
			t.Fatalf("roster must list the agent (call %d): %q", i+1, got)
		}
	}

	b := withRoster(t, false)
	if got := b.boardRosterBlockSlim(); got == "" {
		t.Fatalf("trim off must render the roster on the first envelope")
	}
	if got := b.boardRosterBlockSlim(); !strings.Contains(got, "<unchanged/>") {
		t.Fatalf("trim off must collapse an unchanged roster to <unchanged/>, got: %q", got)
	}
	t.Logf("ROSTER_TRIM|on_bytes=%d|off_second_placeholder=true", len(second))
}
