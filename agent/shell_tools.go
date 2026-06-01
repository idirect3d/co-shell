// Author: L.Shuang
// Created: 2026-05-28
// Last Modified: 2026-05-30
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

	// Create and start a new session with virtual terminal
	sess := &shell.Session{}
	// Apply VT size from config if set
	if a.cfg != nil && a.cfg.LLM.ShellVTRows > 0 && a.cfg.LLM.ShellVTCols > 0 {
		sess.SetVT(a.cfg.LLM.ShellVTRows, a.cfg.LLM.ShellVTCols)
	}
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

// shellSendTool sends content to the persistent shell session and observes the output.
// The content can be a shell command, a line of Python code, or any input for an
// interactive program running in the shell. The function waits for output to become
// idle (no new output for wait_ms milliseconds) before returning, allowing the LLM
// to observe the result of each individual input.
//
// LLMs should send content one logical unit at a time (one shell command, one Python
// statement, etc.) so they can observe the result of each input and decide what to
// send next based on the output.
func (a *Agent) shellSendTool(ctx context.Context, args map[string]interface{}) (string, error) {
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

	// Unescape escape sequences in the command (\n, \xNN, etc.)
	command = unescapeCommand(command)

	// Get wait_ms (optional, in milliseconds, default 500)
	waitMs := 200
	if w, ok := args["wait_ms"].(float64); ok && w > 0 {
		waitMs = int(w)
	}

	// Get LLM-suggested total timeout (optional)
	llmSuggested := 0
	if t, ok := args["timeout_seconds"].(float64); ok {
		llmSuggested = int(t)
	}

	// Effective timeout = max(user-configured ShellSessionTimeout, LLM-suggested)
	userMin := 0
	if a.cfg != nil {
		userMin = a.cfg.LLM.ShellSessionTimeout
	}
	effectiveTimeout := userMin
	if llmSuggested > effectiveTimeout {
		effectiveTimeout = llmSuggested
	}

	// Create context with optional timeout
	execCtx := ctx
	if effectiveTimeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, time.Duration(effectiveTimeout)*time.Second)
		defer cancel()
	}

	output, err := a.shellSession.Exec(execCtx, command, waitMs)
	if err != nil {
		log.Debug("Shell send error: %v (output: %s)", err, output)
		if output != "" {
			return fmt.Sprintf("%s\n\n⚠️ 命令执行出错：%v", output, err), nil
		}
		return "", fmt.Errorf("shell command failed: %w", err)
	}

	return output, nil
}

// shellGetOutputTool retrieves scrollback content from the shell session.
// If no last_from/count is provided, it returns only content that has been
// added since the last shell_send or shell_get_output call (auto-increment mode).
// The wait_ms parameter controls how long to wait for new output before returning.
func (a *Agent) shellGetOutputTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if !a.shellEnabled {
		return "", fmt.Errorf("persistent shell session is disabled")
	}

	if a.shellSession == nil {
		return "", fmt.Errorf("no persistent shell session is active. Use shell_start to start one first")
	}

	// Get wait_ms (optional, in milliseconds, default 200)
	waitMs := 200
	if w, ok := args["wait_ms"].(float64); ok && w > 0 {
		waitMs = int(w)
	}

	// Get last_from (optional, 1-based from end)
	lastFrom := 0
	if lf, ok := args["last_from"].(float64); ok {
		lastFrom = int(lf)
	}
	if lastFrom < 0 {
		lastFrom = 0
	}

	// Get count (optional)
	count := 0
	if c, ok := args["count"].(float64); ok {
		count = int(c)
	}
	if count < 0 {
		count = 0
	}

	// Get LLM-suggested total timeout (optional)
	llmSuggested := 0
	if t, ok := args["timeout_seconds"].(float64); ok {
		llmSuggested = int(t)
	}

	// Effective timeout = max(user-configured ShellSessionTimeout, LLM-suggested)
	userMin := 0
	if a.cfg != nil {
		userMin = a.cfg.LLM.ShellSessionTimeout
	}
	effectiveTimeout := userMin
	if llmSuggested > effectiveTimeout {
		effectiveTimeout = llmSuggested
	}

	// Create context with optional timeout
	execCtx := ctx
	if effectiveTimeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, time.Duration(effectiveTimeout)*time.Second)
		defer cancel()
	}

	// If wait_ms > 0, wait briefly for any pending output to arrive (if not timed out)
	select {
	case <-execCtx.Done():
		output, totalLines := a.shellSession.GetOutput(lastFrom, count)
		return fmt.Sprintf("终端输出（共%d行）：\n%s", totalLines, output), nil
	case <-time.After(time.Duration(waitMs) * time.Millisecond):
	}

	output, totalLines := a.shellSession.GetOutput(lastFrom, count)
	return fmt.Sprintf("终端输出（共%d行）：\n%s", totalLines, output), nil
}

