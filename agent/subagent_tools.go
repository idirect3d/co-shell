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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/subagent"
)

// subAgentMemoryKey returns the memory key for a sub-agent by ID.
func subAgentMemoryKey(id int) string {
	return fmt.Sprintf("sub_agent:%d", id)
}

// getNextSubAgentID finds the next available sub-agent ID by scanning memory.
func (a *Agent) getNextSubAgentID() (int, error) {
	entries, err := a.store.SearchMemory("sub_agent:")
	if err != nil {
		return 1, nil // start from 1 if search fails
	}

	maxID := 0
	for _, entry := range entries {
		var info subagent.SubAgentInfo
		if err := json.Unmarshal([]byte(entry.Value), &info); err != nil {
			continue
		}
		if info.ID > maxID {
			maxID = info.ID
		}
	}
	return maxID + 1, nil
}

// getSubAgentInfo retrieves sub-agent info from memory by ID.
func (a *Agent) getSubAgentInfo(id int) (*subagent.SubAgentInfo, error) {
	val, found, err := a.store.GetMemory(subAgentMemoryKey(id))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("sub-agent #%d not found in memory", id)
	}

	var info subagent.SubAgentInfo
	if err := json.Unmarshal([]byte(val), &info); err != nil {
		return nil, fmt.Errorf("cannot parse sub-agent info: %w", err)
	}
	return &info, nil
}

// saveSubAgentInfo saves sub-agent info to memory.
func (a *Agent) saveSubAgentInfo(info *subagent.SubAgentInfo) error {
	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("cannot marshal sub-agent info: %w", err)
	}
	return a.store.SaveMemory(subAgentMemoryKey(info.ID), string(data))
}

// launchSubAgentTool launches a sub-agent process and returns its results.
// Sub-agent workspaces are auto-created under {parent_workspace}/sub-agents/{id}/.
// Each sub-agent is tracked in memory with its ID, workspace path, and purpose.
func (a *Agent) launchSubAgentTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("launchSubAgentTool called: args=%v", args)
	instruction, ok := args["instruction"].(string)
	if !ok {
		return "", fmt.Errorf("instruction argument is required")
	}

	var timeout int
	if t, ok := args["timeout_seconds"].(float64); ok {
		timeout = int(t)
	}

	purpose, _ := args["purpose"].(string)

	// Determine parent workspace
	parentWorkspace, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("cannot get parent workspace: %w", err)
	}

	// Check if reusing an existing sub-agent
	var subID int
	var workspacePath string
	var isNew bool

	if idVal, ok := args["sub_agent_id"].(float64); ok {
		// Reuse existing sub-agent
		subID = int(idVal)
		info, err := a.getSubAgentInfo(subID)
		if err != nil {
			return "", fmt.Errorf("cannot reuse sub-agent #%d: %v", subID, err)
		}
		workspacePath = info.Workspace
		// Update last instruction
		info.LastInstruction = instruction
		if purpose != "" {
			info.Purpose = purpose
		}
		if err := a.saveSubAgentInfo(info); err != nil {
			log.Warn("Cannot update sub-agent #%d memory: %v", subID, err)
		}
		fmt.Printf("\n🔄 Reusing sub-agent #%d (workspace: %s)\n\n", subID, workspacePath)
	} else {
		// Create new sub-agent
		subID, err = a.getNextSubAgentID()
		if err != nil {
			return "", fmt.Errorf("cannot allocate sub-agent ID: %w", err)
		}
		// Use agent name in workspace folder: {name}-{id}
		workspacePath = filepath.Join(parentWorkspace, "sub-agents", fmt.Sprintf("%s-%d", a.name, subID))

		// Save to memory
		info := &subagent.SubAgentInfo{
			ID:              subID,
			Workspace:       workspacePath,
			Purpose:         purpose,
			CreatedAt:       time.Now().Format("2006-01-02 15:04:05"),
			LastInstruction: instruction,
		}
		if err := a.saveSubAgentInfo(info); err != nil {
			log.Warn("Cannot save sub-agent #%d memory: %v", subID, err)
		}
		isNew = true
		fmt.Printf("\n📂 [%s] Creating sub-agent #%d (workspace: %s)\n\n", a.name, subID, workspacePath)
	}

	cfg := subagent.SubAgentConfig{
		Workspace:         workspacePath,
		Instruction:       instruction,
		TimeoutSeconds:    timeout,
		Purpose:           purpose,
		ImagePaths:        a.imagePaths,
		ConfirmCommandOff: a.approveAll,
	}

	log.Info("Launching sub-agent #%d: workspace=%s, instruction=%s, timeout=%ds", subID, workspacePath, instruction, timeout)

	result, err := a.subAgentMgr.LaunchSubAgent(ctx, cfg)
	if err != nil {
		log.Error("Failed to launch sub-agent #%d: %v", subID, err)
		return "", fmt.Errorf("failed to launch sub-agent #%d: %w", subID, err)
	}

	// Build result summary
	var sb strings.Builder
	if isNew {
		sb.WriteString(fmt.Sprintf("Sub-agent #%d completed.\n", subID))
	} else {
		sb.WriteString(fmt.Sprintf("Sub-agent #%d (reused) completed.\n", subID))
	}
	sb.WriteString(result.ResultSummary())

	// Include output file contents if any
	for _, f := range result.OutputFiles {
		filePath := filepath.Join(workspacePath, "output", f)
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

	log.Info("Sub-agent #%d completed: duration=%s, exitCode=%d", subID, result.Duration, result.ExitCode)
	return sb.String(), nil
}
