// RawKeySource implements agent.InputSource for interactive (tui) mode.
//
// Author: L.Shuang
// Created: 2026-08-16
// Last Modified: 2026-08-16
// MIT License - Copyright (c) 2026 L.Shuang

package repl

import (
	"context"
	"io"
	"unicode/utf8"

	"github.com/idirect3d/co-shell/agent"
)

// byteReader abstracts a cancellable single-byte stdin reader so the
// platform-specific implementation (unix.Poll on POSIX, goroutine + CancelIoEx
// on Windows) is hidden from RawKeySource. The lowercase name avoids a vet
// conflict with io.ByteReader's ReadByte signature.
type byteReader interface {
	// readByte blocks until one byte is available, EOF, or ctx is cancelled.
	readByte(ctx context.Context) (byte, error)
}

// RawKeySource reads raw bytes from stdin and parses them into semantic
// InputEvents (arrow keys, ESC, Ctrl+C, tab, backspace, enter, UTF-8 chars).
// It is the tui-mode input source. Only one goroutine reads stdin via this
// source — the InputReader owns that goroutine (see input_reader.go).
type RawKeySource struct {
	r byteReader
}

// NewRawKeySource creates a RawKeySource bound to os.Stdin.
func NewRawKeySource() *RawKeySource {
	return &RawKeySource{r: newByteReader()}
}

// NextEvent reads and parses the next input event.
func (s *RawKeySource) NextEvent(ctx context.Context) (agent.InputEvent, error) {
	b, err := s.r.readByte(ctx)
	if err != nil {
		if err == io.EOF {
			return agent.InputEvent{Kind: agent.InputEOF}, io.EOF
		}
		return agent.InputEvent{}, err
	}
	return s.parseByte(ctx, b)
}

// parseByte interprets a single byte (possibly consuming a full ESC sequence
// or multi-byte UTF-8 rune) into a semantic InputEvent.
func (s *RawKeySource) parseByte(ctx context.Context, b byte) (agent.InputEvent, error) {
	switch {
	case b == 0x1b:
		seq, err := s.readEscapeSequence(ctx)
		if err != nil {
			// A lone ESC with no trailing sequence is the ESC key itself.
			if err == io.EOF || err == context.Canceled {
				return agent.InputEvent{Kind: agent.InputEsc}, nil
			}
			return agent.InputEvent{Kind: agent.InputEsc}, nil
		}
		switch seq {
		case "up":
			return agent.InputEvent{Kind: agent.InputArrowUp}, nil
		case "down":
			return agent.InputEvent{Kind: agent.InputArrowDn}, nil
		case "left":
			return agent.InputEvent{Kind: agent.InputArrowLt}, nil
		case "right":
			return agent.InputEvent{Kind: agent.InputArrowRt}, nil
		case "home":
			return agent.InputEvent{Kind: agent.InputHome}, nil
		case "end":
			return agent.InputEvent{Kind: agent.InputEnd}, nil
		case "del":
			return agent.InputEvent{Kind: agent.InputDelete}, nil
		default:
			return agent.InputEvent{Kind: agent.InputKey, Data: seq}, nil
		}
	case b == 0x03:
		return agent.InputEvent{Kind: agent.InputCtrlC}, nil
	case b == '\r' || b == '\n':
		return agent.InputEvent{Kind: agent.InputEnter}, nil
	case b == '\t':
		return agent.InputEvent{Kind: agent.InputTab}, nil
	case b == 0x7f || b == '\b':
		return agent.InputEvent{Kind: agent.InputBackspace}, nil
	case b >= 0x20 && b < 0x7f:
		return agent.InputEvent{Kind: agent.InputKey, Data: string(b)}, nil
	case b >= 0xc0:
		// Multi-byte UTF-8 character.
		r, size := s.readRune(ctx, b)
		if r == utf8.RuneError && size <= 1 {
			return agent.InputEvent{Kind: agent.InputKey, Data: "\ufffd"}, nil
		}
		return agent.InputEvent{Kind: agent.InputKey, Data: string(r)}, nil
	default:
		// Other control bytes (Ctrl+A/E/W/U/L/D etc.) are passed through so
		// the line editor can keep its current handling.
		return agent.InputEvent{Kind: agent.InputKey, Data: string([]byte{b})}, nil
	}
}

// readEscapeSequence reads a complete ANSI escape sequence after the leading
// ESC byte and returns its semantic name ("up", "home", "del", ...) or ""
// when the ESC was not followed by a recognised sequence.
func (s *RawKeySource) readEscapeSequence(ctx context.Context) (string, error) {
	b, err := s.r.readByte(ctx)
	if err != nil {
		return "", err
	}
	if b == '[' {
		return s.readCSI(ctx)
	}
	if b == 'O' {
		return s.readSS3(ctx)
	}
	return "", nil
}

// readCSI reads a CSI sequence (ESC [ ... ) and maps it to a semantic name.
func (s *RawKeySource) readCSI(ctx context.Context) (string, error) {
	var seq []byte
	for {
		b, err := s.r.readByte(ctx)
		if err != nil {
			return "", err
		}
		seq = append(seq, b)
		if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || b == '~' {
			break
		}
	}
	switch string(seq) {
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

// readSS3 reads an SS3 sequence (ESC O ... ) and maps it to a semantic name.
func (s *RawKeySource) readSS3(ctx context.Context) (string, error) {
	b, err := s.r.readByte(ctx)
	if err != nil {
		return "", err
	}
	switch b {
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

// readRune reads the remaining bytes of a multi-byte UTF-8 rune.
func (s *RawKeySource) readRune(ctx context.Context, first byte) (rune, int) {
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
		b, err := s.r.readByte(ctx)
		if err != nil {
			return utf8.RuneError, i
		}
		raw[i] = b
	}
	r, _ := utf8.DecodeRune(raw)
	return r, size
}
