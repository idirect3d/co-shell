// InputReader unified event stream tests: fan-out, pause/resume, ReadKey (P2.5).
//
// Author: L.Shuang
// Created: 2026-08-16
// Last Modified: 2026-08-16
// MIT License - Copyright (c) 2026 L.Shuang

package repl

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/idirect3d/co-shell/agent"
)

// fakeAgentSource implements agent.InputSource with an in-memory event channel.
type fakeAgentSource struct {
	ch chan agent.InputEvent
}

func newFakeAgentSource() *fakeAgentSource {
	return &fakeAgentSource{ch: make(chan agent.InputEvent, 8)}
}

func (f *fakeAgentSource) NextEvent(ctx context.Context) (agent.InputEvent, error) {
	select {
	case ev := <-f.ch:
		return ev, nil
	case <-ctx.Done():
		return agent.InputEvent{}, ctx.Err()
	}
}

func (f *fakeAgentSource) emit(ev agent.InputEvent) { f.ch <- ev }

// startTestReader starts the reader loop without touching the real terminal.
func startTestReader(t *testing.T, src agent.InputSource) *InputReader {
	t.Helper()
	r := NewInputReader(src)
	r.mu.Lock()
	r.runCtx, r.runCancel = context.WithCancel(r.readerCtx)
	r.mu.Unlock()
	r.wg.Add(1)
	go r.loop()
	t.Cleanup(func() { _ = r.Close() })
	return r
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met within timeout")
}

func TestInputReaderFanOut(t *testing.T) {
	src := newFakeAgentSource()
	r := startTestReader(t, src)

	var mu sync.Mutex
	var events []agent.InputEvent
	cancel := r.Subscribe(func(ev agent.InputEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	})
	defer cancel()

	src.emit(agent.InputEvent{Kind: agent.InputKey, Data: "a"})
	src.emit(agent.InputEvent{Kind: agent.InputEnter})

	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(events) == 2
	})
	if events[0].Kind != agent.InputKey || events[1].Kind != agent.InputEnter {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestInputReaderPauseResume(t *testing.T) {
	src := newFakeAgentSource()
	r := startTestReader(t, src)

	var mu sync.Mutex
	var events []agent.InputEvent
	cancel := r.Subscribe(func(ev agent.InputEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	})
	defer cancel()

	src.emit(agent.InputEvent{Kind: agent.InputKey, Data: "a"})
	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(events) == 1
	})

	// Pause stops event delivery.
	r.Pause()
	src.emit(agent.InputEvent{Kind: agent.InputKey, Data: "b"})
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	if len(events) != 1 {
		t.Fatalf("events delivered while paused: got %d, want 1", len(events))
	}
	mu.Unlock()

	// Resume restores delivery; the buffered event is processed.
	r.Resume()
	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(events) == 2
	})
}

func TestEnhancedIOReadKey(t *testing.T) {
	src := newFakeAgentSource()
	r := startTestReader(t, src)
	eio := &EnhancedIO{reader: r}

	type res struct {
		b   byte
		err error
	}
	ch := make(chan res, 1)
	go func() {
		b, err := eio.ReadKey()
		ch <- res{b, err}
	}()
	// Give the subscription time to register before emitting.
	time.Sleep(20 * time.Millisecond)
	src.emit(agent.InputEvent{Kind: agent.InputKey, Data: "y"})

	select {
	case got := <-ch:
		if got.err != nil {
			t.Fatalf("ReadKey: %v", got.err)
		}
		if got.b != 'y' {
			t.Fatalf("got %q, want 'y'", got.b)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ReadKey did not return")
	}
}

func TestKeyEventToByte(t *testing.T) {
	cases := []struct {
		name  string
		ev    agent.InputEvent
		wantB byte
		want  bool
	}{
		{"ascii key", agent.InputEvent{Kind: agent.InputKey, Data: "y"}, 'y', true},
		{"esc", agent.InputEvent{Kind: agent.InputEsc}, 0x1b, true},
		{"ctrl-c", agent.InputEvent{Kind: agent.InputCtrlC}, 0x03, true},
		{"enter", agent.InputEvent{Kind: agent.InputEnter}, '\r', true},
		{"tab", agent.InputEvent{Kind: agent.InputTab}, '\t', true},
		{"backspace", agent.InputEvent{Kind: agent.InputBackspace}, 0x7f, true},
		{"arrow consumed", agent.InputEvent{Kind: agent.InputArrowUp}, 0, false},
		{"home consumed", agent.InputEvent{Kind: agent.InputHome}, 0, false},
		{"empty key", agent.InputEvent{Kind: agent.InputKey, Data: ""}, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, ok := keyEventToByte(tc.ev)
			if b != tc.wantB || ok != tc.want {
				t.Fatalf("got (%d, %v), want (%d, %v)", b, ok, tc.wantB, tc.want)
			}
		})
	}
}