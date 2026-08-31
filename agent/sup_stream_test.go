// Author: L.Shuang
// Created: 2026-08-31
//
// MIT License
//
// Copyright (c) 2026 L.Shuang

package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/llm"
)

// newSupTestAgent builds a minimal Agent with the SUP streaming switches set.
func newSupTestAgent(showPrompt, showStream bool) *Agent {
	ag := &Agent{}
	ag.SetConfig(&config.Config{
		LLM: config.LLMConfig{
			Supervisor: config.SupervisorConfig{
				ShowSupPrompt: showPrompt,
				ShowSupStream: showStream,
			},
		},
	})
	return ag
}

// supTitle returns the correct SUP block title per scenario.
func TestSupTitle(t *testing.T) {
	cases := []struct {
		scenario SupScenario
		want     string
	}{
		{SupScenarioProblemSolver, "SUP·问题解决"},
		{SupScenarioLoopJudge, "SUP·循环判定"},
		{SupScenarioSupervisor, "SUP·监督"},
		{SupScenario("unknown"), "SUP"},
	}
	for _, c := range cases {
		if got := c.scenario.supTitle(); got != c.want {
			t.Errorf("supTitle(%q) = %q, want %q", c.scenario, got, c.want)
		}
	}
}

// supPromptEnabled / supStreamEnabled respect the config switches.
func TestSupSwitches(t *testing.T) {
	on := newSupTestAgent(true, true)
	off := newSupTestAgent(false, false)
	nilCfg := &Agent{}

	if !on.supPromptEnabled(SupScenarioSupervisor) {
		t.Error("supPromptEnabled should be true when switch on")
	}
	if on.supStreamEnabled(SupScenarioSupervisor) != true {
		t.Error("supStreamEnabled should be true when switch on")
	}
	if off.supPromptEnabled(SupScenarioSupervisor) {
		t.Error("supPromptEnabled should be false when switch off")
	}
	if off.supStreamEnabled(SupScenarioSupervisor) {
		t.Error("supStreamEnabled should be false when switch off")
	}
	if nilCfg.supPromptEnabled(SupScenarioSupervisor) {
		t.Error("supPromptEnabled should be false with nil config")
	}
	if nilCfg.supStreamEnabled(SupScenarioSupervisor) {
		t.Error("supStreamEnabled should be false with nil config")
	}
}

// emitSupPrompt emits a content_chunk event on the supervisor channel with the
// scenario title in Meta when the switch is on; emits nothing when off.
func TestEmitSupPrompt(t *testing.T) {
	ag := newSupTestAgent(true, false)
	var got []StreamEvent
	ag.streamCb = func(ev StreamEvent) { got = append(got, ev) }

	ag.emitSupPrompt(SupScenarioProblemSolver, "hello prompt")
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}
	ev := got[0]
	if ev.Type != EventContentChunk {
		t.Errorf("expected content_chunk, got %s", ev.Type)
	}
	if ev.Chan != ChannelSupervisor {
		t.Errorf("expected supervisor channel, got %s", ev.Chan)
	}
	if ev.Text != "hello prompt" {
		t.Errorf("unexpected text %q", ev.Text)
	}
	if ev.Meta[MetaKeySupScenario] != "SUP·问题解决" {
		t.Errorf("unexpected sup_scenario meta %q", ev.Meta[MetaKeySupScenario])
	}

	// Switch off → no event.
	ag2 := newSupTestAgent(false, false)
	ag2.streamCb = func(ev StreamEvent) { got = append(got, ev) }
	before := len(got)
	ag2.emitSupPrompt(SupScenarioSupervisor, "ignored")
	if len(got) != before {
		t.Error("emitSupPrompt should emit nothing when switch off")
	}

	// Empty prompt → no event even when on.
	ag3 := newSupTestAgent(true, false)
	ag3.streamCb = func(ev StreamEvent) { got = append(got, ev) }
	before = len(got)
	ag3.emitSupPrompt(SupScenarioSupervisor, "")
	if len(got) != before {
		t.Error("emitSupPrompt should emit nothing for empty prompt")
	}
}

