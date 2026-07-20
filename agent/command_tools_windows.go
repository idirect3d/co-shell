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
// IMPLIED, BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

//go:build windows

package agent

import (
	"os/exec"
)

// isSignaledExit returns true if the error indicates the process was killed by
// a signal (e.g., timeout). On Windows, there is no Unix-style signal, so we
// return false here. The timeout goroutine kills with Process.Kill() which
// produces an expected ExitError — we always treat that as a timeout.
func isSignaledExit(err error) bool {
	// On Windows, process.Kill() always produces an ExitError with exit code -1.
	// We return true to treat any kill as a signal-style exit.
	if _, ok := err.(*exec.ExitError); ok {
		return true
	}
	return false
}

// setProcessGroupAttr configures the command to run in its own process group.
// On Windows, process group management via Setpgid is not available.
// This is a no-op; the process runs in its own default group.
func setProcessGroupAttr(cmd *exec.Cmd) {
	// No-op: Windows does not support Setpgid.
	// cmd.SysProcAttr can be set with CreationFlags for job objects if needed.
}

// killProcessGroup kills the command's process tree.
// On Windows, process.Kill() terminates the process and its children.
func killProcessGroup(cmd *exec.Cmd) {
	cmd.Process.Kill()
}
