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

package cmd

import (
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
)

// TestBrowserSettingInventory guarantees that the declared browser-* inventory
// (browserSettingNames) and the .set dispatcher in settings_agent.go agree:
// every declared key is readable through handleBrowserSetting.
func TestBrowserSettingInventory(t *testing.T) {
	h := &SettingsHandler{cfg: config.DefaultConfig()}
	if len(browserSettingNames) != 17 {
		t.Fatalf("expected 17 browser settings, got %d", len(browserSettingNames))
	}
	for name := range browserSettingNames {
		if !isBrowserSettingName(name) {
			t.Errorf("isBrowserSettingName(%q) = false", name)
		}
		out, err := h.handleBrowserSetting(name, []string{name})
		if err != nil {
			t.Errorf("handleBrowserSetting(%q) returned error: %v", name, err)
			continue
		}
		if strings.Contains(out, "unknown browser setting") {
			t.Errorf("%q is declared but not wired into handleBrowserSetting", name)
		}
		if !strings.HasPrefix(out, name) {
			t.Errorf("read-back line %q does not start with the setting name", out)
		}
	}
	if _, err := h.handleBrowserSetting("browser-nope", nil); err == nil {
		t.Error("an unknown browser setting must return an error")
	}
}

// TestBrowserSettingHelpers covers the pure helpers shared by the status table
// and the Web settings panel.
func TestBrowserSettingHelpers(t *testing.T) {
	if got := browserDisplayValue(""); got != "-" {
		t.Errorf("browserDisplayValue(empty) = %q, want -", got)
	}
	if got := browserDisplayValue(" zh-CN "); got != " zh-CN " {
		t.Errorf("browserDisplayValue must keep the raw value, got %q", got)
	}
	if got := browserListDisplay(nil); got != "-" {
		t.Errorf("browserListDisplay(nil) = %q, want -", got)
	}
	if got := browserListDisplay([]string{"a", "b"}); got != "a, b" {
		t.Errorf("browserListDisplay = %q, want %q", got, "a, b")
	}
	if got := browserListValue([]string{"a", "b"}); got != "a b" {
		t.Errorf("browserListValue = %q, want %q", got, "a b")
	}
	if got := browserListValue(nil); got != "" {
		t.Errorf("browserListValue(nil) = %q, want empty", got)
	}
	if browserOnOff(true) == browserOnOff(false) {
		t.Error("browserOnOff must differ for true and false")
	}

	if !validSizePair("1280,800", ",") {
		t.Error("1280,800 must be a valid size pair")
	}
	if validSizePair("1280", ",") || validSizePair("0,800", ",") || validSizePair("axb", ",") {
		t.Error("invalid size pairs must be rejected")
	}
	if !validSizePair("1440x900", "x") || validSizePair("1440x", "x") {
		t.Error("viewport validation is wrong")
	}
}

// TestBrowserSettingValidation checks that malformed input is rejected.
func TestBrowserSettingValidation(t *testing.T) {
	h := &SettingsHandler{cfg: config.DefaultConfig()}

	if _, err := h.setBrowserWindowSize("browser-window-size", []string{"browser-window-size", "1280"}); err == nil {
		t.Error("browser-window-size must reject a single number")
	}
	if _, err := h.setBrowserViewport("browser-viewport", []string{"browser-viewport", "1440"}); err == nil {
		t.Error("browser-viewport must reject a value without HEIGHT")
	}
	if _, err := h.setBrowserDialogPolicy("browser-dialog-policy", []string{"browser-dialog-policy", "reject"}); err == nil {
		t.Error("browser-dialog-policy must reject unknown policies")
	}

	enabled := true
	if _, err := h.setBrowserBool("browser-persist", &enabled, []string{"browser-persist", "maybe"}); err == nil {
		t.Error("browser-persist must reject non-boolean input")
	}
	if !enabled {
		t.Error("a rejected value must not modify the setting")
	}
}
