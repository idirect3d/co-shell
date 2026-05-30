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
	marker = "COSHELL__CMD_END_MARKER_1748489600"
)

// DefaultMaxLineLen is the default maximum characters per line in scrollback output.
const DefaultMaxLineLen = 4096

// Session represents a persistent interactive shell session.
// A single background goroutine reads all stdout data and pushes lines
// into a channel. Each Exec() call reads from this channel until it
// finds the end marker. This avoids goroutine leaks and data races
// that occur with per-call buffered readers.
type Session struct {
	mu         sync.Mutex
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	started    bool
	closed     bool
	shellType  string
	workingDir string

	// lineCh carries lines read from stdout by the background reader goroutine.
	// Each Exec() call reads from this channel until marker found.
	lineCh chan string
	// readErr carries reader errors from the background goroutine.
	readErr chan error
	// stopRead signals the background reader goroutine to stop.
	stopRead chan struct{}

	logFile     *os.File
	logFilePath string
	maxLineLen  int
}

// Status represents the current state of the shell session.
type Status struct {
	Running     bool   `json:"running"`
	ShellType   string `json:"shell_type"`
	WorkingDir  string `json:"working_dir"`
	StartedAt   string `json:"started_at"`
	LogFilePath string `json:"log_file_path"`
	LogFileSize int64  `json:"log_file_size"`
}

