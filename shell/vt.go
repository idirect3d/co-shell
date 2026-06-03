// Author: L.Shuang
// Created: 2026-05-31
// Last Modified: 2026-06-01
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

package shell

import (
	"strings"
	"sync"

	"github.com/idirect3d/co-shell/log"
)

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

// VirtualTerminal emulates a character-cell terminal.
type VirtualTerminal struct {
	mu sync.Mutex

	cells      [][]rune
	rows       int
	cols       int
	scrollH    int
	cursorR    int
	cursorC    int
	scrollback [][]rune

	ansiState int
	paramBuf  []int
	paramStr  []byte
	oscBuf    []byte

	// utf8Buf accumulates UTF-8 multi-byte sequences.
	utf8Buf []byte

	// lineBuf accumulates displayable characters for the current line.
	// \r resets the buffer. \n flushes it to log and lineCh.
	lineBuf strings.Builder
	// logWriter is where flushed lines are written (shell log file).
	logWriter interface{ Write([]byte) (int, error) }
	// lineCh is where flushed lines are pushed (for Exec idle detection).
	lineCh chan<- string
}

// NewVirtualTerminal creates a new virtual terminal.
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

// SetLineChannel sets the channel for complete line output.
func (vt *VirtualTerminal) SetLineChannel(ch chan<- string) {
	vt.mu.Lock()
	defer vt.mu.Unlock()
	vt.lineCh = ch
}

// SetLogWriter sets the writer for complete line output (shell log).
func (vt *VirtualTerminal) SetLogWriter(w interface{ Write([]byte) (int, error) }) {
	vt.mu.Lock()
	defer vt.mu.Unlock()
	vt.logWriter = w
}

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
	oldCells := vt.cells
	oldRows := vt.rows
	oldCols := vt.cols
	vt.rows = rows
	vt.cols = cols
	vt.allocCells()
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

func (vt *VirtualTerminal) Reset() {
	vt.mu.Lock()
	defer vt.mu.Unlock()
	vt.allocCells()
	vt.scrollback = nil
	vt.resetAnsiState()
	vt.lineBuf.Reset()
}

func (vt *VirtualTerminal) scrollUp() {
	if vt.scrollH > 0 {
		topLine := make([]rune, vt.cols)
		copy(topLine, vt.cells[0])
		vt.scrollback = append(vt.scrollback, topLine)
		if len(vt.scrollback) > vt.scrollH {
			vt.scrollback = vt.scrollback[len(vt.scrollback)-vt.scrollH:]
		}
	}
	copy(vt.cells, vt.cells[1:])
	lastRow := vt.rows - 1
	for c := 0; c < vt.cols; c++ {
		vt.cells[lastRow][c] = ' '
	}
}

