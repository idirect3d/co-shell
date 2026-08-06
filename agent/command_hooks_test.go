// Author: L.Shuang
// Created: 2026-08-07
// Last Modified: 2026-08-07
//
// # MIT License
//
// # Copyright (c) 2026 L.Shuang
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
	"sync/atomic"
	"testing"
)

// TestCommandHooks_Invoked verifies UC-0001: registered BeforeCommand/AfterCommand
// hooks are invoked exactly once by onCommandStart/onCommandEnd.
func TestCommandHooks_Invoked(t *testing.T) {
	a := &Agent{}
	var beforeCalls, afterCalls atomic.Int32
	a.SetCommandHooks(CommandHooks{
		BeforeCommand: func() { beforeCalls.Add(1) },
		AfterCommand:  func() { afterCalls.Add(1) },
	})

	a.onCommandStart()
	if got := beforeCalls.Load(); got != 1 {
		t.Fatalf("before hook calls = %d, want 1", got)
	}
	if got := afterCalls.Load(); got != 0 {
		t.Fatalf("after hook calls = %d, want 0 before onCommandEnd", got)
	}

	a.onCommandEnd()
	if got := afterCalls.Load(); got != 1 {
		t.Fatalf("after hook calls = %d, want 1", got)
	}
}

// TestCommandHooks_UnregisteredNoPanic verifies UC-0002: calling onCommandStart/
// onCommandEnd on an agent without registered hooks must not panic.
func TestCommandHooks_UnregisteredNoPanic(t *testing.T) {
	a := &Agent{}
	a.onCommandStart()
	a.onCommandEnd()
}

// TestCommandHooks_SafeCallback verifies UC-0003: the hook is invoked OUTSIDE the
// agent mutex, so a hook may safely call back into the agent (e.g. IsCommandRunning)
// without deadlocking.
func TestCommandHooks_SafeCallback(t *testing.T) {
	a := &Agent{}
	a.SetCommandRunning(true)

	var called atomic.Bool
	a.SetCommandHooks(CommandHooks{
		BeforeCommand: func() {
			// This call takes a.mu itself. If onCommandStart held the lock while
			// invoking the hook, this would deadlock and the test would hang.
			if !a.IsCommandRunning() {
				t.Error("IsCommandRunning() = false while command running flag is set")
			}
			called.Store(true)
		},
	})

	a.onCommandStart()
	if !called.Load() {
		t.Fatal("before hook was not invoked")
	}
}
