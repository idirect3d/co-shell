//go:build windows

package shell

import (
	"os/exec"
	"syscall"
)

// sysProcAttr returns nil on Windows (no process group support).
func sysProcAttr() *syscall.SysProcAttr {
	return nil
}

// killProcess kills the process on Windows.
func killProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
