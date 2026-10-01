// Package agent - deterministic token baseline measurement (FEATURE-577).
//
// It measures, without any LLM call, how much context each stage of a request
// contributes before the token optimization work (see
// research/token优化设计方案.md and research/token-baseline/测试方案.md):
//
//	S0  system prompt (the fixed system message of every request)
//	S1  tool declarations (full JSON schema per round, openai mode)
//	S2  a single <environment_details> envelope (L0) and its blocks, plus the
//	    planned L1 (lite) / L2 (minimal) levels
//	S3  same-round repetition of the envelope (one per tool result)
//	S4  cumulative growth over N rounds of tool iterations
//	S5  composed single-round request size (S0 + S1 + history + envelope)
//
// Run with:
//
//	go test ./agent -run TestTokenBaselineStages -v -count=1
//
// The printed data lines ("S0|...") must be byte-identical across runs so that
// three consecutive runs produce the same md5 (see use-case UC-0001).
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

// bytesPerToken is the rough conversion used for the token estimates. Byte
// counts are authoritative; token numbers are indicative (see 测试方案 §6).
const bytesPerToken = 3.7

func estTokens(b int) float64 { return float64(b) / bytesPerToken }

// extractEnvTag returns the "<tag>...</tag>" slice of an envelope, or "".
func extractEnvTag(s, tag string) string {
	open := "<" + tag + ">"
	closeTag := "</" + tag + ">"
	i := strings.Index(s, open)
	if i < 0 {
		return ""
	}
	j := strings.Index(s[i:], closeTag)
	if j < 0 {
		return ""
	}
	return s[i : i+j+len(closeTag)]
}

// blockBytes returns the byte length of a top-level envelope block.
func blockBytes(env, tag string) int { return len(extractEnvTag(env, tag)) }

// liteEnvBytes rebuilds the L1 (lite) envelope from the full one: volatile
// context plus task-scoped blocks, without the static file listings, tool
// schema dump, runtime info, board roster and dynamic events.
func liteEnvBytes(full string) int {
	var b strings.Builder
	b.WriteString("<environment_details>\n")
	for _, tag := range []string{
		"time", "message_no", "context_window", "cwd", "current_mode",
		"no_tool_action", "session_title", "task_plan", "opened_resources",
		"sub_agents", "board_replies",
	} {
		if v := extractEnvTag(full, tag); v != "" {
			b.WriteString(v)
			b.WriteString("\n")
		}
	}
	b.WriteString("</environment_details>")
	return b.Len()
}

// minimalEnvBytes is the L2 (minimal) envelope: time + message_no only.
func minimalEnvBytes() int {
	return len("<environment_details>\n<time>2026-10-02 00:00:00 Friday</time>\n<message_no>1</message_no>\n</environment_details>")
}

// metaParamBytes counts how many tools declare a "meta" parameter and the total
// byte size of those duplicated descriptions (the D1 optimization target).
func metaParamBytes(tools []toolLike) (count int, bytes int) {
	for _, tl := range tools {
		props, _ := tl.params["properties"].(map[string]interface{})
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
		count++
		bytes += len(desc)
	}
	return count, bytes
}

// toolLike is a minimal view over llm.Tool used by the measurement helpers.
type toolLike struct {
	name   string
	params map[string]interface{}
}

// wireTool mirrors the JSON shape the OpenAI client actually sends for tool
// declarations (llm/client.go: toolJSON / functionDefinitionJSON), so the
// measured byte count matches the real request payload.
type wireTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string      `json:"name"`
		Description string      `json:"description"`
		Parameters  interface{} `json:"parameters"`
	} `json:"function"`
}

