// Author: L.Shuang
// Created: 2026-06-04
// Last Modified: 2026-06-04
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
package repl

import (
	"fmt"
	"os"
	"syscall"
	"unicode/utf8"
	"unsafe"
)

// EnhancedInput implements an interactive line editor with:
// - History navigation via Up/Down arrow keys
// - Proper multi-byte character handling (Chinese, emoji, etc.)
// - Correct backspace behavior for multi-byte characters
// - Home/End key navigation
//
// Pure standard library implementation (no external dependencies).
type EnhancedInput struct {
	buffer    []rune
	cursor    int // cursor position within buffer (in runes)
	prompt    string
	history   []string
	histIdx   int // current history position (-1 = new input, 0..len-1 = history entry)
	oldTerm   *syscall.Termios
	termWidth int
	termFd    int
	inRaw     bool
}

// NewEnhancedInput creates a new EnhancedInput instance.
func NewEnhancedInput(prompt string, history []string) *EnhancedInput {
	e := &EnhancedInput{
		buffer:    make([]rune, 0, 256),
		cursor:    0,
		prompt:    prompt,
		history:   history,
		histIdx:   -1,
		termFd:    int(os.Stdin.Fd()),
		termWidth: 80,
		inRaw:     false,
	}
	if w, _, err := getTermSize(e.termFd); err == nil {
		e.termWidth = w
	}
	return e
}

// toTermiosPtr converts a *syscall.Termios to uintptr for syscall.
func toTermiosPtr(t *syscall.Termios) uintptr {
	return uintptr(unsafe.Pointer(t))
}

// toWinsizePtr converts a winsize struct pointer to uintptr for syscall.
func toWinsizePtr(ws *winsize) uintptr {
	return uintptr(unsafe.Pointer(ws))
}

// winsize mirrors the C struct winsize for ioctl TIOCGWINSZ.
type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

// makeRaw puts the terminal into raw mode and returns the original state.
func makeRaw(fd int) (*syscall.Termios, error) {
	var old syscall.Termios
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd),
		syscall.TIOCGETA, toTermiosPtr(&old), 0, 0, 0); err != 0 {
		return nil, fmt.Errorf("failed to get terminal attributes: %w", err)
	}

	raw := old

	// Input modes: no BRKINT, no ICRNL, no INPCK, no ISTRIP, no IXON
	raw.Iflag &^= syscall.BRKINT | syscall.ICRNL | syscall.INPCK | syscall.ISTRIP | syscall.IXON

	// Output modes: disable all output processing
	raw.Oflag &^= syscall.OPOST

	// Control modes: enable CS8, no PARENB
	raw.Cflag &^= syscall.CSIZE | syscall.PARENB
	raw.Cflag |= syscall.CS8

	// Local modes: disable ECHO, ICANON, ISIG, IEXTEN
	raw.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG | syscall.IEXTEN

	// Control characters: VMIN=1, VTIME=0 (read minimal 1 byte, no timeout)
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0

	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd),
		syscall.TIOCSETA, toTermiosPtr(&raw), 0, 0, 0); err != 0 {
		return nil, fmt.Errorf("failed to set raw terminal mode: %w", err)
	}

	return &old, nil
}

// restoreTerm restores the terminal to its original state.
func restoreTerm(fd int, old *syscall.Termios) error {
	if old == nil {
		return nil
	}
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd),
		syscall.TIOCSETA, toTermiosPtr(old), 0, 0, 0); err != 0 {
		return fmt.Errorf("failed to restore terminal: %w", err)
	}
	return nil
}

// getTermSize returns the terminal width and height.
func getTermSize(fd int) (int, int, error) {
	var ws winsize
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd),
		syscall.TIOCGWINSZ, toWinsizePtr(&ws), 0, 0, 0); err != 0 {
		return 0, 0, fmt.Errorf("failed to get terminal size")
	}
	return int(ws.Col), int(ws.Row), nil
}

