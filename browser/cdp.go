// Author: L.Shuang
// Created: 2026-06-04
// Last Modified: 2026-06-09
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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/idirect3d/co-shell/log"
)

// CDPRequest represents a JSON-RPC request to the Chrome DevTools Protocol.
type CDPRequest struct {
	ID     int                    `json:"id"`
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// CDPResponse represents a JSON-RPC response from the Chrome DevTools Protocol.
type CDPResponse struct {
	ID     int                    `json:"id"`
	Result map[string]interface{} `json:"result,omitempty"`
	Error  *CDPError              `json:"error,omitempty"`
}

// CDPError represents an error returned by CDP.
type CDPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// CDPClient manages a WebSocket connection to Chrome DevTools Protocol.
type CDPClient struct {
	conn      *websocket.Conn
	mu        sync.Mutex
	nextID    int
	pending   map[int]chan<- CDPResponse
	pendingMu sync.Mutex

	// events dispatches CDP events (messages without an "id") to the
	// subscribers registered via Watch.
	events *EventWatchers
}

// defaultCDPTimeout is the maximum time to wait for a CDP response.
// If the context passed to Call has no deadline, this timeout is used.
const defaultCDPTimeout = 30 * time.Second

// NewCDPClient connects to a Chrome DevTools Protocol WebSocket endpoint.
// The wsURL is typically obtained from http://localhost:<port>/json
// e.g., "ws://localhost:9222/devtools/page/<pageID>"
func NewCDPClient(wsURL string) (*CDPClient, error) {
	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second

	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to CDP WebSocket: %w", err)
	}

	client := &CDPClient{
		conn:    conn,
		nextID:  1,
		pending: make(map[int]chan<- CDPResponse),
		events:  newEventWatchers(),
	}

	// Start a goroutine to read responses from the WebSocket
	go client.readLoop()

	return client, nil
}

// ensureTimeoutContext returns a context with a default timeout if the
// given context has no deadline. This prevents CDP calls from hanging
// forever when the caller does not set a timeout.
func ensureTimeoutContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		return context.WithTimeout(ctx, defaultCDPTimeout)
	}
	// Context already has a deadline — use it as-is (but still return a no-op cancel)
	return ctx, func() {}
}

// readLoop continuously reads messages from the WebSocket connection,
// dispatching command responses to their pending channels and CDP events
// (messages carrying "method" but no "id") to event subscribers.
func (c *CDPClient) readLoop() {
	defer func() {
		if r := recover(); r != nil {
			// Channel closed while sending — connection is shutting down
		}
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			// Connection closed or error
			c.pendingMu.Lock()
			for id, ch := range c.pending {
				close(ch)
				delete(c.pending, id)
			}
			c.pendingMu.Unlock()
			c.events.closeAll()
			return
		}

		var msg cdpMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		// CDP events carry a "method" and no "id"; responses carry an "id".
		// A message carrying a "method" is always an event — some proxies attach
		// a synthetic "id":0, so dispatching by method (not by a missing id)
		// keeps those events from being treated as responses.
		if msg.Method != "" {
			c.events.dispatch(msg.Method, msg.Params)
			continue
		}
		if msg.ID == nil {
			continue
		}

		// Dispatch the response to the pending channel
		id := *msg.ID
		c.pendingMu.Lock()
		ch, ok := c.pending[id]
		if ok {
			delete(c.pending, id)
		}
		c.pendingMu.Unlock()

		if ok {
			ch <- CDPResponse{ID: id, Result: msg.Result, Error: msg.Error}
			close(ch)
		}
	}
}

// cdpMessage is the wire format of any message received from CDP: either a
// command response (carries "id") or an event (carries "method" only).
type cdpMessage struct {
	ID     *int                   `json:"id"`
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
	Result map[string]interface{} `json:"result"`
	Error  *CDPError              `json:"error"`
}

