// Author: L.Shuang
// Created: 2026-05-31
// Last Modified: 2026-05-31
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

// Package shell provides a persistent interactive shell session that maintains
// state across command executions.
package shell

import (
	"strings"
	"sync"
)

// VirtualTerminal emulates a character-cell terminal for rendering shell output
// to the LLM as a consistent window of text. It parses ANSI escape sequences
// to maintain an accurate character grid, mimicking how a human sees a terminal.
//
// No external dependencies are used — the ANSI parser is a hand-written state
// machine that supports a minimal, practical subset of control sequences.
type VirtualTerminal struct {
	mu sync.Mutex

	// Character grid: cells[row][col] stores the character at each position.
	// row 0 is the top of the window, row rows-1 is the bottom.
	cells [][]rune

	rows    int // number of visible rows in the window
	cols    int // number of visible columns in the window
	scrollH int // scrollback history size (lines preserved above the window)
	cursorR int // current cursor row (0-based, relative to window top)
	cursorC int // current cursor column (0-based)

	// Scrollback buffer: keeps lines that scrolled off the top of the window.
	scrollback [][]rune

	// ANSI parser state
	ansiState int    // 0=normal, 1=ESC received, 2=CSI params, 3=OSC
	paramBuf  []int  // parsed numeric parameters for current CSI sequence
	paramStr  []byte // raw parameter string for current sequence
	oscBuf    []byte // OSC string buffer
}

// VT ANSI parser states
const (
	vtStateNormal = iota
	vtStateESC
	vtStateCSI
	vtStateOSC
)

// DefaultVTSize constants
const (
	DefaultVTRows   = 24
	DefaultVTCols   = 80
	DefaultVTScroll = 1000
)

// NewVirtualTerminal creates a new virtual terminal with the given dimensions.
// rows: number of visible rows (default 24)
// cols: number of visible columns (default 80)
func NewVirtualTerminal(rows, cols int) *VirtualTerminal {
	if rows <= 0 {
		rows = DefaultVTRows
	}
	if cols <= 0 {
		cols = DefaultVTCols
	}
	vt := &VirtualTerminal{
		rows:    rows,
		cols:    cols,
		scrollH: DefaultVTScroll,
	}
	vt.allocCells()
	return vt
}

// allocCells reallocates the cell grid with current dimensions.
func (vt *VirtualTerminal) allocCells() {
	vt.cells = make([][]rune, vt.rows)
	for r := 0; r < vt.rows; r++ {
		vt.cells[r] = make([]rune, vt.cols)
		for c := 0; c < vt.cols; c++ {
			vt.cells[r][c] = ' '
		}
	}
	vt.clampCursor()
}

// clampCursor ensures cursor is within valid bounds.
func (vt *VirtualTerminal) clampCursor() {
	if vt.cursorR < 0 {
		vt.cursorR = 0
	}
	if vt.cursorR >= vt.rows {
		vt.cursorR = vt.rows - 1
	}
	if vt.cursorC < 0 {
		vt.cursorC = 0
	}
	if vt.cursorC >= vt.cols {
		vt.cursorC = vt.cols - 1
	}
}

// Resize changes the terminal window dimensions.
// Content that fits within the new dimensions is preserved.
func (vt *VirtualTerminal) Resize(rows, cols int) {
	vt.mu.Lock()
	defer vt.mu.Unlock()

	if rows <= 0 {
		rows = DefaultVTRows
	}
	if cols <= 0 {
		cols = DefaultVTCols
	}
	if rows == vt.rows && cols == vt.cols {
		return
	}

	// Save old content
	oldCells := vt.cells
	oldRows := vt.rows
	oldCols := vt.cols

	vt.rows = rows
	vt.cols = cols
	vt.allocCells()

	// Copy over content that fits
	copyRows := oldRows
	if copyRows > vt.rows {
		copyRows = vt.rows
	}
	copyCols := oldCols
	if copyCols > vt.cols {
		copyCols = vt.cols
	}
	for r := 0; r < copyRows; r++ {
		for c := 0; c < copyCols; c++ {
			vt.cells[r][c] = oldCells[r][c]
		}
	}

	vt.clampCursor()
}

// Reset clears the terminal window and scrollback buffer.
func (vt *VirtualTerminal) Reset() {
	vt.mu.Lock()
	defer vt.mu.Unlock()
	vt.allocCells()
	vt.scrollback = nil
	vt.resetAnsiState()
}

