//
// FEATURE-623: the dynamic-info panel keeps showing the tool list even while the
// envelope carries the compact <unchanged …/> placeholder, by caching the last
// fully rendered <tools> block.
//
// Author: L.Shuang
// MIT License - Copyright (c) 2026 L.Shuang
//

package web

import (
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/agent"
)

// TestApplyToolCacheKeepsToolListOnPlaceholder checks the FEATURE-623 panel
// behaviour: a real block refreshes the cache, a placeholder reuses it.
func TestApplyToolCacheKeepsToolListOnPlaceholder(t *testing.T) {
	s := &WebSession{}
	full := "<global>\n/g\n</global>\n<workspace>\n/w\nspace_tool.py 1 1B\n</workspace>"

	got := s.applyToolCache([]agent.EnvBlock{{Tag: "tools", Text: full}})
	if got[0].Text != full {
		t.Fatalf("a full block must pass through unchanged, got %q", got[0].Text)
	}

	got = s.applyToolCache([]agent.EnvBlock{{Tag: "tools", Text: "<unchanged since msg#3/>"}})
	if !strings.Contains(got[0].Text, "space_tool.py") {
		t.Fatalf("placeholder must be replaced by the cached list, got %q", got[0].Text)
	}

	// Other blocks are never touched.
	got = s.applyToolCache([]agent.EnvBlock{{Tag: "runtime_info", Text: "<runtime_info></runtime_info>"}})
	if !strings.Contains(got[0].Text, "runtime_info") {
		t.Fatalf("non-tools block must be left alone, got %q", got[0].Text)
	}
	t.Logf("PANEL_TOOLS|cached_bytes=%d", len(s.lastToolsBlock))
}
