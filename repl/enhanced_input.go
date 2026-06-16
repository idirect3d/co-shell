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
	"unicode/utf8"
)

// EnhancedInput implements an interactive line editor with:
// - History navigation via Up/Down arrow keys
// - Proper multi-byte character handling (Chinese, emoji, etc.)
// - Correct backspace behavior for multi-byte characters
// - Home/End key navigation
type EnhancedInput struct {
	buffer  []rune
	cursor  int // cursor position within buffer (in runes)
	prompt  string
	history []string
	histIdx int         // current history position (-1 = new input, 0..len-1 = history entry)
	oldTerm interface{} // *unix.Termios on POSIX, nil on Windows
	termFd  int
	inRaw   bool
}

// NewEnhancedInput creates a new EnhancedInput instance.
func NewEnhancedInput(prompt string, history []string) *EnhancedInput {
	e := &EnhancedInput{
		buffer:  make([]rune, 0, 256),
		cursor:  0,
		prompt:  prompt,
		history: history,
		histIdx: -1,
		termFd:  int(os.Stdin.Fd()),
		inRaw:   false,
	}
	return e
}

// cursorLeftN moves cursor left by N display columns.
func cursorLeftN(n int) {
	if n > 0 {
		fmt.Printf("\033[%dD", n)
	}
}

// cursorRightN moves cursor right by N display columns.
func cursorRightN(n int) {
	if n > 0 {
		fmt.Printf("\033[%dC", n)
	}
}

// runeWidth returns the display width of a rune.
// CJK characters are 2 columns wide, ASCII is 1.
func runeWidth(r rune) int {
	if r == '\t' {
		return 8
	}
	// CJK Unified Ideographs and related blocks
	if r >= 0x1100 &&
		(r <= 0x115f || r == 0x2329 || r == 0x232a ||
			(r >= 0x2e80 && r <= 0x303e) ||
			(r >= 0x3040 && r <= 0x33ff) ||
			(r >= 0x3400 && r <= 0x4dbf) ||
			(r >= 0x4e00 && r <= 0xa4cf) ||
			(r >= 0xac00 && r <= 0xd7af) ||
			(r >= 0xf900 && r <= 0xfaff) ||
			(r >= 0xfe30 && r <= 0xfe6f) ||
			(r >= 0xff01 && r <= 0xff60) ||
			(r >= 0xffe0 && r <= 0xffe6) ||
			(r >= 0x1b000 && r <= 0x1b0ff) ||
			(r >= 0x1f000 && r <= 0x1f9ff) ||
			(r >= 0x20000 && r <= 0x2ffff)) {
		return 2
	}
	return 1
}

// promptDisplayLen returns the display column width of the prompt string.
func promptDisplayLen(prompt string) int {
	width := 0
	for _, r := range prompt {
		width += runeWidth(r)
	}
	return width
}

// bufferDisplayWidth returns total display columns of buffer content.
func bufferDisplayWidth(buf []rune) int {
	w := 0
	for _, r := range buf {
		w += runeWidth(r)
	}
	return w
}

// cursorDisplayColumn returns the display column of the cursor (0-based, after prompt).
func (e *EnhancedInput) cursorDisplayColumn() int {
	w := 0
	for i := 0; i < e.cursor; i++ {
		w += runeWidth(e.buffer[i])
	}
	return w
}

