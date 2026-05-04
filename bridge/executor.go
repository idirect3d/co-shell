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
// furnished to go, subject to the following conditions:
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
	"io"
	"log"
	"os/exec"
	"strings"
	"time"
)

// Executor manages co-shell subprocess execution.
type Executor struct {
	CoShellPath string
	Workspace   string
	ConfigPath  string
	Timeout     time.Duration
}

// buildArgs builds the command-line arguments for co-shell.
func (e *Executor) buildArgs(instruction string) []string {
	return []string{instruction}
}

// getTimeout returns the effective timeout duration.
func (e *Executor) getTimeout() time.Duration {
	if e.Timeout > 0 {
		return e.Timeout
	}
	return 120 * time.Second
}

// inputPromptPatterns are patterns that indicate co-shell is waiting for user input.
var inputPromptPatterns = []string{
	"请选择 (Enter/C/A):",
	"请选择 (Enter/C/A): ",
	"请选择操作:",
	"是否执行以下命令？",
	"是否执行命令？",
}

// isWaitingForInput checks if the output indicates co-shell is waiting for user input.
func isWaitingForInput(output string) bool {
	lower := strings.ToLower(output)
	for _, pattern := range inputPromptPatterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// newCommand creates a new exec.Cmd for co-shell, configured to run in the workspace directory.
func (e *Executor) newCommand(ctx context.Context, instruction string) *exec.Cmd {
	args := e.buildArgs(instruction)
	cmd := exec.CommandContext(ctx, e.CoShellPath, args...)
	if e.Workspace != "" {
		cmd.Dir = e.Workspace
	}
	return cmd
}

// Execute runs co-shell with the given instruction and returns the output.
func (e *Executor) Execute(instruction string) (string, error) {
	log.Printf("Executing: %s %v (workspace=%s)", e.CoShellPath, e.buildArgs(instruction), e.Workspace)

	timeout := e.getTimeout()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := e.newCommand(ctx, instruction)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return string(output), fmt.Errorf("execution timed out after %v", timeout)
		}
		return string(output), fmt.Errorf("execution failed: %w", err)
	}
	return string(output), nil
}

// ExecuteWithCancel runs co-shell with a cancellable context.
func (e *Executor) ExecuteWithCancel(ctx context.Context, instruction string) (string, error) {
	log.Printf("Executing (cancellable): %s %v (workspace=%s)", e.CoShellPath, e.buildArgs(instruction), e.Workspace)

	timeout := e.getTimeout()
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := e.newCommand(execCtx, instruction)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return string(output), fmt.Errorf("execution timed out after %v", timeout)
		}
		return string(output), fmt.Errorf("execution failed: %w", err)
	}
	return string(output), nil
}

// ExecuteInteractive runs co-shell with interactive stdin/stdout support.
// When co-shell prompts for user input, it calls inputRequestFunc to get the input.
// The ctx parameter allows external cancellation (e.g., Ctrl+C in bridge).
// Interactive mode uses a longer timeout (30 min) because user input via Feishu
// may take a long time. The external ctx cancellation (Ctrl+C) still works.
func (e *Executor) ExecuteInteractive(ctx context.Context, instruction string, inputRequestFunc func(currentOutput string) <-chan string) (string, error) {
	args := e.buildArgs(instruction)
	log.Printf("Executing interactive: %s %v (workspace=%s)", e.CoShellPath, args, e.Workspace)

	// Show the co-shell command being executed
	fmt.Printf("🔧 运行: %s %s\n", e.CoShellPath, strings.Join(args, " "))
	fmt.Println()

	// Interactive mode uses a much longer timeout (30 min) because the user
	// may need time to respond via Feishu. The external ctx (Ctrl+C) still
	// provides cancellation.
	timeout := 30 * time.Minute
	execCtx, execCancel := context.WithTimeout(ctx, timeout)
	defer execCancel()

	cmd := e.newCommand(execCtx, instruction)

	// Create pipes for stdin and stdout
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("cannot create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("cannot create stdout pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("cannot start co-shell: %w", err)
	}

	// Read stdout in chunks
	outputCh := make(chan string, 100)
	errCh := make(chan error, 1)

	go func() {
		defer close(outputCh)
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				outputCh <- string(buf[:n])
			}
			if err != nil {
				if err != io.EOF {
					errCh <- err
				}
				return
			}
		}
	}()

	// Process output and handle input requests
	var finalOutput strings.Builder
	var inputRequested bool
	var pendingOutput strings.Builder

	// idleTimer detects when co-shell has stopped producing output but is still running.
	// This is a fallback for when isWaitingForInput doesn't catch the prompt.
	const idleThreshold = 500 * time.Millisecond
	idleTimer := time.NewTimer(0)

	// Drain the timer channel initially
	if !idleTimer.Stop() {
		<-idleTimer.C
	}

	hasOutput := false

loop:
	for {
		select {
		case chunk, ok := <-outputCh:
			if !ok {
				break loop
			}
			hasOutput = true
			finalOutput.WriteString(chunk)
			pendingOutput.WriteString(chunk)

			// Check if co-shell is waiting for input by pattern matching
			currentOutput := pendingOutput.String()
			if !inputRequested && isWaitingForInput(currentOutput) && inputRequestFunc != nil {
				inputRequested = true
				idleTimer.Stop()

				inputCh := inputRequestFunc(currentOutput)
				if inputCh != nil {
					select {
					case userInput, ok := <-inputCh:
						if ok && userInput != "" {
							fmt.Fprintf(stdin, "%s\n", userInput)
							pendingOutput.Reset()
							inputRequested = false
						} else {
							fmt.Fprintf(stdin, "\n")
							pendingOutput.Reset()
							inputRequested = false
						}
					case <-execCtx.Done():
						break loop
					}
				}
			} else if !inputRequested {
				// Reset idle timer on each chunk
				idleTimer.Reset(idleThreshold)
			}

		case <-idleTimer.C:
			// No output for idleThreshold - co-shell might be waiting for input
			// that we didn't detect via pattern matching.
			if !inputRequested && hasOutput && inputRequestFunc != nil {
				currentOutput := pendingOutput.String()
				if currentOutput != "" {
					inputRequested = true

					inputCh := inputRequestFunc(currentOutput)
					if inputCh != nil {
						select {
						case userInput, ok := <-inputCh:
							if ok && userInput != "" {
								fmt.Fprintf(stdin, "%s\n", userInput)
								pendingOutput.Reset()
								inputRequested = false
							} else {
								fmt.Fprintf(stdin, "\n")
								pendingOutput.Reset()
								inputRequested = false
							}
						case <-execCtx.Done():
							break loop
						}
					}
				}
			}

		case err := <-errCh:
			log.Printf("Error reading stdout: %v", err)
			break loop

		case <-execCtx.Done():
			if execCtx.Err() == context.DeadlineExceeded {
				return finalOutput.String(), fmt.Errorf("execution timed out after %v", timeout)
			}
			// External cancellation (e.g., Ctrl+C) - kill the subprocess
			if cmd.Process != nil {
				log.Printf("Context cancelled, killing co-shell subprocess (pid=%d)", cmd.Process.Pid)
				cmd.Process.Kill()
			}
			break loop
		}
	}

	idleTimer.Stop()
	stdin.Close()

	waitErr := cmd.Wait()

	// Read any remaining output
	for chunk := range outputCh {
		finalOutput.WriteString(chunk)
	}

	if waitErr != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return finalOutput.String(), fmt.Errorf("execution timed out after %v", timeout)
		}
		return finalOutput.String(), fmt.Errorf("execution failed: %w", waitErr)
	}

	return finalOutput.String(), nil
}
