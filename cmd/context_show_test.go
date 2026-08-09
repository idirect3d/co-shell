// Author: L.Shuang
// Created: 2026-08-07
// Last Modified: 2026-08-07
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

package cmd

import (
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/mcp"
	"github.com/idirect3d/co-shell/store"
	"github.com/idirect3d/co-shell/workspace"
)

// envPart builds a text ContentPart wrapping the given inner tags inside a full
// <environment_details> block with a message_no and an optional retried_count.
func envPart(messageNo int, timeStr string, retried int) string {
	var sb strings.Builder
	sb.WriteString("<environment_details>\n")
	if timeStr != "" {
		sb.WriteString("<time>")
		sb.WriteString(timeStr)
		sb.WriteString("</time>\n")
	}
	sb.WriteString("<message_no>")
	sb.WriteString(itoaSimple(messageNo))
	sb.WriteString("</message_no>\n")
	if retried > 0 {
		sb.WriteString("<retried_count>")
		sb.WriteString(itoaSimple(retried))
		sb.WriteString("</retried_count>\n")
	}
	sb.WriteString("<cwd>/tmp/test</cwd>\n")
	sb.WriteString("</environment_details>")
	return sb.String()
}

func itoaSimple(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}

// newTestContextHandler builds a ContextHandler with the given message history.
func newTestContextHandler(t *testing.T, messages []llm.Message) *ContextHandler {
	t.Helper()
	i18n.Init("zh")

	ws := t.TempDir()
	cfg := config.DefaultConfig()
	wsObj, err := workspace.New(ws)
	if err != nil {
		t.Fatalf("cannot init workspace: %v", err)
	}
	boltStore, err := store.NewStore(wsObj)
	if err != nil {
		t.Fatalf("cannot init bbolt store: %v", err)
	}
	ds := store.NewDualStore(boltStore, nil)
	ag := agent.New(nil, nil, ds, "")
	ag.SetWorkspacePath(ws)
	ag.SetConfig(cfg)
	if len(messages) > 0 {
		ag.SetHistory(messages)
	}
	return NewContextHandler(ag, ds)
}

// buildSampleMessages constructs a representative history:
//
//	[0] system
//	[1] user      (multiline text + env with message_no=1, retried_count=3)
//	[2] assistant (content + ToolCalls)
//	[3] tool      (result + env with message_no=3)
//	[4] user      (env with message_no=4, no retried_count)
func buildSampleMessages() []llm.Message {
	return []llm.Message{
		{Role: "system", Content: "你的名字是 work。"},
		{
			Role: "user",
			ContentParts: []llm.ContentPart{
				{Type: llm.ContentPartText, Text: "第一行指令\n    缩进第二行\ttab第三行\n\n空行后"},
				{Type: llm.ContentPartText, Text: envPart(1, "2026-08-07 12:38:07", 3)},
			},
		},
		{
			Role:    "assistant",
			Content: "这是一个关于 IT 治理的分析问题。",
			ToolCalls: []llm.ToolCall{
				{Name: "read_file", Arguments: `{"path":"cmd/context.go","intent":"查看"}`},
			},
		},
		{
			Role: "tool",
			ContentParts: []llm.ContentPart{
				{Type: llm.ContentPartText, Text: "[返回结果]total 559952\n-rw-r--r--   1 direct3d  staff     10244  8月  6 08:33 .DS_Store"},
				{Type: llm.ContentPartText, Text: envPart(3, "", 0)},
			},
		},
		{
			Role: "user",
			ContentParts: []llm.ContentPart{
				{Type: llm.ContentPartText, Text: "继续分析"},
				{Type: llm.ContentPartText, Text: envPart(4, "", 0)},
			},
		},
	}
}

// TestShowContext_TotalCount verifies the total message count matches the real
// message array length (tool_calls sub-blocks do not inflate the count).
func TestShowContext_TotalCount(t *testing.T) {
	h := newTestContextHandler(t, buildSampleMessages())
	out, err := h.showContext(false)
	if err != nil {
		t.Fatalf("showContext error: %v", err)
	}
	if !strings.Contains(out, "（总消息数: 5）") {
		t.Fatalf("expected total message count 5, got:\n%s", out)
	}
}

// TestShowContext_MessageSeparator verifies a long separator line separates
// consecutive message blocks.
func TestShowContext_MessageSeparator(t *testing.T) {
	h := newTestContextHandler(t, buildSampleMessages())
	out, err := h.showContext(false)
	if err != nil {
		t.Fatalf("showContext error: %v", err)
	}

	// Separator count should equal the number of message blocks (5 in sample).
	want := 5
	if got := strings.Count(out, contextSeparator); got != want {
		t.Fatalf("expected %d message separators, got %d:\n%s", want, got, out)
	}
}

