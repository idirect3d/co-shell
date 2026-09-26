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
	"strconv"
	"time"

	"github.com/idirect3d/co-shell/log"
)

const (
	// pageLoadTimeout bounds the wait for a page load event.
	pageLoadTimeout = 10 * time.Second
	// readyStateTimeout bounds the fallback document.readyState polling.
	readyStateTimeout = 3 * time.Second
	// waitPollInterval is the polling interval used by readyState/selector waits.
	waitPollInterval = 200 * time.Millisecond
)

// EnablePageEvents turns on Page domain events (Page.loadEventFired,
// Page.lifecycleEvent, Page.javascriptDialogOpening). It must be called once
// per CDP connection before waiting on page events.
func (c *CDPClient) EnablePageEvents(ctx context.Context) error {
	if _, err := c.Call(ctx, "Page.enable", nil); err != nil {
		return fmt.Errorf("cannot enable Page events: %w", err)
	}
	return nil
}

// waitForLoadEvent waits for the page load event on the subscription channel
// created BEFORE navigation, and falls back to polling document.readyState
// when the event does not arrive in time (e.g. same-document navigation).
func (c *CDPClient) waitForLoadEvent(ctx context.Context, loadCh <-chan Event, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case _, ok := <-loadCh:
		if !ok {
			return fmt.Errorf("CDP connection closed while waiting for page load")
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		log.Debug("no page load event within %s, falling back to readyState", timeout)
		return c.waitForReadyState(ctx, readyStateTimeout)
	}
}

// waitForHistoryNavigation waits until a history traversal took effect. A
// back/forward restore served from the back-forward cache does not fire
// Page.loadEventFired, so the wait also accepts Page.frameNavigated and falls
// back to polling the document URL. That keeps browser_go_back bounded instead
// of blocking until the full load timeout.
func (c *CDPClient) waitForHistoryNavigation(ctx context.Context, loadCh, navCh <-chan Event, expectedURL string) error {
	ticker := time.NewTicker(waitPollInterval)
	defer ticker.Stop()

	deadline := time.NewTimer(pageLoadTimeout)
	defer deadline.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case _, ok := <-loadCh:
			if !ok {
				return fmt.Errorf("CDP connection closed while waiting for history navigation")
			}
			return nil
		case _, ok := <-navCh:
			if !ok {
				return fmt.Errorf("CDP connection closed while waiting for history navigation")
			}
			return nil
		case <-ticker.C:
			current, err := c.GetCurrentURL(ctx)
			if err == nil && (expectedURL == "" || current == expectedURL) {
				return nil
			}
		case <-deadline.C:
			return fmt.Errorf("history navigation to %q not confirmed within %s", expectedURL, pageLoadTimeout)
		}
	}
}

// waitForReadyState polls document.readyState until it is "complete" or the
// timeout elapses. Unlike the previous implementation it never retries
// forever — it gives up and reports the timeout.
func (c *CDPClient) waitForReadyState(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		state, err := c.Evaluate(ctx, "document.readyState")
		if err == nil {
			if s, ok := state.(string); ok && s == "complete" {
				return nil
			}
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("page load wait timed out after %s", timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitPollInterval):
		}
	}
}

// WaitForPageLoad waits until the current document reports readyState
// "complete", bounded by pageLoadTimeout.
func (c *CDPClient) WaitForPageLoad(ctx context.Context) error {
	return c.waitForReadyState(ctx, pageLoadTimeout)
}

// WaitForSelector waits until selector matches an element in the page, or the
// timeout elapses. It polls with Runtime.evaluate so it keeps working across
// navigations and never holds a stale DOM node id.
func (c *CDPClient) WaitForSelector(ctx context.Context, selector string, timeout time.Duration) error {
	if selector == "" {
		return fmt.Errorf("selector must not be empty")
	}
	if timeout <= 0 {
		timeout = defaultCDPTimeout
	}

	expression := fmt.Sprintf("!!document.querySelector(%s)", strconv.Quote(selector))
	deadline := time.Now().Add(timeout)
	for {
		found, err := c.Evaluate(ctx, expression)
		if err == nil {
			if ok, isBool := found.(bool); isBool && ok {
				return nil
			}
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("timed out after %s waiting for selector %s", timeout, selector)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitPollInterval):
		}
	}
}