// ReadLine reads a line of input with full line editing support.
// Returns the input string (without trailing newline) or an error.
func (e *EnhancedInput) ReadLine() (string, error) {
	// Save terminal state and switch to raw mode
	oldState, err := makeRaw(e.termFd)
	if err != nil {
		return "", fmt.Errorf("failed to set raw terminal mode: %w", err)
	}
	e.oldTerm = oldState
	e.inRaw = true
	defer func() {
		if e.inRaw && e.oldTerm != nil {
			restoreTerm(e.termFd, e.oldTerm)
			e.inRaw = false
		}
	}()

	// Display initial prompt
	e.displayPrompt()

	// Read bytes one by one for proper escape sequence handling
	buf := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			return "", err
		}

		b := buf[0]

		// ESC sequence (potential arrow keys, Home, End, etc.)
		if b == '\x1b' {
			seq, err := e.readEscapeSequence()
			if err != nil {
				if seq == "" {
					// Plain ESC key: cancel current input
					e.clearLine()
					e.buffer = e.buffer[:0]
					e.cursor = 0
					e.displayPrompt()
					continue
				}
				return "", err
			}
			e.handleEscapeSequence(seq)
			continue
		}

		// Enter/Return
		if b == '\r' || b == '\n' {
			e.clearLine()
			// Restore terminal before returning
			if e.oldTerm != nil {
				restoreTerm(e.termFd, e.oldTerm)
				e.inRaw = false
			}
			result := string(e.buffer)
			e.resetState()
			fmt.Println() // newline after input
			return result, nil
		}

		// Ctrl+C
		if b == 0x03 {
			e.clearLine()
			if e.oldTerm != nil {
				restoreTerm(e.termFd, e.oldTerm)
				e.inRaw = false
			}
			e.resetState()
			return "", fmt.Errorf("interrupt")
		}

		// Ctrl+D (EOF)
		if b == 0x04 {
			if len(e.buffer) == 0 {
				e.clearLine()
				if e.oldTerm != nil {
					restoreTerm(e.termFd, e.oldTerm)
					e.inRaw = false
				}
				e.resetState()
				return "", nil // EOF
			}
			// Ctrl+D with non-empty buffer: delete forward
			e.deleteForward()
			continue
		}

		// Ctrl+A (Home)
		if b == 0x01 {
			e.moveCursorToStart()
			continue
		}

		// Ctrl+E (End)
		if b == 0x05 {
			e.moveCursorToEnd()
			continue
		}

		// Ctrl+K (kill to end of line)
		if b == 0x0b {
			e.killToEnd()
			continue
		}

		// Ctrl+L (clear screen and redraw)
		if b == 0x0c {
			fmt.Print("\033[2J\033[H") // clear screen, home cursor
			e.displayPrompt()
			continue
		}

		// Ctrl+U (kill to start)
		if b == 0x15 {
			e.clearLine()
			e.buffer = e.buffer[:0]
			e.cursor = 0
			e.displayPrompt()
			continue
		}

		// Ctrl+W (kill previous word)
		if b == 0x17 {
			e.deletePreviousWord()
			continue
		}

		// Tab (no completion, just insert as regular char)
		if b == '\t' {
			e.insertRune('\t')
			continue
		}

		// Backspace
		if b == '\x7f' || b == '\b' {
			e.backspace()
			continue
		}

		// Regular ASCII printable characters
		if b >= 0x20 && b < 0x7f {
			e.insertRune(rune(b))
			continue
		}

		// Multi-byte UTF-8 sequence
		if b >= 0xc0 {
			r, size := e.readRune(b)
			if r != utf8.RuneError || size > 1 {
				e.insertRune(r)
			}
		}
	}
}

// readEscapeSequence reads a complete ANSI escape sequence.
// Returns the sequence type: "up", "down", "left", "right", "home", "end",
// "del", or empty string for plain ESC.
func (e *EnhancedInput) readEscapeSequence() (string, error) {
	// Read next byte after ESC
	buf := make([]byte, 1)
	_, err := os.Stdin.Read(buf)
	if err != nil {
		return "", err
	}

	// ESC sequence: ESC [ ...
	if buf[0] == '[' {
		seq, err := e.readCSI()
		if err != nil {
			return "", err
		}
		return seq, nil
	}

	// ESC O ... (SS3 sequences for Home/End on some terminals)
	if buf[0] == 'O' {
		seq, err := e.readSS3()
		if err != nil {
			return "", err
		}
		return seq, nil
	}

	// Plain ESC (single ESC key press)
	return "", nil
}