// scrollUp scrolls the window content up by one line.
// The top line is moved to the scrollback buffer, and a blank line
// is added at the bottom.
func (vt *VirtualTerminal) scrollUp() {
	// Save top line to scrollback
	if vt.scrollH > 0 {
		topLine := make([]rune, vt.cols)
		copy(topLine, vt.cells[0])
		vt.scrollback = append(vt.scrollback, topLine)
		// Limit scrollback size
		if len(vt.scrollback) > vt.scrollH {
			vt.scrollback = vt.scrollback[len(vt.scrollback)-vt.scrollH:]
		}
	}

	// Shift all lines up
	copy(vt.cells, vt.cells[1:])

	// Clear last line
	lastRow := vt.rows - 1
	for c := 0; c < vt.cols; c++ {
		vt.cells[lastRow][c] = ' '
	}
}

// writeChar writes a single character at the current cursor position
// and advances the cursor. Handles wrapping and scrolling.
func (vt *VirtualTerminal) writeChar(ch rune) {
	if vt.cursorC >= vt.cols {
		// Automatic line wrap
		vt.cursorC = 0
		vt.cursorR++
		if vt.cursorR >= vt.rows {
			vt.scrollUp()
			vt.cursorR = vt.rows - 1
		}
	}
	if vt.cursorC < vt.cols && vt.cursorR < vt.rows {
		vt.cells[vt.cursorR][vt.cursorC] = ch
		vt.cursorC++
	}
}

// Process feeds raw byte data (containing ANSI escape sequences) into the
// virtual terminal. The terminal updates its internal state accordingly.
func (vt *VirtualTerminal) Process(data []byte) {
	vt.mu.Lock()
	defer vt.mu.Unlock()

	for _, b := range data {
		vt.processByte(b)
	}
}

// processByte handles a single byte according to the ANSI parser state machine.
func (vt *VirtualTerminal) processByte(b byte) {
	switch vt.ansiState {
	case vtStateNormal:
		if b == 0x1b { // ESC
			vt.ansiState = vtStateESC
			return
		}
		// Handle control characters
		switch b {
		case '\r': // CR - carriage return
			vt.cursorC = 0
		case '\n': // LF - line feed
			vt.cursorR++
			if vt.cursorR >= vt.rows {
				vt.scrollUp()
				vt.cursorR = vt.rows - 1
			}
		case '\b': // BS - backspace
			if vt.cursorC > 0 {
				vt.cursorC--
			}
		case '\t': // TAB - horizontal tab
			nextTab := ((vt.cursorC / 8) + 1) * 8
			if nextTab > vt.cols {
				nextTab = vt.cols - 1
			}
			// Fill with spaces up to next tab stop
			for vt.cursorC < nextTab {
				vt.writeChar(' ')
			}
			// writeChar advanced one past, correct it
			vt.cursorC = nextTab
		case '\x0b', '\x0c': // VT/FF - treated as LF
			vt.cursorR++
			if vt.cursorR >= vt.rows {
				vt.scrollUp()
				vt.cursorR = vt.rows - 1
			}
		case '\x07': // BEL - ignored in VT (no audio)
		default:
			// Printable character or other byte
			if b >= 0x20 || b == 0x00 {
				vt.writeChar(rune(b))
			}
		}

	case vtStateESC:
		if b == '[' { // CSI sequence
			vt.ansiState = vtStateCSI
			vt.paramBuf = nil
			vt.paramStr = nil
			return
		}
		if b == ']' { // OSC sequence
			vt.ansiState = vtStateOSC
			vt.oscBuf = nil
			return
		}
		// Single-character escape sequences
		switch b {
		case 'c': // RIS - reset
			vt.allocCells()
		case 'D': // IND - index (scroll up)
			vt.cursorR++
			if vt.cursorR >= vt.rows {
				vt.scrollUp()
				vt.cursorR = vt.rows - 1
			}
		case 'M': // RI - reverse index (scroll down)
			vt.cursorR--
			if vt.cursorR < 0 {
				// Scroll down: insert blank line at top
				vt.cursorR = 0
				copy(vt.cells[1:], vt.cells)
				for c := 0; c < vt.cols; c++ {
					vt.cells[0][c] = ' '
				}
			}
		case '7': // DECSC - save cursor position
			// Not implemented for now
		case '8': // DECRC - restore cursor position
			// Not implemented for now
		case '=', '>': // Alt keypad / normal keypad
			// Ignored
		}
		vt.resetAnsiState()

	case vtStateCSI:
		if b >= '0' && b <= '9' {
			vt.paramStr = append(vt.paramStr, b)
			return
		}
		if b == ';' || b == ':' {
			// Save current parameter
			vt.flushParam()
			vt.paramStr = append(vt.paramStr, b) // keep delimiter for now
			return
		}
		if b == '?' {
			// DEC private marker - just ignore the '?' for parsing
			return
		}
		// Final byte - execute the CSI command
		vt.flushParam()
		vt.executeCSI(b)
		vt.resetAnsiState()

	case vtStateOSC:
		// OSC: ESC ] ... ST (ESC \) or BEL
		if b == 0x07 {
			vt.executeOSC()
			vt.resetAnsiState()
			return
		}
		if b == 0x1b {
			// Could be ST (ESC \)
			vt.oscBuf = append(vt.oscBuf, b)
			return
		}
		if b == '\\' && len(vt.oscBuf) > 0 && vt.oscBuf[len(vt.oscBuf)-1] == 0x1b {
			// ST terminator (ESC \)
			vt.executeOSC()
			vt.resetAnsiState()
			return
		}
		vt.oscBuf = append(vt.oscBuf, b)
	}
}

