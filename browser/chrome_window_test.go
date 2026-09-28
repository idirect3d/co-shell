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
	"errors"
	"testing"
)

// winModeRecorder records the CDP calls of SetWindowState. It answers
// Browser.getWindowForTarget with windowID (empty response when nil) and can be
// told to fail one method.
type winModeRecorder struct {
	methods  []string
	params   []map[string]interface{}
	windowID interface{}
	failOn   string
}

func (r *winModeRecorder) Call(_ context.Context, method string, params map[string]interface{}) (map[string]interface{}, error) {
	r.methods = append(r.methods, method)
	r.params = append(r.params, params)
	if r.failOn == method {
		return nil, errors.New("boom")
	}
	if method == "Browser.getWindowForTarget" {
		if r.windowID == nil {
			return map[string]interface{}{}, nil
		}
		return map[string]interface{}{"windowId": r.windowID}, nil
	}
	return map[string]interface{}{}, nil
}

// TestNormalizeWindowMode covers FEATURE-569 UC-0001: only the three documented
// values are accepted; anything else (including upper case) means "no state".
func TestNormalizeWindowMode(t *testing.T) {
	cases := map[string]struct {
		want string
		ok   bool
	}{
		"minimized": {WindowModeMinimized, true},
		"maximized": {WindowModeMaximized, true},
		"normal":    {WindowModeNormal, true},
		"":          {"", false},
		"foo":       {"", false},
		"MINIMIZED": {"", false},
	}
	for in, want := range cases {
		got, ok := NormalizeWindowMode(in)
		if ok != want.ok || got != want.want {
			t.Errorf("NormalizeWindowMode(%q) = (%q, %v), want (%q, %v)", in, got, ok, want.want, want.ok)
		}
	}
}

// TestWindowModeToApply covers FEATURE-569 UC-0003: a headless browser never
// gets a window state, an unset mode does nothing.
func TestWindowModeToApply(t *testing.T) {
	for _, mode := range []string{WindowModeMinimized, WindowModeMaximized, WindowModeNormal} {
		if _, ok := windowModeToApply(mode, true); ok {
			t.Errorf("headless browser must not apply window mode %q", mode)
		}
		if _, ok := windowModeToApply(mode, false); !ok {
			t.Errorf("non-headless browser must apply window mode %q", mode)
		}
	}
	for _, mode := range []string{"", "foo"} {
		if _, ok := windowModeToApply(mode, false); ok {
			t.Errorf("mode %q must not produce a CDP call", mode)
		}
	}
}

// TestSetWindowStateSequence covers FEATURE-569 UC-0003: every mode maps to
// Browser.setWindowBounds{windowState} after resolving the window id.
func TestSetWindowStateSequence(t *testing.T) {
	for _, state := range []string{WindowModeMinimized, WindowModeMaximized, WindowModeNormal} {
		rec := &winModeRecorder{windowID: float64(7)}
		if err := SetWindowState(context.Background(), rec, state); err != nil {
			t.Fatalf("SetWindowState(%q): %v", state, err)
		}
		want := []string{"Browser.getWindowForTarget", "Browser.setWindowBounds"}
		if len(rec.methods) != len(want) {
			t.Fatalf("SetWindowState(%q) calls = %v, want %v", state, rec.methods, want)
		}
		for i, method := range want {
			if rec.methods[i] != method {
				t.Errorf("call %d = %q, want %q", i, rec.methods[i], method)
			}
		}
		params := rec.params[1]
		if got := params["windowId"]; got != float64(7) {
			t.Errorf("windowId = %v, want 7", got)
		}
		bounds, ok := params["bounds"].(map[string]interface{})
		if !ok || bounds["windowState"] != state {
			t.Errorf("bounds = %v, want windowState=%q", params["bounds"], state)
		}
	}
}

// TestSetWindowStateErrors covers FEATURE-569 UC-0003: CDP failures are
// reported as errors and never panic.
func TestSetWindowStateErrors(t *testing.T) {
	ctx := context.Background()

	rec := &winModeRecorder{windowID: float64(1), failOn: "Browser.getWindowForTarget"}
	if err := SetWindowState(ctx, rec, WindowModeMinimized); err == nil {
		t.Error("getWindowForTarget failure must be reported")
	}

	rec = &winModeRecorder{windowID: float64(1), failOn: "Browser.setWindowBounds"}
	if err := SetWindowState(ctx, rec, WindowModeMinimized); err == nil {
		t.Error("setWindowBounds failure must be reported")
	}

	rec = &winModeRecorder{}
	if err := SetWindowState(ctx, rec, WindowModeMinimized); err == nil {
		t.Error("missing windowId must be reported")
	}
}

// TestDefaultChromeConfigWindowMode covers FEATURE-569: the packaged default is
// "minimized" so an unconfigured browser opens minimized.
func TestDefaultChromeConfigWindowMode(t *testing.T) {
	if got := DefaultChromeConfig().WindowMode; got != WindowModeMinimized {
		t.Errorf("DefaultChromeConfig().WindowMode = %q, want %q", got, WindowModeMinimized)
	}
}
