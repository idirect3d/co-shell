// Author: L.Shuang
// Created: 2026-04-26
// Last Modified: 2026-04-26
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

// Package subagent provides the ability to launch and manage sub-agent processes.
//
// A sub-agent is an independent co-shell process that runs in its own workspace,
// sharing the same terminal (stdin/stdout/stderr) with the parent agent.
// The parent agent monitors the sub-agent's execution and collects its results.
package subagent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// SubAgentInfo holds the metadata for a registered sub-agent.
type SubAgentInfo struct {
	// ID is the sequential number (1, 2, 3, ...).
	ID int `json:"id"`

	// Workspace is the absolute path to the sub-agent's workspace directory.
	Workspace string `json:"workspace"`

	// Purpose describes what this sub-agent is used for.
	Purpose string `json:"purpose"`

	// CreatedAt is the timestamp when the sub-agent was created.
	CreatedAt string `json:"created_at"`

	// LastInstruction is the last instruction given to this sub-agent.
	LastInstruction string `json:"last_instruction"`
}

// SubAgentConfig holds the configuration for launching a sub-agent.
type SubAgentConfig struct {
	// Workspace is the path to the sub-agent's workspace directory.
	Workspace string

	// Instruction is the natural language instruction or system command to execute.
	Instruction string

	// TimeoutSeconds is the maximum time to wait for the sub-agent to complete.
	// 0 means no timeout.
	TimeoutSeconds int

	// ExecPath is the path to the co-shell executable.
	// If empty, it will be determined from os.Executable().
	ExecPath string

	// Purpose describes what this sub-agent is used for (for memory tracking).
	Purpose string
}

// SubAgentResult holds the result of a sub-agent execution.
type SubAgentResult struct {
	// Stdout contains the standard output from the sub-agent.
	Stdout string

	// Stderr contains the standard error from the sub-agent.
	Stderr string

	// ExitCode is the exit code of the sub-agent process.
	ExitCode int

	// OutputFiles lists the files found in the sub-agent's output/ directory.
	OutputFiles []string

	// Duration is the actual execution duration.
	Duration time.Duration

	// Err contains any error that occurred during execution.
	Err error
}

// Manager manages sub-agent processes.
type Manager struct {
	mu       sync.Mutex
	execPath string
	active   map[int]*context.CancelFunc
}

// NewManager creates a new sub-agent manager.
func NewManager() *Manager {
	execPath, _ := os.Executable()
	return &Manager{
		execPath: execPath,
		active:   make(map[int]*context.CancelFunc),
	}
}

// LaunchSubAgent launches a sub-agent process with the given configuration.
// It monitors the process and collects results.
func (m *Manager) LaunchSubAgent(ctx context.Context, cfg SubAgentConfig) (*SubAgentResult, error) {
	// Determine executable path
	execPath := m.execPath
	if cfg.ExecPath != "" {
		execPath = cfg.ExecPath
	}

	// Ensure workspace exists
	if err := os.MkdirAll(cfg.Workspace, 0755); err != nil {
		return nil, fmt.Errorf("cannot create sub-agent workspace %q: %w", cfg.Workspace, err)
	}

	// Build command: co-shell -w <workspace> -c <instruction>
	args := []string{
		"-w", cfg.Workspace,
		"-c", cfg.Instruction,
	}

	// If parent has a config path, pass it via -c/--config
	if configPath := os.Getenv("CO_SHELL_CONFIG_PATH"); configPath != "" {
		args = append([]string{"-c", configPath}, args...)
	}

	// Print the full command for debugging
	fmt.Printf("\n🔧 Sub-agent command: %s %s\n\n", execPath, strings.Join(args, " "))

	// Create context with timeout if specified
	var cancel context.CancelFunc
	if cfg.TimeoutSeconds > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(cfg.TimeoutSeconds)*time.Second)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	// Prepare command
	cmd := exec.CommandContext(ctx, execPath, args...)

	// Set environment: mark as sub-agent and pass config path
	env := os.Environ()
	env = append(env, "CO_SHELL_SUB_AGENT=true")
	if configPath := os.Getenv("CO_SHELL_CONFIG_PATH"); configPath != "" {
		env = append(env, "CO_SHELL_CONFIG_PATH="+configPath)
	}
	cmd.Env = env

	// Share stdin/stdout/stderr with parent
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Record start time
	startTime := time.Now()

	// Track the process
	m.mu.Lock()
	pid := -1
	m.active[pid] = &cancel
	m.mu.Unlock()

	// Run the command
	err := cmd.Run()

	// Clean up tracking
	m.mu.Lock()
	delete(m.active, pid)
	m.mu.Unlock()

	duration := time.Since(startTime)

	// Collect results
	result := &SubAgentResult{
		Duration: duration,
		ExitCode: 0,
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Err = fmt.Errorf("sub-agent timed out after %d seconds", cfg.TimeoutSeconds)
			result.ExitCode = -1
			return result, nil
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			result.Stderr = string(exitErr.Stderr)
		} else {
			result.Err = err
			result.ExitCode = -1
		}
	}

	// Collect output files from workspace output/ directory
	outputDir := filepath.Join(cfg.Workspace, "output")
	if entries, err := os.ReadDir(outputDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				result.OutputFiles = append(result.OutputFiles, entry.Name())
			}
		}
	}

	return result, nil
}

// Close cancels all active sub-agent processes.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, cancel := range m.active {
		(*cancel)()
	}
	m.active = make(map[int]*context.CancelFunc)
}

// ResultSummary returns a human-readable summary of the sub-agent result.
func (r *SubAgentResult) ResultSummary() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("⏱ 执行时长: %s\n", r.Duration))

	if r.Err != nil {
		sb.WriteString(fmt.Sprintf("❌ 错误: %v\n", r.Err))
	}

	sb.WriteString(fmt.Sprintf("🔚 退出码: %d\n", r.ExitCode))

	if len(r.OutputFiles) > 0 {
		sb.WriteString("📁 输出文件:\n")
		for _, f := range r.OutputFiles {
			sb.WriteString(fmt.Sprintf("  - %s\n", f))
		}
	}

	return sb.String()
}
