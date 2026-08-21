// Author: L.Shuang
// Created: 2026-04-30
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
	"strings"
	"sync/atomic"
	"time"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
)

// rawOutputWriter wraps an io.Writer to replace \n with \r\n.
// In raw terminal mode, \n (LF) moves the cursor down but does NOT return to
// column 0, so \r\n is required for proper newline behavior.
type rawOutputWriter struct {
	w io.Writer
}

func (r *rawOutputWriter) Write(p []byte) (n int, err error) {
	// Replace each \n with \r\n
	converted := bytes.ReplaceAll(p, []byte("\n"), []byte("\r\n"))
	_, err = r.w.Write(converted)
	// Return len(p), not len(converted), because io.MultiWriter checks
	// that the returned n matches the original input length. If \n→\r\n
	// conversion increases the byte count, MultiWriter would report a
	// spurious "short write" error even though all data was written.
	return len(p), err
}

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

// encodeForShell encodes the command string for the current platform's shell.
// On Windows, UTF-8 command is encoded to the system's active code page (ACP)
// using Win32 API so cmd.exe can correctly interpret non-ASCII characters.
// On non-Windows platforms, the command is returned as-is.
func encodeForShell(command string) string {
	if runtime.GOOS != "windows" {
		return command
	}
	return acpEncodeString(command)
}

// decodeToUTF8 converts the shell output bytes from the system's active code page
// to UTF-8 string on Windows. On non-Windows platforms, it returns the raw
// string as-is.
func decodeToUTF8(data []byte) string {
	if runtime.GOOS != "windows" {
		return string(data)
	}
	return acpDecodeString(string(data))
}

