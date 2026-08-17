// StdioSource implements agent.InputSource for standard streaming (stdio) mode.
//
// Author: L.Shuang
// Created: 2026-08-16
// Last Modified: 2026-08-16
// MIT License - Copyright (c) 2026 L.Shuang

package repl

import (
	"bufio"
	"context"
	"io"
	"os"

	"github.com/idirect3d/co-shell/agent"
)

// StdioSource reads whole lines from stdin (non-raw, pipe-friendly). It is
// used by --input-mode stdio and keeps the legacy bufio.Scanner behaviour.
type StdioSource struct {
	reader *bufio.Scanner
}

// NewStdioSource creates a StdioSource bound to os.Stdin.
func NewStdioSource() *StdioSource {
	return &StdioSource{reader: bufio.NewScanner(os.Stdin)}
}

// NextEvent returns the next InputLine, or (InputEOF, io.EOF) at end of input.
func (s *StdioSource) NextEvent(ctx context.Context) (agent.InputEvent, error) {
	if !s.reader.Scan() {
		if err := s.reader.Err(); err != nil {
			return agent.InputEvent{}, err
		}
		return agent.InputEvent{Kind: agent.InputEOF}, io.EOF
	}
	return agent.InputEvent{Kind: agent.InputLine, Data: s.reader.Text()}, nil
}
