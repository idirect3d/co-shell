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

//go:build !windows

package subagent

import (
	"errors"
	"os/exec"
	"syscall"
)

// processAlive reports whether the process with the given pid still exists.
// Signal 0 performs the permission/existence check without delivering a
// signal; EPERM means the process exists but belongs to another user.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// setProcAttr puts the child into its own process group so the whole tree
// (child plus its grandchildren) can be terminated as one unit.
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killGroup sends sig to the whole process group of a sub-agent. It falls
// back to the process-group id derived from pid when pgid is unusable.
func killGroup(pid, pgid int, sig syscall.Signal) error {
	if pgid <= 0 {
		pgid = pid
	}
	if pgid <= 0 {
		return nil
	}
	if err := syscall.Kill(-pgid, sig); err == nil {
		return nil
	}
	return syscall.Kill(pid, sig)
}

// readPGID returns the process-group id of pid (pid itself when it cannot be
// determined, which keeps the kill fallback path working).
func readPGID(pid int) int {
	pgid, err := syscall.Getpgid(pid)
	if err != nil || pgid <= 0 {
		return pid
	}
	return pgid
}
