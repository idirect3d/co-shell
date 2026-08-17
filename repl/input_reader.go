// InputReader owns the single stdin reading goroutine (tui mode) and fans
// parsed InputEvents out to subscribed consumers.
//
// Author: L.Shuang
// Created: 2026-08-16
// Last Modified: 2026-08-16
// MIT License - Copyright (c) 2026 L.Shuang

package repl

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/log"
)

// InputReader wraps an agent.InputSource (RawKeySource in tui mode) behind a
// single reader goroutine that broadcasts events to subscribers. It also owns
// raw terminal mode: raw is entered on Start, restored on Pause (so an
// interactive sub-process like sudo owns stdin), re-entered on Resume, and
// restored on Close.
type InputReader struct {
	src agent.InputSource

	readerCtx context.Context
	stop      context.CancelFunc

	mu        sync.Mutex
	subs      map[int64]func(agent.InputEvent)
	nextID    int64
	paused    bool
	pauseAck  chan struct{}
	resumeCh  chan struct{}
	runCtx    context.Context
	runCancel context.CancelFunc

	wg sync.WaitGroup

	rawFd     int
	oldRaw    interface{}
	rawActive bool
}

// NewInputReader creates an InputReader over the given source.
func NewInputReader(src agent.InputSource) *InputReader {
	ctx, cancel := context.WithCancel(context.Background())
	return &InputReader{
		src:       src,
		readerCtx: ctx,
		stop:      cancel,
		subs:      make(map[int64]func(agent.InputEvent)),
		rawFd:     int(os.Stdin.Fd()),
	}
}

// Start enters raw mode and launches the reader goroutine.
func (r *InputReader) Start() error {
	old, err := MakeRaw(r.rawFd)
	if err != nil {
		return fmt.Errorf("raw terminal mode: %w", err)
	}
	r.mu.Lock()
	r.oldRaw = old
	r.rawActive = true
	r.runCtx, r.runCancel = context.WithCancel(r.readerCtx)
	r.mu.Unlock()
	r.wg.Add(1)
	go r.loop()
	return nil
}

// RawActive reports whether the terminal is currently in raw mode (false while
// paused so callers can skip \r\n conversion).
func (r *InputReader) RawActive() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rawActive && !r.paused
}

// IsPaused reports whether the reader is currently paused (not reading stdin
// and terminal restored to cooked mode). Callers use this to fall back to
// plain cooked-mode input while builtin command wizards own stdin.
func (r *InputReader) IsPaused() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.paused
}

// Done returns a channel closed when the reader is closed.
func (r *InputReader) Done() <-chan struct{} {
	return r.readerCtx.Done()
}

// Subscribe registers a consumer callback. The returned function unsubscribes.
func (r *InputReader) Subscribe(fn func(agent.InputEvent)) func() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	id := r.nextID
	r.subs[id] = fn
	return func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		delete(r.subs, id)
	}
}

// Pause stops the reader goroutine (honouring any in-flight read) and restores
// the terminal to cooked mode so an interactive sub-process owns stdin. It
// blocks until the goroutine has actually stopped reading.
func (r *InputReader) Pause() {
	r.mu.Lock()
	if r.paused {
		r.mu.Unlock()
		return
	}
	r.paused = true
	r.pauseAck = make(chan struct{}, 1)
	r.resumeCh = make(chan struct{})
	if r.runCancel != nil {
		r.runCancel()
	}
	ack := r.pauseAck
	r.mu.Unlock()

	select {
	case <-ack:
	case <-r.readerCtx.Done():
		return
	}

	r.mu.Lock()
	if r.rawActive && r.oldRaw != nil {
		_ = RestoreTerm(r.rawFd, r.oldRaw)
		r.rawActive = false
	}
	r.mu.Unlock()
}

// Resume re-enters raw mode and lets the reader goroutine continue.
func (r *InputReader) Resume() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.paused {
		return
	}
	if !r.rawActive {
		old, err := MakeRaw(r.rawFd)
		if err == nil {
			r.oldRaw = old
			r.rawActive = true
		}
	}
	close(r.resumeCh)
	r.paused = false
}

// Close stops the reader goroutine and restores the terminal.
func (r *InputReader) Close() error {
	r.stop()
	r.wg.Wait()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.rawActive && r.oldRaw != nil {
		_ = RestoreTerm(r.rawFd, r.oldRaw)
		r.rawActive = false
	}
	return nil
}

// loop is the single stdin consumer goroutine.
func (r *InputReader) loop() {
	defer r.wg.Done()
	for {
		r.mu.Lock()
		if r.paused {
			ack := r.pauseAck
			resumeCh := r.resumeCh
			r.mu.Unlock()
			select {
			case ack <- struct{}{}:
			default:
			}
			select {
			case <-resumeCh:
				r.mu.Lock()
				r.runCtx, r.runCancel = context.WithCancel(r.readerCtx)
				r.paused = false
				r.mu.Unlock()
				continue
			case <-r.readerCtx.Done():
				return
			}
		}
		runCtx := r.runCtx
		r.mu.Unlock()

		ev, err := r.src.NextEvent(runCtx)
		if err != nil {
			if err == context.Canceled {
				// Distinguish Close (readerCtx cancelled → exit) from Pause
				// (readerCtx alive → re-check flags at loop top).
				select {
				case <-r.readerCtx.Done():
					return
				default:
				}
				continue
			}
			if err == io.EOF {
				r.broadcast(agent.InputEvent{Kind: agent.InputEOF})
				return
			}
			log.Warn("InputReader: %v", err)
			return
		}
		r.broadcast(ev)
	}
}

// broadcast delivers an event to all subscribed consumers.
func (r *InputReader) broadcast(ev agent.InputEvent) {
	r.mu.Lock()
	subs := make([]func(agent.InputEvent), 0, len(r.subs))
	for _, fn := range r.subs {
		subs = append(subs, fn)
	}
	r.mu.Unlock()
	for _, fn := range subs {
		fn(ev)
	}
}
