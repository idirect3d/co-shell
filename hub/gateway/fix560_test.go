package gateway

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// FIX-560: hub 列表项红色删除按钮显示修复（默认隐藏 + 滑动 16px 渐入 + 左边延长 20px）。
// These tests cover the A group of use-case/FIX-560/FIX-560-UC-0001.md; the static
// page is checked as source text, the real rendering is verified by the B group.

// fix560DelRule returns the body of the ".agent-del {" CSS rule.
func fix560DelRule(t *testing.T, page string) string {
	t.Helper()
	start := strings.Index(page, ".agent-del {")
	if start < 0 {
		t.Fatal(".agent-del rule not found in webui_static.go")
	}
	rest := page[start:]
	end := strings.Index(rest, "}")
	if end < 0 {
		t.Fatal(".agent-del rule is not terminated by \"}\"")
	}
	return rest[:end]
}

// fix560CSSPx extracts the pixel value of a "<prop>:<n>px" declaration.
func fix560CSSPx(t *testing.T, rule, pattern, label string) int {
	t.Helper()
	m := regexp.MustCompile(pattern).FindStringSubmatch(rule)
	if m == nil {
		t.Fatalf("%s not found in the .agent-del rule:\n%s", label, rule)
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatalf("%s: bad pixel value %q: %v", label, m[1], err)
	}
	return n
}

// A-001/A-002: the button is invisible at rest and only one CSS rule defines it.
func TestFix560DeleteButtonHiddenAtRest(t *testing.T) {
	page := feat559ReadFile(t, feat559StaticPath)
	rule := fix560DelRule(t, page)
	if !strings.Contains(rule, "opacity:0") {
		t.Errorf("delete button is not hidden at rest (opacity:0 missing):\n%s", rule)
	}
	if !strings.Contains(rule, "transition:opacity") {
		t.Errorf("delete button has no opacity transition:\n%s", rule)
	}
	if n := strings.Count(page, ".agent-del {"); n != 1 {
		t.Errorf(".agent-del is defined %d times, want exactly 1 (a stale rule would override the fix)", n)
	}
}

// A-003: the button is 20px wider than before (64px + 20px) and the label stays
// centred in the revealed area via an equal left padding.
func TestFix560DeleteButtonLeftExtended(t *testing.T) {
	page := feat559ReadFile(t, feat559StaticPath)
	rule := fix560DelRule(t, page)
	width := fix560CSSPx(t, rule, `width:(\d+)px`, "width")
	if width < 84 {
		t.Errorf("delete button width = %dpx, want >= 84px (63px revealed + 20px slack)", width)
	}
	pad := fix560CSSPx(t, rule, `padding:0 0 0 (\d+)px`, "padding-left")
	if pad < 20 {
		t.Errorf("delete button padding-left = %dpx, want >= 20px to keep the label centred", pad)
	}
	if width-pad < 63 {
		t.Errorf("visible content width = %dpx, want >= 63px (the revealed area)", width-pad)
	}
}

// A-004: the fade completes within 16px of drag.
func TestFix560FadeSpanIs16px(t *testing.T) {
	page := feat559ReadFile(t, feat559StaticPath)
	if !strings.Contains(page, "var DELETE_FADE_PX = 16;") {
		t.Error("DELETE_FADE_PX = 16 not found")
	}
	if !strings.Contains(page, "setDelOpacity(card, -dx / DELETE_FADE_PX);") {
		t.Error("the drag handler does not scale the opacity by -dx / DELETE_FADE_PX")
	}
}

// A-005: opening/closing the swipe drives the button to fully opaque / hidden.
func TestFix560SwipeSetsOpacityEndpoints(t *testing.T) {
	page := feat559ReadFile(t, feat559StaticPath)
	if !strings.Contains(page, "setDelOpacity(card, open ? 1 : 0);") {
		t.Error("setSwipe does not pin the delete button opacity to 1 (open) / 0 (closed)")
	}
	if !strings.Contains(page, "function setDelOpacity(card, v){") {
		t.Error("setDelOpacity helper not found")
	}
	for _, want := range []string{"v <= 0 ? '0'", "v >= 1 ? '1'"} {
		if !strings.Contains(page, want) {
			t.Errorf("setDelOpacity does not clamp with %s", want)
		}
	}
	if !strings.Contains(page, "r._del = del;") {
		t.Error("renderList does not expose the delete button on the card (card._del)")
	}
}

// A-006: the drag follows the pointer (no transition) and a vertical-scroll
// abort restores the resting/open look instead of leaving a half-faded button.
func TestFix560DragFadeAndScrollAbort(t *testing.T) {
	page := feat559ReadFile(t, feat559StaticPath)
	if !strings.Contains(page, "if (card._del) card._del.style.transition = 'none';") {
		t.Error("begin() does not disable the opacity transition, so the fade would lag the drag")
	}
	if !strings.Contains(page, "setSwipe(card, !!card._open); // restore the resting/open look") {
		t.Error("the vertical-scroll abort does not restore the card state (stale red edge possible)")
	}
}

// A-007: the swipe behaviour itself is unchanged (threshold, clamp, snap).
func TestFix560SwipeBehaviourUnchanged(t *testing.T) {
	page := feat559ReadFile(t, feat559StaticPath)
	for _, want := range []string{
		"setSwipe(card, dx < -32);",
		"dx = Math.max(-63, Math.min(0, (card._open ? -63 : 0) + mx));",
		"card.style.transform = open ? 'translateX(-63px)' : 'translateX(0)';",
		"if (Math.abs(mx) > 4) card._moved = true;",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("swipe behaviour changed, missing: %s", want)
		}
	}
}

// A-008: co-shell and co-shell-hub share the same version and build number.
func TestFix560VersionAndBuildInSync(t *testing.T) {
	root := feat559ReadFile(t, filepath.Join("..", "..", "main.go"))
	hubMain := feat559ReadFile(t, filepath.Join("..", "..", "cmd", "co-shell-hub", "main.go"))
	match := func(src, pattern, label string) string {
		m := regexp.MustCompile(pattern).FindStringSubmatch(src)
		if m == nil {
			t.Fatalf("%s not found", label)
		}
		return m[1]
	}
	coreVer := match(root, `(?m)^const version = "([^"]+)"`, "version")
	coreBuild := match(root, `(?m)^const build = "([^"]+)"`, "build")
	hubVer := match(hubMain, `hubVersion = "([^"]+)"`, "hubVersion")
	hubBuild := match(hubMain, `hubBuild\s+= "([^"]+)"`, "hubBuild")

	if coreVer != hubVer {
		t.Errorf("version mismatch: co-shell %s != co-shell-hub %s", coreVer, hubVer)
	}
	if coreBuild != hubBuild {
		t.Errorf("build mismatch: co-shell %s != co-shell-hub %s", coreBuild, hubBuild)
	}
}