// flushParam saves the current parameter string as a parsed integer.
func (vt *VirtualTerminal) flushParam() {
	if len(vt.paramStr) == 0 {
		vt.paramBuf = append(vt.paramBuf, 0)
		return
	}
	// Parse the parameter string, handling semicolons as separators
	parts := strings.Split(string(vt.paramStr), ";")
	for _, p := range parts {
		if p == "" {
			vt.paramBuf = append(vt.paramBuf, 0)
		} else {
			val := 0
			for _, c := range p {
				if c >= '0' && c <= '9' {
					val = val*10 + int(c-'0')
				}
			}
			vt.paramBuf = append(vt.paramBuf, val)
		}
	}
	vt.paramStr = nil
}

// param returns the n-th CSI parameter (0-based), defaulting to def if not set.
func (vt *VirtualTerminal) param(n int, def int) int {
	if n < len(vt.paramBuf) {
		return vt.paramBuf[n]
	}
	return def
}

// executeCSI executes a CSI (Control Sequence Introducer) command.
func (vt *VirtualTerminal) executeCSI(cmd byte) {
	switch cmd {
	case 'A': // CUU - Cursor Up
		n := vt.param(0, 1)
		vt.cursorR -= n
		if vt.cursorR < 0 {
			vt.cursorR = 0
		}

	case 'B': // CUD - Cursor Down
		n := vt.param(0, 1)
		vt.cursorR += n
		if vt.cursorR >= vt.rows {
			vt.cursorR = vt.rows - 1
		}

	case 'C': // CUF - Cursor Forward
		n := vt.param(0, 1)
		vt.cursorC += n
		if vt.cursorC >= vt.cols {
			vt.cursorC = vt.cols - 1
		}

	case 'D': // CUB - Cursor Back
		n := vt.param(0, 1)
		vt.cursorC -= n
		if vt.cursorC < 0 {
			vt.cursorC = 0
		}

	case 'H', 'f': // CUP / HVP - Cursor Position
		row := vt.param(0, 1) - 1 // 1-based to 0-based
		col := vt.param(1, 1) - 1
		if row < 0 {
			row = 0
		}
		if row >= vt.rows {
			row = vt.rows - 1
		}
		if col < 0 {
			col = 0
		}
		if col >= vt.cols {
			col = vt.cols - 1
		}
		vt.cursorR = row
		vt.cursorC = col

	case 'G': // CHA - Cursor Horizontal Absolute
		col := vt.param(0, 1) - 1
		if col < 0 {
			col = 0
		}
		if col >= vt.cols {
			col = vt.cols - 1
		}
		vt.cursorC = col

	case 'J': // ED - Erase in Display
		mode := vt.param(0, 0)
		switch mode {
		case 0: // Erase from cursor to end of screen
			vt.eraseCursorToEnd()
		case 1: // Erase from start to cursor
			vt.eraseStartToCursor()
		case 2: // Erase entire screen
			vt.eraseEntireScreen()
		case 3: // Erase scrollback buffer
			vt.scrollback = nil
		}

	case 'K': // EL - Erase in Line
		mode := vt.param(0, 0)
		switch mode {
		case 0: // Erase from cursor to end of line
			for c := vt.cursorC; c < vt.cols; c++ {
				vt.cells[vt.cursorR][c] = ' '
			}
		case 1: // Erase from start of line to cursor
			for c := 0; c <= vt.cursorC && c < vt.cols; c++ {
				vt.cells[vt.cursorR][c] = ' '
			}
		case 2: // Erase entire line
			for c := 0; c < vt.cols; c++ {
				vt.cells[vt.cursorR][c] = ' '
			}
		}

	case 'L': // IL - Insert Line
		n := vt.param(0, 1)
		if n <= 0 || n >= vt.rows {
			n = 1
		}
		if vt.cursorR < vt.rows {
			// Scroll lines down from cursor
			space := vt.rows - vt.cursorR - n
			if space < 0 {
				n = vt.rows - vt.cursorR
				space = 0
			}
			if space > 0 {
				copy(vt.cells[vt.cursorR+n:], vt.cells[vt.cursorR:])
			}
			// Clear inserted lines
			for r := vt.cursorR; r < vt.cursorR+n && r < vt.rows; r++ {
				for c := 0; c < vt.cols; c++ {
					vt.cells[r][c] = ' '
				}
			}
		}

	case 'M': // DL - Delete Line
		n := vt.param(0, 1)
		if n <= 0 || n >= vt.rows {
			n = 1
		}
		if vt.cursorR < vt.rows {
			remain := vt.rows - vt.cursorR - n
			if remain > 0 {
				copy(vt.cells[vt.cursorR:], vt.cells[vt.cursorR+n:])
			}
			// Clear bottom lines
			startClear := vt.rows - n
			if startClear < vt.cursorR {
				startClear = vt.cursorR + remain
			}
			for r := startClear; r < vt.rows; r++ {
				for c := 0; c < vt.cols; c++ {
					vt.cells[r][c] = ' '
				}
			}
		}

	case 'P': // DCH - Delete Character
		n := vt.param(0, 1)
		if n <= 0 {
			n = 1
		}
		if vt.cursorR < vt.rows {
			row := vt.cells[vt.cursorR]
			// Shift characters left
			copy(row[vt.cursorC:], row[vt.cursorC+n:])
			// Clear rightmost characters
			for c := vt.cols - n; c < vt.cols; c++ {
				row[c] = ' '
			}
		}

	case '@': // ICH - Insert Character
		n := vt.param(0, 1)
		if n <= 0 {
			n = 1
		}
		if vt.cursorR < vt.rows {
			row := vt.cells[vt.cursorR]
			// Shift characters right
			copy(row[vt.cursorC+n:], row[vt.cursorC:])
			// Clear inserted positions
			for c := vt.cursorC; c < vt.cursorC+n && c < vt.cols; c++ {
				row[c] = ' '
			}
		}

	case 'X': // ECH - Erase Character
		n := vt.param(0, 1)
		if n <= 0 {
			n = 1
		}
		if vt.cursorR < vt.rows {
			for c := vt.cursorC; c < vt.cursorC+n && c < vt.cols; c++ {
				vt.cells[vt.cursorR][c] = ' '
			}
		}

	case 'm': // SGR - Select Graphic Rendition
		// Parse and apply SGR parameters, but we ignore them for rendering
		// since we only return plain text. We just consume the sequence.

	case 's': // SCOSC - Save cursor position (ANSI)
		// Save current cursor position
		// Not implemented for rendering

	case 'u': // SCORC - Restore cursor position (ANSI)
		// Not implemented for rendering

	case 'r': // DECSTBM - Set scrolling region
		// Not implemented, full screen scroll only

	case 'h': // SM - Set Mode
		// Not implemented

	case 'l': // RM - Reset Mode
		// Not implemented
	}
}

