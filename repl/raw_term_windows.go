//go:build windows

// Author: L.Shuang
// Created: 2026-06-05
// Last Modified: 2026-07-20
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
	"errors"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// MakeRaw puts the Windows console into raw mode and returns the original mode.
// It disables line input, echo input, and processed input so that ReadFile
// returns each keystroke immediately without waiting for Enter.
func MakeRaw(fd int) (interface{}, error) {
	if fd == 0 {
		fd = int(os.Stdin.Fd())
	}

	handle := syscall.Handle(fd)

	var mode uint32
	err := getConsoleMode(handle, &mode)
	if err != nil {
		return nil, fmt.Errorf("failed to get console mode: %w", err)
	}

	// Save original mode for restoration
	oldMode := mode

	// Disable line buffering, echo, and processed input
	// Enable virtual terminal input for ANSI escape sequence support
	mode &^= (ENABLE_LINE_INPUT | ENABLE_ECHO_INPUT | ENABLE_PROCESSED_INPUT)
	mode |= ENABLE_VIRTUAL_TERMINAL_INPUT

	err = setConsoleMode(handle, mode)
	if err != nil {
		return nil, fmt.Errorf("failed to set console mode: %w", err)
	}

	return oldMode, nil
}

// RestoreTerm restores the Windows console to its original mode.
func RestoreTerm(fd int, old interface{}) error {
	if old == nil {
		return nil
	}

	mode, ok := old.(uint32)
	if !ok {
		return nil
	}

	if fd == 0 {
		fd = int(os.Stdin.Fd())
	}

	handle := syscall.Handle(fd)
	return setConsoleMode(handle, mode)
}

// ============================================================================
// Windows Console API constants and syscalls
// ============================================================================

const (
	ENABLE_LINE_INPUT             = 0x0002
	ENABLE_ECHO_INPUT             = 0x0004
	ENABLE_PROCESSED_INPUT        = 0x0001
	ENABLE_VIRTUAL_TERMINAL_INPUT = 0x0200
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode = kernel32.NewProc("SetConsoleMode")
)

func getConsoleMode(handle syscall.Handle, mode *uint32) error {
	r0, _, e1 := procGetConsoleMode.Call(uintptr(handle), uintptr(unsafe.Pointer(mode)))
	if r0 == 0 {
		if e1 != nil {
			return e1
		}
		return errors.New("GetConsoleMode failed")
	}
	return nil
}

func setConsoleMode(handle syscall.Handle, mode uint32) error {
	r0, _, e1 := procSetConsoleMode.Call(uintptr(handle), uintptr(mode))
	if r0 == 0 {
		if e1 != nil {
			return e1
		}
		return errors.New("SetConsoleMode failed")
	}
	return nil
}
