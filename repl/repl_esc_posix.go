//go:build !windows

package repl

import (
	"log"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// startESCMonitor monitors stdin for ESC key using unix.Poll with 100ms timeout.
// Terminal must already be in raw mode before calling this.
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

			// Data available, read one byte (non-blocking because poll said data is ready)
			nRead, err := syscall.Read(fd, buf)
			if err != nil || nRead == 0 {
				return
			}

			if buf[0] == 0x1b {
				log.Println("ESC detected!")
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
