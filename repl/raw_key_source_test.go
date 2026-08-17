// RawKeySource ESC/byte sequence parsing tests (P2.5).
//
// Author: L.Shuang
// Created: 2026-08-16
// Last Modified: 2026-08-16
// MIT License - Copyright (c) 2026 L.Shuang

package repl

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/idirect3d/co-shell/agent"
)

// fakeByteReader feeds a fixed byte sequence to RawKeySource for parsing tests.
type fakeByteReader struct {
	data []byte
	pos  int
}

func newFakeByteReader(data ...byte) *fakeByteReader {
	return &fakeByteReader{data: data}
}

func (f *fakeByteReader) readByte(ctx context.Context) (byte, error) {
	if f.pos >= len(f.data) {
		return 0, io.EOF
	}
	b := f.data[f.pos]
	f.pos++
	return b, nil
}

func TestRawKeySourceParse(t *testing.T) {
	cases := []struct {
		name  string
		input []byte
		want  agent.InputEvent
	}{
		{"esc single", []byte{0x1b}, agent.InputEvent{Kind: agent.InputEsc}},
		{"arrow up", []byte{0x1b, '[', 'A'}, agent.InputEvent{Kind: agent.InputArrowUp}},
		{"arrow down", []byte{0x1b, '[', 'B'}, agent.InputEvent{Kind: agent.InputArrowDn}},
		{"arrow right", []byte{0x1b, '[', 'C'}, agent.InputEvent{Kind: agent.InputArrowRt}},
		{"arrow left", []byte{0x1b, '[', 'D'}, agent.InputEvent{Kind: agent.InputArrowLt}},
		{"home CSI H", []byte{0x1b, '[', 'H'}, agent.InputEvent{Kind: agent.InputHome}},
		{"end CSI F", []byte{0x1b, '[', 'F'}, agent.InputEvent{Kind: agent.InputEnd}},
		{"home 1~", []byte{0x1b, '[', '1', '~'}, agent.InputEvent{Kind: agent.InputHome}},
		{"end 4~", []byte{0x1b, '[', '4', '~'}, agent.InputEvent{Kind: agent.InputEnd}},
		{"delete 3~", []byte{0x1b, '[', '3', '~'}, agent.InputEvent{Kind: agent.InputDelete}},
		{"ss3 home", []byte{0x1b, 'O', 'H'}, agent.InputEvent{Kind: agent.InputHome}},
		{"ss3 up", []byte{0x1b, 'O', 'A'}, agent.InputEvent{Kind: agent.InputArrowUp}},
		{"ctrl-c", []byte{0x03}, agent.InputEvent{Kind: agent.InputCtrlC}},
		{"enter cr", []byte{'\r'}, agent.InputEvent{Kind: agent.InputEnter}},
		{"enter lf", []byte{'\n'}, agent.InputEvent{Kind: agent.InputEnter}},
		{"tab", []byte{'\t'}, agent.InputEvent{Kind: agent.InputTab}},
		{"backspace del", []byte{0x7f}, agent.InputEvent{Kind: agent.InputBackspace}},
		{"backspace bs", []byte{'\b'}, agent.InputEvent{Kind: agent.InputBackspace}},
		{"ascii key", []byte{'a'}, agent.InputEvent{Kind: agent.InputKey, Data: "a"}},
		{"ctrl-d", []byte{0x04}, agent.InputEvent{Kind: agent.InputKey, Data: "\x04"}},
		{"ctrl-a", []byte{0x01}, agent.InputEvent{Kind: agent.InputKey, Data: "\x01"}},
		{"utf8 chinese", []byte{0xe4, 0xbd, 0xa0}, agent.InputEvent{Kind: agent.InputKey, Data: "你"}},
		{"utf8 emoji", []byte{0xf0, 0x9f, 0x9a, 0x80}, agent.InputEvent{Kind: agent.InputKey, Data: "🚀"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &RawKeySource{r: newFakeByteReader(tc.input...)}
			ev, err := s.NextEvent(context.Background())
			if err != nil {
				t.Fatalf("NextEvent: %v", err)
			}
			if ev.Kind != tc.want.Kind || ev.Data != tc.want.Data {
				t.Fatalf("got %+v, want %+v", ev, tc.want)
			}
		})
	}
}

