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

// Package shell provides a persistent interactive shell session that maintains
// state across command executions. This enables LLM to perform sequential
// operations (like cd, environment setup, Python REPL) in a single session.
package shell

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/idirect3d/co-shell/log"
)

const (
	// marker is a unique sentinel string used to mark command output boundaries.
	// It's highly unlikely to appear in real command output.
	marker = "COSHELL__CMD_END_MARKER_1748489600"
)

// DefaultScrollbackLines is the default maximum number of lines kept in the scrollback buffer.
const DefaultScrollbackLines = 1000

// DefaultMaxLineLen is the default maximum characters per line in the scrollback buffer.
const DefaultMaxLineLen = 4096

// Session represents a persistent interactive shell session.
// It maintains a long-running shell process and allows sending commands
// and capturing their output within the same environment.
type Session struct {
	mu         sync.Mutex
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	started    bool
	closed     bool
	shellType  string // "bash", "zsh", "cmd", or "powershell"
	workingDir string // current working directory (tracked via pwd)
	cwd        string // last known working directory

	// scrollback is a ring buffer that records all terminal output
	// (stdin commands + stdout/stderr) for LLM inspection.
	scrollback      *RingBuffer
	scrollbackLines int // max lines in scrollback (default: DefaultScrollbackLines)
	maxLineLen      int // max characters per line in output (default: DefaultMaxLineLen)
}

// RingBuffer is a fixed-size ring buffer for storing output lines.
type RingBuffer struct {
	lines []string
	max   int
	pos   int // next write position
	count int // number of lines currently stored
}

// NewRingBuffer creates a ring buffer with the given maximum capacity.
func NewRingBuffer(max int) *RingBuffer {
	if max <= 0 {
		max = DefaultScrollbackLines
	}
	return &RingBuffer{
		lines: make([]string, max),
		max:   max,
	}
}

// Add appends a line to the ring buffer, overwriting oldest if full.
func (rb *RingBuffer) Add(line string) {
	rb.lines[rb.pos] = line
	rb.pos = (rb.pos + 1) % rb.max
	if rb.count < rb.max {
		rb.count++
	}
}

// Lines returns up to count lines from the buffer, starting from the oldest.
// If count <= 0 or count > stored lines, returns all stored lines.
func (rb *RingBuffer) Lines(count int) []string {
	if rb.count == 0 {
		return nil
	}
	if count <= 0 || count > rb.count {
		count = rb.count
	}
	result := make([]string, count)
	start := rb.pos - count
	if start < 0 {
		// Wrapped around: first read from end of array, then from beginning
		firstPart := rb.lines[rb.max+start : rb.max]
		secondPart := rb.lines[:rb.pos]
		result = append(firstPart, secondPart...)
	} else {
		copy(result, rb.lines[start:rb.pos])
	}
	return result
}

// GetLastFrom returns a slice of lines starting from lastFrom (1-based from end, 1=most recent),
// returning at most count lines. Each line is truncated to maxLineLen characters.
// Returns (lines, truncatedCount) where truncatedCount is the total number of lines
// that were truncated due to maxLineLen.
func (rb *RingBuffer) GetLastFrom(lastFrom int, count int, maxLineLen int) ([]string, int) {
	if rb.count == 0 || lastFrom <= 0 || count <= 0 {
		return nil, 0
	}

	// Constrain: lastFrom cannot exceed total stored lines
	if lastFrom > rb.count {
		lastFrom = rb.count
	}

	// Calculate start position in ring buffer (lastFrom = 1 means most recent)
	// The most recent line is at position (rb.pos - 1) mod rb.max (if count>0)
	startInRing := (rb.pos - lastFrom + rb.max) % rb.max
	linesToRead := count
	if linesToRead > lastFrom {
		linesToRead = lastFrom
	}

	result := make([]string, 0, linesToRead)
	truncatedCount := 0

	for i := 0; i < linesToRead; i++ {
		idx := (startInRing + i) % rb.max
		line := rb.lines[idx]
		if maxLineLen > 0 && len(line) > maxLineLen {
			line = line[:maxLineLen] + fmt.Sprintf("...(被截断%d字符)", len(line)-maxLineLen)
			truncatedCount++
		}
		result = append(result, line)
	}

	return result, truncatedCount
}

// Total returns the total number of lines stored in the buffer.
func (rb *RingBuffer) Total() int {
	return rb.count
}

// Clear resets the ring buffer.
func (rb *RingBuffer) Clear() {
	rb.pos = 0
	rb.count = 0
}

