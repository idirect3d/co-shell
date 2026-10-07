// FEATURE-612: the dynamic-info panel shows the board roster even while the
// envelope carries the compact <unchanged/> form.
//
// Author: L.Shuang
// Created: 2026-10-07
// MIT License - Copyright (c) 2026 L.Shuang

package web

import (
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/agent"
)

// TestApplyBoardRoster checks that the compact board block is swapped for the
// current roster and that every other block survives untouched.
func TestApplyBoardRoster(t *testing.T) {
	const (
		compact = "<board_agents rev=\"7\">\n<unchanged/>\n</board_agents>"
		full    = "<board_agents rev=\"7\">\n  <agent id=\"a1\" name=\"work\" status=\"online\"/>\n</board_agents>"
		timeTag = "<time>2026-10-07 00:00:00 Tuesday</time>"
	)

	blocks := []agent.EnvBlock{
		{Tag: "time", Text: timeTag},
		{Tag: "board_agents", Text: compact},
	}
	got := applyBoardRoster(blocks, full)
	if got[1].Text != full {
		t.Fatalf("board block = %q, want the current roster", got[1].Text)
	}
	if got[0].Text != timeTag {
		t.Fatalf("time block = %q, want it untouched", got[0].Text)
	}

	// Without a roster the compact form is kept, so the panel can fall back to
	// what it parsed earlier.
	keep := applyBoardRoster([]agent.EnvBlock{{Tag: "board_agents", Text: compact}}, "")
	if !strings.Contains(keep[0].Text, "<unchanged") {
		t.Fatalf("compact form must survive an empty roster, got %q", keep[0].Text)
	}

	// A detailed block is never rewritten.
	detail := applyBoardRoster([]agent.EnvBlock{{Tag: "board_agents", Text: full}}, "")
	if detail[0].Text != full {
		t.Fatalf("detailed block = %q, want it untouched", detail[0].Text)
	}
}
