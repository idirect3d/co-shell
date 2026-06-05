//go:build windows

package repl

// startESCMonitor is a no-op on Windows (no unix.Poll available).
// ESC key monitoring is not supported on Windows.
func (r *REPL) startESCMonitor() func() {
	return func() {} // no-op stop function
}
