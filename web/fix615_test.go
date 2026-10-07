package web

import (
	"os"
	"strings"
	"testing"
)

// FIX-615: the agent prints bare blank lines for terminal readability
// (io.Println() with no arguments), which reach the Web UI through WebIO as
// empty ui_text events — the front end paints each of those as an empty "SYS"
// block at the end of a reloaded session. pushText must therefore drop
// whitespace-only text instead of pushing it.
func TestFix615PushTextDropsBlankText(t *testing.T) {
	data, err := os.ReadFile("session.go")
	if err != nil {
		t.Fatalf("read session.go: %v", err)
	}
	src := string(data)

	const sig = "func (w *WebIO) pushText(text string) {"
	i := strings.Index(src, sig)
	if i < 0 {
		t.Fatal("pushText not found in session.go")
	}
	body := src[i:]
	if end := strings.Index(body, "\n}"); end > 0 {
		body = body[:end]
	}

	if !strings.Contains(body, `if strings.TrimSpace(text) == "" {`) {
		t.Error("pushText does not filter whitespace-only text")
	}
	if !strings.Contains(body, "return") {
		t.Error("pushText does not return early for blank text")
	}
	// The real event must still be sent for non-blank text.
	if !strings.Contains(body, `w.srv.sendEvent(agent.NewStreamEvent("ui_text", agent.ChannelREPL, agent.LevelInfo, text))`) {
		t.Error("pushText no longer sends the ui_text event")
	}
}
