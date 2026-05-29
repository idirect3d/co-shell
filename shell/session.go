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
	"path/filepath"
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

// DefaultMaxLineLen is the default maximum characters per line in scrollback output.
const DefaultMaxLineLen = 4096

// Session represents a persistent interactive shell session.
// It maintains a long-running shell process and allows sending commands
// and capturing their output within the same environment.
// All terminal I/O (commands, stdout, stderr) is recorded to a log file
// for LLM inspection via GetOutput().
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

	// logFile records all terminal I/O (commands sent + stdout/stderr)
	// for LLM scrollback inspection. Written during Exec() and read
	// by GetOutput(). Closed on Close().
	logFile     *os.File
	logFilePath string // absolute path to the log file
	maxLineLen  int    // max characters per line in output (default: DefaultMaxLineLen)
}

// Status represents the current state of the shell session.
type Status struct {
	Running     bool   `json:"running"`
	ShellType   string `json:"shell_type"`
	WorkingDir  string `json:"working_dir"`
	StartedAt   string `json:"started_at"`
	CommandSent int    `json:"command_sent"`
	ErrorCount  int    `json:"error_count"`
	LogFilePath string `json:"log_file_path"`
	LogFileSize int64  `json:"log_file_size"`
}

// Start initializes a new persistent shell session.
// It spawns a long-running shell process connected via pipes.
// Supports bash/zsh on Unix and cmd.exe on Windows.
// A timestamped log file is created in the specified log directory
// (defaults to the current working directory).
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

	// Get initial working directory for log file placement
	wd, _ := os.Getwd()
	s.workingDir = wd
	s.cwd = wd

	// Create log directory if needed (default: ./log/shell/)
	logDir := filepath.Join(wd, "log", "shell")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Warn("Cannot create shell log directory %s: %v", logDir, err)
		logDir = wd
	}

	// Generate timestamped log file name
	now := time.Now()
	logFileName := fmt.Sprintf("shell_%s_%s.log",
		now.Format("20060102_150405"),
		s.shellType)
	logFilePath := filepath.Join(logDir, logFileName)

	logFile, err := os.Create(logFilePath)
	if err != nil {
		log.Warn("Cannot create shell log file %s: %v", logFilePath, err)
	} else {
		s.logFile = logFile
		s.logFilePath = logFilePath
		// Write session header
		fmt.Fprintf(logFile, "=== Shell Session Started ===\n")
		fmt.Fprintf(logFile, "Started At: %s\n", now.Format(time.RFC3339))
		fmt.Fprintf(logFile, "Shell Type: %s\n", s.shellType)
		fmt.Fprintf(logFile, "Working Dir: %s\n", wd)
		fmt.Fprintf(logFile, "PID: %d\n", cmd.Process.Pid)
		fmt.Fprintf(logFile, "============================\n")
	}

	if s.maxLineLen <= 0 {
		s.maxLineLen = DefaultMaxLineLen
	}

	s.started = true

	// For non-Windows, configure prompt to ensure reliable output parsing
	if runtime.GOOS != "windows" {
		s.sendRaw("PS1='$ '\nexport PS1='$ '\n")
		s.sendRaw("alias ls='ls --color=never 2>/dev/null || ls'\n")
	}

	// Wait for shell to be ready
	time.Sleep(100 * time.Millisecond)
	s.drainOutput()

	startedAt := now.Format(time.RFC3339)

	log.Info("Persistent shell session started: type=%s, pid=%d, log=%s", s.shellType, cmd.Process.Pid, logFilePath)

	return &Status{
		Running:     true,
		ShellType:   s.shellType,
		WorkingDir:  s.workingDir,
		StartedAt:   startedAt,
		LogFilePath: logFilePath,
	}, nil
}

// writeLog writes a line to the shell log file.
// Must be called with s.mu held or in a safe context.
func (s *Session) writeLog(line string) {
	if s.logFile != nil {
		fmt.Fprintln(s.logFile, line)
	}
}

