// Author: L.Shuang
// Created: 2026-06-11
// Last Modified: 2026-06-11
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
// IMPLIED, BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
)

// SimulateHandler handles the .simulate built-in command.
type SimulateHandler struct {
	agent *agent.Agent
	cfg   *config.Config
}

// NewSimulateHandler creates a new SimulateHandler.
func NewSimulateHandler(ag *agent.Agent, cfg *config.Config) *SimulateHandler {
	return &SimulateHandler{agent: ag, cfg: cfg}
}

// Handle processes .simulate commands.
func (h *SimulateHandler) Handle(args []string) (string, error) {
	if len(args) == 0 {
		return showSimulateHelp(), nil
	}

	content := strings.Join(args, " ")
	ep := config.GetEmojiPrefixes(h.cfg.LLM.EmojiEnabled)

	// Parse and execute tool calls
	results, err := h.agent.SimulateToolCall(context.Background(), content)
	if err != nil {
		// If we have partial results, still show them
		if results != nil && len(results) > 0 {
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("%s%s\n", ep.Warning, i18n.T(i18n.KeySimulatePartial)))
			for _, r := range results {
				sb.WriteString(formatSimulateResult(ep, r))
			}
			sb.WriteString(fmt.Sprintf("\n%s%s: %v\n", ep.Error, i18n.T(i18n.KeyError), err))
			return sb.String(), nil
		}
		return "", fmt.Errorf("%v", err)
	}

	// Build output
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s%s\n", ep.Info, i18n.TF(i18n.KeySimulateParsingResult, len(results))))

	for _, r := range results {
		sb.WriteString(formatSimulateResult(ep, r))
	}

	return sb.String(), nil
}

// formatSimulateResult formats a single tool call result for display.
func formatSimulateResult(ep config.EmojiPrefixes, r agent.ToolCallResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n%s%s\n", ep.ToolCallInput, r.Name))
	sb.WriteString(fmt.Sprintf("  %s%s\n", i18n.T(i18n.KeySimulateLabelArgs), r.Arguments))

	if r.Error != "" {
		sb.WriteString(fmt.Sprintf("  %s%s: %s\n", ep.Error, i18n.T(i18n.KeySimulateLabelError), r.Error))
	} else {
		sb.WriteString(fmt.Sprintf("  %s%s\n", ep.Success, i18n.T(i18n.KeySimulateLabelSuccess)))
		if r.Result != "" {
			// Truncate long results for display
			resultDisplay := r.Result
			if len(resultDisplay) > 500 {
				resultDisplay = resultDisplay[:500] + "..."
			}
			sb.WriteString(fmt.Sprintf("  %s%s\n", i18n.T(i18n.KeySimulateLabelResult), resultDisplay))
		}
	}

	return sb.String()
}

// showSimulateHelp displays the .simulate command usage.
func showSimulateHelp() string {
	return `🔧 模拟 LLM 方法调用 (.simulate)

  模拟 LLM 返回的方法调用内容，进行完整解析和执行测试。
  自动跟随当前 tool call 模式（openai/xml）。

  XML 格式:
    .simulate <execute_command><command>ls -la</command></execute_command>
    .simulate <read_file><path>main.go</path></read_file>

  JSON 格式（OpenAI 模式）:
    .simulate {"name":"read_file","arguments":{"path":"main.go"}}
    .simulate [{"name":"read_file","arguments":{"path":"main.go"}}]

  说明:
    - 解析、确认、执行逻辑与 LLM 调用的流程完全一致
    - 命令会被记录到历史中，但执行结果不会加入对话上下文
    - 当前模式为 XML 时自动使用 XML 解析，OpenAI 模式时使用 JSON 解析`
}