func TestRawKeySourceEOF(t *testing.T) {
	s := &RawKeySource{r: newFakeByteReader()}
	ev, err := s.NextEvent(context.Background())
	if err == nil || ev.Kind != agent.InputEOF {
		t.Fatalf("got (%+v, %v), want (InputEOF, io.EOF)", ev, err)
	}
}

// blockingByteReader simulates a real terminal: readByte blocks until a byte
// is pushed via push, or the context is cancelled. Unlike fakeByteReader it
// never returns io.EOF, which is what exposed FIX-357 (a lone ESC keypress
// blocked forever waiting for a second byte).
type blockingByteReader struct {
	ch chan byte
}

func newBlockingByteReader() *blockingByteReader {
	return &blockingByteReader{ch: make(chan byte, 8)}
}

func (b *blockingByteReader) push(data ...byte) {
	for _, d := range data {
		b.ch <- d
	}
}

func (b *blockingByteReader) readByte(ctx context.Context) (byte, error) {
	select {
	case v := <-b.ch:
		return v, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// TestRawKeySourceLoneEscTimeout is the FIX-357 regression test: a standalone
// ESC (no trailing bytes) must surface as InputEsc within the escape-sequence
// timeout instead of blocking until some unrelated key arrives.
func TestRawKeySourceLoneEscTimeout(t *testing.T) {
	r := newBlockingByteReader()
	s := &RawKeySource{r: r}
	r.push(0x1b)

	type result struct {
		ev  agent.InputEvent
		err error
	}
	resCh := make(chan result, 1)
	go func() {
		ev, err := s.NextEvent(context.Background())
		resCh <- result{ev, err}
	}()

	select {
	case res := <-resCh:
		if res.err != nil || res.ev.Kind != agent.InputEsc {
			t.Fatalf("got (%+v, %v), want (InputEsc, nil)", res.ev, res.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("lone ESC was not detected within 2s (escape-sequence read blocked)")
	}
}

// TestRawKeySourceEscThenDelayedKey verifies that a byte arriving after the
// escape-sequence timeout is parsed as its own key, not swallowed into the
// ESC sequence.
func TestRawKeySourceEscThenDelayedKey(t *testing.T) {
	r := newBlockingByteReader()
	s := &RawKeySource{r: r}
	r.push(0x1b)

	ev, err := s.NextEvent(context.Background())
	if err != nil || ev.Kind != agent.InputEsc {
		t.Fatalf("first event: got (%+v, %v), want (InputEsc, nil)", ev, err)
	}

	time.Sleep(escSeqTimeout * 3)
	r.push('x')
	ev, err = s.NextEvent(context.Background())
	if err != nil || ev.Kind != agent.InputKey || ev.Data != "x" {
		t.Fatalf("second event: got (%+v, %v), want (InputKey x, nil)", ev, err)
	}
}

// TestRawKeySourceArrowWithBlockingReader ensures real escape sequences are
// still recognised (not mistaken for lone ESC) with a blocking reader.
func TestRawKeySourceArrowWithBlockingReader(t *testing.T) {
	r := newBlockingByteReader()
	s := &RawKeySource{r: r}
	r.push(0x1b, '[', 'A')

	ev, err := s.NextEvent(context.Background())
	if err != nil || ev.Kind != agent.InputArrowUp {
		t.Fatalf("got (%+v, %v), want (InputArrowUp, nil)", ev, err)
	}
}

// TestRawKeySourceEscParentCancel verifies that a Pause/Close (parent context
// cancellation) mid escape-sequence propagates the error instead of reporting
// a phantom ESC keypress.
func TestRawKeySourceEscParentCancel(t *testing.T) {
	r := newBlockingByteReader()
	s := &RawKeySource{r: r}
	ctx, cancel := context.WithCancel(context.Background())
	r.push(0x1b)

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	_, err := s.NextEvent(ctx)
	if err == nil {
		t.Fatal("got nil error, want context.Canceled on parent cancel")
	}
}