// readCSI reads a Control Sequence Introducer (CSI) sequence: ESC [ ... ~ or ESC [ char
func (e *EnhancedInput) readCSI() (string, error) {
	var seq []byte
	for {
		buf := make([]byte, 1)
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			return "", err
		}
		b := buf[0]
		seq = append(seq, b)

		// Standard CSI sequences end with a letter (A-Z, a-z)
		// Extended CSI sequences end with ~ (e.g., ESC [ 3 ~ for Delete)
		if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || b == '~' {
			break
		}
	}

	s := string(seq)
	switch s {
	case "A":
		return "up", nil
	case "B":
		return "down", nil
	case "C":
		return "right", nil
	case "D":
		return "left", nil
	case "H":
		return "home", nil
	case "F":
		return "end", nil
	case "3~":
		return "del", nil
	case "1~":
		return "home", nil
	case "4~":
		return "end", nil
	case "5~":
		return "pageup", nil
	case "6~":
		return "pagedown", nil
	case "7~":
		return "home", nil
	default:
		// Other CSI sequences: ignore
		return "", nil
	}
}

// readSS3 reads a SS3 sequence: ESC O char (used by some terminals for Home/End)
func (e *EnhancedInput) readSS3() (string, error) {
	buf := make([]byte, 1)
	_, err := os.Stdin.Read(buf)
	if err != nil {
		return "", err
	}

	switch buf[0] {
	case 'H':
		return "home", nil
	case 'F':
		return "end", nil
	case 'A':
		return "up", nil
	case 'B':
		return "down", nil
	case 'C':
		return "right", nil
	case 'D':
		return "left", nil
	}
	return "", nil
}

// readRune reads a multi-byte UTF-8 rune starting with the first byte.
func (e *EnhancedInput) readRune(first byte) (rune, int) {
	var size int
	switch {
	case first >= 0xf0:
		size = 4
	case first >= 0xe0:
		size = 3
	case first >= 0xc0:
		size = 2
	default:
		return rune(first), 1
	}

	raw := make([]byte, size)
	raw[0] = first
	for i := 1; i < size; i++ {
		buf := make([]byte, 1)
		_, err := os.Stdin.Read(buf)
		if err != nil {
			return utf8.RuneError, i
		}
		raw[i] = buf[0]
	}

	r, _ := utf8.DecodeRune(raw)
	return r, size
}

// handleEscapeSequence processes a recognized escape sequence.
func (e *EnhancedInput) handleEscapeSequence(seq string) {
	switch seq {
	case "up":
		e.navigateHistory(-1)
	case "down":
		e.navigateHistory(1)
	case "left":
		e.moveCursorLeft()
	case "right":
		e.moveCursorRight()
	case "home":
		e.moveCursorToStart()
	case "end":
		e.moveCursorToEnd()
	case "del":
		e.deleteForward()
	}
}

// navigateHistory navigates through history entries.
// dir: -1 = up (older), 1 = down (newer)
func (e *EnhancedInput) navigateHistory(dir int) {
	if len(e.history) == 0 {
		return
	}

	newIdx := e.histIdx + dir

	// Going up past oldest history entry: stay at oldest
	if newIdx < 0 {
		newIdx = len(e.history) - 1
	} else if newIdx >= len(e.history) {
		// Going down past newest history entry: clear input
		e.clearLine()
		e.buffer = e.buffer[:0]
		e.cursor = 0
		e.histIdx = -1
		e.displayPrompt()
		return
	}

	e.histIdx = newIdx
	entry := e.history[e.histIdx]

	// Replace buffer with history entry
	e.clearLine()
	e.buffer = []rune(entry)
	e.cursor = len(e.buffer)
	e.displayPrompt()
}

// displayPrompt shows the prompt and current buffer content.
func (e *EnhancedInput) displayPrompt() {
	fmt.Print(e.prompt)
	if len(e.buffer) > 0 {
		fmt.Print(string(e.buffer))
	}

	// Move cursor back to e.cursor position (from end of buffer)
	back := len(e.buffer) - e.cursor
	if back > 0 {
		fmt.Printf("\033[%dD", back)
	}
}

// clearLine clears the current line and moves cursor to beginning.
func (e *EnhancedInput) clearLine() {
	fmt.Print("\033[J") // Clear from cursor to end of screen
	fmt.Print("\r")
}

// redrawLine clears and redraws the prompt + buffer + positions cursor
func (e *EnhancedInput) redrawLine() {
	e.clearLine()
	e.displayPrompt()
}

// resetState resets the buffer state for next input.
func (e *EnhancedInput) resetState() {
	e.buffer = e.buffer[:0]
	e.cursor = 0
	e.histIdx = -1
}

