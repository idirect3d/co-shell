// Author: L.Shuang
// Created: 2026-05-01
// Last Modified: 2026-05-01
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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/subagent"
)

// launchSubAgentTool launches a sub-agent process to communicate with another
// co-shell agent. The target agent's workspace is a sibling folder of the
// current agent's workspace, identified by sub_agent_name.
//
// Workspace layout:
//
//	{parent}/
//	  {current_agent_name}/     <- current agent's workspace
//	  {target_agent_name}/      <- target agent's workspace (sibling)
//
// The target agent's workspace must already exist. If not, an error is returned.
func (a *Agent) launchSubAgentTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("launchSubAgentTool called: args=%v", args)

	instruction, ok := args["instruction"].(string)
	if !ok {
		return "", fmt.Errorf("instruction argument is required")
	}

	agentName, ok := args["sub_agent_name"].(string)
	if !ok || agentName == "" {
		return "", fmt.Errorf("sub_agent_name argument is required")
	}

	var timeout int
	if t, ok := args["timeout_seconds"].(float64); ok {
		timeout = int(t)
	}

	// Determine current workspace (parent of current agent's workspace folder)
	currentWorkspace, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("cannot get current workspace: %w", err)
	}

	// Target workspace is a sibling folder: {parent}/{agentName}
	parentDir := filepath.Dir(currentWorkspace)
	targetWorkspace := filepath.Join(parentDir, agentName)

	// Verify the target agent's workspace exists
	if _, statErr := os.Stat(targetWorkspace); os.IsNotExist(statErr) {
		return "", fmt.Errorf("target agent %q workspace not found at %s. The target agent's workspace must exist before calling launch_sub_agent", agentName, targetWorkspace)
	}

	fmt.Printf("\n📡 [%s] Communicating with agent '%s' (workspace: %s)\n\n", a.name, agentName, targetWorkspace)

	cfg := subagent.SubAgentConfig{
		Workspace:      targetWorkspace,
		Instruction:    instruction,
		TimeoutSeconds: timeout,
		ImagePaths:     a.imagePaths,
		ConfirmToolOff: a.approveAll,
	}

	log.Info("Launching sub-agent for agent '%s': workspace=%s, instruction=%s, timeout=%ds",
		agentName, targetWorkspace, instruction, timeout)

	result, err := a.subAgentMgr.LaunchSubAgent(ctx, cfg)
	if err != nil {
		log.Error("Failed to launch sub-agent for agent '%s': %v", agentName, err)
		return "", fmt.Errorf("failed to launch sub-agent for agent '%s': %w", agentName, err)
	}

	// Build result summary
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Agent '%s' completed.\n", agentName))
	sb.WriteString(result.ResultSummary())

	// Include output file contents if any
	for _, f := range result.OutputFiles {
		filePath := filepath.Join(targetWorkspace, "output", f)
		data, readErr := os.ReadFile(filePath)
		if readErr != nil {
			sb.WriteString(fmt.Sprintf("\n  ⚠️ Cannot read output file %s: %v\n", f, readErr))
			continue
		}
		sb.WriteString(fmt.Sprintf("\n📄 Output file: %s\n", f))
		sb.WriteString(string(data))
		if !strings.HasSuffix(string(data), "\n") {
			sb.WriteString("\n")
		}
	}

	log.Info("Agent '%s' completed: duration=%s, exitCode=%d", agentName, result.Duration, result.ExitCode)
	return sb.String(), nil
}
