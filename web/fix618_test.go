package web

import (
	"os"
	"strings"
	"testing"
)

// FIX-618: interaction_request / interaction_resolved are control events
// (ChannelSystem + empty text) that handleInteractionEvent already renders as a
// dialog. They used to fall through to the generic block at the bottom of
// renderEvent, which painted an empty "SYS" block for each — so one interaction
// (request + resolved) left TWO empty SYS blocks at the end of the stream.
// (FIX-615 filtered pushText's ui_text instead, but that always travels on
// ChannelREPL and renders as a REPL block, so it never was the culprit.)
func TestFix618NoEmptySysBlocks(t *testing.T) {
	data, err := os.ReadFile("static/app.js")
	if err != nil {
		t.Fatalf("read static/app.js: %v", err)
	}
	js := string(data)

	// A-001 (fix A): the two interaction lifecycle events are ignored explicitly.
	want := `if (ev.type === "interaction_request" || ev.type === "interaction_resolved") return;`
	if !strings.Contains(js, want) {
		t.Error("renderEvent does not skip the interaction control events")
	}

	// A-002 (fix B): the generic branch guards on empty text BEFORE makeBlock.
	guard := "if (!text) return;"
	gi := strings.Index(js, guard)
	if gi < 0 {
		t.Fatal("the generic branch has no empty-text guard")
	}
	mk := "makeBlock(eventClass(ev), blockLabel, msgIndex)"
	mi := strings.Index(js, mk)
	if mi < 0 {
		t.Fatal("the generic block creation call is gone")
	}
	if gi > mi {
		t.Error("the empty-text guard sits after makeBlock, so empty blocks are still painted")
	}
	if !strings.Contains(js, `const text = ev.text || "";`) {
		t.Error("the generic branch does not use a local text variable")
	}
}
