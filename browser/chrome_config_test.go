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
	"errors"
	"reflect"
	"strings"
	"testing"
)

// cfg556Recorder records the CDP calls issued by ApplySessionSettings.
type cfg556Recorder struct {
	methods []string
	params  []map[string]interface{}
	err     error
}

func (r *cfg556Recorder) Call(ctx context.Context, method string, params map[string]interface{}) (map[string]interface{}, error) {
	if r.err != nil {
		return nil, r.err
	}
	r.methods = append(r.methods, method)
	r.params = append(r.params, params)
	return map[string]interface{}{}, nil
}

func cfg556IndexOf(args []string, want string) int {
	for i, arg := range args {
		if arg == want {
			return i
		}
	}
	return -1
}

// TestBuildChromeArgsDefaultsMatchLegacy covers use-case A-009: with the default
// configuration the generated argument list is item-for-item identical to the
// pre-FEATURE-556 hard-coded list (no behaviour change by default).
func TestBuildChromeArgsDefaultsMatchLegacy(t *testing.T) {
	got := BuildChromeArgs(DefaultChromeConfig(), 9222, true, "/tmp/profile")
	want := []string{
		"--remote-debugging-port=9222",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-background-networking",
		"--disable-sync",
		"--disable-translate",
		"--disable-extensions",
		"--disable-default-apps",
		"--mute-audio",
		"--disable-features=TranslateUI",
		"--window-size=1280,800",
		"--headless=new",
		"--user-data-dir=/tmp/profile",
		"about:blank",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("default launch args changed:\n got=%v\nwant=%v", got, want)
	}
}

// TestBuildChromeArgsConfigured covers use-case A-010: configured values show up
// as the expected Chrome switches and --disable-extensions disappears when
// extensions are enabled.
func TestBuildChromeArgsConfigured(t *testing.T) {
	cfg := DefaultChromeConfig()
	cfg.Proxy = "http://127.0.0.1:1080"
	cfg.UserAgent = "co-shell-test/1.0"
	cfg.Language = "zh-CN"
	cfg.WindowSize = "1440,900"
	cfg.IgnoreCertErrors = true
	cfg.Extensions = true
	cfg.ExtraArgs = []string{"--disable-gpu"}

	got := BuildChromeArgs(cfg, 9222, false, "/tmp/profile")
	joined := strings.Join(got, " ")
	for _, want := range []string{
		"--proxy-server=http://127.0.0.1:1080",
		"--user-agent=co-shell-test/1.0",
		"--lang=zh-CN",
		"--ignore-certificate-errors",
		"--window-size=1440,900",
		"--disable-gpu",
		"--user-data-dir=/tmp/profile",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("args %v missing %q", got, want)
		}
	}
	if strings.Contains(joined, "--disable-extensions") {
		t.Errorf("extensions=on must not disable extensions: %v", got)
	}
	if strings.Contains(joined, "--headless") {
		t.Errorf("headless=false must not add --headless: %v", got)
	}
	// Deterministic order: built-ins, window size, proxy, UA, lang, headless,
	// profile, extra args, then the initial page.
	order := []string{"--window-size=1440,900", "--proxy-server=", "--user-agent=", "--lang=", "--ignore-certificate-errors", "--user-data-dir=", "--disable-gpu", "about:blank"}
	prev := -1
	for _, key := range order {
		idx := -1
		for i, arg := range got {
			if strings.HasPrefix(arg, key) || arg == key {
				idx = i
				break
			}
		}
		if idx < 0 {
			t.Fatalf("args %v missing %q", got, key)
		}
		if idx <= prev {
			t.Fatalf("unstable arg order around %q: %v", key, got)
		}
		prev = idx
	}
}

