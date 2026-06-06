// Author: L.Shuang
// Created: 2026-06-05
// Last Modified: 2026-06-06
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
// IMPLIED, BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package agent

import (
	"bufio"
	"fmt"
	"os"
)

// UserIO defines the interface for user input/output operations.
// Two implementations exist:
//   - StdioIO: standard input/output (used in stdio mode)
//   - EnhancedIO: raw terminal mode with enhanced editing (used in enhanced mode)
//
// EnhancedIO sets IsReading() to true while waiting for user input, so that
// the ESC monitor goroutine (in repl/repl.go) temporarily stops polling stdin
// to avoid data races.
type UserIO interface {
	// Print/Printf/Println output text to the user (stdout).
	// EnhancedIO automatically handles \r\n conversion in raw mode.
	Print(args ...interface{})
	Printf(format string, args ...interface{})
	Println(args ...interface{})

	// ErrPrintf outputs formatted text to the error stream (stderr).
	ErrPrintf(format string, args ...interface{})

	// ReadLine reads a line of input (terminated by Enter).
	// EnhancedIO supports full line editing (arrows, backspace, history, etc.).
	ReadLine() (string, error)

	// ReadKey reads a single key press (1 byte or escape sequence).
	// Used for single-key confirmations (Enter/C/A/G etc.).
	// EnhancedIO echoes the key to the user; StdioIO does not.
	ReadKey() (byte, error)

	// IsReading returns true while ReadLine or ReadKey is blocking on input.
	// The ESC monitor goroutine should skip polling when this is true.
	IsReading() bool
}

// DefaultUserIO implements UserIO using os.Stdout/os.Stdin/os.Stderr directly.
// Used as fallback when no enhanced/stdio REPL input mode is configured.
// This is the default UserIO for cmd handlers and main.go startup code.
type DefaultUserIO struct {
	reader *bufio.Scanner
}

// NewDefaultUserIO creates a new DefaultUserIO instance.
func NewDefaultUserIO() *DefaultUserIO {
	return &DefaultUserIO{
		reader: bufio.NewScanner(os.Stdin),
	}
}

func (d *DefaultUserIO) Print(args ...interface{}) {
	fmt.Print(args...)
}

func (d *DefaultUserIO) Printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

func (d *DefaultUserIO) Println(args ...interface{}) {
	fmt.Println(args...)
}

func (d *DefaultUserIO) ErrPrintf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
}

func (d *DefaultUserIO) ReadLine() (string, error) {
	d.reader = bufio.NewScanner(os.Stdin)
	if !d.reader.Scan() {
		return "", d.reader.Err()
	}
	return d.reader.Text(), nil
}

func (d *DefaultUserIO) ReadKey() (byte, error) {
	buf := make([]byte, 1)
	n, err := os.Stdin.Read(buf)
	if err != nil || n == 0 {
		return 0, err
	}
	return buf[0], nil
}

func (d *DefaultUserIO) IsReading() bool {
	return false
}

// Ensure DefaultUserIO implements UserIO.
var _ UserIO = (*DefaultUserIO)(nil)

// GetIO returns the UserIO from the given agent, falling back to DefaultUserIO.
// This is the recommended way for cmd handlers and other components to obtain
// a UserIO instance for user interaction.
func GetIO(a *Agent) UserIO {
	if a != nil && a.io != nil {
		return a.io
	}
	return NewDefaultUserIO()
}
