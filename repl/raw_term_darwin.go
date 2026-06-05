//go:build darwin

package repl

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// MakeRaw puts the terminal into raw mode and returns the original state.
func MakeRaw(fd int) (interface{}, error) {
	old, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return nil, fmt.Errorf("failed to get terminal attributes: %w", err)
	}
	raw := *old
	raw.Iflag &^= unix.BRKINT | unix.ICRNL | unix.INPCK | unix.ISTRIP | unix.IXON
	raw.Oflag &^= unix.OPOST
	raw.Cflag &^= unix.CSIZE | unix.PARENB
	raw.Cflag |= unix.CS8
	raw.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	raw.Cc[unix.VMIN] = 1
	raw.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(fd, unix.TIOCSETA, &raw); err != nil {
		return nil, fmt.Errorf("failed to set raw terminal mode: %w", err)
	}
	return old, nil
}

// RestoreTerm restores the terminal to its original state.
func RestoreTerm(fd int, old interface{}) error {
	if old == nil {
		return nil
	}
	termios, ok := old.(*unix.Termios)
	if !ok {
		return nil
	}
	return unix.IoctlSetTermios(fd, unix.TIOCSETA, termios)
}
