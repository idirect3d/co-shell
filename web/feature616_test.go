package web

import (
	"os"
	"strings"
	"testing"
)

// FEATURE-616: hovering a message-indicator line (the message-visualization
// chart, #msgVizTrack) jumps the message area (#streamB) to the block behind
// that line, leaving the block's title bar (.ev-head) at the top of the
// scroller. These assertions pin the hover-to-locate wiring and, importantly,
// the offset maths: .stream-b is position:static, so a block's offsetTop is
// measured against #stream-wrap (the 35px .stream-head and the .stream padding
// included) and would land the title too low — the fix must use the client-rect
// delta instead.
func TestFeature616MsgVizHoverLocate(t *testing.T) {
	data, err := os.ReadFile("static/app.js")
	if err != nil {
		t.Fatalf("read static/app.js: %v", err)
	}
	js := string(data)

	// A-001: the locate helper exists and aligns via the two client rects.
	if !strings.Contains(js, "function msgVizLocate(line) {") {
		t.Fatal("app.js has no msgVizLocate")
	}
	if !strings.Contains(js, "box.getBoundingClientRect().top - streamB.getBoundingClientRect().top") {
		t.Error("msgVizLocate does not align via the client-rect delta")
	}
	// The offsetTop form would be taken against #stream-wrap (see the comment
	// above), so it must not be used for the jump.
	if strings.Contains(js, "streamB.scrollTop = box.offsetTop") {
		t.Error("msgVizLocate uses box.offsetTop, which is off by the .stream-head height")
	}

	// A-002: the delta is applied incrementally to the scroller.
	if !strings.Contains(js, "streamB.scrollTop += ") {
		t.Error("msgVizLocate does not apply the delta to streamB.scrollTop")
	}

	// A-003: the track delegates a mouseover to locate the hovered line.
	if !strings.Contains(js, `msgVizTrack.addEventListener("mouseover"`) {
		t.Error("the msgviz track has no mouseover listener")
	}
	if !strings.Contains(js, `closest(".msgviz-line")`) {
		t.Error("the mouseover handler does not resolve the hovered line")
	}
	if !strings.Contains(js, "if (line) msgVizLocate(line);") {
		t.Error("the mouseover handler does not call msgVizLocate")
	}

	// A-004: panning must not trigger the hover jump.
	if !strings.Contains(js, "if (dragging) return;\n    const line = e.target") {
		t.Error("the mouseover handler is not guarded while dragging")
	}

	// A-005: a line whose block is gone is ignored, so a hover never throws.
	if !strings.Contains(js, "if (!box || !box.isConnected) return;") {
		t.Error("msgVizLocate does not guard against a missing/detached block")
	}

	// A-006 (regression): click-to-locate and drag-to-pan stay in place.
	if !strings.Contains(js, `line.addEventListener("click"`) {
		t.Error("the existing click-to-locate handler is gone")
	}
	if !strings.Contains(js, `msgViz.addEventListener("pointerdown"`) ||
		!strings.Contains(js, `msgViz.addEventListener("pointermove"`) {
		t.Error("the existing drag-to-pan handlers are gone")
	}
}
