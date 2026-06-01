// Author: L.Shuang
// Created: 2026-05-28
// Last Modified: 2026-06-01
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

const DefaultMaxLineLen = 4096

type Session struct {
	mu         sync.Mutex
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	started    bool
	closed     bool
	shellType  string
	workingDir string

	lineCh   chan string
	readErr  chan error
	stopRead chan struct{}

	logFile       *os.File
	logFilePath   string
	maxLineLen    int
	outputPointer int64

	vt *VirtualTerminal
}

type Status struct {
	Running     bool   `json:"running"`
	ShellType   string `json:"shell_type"`
	WorkingDir  string `json:"working_dir"`
	StartedAt   string `json:"started_at"`
	LogFilePath string `json:"log_file_path"`
	LogFileSize int64  `json:"log_file_size"`
}

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

	time.Sleep(300 * time.Millisecond)
	s.drainLines()

	if s.vt == nil {
		s.vt = NewVirtualTerminal(DefaultVTRows, DefaultVTCols)
	} else {
		r, c := s.vt.Size()
		if r <= 0 || c <= 0 {
			s.vt.Resize(DefaultVTRows, DefaultVTCols)
		}
	}

	// Register VT output channels: lineCh for Exec idle detection,
	// logFile for shell log recording.
	s.vt.SetLineChannel(s.lineCh)
	if s.logFile != nil {
		s.vt.SetLogWriter(s.logFile)
	}

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

// readLines reads raw stdout and feeds VT.
func (s *Session) readLines() {
	buf := make([]byte, 4096)
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
		if s.vt != nil {
			s.vt.Process(buf[:n])
		}
	}
}

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

	s.drainLines()

	if _, err := fmt.Fprint(s.stdin, command); err != nil {
		return "", fmt.Errorf("cannot send command to shell: %w", err)
	}

	log.Debug("Shell exec: command=%q, wait_ms=%d", command, waitMs)

	var outputBuf bytes.Buffer
	dur := time.Duration(waitMs) * time.Millisecond
	idleTimer := time.NewTimer(dur)
	defer idleTimer.Stop()

	if !idleTimer.Stop() {
		select {
		case <-idleTimer.C:
		default:
		}
	}

	for {
		idleTimer.Reset(dur)
		select {
		case data := <-s.lineCh:
			outputBuf.WriteString(data)
		case err := <-s.readErr:
			s.started = false
			s.updateOutputPointer()
			return outputBuf.String(), fmt.Errorf("shell process terminated: %w", err)
		case <-idleTimer.C:
			s.updateOutputPointer()
			if s.vt != nil {
				return s.vt.Render(), nil
			}
			return outputBuf.String(), nil
		case <-ctx.Done():
			s.updateOutputPointer()
			return outputBuf.String(), fmt.Errorf("shell command timed out: %w", ctx.Err())
		}
	}
}

func (s *Session) updateOutputPointer() {
	if s.logFile != nil {
		if fi, err := s.logFile.Stat(); err == nil {
			s.outputPointer = fi.Size()
		}
	}
}

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

func (s *Session) SetVT(rows, cols int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rows <= 0 {
		rows = DefaultVTRows
	}
	if cols <= 0 {
		cols = DefaultVTCols
	}
	if s.started && s.vt != nil {
		s.vt.Resize(rows, cols)
	} else {
		s.vt = NewVirtualTerminal(rows, cols)
	}
}

func (s *Session) GetWindowContent() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.vt == nil {
		return "", fmt.Errorf("virtual terminal is not available")
	}
	return s.vt.Render(), nil
}

func (s *Session) GetVTSize() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.vt == nil {
		return 0, 0
	}
	return s.vt.Size()
}

func (s *Session) SetVTSize(rows, cols int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.vt == nil {
		return
	}
	if rows <= 0 {
		rows = DefaultVTRows
	}
	if cols <= 0 {
		cols = DefaultVTCols
	}
	s.vt.Resize(rows, cols)
}

func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started || s.closed {
		return nil
	}
	s.closed = true
	close(s.stopRead)

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