// Start initializes a new persistent shell session.
func (s *Session) Start() (*Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return nil, fmt.Errorf("shell session already started")
	}

	var shellPath string
	var shellArgs []string

	if runtime.GOOS == "windows" {
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
		// `script -q /dev/null zsh` allocates a PTY for line-buffered output.
		shellPath = "/usr/bin/script"
		shellArgs = []string{"-q", "/dev/null"}
		if _, err := exec.LookPath("zsh"); err == nil {
			shellArgs = append(shellArgs, "zsh")
			s.shellType = "zsh"
		} else {
			shellArgs = append(shellArgs, "bash")
			s.shellType = "bash"
		}
	}

	cmd := exec.Command(shellPath, shellArgs...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("cannot create stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("cannot create stdout pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout

	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = sysProcAttr()
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("cannot start shell: %w", err)
	}

	s.cmd = cmd
	s.stdin = stdin
	s.stdout = stdout
	s.lineCh = make(chan string, 1000)
	s.readErr = make(chan error, 1)
	s.stopRead = make(chan struct{})

	// Start persistent background line reader.
	// ONE goroutine for the entire session lifetime.
	go s.readLines()

	wd, _ := os.Getwd()
	s.workingDir = wd

	logDir := filepath.Join(wd, "log", "shell")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Warn("Cannot create shell log directory %s: %v", logDir, err)
		logDir = wd
	}

	now := time.Now()
	logFileName := fmt.Sprintf("shell_%s_%s.log", now.Format("20060102_150405"), s.shellType)
	logFilePath := filepath.Join(logDir, logFileName)

	logFile, err := os.Create(logFilePath)
	if err != nil {
		log.Warn("Cannot create shell log file %s: %v", logFilePath, err)
	} else {
		s.logFile = logFile
		s.logFilePath = logFilePath
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

	if runtime.GOOS != "windows" {
		s.sendRaw("PS1='$ '\n")
		s.sendRaw("export PS1='$ '\n")
		s.sendRaw("alias ls='ls --color=never 2>/dev/null || ls'\n")
	}

	time.Sleep(150 * time.Millisecond)
	// Drain initial output from the background reader by reading
	// from lineCh briefly. Do NOT call drainOutput() which starts
	// a competing goroutine reading the same s.stdout pipe.
	s.drainLines()

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

// readLines is a persistent goroutine that reads lines from stdout
// and pushes them into lineCh. It runs for the entire session lifetime.
// stdin/stderr write to logfile.
func (s *Session) readLines() {
	buf := make([]byte, 1)
	var lineBuf bytes.Buffer

	for {
		select {
		case <-s.stopRead:
			return
		default:
		}

		// Read one byte at a time
		n, err := s.stdout.Read(buf)
		if err != nil {
			if err != io.EOF {
				s.readErr <- err
			}
			return
		}

		if n == 0 {
			continue
		}

		log.Raw("%c", buf[0])

		lineBuf.WriteByte(buf[0])

		// When we hit newline, push the complete line
		if buf[0] == '\n' {
			line := lineBuf.String()
			log.Debug("shell readLines: complete line=%q", line)
			lineBuf.Reset()
			s.lineCh <- line
		}
	}
}

func (s *Session) writeLog(line string) {
	if s.logFile != nil {
		fmt.Fprintln(s.logFile, line)
	}
}

// Exec sends a command and reads output until the marker.
// Uses the persistent background reader to avoid goroutine leaks.
func (s *Session) Exec(ctx context.Context, command string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started || s.closed {
		return "", fmt.Errorf("shell session is not running")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	fullCmd := fmt.Sprintf("%s\necho \"%s\" $?\n", command, marker)
	if strings.Contains(command, "\n") {
		fullCmd = fmt.Sprintf("%s\necho \"%s\" $?\n", command, marker)
	}

	s.writeLog("$ " + command)

	if _, err := fmt.Fprint(s.stdin, fullCmd); err != nil {
		return "", fmt.Errorf("cannot send command to shell: %w", err)
	}

	log.Debug("Shell exec: %s", command)

	var outputBuf bytes.Buffer

	for {
		var line string
		var err error

		select {
		case line = <-s.lineCh:
		case err = <-s.readErr:
			s.started = false
			partial := outputBuf.String()
			s.writeLog(partial)
			return partial, fmt.Errorf("shell process terminated: %w", err)
		case <-ctx.Done():
			partial := outputBuf.String()
			s.writeLog(partial)
			return partial, fmt.Errorf("shell command timed out: %w", ctx.Err())
		}

		markerIdx := strings.Index(line, marker)
		if markerIdx >= 0 {
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

		trimmed := strings.TrimRight(line, "\n\r")
		if trimmed != "" {
			s.writeLog(trimmed)
		}
		outputBuf.WriteString(line)
	}
}

// GetOutput reads lines from the end of the shell log file.
func (s *Session) GetOutput(lastFrom int, count int) (string, int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.logFilePath == "" {
		return "log file not available", 0, 0
	}

	data, err := os.ReadFile(s.logFilePath)
	if err != nil {
		return fmt.Sprintf("cannot read log file: %v", err), 0, 0
	}

	allLines := strings.Split(string(data), "\n")
	if len(allLines) > 0 && allLines[len(allLines)-1] == "" {
		allLines = allLines[:len(allLines)-1]
	}
	totalLines := len(allLines)

	if totalLines == 0 {
		return "(empty)", 0, 0
	}

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

func (s *Session) SetMaxLineLen(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n <= 0 {
		n = DefaultMaxLineLen
	}
	s.maxLineLen = n
}

// Close terminates the shell session gracefully.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started || s.closed {
		return nil
	}
	s.closed = true

	// Stop the background reader goroutine
	close(s.stopRead)

	s.writeLog("============================")
	s.writeLog("=== Shell Session Closed ===")

	if s.logFile != nil {
		s.logFile.Close()
		s.logFile = nil
	}

	if s.stdin != nil {
		fmt.Fprint(s.stdin, "exit\n")
		time.Sleep(100 * time.Millisecond)
	}

	if s.cmd != nil && s.cmd.Process != nil {
		if err := killProcess(s.cmd); err != nil {
			log.Warn("Failed to kill shell process: %v", err)
		}
	}

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

func (s *Session) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.started && !s.closed
}

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

func (s *Session) LogFilePath() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.logFilePath
}

func (s *Session) sendRaw(text string) {
	if s.stdin != nil {
		fmt.Fprint(s.stdin, text)
	}
}

// drainLines reads and discards any pending lines from the background reader.
// This replaces the old drainOutput() which spawned a competing goroutine.
func (s *Session) drainLines() {
	if s.lineCh == nil {
		return
	}
	for {
		select {
		case <-s.lineCh:
		case <-time.After(50 * time.Millisecond):
			return
		}
	}
}

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
	case <-time.After(300 * time.Millisecond):
	}
}
