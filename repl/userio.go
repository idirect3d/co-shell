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

package repl

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync/atomic"

	"github.com/idirect3d/co-shell/agent"
)

// StdioIO implements agent.UserIO for standard terminal I/O.
// Used when input mode is "stdio".
type StdioIO struct {
	reader *bufio.Scanner
}

// NewStdioIO creates a new StdioIO instance.
func NewStdioIO() *StdioIO {
	return &StdioIO{
		reader: bufio.NewScanner(os.Stdin),
	}
}

func (s *StdioIO) Print(args ...interface{}) {
	fmt.Print(args...)
}

func (s *StdioIO) Printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

func (s *StdioIO) Println(args ...interface{}) {
	fmt.Println(args...)
}

func (s *StdioIO) ReadLine() (string, error) {
	s.reader = bufio.NewScanner(os.Stdin)
	if !s.reader.Scan() {
		return "", s.reader.Err()
	}
	return s.reader.Text(), nil
}

func (s *StdioIO) ReadKey() (byte, error) {
	buf := make([]byte, 1)
	n, err := os.Stdin.Read(buf)
	if err != nil || n == 0 {
		return 0, err
	}
	return buf[0], nil
}

func (s *StdioIO) IsReading() bool {
	return false
}

// EnhancedIO implements agent.UserIO for enhanced interactive mode.
// It uses raw terminal mode for full line editing support.
type EnhancedIO struct {
	// reading is an atomic flag set to true while waiting for user input.
	// The ESC monitor goroutine checks this to avoid data races on stdin.
	reading atomic.Bool

	// raw term state management
	fd      int
	oldTerm interface{} // *unix.Termios on POSIX, nil on Windows
	inRaw   bool

	// history shared with REPL
	history []string
}

// NewEnhancedIO creates a new EnhancedIO instance.
func NewEnhancedIO(history []string) *EnhancedIO {
	return &EnhancedIO{
		fd:      int(os.Stdin.Fd()),
		history: history,
	}
}

// startRaw puts the terminal into raw mode.
func (e *EnhancedIO) startRaw() error {
	if e.inRaw {
		return nil
	}
	oldState, err := MakeRaw(e.fd)
	if err != nil {
		return fmt.Errorf("failed to set raw terminal mode: %w", err)
	}
	e.oldTerm = oldState
	e.inRaw = true
	return nil
}

// stopRaw restores the terminal from raw mode.
func (e *EnhancedIO) stopRaw() {
	if e.inRaw && e.oldTerm != nil {
		_ = RestoreTerm(e.fd, e.oldTerm)
		e.oldTerm = nil
		e.inRaw = false
	}
}

func (e *EnhancedIO) Print(args ...interface{}) {
	if e.inRaw {
		// In raw mode, replace \n with \r\n so the cursor returns to column 0.
		s := fmt.Sprint(args...)
		s = strings.ReplaceAll(s, "\n", "\r\n")
		fmt.Print(s)
	} else {
		fmt.Print(args...)
	}
}

func (e *EnhancedIO) Printf(format string, args ...interface{}) {
	if e.inRaw {
		s := fmt.Sprintf(format, args...)
		s = strings.ReplaceAll(s, "\n", "\r\n")
		fmt.Print(s)
	} else {
		fmt.Printf(format, args...)
	}
}

func (e *EnhancedIO) Println(args ...interface{}) {
	if e.inRaw {
		s := fmt.Sprint(args...)
		s = strings.ReplaceAll(s, "\n", "\r\n")
		fmt.Print("\r" + s + "\r\n")
	} else {
		fmt.Println(args...)
	}
}

func (e *EnhancedIO) ReadLine() (string, error) {
	e.reading.Store(true)
	defer e.reading.Store(false)

	// startRaw is a no-op if already in raw mode
	if err := e.startRaw(); err != nil {
		return "", err
	}

	ei := NewEnhancedInput("", e.history)
	input, err := ei.ReadLine()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

func (e *EnhancedIO) ReadKey() (byte, error) {
	e.reading.Store(true)
	defer e.reading.Store(false)

	if err := e.startRaw(); err != nil {
		return 0, err
	}

	buf := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			return 0, err
		}
		b := buf[0]

		// Handle escape sequences (ESC + [ + ...)
		if b == 0x1b {
			// Check if it's a full escape sequence
			seqBuf := make([]byte, 2)
			n, err = os.Stdin.Read(seqBuf)
			if err != nil || n == 0 {
				// Just ESC alone
				return b, nil
			}
			if seqBuf[0] == '[' || seqBuf[0] == 'O' {
				// Consume rest of sequence
				for {
					ch := make([]byte, 1)
					n, err = os.Stdin.Read(ch)
					if err != nil || n == 0 {
						break
					}
					if (ch[0] >= 'A' && ch[0] <= 'Z') || (ch[0] >= 'a' && ch[0] <= 'z') || ch[0] == '~' {
						break
					}
				}
				continue // skip escape sequences, wait for a real key
			}
			return b, nil
		}

		// Echo the key back to the user
		if b >= 0x20 && b < 0x7f {
			fmt.Print(string(b))
		} else if b == '\r' || b == '\n' {
			fmt.Print("\r\n")
		}

		return b, nil
	}
}

func (e *EnhancedIO) IsReading() bool {
	return e.reading.Load()
}

// Ensure StdioIO and EnhancedIO implement agent.UserIO.
var _ agent.UserIO = (*StdioIO)(nil)
var _ agent.UserIO = (*EnhancedIO)(nil)
