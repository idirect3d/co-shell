// Author: L.Shuang
// Created: 2026-08-03
// Last Modified: 2026-08-03
//
// MIT License
//
// Copyright (c) 2026 L.Shuang
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

package agent

import (
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
)

func init() {
	i18n.Init("")
}

// TestBuildJudgeContext verifies buildJudgeContext emits cwd, workspace,
// recent paths and available tools so the judge can write a concrete
// exit_strategy (FIX-322).
func TestBuildJudgeContext(t *testing.T) {
	a := &Agent{
		workspacePath: "/tmp/demo-ws",
		messages: []llm.Message{
			{Role: "system", Content: "sys"},
			{Role: "user", Content: "请修改 src/main.go 和 config/dev.yaml"},
			{Role: "assistant", Content: "我先读取 agent/loop.go"},
		},
	}

	// NOTE: available_tools is only emitted when the agent is fully initialized
	// (mcpMgr != nil, toolCallEnabled=true). Unit tests use an uninitialized
	// Agent, so we verify the environment core content only.
	ctx := a.buildJudgeContext()

	for _, want := range []string{
		"cwd:",
		"workspace: /tmp/demo-ws",
		"recent_paths:",
		"src/main.go",
		"config/dev.yaml",
		"agent/loop.go",
	} {
		if !strings.Contains(ctx, want) {
			t.Errorf("buildJudgeContext should contain %q, got:\n%s", want, ctx)
		}
	}
}

// TestBuildLoopJudgeUserPrompt_ContainsContextBlock verifies the judge user
// prompt includes the workspace/tools context block replacing {CONTEXT}
// (FIX-322).
func TestBuildLoopJudgeUserPrompt_ContainsContextBlock(t *testing.T) {
	a := &Agent{
		workspacePath: "/tmp/demo-ws",
		lastUserInput: "调研",
	}

	prompt := a.buildLoopJudgeUserPrompt("no plan", "suspect content")

	// The raw {CONTEXT} placeholder must be replaced (i18n.Init must be done).
	if strings.Contains(prompt, "{CONTEXT}") {
		t.Errorf("{CONTEXT} placeholder not replaced; i18n.Init may not have loaded the template")
	}
	if !strings.Contains(prompt, "cwd:") {
		t.Errorf("prompt should contain the context block with cwd, got:\n%s", prompt)
	}
}

// TestLoopJudgeFallbackI18n verifies the fallback directive key exists and is
// non-empty in the active language (FIX-322).
func TestLoopJudgeFallbackI18n(t *testing.T) {
	fallback := i18n.T(i18n.KeyLoopJudgeFallback)
	if strings.TrimSpace(fallback) == "" {
		t.Fatal("KeyLoopJudgeFallback should have a non-empty translation")
	}
}