// Call sends a CDP command and waits for the response.
// If the provided context has no deadline, a default timeout of 30 seconds
// is automatically applied to prevent hanging indefinitely.
func (c *CDPClient) Call(ctx context.Context, method string, params map[string]interface{}) (map[string]interface{}, error) {
	// Apply default timeout if context has no deadline
	timeoutCtx, cancel := ensureTimeoutContext(ctx)
	defer cancel()

	c.mu.Lock()
	id := c.nextID
	c.nextID++
	c.mu.Unlock()

	// Create a buffered channel for the response
	ch := make(chan CDPResponse, 1)

	c.pendingMu.Lock()
	c.pending[id] = ch
	c.pendingMu.Unlock()

	// Build and send the request
	req := CDPRequest{
		ID:     id,
		Method: method,
		Params: params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("cannot marshal CDP request: %w", err)
	}

	c.mu.Lock()
	err = c.conn.WriteMessage(websocket.TextMessage, data)
	c.mu.Unlock()
	if err != nil {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("cannot send CDP request: %w", err)
	}

	// Wait for the response with context timeout
	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("CDP error (code=%d): %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	case <-timeoutCtx.Done():
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("CDP call timed out (%s): %w", method, timeoutCtx.Err())
	}
}

// Close closes the WebSocket connection.
func (c *CDPClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		err := c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			c.conn.Close()
			return err
		}
		return c.conn.Close()
	}
	return nil
}

// Navigate navigates the page to the given URL and waits for the load event
// (falling back to document.readyState) so that subsequent calls like
// CaptureScreenshot or GetCurrentURL operate on a fully loaded page.
//
// CDP reports navigation failures (DNS/connection errors, blocked schemes) as
// errorText inside the result instead of a protocol error — those are surfaced
// as a Go error so callers fail fast instead of waiting for a timeout.
func (c *CDPClient) Navigate(ctx context.Context, url string) (string, error) {
	// Subscribe BEFORE navigating: CDP does not replay past events.
	loadCh, cancel := c.Watch("Page.loadEventFired")
	defer cancel()

	if err := c.EnablePageEvents(ctx); err != nil {
		log.Debug("cannot enable Page events before navigate: %v", err)
	}

	result, err := c.Call(ctx, "Page.navigate", map[string]interface{}{
		"url": url,
	})
	if err != nil {
		return "", err
	}

	frameID, _ := result["frameId"].(string)
	if errText, _ := result["errorText"].(string); errText != "" {
		return frameID, fmt.Errorf("navigation to %s failed: %s", url, errText)
	}

	if err := c.waitForLoadEvent(ctx, loadCh, pageLoadTimeout); err != nil {
		log.Debug("page load not confirmed for %s: %v", url, err)
	}

	return frameID, nil
}

// CaptureScreenshot captures a screenshot of the current page.
// Returns base64-encoded JPEG data.
// quality ranges from 0-100, 80 is recommended for LLM analysis.
// fullPage: true captures the full page, false captures the visible viewport only.
func (c *CDPClient) CaptureScreenshot(ctx context.Context, quality int, fullPage bool) (string, error) {
	params := map[string]interface{}{
		"format":  "jpeg",
		"quality": quality,
	}
	if fullPage {
		// Get page metrics for full-page screenshot. captureBeyondViewport is
		// required, otherwise Chrome clamps the capture to the viewport and
		// fixed headers repeat on every scroll position.
		metrics, err := c.Call(ctx, "Page.getLayoutMetrics", nil)
		if err != nil {
			return "", fmt.Errorf("cannot read layout metrics for full-page screenshot: %w", err)
		}
		contentSize, ok := metrics["contentSize"].(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("unexpected layout metrics for full-page screenshot")
		}
		params["clip"] = map[string]interface{}{
			"x":      0,
			"y":      0,
			"width":  jsonFloat(contentSize["width"]),
			"height": jsonFloat(contentSize["height"]),
			"scale":  1,
		}
		params["captureBeyondViewport"] = true
	}

	result, err := c.Call(ctx, "Page.captureScreenshot", params)
	if err != nil {
		return "", err
	}
	data, _ := result["data"].(string)
	return data, nil
}

// Click performs a left-button single click at the given coordinates.
// Use ClickEx for right/middle buttons or double clicks.
func (c *CDPClient) Click(ctx context.Context, x, y float64) error {
	return c.ClickEx(ctx, x, y, MouseButtonLeft, 1)
}

