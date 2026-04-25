// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-04-25
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
package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Logger provides file-based logging for co-shell.
// Logs are written to a file in the current working directory.
type Logger struct {
	mu      sync.Mutex
	writer  io.WriteCloser
	enabled bool
}

var (
	defaultLogger *Logger
	once          sync.Once
)

// Init initializes the global logger.
// The log file is created in the current working directory with the name co-shell-YYYY-MM-DD.log.
// If enabled is false, no log file is created and all log calls are no-ops.
func Init(enabled bool) error {
	var err error
	once.Do(func() {
		defaultLogger = &Logger{enabled: enabled}
		if enabled {
			err = defaultLogger.openFile()
		}
	})
	return err
}

// openFile creates or opens the log file for writing.
func (l *Logger) openFile() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get working directory: %w", err)
	}

	filename := fmt.Sprintf("co-shell-%s.log", time.Now().Format("2006-01-02"))
	path := filepath.Join(cwd, filename)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("cannot open log file %s: %w", path, err)
	}

	l.writer = f

	// Write initial log entry
	now := time.Now().Format("2006-01-02 15:04:05.000")
	fmt.Fprintf(l.writer, "[%s] [INIT] co-shell logger initialized\n", now)

	return nil
}

// write writes a formatted log entry to the file.
func (l *Logger) write(level, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.enabled || l.writer == nil {
		return
	}

	now := time.Now().Format("2006-01-02 15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.writer, "[%s] [%s] %s\n", now, level, msg)
}

// SetEnabled enables or disables logging at runtime.
// When enabling, it opens the log file if not already open.
func SetEnabled(enabled bool) error {
	if defaultLogger == nil {
		return fmt.Errorf("logger not initialized")
	}

	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()

	if enabled == defaultLogger.enabled {
		return nil
	}

	if enabled {
		if defaultLogger.writer != nil {
			defaultLogger.writer.Close()
		}
		defaultLogger.enabled = true
		return defaultLogger.openFile()
	}

	// Disable: close the writer
	defaultLogger.enabled = false
	if defaultLogger.writer != nil {
		defaultLogger.writer.Close()
		defaultLogger.writer = nil
	}
	return nil
}

// IsEnabled returns whether logging is currently enabled.
func IsEnabled() bool {
	if defaultLogger == nil {
		return false
	}
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()
	return defaultLogger.enabled
}

// --- Public log functions ---

// Info logs an informational message.
func Info(format string, args ...interface{}) {
	if defaultLogger == nil {
		return
	}
	defaultLogger.write("INFO", format, args...)
}

// Warn logs a warning message.
func Warn(format string, args ...interface{}) {
	if defaultLogger == nil {
		return
	}
	defaultLogger.write("WARN", format, args...)
}

// Error logs an error message.
func Error(format string, args ...interface{}) {
	if defaultLogger == nil {
		return
	}
	defaultLogger.write("ERROR", format, args...)
}

// Debug logs a debug message.
func Debug(format string, args ...interface{}) {
	if defaultLogger == nil {
		return
	}
	defaultLogger.write("DEBUG", format, args...)
}

// Close closes the log file.
func Close() {
	if defaultLogger == nil {
		return
	}
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()
	if defaultLogger.writer != nil {
		defaultLogger.writer.Close()
		defaultLogger.writer = nil
	}
}
