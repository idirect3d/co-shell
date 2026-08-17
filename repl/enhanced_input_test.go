// EnhancedInput event-driven line editor tests (P2.5).
//
// Author: L.Shuang
// Created: 2026-08-16
// Last Modified: 2026-08-16
// MIT License - Copyright (c) 2026 L.Shuang

package repl

import (
	"sync"
	"testing"
	"time"

	"github.com/idirect3d/co-shell/agent"
)

// fakeInputSource is a synchronous inputEventSource for line editor tests.
// emit() invokes the subscriber callbacks directly on the calling goroutine.
type fakeInputSource struct {
	mu      sync.Mutex
	subs    map[int64]func(agent.InputEvent)
	nextID  int64
	done    chan struct{}
	doneOne sync.Once
}

func newFakeInputSource() *fakeInputSource {
	return &fakeInputSource{
		subs: make(map[int64]func(agent.InputEvent)),
		done: make(chan struct{}),
	}
}

func (f *fakeInputSource) Subscribe(fn func(agent.InputEvent)) func() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	id := f.nextID
	f.subs[id] = fn
	return func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		delete(f.subs, id)
	}
}

func (f *fakeInputSource) Done() <-chan struct{} { return f.done }

func (f *fakeInputSource) emit(ev agent.InputEvent) {
	f.mu.Lock()
	subs := make([]func(agent.InputEvent), 0, len(f.subs))
	for _, fn := range f.subs {
		subs = append(subs, fn)
	}
	f.mu.Unlock()
	for _, fn := range subs {
		fn(ev)
	}
}

func (f *fakeInputSource) close() {
	f.doneOne.Do(func() { close(f.done) })
}

// waitSubscribers blocks until at least n subscriber callbacks are registered.
func (f *fakeInputSource) waitSubscribers(t *testing.T, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		c := len(f.subs)
		f.mu.Unlock()
		if c >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("subscriber did not register in time")
}

type readLineResult struct {
	line string
	err  error
}

// runReadLine starts ReadLineFrom in a goroutine and waits for its
// subscription so events emitted afterwards are not lost.
func runReadLine(t *testing.T, ei *EnhancedInput, src *fakeInputSource) <-chan readLineResult {
	t.Helper()
	ch := make(chan readLineResult, 1)
	go func() {
		line, err := ei.ReadLineFrom(src)
		ch <- readLineResult{line: line, err: err}
	}()
	src.waitSubscribers(t, 1)
	return ch
}

func TestEnhancedInputReadLineFrom_Basic(t *testing.T) {
	src := newFakeInputSource()
	ei := NewEnhancedInput("", nil)
	ch := runReadLine(t, ei, src)
	src.emit(agent.InputEvent{Kind: agent.InputKey, Data: "h"})
	src.emit(agent.InputEvent{Kind: agent.InputKey, Data: "e"})
	src.emit(agent.InputEvent{Kind: agent.InputKey, Data: "l"})
	src.emit(agent.InputEvent{Kind: agent.InputKey, Data: "l"})
	src.emit(agent.InputEvent{Kind: agent.InputKey, Data: "o"})
	src.emit(agent.InputEvent{Kind: agent.InputEnter})

	got := <-ch
	if got.err != nil {
		t.Fatalf("ReadLineFrom: %v", got.err)
	}
	if got.line != "hello" {
		t.Fatalf("got %q, want %q", got.line, "hello")
	}
}

func TestEnhancedInputReadLineFrom_Backspace(t *testing.T) {
	src := newFakeInputSource()
	ei := NewEnhancedInput("", nil)
	ch := runReadLine(t, ei, src)
	src.emit(agent.InputEvent{Kind: agent.InputKey, Data: "h"})
	src.emit(agent.InputEvent{Kind: agent.InputKey, Data: "e"})
	src.emit(agent.InputEvent{Kind: agent.InputBackspace})
	src.emit(agent.InputEvent{Kind: agent.InputKey, Data: "i"})
	src.emit(agent.InputEvent{Kind: agent.InputEnter})

	got := <-ch
	if got.err != nil {
		t.Fatalf("ReadLineFrom: %v", got.err)
	}
	if got.line != "hi" {
		t.Fatalf("got %q, want %q", got.line, "hi")
	}
}