// TestShowContext_IndexesMatchMessageNo verifies every header index equals the
// real message array index (and thus the <message_no> injected in each message).
// tool_calls must NOT create a separate index.
func TestShowContext_IndexesMatchMessageNo(t *testing.T) {
	h := newTestContextHandler(t, buildSampleMessages())
	out, err := h.showContext(false)
	if err != nil {
		t.Fatalf("showContext error: %v", err)
	}

	// Every real message index must appear as a header.
	for _, idx := range []string{"  0  ", "  1  ", "  2  ", "  3  ", "  4  "} {
		if !strings.Contains(out, idx) {
			t.Fatalf("expected header index %q in output:\n%s", idx, out)
		}
	}

	// The index 3 header must be the tool role, not a tool_calls pseudo message.
	lines := strings.Split(out, "\n")
	foundIdx3 := false
	for _, line := range lines {
		if strings.Contains(line, "  3  [tool") {
			foundIdx3 = true
			break
		}
	}
	if !foundIdx3 {
		t.Fatalf("index 3 must be the tool message (no pseudo tool_calls index), got:\n%s", out)
	}

	// No header may use index 5 (would be a phantom tool_calls index).
	if strings.Contains(out, "  5  [") {
		t.Fatalf("unexpected phantom index 5, got:\n%s", out)
	}
}