// ReadLine reads a line of input with full line editing support.
func (e *EnhancedInput) ReadLine() (string, error) {
	oldState, err := MakeRaw(e.termFd)
	if err != nil {
		return "", fmt.Errorf("failed to set raw terminal mode: %w", err)
	}
	e.oldTerm = oldState
	e.inRaw = true
	defer func() {
		if e.inRaw && e.oldTerm != nil {
			RestoreTerm(e.termFd, e.oldTerm)
			e.inRaw = false
		}
	}()

	e.displayPrompt()

	buf := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			return "", err
		}

		b := buf[0]

		if b == '\x1b' {
			seq, err := e.readEscapeSequence()
			if err != nil {
				if seq == "" {
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

		if b == '\r' || b == '\n' {
			if e.oldTerm != nil {
				RestoreTerm(e.termFd, e.oldTerm)
				e.inRaw = false
			}
			result := string(e.buffer)
			e.resetState()
			fmt.Print("\r\n")
			return result, nil
		}

		if b == 0x03 {
			e.clearLine()
			if e.oldTerm != nil {
				RestoreTerm(e.termFd, e.oldTerm)
				e.inRaw = false
			}
			e.resetState()
			return "", fmt.Errorf("interrupt")
		}

		if b == 0x04 {
			if len(e.buffer) == 0 {
				e.clearLine()
				if e.oldTerm != nil {
					RestoreTerm(e.termFd, e.oldTerm)
					e.inRaw = false
				}
				e.resetState()
				return "", nil
			}
			e.deleteForward()
			continue
		}

		if b == 0x01 {
			e.moveCursorToStart()
			continue
		}

		if b == 0x05 {
			e.moveCursorToEnd()
			continue
		}

		if b == 0x0b {
			e.killToEnd()
			continue
		}

		if b == 0x0c {
			fmt.Print("\033[2J\033[H")
			e.displayPrompt()
			continue
		}

		if b == 0x15 {
			e.clearLine()
			e.buffer = e.buffer[:0]
			e.cursor = 0
			e.displayPrompt()
			continue
		}

		if b == 0x17 {
			e.deletePreviousWord()
			continue
		}

		if b == '\t' {
			e.insertRune('\t')
			continue
		}

		if b == '\x7f' || b == '\b' {
			e.backspace()
			continue
		}

		if b >= 0x20 && b < 0x7f {
			e.insertRune(rune(b))
			continue
		}

		if b >= 0xc0 {
			r, size := e.readRune(b)
			if r != utf8.RuneError || size > 1 {
				e.insertRune(r)
			}
		}
	}
}

// readEscapeSequence reads a complete ANSI escape sequence.
func (e *EnhancedInput) readEscapeSequence() (string, error) {
	buf := make([]byte, 1)
	_, err := os.Stdin.Read(buf)
	if err != nil {
		return "", err
	}
	if buf[0] == '[' {
		return e.readCSI()
	}
	if buf[0] == 'O' {
		return e.readSS3()
	}
	return "", nil
}

// readCSI reads a CSI sequence.
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
	default:
		return "", nil
	}
}

// readSS3 reads a SS3 sequence.
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

// readRune reads a multi-byte UTF-8 rune.
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

func (e *EnhancedInput) navigateHistory(dir int) {
	if len(e.history) == 0 {
		return
	}
	newIdx := e.histIdx + dir
	if newIdx < 0 {
		newIdx = len(e.history) - 1
	} else if newIdx >= len(e.history) {
		e.clearLine()
		e.buffer = e.buffer[:0]
		e.cursor = 0
		e.histIdx = -1
		e.displayPrompt()
		return
	}
	e.histIdx = newIdx
	e.clearLine()
	e.buffer = []rune(e.history[e.histIdx])
	e.cursor = len(e.buffer)
	e.displayPrompt()
}

func (e *EnhancedInput) displayPrompt() {
	fmt.Print(e.prompt)
	if len(e.buffer) > 0 {
		fmt.Print(string(e.buffer))
	}
	// Move cursor back to current position using display width
	curCol := e.cursorDisplayColumn()
	totalCol := bufferDisplayWidth(e.buffer)
	back := totalCol - curCol
	if back > 0 {
		cursorLeftN(back)
	}
}

func (e *EnhancedInput) clearLine() {
	fmt.Print("\r")
	fmt.Print("\033[2K")
}

func (e *EnhancedInput) resetState() {
	e.buffer = e.buffer[:0]
	e.cursor = 0
	e.histIdx = -1
}

func (e *EnhancedInput) insertRune(r rune) {
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
		e.buffer = append(e.buffer, 0)
		copy(e.buffer[e.cursor+1:], e.buffer[e.cursor:])
		e.buffer[e.cursor] = r
	}
	e.cursor++

	// Print everything from the inserted position to end of buffer
	fmt.Print(string(e.buffer[e.cursor-1:]))

	// Move cursor back to e.cursor position using display width
	curCol := e.cursorDisplayColumn()
	totalCol := bufferDisplayWidth(e.buffer)
	back := totalCol - curCol
	if back > 0 {
		cursorLeftN(back)
	}
}

