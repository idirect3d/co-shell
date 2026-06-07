//go:build !windows

package repl

import (
	"os"
	"syscall"

	"github.com/idirect3d/co-shell/log"
	"golang.org/x/sys/unix"
)

// startESCMonitor monitors stdin for ESC key using unix.Poll with 100ms timeout.
// Terminal must already be in raw mode before calling this.
// CRITICAL: This goroutine must NOT read stdin bytes while a system command
// (e.g. sudo, passwd) is running with stdin connected. The Agent sets
// commandRunning=true before such commands execute, and we skip polling
// entirely when commandRunning is true to avoid stealing stdin bytes
// from the sub-process.
func (r *REPL) startESCMonitor() func() {
	stopCh := make(chan struct{})
	r.escWg.Add(1)

	go func() {
		defer r.escWg.Done()

		fd := int(os.Stdin.Fd())
		buf := make([]byte, 1)
		pollFds := []unix.PollFd{
			{Fd: int32(fd), Events: unix.POLLIN},
		}

		for {
			// Poll with 100ms timeout, allowing stopCh to be checked
			n, err := unix.Poll(pollFds, 100)
			if err != nil {
				if err == unix.EINTR {
					continue
				}
				return
			}

			// Check if we should stop
			select {
			case <-stopCh:
				return
			default:
			}

			if n == 0 {
				// Timeout, no data available - loop and re-check stopCh
				continue
			}

			// Check if the agent has a UserIO that is currently reading input.
			// If so, skip this poll cycle to avoid data races on stdin.
			if io := r.agent.IO(); io != nil && io.IsReading() {
				// User is being prompted for input (confirmation, etc.).
				// Skip this poll cycle — the 100ms timeout will bring us back.
				continue
			}

			// CRITICAL FIX-209: If a system command (e.g. sudo, passwd) is currently
			// executing with stdin connected, skip polling entirely. The sub-process
			// is reading stdin bytes directly, and we must not compete with it.
			// This fixes the bug where sudo password input was being consumed by the
			// ESC monitor and ignored (only 0x1b triggers an action).
			if r.agent.IsCommandRunning() {
				continue
			}

			// Data available, read one byte (non-blocking because poll said data is ready)
			nRead, err := syscall.Read(fd, buf)
			if err != nil || nRead == 0 {
				return
			}

			if buf[0] == 0x1b {
				log.Info("ESC detected!")
				r.agent.Interrupt()
				// Do NOT return here. The goroutine must keep monitoring for
				// subsequent ESC presses after a retry. The goroutine exits
				// only when stopCh is closed (after RunStream returns).
			}
			// Ignore any other bytes received
		}
	}()

	return func() {
		close(stopCh)
		r.escWg.Wait()
	}
}
