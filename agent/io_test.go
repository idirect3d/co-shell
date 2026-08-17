// Package agent - DefaultUserIO line reading tests (FIX-358).
//
// Author: L.Shuang
// Created: 2026-08-18
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"io"
	"strings"
	"testing"
)

// TestDefaultUserIOReadLineEOF verifies that a clean EOF is reported as
// io.EOF, distinguishable from an empty line (bufio.Scanner.Err() returns nil
// at EOF, which previously made EOF indistinguishable and let wizard loops
// spin forever on piped/closed stdin).
func TestDefaultUserIOReadLineEOF(t *testing.T) {
	d := &DefaultUserIO{input: strings.NewReader("")}
	line, err := d.ReadLine()
	if line != "" || err != io.EOF {
		t.Fatalf("got (%q, %v), want (\"\", io.EOF)", line, err)
	}
	// EOF is sticky: subsequent calls keep reporting io.EOF (no re-read race).
	if _, err := d.ReadLine(); err != io.EOF {
		t.Fatalf("second call: got err %v, want io.EOF", err)
	}
}

// TestDefaultUserIOReadLineNoReadAheadLoss verifies the persistent scanner
// keeps bytes read ahead from the source: two piped lines delivered in one
// write must both be returned, in order.
func TestDefaultUserIOReadLineNoReadAheadLoss(t *testing.T) {
	d := &DefaultUserIO{input: strings.NewReader("first\nsecond\n")}
	line, err := d.ReadLine()
	if err != nil || line != "first" {
		t.Fatalf("first: got (%q, %v), want (\"first\", nil)", line, err)
	}
	line, err = d.ReadLine()
	if err != nil || line != "second" {
		t.Fatalf("second: got (%q, %v), want (\"second\", nil)", line, err)
	}
	if _, err := d.ReadLine(); err != io.EOF {
		t.Fatalf("third: got err %v, want io.EOF", err)
	}
}

// TestDefaultUserIOReadLineEmptyLine verifies an empty line is still a valid
// ("", nil) result, distinct from EOF.
func TestDefaultUserIOReadLineEmptyLine(t *testing.T) {
	d := &DefaultUserIO{input: strings.NewReader("\n")}
	line, err := d.ReadLine()
	if line != "" || err != nil {
		t.Fatalf("got (%q, %v), want (\"\", nil)", line, err)
	}
}
