// Author: L.Shuang
// Created: 2026-09-27
// Last Modified: 2026-09-27
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

//go:build windows

package subagent

import (
	"os"
	"os/exec"
	"syscall"
)

// processAlive reports whether the process with the given pid still exists.
// Windows has no signal 0 idiom, so the handle-based Signal(0) probe is used
// as the closest equivalent.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}

// setProcAttr is a no-op on Windows: process groups cannot be created the
// POSIX way, termination falls back to Process.Kill.
func setProcAttr(cmd *exec.Cmd) {}

// killGroup terminates the process. Windows has no process-group signal
// delivery, so only the direct child can be killed; pgid/sig are unused.
func killGroup(pid, pgid int, sig syscall.Signal) error {
	if pid <= 0 {
		return nil
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}

// readPGID returns pid: Windows has no process-group id.
func readPGID(pid int) int {
	return pid
}
