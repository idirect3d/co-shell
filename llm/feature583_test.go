// Author: L.Shuang
// Created: 2026-10-02
//
// MIT License - Copyright (c) 2026 L.Shuang
//
// FEATURE-583 UC-0019: the pinned flag never reaches the LLM request body but
// survives the session JSON round trip.

package llm

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFeature583UC0019RequestStripsPinned(t *testing.T) {
	msgs := []Message{
		{
			Role:      "assistant",
			ToolCalls: []ToolCall{{ID: "call-1", Name: "skill_load", Arguments: `{"name":"demo"}`}},
			Pinned:    true,
			PinnedFor: "demo",
		},
		{
			Role:       "tool",
			ToolCallID: "call-1",
			Content:    "body",
			Pinned:     true,
			PinnedFor:  "demo",
		},
	}

	built := buildMessages(msgs)
	data, err := json.Marshal(built)
	if err != nil {
		t.Fatalf("marshal request messages: %v", err)
	}
	for _, forbidden := range []string{"pinned", "pinned_for"} {
		if strings.Contains(string(data), forbidden) {
			t.Errorf("request body leaks %q:\n%s", forbidden, data)
		}
	}
	// The declaration itself must still reach the model.
	if !strings.Contains(string(data), "skill_load") || !strings.Contains(string(data), "call-1") {
		t.Errorf("request body lost the tool call pair:\n%s", data)
	}

	// Session persistence keeps the flags.
	raw, err := json.Marshal(msgs)
	if err != nil {
		t.Fatalf("marshal messages: %v", err)
	}
	var back []Message
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal messages: %v", err)
	}
	if len(back) != 2 || !back[0].Pinned || back[0].PinnedFor != "demo" {
		t.Errorf("assistant pin lost in the round trip: %+v", back)
	}
	if !back[1].Pinned || back[1].PinnedFor != "demo" {
		t.Errorf("tool pin lost in the round trip: %+v", back[1])
	}
	// A message without pins stays clean on the wire (omitempty).
	plain, err := json.Marshal(Message{Role: "user", Content: "hi"})
	if err != nil {
		t.Fatalf("marshal plain message: %v", err)
	}
	if strings.Contains(string(plain), "pinned") {
		t.Errorf("plain message carries the pin fields: %s", plain)
	}
}
