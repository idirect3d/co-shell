// Package agent - FEATURE-579 (D1) tool declaration compaction test.
//
// It builds the OpenAI tool declarations twice, once with
// LLM.ToolDescCompact=true and once false, and verifies that:
//   - compact mode makes every meta parameter description a single line equal
//     to the toolMetaDescCompact constant;
//   - full mode keeps the original long wording byte-for-byte;
//   - the compact schema is strictly smaller than the full one.
//
// Evidence lines (deterministic):
//
//	META|compact=<bool>|count=<n>|bytes=<n>
//	SCHEMA|compact=<bool>|total=<n>
//
// Run with: go test ./agent -run TestToolDescCompactSchema -v -count=1
//
// Author: L.Shuang
// Created: 2026-10-02
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/mcp"
)

// compactWireTool mirrors the JSON shape the OpenAI client sends for tool
// declarations (same as token_baseline_test.go), so measured bytes match the
// real request payload.
type compactWireTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string      `json:"name"`
		Description string      `json:"description"`
		Parameters  interface{} `json:"parameters"`
	} `json:"function"`
}

// compactSchemaAgent builds the minimal agent needed by ToolDeclarations().
func compactSchemaAgent(compact bool) *Agent {
	cfg := config.DefaultConfig()
	cfg.LLM.ToolDescCompact = compact
	return &Agent{
		cfg:                   cfg,
		toolCallEnabled:       true,
		mcpMgr:                mcp.NewManager(),
		toolCallModeMgr:       NewToolCallModeManager(),
		intentExposureEnabled: true, // keep the meta parameter in the schema
	}
}

// measureCompactSchema returns (metaCount, metaBytes, schemaBytes, metaDescs).
func measureCompactSchema(t *testing.T, compact bool) (int, int, int, []string) {
	t.Helper()
	a := compactSchemaAgent(compact)
	tools := a.ToolDeclarations()
	if len(tools) == 0 {
		t.Fatalf("ToolDeclarations returned no tools (compact=%v)", compact)
	}

	var descs []string
	metaCount, metaBytes := 0, 0
	for i := range tools {
		props, _ := tools[i].Parameters["properties"].(map[string]interface{})
		if props == nil {
			continue
		}
		m, _ := props["meta"].(map[string]interface{})
		if m == nil {
			continue
		}
		desc, _ := m["description"].(string)
		if desc == "" {
			continue
		}
		metaCount++
		metaBytes += len(desc)
		descs = append(descs, desc)
	}

	wire := make([]compactWireTool, 0, len(tools))
	for _, tl := range tools {
		var w compactWireTool
		w.Type = "function"
		w.Function.Name = tl.Name
		w.Function.Description = tl.Description
		w.Function.Parameters = tl.Parameters
		wire = append(wire, w)
	}
	raw, err := json.Marshal(wire)
	if err != nil {
		t.Fatalf("marshal tools: %v", err)
	}
	t.Logf("META|compact=%v|count=%d|bytes=%d", compact, metaCount, metaBytes)
	t.Logf("SCHEMA|compact=%v|total=%d", compact, len(raw))
	return metaCount, metaBytes, len(raw), descs
}

// TestToolDescCompactSchema verifies the switch-aware meta descriptions.
func TestToolDescCompactSchema(t *testing.T) {
	// Match token_baseline_test.go: the package test runs with agent/ as cwd,
	// the product with the workspace root.
	if oldWD, err := os.Getwd(); err == nil {
		if err := os.Chdir(".."); err == nil {
			defer func() { _ = os.Chdir(oldWD) }()
		}
	}

	countOn, metaBytesOn, totalOn, descsOn := measureCompactSchema(t, true)
	countOff, metaBytesOff, totalOff, descsOff := measureCompactSchema(t, false)

	if countOn == 0 {
		t.Fatal("no meta parameters found in the tool declarations")
	}
	if countOn != countOff {
		t.Errorf("meta count differs: compact=%d full=%d", countOn, countOff)
	}
	for i, d := range descsOn {
		if strings.Contains(d, "\n") {
			t.Errorf("compact meta desc #%d is multi-line: %q", i, d)
		}
		if d != toolMetaDescCompact {
			t.Errorf("compact meta desc #%d != toolMetaDescCompact: %q", i, d)
		}
	}
	for i, d := range descsOff {
		if d != toolMetaDescLong {
			t.Errorf("full meta desc #%d != toolMetaDescLong: %q", i, d)
		}
	}
	if totalOn >= totalOff {
		t.Errorf("compact schema total (%d) must be strictly < full (%d)", totalOn, totalOff)
	}
	saved := totalOff - totalOn
	pct := 100 * float64(saved) / float64(totalOff)
	t.Logf("SAVED|compact_total=%d|full_total=%d|saved=%d|save_pct=%.2f%%|meta_compact=%d|meta_full=%d",
		totalOn, totalOff, saved, pct, metaBytesOn, metaBytesOff)
}
