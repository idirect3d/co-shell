// Package agent - FEATURE-611 tests: dynamic-event retention (unacked replay +
// meta.ack acknowledgement).
//
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"regexp"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
)

// feature611Agent builds an agent carrying the FEATURE-611 defaults
// (dynamic_event_retain_max = 100, user_message_mode = dynamic).
func feature611Agent() *Agent {
	return &Agent{cfg: config.DefaultConfig()}
}

// UC-35/UC-37 (config): the new settings exist with their documented defaults.
func TestFeature611ConfigDefaults(t *testing.T) {
	cfg := config.DefaultConfig()
	if cfg.DynamicEventRetainMax != 100 {
		t.Errorf("DynamicEventRetainMax = %d, want 100", cfg.DynamicEventRetainMax)
	}
	if cfg.UserMessageMode != UserMessageModeDynamic {
		t.Errorf("UserMessageMode = %q, want %q", cfg.UserMessageMode, UserMessageModeDynamic)
	}
}

// UC-01/UC-03: every enqueued event gets a unique id and a replay keeps it.
func TestFeature611EventIDsUniqueAndStable(t *testing.T) {
	a := feature611Agent()
	a.AddDynamicEvent(DynamicUserMessage, "first")
	a.AddDynamicEvent(DynamicUserMessage, "second")

	first := a.consumeDynamicEvents(true)
	for _, want := range []string{`ack_id="ev-1"`, `ack_id="ev-2"`} {
		if !strings.Contains(first, want) {
			t.Fatalf("first injection missing %s:\n%s", want, first)
		}
	}
	second := a.consumeDynamicEvents(true)
	if !strings.Contains(second, `ack_id="ev-1"`) || !strings.Contains(second, `ack_id="ev-2"`) {
		t.Errorf("replay must keep the original ids:\n%s", second)
	}
}

// UC-05/UC-06/UC-07: a retainable event is replayed with the unacked marker and
// its original timestamp until it is acknowledged.
func TestFeature611UnackedReplayMarkerAndTimestamp(t *testing.T) {
	a := feature611Agent()
	a.AddDynamicEvent(DynamicUserMessage, "补充消息")

	first := a.consumeDynamicEvents(true)
	if strings.Contains(first, "unacked") {
		t.Errorf("first injection must not be marked unacked:\n%s", first)
	}
	second := a.consumeDynamicEvents(true)
	for _, want := range []string{"[unacked]", `unacked="true"`} {
		if !strings.Contains(second, want) {
			t.Fatalf("replay missing %s:\n%s", want, second)
		}
	}

	// The replay keeps the event's original timestamp.
	ts := regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}`)
	t1 := ts.FindString(first)
	t2 := ts.FindString(second)
	if t1 == "" || t1 != t2 {
		t.Errorf("replay timestamp %q != original %q (first:\n%s\nsecond:\n%s)", t2, t1, first, second)
	}
}

// UC-11/UC-12/UC-16/UC-17: acknowledgement is precise and idempotent.
func TestFeature611AckStopsReplayPrecisely(t *testing.T) {
	a := feature611Agent()
	a.AddDynamicEvent(DynamicUserMessage, "one")
	a.AddDynamicEvent(DynamicUserMessage, "two")
	a.consumeDynamicEvents(true)

	if n := a.AckDynamicEvents([]string{"ev-1"}); n != 1 {
		t.Fatalf("ack count = %d, want 1", n)
	}
	block := a.consumeDynamicEvents(true)
	if strings.Contains(block, `ev-1"`) {
		t.Errorf("acknowledged event must not be replayed:\n%s", block)
	}
	if !strings.Contains(block, `ack_id="ev-2"`) {
		t.Errorf("the other unacked event must still be replayed:\n%s", block)
	}
	// Idempotent: re-acking the same id (or an unknown one) is a no-op.
	if n := a.AckDynamicEvents([]string{"ev-1", "ev-999"}); n != 0 {
		t.Errorf("repeat/unknown ack count = %d, want 0", n)
	}
}