// truncatedOutputSummary returns a summary of how many lines were truncated
func truncatedOutputSummary(truncatedCount int) string {
	if truncatedCount > 0 {
		return fmt.Sprintf("⚠️ 有%d行超出每行最大字符数限制被截断", truncatedCount)
	}
	return ""
}

// shellResetTool stops and restarts the shell session, resetting it to a clean state.
func (a *Agent) shellResetTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if !a.shellEnabled {
		return "", fmt.Errorf("persistent shell session is disabled")
	}

	if a.shellSession == nil {
		return "", fmt.Errorf("no active shell session to reset. It will be auto-started")
	}

	// Close and reopen session
	a.CloseShellSession()

	sess := &shell.Session{}
	if a.cfg != nil && a.cfg.LLM.ShellVTRows > 0 && a.cfg.LLM.ShellVTCols > 0 {
		sess.SetVT(a.cfg.LLM.ShellVTRows, a.cfg.LLM.ShellVTCols)
	}
	if _, err := sess.Start(); err != nil {
		return "", fmt.Errorf("failed to restart shell session: %w", err)
	}

	a.mu.Lock()
	a.shellSession = sess
	a.mu.Unlock()

	return "shell session has been reset to a clean state", nil
}

// shellWindowContentTool returns the current virtual terminal window content.
// This provides a snapshot of what the terminal currently displays, useful for
// checking the state of a long-running process or reviewing command output
// without sending a new command.
func (a *Agent) shellWindowContentTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if !a.shellEnabled {
		return "", fmt.Errorf("persistent shell session is disabled")
	}

	if a.shellSession == nil {
		return "", fmt.Errorf("no persistent shell session is active. Use shell_start to start one first")
	}

	content, err := a.shellSession.GetWindowContent()
	if err != nil {
		return "", fmt.Errorf("cannot get window content: %w", err)
	}

	rows, cols := a.shellSession.GetVTSize()

	return fmt.Sprintf("终端窗口内容（%d行 x %d列）：\n%s", rows, cols, content), nil
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

// unescapeCommand converts escape sequences in a command string to literal bytes.
// Supports \n, \r, \t, \\, and \xNN (hex) sequences.
// This allows LLMs to send control characters like \n (Enter) and \x03 (Ctrl+C)
// as human-readable escape sequences in the command string.
func unescapeCommand(s string) string {
	var result []byte
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' {
			result = append(result, s[i])
			continue
		}
		if i+1 >= len(s) {
			result = append(result, '\\')
			break
		}
		switch s[i+1] {
		case 'n':
			result = append(result, '\n')
			i++
		case 'r':
			result = append(result, '\r')
			i++
		case 't':
			result = append(result, '\t')
			i++
		case '\\':
			result = append(result, '\\')
			i++
		case 'x':
			// \xNN hex escape
			if i+3 < len(s) {
				var b byte
				for j := 0; j < 2; j++ {
					c := s[i+2+j]
					switch {
					case c >= '0' && c <= '9':
						b = b*16 + (c - '0')
					case c >= 'a' && c <= 'f':
						b = b*16 + (c - 'a' + 10)
					case c >= 'A' && c <= 'F':
						b = b*16 + (c - 'A' + 10)
					default:
						result = append(result, s[i:i+2+j]...)
						i += 1 + j
						goto next
					}
				}
				result = append(result, b)
				i += 3
			} else {
				result = append(result, s[i:]...)
				i = len(s)
			}
		default:
			result = append(result, '\\')
		}
	next:
	}
	return string(result)
}
