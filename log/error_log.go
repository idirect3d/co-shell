// Package log - optional error logs (FEATURE-554).
//
// Two developer switches create dedicated, date-rotated log files under the
// workspace log/ directory:
//
//   - tool-call-error-log → log/tool-call-error-YYYY-MM-DD.log
//   - llm-call-error-log  → log/llm-call-error-YYYY-MM-DD.log
//
// Both are off by default. The file is created lazily on the first record
// written while the switch is on, so a disabled switch never creates a file.
// When the date changes, the writer rotates to the new day's file.
//
// Author: L.Shuang
// Created: 2026-09-25
// Last Modified: 2026-09-25
// MIT License - Copyright (c) 2026 L.Shuang
package log

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// errorLogSeparator visually separates consecutive records.
const errorLogSeparator = "\n" +
	"----------------------------------------------------------------------\n\n"

// errorLogger is a lazily created, date-rotated writer shared by the optional
// error logs. It is independent of the main Logger and its level filtering.
type errorLogger struct {
	mu     sync.Mutex
	on     bool
	dir    string
	prefix string
	date   string
	file   *os.File
}

// setDir sets the directory receiving the log files. Changing the directory
// closes the current file so the next record lands in the new location.
func (l *errorLogger) setDir(dir string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.dir == dir {
		return
	}
	l.dir = dir
	l.closeLocked()
}

// setEnabled turns the logger on or off. Turning it off closes the file so a
// disabled switch keeps the file untouched.
func (l *errorLogger) setEnabled(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.on = enabled
	if !enabled {
		l.closeLocked()
	}
}

// enabled reports whether the logger is currently on.
func (l *errorLogger) enabled() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.on
}

// write appends one record: a timestamped header line followed by the given
// key/value fields and a separator. It is a no-op while the switch is off.
func (l *errorLogger) write(header string, fields [][2]string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.on {
		return
	}
	now := time.Now()
	f, err := l.fileLocked(now.Format("2006-01-02"))
	if err != nil {
		return
	}
	fmt.Fprintf(f, "[%s] %s\n", now.Format("2006-01-02 15:04:05.000"), header)
	for _, kv := range fields {
		fmt.Fprintf(f, "%s: %s\n", kv[0], kv[1])
	}
	fmt.Fprint(f, errorLogSeparator)
}

// fileLocked returns the append handle for the given date, opening the file
// (or rotating to a new day's file) when needed.
func (l *errorLogger) fileLocked(date string) (*os.File, error) {
	if l.file != nil && l.date == date {
		return l.file, nil
	}
	l.closeLocked()
	if l.dir == "" {
		return nil, fmt.Errorf("error log directory is not set")
	}
	name := fmt.Sprintf("%s-%s.log", l.prefix, date)
	f, err := os.OpenFile(filepath.Join(l.dir, name), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	l.file = f
	l.date = date
	return f, nil
}

// closeLocked closes the current file, if any.
func (l *errorLogger) closeLocked() {
	if l.file != nil {
		l.file.Close()
		l.file = nil
	}
	l.date = ""
}

var (
	// toolCallErrorLog backs the tool-call-error-log switch.
	toolCallErrorLog = &errorLogger{prefix: "tool-call-error"}
	// llmCallErrorLog backs the llm-call-error-log switch.
	llmCallErrorLog = &errorLogger{prefix: "llm-call-error"}
)

// SetToolCallErrorLogDir sets the directory used for the tool call error log.
// The file itself is created on the first record, so calling this while the
// switch is off never creates a file.
func SetToolCallErrorLogDir(dir string) {
	toolCallErrorLog.setDir(dir)
}

// SetToolCallErrorEnabled enables or disables the tool call error log at runtime.
func SetToolCallErrorEnabled(enabled bool) {
	toolCallErrorLog.setEnabled(enabled)
}

// IsToolCallErrorEnabled reports whether the tool call error log is currently on.
func IsToolCallErrorEnabled() bool {
	return toolCallErrorLog.enabled()
}

// WriteToolCallError appends one tool call error record: the call time, the
// tool name, the raw tool call payload (JSON/XML) and the error text shown to
// the user by the UI.
func WriteToolCallError(toolName, rawCall, errText string) {
	toolCallErrorLog.write("tool="+toolName, [][2]string{
		{"raw_call", rawCall},
		{"error", errText},
	})
}

// CloseToolCallErrorLog closes the tool call error log file.
func CloseToolCallErrorLog() {
	toolCallErrorLog.setEnabled(false)
}

// SetLLMCallErrorLogDir sets the directory used for the LLM call error log.
// The file itself is created on the first record, so calling this while the
// switch is off never creates a file.
func SetLLMCallErrorLogDir(dir string) {
	llmCallErrorLog.setDir(dir)
}

// SetLLMCallErrorEnabled enables or disables the LLM call error log at runtime.
func SetLLMCallErrorEnabled(enabled bool) {
	llmCallErrorLog.setEnabled(enabled)
}

// IsLLMCallErrorEnabled reports whether the LLM call error log is currently on.
func IsLLMCallErrorEnabled() bool {
	return llmCallErrorLog.enabled()
}

// WriteLLMCallError appends one LLM call error record: the call time, the
// stage that failed (request / http-status / stream / decode) and the raw
// error detail returned by the remote API.
func WriteLLMCallError(stage, detail string) {
	llmCallErrorLog.write("stage="+stage, [][2]string{
		{"detail", detail},
	})
}

// CloseLLMCallErrorLog closes the LLM call error log file.
func CloseLLMCallErrorLog() {
	llmCallErrorLog.setEnabled(false)
}
