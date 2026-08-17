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