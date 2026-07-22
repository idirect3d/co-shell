// Author: L.Shuang
// Created: 2026-07-20
// Last Modified: 2026-07-22
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
// IMPLIED, BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

//go:build windows

package agent

import (
	"os/exec"
	"syscall"
	"unsafe"
)

// ============================================================================
// Win32 code page conversion API
// ============================================================================

const (
	CP_ACP  = 0 // system active code page
	CP_UTF8 = 65001
)

var (
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procGetACP              = kernel32.NewProc("GetACP")
	procMultiByteToWideChar = kernel32.NewProc("MultiByteToWideChar")
	procWideCharToMultiByte = kernel32.NewProc("WideCharToMultiByte")
)

// getACP returns the current Windows active code page (e.g. 936 for GBK,
// 932 for Shift-JIS, 949 for Korean, 950 for Big5).
func getACP() uint32 {
	cp, _, _ := procGetACP.Call()
	return uint32(cp)
}

// acpEncodeString converts a UTF-8 string to the system's active code page
// encoding, suitable for passing to cmd.exe on Windows.
func acpEncodeString(utf8Str string) string {
	if utf8Str == "" {
		return ""
	}

	// Append null terminator for Win32 API calls
	src := utf8Str + "\x00"
	acp := getACP()

	// Step 1: UTF-8 → UTF-16 (WideChar)
	wideLen, _, _ := procMultiByteToWideChar.Call(
		CP_UTF8, 0,
		uintptr(unsafe.Pointer(unsafe.StringData(src))),
		uintptr(len(src)),
		0, 0,
	)
	if wideLen == 0 {
		return utf8Str // fallback
	}
	wideBuf := make([]uint16, wideLen)
	procMultiByteToWideChar.Call(
		CP_UTF8, 0,
		uintptr(unsafe.Pointer(unsafe.StringData(src))),
		uintptr(len(src)),
		uintptr(unsafe.Pointer(&wideBuf[0])),
		wideLen,
	)

	// Step 2: UTF-16 → ACP (MultiByte)
	acpLen, _, _ := procWideCharToMultiByte.Call(
		uintptr(acp), 0,
		uintptr(unsafe.Pointer(&wideBuf[0])),
		wideLen,
		0, 0, 0, 0,
	)
	if acpLen == 0 {
		return utf8Str // fallback
	}
	acpBuf := make([]byte, acpLen)
	procWideCharToMultiByte.Call(
		uintptr(acp), 0,
		uintptr(unsafe.Pointer(&wideBuf[0])),
		wideLen,
		uintptr(unsafe.Pointer(&acpBuf[0])),
		acpLen,
		0, 0,
	)

	// Trim trailing null
	if acpBuf[len(acpBuf)-1] == 0 {
		return string(acpBuf[:len(acpBuf)-1])
	}
	return string(acpBuf)
}

// acpDecodeString converts a string from the system's active code page to
// UTF-8 encoding, used for decoding cmd.exe output on Windows.
func acpDecodeString(acpStr string) string {
	if acpStr == "" {
		return ""
	}

	// Append null terminator for Win32 API calls
	src := acpStr + "\x00"
	acp := getACP()

	// Step 1: ACP → UTF-16 (WideChar)
	wideLen, _, _ := procMultiByteToWideChar.Call(
		uintptr(acp), 0,
		uintptr(unsafe.Pointer(unsafe.StringData(src))),
		uintptr(len(src)),
		0, 0,
	)
	if wideLen == 0 {
		return acpStr // fallback — not ACP encoded, return as-is
	}
	wideBuf := make([]uint16, wideLen)
	procMultiByteToWideChar.Call(
		uintptr(acp), 0,
		uintptr(unsafe.Pointer(unsafe.StringData(src))),
		uintptr(len(src)),
		uintptr(unsafe.Pointer(&wideBuf[0])),
		wideLen,
	)

	// Step 2: UTF-16 → UTF-8 (MultiByte)
	utf8Len, _, _ := procWideCharToMultiByte.Call(
		CP_UTF8, 0,
		uintptr(unsafe.Pointer(&wideBuf[0])),
		wideLen,
		0, 0, 0, 0,
	)
	if utf8Len == 0 {
		return acpStr // fallback
	}
	utf8Buf := make([]byte, utf8Len)
	procWideCharToMultiByte.Call(
		CP_UTF8, 0,
		uintptr(unsafe.Pointer(&wideBuf[0])),
		wideLen,
		uintptr(unsafe.Pointer(&utf8Buf[0])),
		utf8Len,
		0, 0,
	)

	// Trim trailing null
	if utf8Buf[len(utf8Buf)-1] == 0 {
		return string(utf8Buf[:len(utf8Buf)-1])
	}
	return string(utf8Buf)
}

// isSignaledExit returns true if the error indicates the process was killed by
// a signal (e.g., timeout). On Windows, there is no Unix-style signal, so we
// return true only for ExitError — the actual timeout detection is done by
// the timedOut atomic flag in executeSystemCommand (FIX-284).
func isSignaledExit(err error) bool {
	if _, ok := err.(*exec.ExitError); ok {
		return true
	}
	return false
}

// setProcessGroupAttr configures the command to run in its own process group.
// On Windows, process group management via Setpgid is not available.
// This is a no-op; the process runs in its own default group.
func setProcessGroupAttr(cmd *exec.Cmd) {
	// No-op: Windows does not support Setpgid.
	// cmd.SysProcAttr can be set with CreationFlags for job objects if needed.
}

// killProcessGroup kills the command's process tree.
// On Windows, process.Kill() terminates the process and its children.
func killProcessGroup(cmd *exec.Cmd) {
	cmd.Process.Kill()
}
