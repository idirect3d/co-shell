// Tests for FEATURE-579 plans A/B/C: historical env degradation, single-block
// tool batches and the env slim-down helpers.
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
	cfg.LLM.EnvHistoryFullKeep = 1
	cfg.LLM.EnvHistoryLiteKeep = 1
	return &Agent{cfg: cfg}
}

// TestEnvPruneLevels checks the default 1/1 split: L2, L2, L1, L0 (old → new).
func TestEnvPruneLevels(t *testing.T) {
	a := testAgent(t)
	got := a.pruneEnvContext(envMsgs(4, 1))

	last := envTextAt(&got[3])
	if !strings.Contains(last, "<current_dir>") {
		t.Fatalf("newest message must stay L0, got: %q", last)
	}
	l1 := envTextAt(&got[2])
	if !strings.Contains(l1, "<cwd>") || !strings.Contains(l1, "<task_plan>") {
		t.Fatalf("dist=1 must be L1 (cwd + task_plan kept), got: %q", l1)
	}
	if strings.Contains(l1, "<current_dir>") || strings.Contains(l1, "<tools>") ||
		strings.Contains(l1, "<research>") || strings.Contains(l1, "<board_agents>") {
		t.Fatalf("L1 must drop current_dir/tools/research/board_agents, got: %q", l1)
	}
	for _, i := range []int{0, 1} {
		l2 := envTextAt(&got[i])
		if !strings.Contains(l2, "<time>") || !strings.Contains(l2, "<message_no>") {
			t.Fatalf("dist>=2 must keep time+message_no, got: %q", l2)
		}
		if strings.Contains(l2, "<cwd>") || strings.Contains(l2, "<task_plan>") || strings.Contains(l2, "<context_window>") {
			t.Fatalf("L2 must keep only time+message_no, got: %q", l2)
		}
	}
	t.Logf("LEVELS|L2,L2,L1,L0|bytes=%d,%d,%d,%d",
		len(envTextAt(&got[0])), len(envTextAt(&got[1])), len(envTextAt(&got[2])), len(envTextAt(&got[3])))
}