func (vt *VirtualTerminal) writeChar(ch rune) {
	if vt.cursorC >= vt.cols {
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

func (vt *VirtualTerminal) Process(data []byte) {
	vt.mu.Lock()
	defer vt.mu.Unlock()
	for _, b := range data {
		vt.processByte(b)
	}
}

func (vt *VirtualTerminal) processByte(b byte) {
	switch vt.ansiState {
	case vtStateNormal:
		if b == 0x1b {
			vt.ansiState = vtStateESC
			return
		}
		switch b {
		case '\r':
			// Flush current line before overwriting (captures prompts like $, >>>).
			// Subsequent chars overwrite from column 1.
			vt.utf8Buf = nil // discard incomplete UTF-8 sequence
			if vt.lineBuf.Len() > 0 {
				vt.flushLine(vt.lineBuf.String() + "\n")
			}
			vt.lineBuf.Reset()
			vt.cursorC = 0
		case '\n':
			vt.utf8Buf = nil // discard incomplete UTF-8 sequence
			if vt.lineBuf.Len() > 0 {
				vt.flushLine(vt.lineBuf.String() + "\n")
			} else {
				vt.flushLine("\n")
			}
			vt.lineBuf.Reset()
			vt.cursorC = 0
			vt.cursorR++
			if vt.cursorR >= vt.rows {
				vt.scrollUp()
				vt.cursorR = vt.rows - 1
			}
		case '\b':
			if vt.cursorC > 0 {
				vt.cursorC--
			}
			s := vt.lineBuf.String()
			if len(s) > 0 {
				vt.lineBuf.Reset()
				vt.lineBuf.WriteString(s[:len(s)-1])
			}
		case '\t':
			nextTab := ((vt.cursorC / 8) + 1) * 8
			if nextTab > vt.cols {
				nextTab = vt.cols - 1
			}
			for vt.cursorC < nextTab {
				vt.writeChar(' ')
				vt.lineBuf.WriteRune(' ')
			}
			vt.cursorC = nextTab
		case '\x0b', '\x0c':
			vt.cursorR++
			if vt.cursorR >= vt.rows {
				vt.scrollUp()
				vt.cursorR = vt.rows - 1
			}
		case '\x07':
		default:
			if vt.utf8Buf != nil {
				// We are in the middle of a UTF-8 multi-byte sequence.
				// Continuation bytes must be 0x80-0xbf.
				vt.utf8Buf = append(vt.utf8Buf, b)
				if isCompleteUTF8(vt.utf8Buf) {
					r, _ := decodeUTF8(vt.utf8Buf)
					vt.utf8Buf = nil
					vt.writeChar(r)
					vt.lineBuf.WriteRune(r)
				}
				break
			}
			if b >= 0xc0 {
				// Start of a UTF-8 multi-byte sequence (2-4 bytes).
				vt.utf8Buf = append(vt.utf8Buf[:0], b)
				break
			}
			if b >= 0x20 || b == 0x00 {
				vt.writeChar(rune(b))
				vt.lineBuf.WriteRune(rune(b))
			}
		}

	case vtStateESC:
		if b == '[' {
			vt.ansiState = vtStateCSI
			vt.paramBuf = nil
			vt.paramStr = nil
			return
		}
		if b == ']' {
			vt.ansiState = vtStateOSC
			vt.oscBuf = nil
			return
		}
		switch b {
		case 'c':
			vt.allocCells()
		case 'D':
			vt.cursorR++
			if vt.cursorR >= vt.rows {
				vt.scrollUp()
				vt.cursorR = vt.rows - 1
			}
		case 'M':
			vt.cursorR--
			if vt.cursorR < 0 {
				vt.cursorR = 0
				copy(vt.cells[1:], vt.cells)
				for c := 0; c < vt.cols; c++ {
					vt.cells[0][c] = ' '
				}
			}
		case '7', '8':
		case '=', '>':
		}
		vt.resetAnsiState()

	case vtStateCSI:
		if len(vt.paramStr) > 64 {
			vt.resetAnsiState()
			return
		}
		if b >= '0' && b <= '9' {
			vt.paramStr = append(vt.paramStr, b)
			return
		}
		if b == ';' || b == ':' {
			vt.flushParam()
			vt.paramStr = append(vt.paramStr, b)
			return
		}
		if b == '?' {
			return
		}
		vt.flushParam()
		vt.executeCSI(b)
		vt.resetAnsiState()

	case vtStateOSC:
		if len(vt.oscBuf) > 4096 {
			log.Debug("VT: forced OSC exit at %d bytes", len(vt.oscBuf))
			vt.resetAnsiState()
			return
		}
		if b == 0x07 {
			vt.executeOSC()
			vt.resetAnsiState()
			return
		}
		if b == 0x1b {
			vt.oscBuf = append(vt.oscBuf, b)
			return
		}
		if b == '\\' && len(vt.oscBuf) > 0 && vt.oscBuf[len(vt.oscBuf)-1] == 0x1b {
			vt.executeOSC()
			vt.resetAnsiState()
			return
		}
		vt.oscBuf = append(vt.oscBuf, b)
	}
}

// flushLine writes a complete line to lineCh and logWriter.
func (vt *VirtualTerminal) flushLine(line string) {
	if vt.lineCh != nil {
		select {
		case vt.lineCh <- line:
		default:
		}
	}
	if vt.logWriter != nil {
		vt.logWriter.Write([]byte(line))
	}
}

func (vt *VirtualTerminal) flushParam() {
	if len(vt.paramStr) == 0 {
		vt.paramBuf = append(vt.paramBuf, 0)
		return
	}
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

func (vt *VirtualTerminal) param(n int, def int) int {
	if n < len(vt.paramBuf) {
		return vt.paramBuf[n]
	}
	return def
}

func (vt *VirtualTerminal) executeCSI(cmd byte) {
	switch cmd {
	case 'A':
		n := vt.param(0, 1)
		vt.cursorR -= n
		if vt.cursorR < 0 {
			vt.cursorR = 0
		}
	case 'B':
		n := vt.param(0, 1)
		vt.cursorR += n
		if vt.cursorR >= vt.rows {
			vt.cursorR = vt.rows - 1
		}
	case 'C':
		n := vt.param(0, 1)
		vt.cursorC += n
		if vt.cursorC >= vt.cols {
			vt.cursorC = vt.cols - 1
		}
	case 'D':
		n := vt.param(0, 1)
		vt.cursorC -= n
		if vt.cursorC < 0 {
			vt.cursorC = 0
		}
	case 'H', 'f':
		row := vt.param(0, 1) - 1
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
	case 'G':
		col := vt.param(0, 1) - 1
		if col < 0 {
			col = 0
		}
		if col >= vt.cols {
			col = vt.cols - 1
		}
		vt.cursorC = col
	case 'J':
		mode := vt.param(0, 0)
		switch mode {
		case 0:
			vt.eraseCursorToEnd()
		case 1:
			vt.eraseStartToCursor()
		case 2:
			vt.eraseEntireScreen()
		case 3:
			vt.scrollback = nil
		}
	case 'K':
		mode := vt.param(0, 0)
		switch mode {
		case 0:
			for c := vt.cursorC; c < vt.cols; c++ {
				vt.cells[vt.cursorR][c] = ' '
			}
		case 1:
			for c := 0; c <= vt.cursorC && c < vt.cols; c++ {
				vt.cells[vt.cursorR][c] = ' '
			}
		case 2:
			for c := 0; c < vt.cols; c++ {
				vt.cells[vt.cursorR][c] = ' '
			}
		}
	case 'L', 'M', 'P', '@', 'X':
	case 'm':
	case 's', 'u', 'r', 'h', 'l':
	}
}

func (vt *VirtualTerminal) eraseCursorToEnd() {
	if vt.cursorR < vt.rows {
		for c := vt.cursorC; c < vt.cols; c++ {
			vt.cells[vt.cursorR][c] = ' '
		}
		for r := vt.cursorR + 1; r < vt.rows; r++ {
			for c := 0; c < vt.cols; c++ {
				vt.cells[r][c] = ' '
			}
		}
	}
}
func (vt *VirtualTerminal) eraseStartToCursor() {
	for r := 0; r < vt.cursorR; r++ {
		for c := 0; c < vt.cols; c++ {
			vt.cells[r][c] = ' '
		}
	}
	if vt.cursorR < vt.rows {
		for c := 0; c <= vt.cursorC && c < vt.cols; c++ {
			vt.cells[vt.cursorR][c] = ' '
		}
	}
}
func (vt *VirtualTerminal) eraseEntireScreen() {
	for r := 0; r < vt.rows; r++ {
		for c := 0; c < vt.cols; c++ {
			vt.cells[r][c] = ' '
		}
	}
	vt.cursorR = 0
	vt.cursorC = 0
}
func (vt *VirtualTerminal) executeOSC() {}
func (vt *VirtualTerminal) resetAnsiState() {
	vt.ansiState = vtStateNormal
	vt.paramBuf = nil
	vt.paramStr = nil
	vt.oscBuf = nil
	vt.utf8Buf = nil
}
func (vt *VirtualTerminal) Render() string { vt.mu.Lock(); defer vt.mu.Unlock(); return vt.render() }
func (vt *VirtualTerminal) Size() (int, int) {
	vt.mu.Lock()
	defer vt.mu.Unlock()
	return vt.rows, vt.cols
}
func (vt *VirtualTerminal) CursorPosition() (int, int) {
	vt.mu.Lock()
	defer vt.mu.Unlock()
	return vt.cursorR, vt.cursorC
}
func (vt *VirtualTerminal) ScrollbackLines() int {
	vt.mu.Lock()
	defer vt.mu.Unlock()
	return len(vt.scrollback)
}

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

// isCompleteUTF8 checks whether buf contains a complete UTF-8 multi-byte sequence.
func isCompleteUTF8(buf []byte) bool {
	if len(buf) == 0 {
		return false
	}
	first := buf[0]
	var expectedLen int
	switch {
	case first >= 0xf0:
		expectedLen = 4
	case first >= 0xe0:
		expectedLen = 3
	case first >= 0xc0:
		expectedLen = 2
	default:
		return true
	}
	if len(buf) < expectedLen {
		return false
	}
	// Validate continuation bytes
	for i := 1; i < expectedLen; i++ {
		if buf[i]&0xc0 != 0x80 {
			return false
		}
	}
	return true
}

// decodeUTF8 decodes a complete UTF-8 sequence into a rune.
// Assumes the buffer has been validated by isCompleteUTF8.
func decodeUTF8(buf []byte) (rune, int) {
	if len(buf) == 0 {
		return 0, 0
	}
	switch {
	case buf[0] >= 0xf0:
		return rune(buf[0]&0x07)<<18 | rune(buf[1]&0x3f)<<12 | rune(buf[2]&0x3f)<<6 | rune(buf[3]&0x3f), 4
	case buf[0] >= 0xe0:
		return rune(buf[0]&0x0f)<<12 | rune(buf[1]&0x3f)<<6 | rune(buf[2]&0x3f), 3
	case buf[0] >= 0xc0:
		return rune(buf[0]&0x1f)<<6 | rune(buf[1]&0x3f), 2
	default:
		return rune(buf[0]), 1
	}
}

func (vt *VirtualTerminal) render() string {
	var sb strings.Builder
	for r := 0; r < vt.rows; r++ {
		if r > 0 {
			sb.WriteByte('\n')
		}
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
