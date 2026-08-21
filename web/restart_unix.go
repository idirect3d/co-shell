//go:build !windows

package web

import (
	"os"
	"syscall"
)

// requestRestart sends a SIGHUP to the current process so an external
// supervisor (launchd/systemd) restarts the service (FEATURE-398).
func requestRestart() {
	syscall.Kill(os.Getpid(), syscall.SIGHUP)
}