// Status represents the current state of the shell session.
type Status struct {
	Running          bool   `json:"running"`
	ShellType        string `json:"shell_type"`
	WorkingDir       string `json:"working_dir"`
	StartedAt        string `json:"started_at"`
	CommandSent      int    `json:"command_sent"`
	ErrorCount       int    `json:"error_count"`
	ScrollbackLines  int    `json:"scrollback_lines"`
	ScrollbackMaxLen int    `json:"scrollback_max_len"`
}

// Start initializes a new persistent shell session.
// It spawns a long-running shell process connected via pipes.
// Supports bash/zsh on Unix and cmd.exe on Windows.
// Returns the session status and any error encountered.
func (s *Session) Start() (*Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return nil, fmt.Errorf("shell session already started")
	}

	var shellPath string
	var shellArgs []string

	if runtime.GOOS == "windows" {
		// Try PowerShell first, fall back to cmd
		if _, err := exec.LookPath("powershell.exe"); err == nil {
			shellPath = "powershell.exe"
			shellArgs = []string{"-NoLogo", "-NoProfile", "-Command", "-"}
			s.shellType = "powershell"
		} else {
			shellPath = "cmd.exe"
			shellArgs = []string{"/Q"}
			s.shellType = "cmd"
		}
	} else {
		// Try zsh first, fall back to bash
		if _, err := exec.LookPath("zsh"); err == nil {
			shellPath = "zsh"
			shellArgs = []string{"-i"}
			s.shellType = "zsh"
		} else {
			shellPath = "bash"
			shellArgs = []string{"-i"}
			s.shellType = "bash"
		}
	}

	cmd := exec.Command(shellPath, shellArgs...)

	// Set up pipes for stdin/stdout
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("cannot create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("cannot create stdout pipe: %w", err)
	}

	// Merge stderr into stdout so we capture error output too
	cmd.Stderr = cmd.Stdout

	// Set PGID for process group management on Unix
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = sysProcAttr()
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("cannot start shell: %w", err)
	}

	s.cmd = cmd
	s.stdin = stdin
	s.stdout = stdout
	// Initialize scrollback buffer
	if s.scrollbackLines <= 0 {
		s.scrollbackLines = DefaultScrollbackLines
	}
	if s.maxLineLen <= 0 {
		s.maxLineLen = DefaultMaxLineLen
	}
	s.scrollback = NewRingBuffer(s.scrollbackLines)

	s.started = true

	// Get initial working directory
	wd, _ := os.Getwd()
	s.workingDir = wd
	s.cwd = wd

	// For non-Windows, configure prompt to ensure reliable output parsing
	if runtime.GOOS != "windows" {
		// Set a simple, predictable prompt to avoid ANSI/color issues
		s.sendRaw("PS1='$ '\nexport PS1='$ '\n")
		// Also disable colors and ls grouping that may cause parsing issues
		s.sendRaw("alias ls='ls --color=never 2>/dev/null || ls'\n")
	}

	// Wait for shell to be ready
	time.Sleep(100 * time.Millisecond)
	// Drain any initial output (like MOTD, prompt, etc.)
	s.drainOutput()

	startedAt := time.Now().Format(time.RFC3339)

	log.Info("Persistent shell session started: type=%s, pid=%d", s.shellType, cmd.Process.Pid)

	return &Status{
		Running:    true,
		ShellType:  s.shellType,
		WorkingDir: s.workingDir,
		StartedAt:  startedAt,
	}, nil
}

