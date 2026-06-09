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

// readLoop continuously reads messages from the WebSocket connection
// and dispatches responses to the appropriate pending channels.
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
			return
		}

		var resp CDPResponse
		if err := json.Unmarshal(message, &resp); err != nil {
			continue
		}

		// Dispatch to the pending channel
		c.pendingMu.Lock()
		ch, ok := c.pending[resp.ID]
		if ok {
			delete(c.pending, resp.ID)
		}
		c.pendingMu.Unlock()

		if ok {
			ch <- resp
			close(ch)
		}
	}
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

// Navigate navigates the page to the given URL.
// After navigation, it waits for the page to load (up to 10 seconds)
// so that subsequent calls like CaptureScreenshot or GetCurrentURL
// can operate on a fully loaded page.
func (c *CDPClient) Navigate(ctx context.Context, url string) (string, error) {
	result, err := c.Call(ctx, "Page.navigate", map[string]interface{}{
		"url": url,
	})
	if err != nil {
		return "", err
	}
	frameID, _ := result["frameId"].(string)

	// Wait for the page to finish loading
	_ = c.WaitForPageLoad(ctx)

	return frameID, nil
}

// WaitForPageLoad waits for the page to finish loading by polling
// document.readyState until it reaches "complete". This ensures
// subsequent calls like CaptureScreenshot or GetCurrentURL don't
// operate on a partially loaded page.
// Returns the total wait time, or an error if the page does not
// load within the timeout.
func (c *CDPClient) WaitForPageLoad(ctx context.Context) error {
	timeoutCtx, cancel := ensureTimeoutContext(ctx)
	defer cancel()

	pollInterval := 200 * time.Millisecond
	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("page load wait timed out: %w", timeoutCtx.Err())
		default:
		}

		readyState, err := c.Evaluate(timeoutCtx, "document.readyState")
		if err != nil {
			// If evaluate fails (e.g. page is still navigating), just retry
			time.Sleep(pollInterval)
			continue
		}

		state, ok := readyState.(string)
		if ok && state == "complete" {
			return nil
		}

		time.Sleep(pollInterval)
	}
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
		// Get page metrics for full-page screenshot
		metrics, err := c.Call(ctx, "Page.getLayoutMetrics", nil)
		if err == nil {
			if contentSize, ok := metrics["contentSize"].(map[string]interface{}); ok {
				width := contentSize["width"].(float64)
				height := contentSize["height"].(float64)
				params["clip"] = map[string]interface{}{
					"x":      0,
					"y":      0,
					"width":  width,
					"height": height,
					"scale":  1,
				}
			}
		}
	}

	result, err := c.Call(ctx, "Page.captureScreenshot", params)
	if err != nil {
		return "", err
	}
	data, _ := result["data"].(string)
	return data, nil
}

// Click performs a mouse click at the given coordinates.
func (c *CDPClient) Click(ctx context.Context, x, y float64) error {
	// Mouse move to position
	_, err := c.Call(ctx, "Input.dispatchMouseEvent", map[string]interface{}{
		"type":       "mousePressed",
		"x":          x,
		"y":          y,
		"button":     "left",
		"clickCount": 1,
	})
	if err != nil {
		return err
	}

	// Mouse release
	_, err = c.Call(ctx, "Input.dispatchMouseEvent", map[string]interface{}{
		"type":       "mouseReleased",
		"x":          x,
		"y":          y,
		"button":     "left",
		"clickCount": 1,
	})
	return err
}

// Type sends text input to the page. It clears the existing content first
// if clear is true, then types the text character by character.
func (c *CDPClient) Type(ctx context.Context, text string) error {
	for _, ch := range text {
		_, err := c.Call(ctx, "Input.dispatchKeyEvent", map[string]interface{}{
			"type":                  "keyDown",
			"windowsVirtualKeyCode": int(ch),
			"key":                   string(ch),
			"text":                  string(ch),
		})
		if err != nil {
			return err
		}
		_, err = c.Call(ctx, "Input.dispatchKeyEvent", map[string]interface{}{
			"type":                  "keyUp",
			"windowsVirtualKeyCode": int(ch),
			"key":                   string(ch),
		})
		if err != nil {
			return err
		}
	}
	return nil
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

// ScrollBy scrolls the page by the given delta.
func (c *CDPClient) ScrollBy(ctx context.Context, deltaX, deltaY float64) error {
	_, err := c.Evaluate(ctx, fmt.Sprintf("window.scrollBy(%f, %f)", deltaX, deltaY))
	return err
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
	result, err := c.Call(ctx, "Page.navigateToHistoryEntry", map[string]interface{}{
		"entryId": -1, // -1 means go back
	})
	if err != nil {
		// Fallback: use JavaScript history.back()
		_, err = c.Evaluate(ctx, "window.history.back()")
		if err != nil {
			return "", fmt.Errorf("cannot go back: %w", err)
		}
		return "", nil
	}
	frameID, _ := result["frameId"].(string)
	return frameID, nil
}

// GoForward navigates forward in the browser history.
func (c *CDPClient) GoForward(ctx context.Context) (string, error) {
	_, err := c.Evaluate(ctx, "window.history.forward()")
	if err != nil {
		return "", fmt.Errorf("cannot go forward: %w", err)
	}
	return "", nil
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
