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
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"

	"github.com/idirect3d/co-shell/agent"
)

// StdioIO implements agent.UserIO for standard terminal I/O.
// Used when input mode is "stdio".
type StdioIO struct {
	src     *StdioSource
	fd      int
	rawTerm bool
}

// NewStdioIO creates a new StdioIO instance.
func NewStdioIO() *StdioIO {
	return &StdioIO{
		src: NewStdioSource(),
		fd:  int(os.Stdin.Fd()),
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

func (s *StdioIO) ErrPrintf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
}

func (s *StdioIO) ReadLine() (string, error) {
	ev, err := s.src.NextEvent(context.Background())
	if err != nil {
		return "", err
	}
	if ev.Kind == agent.InputEOF {
		return "", io.EOF
	}
	return ev.Data, nil
}

func (s *StdioIO) ReadKey() (byte, error) {
	// Enable raw terminal mode so we get immediate single-byte reads
	// instead of line-buffered input. This is essential on Windows where
	// the console is line-buffered by default.
	var oldState interface{}
	oldState, err := MakeRaw(s.fd)
	if err == nil {
		s.rawTerm = true
	}
	if s.rawTerm && oldState != nil {
		defer RestoreTerm(s.fd, oldState)
	}

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
	// The ESC monitor consumer checks this so it yields while an exclusive
	// consumer (ReadLine/ReadKey) owns the input event.
	reading atomic.Bool

	// history shared with REPL
	history []string

	// reader is the unified input event source (tui mode, P2.5).
	reader *InputReader
}

// NewEnhancedIO creates a new EnhancedIO instance bound to the unified reader.
func NewEnhancedIO(history []string, reader *InputReader) *EnhancedIO {
	return &EnhancedIO{
		history: history,
		reader:  reader,
	}
}

func (e *EnhancedIO) Print(args ...interface{}) {
	if e.reader != nil && e.reader.RawActive() {
		// In raw mode, replace \n with \r\n so the cursor returns to column 0.
		s := fmt.Sprint(args...)
		s = strings.ReplaceAll(s, "\n", "\r\n")
		fmt.Print(s)
	} else {
		fmt.Print(args...)
	}
}

func (e *EnhancedIO) Printf(format string, args ...interface{}) {
	if e.reader != nil && e.reader.RawActive() {
		s := fmt.Sprintf(format, args...)
		s = strings.ReplaceAll(s, "\n", "\r\n")
		fmt.Print(s)
	} else {
		fmt.Printf(format, args...)
	}
}

func (e *EnhancedIO) Println(args ...interface{}) {
	if e.reader != nil && e.reader.RawActive() {
		s := fmt.Sprint(args...)
		s = strings.ReplaceAll(s, "\n", "\r\n")
		fmt.Print("\r" + s + "\r\n")
	} else {
		fmt.Println(args...)
	}
}

func (e *EnhancedIO) ErrPrintf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
}

func (e *EnhancedIO) ReadLine() (string, error) {
	e.reading.Store(true)
	defer e.reading.Store(false)

	ei := NewEnhancedInput("", e.history)
	input, err := ei.ReadLineFrom(e.reader)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

func (e *EnhancedIO) ReadKey() (byte, error) {
	e.reading.Store(true)
	defer e.reading.Store(false)

	resCh := make(chan byte, 1)
	var done atomic.Bool
	cancel := e.reader.Subscribe(func(ev agent.InputEvent) {
		if done.Load() {
			return
		}
		b, ok := keyEventToByte(ev)
		if !ok {
			return // arrow/home/end/delete are consumed
		}
		// Echo the key back to the user (legacy behaviour).
		if b >= 0x20 && b < 0x7f {
			fmt.Print(string(b))
		} else if b == '\r' || b == '\n' {
			fmt.Print("\r\n")
		}
		done.Store(true)
		resCh <- b
	})
	defer cancel()

	select {
	case b := <-resCh:
		return b, nil
	case <-e.reader.Done():
		return 0, io.EOF
	}
}

// keyEventToByte maps an input event to the legacy single-byte value used by
// ReadKey callers (confirmation prompts etc.). Arrow/home/end/delete are not
// "real" keys and are consumed, matching the legacy ReadKey that skipped
// escape sequences.
func keyEventToByte(ev agent.InputEvent) (byte, bool) {
	switch ev.Kind {
	case agent.InputKey:
		if ev.Data == "" {
			return 0, false
		}
		return ev.Data[0], true
	case agent.InputEsc:
		return 0x1b, true
	case agent.InputCtrlC:
		return 0x03, true
	case agent.InputEnter:
		return '\r', true
	case agent.InputTab:
		return '\t', true
	case agent.InputBackspace:
		return 0x7f, true
	default:
		return 0, false
	}
}

func (e *EnhancedIO) IsReading() bool {
	return e.reading.Load()
}

// Ensure StdioIO and EnhancedIO implement agent.UserIO.
var _ agent.UserIO = (*StdioIO)(nil)
var _ agent.UserIO = (*EnhancedIO)(nil)
