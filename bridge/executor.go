// Author: L.Shuang
// Created: 2026-05-04
// Last Modified: 2026-05-04
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

package bridge

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// Executor manages co-shell subprocess execution.
type Executor struct {
	CoShellPath string
	Workspace   string
	ConfigPath  string
	Timeout     time.Duration
}

// Execute runs co-shell with the given instruction and returns the output.
func (e *Executor) Execute(instruction string) (string, error) {
	args := []string{"-c", instruction, "-w", e.Workspace}
	if e.ConfigPath != "" {
		args = append(args, "-c", e.ConfigPath)
	}

	timeout := e.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, e.CoShellPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return string(output), fmt.Errorf("execution timed out after %v", timeout)
		}
		return string(output), fmt.Errorf("execution failed: %w", err)
	}
	return string(output), nil
}

// ExecuteWithCancel runs co-shell with a cancellable context and returns the output.
// The caller can use the cancel function to interrupt the process (preempt mode).
func (e *Executor) ExecuteWithCancel(ctx context.Context, instruction string) (string, error) {
	args := []string{"-c", instruction, "-w", e.Workspace}
	if e.ConfigPath != "" {
		args = append(args, "-c", e.ConfigPath)
	}

	timeout := e.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, e.CoShellPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return string(output), fmt.Errorf("execution timed out after %v", timeout)
		}
		return string(output), fmt.Errorf("execution failed: %w", err)
	}
	return string(output), nil
}