func TestEnhancedInputReadLineFrom_HistoryUpDown(t *testing.T) {
	src := newFakeInputSource()
	ei := NewEnhancedInput("", []string{"ls", "date"})
	ch := runReadLine(t, ei, src)
	// history[1] = "date" (newest), history[0] = "ls".
	src.emit(agent.InputEvent{Kind: agent.InputArrowUp})
	src.emit(agent.InputEvent{Kind: agent.InputEnter})

	got := <-ch
	if got.err != nil {
		t.Fatalf("ReadLineFrom: %v", got.err)
	}
	if got.line != "date" {
		t.Fatalf("got %q, want %q", got.line, "date")
	}
}

func TestEnhancedInputReadLineFrom_EscClearsLine(t *testing.T) {
	src := newFakeInputSource()
	ei := NewEnhancedInput("", nil)
	ch := runReadLine(t, ei, src)
	src.emit(agent.InputEvent{Kind: agent.InputKey, Data: "a"})
	src.emit(agent.InputEvent{Kind: agent.InputKey, Data: "b"})
	src.emit(agent.InputEvent{Kind: agent.InputEsc})
	src.emit(agent.InputEvent{Kind: agent.InputKey, Data: "x"})
	src.emit(agent.InputEvent{Kind: agent.InputEnter})

	got := <-ch
	if got.err != nil {
		t.Fatalf("ReadLineFrom: %v", got.err)
	}
	if got.line != "x" {
		t.Fatalf("got %q, want %q", got.line, "x")
	}
}

func TestEnhancedInputReadLineFrom_CtrlC(t *testing.T) {
	src := newFakeInputSource()
	ei := NewEnhancedInput("", nil)
	ch := runReadLine(t, ei, src)
	src.emit(agent.InputEvent{Kind: agent.InputKey, Data: "a"})
	src.emit(agent.InputEvent{Kind: agent.InputCtrlC})

	got := <-ch
	if got.err == nil || got.err.Error() != "interrupt" {
		t.Fatalf("got %v, want interrupt error", got.err)
	}
}

func TestEnhancedInputReadLineFrom_CtrlDEmpty(t *testing.T) {
	src := newFakeInputSource()
	ei := NewEnhancedInput("", nil)
	ch := runReadLine(t, ei, src)
	src.emit(agent.InputEvent{Kind: agent.InputKey, Data: "\x04"})

	got := <-ch
	if got.err != nil {
		t.Fatalf("ReadLineFrom: %v", got.err)
	}
	if got.line != "" {
		t.Fatalf("got %q, want empty", got.line)
	}
}

func TestEnhancedInputReadLineFrom_ChineseInput(t *testing.T) {
	src := newFakeInputSource()
	ei := NewEnhancedInput("", nil)
	ch := runReadLine(t, ei, src)
	src.emit(agent.InputEvent{Kind: agent.InputKey, Data: "你好"})
	src.emit(agent.InputEvent{Kind: agent.InputEnter})

	got := <-ch
	if got.err != nil {
		t.Fatalf("ReadLineFrom: %v", got.err)
	}
	if got.line != "你好" {
		t.Fatalf("got %q, want %q", got.line, "你好")
	}
}

func TestEnhancedInputReadLineFrom_EditorClosed(t *testing.T) {
	src := newFakeInputSource()
	ei := NewEnhancedInput("", nil)
	ch := runReadLine(t, ei, src)
	src.close()

	select {
	case got := <-ch:
		if got.err == nil {
			t.Fatal("expected error after source closed, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ReadLineFrom did not return after source closed")
	}
}