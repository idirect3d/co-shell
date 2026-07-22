// Author: L.Shuang
// Created: 2026-07-20
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

//go:build !windows

package agent

import (
	"os/exec"
	"syscall"
)

// acpEncodeString is a no-op on Unix platforms — the shell natively supports UTF-8.
func acpEncodeString(command string) string {
	return command
}

// acpDecodeString is a no-op on Unix platforms — the shell natively outputs UTF-8.
func acpDecodeString(s string) string {
	return s
}

// isSignaledExit returns true if the error indicates the process was killed by
// a signal (e.g., timeout). On Unix, this is detected via WaitStatus.Signaled().
func isSignaledExit(err error) bool {
	if exiterr, ok := err.(*exec.ExitError); ok {
		if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
			return status.Signaled()
		}
	}
	return false
}

// setProcessGroupAttr configures the command to run in its own process group.
// On Unix, this is done via Setpgid which ensures the shell and all piped
// children share the same PGID. This allows killing the entire process tree
// on timeout.
func setProcessGroupAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup kills the entire process group of the given command's PID.
// Negative PID sends the signal to the process group (PGID).
func killProcessGroup(cmd *exec.Cmd) {
	syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
