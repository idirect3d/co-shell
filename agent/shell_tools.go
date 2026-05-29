// Author: L.Shuang
// Created: 2026-05-28
// Last Modified: 2026-05-28
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
	"time"

	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/shell"
)

// shellStartTool starts a persistent interactive shell session.
// This creates a long-running shell process that maintains state (cd, env vars, etc.)
// across multiple command executions. Returns the session status.
func (a *Agent) shellStartTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if !a.shellEnabled {
		return "", fmt.Errorf("persistent shell session is disabled")
	}

	// Close any existing session first
	if a.shellSession != nil {
		if err := a.shellSession.Close(); err != nil {
			log.Warn("Failed to close existing shell session: %v", err)
		}
		a.shellSession = nil
	}

	// Create and start a new session
	sess := &shell.Session{}
	status, err := sess.Start()
	if err != nil {
		return "", fmt.Errorf("failed to start persistent shell: %w", err)
	}

	a.shellSession = sess

	result, _ := json.Marshal(map[string]interface{}{
		"status":      "started",
		"shell_type":  status.ShellType,
		"working_dir": status.WorkingDir,
		"started_at":  status.StartedAt,
	})

	log.Info("Persistent shell session started via tool call: type=%s", status.ShellType)
	return string(result), nil
}

// shellExecTool executes a command in the persistent shell session.
// The command runs in the same shell environment, preserving state
// (current directory, environment variables, etc.).
func (a *Agent) shellExecTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if !a.shellEnabled {
		return "", fmt.Errorf("persistent shell session is disabled")
	}

	if a.shellSession == nil {
		return "", fmt.Errorf("no persistent shell session is active. Use shell_start to start one first")
	}

	command, ok := args["command"].(string)
	if !ok {
		return "", fmt.Errorf("command argument is required")
	}

	// Get LLM-suggested timeout (optional)
	timeoutSeconds := 0
	if t, ok := args["timeout_seconds"].(float64); ok {
		timeoutSeconds = int(t)
	}

	// Create context with optional timeout
	execCtx := ctx
	if timeoutSeconds > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
		defer cancel()
	}

	output, err := a.shellSession.Exec(execCtx, command)
	if err != nil {
		log.Debug("Shell exec error: %v (output: %s)", err, output)
		// Return partial output with error info
		if output != "" {
			return fmt.Sprintf("%s\n\n⚠️ 命令执行出错：%v", output, err), nil
		}
		return "", fmt.Errorf("shell command failed: %w", err)
	}

	return output, nil
}

// shellGetOutputTool retrieves scrollback content from the shell session.
// Returns the requested lines from the terminal history, total lines available,
// and whether any lines were truncated.
func (a *Agent) shellGetOutputTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if !a.shellEnabled {
		return "", fmt.Errorf("persistent shell session is disabled")
	}

	if a.shellSession == nil {
		return "", fmt.Errorf("no persistent shell session is active. Use shell_start to start one first")
	}

	lastFrom := 1
	if lf, ok := args["last_from"].(float64); ok {
		lastFrom = int(lf)
	}
	if lastFrom < 1 {
		lastFrom = 1
	}

	count := 50
	if c, ok := args["count"].(float64); ok {
		count = int(c)
	}
	if count < 1 {
		count = 50
	}

	output, totalLines, truncatedCount := a.shellSession.GetOutput(lastFrom, count)
	return fmt.Sprintf("终端历史输出（共%d行，本次返回从倒数第%d行开始%d行）：\n%s\n\n%s",
		totalLines, lastFrom, count, output,
		truncatedOutputSummary(truncatedCount)), nil
}

// truncatedOutputSummary returns a summary of how many lines were truncated
func truncatedOutputSummary(truncatedCount int) string {
	if truncatedCount > 0 {
		return fmt.Sprintf("⚠️ 有%d行超出每行最大字符数限制被截断", truncatedCount)
	}
	return ""
}

// shellStopTool stops the persistent shell session.
// Sends exit command and cleans up resources.
func (a *Agent) shellStopTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if a.shellSession == nil {
		return "no persistent shell session is active", nil
	}

	if err := a.shellSession.Close(); err != nil {
		log.Warn("Failed to close shell session: %v", err)
		return "", fmt.Errorf("failed to close shell session: %w", err)
	}

	a.shellSession = nil

	return "persistent shell session closed successfully", nil
}