// Exec sends a command to the persistent shell and captures its output.
// The command is executed within the existing shell session, preserving
// all state (environment variables, current directory, etc.).
// Returns the command output or an error if the session is not running.
// All I/O is recorded to the session log file.
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
	fullCmd := fmt.Sprintf("%s\necho \"%s\" $?\n", command, marker)
	if strings.Contains(command, "\n") {
		fullCmd = fmt.Sprintf("%s\necho \"%s\" $?\n", command, marker)
	}

	// Log the command
	s.writeLog("$ " + command)

	_, err := fmt.Fprint(s.stdin, fullCmd)
	if err != nil {
		return "", fmt.Errorf("cannot send command to shell: %w", err)
	}

	log.Debug("Shell exec: %s", command)

	// Read output until we find the marker line
	var outputBuf bytes.Buffer
	reader := bufio.NewReader(s.stdout)

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
			partial := outputBuf.String()
			s.writeLog(partial)
			return partial, fmt.Errorf("shell command timed out: %w", ctx.Err())
		case result := <-resultCh:
			if result.err != nil {
				if result.err == io.EOF {
					s.started = false
					partial := outputBuf.String()
					s.writeLog(partial)
					return partial, fmt.Errorf("shell process terminated unexpectedly")
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
				output := strings.TrimRight(outputBuf.String(), "\n")
				s.writeLog(fmt.Sprintf("(exit code: %d)", exitCode))
				if exitCode != 0 {
					return output, fmt.Errorf("command exited with code %d", exitCode)
				}
				return output, nil
			}

			// Log output line and add to buffer
			trimmed := strings.TrimRight(line, "\n\r")
			s.writeLog(trimmed)
			outputBuf.WriteString(line)
		}
	}
}

// GetOutput retrieves the tail of the shell log file.
// Reads up to count lines from the end of the file (most recent).
// Each line is truncated to s.maxLineLen characters.
// Returns the output text, total number of lines in the file,
// and how many lines were truncated.
func (s *Session) GetOutput(lastFrom int, count int) (string, int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.logFile == nil || s.logFilePath == "" {
		return "shell session log file not available", 0, 0
	}

	// Reopen for reading (log file was opened for writing)
	data, err := os.ReadFile(s.logFilePath)
	if err != nil {
		return fmt.Sprintf("cannot read log file: %v", err), 0, 0
	}

	// Split into lines
	allLines := strings.Split(string(data), "\n")
	// Remove trailing empty line from final newline
	if len(allLines) > 0 && allLines[len(allLines)-1] == "" {
		allLines = allLines[:len(allLines)-1]
	}
	totalLines := len(allLines)

	if totalLines == 0 {
		return "(empty)", 0, 0
	}

	// Constrain lastFrom
	if lastFrom <= 0 || lastFrom > totalLines {
		lastFrom = totalLines
	}

	linesToRead := count
	if linesToRead <= 0 {
		linesToRead = 50
	}
	if linesToRead > lastFrom {
		linesToRead = lastFrom
	}

	startLine := totalLines - lastFrom
	selected := allLines[startLine : startLine+linesToRead]

	truncatedCount := 0
	var result strings.Builder
	for _, line := range selected {
		if s.maxLineLen > 0 && len(line) > s.maxLineLen {
			line = line[:s.maxLineLen] + fmt.Sprintf("...(被截断%d字符)", len(line)-s.maxLineLen)
			truncatedCount++
		}
		result.WriteString(line)
		result.WriteString("\n")
	}

	return strings.TrimRight(result.String(), "\n"), totalLines, truncatedCount
}

// SetMaxLineLen sets the maximum characters per line for scrollback output.
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
// The log file is closed and will no longer be appended to.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started || s.closed {
		return nil
	}

	s.closed = true

	// Write session footer to log file
	s.writeLog("============================")
	s.writeLog("=== Shell Session Closed ===")

	// Close log file
	if s.logFile != nil {
		s.logFile.Close()
		s.logFile = nil
	}

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
	log.Info("Persistent shell session closed, log saved to: %s", s.logFilePath)

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

	var fileSize int64
	if s.logFile != nil {
		if fi, err := s.logFile.Stat(); err == nil {
			fileSize = fi.Size()
		}
	}

	return &Status{
		Running:     s.started && !s.closed,
		ShellType:   s.shellType,
		WorkingDir:  s.workingDir,
		LogFilePath: s.logFilePath,
		LogFileSize: fileSize,
	}
}

// LogFilePath returns the full path to the session log file.
func (s *Session) LogFilePath() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.logFilePath
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
