// Author: L.Shuang
// Created: 2026-10-02
//
// # MIT License
//
// # Copyright (c) 2026 L.Shuang
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
package web

import (
	"strings"
	"testing"
)

// FIX-578: when an OLDER history page is prepended (reload + scroll up, session
// restore), every message-visualization line of that page must keep the page's
// chronological order. Inserting each line before the LIVE firstChild moved the
// page to the left but also reversed it: the page's newest (highest context
// usage) line ended up leftmost, so each reloaded page drew its trend upside
// down and the whole indicator looked like it bounced up and down instead of
// rising to the window limit and dropping back.
func TestFIX578MsgVizPrependKeepsChronologicalOrder(t *testing.T) {
	js := staticAsset(t, "/static/app.js")

	// 1. The line insertion uses the anchor captured at page start, not the live
	//    first child (the live one is what reversed the page).
	flush := between(js, "function msgVizFlush()", "\n// msgVizApplyPan")
	if flush == "" {
		t.Fatal("msgVizFlush not found in app.js")
	}
	if strings.Contains(flush, "insertBefore(line, msgVizTrack.firstChild)") {
		t.Fatal("msgVizFlush still inserts prepended lines before the LIVE firstChild: each page is reversed")
	}
	if !strings.Contains(flush, "insertBefore(line, msgVizPrependAnchor)") {
		t.Fatal("msgVizFlush does not insert prepended lines before the page anchor")
	}

	// 2. The anchor is captured once, when the older page starts, and cleared when
	//    the page's replay ends.
	page := between(js, "prependingHistory = true;", "prependingHistory = false;")
	if page == "" {
		t.Fatal("renderHistory prepend branch not found in app.js")
	}
	if !strings.Contains(page, "msgVizPrependAnchor = msgVizTrack ? msgVizTrack.firstChild : null;") {
		t.Fatal("renderHistory does not capture the msgviz prepend anchor before replaying the page")
	}
	if !strings.Contains(page, "msgVizPrependAnchor = null;") {
		t.Fatal("renderHistory does not clear the msgviz prepend anchor after the page")
	}

	// 3. A view reset (new connection / session switch) drops the anchor too, so a
	//    stale node from the previous view can never be used as insert point.
	reset := between(js, "function resetStreamView()", "function initTopSentinel")
	if reset == "" {
		t.Fatal("resetStreamView not found in app.js")
	}
	if !strings.Contains(reset, "msgVizPrependAnchor = null;") {
		t.Fatal("resetStreamView does not clear the msgviz prepend anchor")
	}
}
