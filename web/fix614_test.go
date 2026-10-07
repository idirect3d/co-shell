package web

import (
	"os"
	"strings"
	"testing"
)

// FIX-614: a failed tool call is marked (level-error) only when its result event
// arrives — later than the msgVizFlush() that painted its indicator line in the
// plain tool colour. The line therefore stayed yellow until the page was
// reloaded (a history replay paints the block and its error in one pass). These
// assertions pin the fix: a recolor helper, the ratio it needs for the tooltip,
// the tool-result call site, and the live-class reads that make both work.
func TestFix614MsgVizRecolor(t *testing.T) {
	data, err := os.ReadFile("static/app.js")
	if err != nil {
		t.Fatalf("read static/app.js: %v", err)
	}
	js := string(data)

	// A-001: the helper exists and repaints colour + tooltip for the line whose
	// block changed class after it was drawn.
	if !strings.Contains(js, "function msgVizRecolor(box)") {
		t.Fatal("app.js has no msgVizRecolor")
	}
	for _, want := range []string{
		`const line = Array.from(msgVizTrack.children).find((l) => l._box === box);`,
		`line.style.setProperty("--line-color", msgVizColor(line._cls, box));`,
		`line.title = msgVizTip(line._cls, box, line._ratio || 0);`,
	} {
		if !strings.Contains(js, want) {
			t.Errorf("msgVizRecolor is missing %q", want)
		}
	}

	// A-002: flush stores the ratio so the later repaint can rebuild the tooltip.
	if !strings.Contains(js, "line._ratio = ratio;") {
		t.Error("msgVizFlush does not store _ratio on the line")
	}

	// A-003: the tool-result failure path repaints the line it just marked.
	if !strings.Contains(js, `errBox.classList.add("level-error");`) ||
		!strings.Contains(js, "msgVizRecolor(errBox);") {
		t.Error("the tool error path does not repaint the indicator line")
	}

	// A-004: msgVizColor still ranks level-error above the plain tool colour.
	if !strings.Contains(js, `if (/level-error/.test(cls) || /level-error/.test(bcls)) return "#FF0000";`) {
		t.Error("msgVizColor no longer returns pure red for level-error")
	}

	// A-005: the tooltip type must read live classes too, otherwise a red line
	// would still be labelled "Tool".
	if !strings.Contains(js, "const hasLevel = (re) => re.test(cls) || re.test(bcls);") {
		t.Error("msgVizTip does not read the live classes")
	}
}
