// Author: L.Shuang
// Created: 2026-05-17
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

package hub

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// Start starts the co-shell agent process.
func (a *AgentSession) Start(coShellPath, workspace string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isRunning {
		return nil
	}

	// Build command arguments
	args := []string{
		"-w", workspace,
		"--name", a.config.NameFlag,
	}

	// Add config path if specified
	if a.config.ConfigPath != "" {
		args = append(args, "-c", a.config.ConfigPath)
	}

	cmd := exec.Command(coShellPath, args...)
	cmd.Dir = workspace

	// Get stdio
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Create context for process management
	ctx, cancel := context.WithCancel(context.Background())
	a.ctx = ctx
	a.cancel = cancel

	// Start command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start co-shell: %w", err)
	}

	a.cmd = cmd
	a.stdin = stdin
	a.stdout = stdout
	a.stderr = stderr
	a.isRunning = true

	// Start reading output in background
	go a.readOutput()

	// Start reading stderr
	go a.readStderr(stderr)

	return nil
}

// Stop stops the co-shell agent process.
func (a *AgentSession) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isRunning {
		return
	}

	a.cancel()

	// Close stdin
	if a.stdin != nil {
		a.stdin.Close()
	}

	// Wait for process to exit
	if a.cmd != nil && a.cmd.Process != nil {
		a.cmd.Wait()
	}

	a.isRunning = false
}

// IsRunning returns whether the agent is currently running.
func (a *AgentSession) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.isRunning
}

// Send sends a message to the agent's stdin.
func (a *AgentSession) Send(message string) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !a.isRunning || a.stdin == nil {
		return fmt.Errorf("agent is not running")
	}

	_, err := fmt.Fprintln(a.stdin, message)
	return err
}

// ReadResponse reads a response from the agent's stdout with a timeout.
func (a *AgentSession) ReadResponse(timeout time.Duration) (string, error) {
	a.mu.RLock()
	stdout := a.stdout
	a.mu.RUnlock()

	if stdout == nil {
		return "", fmt.Errorf("agent stdout is not available")
	}

	reader := bufio.NewReader(stdout)

	done := make(chan string, 1)

	go func() {
		var response strings.Builder
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			response.WriteString(line)
		}
		done <- response.String()
	}()

	select {
	case result := <-done:
		return result, nil
	case <-time.After(timeout):
		return "", fmt.Errorf("timeout waiting for agent response")
	}
}

// readOutput reads output from the agent's stdout.
func (a *AgentSession) readOutput() {
	if a.stdout == nil {
		return
	}

	scanner := bufio.NewScanner(a.stdout)
	for scanner.Scan() {
		text := scanner.Text()
		if strings.TrimSpace(text) != "" {
			// Output could be sent to connected clients
		}
	}

	if err := scanner.Err(); err != nil {
		// Handle error
	}
}

// readStderr reads output from the agent's stderr.
func (a *AgentSession) readStderr(stderr io.ReadCloser) {
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		text := scanner.Text()
		_ = text // Log stderr output
	}
}