// streamSupReply accumulates content and tool calls, and forwards content
// chunks to the frontend when the stream switch is on.
func TestStreamSupReply(t *testing.T) {
	ag := newSupTestAgent(false, true)
	var got []StreamEvent
	ag.streamCb = func(ev StreamEvent) { got = append(got, ev) }

	eventCh := make(chan llm.StreamEvent, 8)
	eventCh <- llm.StreamEvent{Type: llm.StreamEventContent, Content: "part1"}
	eventCh <- llm.StreamEvent{Type: llm.StreamEventReasoning, Content: "think"}
	eventCh <- llm.StreamEvent{Type: llm.StreamEventContent, Content: "part2"}
	eventCh <- llm.StreamEvent{Type: llm.StreamEventToolCall, ToolCall: &llm.ToolCall{Name: "submit_review", Arguments: `{"approved":true}`}}
	eventCh <- llm.StreamEvent{Type: llm.StreamEventDone}
	close(eventCh)

	content, calls, err := ag.streamSupReply(context.Background(), SupScenarioSupervisor, eventCh)
	if err != nil {
		t.Fatalf("streamSupReply error: %v", err)
	}
	if content != "part1thinkpart2" {
		t.Errorf("unexpected accumulated content %q", content)
	}
	if len(calls) != 1 || calls[0].Name != "submit_review" {
		t.Errorf("unexpected tool calls %+v", calls)
	}
	// Content chunks forwarded (reasoning not forwarded).
	if len(got) != 2 {
		t.Fatalf("expected 2 forwarded content events, got %d", len(got))
	}
	for _, ev := range got {
		if ev.Chan != ChannelSupervisor {
			t.Errorf("expected supervisor channel, got %s", ev.Chan)
		}
		if ev.Meta[MetaKeySupScenario] != "SUP·监督" {
			t.Errorf("unexpected sup_scenario meta %q", ev.Meta[MetaKeySupScenario])
		}
	}
	if got[0].Text != "part1" || got[1].Text != "part2" {
		t.Errorf("unexpected forwarded texts %q / %q", got[0].Text, got[1].Text)
	}
}

// streamSupReply does not forward content when the stream switch is off, but
// still accumulates the full content for structured parsing.
func TestStreamSupReply_NoForward(t *testing.T) {
	ag := newSupTestAgent(false, false)
	var got []StreamEvent
	ag.streamCb = func(ev StreamEvent) { got = append(got, ev) }

	eventCh := make(chan llm.StreamEvent, 4)
	eventCh <- llm.StreamEvent{Type: llm.StreamEventContent, Content: "full"}
	eventCh <- llm.StreamEvent{Type: llm.StreamEventDone}
	close(eventCh)

	content, _, err := ag.streamSupReply(context.Background(), SupScenarioProblemSolver, eventCh)
	if err != nil {
		t.Fatalf("streamSupReply error: %v", err)
	}
	if content != "full" {
		t.Errorf("unexpected content %q", content)
	}
	if len(got) != 0 {
		t.Errorf("expected no forwarded events when switch off, got %d", len(got))
	}
}

// streamSupReply propagates a stream error.
func TestStreamSupReply_Error(t *testing.T) {
	ag := newSupTestAgent(false, false)
	eventCh := make(chan llm.StreamEvent, 2)
	eventCh <- llm.StreamEvent{Type: llm.StreamEventError, Err: context.DeadlineExceeded}
	close(eventCh)

	_, _, err := ag.streamSupReply(context.Background(), SupScenarioSupervisor, eventCh)
	if err == nil || !strings.Contains(err.Error(), "deadline") {
		t.Errorf("expected deadline error, got %v", err)
	}
}
