//go:build windows

// Windows cancellable byte reader for RawKeySource.
//
// Author: L.Shuang
// Created: 2026-08-16
// Last Modified: 2026-08-16
// MIT License - Copyright (c) 2026 L.Shuang

package repl

import (
	"context"
	"io"
	"os"
	"syscall"
)

// winByteReader reads single bytes from the Windows console. Each ReadByte
// spawns a short-lived goroutine so a pending ReadFile can be interrupted by
// CancelIoEx when the context is cancelled (used by InputReader.Pause/Close).
// This is what makes ESC/Ctrl+C monitoring work on Windows: in raw mode both
// arrive as regular bytes (0x1b / 0x03) on stdin.
type winByteReader struct {
	fd int
	h  syscall.Handle
}

// newByteReader creates the platform byte reader for stdin.
func newByteReader() byteReader {
	fd := int(os.Stdin.Fd())
	return &winByteReader{fd: fd, h: syscall.Handle(fd)}
}

// readByte blocks until a byte is available, EOF, or ctx is cancelled.
func (r *winByteReader) readByte(ctx context.Context) (byte, error) {
	resCh := make(chan byte, 1)
	errCh := make(chan error, 1)
	go func() {
		buf := make([]byte, 1)
		n, err := os.Stdin.Read(buf)
		if err != nil {
			errCh <- err
			return
		}
		if n == 0 {
			errCh <- io.EOF
			return
		}
		resCh <- buf[0]
	}()

	select {
	case b := <-resCh:
		return b, nil
	case err := <-errCh:
		return 0, err
	case <-ctx.Done():
		// Interrupt the pending ReadFile so the goroutine can exit.
		cancelIoEx(r.h)
		return 0, ctx.Err()
	}
}

// CancelIoEx is declared on the shared kernel32 DLL (see raw_term_windows.go).
var procCancelIoEx = kernel32.NewProc("CancelIoEx")

// cancelIoEx cancels the pending synchronous read on the console handle.
func cancelIoEx(h syscall.Handle) {
	_, _, _ = procCancelIoEx.Call(uintptr(h), 0)
}