// Type sends text input to the page. Literal text (including CJK and emoji)
// is inserted with Input.insertText; "\n", "\t" and "{Key}" tokens such as
// "{Enter}" or "{Ctrl+A}" are translated into real key presses.
func (c *CDPClient) Type(ctx context.Context, text string) error {
	return c.TypeText(ctx, text)
}

// Evaluate executes JavaScript in the page context and returns the result as a JSON string.
func (c *CDPClient) Evaluate(ctx context.Context, expression string) (interface{}, error) {
	result, err := c.Call(ctx, "Runtime.evaluate", map[string]interface{}{
		"expression":    expression,
		"returnByValue": true,
	})
	if err != nil {
		return nil, err
	}

	if exceptionDetails, ok := result["exceptionDetails"]; ok && exceptionDetails != nil {
		return nil, fmt.Errorf("JavaScript error: %v", exceptionDetails)
	}

	r, ok := result["result"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected result format")
	}

	if r["type"] == "undefined" {
		return nil, nil
	}

	return r["value"], nil
}

// GetDocument returns the root DOM node of the page.
func (c *CDPClient) GetDocument(ctx context.Context) (int, error) {
	result, err := c.Call(ctx, "DOM.getDocument", map[string]interface{}{
		"depth": 0,
	})
	if err != nil {
		return 0, err
	}
	root, ok := result["root"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("cannot get DOM document")
	}
	nodeID := int(root["nodeId"].(float64))
	return nodeID, nil
}

// GetOuterHTML returns the outer HTML of the document node.
func (c *CDPClient) GetOuterHTML(ctx context.Context, nodeID int) (string, error) {
	result, err := c.Call(ctx, "DOM.getOuterHTML", map[string]interface{}{
		"nodeId": nodeID,
	})
	if err != nil {
		return "", err
	}
	html, _ := result["outerHTML"].(string)
	return html, nil
}

// ScrollBy scrolls the page by the given delta using a mouse wheel event,
// falling back to window.scrollBy when the wheel event cannot be delivered.
func (c *CDPClient) ScrollBy(ctx context.Context, deltaX, deltaY float64) error {
	return c.ScrollByWheel(ctx, deltaX, deltaY)
}

// GetViewportSize returns the current viewport dimensions.
func (c *CDPClient) GetViewportSize(ctx context.Context) (width, height float64, err error) {
	result, err := c.Evaluate(ctx, "JSON.stringify({width: window.innerWidth, height: window.innerHeight})")
	if err != nil {
		return 0, 0, err
	}
	str, ok := result.(string)
	if !ok {
		return 0, 0, fmt.Errorf("unexpected viewport result type")
	}
	var viewport struct {
		Width  float64 `json:"width"`
		Height float64 `json:"height"`
	}
	if err := json.Unmarshal([]byte(str), &viewport); err != nil {
		return 0, 0, fmt.Errorf("cannot parse viewport: %w", err)
	}
	return viewport.Width, viewport.Height, nil
}

// GetInteractiveElements returns information about all interactive elements on the page.
// Returns a JSON array of elements with tag, text, id, class, role, aria-label, and bounding rect.
func (c *CDPClient) GetInteractiveElements(ctx context.Context) (string, error) {
	script := `
(() => {
    const elements = document.querySelectorAll(
        'button, a, input, select, textarea, [role="button"], [role="link"], [role="checkbox"], [role="radio"], [role="tab"], [role="menuitem"], [onclick]'
    );
    const interactive = [];
    const seen = new Set();
    elements.forEach(el => {
        const rect = el.getBoundingClientRect();
        // Skip zero-size or hidden elements
        if (rect.width === 0 || rect.height === 0) return;
        // Skip elements outside viewport
        if (rect.bottom < 0 || rect.right < 0) return;
        // Deduplicate by text + position
        const key = el.tagName + ':' + el.innerText?.trim()?.substring(0, 50) + ':' + Math.round(rect.x) + ':' + Math.round(rect.y);
        if (seen.has(key)) return;
        seen.add(key);
        interactive.push({
            tag: el.tagName,
            type: el.type || '',
            text: el.innerText?.trim()?.substring(0, 100) || '',
            title: el.title || '',
            role: el.getAttribute('role') || '',
            ariaLabel: el.getAttribute('aria-label') || '',
            id: el.id || '',
            name: el.name || '',
            className: el.className?.substring(0, 50) || '',
            href: el.href || '',
            placeholder: el.placeholder || '',
            value: el.value || '',
            visible: el.offsetParent !== null,
            rect: {
                x: Math.round(rect.x),
                y: Math.round(rect.y),
                width: Math.round(rect.width),
                height: Math.round(rect.height),
                centerX: Math.round(rect.x + rect.width / 2),
                centerY: Math.round(rect.y + rect.height / 2)
            }
        });
    });
    return JSON.stringify(interactive);
})()
`
	result, err := c.Evaluate(ctx, script)
	if err != nil {
		return "", err
	}
	str, ok := result.(string)
	if !ok {
		return "[]", nil
	}
	return str, nil
}