// UC-18: the retained set is capped by dropping its oldest entries.
func TestFeature611RetainCapDropsOldest(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DynamicEventRetainMax = 2
	a := &Agent{cfg: cfg}
	a.AddDynamicEvent(DynamicUserMessage, "m1")
	a.AddDynamicEvent(DynamicUserMessage, "m2")
	a.AddDynamicEvent(DynamicUserMessage, "m3")
	a.consumeDynamicEvents(true)

	block := a.consumeDynamicEvents(true)
	if strings.Contains(block, "m1") {
		t.Errorf("oldest retained event must be dropped:\n%s", block)
	}
	for _, want := range []string{"m2", "m3"} {
		if !strings.Contains(block, want) {
			t.Errorf("replay missing %s:\n%s", want, block)
		}
	}
}

// UC-19/UC-20: retain max 0 disables replay (one-shot), negative values too.
func TestFeature611RetainDisabledIsOneShot(t *testing.T) {
	for _, max := range []int{0, -3} {
		cfg := config.DefaultConfig()
		cfg.DynamicEventRetainMax = max
		a := &Agent{cfg: cfg}
		a.AddDynamicEvent(DynamicUserMessage, "once")
		if first := a.consumeDynamicEvents(true); !strings.Contains(first, "once") {
			t.Fatalf("max=%d: first injection missing the event:\n%s", max, first)
		}
		if second := a.consumeDynamicEvents(true); second != "" {
			t.Errorf("max=%d: one-shot must not replay, got:\n%s", max, second)
		}
	}
}

// UC-09: transient path events stay one-shot and carry no ack id.
func TestFeature611TransientEventsNotReplayed(t *testing.T) {
	a := feature611Agent()
	a.AddDynamicEvent(DynamicClipObject, "input/a.png")
	first := a.consumeDynamicEvents(false)
	if !strings.Contains(first, "<clip_object>") {
		t.Fatalf("clip event missing:\n%s", first)
	}
	if strings.Contains(first, "ack_id") {
		t.Errorf("transient events must not carry ack_id:\n%s", first)
	}
	if second := a.consumeDynamicEvents(false); second != "" {
		t.Errorf("transient events must not be replayed, got:\n%s", second)
	}
}

// UC-10 / UC-13 / UC-14: board and sub-agent notices are replayed too, and the
// optional meta.ack field is parsed without becoming a misplaced parameter.
func TestFeature611BoardEventReplayAndAckParsing(t *testing.T) {
	a := feature611Agent()
	a.AddDynamicEvent(DynamicBoardMessage, `<board_message id="m-1" from="agent-a">hi</board_message>`)

	first := a.consumeDynamicEvents(true)
	if !strings.Contains(first, `ack_id="ev-1"`) {
		t.Fatalf("board event must carry an ack id:\n%s", first)
	}
	if strings.Contains(first, "unacked") {
		t.Errorf("first injection must not be marked unacked:\n%s", first)
	}
	second := a.consumeDynamicEvents(true)
	if !strings.Contains(second, `unacked="true"`) || !strings.Contains(second, "[unacked]") {
		t.Errorf("board event must be replayed as unacked:\n%s", second)
	}
	if !strings.Contains(second, `<board_message id="m-1" from="agent-a"`) {
		t.Errorf("replay must preserve the rendered tag:\n%s", second)
	}
}

// UC-13: parseMetaAck reads the optional ack array (and tolerates a bare string
// or a malformed value) without failing the call.
func TestFeature611ParseMetaAck(t *testing.T) {
	cases := []struct {
		name string
		args map[string]interface{}
		want int
	}{
		{"absent", map[string]interface{}{"meta": map[string]interface{}{"intent": "x"}}, 0},
		{"array", map[string]interface{}{"meta": map[string]interface{}{"ack": []interface{}{"ev-1", "ev-2"}}}, 2},
		{"array of strings", map[string]interface{}{"meta": map[string]interface{}{"ack": []string{"ev-1"}}}, 1},
		{"bare string", map[string]interface{}{"meta": map[string]interface{}{"ack": "ev-3"}}, 1},
		{"empty array", map[string]interface{}{"meta": map[string]interface{}{"ack": []interface{}{}}}, 0},
		{"malformed", map[string]interface{}{"meta": map[string]interface{}{"ack": 42}}, 0},
		{"no meta", map[string]interface{}{"path": "/x"}, 0},
	}
	for _, tc := range cases {
		if got := parseMetaAck(tc.args); len(got) != tc.want {
			t.Errorf("%s: parseMetaAck = %v, want %d entries", tc.name, got, tc.want)
		}
	}
}