// eraseCursorToEnd erases from cursor position to end of screen.
func (vt *VirtualTerminal) eraseCursorToEnd() {
	// Erase current line from cursor to end
	if vt.cursorR < vt.rows {
		for c := vt.cursorC; c < vt.cols; c++ {
			vt.cells[vt.cursorR][c] = ' '
		}
		// Erase all lines below
		for r := vt.cursorR + 1; r < vt.rows; r++ {
			for c := 0; c < vt.cols; c++ {
				vt.cells[r][c] = ' '
			}
		}
	}
}

// eraseStartToCursor erases from beginning of screen to cursor.
func (vt *VirtualTerminal) eraseStartToCursor() {
	// Erase lines above cursor
	for r := 0; r < vt.cursorR; r++ {
		for c := 0; c < vt.cols; c++ {
			vt.cells[r][c] = ' '
		}
	}
	// Erase current line from start to cursor
	if vt.cursorR < vt.rows {
		for c := 0; c <= vt.cursorC && c < vt.cols; c++ {
			vt.cells[vt.cursorR][c] = ' '
		}
	}
}

// eraseEntireScreen erases the entire screen.
func (vt *VirtualTerminal) eraseEntireScreen() {
	for r := 0; r < vt.rows; r++ {
		for c := 0; c < vt.cols; c++ {
			vt.cells[r][c] = ' '
		}
	}
	// Some clear screen sequences also reset cursor position
	vt.cursorR = 0
	vt.cursorC = 0
}