// insertRune inserts a rune at the cursor position.
func (e *EnhancedInput) insertRune(r rune) {
	// Grow buffer if capacity is full
	if cap(e.buffer) == len(e.buffer) {
		newCap := cap(e.buffer) * 2
		if newCap < 64 {
			newCap = 64
		}
		newBuf := make([]rune, len(e.buffer)+1, newCap)
		copy(newBuf, e.buffer[:e.cursor])
		copy(newBuf[e.cursor+1:], e.buffer[e.cursor:])
		newBuf[e.cursor] = r
		e.buffer = newBuf
	} else {
		// Insert at cursor position
		e.buffer = append(e.buffer, 0) // make room
		copy(e.buffer[e.cursor+1:], e.buffer[e.cursor:])
		e.buffer[e.cursor] = r
	}
	e.cursor++

	// Redraw from cursor to end
	e.redrawToEnd()
}

// redrawToEnd redraws from cursor position to end of buffer.
func (e *EnhancedInput) redrawToEnd() {
	if len(e.buffer) == e.cursor {
		return // nothing to redraw
	}
	remaining := string(e.buffer[e.cursor:])
	fmt.Print(remaining)
	// Move cursor back to correct position
	back := len(e.buffer) - e.cursor
	if back > 0 {
		fmt.Printf("\033[%dD", back)
	}
}

// backspace removes the rune before the cursor.
// Properly handles multi-byte characters (Chinese, emoji, etc.).
func (e *EnhancedInput) backspace() {
	if e.cursor <= 0 {
		return
	}

	// Remove rune before cursor
	newLen := len(e.buffer) - 1
	newBuf := make([]rune, newLen, cap(e.buffer))
	copy(newBuf, e.buffer[:e.cursor-1])
	copy(newBuf[e.cursor-1:], e.buffer[e.cursor:])
	e.buffer = newBuf
	e.cursor--

	// Redraw from cursor to end
	fmt.Print("\033[D") // move cursor left
	remaining := string(e.buffer[e.cursor:])
	if len(remaining) > 0 {
		fmt.Print(remaining)
		// Move cursor back
		fmt.Printf("\033[%dD", len(remaining))
	} else {
		// Clear the character that was at the end
		fmt.Print(" \033[D")
	}
}

// deleteForward removes the rune at the cursor position (Delete key).
func (e *EnhancedInput) deleteForward() {
	if e.cursor >= len(e.buffer) {
		return
	}

	newLen := len(e.buffer) - 1
	newBuf := make([]rune, newLen, cap(e.buffer))
	copy(newBuf, e.buffer[:e.cursor])
	copy(newBuf[e.cursor:], e.buffer[e.cursor+1:])
	e.buffer = newBuf
	e.redrawToEnd()
}

// moveCursorLeft moves cursor one character left.
func (e *EnhancedInput) moveCursorLeft() {
	if e.cursor > 0 {
		e.cursor--
		fmt.Print("\033[D")
	}
}

// moveCursorRight moves cursor one character right.
func (e *EnhancedInput) moveCursorRight() {
	if e.cursor < len(e.buffer) {
		e.cursor++
		fmt.Print("\033[C")
	}
}

// moveCursorToStart moves cursor to the beginning of the line.
func (e *EnhancedInput) moveCursorToStart() {
	if e.cursor > 0 {
		fmt.Printf("\033[%dD", e.cursor)
		e.cursor = 0
	}
}

// moveCursorToEnd moves cursor to the end of the line.
func (e *EnhancedInput) moveCursorToEnd() {
	if e.cursor < len(e.buffer) {
		moveRight := len(e.buffer) - e.cursor
		fmt.Printf("\033[%dC", moveRight)
		e.cursor = len(e.buffer)
	}
}

// killToEnd deletes from cursor to end of line.
func (e *EnhancedInput) killToEnd() {
	if e.cursor >= len(e.buffer) {
		return
	}
	e.buffer = e.buffer[:e.cursor]
	// Clear to end of screen from cursor
	fmt.Print("\033[J")
}

// deletePreviousWord deletes the word before the cursor.
func (e *EnhancedInput) deletePreviousWord() {
	if e.cursor <= 0 {
		return
	}

	// Find the start of the word before cursor
	start := e.cursor - 1
	for start >= 0 && e.buffer[start] == ' ' {
		start--
	}
	for start >= 0 && e.buffer[start] != ' ' {
		start--
	}
	start++ // Move past the space/word boundary

	// Delete from start to cursor
	if start < e.cursor {
		newLen := len(e.buffer) - (e.cursor - start)
		newBuf := make([]rune, newLen, cap(e.buffer))
		copy(newBuf, e.buffer[:start])
		copy(newBuf[start:], e.buffer[e.cursor:])
		e.buffer = newBuf
		e.cursor = start
		e.redrawToEnd()
	}
}