// executeSystemCommand runs a system command with timeout.
// timeout_seconds is required: 0 means wait forever; when > 0 the effective
// timeout is the maximum of the user-configured minimum timeout and the
// LLM-suggested value.
// on_timeout is required: "kill" terminates the whole process group on
// timeout (legacy behavior); "detach" stops waiting and returns the PID,
// partial output and a log file path while the process keeps running.
// stdin is connected to os.Stdin so interactive commands (e.g. sudo) work
// (except in detach mode, where the process must not compete for stdin
// after the call returns).
// stdout+stderr are both captured for LLM return AND displayed on the terminal.
func (a *Agent) executeSystemCommand(ctx context.Context, args map[string]interface{}) (string, error) {
	command, ok := args["command"].(string)
	if !ok {
		return "", fmt.Errorf("command argument is required")
	}

	// FEATURE-355: timeout_seconds is required; 0 means wait forever.
	t, ok := args["timeout_seconds"].(float64)
	if !ok {
		return "", fmt.Errorf("timeout_seconds argument is required (0 = wait forever)")
	}
	llmSuggested := int(t)

	// FEATURE-355: on_timeout is required: "kill" or "detach".
	onTimeout, ok := args["on_timeout"].(string)
	if !ok {
		return "", fmt.Errorf("on_timeout argument is required (\"kill\" or \"detach\")")
	}
	if onTimeout != "kill" && onTimeout != "detach" {
		return "", fmt.Errorf("invalid on_timeout %q: must be \"kill\" or \"detach\"", onTimeout)
	}
	detachMode := onTimeout == "detach"

	// Effective timeout = max(user-configured minimum, LLM-suggested);
	// an explicit 0 means wait forever (no timeout at all).
	userMin := a.getCommandTimeout()
	userMinSec := int(userMin.Seconds())
	effectiveTimeout := 0
	if llmSuggested > 0 {
		effectiveTimeout = userMinSec
		if llmSuggested > effectiveTimeout {
			effectiveTimeout = llmSuggested
		}
	}

	// Encode command to system code page on Windows before sending to shell
	encodedCommand := encodeForShell(command)

	shell, shellArg := shellCmd()
	log.Debug("Executing command: %s (effective timeout: %ds, user min: %ds, LLM suggested: %ds, shell: %s)",
		command, effectiveTimeout, userMinSec, llmSuggested, shell)

	// Use exec.Command (NOT exec.CommandContext) — we manage kill ourselves.
	// setProcessGroupAttr puts bash into its own session so piped children (python3 | head)
	// are all in the same session. We kill the session on timeout.
	cmd := exec.Command(shell, shellArg, encodedCommand)
	setProcessGroupAttr(cmd)

	// Connect stdin so interactive commands (sudo, passwd, etc.) can read user
	// input. Detach mode is the exception: after the call returns the process
	// keeps running and must not compete with the REPL for stdin.
	if !detachMode {
		cmd.Stdin = os.Stdin
	}

	// Capture stdout+stderr. Kill mode captures into an in-memory buffer.
	// Detach mode writes to a log file instead: after a detach return the
	// process keeps producing output, and an in-memory buffer would grow
	// unbounded — the file also gives the LLM a path to poll later.
	// When showCommandOutput is true, print [🔴] prefix and tee output to
	// terminal via rawOutputWriter that converts \n to \r\n for raw mode.
	var buf bytes.Buffer
	var logFile *os.File
	if detachMode {
		f, err := os.CreateTemp("", "co-shell-detached-*.log")
		if err != nil {
			return "", fmt.Errorf("cannot create detach log file: %w", err)
		}
		logFile = f
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	} else {
		cmd.Stdout = &buf
		cmd.Stderr = &buf
	}
	if a.showCommandOutput {
		ep := config.GetEmojiPrefixes(a.emojiEnabled)
		fmt.Print(ep.CommandOutput)
		if detachMode {
			cmd.Stdout = io.MultiWriter(logFile, &rawOutputWriter{w: os.Stdout})
			cmd.Stderr = io.MultiWriter(logFile, &rawOutputWriter{w: os.Stdout})
		} else {
			cmd.Stdout = io.MultiWriter(&buf, &rawOutputWriter{w: os.Stdout})
			cmd.Stderr = io.MultiWriter(&buf, &rawOutputWriter{w: os.Stdout})
		}
	}

	// FIX-209: Signal the ESC monitor goroutine to stop polling stdin while the
	// sub-process is running, so the sub-process (e.g. sudo) can read from stdin
	// without competition.
	a.SetCommandRunning(true)

	// Restore the terminal to cooked mode (via the registered CommandHooks) so
	// interactive commands (sudo, passwd, etc.) can read user input with echo,
	// line buffering and Ctrl+C handling. Without this, raw mode disables echo
	// and ICRNL, causing commands that prompt for input to hang with no visible
	// feedback.
	a.onCommandStart()

	// Start the command (non-blocking).
	if err := cmd.Start(); err != nil {
		a.SetCommandRunning(false)
		a.onCommandEnd()
		if logFile != nil {
			logPath := logFile.Name()
			logFile.Close()
			os.Remove(logPath)
		}
		return "", fmt.Errorf("cannot start command: %w", err)
	}

	// timedOut flag indicates whether the timeout goroutine fired.
	// This distinguishes a real timeout from a normal ExitError (e.g. exit code 1)
	// so we don't misreport the latter as a timeout. FIX-284.
	var timedOut atomic.Bool

	// done is closed when cmd.Wait() returns, signalling the timeout goroutine
	// to abort its pending kill. Without this, a command that finishes before
	// the timeout would still killProcessGroup later, accidentally killing
	// background children (e.g. `sleep 300 &`) that remain in the same
	// process group. FIX-320.
	done := make(chan struct{})

	// detachCh is closed by the timeout goroutine in detach mode instead of
	// killing the process group (FEATURE-355).
	detachCh := make(chan struct{})

	// Timeout goroutine: wait for timeout, then either kill the entire process
	// group (kill mode) or signal detachment (detach mode).
	// setProcessGroupAttr ensures bash + all pipe children share the same PGID.
	if effectiveTimeout > 0 {
		pid := cmd.Process.Pid
		go func() {
			select {
			case <-done:
				// Command finished before the timeout — nothing to do.
				return
			case <-time.After(time.Duration(effectiveTimeout) * time.Second):
				timedOut.Store(true)
				if detachMode {
					log.Warn("Timeout detach: leaving PID %d running after %ds timeout: %s",
						pid, effectiveTimeout, command)
					close(detachCh)
				} else {
					log.Warn("Timeout kill: killing process group of PID %d after %ds timeout: %s",
						pid, effectiveTimeout, command)
					killProcessGroup(cmd)
				}
			}
		}()
	}

	// readOutput returns the captured output: the log file content in detach
	// mode, the in-memory buffer otherwise.
	readOutput := func() string {
		if logFile != nil {
			logFile.Sync()
			data, rerr := os.ReadFile(logFile.Name())
			if rerr != nil {
				return ""
			}
			return decodeToUTF8(data)
		}
		return decodeToUTF8(buf.Bytes())
	}

	var err error
	if detachMode && effectiveTimeout > 0 {
		// Detach mode: race cmd.Wait() against the timeout signal.
		waitCh := make(chan error, 1)
		go func() { waitCh <- cmd.Wait() }()
		select {
		case err = <-waitCh:
			close(done)
		case <-detachCh:
			// Timeout in detach mode: stop waiting, leave the process running.
			// A reaper goroutine finishes Wait/close/release in the background.
			go func() {
				<-waitCh
				close(done)
				logFile.Close()
				if cmd.Process != nil {
					cmd.Process.Release()
				}
			}()
			a.SetCommandRunning(false)
			a.onCommandEnd()
			pid := cmd.Process.Pid
			logPath := logFile.Name()
			out := readOutput()
			const tailLimit = 4000
			if len(out) > tailLimit {
				out = "...(截断)\n" + out[len(out)-tailLimit:]
			}
			log.Warn("Command detached after %ds timeout: PID %d, log %s", effectiveTimeout, pid, logPath)
			return fmt.Sprintf("命令超时（%ds）但未终止：进程仍在后台运行。\nPID: %d\n输出持续写入日志文件: %s\n后续可用 execute_command 执行 `tail -f %s` 查看进度，或 `kill %d` 终止。\n--- 已产生输出 ---\n%s",
				effectiveTimeout, pid, logPath, logPath, pid, strings.TrimSpace(out)), nil
		}
	} else {
		err = cmd.Wait()
		close(done)
	}
	a.SetCommandRunning(false)
	// Re-enter raw mode (via the registered CommandHooks) now that the command
	// has finished reading from stdin.
	a.onCommandEnd()
	// Release the process resources now that the command has exited (FIX-320).
	// This frees the PID so it can be reused, and is safe to call after Wait.
	if cmd.Process != nil {
		cmd.Process.Release()
	}
	if logFile != nil {
		logFile.Close()
		defer os.Remove(logFile.Name())
	}

	decoded := readOutput()
	if err != nil {
		// Check for timeout — only report as timeout if the timeout goroutine
		// killed the process. A normal ExitError (e.g. exit code 1) is not a
		// timeout even if isSignaledExit returns true on Windows. FIX-284.
		if timedOut.Load() && isSignaledExit(err) {
			log.Warn("Command timed out after %d seconds: %s", effectiveTimeout, command)
			return "", fmt.Errorf("command timed out after %d seconds", effectiveTimeout)
		}
		log.Error("Command failed: %s, error: %v", command, err)
		return decoded, fmt.Errorf("command failed: %w\nOutput: %s", err, decoded)
	}

	log.Debug("Command completed: %s (output length: %d)", command, len(decoded))
	return strings.TrimSpace(decoded), nil
}

