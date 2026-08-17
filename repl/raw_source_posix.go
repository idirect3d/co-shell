//go:build !windows

// POSIX cancellable byte reader for RawKeySource.
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

	"golang.org/x/sys/unix"
)

// pollByteReader reads single bytes from stdin using unix.Poll with a short
// timeout so that context cancellation (used by InputReader.Pause/Close) is
// honoured while no data is available.
type pollByteReader struct {
	fd int
}

// newByteReader creates the platform byte reader for stdin.
func newByteReader() byteReader {
	return &pollByteReader{fd: int(os.Stdin.Fd())}
}

// readByte blocks until a byte is available, EOF, or ctx is cancelled.
func (r *pollByteReader) readByte(ctx context.Context) (byte, error) {
	pfds := []unix.PollFd{{Fd: int32(r.fd), Events: unix.POLLIN}}
	for {
		n, err := unix.Poll(pfds, 100)
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			return 0, err
		}
		if n == 0 {
			// Timeout: give the context a chance to cancel (Pause/Close).
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			default:
			}
			continue
		}
		buf := make([]byte, 1)
		nr, err := syscall.Read(r.fd, buf)
		if err != nil {
			if err == syscall.EINTR || err == syscall.EAGAIN {
				continue
			}
			return 0, err
		}
		if nr == 0 {
			return 0, io.EOF
		}
		return buf[0], nil
	}
}
