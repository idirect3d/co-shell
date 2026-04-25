// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-04-25
//
// # MIT License
//
// # Copyright (c) 2026 L.Shuang
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
package wizard

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// terminalState holds the original terminal settings.
type terminalState struct {
	settings string
}

// rawTerminal puts the terminal into raw mode and returns the old state.
// On Unix, it uses stty. On Windows, it returns an error to trigger fallback.
func rawTerminal() (*terminalState, error) {
	// Windows does not support stty; return error to trigger simple input fallback
	if runtime.GOOS == "windows" {
		return nil, fmt.Errorf("raw terminal not supported on Windows")
	}

	// Save current terminal settings
	oldState, err := exec.Command("stty", "-g").Output()
	if err != nil {
		return nil, fmt.Errorf("cannot save terminal state: %w", err)
	}

	// Set raw mode (keep opost enabled so \n is converted to \r\n)
	rawCmd := exec.Command("stty", "raw", "-echo", "-icanon", "opost", "min", "1", "time", "0")
	rawCmd.Stdin = os.Stdin
	if err := rawCmd.Run(); err != nil {
		return nil, fmt.Errorf("cannot set raw terminal: %w", err)
	}

	return &terminalState{settings: strings.TrimSpace(string(oldState))}, nil
}

// restoreTerminal restores the terminal to the given state.
// On Windows, this is a no-op since rawTerminal always fails.
func restoreTerminal(ts *terminalState) {
	if ts == nil || ts.settings == "" {
		return
	}
	restoreCmd := exec.Command("stty", ts.settings)
	restoreCmd.Stdin = os.Stdin
	restoreCmd.Run()
}

// readKey reads a single key press from raw terminal.
// Returns the key code and whether it's a special key.
func readKey() (string, bool) {
	buf := make([]byte, 3)
	n, err := os.Stdin.Read(buf)
	if err != nil || n == 0 {
		return "", false
	}

	// ESC sequence
	if buf[0] == 0x1b {
		if n == 1 {
			return "esc", true
		}
		// Arrow keys: ESC [ A/B/C/D
		if n >= 2 && buf[1] == '[' {
			switch buf[2] {
			case 'A':
				return "up", true
			case 'B':
				return "down", true
			case 'C':
				return "right", true
			case 'D':
				return "left", true
			}
		}
		return "unknown", true
	}

	// Tab key
	if buf[0] == 0x09 {
		return "tab", true
	}

	// Enter key
	if buf[0] == 0x0d || buf[0] == 0x0a {
		return "enter", true
	}

	// Backspace
	if buf[0] == 0x7f || buf[0] == 0x08 {
		return "backspace", true
	}

	// Regular character
	return string(buf[0]), false
}

// clearLine clears the current line and moves cursor to beginning.
func clearLine() {
	fmt.Print("\r\033[K")
}

// moveUp moves cursor up n lines.
func moveUp(n int) {
	fmt.Printf("\033[%dA", n)
}

// moveDown moves cursor down n lines.
func moveDown(n int) {
	fmt.Printf("\033[%dB", n)
}

// hideCursor hides the terminal cursor.
func hideCursor() {
	fmt.Print("\033[?25l")
}

// showCursor shows the terminal cursor.
func showCursor() {
	fmt.Print("\033[?25h")
}
