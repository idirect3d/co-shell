package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/mcp"
)

// FIX-510 test cases — see use-case/FIX-510/FIX-510-UC-0001.md.

// UC-0010: exemption decision for the context-overflow skip guard.
func TestHasContextOverflowExemptTool(t *testing.T) {
	tests := []struct {
		name      string
		toolCalls []llm.ToolCall
		want      bool
	}{
		{"reorganize_context alone", []llm.ToolCall{{Name: "reorganize_context"}}, true},
		{"attempt_completion alone", []llm.ToolCall{{Name: "attempt_completion"}}, true},
		{"ordinary tool only", []llm.ToolCall{{Name: "read_file"}}, false},
		{"no tool calls", nil, false},
		{"mixed: ordinary + reorganize_context", []llm.ToolCall{{Name: "read_file"}, {Name: "reorganize_context"}}, true},
		{"mixed: ordinary + attempt_completion", []llm.ToolCall{{Name: "read_file"}, {Name: "attempt_completion"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasContextOverflowExemptTool(tt.toolCalls); got != tt.want {
				t.Errorf("hasContextOverflowExemptTool() = %v, want %v", got, tt.want)
			}
		})
	}
}

// UC-0009: reorganize_context no longer declares meta as required, while
// ordinary tools keep requiring it (control group).
func TestReorganizeContextDoesNotRequireMeta(t *testing.T) {
	a := &Agent{cfg: config.DefaultConfig(), mcpMgr: mcp.NewManager(), intentExposureEnabled: true}
	if a.toolRequiresMeta("reorganize_context") {
		t.Error("reorganize_context must not require meta after FIX-510")
	}
	if !a.toolRequiresMeta("read_file") {
		t.Error("read_file must still require meta (control group)")
	}
}

// FIX-510: the {NO_META_TOOLS} hint is derived from the tool schemas, so the
// list shown to the LLM cannot drift from the enforced required lists.
// NOTE: i18n.Init is deliberately NOT called here — switching the global
// language would leak into other tests in this package.
func TestApplyNoMetaToolsHint(t *testing.T) {
	a := &Agent{cfg: config.DefaultConfig(), mcpMgr: mcp.NewManager(), intentExposureEnabled: true}
	tools := a.buildToolsInternal()

	names := toolsWithoutMetaNames(tools)
	if !containsString(names, "reorganize_context") {
		t.Errorf("expected reorganize_context in no-meta tool list, got %v", names)
	}
	if containsString(names, "read_file") {
		t.Errorf("read_file must NOT be in the no-meta tool list, got %v", names)
	}

	// The placeholder is always consumed, whatever the active language is.
	text := applyNoMetaToolsHint("intro\n\n{NO_META_TOOLS}\n", tools)
	if strings.Contains(text, "{NO_META_TOOLS}") {
		t.Error("placeholder must be replaced")
	}

	// Text without the placeholder is returned unchanged.
	plain := "no placeholder here"
	if got := applyNoMetaToolsHint(plain, tools); got != plain {
		t.Errorf("applyNoMetaToolsHint() = %q, want unchanged", got)
	}
}

// UC-0005: reorganize_context executes successfully when meta is omitted.
func TestReorganizeContextToolWithoutMeta(t *testing.T) {
	a := &Agent{}
	result, err := a.reorganizeContextTool(context.Background(), map[string]interface{}{
		"summary_prompt": "   ## Goal\nkeep going   ",
	})
	if err != nil {
		t.Fatalf("reorganizeContextTool without meta failed: %v", err)
	}
	if result == "" {
		t.Error("expected a non-empty result message")
	}
	if !a.reorganizeContextUsed {
		t.Error("reorganizeContextUsed flag must be set so the caller collapses history")
	}
	if got := a.taskInstructionCache.String(); got != "## Goal\nkeep going" {
		t.Errorf("taskInstructionCache = %q, want trimmed summary", got)
	}
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// FIX-510 (follow-up): a round that carries an exempt tool is EXECUTED, so it
// must never be reported as skipped — the "skipped" warning is gated on the
// very same predicate.
func TestContextOverflowSkipsTools(t *testing.T) {
	tests := []struct {
		name      string
		usagePct  float64
		threshold float64
		toolCalls []llm.ToolCall
		want      bool
	}{
		{"over threshold + attempt_completion → executed (no warning)", 26, 25, []llm.ToolCall{{Name: "attempt_completion"}}, false},
		{"over threshold + reorganize_context → executed (no warning)", 90, 80, []llm.ToolCall{{Name: "reorganize_context"}}, false},
		{"over threshold + ordinary tool → skipped", 90, 80, []llm.ToolCall{{Name: "read_file"}}, true},
		{"over threshold + no tool calls → skipped", 90, 80, nil, true},
		{"at threshold boundary + ordinary tool → skipped", 80, 80, []llm.ToolCall{{Name: "read_file"}}, true},
		{"below threshold + ordinary tool → executed", 20, 80, []llm.ToolCall{{Name: "read_file"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := contextOverflowSkipsTools(tt.usagePct, tt.threshold, tt.toolCalls); got != tt.want {
				t.Errorf("contextOverflowSkipsTools(%v, %v, %v) = %v, want %v", tt.usagePct, tt.threshold, tt.toolCalls, got, tt.want)
			}
		})
	}
}