// ExecuteCommandDirectly runs a system command directly without LLM involvement.
// This is used by the REPL when user input is detected as a direct system command.
// stdin is connected to os.Stdin so interactive commands work.
// stdout+stderr are both captured for return AND displayed on the terminal.
// Timeout handling is identical to executeSystemCommand: we manage the kill
// ourselves via a goroutine so the entire process group (bash + pipe children)
// is killed on timeout, and a command that finishes early aborts the pending
// kill instead of killing background children later. FIX-320.
func (a *Agent) ExecuteCommandDirectly(command string) (string, error) {
	// Encode command to system code page on Windows before sending to shell
	encodedCommand := encodeForShell(command)

	timeoutSec := int(a.getCommandTimeout().Seconds())

	shell, shellArg := shellCmd()
	if timeoutSec > 0 {
		log.Info("Direct command: %s (timeout: %ds, shell: %s)", command, timeoutSec, shell)
	} else {
		log.Info("Direct command: %s (no timeout, shell: %s)", command, shell)
	}

	// Use exec.Command (NOT exec.CommandContext) — we manage kill ourselves.
	// setProcessGroupAttr puts bash into its own session so piped children
	// (e.g. `sleep 60 | cat`) share the same PGID and are all killed on
	// timeout. FIX-320.
	cmd := exec.Command(shell, shellArg, encodedCommand)
	setProcessGroupAttr(cmd)

	// Connect stdin so interactive commands can read user input.
	cmd.Stdin = os.Stdin

	// Capture stdout+stderr. Always capture in buf for return value.
	// When showCommandOutput is true, also display output in real-time on the
	// terminal via rawOutputWriter that converts \n to \r\n for raw mode.
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if a.showCommandOutput {
		ep := config.GetEmojiPrefixes(a.emojiEnabled)
		fmt.Print(ep.CommandOutput)
		cmd.Stdout = io.MultiWriter(&buf, &rawOutputWriter{w: os.Stdout})
		cmd.Stderr = io.MultiWriter(&buf, &rawOutputWriter{w: os.Stderr})
	}

	// FIX-209: Signal the ESC monitor goroutine to stop polling stdin while the
	// sub-process is running, so the sub-process (e.g. sudo) can read from stdin
	// without competition.
	a.SetCommandRunning(true)

	// Restore the terminal to cooked mode (via the registered CommandHooks) so
	// interactive commands (sudo, passwd, etc.) can read user input with echo,
	// line buffering and Ctrl+C handling. Without this, raw mode disables echo
	// and ICRNL, causing commands that prompt for input to hang with no visible
	// feedback.
	a.onCommandStart()

	// Start the command (non-blocking).
	if err := cmd.Start(); err != nil {
		a.SetCommandRunning(false)
		a.onCommandEnd()
		return "", fmt.Errorf("cannot start command: %w", err)
	}

	// timedOut flag distinguishes a real timeout from a normal ExitError,
	// mirroring executeSystemCommand (FIX-284).
	var timedOut atomic.Bool

	// done is closed when cmd.Wait() returns, signalling the timeout goroutine
	// to abort its pending kill so early-finishing commands do not kill
	// background children afterwards (FIX-320).
	done := make(chan struct{})

	// Timeout goroutine: wait for timeout, then kill the entire process group.
	if timeoutSec > 0 {
		pid := cmd.Process.Pid
		go func() {
			select {
			case <-done:
				// Command finished before the timeout — nothing to kill.
				return
			case <-time.After(time.Duration(timeoutSec) * time.Second):
				log.Warn("Timeout kill: killing process group of PID %d after %ds timeout: %s",
					pid, timeoutSec, command)
				timedOut.Store(true)
				killProcessGroup(cmd)
			}
		}()
	}

	err := cmd.Wait()
	close(done)
	a.SetCommandRunning(false)
	// Re-enter raw mode (via the registered CommandHooks) now that the command
	// has finished reading from stdin.
	a.onCommandEnd()
	// Release the process resources now that the command has exited (FIX-320).
	if cmd.Process != nil {
		cmd.Process.Release()
	}

	decoded := decodeToUTF8(buf.Bytes())
	if err != nil {
		if timedOut.Load() && isSignaledExit(err) {
			log.Warn("Direct command timed out: %s", command)
			return "", fmt.Errorf("command timed out after %d seconds", timeoutSec)
		}
		log.Error("Direct command failed: %s, error: %v", command, err)
		return decoded, fmt.Errorf("command failed: %w\nOutput: %s", err, decoded)
	}

	log.Debug("Direct command completed: %s (output length: %d)", command, buf.Len())
	return strings.TrimSpace(decoded), nil
}