func (e *EnhancedInput) backspace() {
	if e.cursor <= 0 {
		return
	}

	// Check if the character being deleted has non-standard display width.
	// Tab characters depend on the cursor's terminal tab-stop position (every 8 columns),
	// so their actual display width cannot be calculated with a fixed value.
	// For Tab and other special-width characters, use full line redraw to avoid
	// incorrect cursor positioning.
	deleted := e.buffer[e.cursor-1]
	needsFullRedraw := deleted == '\t'

	if !needsFullRedraw {
		// Fast path: standard-width characters (ASCII, CJK, etc.)
		// Width of character being deleted
		deletedWidth := runeWidth(deleted)

		// Remove rune before cursor
		newLen := len(e.buffer) - 1
		newBuf := make([]rune, newLen, cap(e.buffer))
		copy(newBuf, e.buffer[:e.cursor-1])
		copy(newBuf[e.cursor-1:], e.buffer[e.cursor:])
		e.buffer = newBuf
		e.cursor--

		// Move cursor left by the display width of the deleted character
		cursorLeftN(deletedWidth)

		// Redraw from cursor to end
		remaining := string(e.buffer[e.cursor:])
		if len(remaining) > 0 {
			fmt.Print(remaining)
		}
		// Clear the last character visually (it may still be on screen)
		fmt.Print(" ")
		// Move cursor back to where it should be
		curCol := e.cursorDisplayColumn()
		totalCol := bufferDisplayWidth(e.buffer)
		back := totalCol - curCol
		if back > 0 {
			cursorLeftN(back)
		}
		// Also clear the extra space we added
		cursorLeftN(1)
		return
	}

	// Slow path: Tab or other variable-width characters.
	// Remove the tab rune and redraw the entire line to ensure correct
	// cursor positioning based on the terminal's actual tab-stop rendering.
	newLen := len(e.buffer) - 1
	newBuf := make([]rune, newLen, cap(e.buffer))
	copy(newBuf, e.buffer[:e.cursor-1])
	copy(newBuf[e.cursor-1:], e.buffer[e.cursor:])
	e.buffer = newBuf
	e.cursor--

	e.clearLine()
	e.displayPrompt()
}

func (e *EnhancedInput) deleteForward() {
	if e.cursor >= len(e.buffer) {
		return
	}
	newLen := len(e.buffer) - 1
	newBuf := make([]rune, newLen, cap(e.buffer))
	copy(newBuf, e.buffer[:e.cursor])
	copy(newBuf[e.cursor:], e.buffer[e.cursor+1:])
	e.buffer = newBuf

	// Redraw from cursor
	remaining := string(e.buffer[e.cursor:])
	if len(remaining) > 0 {
		fmt.Print(remaining)
	}
	fmt.Print(" ") // clear last char
	curCol := e.cursorDisplayColumn()
	totalCol := bufferDisplayWidth(e.buffer)
	back := totalCol - curCol
	if back > 0 {
		cursorLeftN(back)
	}
	cursorLeftN(1) // the extra space
}

func (e *EnhancedInput) moveCursorLeft() {
	if e.cursor > 0 {
		e.cursor--
		cursorLeftN(runeWidth(e.buffer[e.cursor]))
	}
}

func (e *EnhancedInput) moveCursorRight() {
	if e.cursor < len(e.buffer) {
		cursorRightN(runeWidth(e.buffer[e.cursor]))
		e.cursor++
	}
}

func (e *EnhancedInput) moveCursorToStart() {
	curCol := e.cursorDisplayColumn()
	if curCol > 0 {
		cursorLeftN(curCol)
		e.cursor = 0
	}
}

func (e *EnhancedInput) moveCursorToEnd() {
	curCol := e.cursorDisplayColumn()
	totalCol := bufferDisplayWidth(e.buffer)
	if curCol < totalCol {
		cursorRightN(totalCol - curCol)
		e.cursor = len(e.buffer)
	}
}

func (e *EnhancedInput) killToEnd() {
	if e.cursor >= len(e.buffer) {
		return
	}
	e.buffer = e.buffer[:e.cursor]
	fmt.Print("\033[J")
}

func (e *EnhancedInput) deletePreviousWord() {
	if e.cursor <= 0 {
		return
	}
	start := e.cursor - 1
	for start >= 0 && e.buffer[start] == ' ' {
		start--
	}
	for start >= 0 && e.buffer[start] != ' ' {
		start--
	}
	start++
	if start < e.cursor {
		newLen := len(e.buffer) - (e.cursor - start)
		newBuf := make([]rune, newLen, cap(e.buffer))
		copy(newBuf, e.buffer[:start])
		copy(newBuf[start:], e.buffer[e.cursor:])
		e.buffer = newBuf
		curCol := e.cursorDisplayColumn()
		// We need to go back to start position, calculate columns back
		targetCol := 0
		for i := 0; i < start; i++ {
			targetCol += runeWidth(e.buffer[i])
		}
		moveBack := curCol - targetCol
		if moveBack > 0 {
			cursorLeftN(moveBack)
		}
		e.cursor = start
		remaining := string(e.buffer[e.cursor:])
		if len(remaining) > 0 {
			fmt.Print(remaining)
		}
		fmt.Print(" ")
		newCurCol := e.cursorDisplayColumn()
		totalCol := bufferDisplayWidth(e.buffer)
		back := totalCol - newCurCol
		if back > 0 {
			cursorLeftN(back)
		}
		cursorLeftN(1)
	}
}
