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
	"errors"
	"fmt"
	"io"
	"sync/atomic"

	"github.com/idirect3d/co-shell/agent"
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
	histIdx int // current history position (-1 = new input, 0..len-1 = history entry)
}

// NewEnhancedInput creates a new EnhancedInput instance.
func NewEnhancedInput(prompt string, history []string) *EnhancedInput {
	e := &EnhancedInput{
		buffer:  make([]rune, 0, 256),
		cursor:  0,
		prompt:  prompt,
		history: history,
		histIdx: -1,
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

// inputEventSource is the minimal event-consumption surface ReadLineFrom needs
// from the unified InputReader. The interface keeps the line editor testable
// with a fake source.
type inputEventSource interface {
	// Subscribe registers a consumer callback; the returned func unsubscribes.
	Subscribe(fn func(agent.InputEvent)) func()
	// Done returns a channel closed when the source shuts down.
	Done() <-chan struct{}
}

// ReadLineFrom reads a line of input from the unified event stream with full
// line editing support (P2.5). Raw terminal mode is owned by the InputReader.
func (e *EnhancedInput) ReadLineFrom(reader inputEventSource) (string, error) {
	e.displayPrompt()

	type result struct {
		line string
		err  error
	}
	resCh := make(chan result, 1)
	var done atomic.Bool

	cancel := reader.Subscribe(func(ev agent.InputEvent) {
		if done.Load() {
			return
		}
		switch ev.Kind {
		case agent.InputEnter:
			line := string(e.buffer)
			e.resetState()
			fmt.Print("\r\n")
			done.Store(true)
			resCh <- result{line: line}
		case agent.InputCtrlC:
			e.clearLine()
			e.resetState()
			done.Store(true)
			resCh <- result{err: errors.New("interrupt")}
		case agent.InputEOF:
			if len(e.buffer) == 0 {
				e.clearLine()
				e.resetState()
				done.Store(true)
				resCh <- result{}
			} else {
				e.deleteForward()
			}
		case agent.InputEsc:
			// A lone ESC clears the current input line (legacy behaviour).
			e.clearLine()
			e.buffer = e.buffer[:0]
			e.cursor = 0
			e.displayPrompt()
		case agent.InputKey:
			if ev.Data == "\x04" {
				// Ctrl+D: empty buffer means EOF, otherwise delete forward.
				if len(e.buffer) == 0 {
					e.clearLine()
					e.resetState()
					done.Store(true)
					resCh <- result{}
				} else {
					e.deleteForward()
				}
				return
			}
			e.handleKeyData(ev.Data)
		default:
			e.handleInputEvent(ev)
		}
	})
	defer cancel()

	select {
	case res := <-resCh:
		return res.line, res.err
	case <-reader.Done():
		return "", io.EOF
	}
}

// handleKeyData applies a single character (or rune) to the line buffer.
func (e *EnhancedInput) handleKeyData(data string) {
	if data == "" {
		return
	}
	b := data[0]
	if b < 0x20 {
		e.handleControlByte(b)
		return
	}
	if b == 0x7f {
		e.backspace()
		return
	}
	for _, r := range data {
		e.insertRune(r)
	}
}

// handleInputEvent applies a non-terminal input event to the line buffer.
func (e *EnhancedInput) handleInputEvent(ev agent.InputEvent) {
	switch ev.Kind {
	case agent.InputBackspace:
		e.backspace()
	case agent.InputArrowUp:
		e.navigateHistory(-1)
	case agent.InputArrowDn:
		e.navigateHistory(1)
	case agent.InputArrowLt:
		e.moveCursorLeft()
	case agent.InputArrowRt:
		e.moveCursorRight()
	case agent.InputHome:
		e.moveCursorToStart()
	case agent.InputEnd:
		e.moveCursorToEnd()
	case agent.InputDelete:
		e.deleteForward()
	case agent.InputTab:
		e.insertRune('\t')
	}
}

// handleControlByte applies legacy Ctrl-combination editing shortcuts.
func (e *EnhancedInput) handleControlByte(b byte) {
	switch b {
	case 0x01: // Ctrl+A — move to start of line
		e.moveCursorToStart()
	case 0x05: // Ctrl+E — move to end of line
		e.moveCursorToEnd()
	case 0x0b: // Ctrl+K — kill to end
		e.killToEnd()
	case 0x0c: // Ctrl+L — clear screen and redraw prompt
		fmt.Print("\033[2J\033[H")
		e.displayPrompt()
	case 0x15: // Ctrl+U — clear line
		e.clearLine()
		e.buffer = e.buffer[:0]
		e.cursor = 0
		e.displayPrompt()
	case 0x17: // Ctrl+W — delete previous word
		e.deletePreviousWord()
	}
}

// navigateHistory navigates the input history buffer.
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