// UC-14: meta.ack is a legitimate meta field, so it is never promoted to the
// top level by promoteMisplacedMetaParams.
func TestFeature611AckNotPromoted(t *testing.T) {
	args := map[string]interface{}{
		"meta": map[string]interface{}{
			"ack":    []interface{}{"ev-1"},
			"intent": "x",
		},
	}
	promoteMisplacedMetaParams(args)
	if _, ok := args["ack"]; ok {
		t.Errorf("meta.ack must not be promoted to the top level: %v", args)
	}
	meta, _ := args["meta"].(map[string]interface{})
	if meta["ack"] == nil {
		t.Errorf("meta.ack must stay inside meta: %v", args)
	}
}

// UC-02/UC-06: attributes are injected into an already-rendered tag without
// breaking container or self-closing forms.
func TestFeature611InjectTagAttrs(t *testing.T) {
	got := injectTagAttrs(`<sub_agent_done id="A1" status="completed">body</sub_agent_done>`, ` ack_id="ev-9"`, "[unacked] ")
	want := `<sub_agent_done id="A1" status="completed" ack_id="ev-9">[unacked] body</sub_agent_done>`
	if got != want {
		t.Errorf("container tag:\n got %q\nwant %q", got, want)
	}
	got = injectTagAttrs(`<board_timeout message_id="m-1"/>`, ` ack_id="ev-8"`, "")
	want = `<board_timeout message_id="m-1" ack_id="ev-8"/>`
	if got != want {
		t.Errorf("self-closing tag:\n got %q\nwant %q", got, want)
	}
	// A non-tag text keeps the attributes appended instead of losing them.
	if got := injectTagAttrs("plain", ` ack_id="ev-7"`, ""); !strings.Contains(got, "ack_id") {
		t.Errorf("non-tag text must not drop the attrs: %q", got)
	}
}

// UC-21/UC-22/UC-40: the user-message mode accessor and its defaults.
func TestFeature611UserMessageModeAccessor(t *testing.T) {
	if got := feature611Agent().UserMessageMode(); got != UserMessageModeDynamic {
		t.Errorf("default UserMessageMode = %q, want %q", got, UserMessageModeDynamic)
	}
	cfg := config.DefaultConfig()
	cfg.UserMessageMode = UserMessageModeFull
	if got := (&Agent{cfg: cfg}).UserMessageMode(); got != UserMessageModeFull {
		t.Errorf("UserMessageMode = %q, want %q", got, UserMessageModeFull)
	}
	if got := (&Agent{}).UserMessageMode(); got != UserMessageModeDynamic {
		t.Errorf("nil cfg must default to dynamic, got %q", got)
	}
}

// UC-25: a replay survives the pruning of the message that first carried the
// event, because an unacked event is re-emitted in the newest envelope.
func TestFeature611ReplayIndependentOfPruning(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.LLM.EnvHistoryPrune = true
	cfg.LLM.EnvHistoryFullKeep = 1
	cfg.LLM.EnvHistoryLiteKeep = 0
	a := &Agent{cfg: cfg}

	a.AddDynamicEvent(DynamicUserMessage, "关键补充")
	first := a.consumeDynamicEvents(true)

	// The first envelope is now an old message; degrade it like the history
	// prune does and make sure the lite level drops the dynamic block.
	lite := buildEnvBlock("<environment_details>\n<time>t</time>\n"+strings.TrimSpace(first)+"\n</environment_details>", envLevelLite)
	if strings.Contains(lite, "user_dynamic_events") {
		t.Fatalf("lite level is expected to drop the dynamic block:\n%s", lite)
	}

	// The event is still delivered: it is replayed in the newest envelope.
	second := a.consumeDynamicEvents(true)
	if !strings.Contains(second, "关键补充") {
		t.Errorf("unacked event must survive the pruning of its original message:\n%s", second)
	}
}