// executeOSC executes an OSC (Operating System Command) sequence.
// OSC sequences are generally ignored as they don't affect the rendered content.
func (vt *VirtualTerminal) executeOSC() {
	// OSC sequences like setting window title (OSC 0;...), icon name (OSC 1;...),
	// or hyperlinks (OSC 8;...;) don't affect the character grid.
	// We simply ignore them after parsing.
}

// resetAnsiState resets the ANSI parser to normal state.
func (vt *VirtualTerminal) resetAnsiState() {
	vt.ansiState = vtStateNormal
	vt.paramBuf = nil
	vt.paramStr = nil
	vt.oscBuf = nil
}

// Render returns the current terminal window content as a plain text string.
// Each row is a line of text, and trailing whitespace on each line is trimmed.
// The result always has exactly rows lines, one per terminal row.
func (vt *VirtualTerminal) Render() string {
	vt.mu.Lock()
	defer vt.mu.Unlock()

	var sb strings.Builder
	for r := 0; r < vt.rows; r++ {
		if r > 0 {
			sb.WriteByte('\n')
		}
		// Find last non-space character
		end := vt.cols - 1
		for end >= 0 && vt.cells[r][end] == ' ' {
			end--
		}
		for c := 0; c <= end; c++ {
			sb.WriteRune(vt.cells[r][c])
		}
	}
	return sb.String()
}

// RenderRaw returns the terminal window content without trimming trailing spaces.
// Each line is exactly cols characters long.
func (vt *VirtualTerminal) RenderRaw() string {
	vt.mu.Lock()
	defer vt.mu.Unlock()

	var sb strings.Builder
	for r := 0; r < vt.rows; r++ {
		if r > 0 {
			sb.WriteByte('\n')
		}
		for c := 0; c < vt.cols; c++ {
			sb.WriteRune(vt.cells[r][c])
		}
	}
	return sb.String()
}

// Size returns the current terminal dimensions (rows, cols).
func (vt *VirtualTerminal) Size() (int, int) {
	vt.mu.Lock()
	defer vt.mu.Unlock()
	return vt.rows, vt.cols
}

// CursorPosition returns the current cursor position (row, col).
func (vt *VirtualTerminal) CursorPosition() (int, int) {
	vt.mu.Lock()
	defer vt.mu.Unlock()
	return vt.cursorR, vt.cursorC
}

// ScrollbackLines returns the number of lines in the scrollback buffer.
func (vt *VirtualTerminal) ScrollbackLines() int {
	vt.mu.Lock()
	defer vt.mu.Unlock()
	return len(vt.scrollback)
}

// SetScrollbackSize sets the maximum number of scrollback lines to keep.
func (vt *VirtualTerminal) SetScrollbackSize(n int) {
	vt.mu.Lock()
	defer vt.mu.Unlock()
	if n < 0 {
		n = 0
	}
	vt.scrollH = n
	if len(vt.scrollback) > vt.scrollH {
		vt.scrollback = vt.scrollback[len(vt.scrollback)-vt.scrollH:]
	}
}