// promptToolConfirmation displays the tool call to the user and asks for
// confirmation via the unified Interaction model (FEATURE-388). It builds a
// confirm Interaction and delegates to the given InteractionManager, mapping
// the structured result back to the legacy CmdConfirmResult for the caller.
//
// Returns the user's choice and any supplementary input.
// - Enter: approve and execute
// - c/C: cancel, return to REPL
// - a/A: approve all tools for this request
// - g/G: approve and disable confirmation for this tool
// - N (a positive integer): approve the next N calls of this tool
// - Any other input: treated as supplementary instructions for the LLM to re-evaluate
func promptToolConfirmation(toolName string, displayStr string, mgr InteractionManager) (CmdConfirmResult, string) {
	res, err := mgr.Ask(context.Background(), Interaction{
		Kind:  InteractionConfirm,
		Title: i18n.TF(i18n.KeyCmdConfirmTitle, displayStr),
		Body:  i18n.T(i18n.KeyCmdConfirmRiskWarning),
		// Structured keys let the Web UI render a button group; AllowFree lets
		// the user type supplementary instructions (FEATURE-388).
		Keys: []KeyOption{
			{Label: i18n.T(i18n.KeyCmdConfirmBtnApprove), Key: "", Value: string(ActionApprove)},
			{Label: i18n.T(i18n.KeyCmdConfirmBtnApproveAll), Key: "a", Value: string(ActionApproveAll)},
			{Label: i18n.T(i18n.KeyCmdConfirmBtnApproveG), Key: "g", Value: string(ActionApproveG)},
			{Label: i18n.T(i18n.KeyCmdConfirmBtnApproveD), Key: "d", Value: string(ActionApproveD)},
			{Label: i18n.T(i18n.KeyCmdConfirmBtnCancel), Key: "c", Value: string(ActionCancel)},
		},
		Presets:   []string{"3", "10", "50"},
		AllowFree: true,
	})
	if err != nil {
		return CmdConfirmCancel, ""
	}

	switch res.Action {
	case ActionApprove:
		return CmdConfirmApprove, ""
	case ActionCancel:
		return CmdConfirmCancel, ""
	case ActionApproveAll:
		return CmdConfirmApproveAll, ""
	case ActionApproveG:
		return CmdConfirmApproveG, ""
	case ActionApproveD:
		return CmdConfirmApproveD, ""
	case ActionApproveCount:
		return CmdConfirmApproveCount, res.Value
	case ActionModify, ActionInput:
		// Supplementary input (typed in the main input box) holds execution and
		// sends the input to the LLM for re-evaluation — it must NOT approve the
		// tool call (FEATURE-388).
		return CmdConfirmModify, res.Value
	default:
		return CmdConfirmApprove, ""
	}
}

// readLine reads a line of input using the provided UserIO.
func readLine(io UserIO) string {
	line, err := io.ReadLine()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(line)
}