// TestEnvPruneEdgeCases covers the 0 = unlimited semantics.
func TestEnvPruneEdgeCases(t *testing.T) {
	// full_keep = 0 → every message stays L0.
	a := testAgent(t)
	a.cfg.LLM.EnvHistoryFullKeep = 0
	got := a.pruneEnvContext(envMsgs(3, 1))
	for i := range got {
		if !strings.Contains(envTextAt(&got[i]), "<current_dir>") {
			t.Fatalf("full_keep=0 must keep every message at L0, index %d: %q", i, envTextAt(&got[i]))
		}
	}
	t.Logf("FULL_KEEP_0|levels=L0,L0,L0|bytes=%d", len(envTextAt(&got[0])))

	// lite_keep = 0 → no L2 level at all.
	b := testAgent(t)
	b.cfg.LLM.EnvHistoryLiteKeep = 0
	gotB := b.pruneEnvContext(envMsgs(4, 1))
	for i := 0; i < len(gotB)-1; i++ {
		env := envTextAt(&gotB[i])
		if !strings.Contains(env, "<cwd>") {
			t.Fatalf("lite_keep=0: non-newest messages must be L1, index %d: %q", i, env)
		}
	}
	t.Logf("LITE_KEEP_0|levels=L1,L1,L1,L0|bytes=%d", len(envTextAt(&gotB[0])))
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

// TestEnvPruneTaskPlanFallback keeps <task_plan> visible when the newest
// message carries none.
func TestEnvPruneTaskPlanFallback(t *testing.T) {
	a := testAgent(t)
	// Only the oldest message carries a task plan; the newest two do not
	// (the skipTaskPlan case for track_task_progress/view_task_plan).
	msgs := envMsgs(3, 1)
	for _, i := range []int{1, 2} {
		msgs[i] = llm.Message{Role: "user", ContentParts: []llm.ContentPart{
			{Type: llm.ContentPartText, Text: testEnv(i+1, false)},
		}}
	}
	got := a.pruneEnvContext(msgs)
	oldest := envTextAt(&got[0])
	if !strings.Contains(oldest, "<task_plan>") {
		t.Fatalf("task plan fallback failed, oldest: %q", oldest)
	}
	if !strings.Contains(oldest, "<cwd>") {
		t.Fatalf("fallback must keep the message at least at L1, got: %q", oldest)
	}
	t.Logf("TASKPLAN_VISIBLE|index=0|level>=L1")
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
	t.Logf("DISABLED|unchanged=true|levels=L0,L0,L0,L0")
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

// TestEnvStaticBlocksChangeDetect covers plan C-2.
func TestEnvStaticBlocksChangeDetect(t *testing.T) {
	a := testAgent(t)
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
	t.Logf("DIRDETECT|full=%d|placeholder=%d|changed=%d", len(full), len(same), len(changed))
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
// FIX-586. FEATURE-579 plan A shipped pruneEnvContext and wired it into
// buildContextMessages, but FEATURE-583 accidentally dropped that single call
// while inserting the <skills> injection: the degradation silently stopped
// running and every request kept the full envelope of every historical message.
//
// The older tests all call pruneEnvContext directly, so none of them noticed.
// This one exercises the real request path (buildContextMessages) and therefore
// fails as soon as the call is removed again.
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

	// dist 0 (newest) must stay L0 — it is the envelope the model acts on.
	newest := envTextAt(&got[len(got)-1])
	if !strings.Contains(newest, "<current_dir>") {
		t.Errorf("newest envelope must stay L0 (current_dir kept), got %q", newest)
	}
	// dist 1 must be L1: volatile context kept, static listings dropped.
	l1 := envTextAt(&got[len(got)-2])
	if !strings.Contains(l1, "<cwd>") || !strings.Contains(l1, "<task_plan>") {
		t.Errorf("dist=1 envelope must be L1 (cwd + task_plan kept), got %q", l1)
	}
	if strings.Contains(l1, "<current_dir>") || strings.Contains(l1, "<tools>") {
		t.Errorf("dist=1 envelope must drop the static listings, got %q", l1)
	}
	// dist >= 2 must be L2: <time> + <message_no> only.
	for i := 1; i < len(got)-2; i++ {
		l2 := envTextAt(&got[i])
		if !strings.Contains(l2, "<time>") || !strings.Contains(l2, "<message_no>") {
			t.Errorf("envelope at index %d must be L2, got %q", i, l2)
		}
		if strings.Contains(l2, "<cwd>") || strings.Contains(l2, "<current_dir>") {
			t.Errorf("envelope at index %d must keep only time+message_no, got %q", i, l2)
		}
	}

	// The persisted history must keep every full envelope.
	for i := range a.messages {
		if envTextAt(&a.messages[i]) != before[i] {
			t.Errorf("persisted history changed at index %d", i)
		}
	}
	t.Logf("FIX586_WIRING|levels=L2,L2,L1,L0|newest_bytes=%d|history_unchanged=true", len(newest))
}

// TestFIX586BuildContextMessagesKeepsFullEnvWhenSwitchedOff is the switch-off
// half of the same guard: with the master switch off nothing may be degraded,
// so the restored call must stay behind the config check.
func TestFIX586BuildContextMessagesKeepsFullEnvWhenSwitchedOff(t *testing.T) {
	a := testAgent(t)
	a.cfg.LLM.EnvHistoryPrune = false
	a.messages = append([]llm.Message{{Role: "system", Content: "system prompt"}}, envMsgs(4, 1)...)

	got := a.buildContextMessages()
	for i := 1; i < len(got); i++ {
		if !strings.Contains(envTextAt(&got[i]), "<current_dir>") {
			t.Errorf("switch off must keep envelope %d at L0, got %q", i, envTextAt(&got[i]))
		}
	}
	t.Logf("FIX586_WIRING_OFF|levels=L0,L0,L0,L0")
}
