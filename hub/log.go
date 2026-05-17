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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogLevel defines the minimum log level to output.
type LogLevel int

const (
	// LogLevelDebug: all log messages are output.
	LogLevelDebug LogLevel = iota
	// LogLevelInfo: INFO, WARN, ERROR messages are output.
	LogLevelInfo
	// LogLevelWarn: WARN, ERROR messages are output.
	LogLevelWarn
	// LogLevelError: only ERROR messages are output.
	LogLevelError
	// LogLevelOff: no log messages are output.
	LogLevelOff
)

// EventType categorizes hub events for structured logging.
type EventType string

const (
	EventSystem    EventType = "SYSTEM"
	EventHandshake EventType = "HANDSHAKE"
	EventMessage   EventType = "MESSAGE"
	EventCommand   EventType = "COMMAND"
	EventAgent     EventType = "AGENT"
	EventSecurity  EventType = "SECURITY"
)

// HubLogger provides structured logging for co-shell-hub.
type HubLogger struct {
	mu      sync.Mutex
	writer  io.WriteCloser
	enabled bool
	level   LogLevel
	logDir  string
}

var (
	hubLogger   *HubLogger
	hubLoggerMu sync.Mutex
)

// InitHubLogger initializes the hub logger.
// Log files are created in the specified logDir with the name hub-YYYY-MM-DD.log.
func InitHubLogger(logDir string, enabled bool, level LogLevel) error {
	hubLoggerMu.Lock()
	defer hubLoggerMu.Unlock()

	hubLogger = &HubLogger{
		enabled: enabled,
		level:   level,
		logDir:  logDir,
	}

	if enabled {
		return hubLogger.openFile()
	}
	return nil
}

// openFile creates or opens the log file for writing.
func (l *HubLogger) openFile() error {
	if l.logDir == "" {
		l.logDir = "log"
	}

	if err := os.MkdirAll(l.logDir, 0755); err != nil {
		return fmt.Errorf("cannot create log directory %s: %w", l.logDir, err)
	}

	date := time.Now().Format("2006-01-02")
	path := filepath.Join(l.logDir, fmt.Sprintf("hub-%s.log", date))

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("cannot open log file %s: %w", path, err)
	}

	l.writer = f
	l.write(EventSystem, "INFO", "hub logger initialized (log file: %s)", path)
	return nil
}

// levelToLogLevel converts a level string to LogLevel for filtering.
func levelToLogLevel(level string) LogLevel {
	switch level {
	case "DEBUG":
		return LogLevelDebug
	case "INFO":
		return LogLevelInfo
	case "WARN":
		return LogLevelWarn
	case "ERROR":
		return LogLevelError
	default:
		return LogLevelInfo
	}
}

// write writes a formatted log entry to the file.
func (l *HubLogger) write(event EventType, level, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.enabled || l.writer == nil {
		return
	}

	if levelToLogLevel(level) < l.level {
		return
	}

	now := time.Now().Format("2006-01-02 15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.writer, "[%s] [%s] [%s] %s\n", now, level, event, msg)
}

// Log writes a structured log entry with event type, level, and message.
func Log(event EventType, level, format string, args ...interface{}) {
	hubLoggerMu.Lock()
	l := hubLogger
	hubLoggerMu.Unlock()

	if l == nil {
		return
	}
	l.write(event, level, format, args...)
}

// LogHandshake logs a handshake event from a remote address.
func LogHandshake(remoteAddr string, success bool, detail string) {
	level := "INFO"
	event := EventHandshake
	if !success {
		level = "WARN"
		event = EventSecurity
	}
	Log(event, level, "[%s] handshake %s: %s", remoteAddr, map[bool]string{true: "success", false: "failed"}[success], detail)
}

// LogMessage logs a message event from a remote address.
func LogMessage(remoteAddr, agentID, content string) {
	// Truncate content for logging
	maxLen := 100
	if len(content) > maxLen {
		content = content[:maxLen] + "..."
	}
	Log(EventMessage, "INFO", "[%s] agent=%s content=%s", remoteAddr, agentID, content)
}

// LogCommand logs a command event from a remote address.
func LogCommand(remoteAddr, command string, detail string) {
	Log(EventCommand, "INFO", "[%s] command=%s %s", remoteAddr, command, detail)
}

// LogAgent logs an agent lifecycle event.
func LogAgent(remoteAddr, agentID, action string) {
	Log(EventAgent, "INFO", "[%s] agent=%s action=%s", remoteAddr, agentID, action)
}

// LogSecurity logs a security-related event (invalid requests, auth failures, etc.).
func LogSecurity(remoteAddr, reason string) {
	Log(EventSecurity, "WARN", "[%s] %s", remoteAddr, reason)
}

// LogError logs an error event.
func LogError(event EventType, remoteAddr, message string) {
	Log(event, "ERROR", "[%s] %s", remoteAddr, message)
}

// CloseHubLogger closes the log file.
func CloseHubLogger() {
	hubLoggerMu.Lock()
	defer hubLoggerMu.Unlock()

	if hubLogger == nil {
		return
	}

	hubLogger.mu.Lock()
	defer hubLogger.mu.Unlock()

	if hubLogger.writer != nil {
		hubLogger.writer.Close()
		hubLogger.writer = nil
	}
}