// TestShowContext_ToolCallsBlock verifies the tool_calls sub-block renders the
// tool name and indented JSON arguments under the assistant message.
func TestShowContext_ToolCallsBlock(t *testing.T) {
	h := newTestContextHandler(t, buildSampleMessages())
	out, err := h.showContext(false)
	if err != nil {
		t.Fatalf("showContext error: %v", err)
	}

	for _, want := range []string{"[tool_calls]", "- read_file", `"path": "cmd/context.go"`, `"intent": "查看"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in tool_calls block, got:\n%s", want, out)
		}
	}

	// The [tool_calls] label shares the 6-space indent with message content,
	// matching the user's requested tighter alignment.
	if !strings.Contains(out, "      [tool_calls]") {
		t.Fatalf("expected [tool_calls] at 6-space indent, got:\n%s", out)
	}

	// The tool_calls sub-block must appear within/after the assistant message (index 2).
	idxAssistant := strings.Index(out, "  2  [assistant")
	idxToolCalls := strings.Index(out, "[tool_calls]")
	if idxAssistant < 0 || idxToolCalls < 0 || idxToolCalls < idxAssistant {
		t.Fatalf("tool_calls must follow assistant index 2, got:\n%s", out)
	}
	// And must appear before the tool message (index 3).
	idxTool := strings.Index(out, "  3  [tool")
	if idxTool >= 0 && idxToolCalls > idxTool {
		t.Fatalf("tool_calls must appear before tool message index 3, got:\n%s", out)
	}
}

// TestShowContext_ControlCharsPreserved verifies multi-line content with
// indentation, tabs and blank lines is preserved (not flattened to spaces).
func TestShowContext_ControlCharsPreserved(t *testing.T) {
	h := newTestContextHandler(t, buildSampleMessages())
	out, err := h.showContext(false)
	if err != nil {
		t.Fatalf("showContext error: %v", err)
	}

	for _, want := range []string{
		"第一行指令",
		"缩进第二行",
		"tab第三行",
		"空行后",
		"[返回结果]total 559952",
		".DS_Store",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q preserved in output, got:\n%s", want, out)
		}
	}

	// The multi-line user text must be indented 6 spaces on continuation lines
	// (content starts 2 spaces left of the role label '[' for a tighter look).
	if !strings.Contains(out, "      缩进第二行") {
		t.Fatalf("expected content indented 6 spaces, got:\n%s", out)
	}
}

// TestShowContext_RetriedCount verifies ♾️N appears in the header for messages
// carrying <retried_count> in their environment details.
func TestShowContext_RetriedCount(t *testing.T) {
	h := newTestContextHandler(t, buildSampleMessages())
	out, err := h.showContext(false)
	if err != nil {
		t.Fatalf("showContext error: %v", err)
	}

	// Message 1 carries retried_count=3 → header shows ♾️3.
	if !strings.Contains(out, "♾️3") {
		t.Fatalf("expected ♾️3 on header for message 1, got:\n%s", out)
	}
	// Header line for index 1 must contain the retry suffix.
	lines := strings.Split(out, "\n")
	found := false
	for _, line := range lines {
		if strings.Contains(line, "  1  [user") && strings.Contains(line, "♾️3") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected ♾️3 on the user index 1 header line, got:\n%s", out)
	}

	// Messages without retried_count must not show ♾️.
	if strings.Contains(out, "♾️0") {
		t.Fatalf("unexpected ♾️0 (no retry suffix for absent tag), got:\n%s", out)
	}
}

// TestShowContext_HeaderTime verifies the <time> value is shown in the header.
func TestShowContext_HeaderTime(t *testing.T) {
	h := newTestContextHandler(t, buildSampleMessages())
	out, err := h.showContext(false)
	if err != nil {
		t.Fatalf("showContext error: %v", err)
	}
	if !strings.Contains(out, "2026-08-07 12:38:07") {
		t.Fatalf("expected message 1 time in header, got:\n%s", out)
	}
}

// TestShowContext_FullShowsEnvAndDefaultHidesIt verifies :context hides
// <environment_details> while :context full shows it for every message.
func TestShowContext_FullShowsEnvAndDefaultHidesIt(t *testing.T) {
	h := newTestContextHandler(t, buildSampleMessages())

	// Default: env hidden.
	outDefault, err := h.showContext(false)
	if err != nil {
		t.Fatalf("showContext(false) error: %v", err)
	}
	if strings.Contains(outDefault, "<environment_details>") {
		t.Fatalf("default output must hide <environment_details>, got:\n%s", outDefault)
	}

	// Full: env shown.
	outFull, err := h.showContext(true)
	if err != nil {
		t.Fatalf("showContext(true) error: %v", err)
	}
	count := strings.Count(outFull, "<environment_details>")
	if count < 3 {
		t.Fatalf("full output must include <environment_details> for each env-bearing message, got %d occurrences:\n%s", count, outFull)
	}
	if !strings.Contains(outFull, "<cwd>/tmp/test</cwd>") {
		t.Fatalf("full output must include cwd tag, got:\n%s", outFull)
	}
}

// TestShowContext_HandleFullSubcommand verifies the Handle dispatcher routes
// "full" to the full display mode.
func TestShowContext_HandleFullSubcommand(t *testing.T) {
	h := newTestContextHandler(t, buildSampleMessages())
	out, err := h.Handle([]string{"full"})
	if err != nil {
		t.Fatalf("Handle(full) error: %v", err)
	}
	if !strings.Contains(out, "<environment_details>") {
		t.Fatalf("Handle(full) must show environment details, got:\n%s", out)
	}
	if !strings.Contains(out, "（总消息数: 5）") {
		t.Fatalf("Handle(full) must show total message count, got:\n%s", out)
	}
}

// TestShowContext_ToolArgumentsInvalidJSON verifies invalid JSON arguments are
// rendered as-is without panicking.
func TestShowContext_ToolArgumentsInvalidJSON(t *testing.T) {
	msgs := []llm.Message{
		{Role: "system", Content: "sys"},
		{Role: "assistant", Content: "探索", ToolCalls: []llm.ToolCall{
			{Name: "execute_command", Arguments: `{bad json`},
		}},
	}
	h := newTestContextHandler(t, msgs)
	out, err := h.showContext(false)
	if err != nil {
		t.Fatalf("showContext error: %v", err)
	}
	if !strings.Contains(out, "- execute_command") {
		t.Fatalf("expected tool name in output, got:\n%s", out)
	}
	if !strings.Contains(out, "{bad json") {
		t.Fatalf("expected raw invalid JSON rendered as-is, got:\n%s", out)
	}
}

// newOpenAIToolHandler builds a ContextHandler with an openai-mode agent that
// has tool calling enabled and a non-nil MCP manager (empty server list), so
// the built-in tool declarations are populated via ToolDeclarations().
func newOpenAIToolHandler(t *testing.T, messages []llm.Message) *ContextHandler {
	t.Helper()
	i18n.Init("zh")

	ws := t.TempDir()
	cfg := config.DefaultConfig()
	wsObj, err := workspace.New(ws)
	if err != nil {
		t.Fatalf("cannot init workspace: %v", err)
	}
	boltStore, err := store.NewStore(wsObj)
	if err != nil {
		t.Fatalf("cannot init bbolt store: %v", err)
	}
	ds := store.NewDualStore(boltStore, nil)
	ag := agent.New(nil, mcp.NewManager(), ds, "")
	ag.SetWorkspacePath(ws)
	ag.SetConfig(cfg)
	ag.SetToolCallEnabled(true)
	if len(messages) > 0 {
		ag.SetHistory(messages)
	}
	return NewContextHandler(ag, ds)
}

// TestShowContext_FullOpenAIShowsToolDeclarations verifies :context full in
// openai mode appends the tool declarations block (name, description,
// parameters JSON) for the built-in tools.
func TestShowContext_FullOpenAIShowsToolDeclarations(t *testing.T) {
	h := newOpenAIToolHandler(t, buildSampleMessages())
	out, err := h.showContext(true)
	if err != nil {
		t.Fatalf("showContext(true) error: %v", err)
	}

	// Title line rendered via i18n zh key.
	if !strings.Contains(out, "[工具声明]") {
		t.Fatalf("expected tool declarations title in full openai output, got:\n%s", out)
	}
	// read_file is a built-in tool always present.
	if !strings.Contains(out, "- read_file") {
		t.Fatalf("expected read_file tool name in declarations, got:\n%s", out)
	}
	// The tool description must be rendered.
	if !strings.Contains(out, "Read the contents of a file at the specified path") {
		t.Fatalf("expected read_file description in declarations, got:\n%s", out)
	}
	// Parameters JSON schema marker and content.
	if !strings.Contains(out, "[parameters]") {
		t.Fatalf("expected [parameters] marker in declarations, got:\n%s", out)
	}
	if !strings.Contains(out, `"properties"`) {
		t.Fatalf("expected parameters JSON schema in declarations, got:\n%s", out)
	}
	// The block must appear after the last message block's separator.
	idxDecl := strings.Index(out, "[工具声明]")
	if idxDecl < 0 {
		t.Fatalf("expected tool declarations title, got:\n%s", out)
	}
	idxMsgEnd := strings.LastIndex(out[:idxDecl], contextSeparator)
	if idxMsgEnd < 0 {
		t.Fatalf("tool declarations must appear after the last message separator, got:\n%s", out)
	}
	// The declarations block ends with its own trailing separator.
	if !strings.Contains(out[idxDecl:], contextSeparator) {
		t.Fatalf("tool declarations block must end with a separator, got:\n%s", out)
	}
}

// TestShowContext_DefaultOpenAIHidesToolDeclarations verifies the default
// (non-full) :context output in openai mode does NOT include declarations.
func TestShowContext_DefaultOpenAIHidesToolDeclarations(t *testing.T) {
	h := newOpenAIToolHandler(t, buildSampleMessages())
	for name, out := range map[string]string{
		"default": mustShowContext(t, h, false),
		"show":    mustHandleContext(t, h, "show"),
		"none":    mustHandleContext(t, h, ""),
	} {
		if strings.Contains(out, "[工具声明]") {
			t.Fatalf("%s output must not include tool declarations, got:\n%s", name, out)
		}
	}
}

// TestShowContext_FullXMLHidesToolDeclarations verifies :context full in XML
// mode does NOT include tool declarations (tools are described in the prompt).
func TestShowContext_FullXMLHidesToolDeclarations(t *testing.T) {
	h := newOpenAIToolHandler(t, buildSampleMessages())
	h.agent.SetToolCallMode("xml")
	out, err := h.showContext(true)
	if err != nil {
		t.Fatalf("showContext(true) error: %v", err)
	}
	if strings.Contains(out, "[工具声明]") {
		t.Fatalf("xml mode must not include tool declarations, got:\n%s", out)
	}
	// Messages and environment details must still be present.
	if !strings.Contains(out, "<environment_details>") {
		t.Fatalf("xml full output must still include environment_details, got:\n%s", out)
	}
}

// TestShowContext_FullMCPNilNoPanic verifies :context full with a nil MCP
// manager (uninitialized agent) does not panic and simply omits declarations.
func TestShowContext_FullMCPNilNoPanic(t *testing.T) {
	h := newTestContextHandler(t, buildSampleMessages())
	out, err := h.showContext(true)
	if err != nil {
		t.Fatalf("showContext(true) error: %v", err)
	}
	if strings.Contains(out, "[工具声明]") {
		t.Fatalf("mcpMgr nil must omit tool declarations, got:\n%s", out)
	}
	// Normal message rendering must still work.
	if !strings.Contains(out, "<environment_details>") {
		t.Fatalf("mcpMgr nil full output must still include environment_details, got:\n%s", out)
	}
}

// mustShowContext runs showContext and fails the test on error.
func mustShowContext(t *testing.T, h *ContextHandler, full bool) string {
	t.Helper()
	out, err := h.showContext(full)
	if err != nil {
		t.Fatalf("showContext(%v) error: %v", full, err)
	}
	return out
}

// mustHandleContext runs Handle and fails the test on error.
func mustHandleContext(t *testing.T, h *ContextHandler, sub string) string {
	t.Helper()
	var args []string
	if sub != "" {
		args = []string{sub}
	}
	out, err := h.Handle(args)
	if err != nil {
		t.Fatalf("Handle(%q) error: %v", sub, err)
	}
	return out
}
