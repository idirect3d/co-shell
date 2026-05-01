// Author: L.Shuang
// Created: 2026-04-30
// Last Modified: 2026-04-30
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

package agent

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// shellCmd returns the appropriate shell command and argument for the current platform.
func shellCmd() (string, string) {
	if runtime.GOOS == "windows" {
		return "cmd", "/c"
	}
	return "bash", "-c"
}

// shellName returns the human-readable shell name for the current platform.
func shellName() string {
	if runtime.GOOS == "windows" {
		return "cmd/powershell"
	}
	return "bash/zsh"
}

// decodeToUTF8 converts GBK encoded bytes to UTF-8 string on Windows.
// On non-Windows platforms, it returns the raw string as-is.
func decodeToUTF8(data []byte) string {
	if runtime.GOOS != "windows" {
		return string(data)
	}
	// Try GBK decode first; if it fails, return raw string
	reader := transform.NewReader(bytes.NewReader(data), simplifiedchinese.GBK.NewDecoder())
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return string(data)
	}
	return string(decoded)
}

// executeSystemCommand runs a system command with timeout.
func (a *Agent) executeSystemCommand(ctx context.Context, args map[string]interface{}) (string, error) {
	command, ok := args["command"].(string)
	if !ok {
		return "", fmt.Errorf("command argument is required")
	}

	// Determine timeout: use args timeout_seconds first, then configured command timeout
	var timeout int
	if t, ok := args["timeout_seconds"].(float64); ok {
		timeout = int(t)
	} else {
		cmdTimeout := a.getCommandTimeout()
		if cmdTimeout > 0 {
			timeout = int(cmdTimeout.Seconds())
		}
	}

	// Only set timeout if a positive value is specified
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	}

	shell, shellArg := shellCmd()
	log.Debug("Executing command: %s (timeout: %ds, shell: %s)", command, timeout, shell)
	cmd := exec.CommandContext(ctx, shell, shellArg, command)
	output, err := cmd.CombinedOutput()
	// Decode GBK to UTF-8 on Windows
	decoded := decodeToUTF8(output)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Warn("Command timed out after %d seconds: %s", timeout, command)
			return "", fmt.Errorf("command timed out after %d seconds", timeout)
		}
		log.Error("Command failed: %s, error: %v", command, err)
		return decoded, fmt.Errorf("command failed: %w\nOutput: %s", err, decoded)
	}

	log.Debug("Command completed: %s (output length: %d)", command, len(output))
	return strings.TrimSpace(decoded), nil
}

// ExecuteCommandDirectly runs a system command directly without LLM involvement.
// This is used by the REPL when user input is detected as a direct system command.
func (a *Agent) ExecuteCommandDirectly(command string) (string, error) {
	timeout := a.getCommandTimeout()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		shell, shellArg := shellCmd()
		log.Info("Direct command: %s (timeout: %ds, shell: %s)", command, int(timeout.Seconds()), shell)
		cmd := exec.CommandContext(ctx, shell, shellArg, command)
		output, err := cmd.CombinedOutput()
		// Decode GBK to UTF-8 on Windows
		decoded := decodeToUTF8(output)
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				log.Warn("Direct command timed out: %s", command)
				return "", fmt.Errorf("command timed out after %d seconds", int(timeout.Seconds()))
			}
			log.Error("Direct command failed: %s, error: %v", command, err)
			return decoded, fmt.Errorf("command failed: %w\nOutput: %s", err, decoded)
		}

		log.Debug("Direct command completed: %s (output length: %d)", command, len(output))
		return strings.TrimSpace(decoded), nil
	}

	// No timeout - use background context
	shell, shellArg := shellCmd()
	log.Info("Direct command: %s (no timeout, shell: %s)", command, shell)
	cmd := exec.CommandContext(context.Background(), shell, shellArg, command)

	output, err := cmd.CombinedOutput()
	// Decode GBK to UTF-8 on Windows
	decoded := decodeToUTF8(output)
	if err != nil {
		log.Error("Direct command failed: %s, error: %v", command, err)
		return decoded, fmt.Errorf("command failed: %w\nOutput: %s", err, decoded)
	}

	log.Debug("Direct command completed: %s (output length: %d)", command, len(output))
	return strings.TrimSpace(decoded), nil
}

// promptCommandConfirmation displays the command to the user and asks for confirmation.
// Returns the user's choice and any supplementary input.
// - Enter: approve and execute
// - c/C: cancel, return to REPL
// - a/A: approve all commands for this request
// - N (a positive integer): approve the next N commands
// - Any other input: treated as supplementary instructions for the LLM to re-evaluate
func promptCommandConfirmation(command string) (CmdConfirmResult, string) {
	fmt.Println()
	fmt.Println(i18n.TF(i18n.KeyCmdConfirmTitle, command))
	fmt.Println()

	// Read a single line from stdin using os.Stdin.Read() which works
	// even when go-prompt has set the terminal to raw mode.
	// We read byte by byte until we get a newline.
	for {
		fmt.Print(i18n.T(i18n.KeyCmdConfirmPrompt))

		var lineBuf []byte
		buf := make([]byte, 1)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil || n == 0 {
				break
			}
			if buf[0] == '\n' || buf[0] == '\r' {
				break
			}
			lineBuf = append(lineBuf, buf[0])
		}

		response := strings.TrimSpace(string(lineBuf))

		if response == "" {
			return CmdConfirmApprove, ""
		}

		lower := strings.ToLower(response)
		if lower == "c" {
			return CmdConfirmCancel, ""
		}

		if lower == "a" {
			return CmdConfirmApproveAll, ""
		}

		// Check if the user entered a positive integer (approve N commands)
		if n, err := strconv.Atoi(response); err == nil && n > 0 {
			return CmdConfirmApproveCount, strconv.Itoa(n)
		}

		// Any other input is treated as supplementary instructions
		// for the LLM to re-evaluate the command
		return CmdConfirmModify, response

	}
}

// readLine reads a line of input from stdin using os.Stdin.Read() which works
// even when go-prompt has set the terminal to raw mode.
func readLine() string {
	var lineBuf []byte
	buf := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			break
		}
		if buf[0] == '\n' || buf[0] == '\r' {
			break
		}
		lineBuf = append(lineBuf, buf[0])
	}
	return strings.TrimSpace(string(lineBuf))
}
