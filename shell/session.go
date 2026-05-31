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

// DefaultMaxLineLen is the default maximum characters per line in scrollback output.
const DefaultMaxLineLen = 4096

// Session represents a persistent interactive shell session.
// A single background goroutine reads all stdout data and pushes lines
// into a channel. Each Exec() call reads from this channel until an idle
// timeout expires (no new output for wait_ms) or the total timeout expires.
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
	// Each Exec() call reads from this channel until idle timeout.
	lineCh chan string
	// readErr carries reader errors from the background goroutine.
	readErr chan error
	// stopRead signals the background reader goroutine to stop.
	stopRead chan struct{}

	logFile     *os.File
	logFilePath string
	maxLineLen  int

	// outputPointer tracks the byte offset in logFile of the last output returned
	// to the caller. Used by GetOutput() for auto-increment mode: when called
	// without explicit last_from/count, it returns only new content since the last call.
	outputPointer int64
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
		s.sendRaw("stty -echo 2>/dev/null\n")
		s.sendRaw("export PS1='$ '\n")
		s.sendRaw("alias ls='ls --color=never 2>/dev/null || ls'\n")
	}

	// Drain initial output from the background reader by reading
	// from lineCh briefly.
	time.Sleep(200 * time.Millisecond)
	s.drainLines()

	// Initialise output pointer to current end of log file.
	if s.logFile != nil {
		if fi, err := s.logFile.Stat(); err == nil {
			s.outputPointer = fi.Size()
		}
	}

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
//
// All output is passed through faithfully; no filtering, no ANSI stripping,
// no prompt removal. The caller (Exec) decides when output is complete
// based on idle timeout.
func (s *Session) readLines() {
	buf := make([]byte, 4096)
	var leftover []byte

	for {
		select {
		case <-s.stopRead:
			return
		default:
		}

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

		data := buf[:n]
		if leftover != nil {
			data = append(leftover, data...)
			leftover = nil
		}

		// Strip ANSI control codes for both log file and LLM output.
		cleanData := stripLogANSI(string(data))

		// Write to log file (clean)
		if s.logFile != nil {
			fmt.Fprint(s.logFile, cleanData)
		}

		// Split on newlines and push each complete line
		cleanBytes := []byte(cleanData)
		for len(cleanBytes) > 0 {
			idx := bytes.IndexByte(cleanBytes, '\n')
			if idx < 0 {
				// Partial line without newline - buffer for next read
				leftover = make([]byte, len(cleanBytes))
				copy(leftover, cleanBytes)
				break
			}
			// Include the newline in the line pushed to channel
			line := cleanBytes[:idx+1]
			cleanBytes = cleanBytes[idx+1:]
			s.lineCh <- string(line)
		}
	}
}

// Exec sends content to the shell session and observes the output.
// Uses an idle timeout mechanism: after each new output line arrives,
// a timer is reset for wait_ms. If no new output arrives within that
// window, the accumulated output is returned.
//
// The command is sent verbatim to the shell's stdin — the LLM is
// responsible for including any necessary newline (\n) in the command.
//
// If waitMs is 0, a default of 500ms is used.
// If ctx has a deadline (via timeout_seconds), it serves as the total
// timeout — after which whatever has been collected is returned.
func (s *Session) Exec(ctx context.Context, command string, waitMs int) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started || s.closed {
		return "", fmt.Errorf("shell session is not running")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if waitMs <= 0 {
		waitMs = 500
	}

	// Write the command to stdin (PTY will echo it back).
	if _, err := fmt.Fprint(s.stdin, command); err != nil {
		return "", fmt.Errorf("cannot send command to shell: %w", err)
	}

	log.Debug("Shell exec: command=%q, wait_ms=%d", command, waitMs)

	var outputBuf bytes.Buffer
	dur := time.Duration(waitMs) * time.Millisecond
	idleTimer := time.NewTimer(dur)
	defer idleTimer.Stop()

	// Drain the timer channel initially to prevent a stale fire on first select.
	if !idleTimer.Stop() {
		select {
		case <-idleTimer.C:
		default:
		}
	}

	for {
		// Reset idle timer: wait_ms after the last output line received.
		idleTimer.Reset(dur)

		select {
		case line := <-s.lineCh:
			outputBuf.WriteString(line)

		case err := <-s.readErr:
			s.started = false
			s.updateOutputPointer()
			return outputBuf.String(), fmt.Errorf("shell process terminated: %w", err)

		case <-idleTimer.C:
			// No new output for wait_ms — return what we have
			s.updateOutputPointer()
			return outputBuf.String(), nil

		case <-ctx.Done():
			// Total timeout (from context deadline) or cancellation.
			// Return collected output with a timeout error.
			s.updateOutputPointer()
			return outputBuf.String(), fmt.Errorf("shell command timed out: %w", ctx.Err())
		}
	}
}