// GoBack navigates back in the browser history.
func (c *CDPClient) GoBack(ctx context.Context) (string, error) {
	return c.navigateHistory(ctx, -1)
}

// GoForward navigates forward in the browser history.
func (c *CDPClient) GoForward(ctx context.Context) (string, error) {
	return c.navigateHistory(ctx, 1)
}

// navigateHistory moves `step` entries through the navigation history. It reads
// the real entry id from Page.getNavigationHistory (the historical -1 pseudo id
// is rejected by Chrome), navigates to it and waits until the traversal took
// effect - so a failed navigation surfaces as an error instead of a silent
// success. It returns the URL of the document reached.
func (c *CDPClient) navigateHistory(ctx context.Context, step int) (string, error) {
	history, err := c.Call(ctx, "Page.getNavigationHistory", nil)
	if err != nil {
		return "", fmt.Errorf("cannot read navigation history: %w", err)
	}

	entries, _ := history["entries"].([]interface{})
	targetIndex := int(jsonFloat(history["currentIndex"])) + step
	if targetIndex < 0 || targetIndex >= len(entries) {
		if step < 0 {
			return "", fmt.Errorf("no previous entry in navigation history")
		}
		return "", fmt.Errorf("no next entry in navigation history")
	}

	entry, ok := entries[targetIndex].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("unexpected navigation history entry")
	}
	entryID := int(jsonFloat(entry["id"]))
	if entryID == 0 {
		return "", fmt.Errorf("navigation history entry %d has no id", targetIndex)
	}

	// Subscribe before navigating: CDP never replays past events.
	loadCh, cancelLoad := c.Watch("Page.loadEventFired")
	defer cancelLoad()
	navCh, cancelNav := c.Watch("Page.frameNavigated")
	defer cancelNav()

	if _, err := c.Call(ctx, "Page.navigateToHistoryEntry", map[string]interface{}{
		"entryId": entryID,
	}); err != nil {
		return "", fmt.Errorf("cannot navigate to history entry %d: %w", entryID, err)
	}

	targetURL, _ := entry["url"].(string)
	if err := c.waitForHistoryNavigation(ctx, loadCh, navCh, targetURL); err != nil {
		log.Debug("history navigation to entry %d not confirmed: %v", entryID, err)
	}

	currentURL, err := c.GetCurrentURL(ctx)
	if err != nil {
		return "", fmt.Errorf("cannot read URL after navigating to history entry %d: %w", entryID, err)
	}
	return currentURL, nil
}

// GetCurrentURL returns the current page URL.
func (c *CDPClient) GetCurrentURL(ctx context.Context) (string, error) {
	result, err := c.Evaluate(ctx, "window.location.href")
	if err != nil {
		return "", err
	}
	url, ok := result.(string)
	if !ok {
		return "", fmt.Errorf("unexpected URL result type")
	}
	return url, nil
}

// GetPageTitle returns the current page title.
func (c *CDPClient) GetPageTitle(ctx context.Context) (string, error) {
	result, err := c.Evaluate(ctx, "document.title")
	if err != nil {
		return "", err
	}
	title, ok := result.(string)
	if !ok {
		return "", nil
	}
	return title, nil
}

// ScreenshotToBytes decodes a base64 screenshot string to raw bytes.
func ScreenshotToBytes(data string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(data)
}
