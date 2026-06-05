// Author: L.Shuang
// Created: 2026-06-05
// Last Modified: 2026-06-05
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

package agent

// UserIO defines the interface for user input/output operations.
// Two implementations exist:
//   - StdioIO: standard input/output (used in stdio mode)
//   - EnhancedIO: raw terminal mode with enhanced editing (used in enhanced mode)
//
// EnhancedIO sets IsReading() to true while waiting for user input, so that
// the ESC monitor goroutine (in repl/repl.go) temporarily stops polling stdin
// to avoid data races.
type UserIO interface {
	// Print/Printf/Println output text to the user.
	// EnhancedIO automatically handles \r\n conversion in raw mode.
	Print(args ...interface{})
	Printf(format string, args ...interface{})
	Println(args ...interface{})

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
