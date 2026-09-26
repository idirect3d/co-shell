// Author: L.Shuang
// Created: 2026-09-26
// Last Modified: 2026-09-26
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

package browser

import (
	"context"
	"fmt"
	"sync"

	"github.com/idirect3d/co-shell/log"
)

// eventBufferSize is the per-subscription event buffer size. A subscriber
// that does not keep up loses events (logged at debug level) instead of
// blocking readLoop — a blocked readLoop would deadlock every CDP call.
const eventBufferSize = 32

// Event is a single CDP event message: a message carrying "method" and
// "params" but no "id".
type Event struct {
	Method string
	Params map[string]interface{}
}

// eventWatcher is one active subscription for a single CDP event method.
type eventWatcher struct {
	method string
	ch     chan Event
}

// EventWatchers holds all subscriptions of a CDPClient, grouped by event
// method. All methods are nil-safe so that a zero-value client (used by
// tests that build CDPClient literals) never panics.
type EventWatchers struct {
	mu       sync.Mutex
	watchers map[string][]*eventWatcher
	closed   bool
}

// newEventWatchers creates an empty subscription registry.
func newEventWatchers() *EventWatchers {
	return &EventWatchers{watchers: make(map[string][]*eventWatcher)}
}

// Watch subscribes to a CDP event method (e.g. "Page.loadEventFired" or
// "Page.javascriptDialogOpening") and returns a buffered channel plus a
// cancel func that unsubscribes and closes the channel.
//
// CDP never replays past events, so callers MUST call Watch BEFORE
// triggering the action that produces the event (subscribe → act → read).
func (c *CDPClient) Watch(method string) (<-chan Event, func()) {
	if c.events == nil {
		closed := make(chan Event)
		close(closed)
		return closed, func() {}
	}

	w := &eventWatcher{method: method, ch: make(chan Event, eventBufferSize)}
	c.events.add(w)

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			c.events.remove(w)
		})
	}
	return w.ch, cancel
}

// WaitEvent subscribes to method and waits for the first matching event, or
// returns an error when ctx is done or the connection closes.
//
// Note: events that already arrived before WaitEvent was called are not
// replayed. Use Watch when the triggering action may complete first.
func (c *CDPClient) WaitEvent(ctx context.Context, method string) (Event, error) {
	ch, cancel := c.Watch(method)
	defer cancel()

	select {
	case ev, ok := <-ch:
		if !ok {
			return Event{}, fmt.Errorf("CDP connection closed while waiting for event %s", method)
		}
		return ev, nil
	case <-ctx.Done():
		return Event{}, fmt.Errorf("timed out waiting for CDP event %s: %w", method, ctx.Err())
	}
}

// add registers a watcher. Returns false when the registry is already closed
// (the caller then receives an immediately closed channel from remove).
func (e *EventWatchers) add(w *eventWatcher) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		close(w.ch)
		return
	}
	e.watchers[w.method] = append(e.watchers[w.method], w)
}

// remove unsubscribes a watcher and closes its channel exactly once.
func (e *EventWatchers) remove(w *eventWatcher) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	list := e.watchers[w.method]
	for i, candidate := range list {
		if candidate == w {
			e.watchers[w.method] = append(list[:i:i], list[i+1:]...)
			close(w.ch)
			return
		}
	}
}

// dispatch delivers an event to every subscriber of its method. Delivery is
// non-blocking: when a subscriber's buffer is full the event is dropped and a
// debug message is logged, so a slow consumer can never stall readLoop.
func (e *EventWatchers) dispatch(method string, params map[string]interface{}) {
	if e == nil {
		return
	}
	e.mu.Lock()
	found := make([]*eventWatcher, len(e.watchers[method]))
	copy(found, e.watchers[method])
	e.mu.Unlock()

	ev := Event{Method: method, Params: params}
	for _, w := range found {
		select {
		case w.ch <- ev:
		default:
			log.Debug("CDP event %s dropped: subscriber buffer full", method)
		}
	}
}

// closeAll closes every subscription (called when the connection dies) and
// marks the registry closed so late subscribers get a closed channel.
func (e *EventWatchers) closeAll() {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return
	}
	e.closed = true
	for method, list := range e.watchers {
		for _, w := range list {
			close(w.ch)
		}
		delete(e.watchers, method)
	}
}