// updateOutputPointer sets outputPointer to the current size of the log file.
func (s *Session) updateOutputPointer() {
	if s.logFile != nil {
		if fi, err := s.logFile.Stat(); err == nil {
			s.outputPointer = fi.Size()
		}
	}
}

// GetOutput reads lines from the end of the shell log file.
//
// If lastFrom is <= 0, it uses auto-increment mode: only returns content
// that has been added to the log file since the last call to Exec() or
// GetOutput(). The outputPointer is updated after each call.
//
// lastFrom is 1‑based from the end (1 = most recent line).
// count is the number of lines to return.
//
// If lastFrom > 0 and count > 0, this function works as before.
func (s *Session) GetOutput(lastFrom int, count int) (string, int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.logFilePath == "" {
		return "log file not available", 0
	}

	data, err := os.ReadFile(s.logFilePath)
	if err != nil {
		return fmt.Sprintf("cannot read log file: %v", err), 0
	}

	// Auto-increment mode: only return content since last outputPointer.
	if lastFrom <= 0 || count <= 0 {
		totalBytes := int64(len(data))
		if s.outputPointer >= totalBytes {
			s.outputPointer = totalBytes
			return "(no new output)", 0
		}
		newData := data[s.outputPointer:]
		s.outputPointer = totalBytes

		allLines := strings.Split(string(newData), "\n")
		if len(allLines) > 0 && allLines[len(allLines)-1] == "" {
			allLines = allLines[:len(allLines)-1]
		}

		if len(allLines) == 0 {
			return "(no new output)", 0
		}

		var result strings.Builder
		for _, line := range allLines {
			if s.maxLineLen > 0 && len(line) > s.maxLineLen {
				line = line[:s.maxLineLen] + fmt.Sprintf("...（被截断%d字符）", len(line)-s.maxLineLen)
			}
			result.WriteString(line)
			result.WriteString("\n")
		}

		return strings.TrimRight(result.String(), "\n"), len(allLines)
	}

	// Legacy mode: read last N lines from end.
	allLines := strings.Split(string(data), "\n")
	if len(allLines) > 0 && allLines[len(allLines)-1] == "" {
		allLines = allLines[:len(allLines)-1]
	}
	totalLines := len(allLines)

	if totalLines == 0 {
		return "(empty)", 0
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

	var result strings.Builder
	for _, line := range selected {
		if s.maxLineLen > 0 && len(line) > s.maxLineLen {
			line = line[:s.maxLineLen] + fmt.Sprintf("...（被截断%d字符）", len(line)-s.maxLineLen)
		}
		result.WriteString(line)
		result.WriteString("\n")
	}

	s.updateOutputPointer()

	return strings.TrimRight(result.String(), "\n"), totalLines
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

func (s *Session) writeLog(line string) {
	if s.logFile != nil {
		cleanLine := stripLogANSI(line)
		fmt.Fprintln(s.logFile, cleanLine)
	}
}

// stripLogANSI removes ANSI escape sequences and non-printable C0 control
// characters from a string for clean log file output.
// The following are preserved as they represent real content:
//   - \t (0x09, tab)
//   - \n (0x0a, newline)
//   - \r (0x0d, carriage return)
//
// All other control characters (0x00-0x1f) and ANSI escape sequences (ESC)
// are stripped. This keeps the log file readable while the raw data is still
// passed through to the LLM via lineCh.
func stripLogANSI(s string) string {
	var result strings.Builder
	inESC := false
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b == 0x1b {
			inESC = true
			continue
		}
		if inESC {
			// OSC sequence ends with BEL (0x07) or ST (ESC \ i.e. 0x1b 0x5c).
			if b == 0x07 {
				inESC = false
				continue
			}
			// CSI / single-letter ESC sequences end with a letter or tilde.
			if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || b == '~' {
				inESC = false
				continue
			}
			// Nested ESC — restart (handles ST as ESC + '\' ending ESC).
			if b == 0x1b {
				continue
			}
			// All other bytes inside ESC are parameters (digits, ;, etc.) — skip.
			continue
		}
		// Outside ESC: skip all C0 control characters except \t(0x09), \n(0x0a), \r(0x0d).
		if b <= 0x1f && b != '\t' && b != '\n' && b != '\r' {
			continue
		}
		result.WriteByte(b)
	}
	return result.String()
}

// drainLines reads and discards any pending lines from the background reader.
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
