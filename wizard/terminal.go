// Author: L.Shuang
package wizard

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// terminalState holds the original terminal settings.
type terminalState struct {
	settings string
}

// rawTerminal puts the terminal into raw mode using stty and returns the old state.
func rawTerminal() (*terminalState, error) {
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