// Exec sends a command to the persistent shell and captures its output.
// The command is executed within the existing shell session, preserving
// all state (environment variables, current directory, etc.).
// Returns the command output or an error if the session is not running.
func (s *Session) Exec(ctx context.Context, command string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started || s.closed {
		return "", fmt.Errorf("shell session is not running")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	// Send the command followed by the marker
	// The marker is printed after the command output so we know when to stop reading
	fullCmd := fmt.Sprintf("%s\necho \"%s\" $?\n", command, marker)

	// For multi-line commands (like heredocs), ensure they work properly
	if strings.Contains(command, "\n") {
		fullCmd = fmt.Sprintf("%s\necho \"%s\" $?\n", command, marker)
	}

	_, err := fmt.Fprint(s.stdin, fullCmd)
	if err != nil {
		return "", fmt.Errorf("cannot send command to shell: %w", err)
	}

	log.Debug("Shell exec: %s", command)

	// Record the command to scrollback
	if s.scrollback != nil {
		s.scrollback.Add("$ " + command)
	}

	// Read output until we find the marker line
	var outputBuf bytes.Buffer
	reader := bufio.NewReader(s.stdout)

	// Use a channel to handle timeout
	type readResult struct {
		line string
		err  error
	}
	resultCh := make(chan readResult, 1)

	for {
		go func() {
			line, err := reader.ReadString('\n')
			resultCh <- readResult{line: line, err: err}
		}()

		select {
		case <-ctx.Done():
			return outputBuf.String(), fmt.Errorf("shell command timed out: %w", ctx.Err())
		case result := <-resultCh:
			if result.err != nil {
				if result.err == io.EOF {
					// Shell process died
					s.started = false
					return outputBuf.String(), fmt.Errorf("shell process terminated unexpectedly")
				}
				return outputBuf.String(), fmt.Errorf("cannot read shell output: %w", result.err)
			}

			line := result.line

			// Check for marker
			markerIdx := strings.Index(line, marker)
			if markerIdx >= 0 {
				// Extract exit code from after the marker
				exitCodeStr := strings.TrimSpace(line[markerIdx+len(marker):])
				exitCode := 0
				if exitCodeStr != "" {
					fmt.Sscanf(exitCodeStr, "%d", &exitCode)
				}
				// Remove marker line from output
				output := strings.TrimRight(outputBuf.String(), "\n")
				if exitCode != 0 {
					return output, fmt.Errorf("command exited with code %d", exitCode)
				}
				return output, nil
			}

			// Record each output line to scrollback
			if s.scrollback != nil {
				s.scrollback.Add(strings.TrimRight(line, "\n\r"))
			}

			outputBuf.WriteString(line)
		}
	}
}

// GetOutput returns the scrollback content starting from lastFrom lines from the end,
// returning up to count lines. Each line is truncated to max returns per-line character limit.
// Returns the output as a string, total available lines in scrollback, and truncated line count.
func (s *Session) GetOutput(lastFrom int, count int) (string, int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.scrollback == nil {
		return "scrollback buffer not available", 0, 0
	}

	lines, truncatedCount := s.scrollback.GetLastFrom(lastFrom, count, s.maxLineLen)

	var result strings.Builder
	for _, line := range lines {
		result.WriteString(line)
		result.WriteString("\n")
	}

	return strings.TrimRight(result.String(), "\n"), s.scrollback.Total(), truncatedCount
}

// SetScrollbackLines sets the maximum number of lines kept in the scrollback buffer.
// Only effective before Start() is called.
func (s *Session) SetScrollbackLines(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n <= 0 {
		n = DefaultScrollbackLines
	}
	s.scrollbackLines = n
}

// SetMaxLineLen sets the maximum characters per line for scrollback output.
// Only effective before Start() is called.
func (s *Session) SetMaxLineLen(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n <= 0 {
		n = DefaultMaxLineLen
	}
	s.maxLineLen = n
}

// Close terminates the shell session gracefully.
// It sends an exit command first, then kills the process if it doesn't exit.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started || s.closed {
		return nil
	}

	s.closed = true

	// Try graceful shutdown
	if s.stdin != nil {
		fmt.Fprint(s.stdin, "exit\n")
		time.Sleep(100 * time.Millisecond)
	}

	// Kill the process group
	if s.cmd != nil && s.cmd.Process != nil {
		if err := killProcess(s.cmd); err != nil {
			log.Warn("Failed to kill shell process: %v", err)
		}
	}

	// Close pipes
	if s.stdin != nil {
		s.stdin.Close()
	}
	if s.stdout != nil {
		s.stdout.Close()
	}

	s.started = false
	log.Info("Persistent shell session closed")

	return nil
}

// IsRunning returns true if the shell session is currently active.
func (s *Session) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.started && !s.closed
}

// Status returns the current status of the shell session.
func (s *Session) Status() *Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	return &Status{
		Running:    s.started && !s.closed,
		ShellType:  s.shellType,
		WorkingDir: s.workingDir,
	}
}

// sendRaw sends raw text to the shell without adding markers.
// Used for initialization commands.
func (s *Session) sendRaw(text string) {
	if s.stdin != nil {
		fmt.Fprint(s.stdin, text)
	}
}

// drainOutput reads and discards any pending output from the shell.
// Used during initialization to clear MOTD and prompt.
func (s *Session) drainOutput() {
	if s.stdout == nil {
		return
	}
	// Read with a short timeout to get any pending output
	done := make(chan struct{})
	go func() {
		buf := make([]byte, 4096)
		for {
			_, err := s.stdout.Read(buf)
			if err != nil {
				break
			}
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
	}
}
