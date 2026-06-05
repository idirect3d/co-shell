//go:build windows

package repl

import "fmt"

// MakeRaw returns an error on Windows — raw terminal mode is not supported.
// EnhancedIO will fall back to non-raw behavior when this fails.
func MakeRaw(fd int) (interface{}, error) {
	return nil, fmt.Errorf("raw terminal mode not available on Windows")
}

// RestoreTerm is a no-op on Windows.
func RestoreTerm(fd int, old interface{}) error {
	return nil
}
