// Author: L.Shuang
// Created: 2026-09-28
// Last Modified: 2026-09-28
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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/log"
)

// Window states accepted by ChromeConfig.WindowMode (FEATURE-569).
const (
	WindowModeMinimized = "minimized"
	WindowModeMaximized = "maximized"
	WindowModeNormal    = "normal"
)

// NormalizeWindowMode validates a configured window mode and returns the
// matching CDP windowState value. An empty or unknown value reports false, in
// which case no window state is applied at all.
func NormalizeWindowMode(mode string) (string, bool) {
	switch strings.TrimSpace(mode) {
	case WindowModeMinimized:
		return WindowModeMinimized, true
	case WindowModeMaximized:
		return WindowModeMaximized, true
	case WindowModeNormal:
		return WindowModeNormal, true
	default:
		return "", false
	}
}

// windowModeToApply reports which window state (if any) has to be applied for
// the given configuration: a headless browser keeps its default state and an
// unset/unknown mode means "do not touch the window".
func windowModeToApply(mode string, headless bool) (string, bool) {
	if headless {
		return "", false
	}
	return NormalizeWindowMode(mode)
}

// SetWindowState applies a window state to the browser window hosting the
// connected page (FEATURE-569). The window is located with
// Browser.getWindowForTarget and then switched with Browser.setWindowBounds.
func SetWindowState(ctx context.Context, c CDPCaller, state string) error {
	resp, err := c.Call(ctx, "Browser.getWindowForTarget", nil)
	if err != nil {
		return fmt.Errorf("Browser.getWindowForTarget: %w", err)
	}
	windowID, ok := windowIDFrom(resp)
	if !ok {
		return fmt.Errorf("Browser.getWindowForTarget: missing windowId")
	}
	if _, err := c.Call(ctx, "Browser.setWindowBounds", map[string]interface{}{
		"windowId": windowID,
		"bounds":   map[string]interface{}{"windowState": state},
	}); err != nil {
		return fmt.Errorf("Browser.setWindowBounds: %w", err)
	}
	return nil
}

// windowIDFrom extracts the windowId of a Browser.getWindowForTarget response.
// JSON numbers arrive as float64; json.Number is accepted as well.
func windowIDFrom(resp map[string]interface{}) (interface{}, bool) {
	value, ok := resp["windowId"]
	if !ok {
		return nil, false
	}
	switch n := value.(type) {
	case float64:
		return n, true
	case int:
		return n, true
	case json.Number:
		if _, err := n.Float64(); err == nil {
			return n, true
		}
	}
	return nil, false
}

// applyInitialWindowMode applies the configured window state to the Chrome
// process Start has just launched (FEATURE-569).
//
// It is a no-op for a headless browser and for an unset/unknown window mode.
// Only newly launched processes go through Start; an already running Chrome
// instance that is merely reused keeps its current window state.
func (m *ChromeManager) applyInitialWindowMode() {
	state, ok := windowModeToApply(m.cfg.WindowMode, m.headless)
	if !ok {
		return
	}

	debugURL := fmt.Sprintf("http://localhost:%d", m.port)
	wsURL, err := getFirstPageWebSocketURL(debugURL)
	if err != nil {
		log.Warn("cannot apply browser window mode %q: %v", state, err)
		return
	}
	client, err := NewCDPClient(wsURL)
	if err != nil {
		log.Warn("cannot apply browser window mode %q: %v", state, err)
		return
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// The window may not exist yet right after launch: retry briefly.
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(300 * time.Millisecond)
		}
		if err := SetWindowState(ctx, client, state); err == nil {
			log.Info("Browser window mode applied: %s", state)
			return
		}
	}
	log.Warn("cannot apply browser window mode %q: %v", state, err)
}