// TestTokenBaselineStages prints the deterministic per-stage baseline sizes.
func TestTokenBaselineStages(t *testing.T) {
	// The package test runs with agent/ as cwd while the product runs with the
	// workspace root as cwd (where bin/ and research/ live). Switch so that the
	// measured <tools>/<research> blocks match reality.
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(".."); err != nil {
		t.Fatalf("chdir to workspace root: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	cfg := config.DefaultConfig()
	// Dynamic events need a live queue; they are irrelevant here and switched
	// off to keep the measurement hermetic.
	cfg.LLM.EnvIncludeUserDynamic = false
	a := &Agent{cfg: cfg, toolCallEnabled: true, mcpMgr: mcp.NewManager()}

	// ---- S0: system prompt -------------------------------------------------
	sysBytes := 0
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("S0|systemPrompt|unavailable (%v)", r)
			}
		}()
		a.RebuildSystemPrompt()
		sysBytes = len(a.systemPrompt)
		t.Logf("S0|systemPrompt|bytes=%d|tokens=%.0f", sysBytes, estTokens(sysBytes))
	}()

	// ---- S1: tool declarations --------------------------------------------
	tools := a.buildToolsInternal()
	// The unified meta parameter is injected by the request path
	// (injectMetaParam, agent/tools.go); apply the same post-processing here so
	// the measured schema equals the one really sent to the model.
	for i := range tools {
		injectMetaParam(&tools[i])
	}
	wire := make([]wireTool, 0, len(tools))
	for _, tl := range tools {
		var w wireTool
		w.Type = "function"
		w.Function.Name = tl.Name
		w.Function.Description = tl.Description
		w.Function.Parameters = tl.Parameters
		wire = append(wire, w)
	}
	toolJSON, err := json.Marshal(wire)
	if err != nil {
		t.Fatalf("marshal tools: %v", err)
	}
	likes := make([]toolLike, 0, len(tools))
	for _, tl := range tools {
		likes = append(likes, toolLike{name: tl.Name, params: tl.Parameters})
	}
	metaCount, metaBytes := metaParamBytes(likes)
	perTool := 0
	if len(tools) > 0 {
		perTool = len(toolJSON) / len(tools)
	}
	t.Logf("S1|tools|count=%d|json_bytes=%d|json_tokens=%.0f|bytes_per_tool=%d|meta_count=%d|meta_bytes=%d|meta_share=%.1f%%",
		len(tools), len(toolJSON), estTokens(len(toolJSON)), perTool, metaCount, metaBytes,
		100*float64(metaBytes)/float64(len(toolJSON)))

	// ---- S2: one envelope (L0) and its blocks ------------------------------
	full := a.buildFullEnvironmentDetails(1, []string{"read_file"})
	l0 := len(full)
	l1 := liteEnvBytes(full)
	l2 := minimalEnvBytes()
	t.Logf("S2|envL0|bytes=%d|tokens=%.0f|current_dir=%d|tools=%d|research=%d|board_agents=%d|runtime_info=%d|task_plan=%d|opened_resources=%d|sub_agents=%d|board_replies=%d|user_dynamic=%d|other=%d",
		l0, estTokens(l0),
		blockBytes(full, "current_dir"), blockBytes(full, "tools"), blockBytes(full, "research"),
		blockBytes(full, "board_agents"), blockBytes(full, "runtime_info"),
		blockBytes(full, "task_plan"), blockBytes(full, "opened_resources"),
		blockBytes(full, "sub_agents"), blockBytes(full, "board_replies"),
		blockBytes(full, "user_dynamic_events"),
		l0-blockBytes(full, "current_dir")-blockBytes(full, "tools")-blockBytes(full, "research")-
			blockBytes(full, "board_agents")-blockBytes(full, "runtime_info")-blockBytes(full, "task_plan")-
			blockBytes(full, "opened_resources")-blockBytes(full, "sub_agents")-blockBytes(full, "board_replies")-
			blockBytes(full, "user_dynamic_events"))
	t.Logf("S2|envL1|bytes=%d|tokens=%.0f", l1, estTokens(l1))
	t.Logf("S2|envL2|bytes=%d|tokens=%.0f", l2, estTokens(l2))

	// ---- S3: same-round repetition (one envelope per tool result) ----------
	for n := 1; n <= 5; n++ {
		now := n * l0
		optB := l0 + (n-1)*l2 // plan B: L0 for the last result, L2 for the rest
		t.Logf("S3|round_tools=%d|current_bytes=%d|planB_bytes=%d|saved=%d|save_pct=%.1f%%",
			n, now, optB, now-optB, 100*float64(now-optB)/float64(now))
	}

	// ---- S4: cumulative growth over rounds ---------------------------------
	t.Logf("S4|%-6s|%-12s|%-12s|%-10s|%-8s", "round", "cum_current", "cum_pruned", "saved", "save_pct")
	for _, rounds := range []int{1, 2, 3, 5, 12, 50} {
		cur := rounds * l0
		pruned := l0
		if rounds >= 2 {
			pruned += l1
		}
		if rounds > 2 {
			pruned += (rounds - 2) * l2
		}
		t.Logf("S4|%-6d|%-12d|%-12d|%-10d|%.1f%%", rounds, cur, pruned, cur-pruned,
			100*float64(cur-pruned)/float64(cur))
	}
	t.Logf("S4|slope|current_bytes_per_round=%d|pruned_bytes_per_round=%d|current_50_rounds=%d|pruned_50_rounds=%d",
		l0, l2, 50*l0, l0+l1+48*l2)

	// ---- S5: composed single-round request --------------------------------
	for _, rounds := range []int{1, 10, 50} {
		history := rounds * l0 // one envelope per round in the current behaviour
		total := sysBytes + len(toolJSON) + history
		fixed := sysBytes + len(toolJSON)
		pct := 0.0
		if total > 0 {
			pct = 100 * float64(fixed) / float64(total)
		}
		t.Logf("S5|rounds=%d|system=%d|tools=%d|history=%d|total_bytes=%d|total_tokens=%.0f|fixed_share=%.1f%%|of_512k=%.2f%%",
			rounds, sysBytes, len(toolJSON), history, total, estTokens(total), pct,
			100*float64(total)/float64(512*1024))
	}
}
