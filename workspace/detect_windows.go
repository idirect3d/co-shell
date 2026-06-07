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

//go:build windows

package workspace

import (
	"os"

	"golang.org/x/sys/windows"
)

// isTerminalLaunch returns true if stdin is connected to a console.
//
// On Windows, this is determined by calling GetConsoleMode on the stdin
// handle. If GetConsoleMode succeeds, stdin is attached to a console
// (terminal / cmd.exe / PowerShell). If it fails, the program was likely
// launched via double-click or from a non-console context.
func isTerminalLaunch() bool {
	var mode uint32
	err := windows.GetConsoleMode(windows.Handle(os.Stdin.Fd()), &mode)
	return err == nil
}
