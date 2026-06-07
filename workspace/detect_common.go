// Author: L.Shuang
// Created: 2026-06-07
// Last Modified: 2026-06-07
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

package workspace

import (
	"os"
	"path/filepath"
)

// DetectDefaultRoot determines the default workspace root directory based on
// how the application was launched:
//
//   - If launched from a terminal, returns the current working directory.
//   - If launched via double-click (GUI), returns the directory of the
//     executable itself.
//
// The distinction is made by checking whether stdin is connected to a
// terminal (TTY). On macOS/Linux this uses unix.Isatty(); on Windows it
// uses windows.GetConsoleMode().
//
// Platform-specific detection is implemented in detect.go (!windows) and
// detect_windows.go (windows).
func DetectDefaultRoot() (string, error) {
	if isTerminalLaunch() {
		// Terminal: use current working directory
		return os.Getwd()
	}

	// GUI / double-click: use executable's directory
	exePath, err := os.Executable()
	if err != nil {
		// Fallback to current working directory on error
		return os.Getwd()
	}
	return filepath.Dir(exePath), nil
}
