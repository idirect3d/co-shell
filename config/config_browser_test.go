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

package config

import (
	"encoding/json"
	"testing"
)

// TestBrowserConfigDefaultsMatchCurrentBehaviour verifies that every
// FEATURE-556 browser key defaults to the pre-FEATURE-556 behaviour (A-009 /
// B-024 prerequisite): the only non-empty defaults are the ones that keep the
// legacy launch arguments and the legacy dialog handling.
func TestBrowserConfigDefaultsMatchCurrentBehaviour(t *testing.T) {
	llm := DefaultConfig().LLM

	if llm.BrowserWindowSize != "1280,800" {
		t.Errorf("BrowserWindowSize default = %q, want %q", llm.BrowserWindowSize, "1280,800")
	}
	if !llm.BrowserPersist {
		t.Error("BrowserPersist default must be true (profile kept across restarts)")
	}
	if llm.BrowserDialogPolicy != "auto" {
		t.Errorf("BrowserDialogPolicy default = %q, want auto", llm.BrowserDialogPolicy)
	}
	if !llm.BrowserApplyOnConnect {
		t.Error("BrowserApplyOnConnect default must be true")
	}
	if llm.BrowserExtensions {
		t.Error("BrowserExtensions default must be false (extensions disabled, as before)")
	}
	if llm.BrowserIgnoreCertErrors {
		t.Error("BrowserIgnoreCertErrors default must be false (no certificate exception)")
	}

	// Every opt-in key must default to "no override": no extra Chrome argument
	// and no extra CDP call.
	overrides := map[string]string{
		"BrowserChromePath":  llm.BrowserChromePath,
		"BrowserProxy":       llm.BrowserProxy,
		"BrowserUserAgent":   llm.BrowserUserAgent,
		"BrowserLanguage":    llm.BrowserLanguage,
		"BrowserTimezone":    llm.BrowserTimezone,
		"BrowserDownloadDir": llm.BrowserDownloadDir,
		"BrowserProfileDir":  llm.BrowserProfileDir,
		"BrowserViewport":    llm.BrowserViewport,
	}
	for name, value := range overrides {
		if value != "" {
			t.Errorf("%s default = %q, want empty (no override)", name, value)
		}
	}
	if len(llm.BrowserExtraArgs) != 0 {
		t.Errorf("BrowserExtraArgs default = %v, want empty", llm.BrowserExtraArgs)
	}
	if len(llm.BrowserExtraHTTPHeaders) != 0 {
		t.Errorf("BrowserExtraHTTPHeaders default = %v, want empty", llm.BrowserExtraHTTPHeaders)
	}
}

// TestBrowserConfigJSONRoundTrip is the "configuration is remembered" check: the
// new browser keys survive a config.json marshal/unmarshal cycle, so a restart
// of co-shell keeps them.
func TestBrowserConfigJSONRoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LLM.BrowserProxy = "http://127.0.0.1:1080"
	cfg.LLM.BrowserUserAgent = "co-shell-test/1.0"
	cfg.LLM.BrowserLanguage = "zh-CN"
	cfg.LLM.BrowserTimezone = "Asia/Shanghai"
	cfg.LLM.BrowserViewport = "1440x900"
	cfg.LLM.BrowserWindowSize = "1440,900"
	cfg.LLM.BrowserDownloadDir = "/tmp/dl"
	cfg.LLM.BrowserProfileDir = "/tmp/profile"
	cfg.LLM.BrowserChromePath = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	cfg.LLM.BrowserExtraArgs = []string{"--disable-gpu"}
	cfg.LLM.BrowserExtraHTTPHeaders = []string{"X-A: 1"}
	cfg.LLM.BrowserExtensions = true
	cfg.LLM.BrowserIgnoreCertErrors = true
	cfg.LLM.BrowserPersist = false
	cfg.LLM.BrowserDialogPolicy = "dismiss"
	cfg.LLM.BrowserApplyOnConnect = false

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}

	loaded := DefaultConfig()
	if err := json.Unmarshal(data, loaded); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	got := loaded.LLM
	if got.BrowserProxy != cfg.LLM.BrowserProxy ||
		got.BrowserUserAgent != cfg.LLM.BrowserUserAgent ||
		got.BrowserLanguage != cfg.LLM.BrowserLanguage ||
		got.BrowserTimezone != cfg.LLM.BrowserTimezone ||
		got.BrowserViewport != cfg.LLM.BrowserViewport ||
		got.BrowserWindowSize != cfg.LLM.BrowserWindowSize ||
		got.BrowserDownloadDir != cfg.LLM.BrowserDownloadDir ||
		got.BrowserProfileDir != cfg.LLM.BrowserProfileDir ||
		got.BrowserChromePath != cfg.LLM.BrowserChromePath ||
		got.BrowserDialogPolicy != cfg.LLM.BrowserDialogPolicy {
		t.Errorf("string browser settings lost in round trip: %+v", got)
	}
	if got.BrowserExtensions != true || got.BrowserIgnoreCertErrors != true {
		t.Errorf("bool browser settings lost in round trip: %+v", got)
	}
	if got.BrowserPersist != false || got.BrowserApplyOnConnect != false {
		t.Errorf("false bool browser settings lost in round trip: %+v", got)
	}
	if len(got.BrowserExtraArgs) != 1 || got.BrowserExtraArgs[0] != "--disable-gpu" {
		t.Errorf("BrowserExtraArgs lost in round trip: %v", got.BrowserExtraArgs)
	}
	if len(got.BrowserExtraHTTPHeaders) != 1 || got.BrowserExtraHTTPHeaders[0] != "X-A: 1" {
		t.Errorf("BrowserExtraHTTPHeaders lost in round trip: %v", got.BrowserExtraHTTPHeaders)
	}
}

// TestBrowserWindowModeConfig covers FEATURE-569: browser_window_mode defaults
// to "minimized" and survives a config.json marshal/unmarshal cycle.
func TestBrowserWindowModeConfig(t *testing.T) {
	if got := DefaultConfig().LLM.BrowserWindowMode; got != "minimized" {
		t.Errorf("BrowserWindowMode default = %q, want minimized", got)
	}

	cfg := DefaultConfig()
	cfg.LLM.BrowserWindowMode = "maximized"
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	loaded := DefaultConfig()
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if loaded.LLM.BrowserWindowMode != "maximized" {
		t.Errorf("BrowserWindowMode lost in round trip: %q", loaded.LLM.BrowserWindowMode)
	}
}
