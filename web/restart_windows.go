//go:build windows

package web

// requestRestart is a no-op on Windows: there is no SIGHUP equivalent and
// Windows services are typically restarted by the service manager (FEATURE-398).
func requestRestart() {
}
