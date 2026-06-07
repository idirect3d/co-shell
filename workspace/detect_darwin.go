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

//go:build darwin

package workspace

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// isTerminalLaunch returns true if we have high confidence the program was
// launched from an interactive terminal session.
//
// On macOS, double-clicking an executable in Finder opens Terminal.app,
// which allocates a PTY — so a simple TTY check on stdin always returns
// true even for GUI launches. To disambiguate, we use an additional
// heuristic: if the current working directory is the user's home directory
// and the executable lives elsewhere, it's almost certainly a double-click
// launch (Finder always starts with CWD=$HOME). Real terminal users rarely
// cd to $HOME just to run a binary located in a completely different path.
func isTerminalLaunch() bool {
	// First, check if stdin is a TTY at all.
	_, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TIOCGETA)
	if err != nil {
		return false
	}

	// Stdin is a TTY. On macOS this could be either a real terminal session
	// or a double-click that opened Terminal.app. Use a heuristic:
	// if CWD is $HOME and the executable is NOT under $HOME, it's a
	// double-click (Finder starts with CWD=$HOME).
	home, homeErr := os.UserHomeDir()
	if homeErr != nil {
		// Cannot determine home dir, conservatively treat as terminal.
		return true
	}

	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		return true
	}

	exePath, exeErr := os.Executable()
	if exeErr != nil {
		return true
	}
	exeDir := filepath.Dir(exePath)

	// If CWD is the user's home directory and the executable is not
	// inside the home directory, this is almost certainly a double-click.
	if cwd == home && exeDir != home {
		return false
	}

	return true
}