// TestApplySessionSettingsSequence covers use-case A-011: a fully configured
// browser produces the documented apply sequence, and an unconfigured browser
// produces no CDP call at all.
func TestApplySessionSettingsSequence(t *testing.T) {
	cfg := DefaultChromeConfig()
	cfg.UserAgent = "co-shell-test/1.0"
	cfg.ExtraHTTPHeaders = []string{"X-A: 1", "X-B: 2"}
	cfg.Viewport = "1440x900"
	cfg.Language = "zh-CN"
	cfg.Timezone = "Asia/Shanghai"
	cfg.IgnoreCertErrors = true
	cfg.DownloadDir = "/tmp/dl"

	rec := &cfg556Recorder{}
	applied, err := ApplySessionSettings(context.Background(), rec, cfg)
	if err != nil {
		t.Fatalf("ApplySessionSettings: %v", err)
	}
	want := []string{
		"Network.setUserAgentOverride",
		"Network.setExtraHTTPHeaders",
		"Emulation.setDeviceMetricsOverride",
		"Emulation.setLocaleOverride",
		"Emulation.setTimezoneOverride",
		"Security.setIgnoreCertificateErrors",
		"Browser.setDownloadBehavior",
	}
	if !reflect.DeepEqual(applied, want) {
		t.Fatalf("apply sequence mismatch:\n got=%v\nwant=%v", applied, want)
	}
	if got := rec.params[0]["userAgent"]; got != "co-shell-test/1.0" {
		t.Errorf("user agent param = %v", got)
	}
	if got := rec.params[2]["width"]; got != 1440 {
		t.Errorf("viewport width = %v, want 1440", got)
	}
	if got := rec.params[6]["downloadPath"]; got != "/tmp/dl" {
		t.Errorf("download path = %v", got)
	}

	// An all-default configuration must not touch the protocol.
	rec2 := &cfg556Recorder{}
	applied2, err := ApplySessionSettings(context.Background(), rec2, ChromeConfig{})
	if err != nil {
		t.Fatalf("ApplySessionSettings(empty): %v", err)
	}
	if len(applied2) != 0 || len(rec2.methods) != 0 {
		t.Fatalf("empty configuration issued CDP calls: %v", rec2.methods)
	}

	// Failures are reported with the methods applied so far.
	rec3 := &cfg556Recorder{err: errors.New("boom")}
	applied3, err := ApplySessionSettings(context.Background(), rec3, cfg)
	if err == nil {
		t.Fatal("expected an error from the recording caller")
	}
	if len(applied3) != 0 {
		t.Fatalf("no method should be reported as applied on failure: %v", applied3)
	}
}

// TestChromeConfigProfileAndDialogPolicy covers the Phase 2-D profile switch and
// the configurable dialog policy.
func TestChromeConfigProfileAndDialogPolicy(t *testing.T) {
	on := DefaultChromeConfig()
	on.ProfileDir = "/tmp/co-shell-profile"
	if got := NewChromeManagerWithConfig(9222, true, on).resolveUserDataDir(); got != "/tmp/co-shell-profile" {
		t.Errorf("persist=on must use the configured profile, got %q", got)
	}

	off := on
	off.Persist = false
	got := NewChromeManagerWithConfig(9222, true, off).resolveUserDataDir()
	if got == "/tmp/co-shell-profile" || got == "" {
		t.Errorf("persist=off must use a fresh temporary profile, got %q", got)
	}

	// The legacy constructor keeps passing the profile dir through.
	if got := NewChromeManager(9222, true, "/tmp/legacy").cfg.ProfileDir; got != "/tmp/legacy" {
		t.Errorf("legacy constructor lost the profile dir: %q", got)
	}

	if p, _ := ParseDialogPolicy(""); p != DialogAuto {
		t.Errorf("empty policy should fall back to auto, got %q", p)
	}
	if p, ok := ParseDialogPolicy("dismiss"); !ok || p != DialogDismiss {
		t.Errorf("dismiss policy not honoured, got %q ok=%v", p, ok)
	}
	if _, ok := ParseDialogPolicy("nope"); ok {
		t.Error("invalid dialog policy should not be reported as valid")
	}
}

// TestParseViewportAndExtraHeaders covers the small parsing helpers.
func TestParseViewportAndExtraHeaders(t *testing.T) {
	cases := []struct {
		in   string
		w, h int
		ok   bool
	}{
		{"1440x900", 1440, 900, true},
		{" 800X600 ", 800, 600, true},
		{"", 0, 0, false},
		{"1280", 0, 0, false},
		{"0x100", 0, 0, false},
		{"abcx100", 0, 0, false},
	}
	for _, c := range cases {
		w, h, ok := ParseViewport(c.in)
		if ok != c.ok || (ok && (w != c.w || h != c.h)) {
			t.Errorf("ParseViewport(%q) = %d,%d,%v; want %d,%d,%v", c.in, w, h, ok, c.w, c.h, c.ok)
		}
	}

	headers := ParseExtraHeaders([]string{"X-A: 1", "X-B:2", "broken", ": empty"})
	if len(headers) != 2 || headers["X-A"] != "1" || headers["X-B"] != "2" {
		t.Errorf("ParseExtraHeaders = %v", headers)
	}
	if ParseExtraHeaders(nil) != nil || ParseExtraHeaders([]string{"broken"}) != nil {
		t.Error("ParseExtraHeaders should return nil when nothing is usable")
	}
}
