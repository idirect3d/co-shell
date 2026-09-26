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
	"fmt"
	"strconv"
	"strings"
)

// ChromeConfig carries browser launch-time and session-level settings
// (FEATURE-556 Phase 2). Each field mirrors one `browser-*` config key.
//
// The zero value keeps the pre-FEATURE-556 behaviour, except for the fields
// set by DefaultChromeConfig. Empty string / nil means "do not override", in
// which case no extra Chrome argument and no extra CDP call is produced.
type ChromeConfig struct {
	// ChromePath is an explicit Chrome/Chromium executable path.
	// Empty means auto-detect from the well-known installation paths.
	ChromePath string
	// ExtraArgs are appended verbatim after the built-in launch arguments.
	ExtraArgs []string
	// Proxy is a proxy server URL, e.g. "http://127.0.0.1:1080".
	Proxy string
	// UserAgent overrides the browser user agent (launch argument plus
	// Network.setUserAgentOverride on connect).
	UserAgent string
	// Language is the browser locale, e.g. "zh-CN".
	Language string
	// Timezone is an IANA timezone id, e.g. "Asia/Shanghai".
	Timezone string
	// ExtraHTTPHeaders are extra request headers in "Name: Value" form.
	ExtraHTTPHeaders []string
	// WindowSize is the launch window size in "WIDTH,HEIGHT" form.
	WindowSize string
	// DownloadDir is the directory downloads are written to.
	DownloadDir string
	// IgnoreCertErrors skips TLS certificate validation errors.
	IgnoreCertErrors bool
	// Extensions keeps browser extensions enabled (default: disabled).
	Extensions bool
	// ProfileDir is Chrome's user data directory. Empty means a temporary dir.
	ProfileDir string
	// Persist keeps the profile across runs; false forces a clean temp profile.
	Persist bool
	// Viewport is the emulated viewport in "WIDTHxHEIGHT" form.
	Viewport string
	// DialogPolicy controls automatic JavaScript dialog handling (auto|accept|dismiss).
	DialogPolicy string
	// ApplyOnConnect replays session-level settings on every new CDP connection.
	ApplyOnConnect bool
}

// DefaultChromeConfig returns the settings that reproduce the pre-FEATURE-556
// hard-coded launch behaviour.
func DefaultChromeConfig() ChromeConfig {
	return ChromeConfig{
		WindowSize:     "1280,800",
		Persist:        true,
		DialogPolicy:   string(DialogAuto),
		ApplyOnConnect: true,
	}
}

// BuildChromeArgs renders the Chrome command line for the given configuration.
// The order is stable so callers (and tests) can rely on it: the built-in flags
// come first, then the optional overrides, then headless/user-data-dir, then
// the user supplied extra arguments.
func BuildChromeArgs(cfg ChromeConfig, port int, headless bool, userDataDir string) []string {
	args := []string{
		fmt.Sprintf("--remote-debugging-port=%d", port),
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-background-networking",
		"--disable-sync",
		"--disable-translate",
	}
	if !cfg.Extensions {
		args = append(args, "--disable-extensions")
	}
	args = append(args,
		"--disable-default-apps",
		"--mute-audio",
		"--disable-features=TranslateUI",
	)
	if cfg.WindowSize != "" {
		args = append(args, "--window-size="+cfg.WindowSize)
	}
	if cfg.Proxy != "" {
		args = append(args, "--proxy-server="+cfg.Proxy)
	}
	if cfg.UserAgent != "" {
		args = append(args, "--user-agent="+cfg.UserAgent)
	}
	if cfg.Language != "" {
		args = append(args, "--lang="+cfg.Language)
	}
	if cfg.IgnoreCertErrors {
		args = append(args, "--ignore-certificate-errors")
	}
	if headless {
		args = append(args, "--headless=new")
	}
	if userDataDir != "" {
		args = append(args, "--user-data-dir="+userDataDir)
	}
	args = append(args, cfg.ExtraArgs...)
	args = append(args, "about:blank")
	return args
}

// ParseViewport parses a "WIDTHxHEIGHT" viewport string (e.g. "1440x900").
// The second return value reports whether the value was usable.
func ParseViewport(s string) (int, int, bool) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(s)), "x")
	if len(parts) != 2 {
		return 0, 0, false
	}
	w, errW := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, errH := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errW != nil || errH != nil || w <= 0 || h <= 0 {
		return 0, 0, false
	}
	return w, h, true
}

// ParseExtraHeaders converts "Name: Value" entries into the header map used by
// Network.setExtraHTTPHeaders. Entries without a colon are ignored; nil is
// returned when nothing usable remains.
func ParseExtraHeaders(entries []string) map[string]string {
	if len(entries) == 0 {
		return nil
	}
	headers := make(map[string]string, len(entries))
	for _, entry := range entries {
		name, value, found := strings.Cut(entry, ":")
		name = strings.TrimSpace(name)
		if !found || name == "" {
			continue
		}
		headers[name] = strings.TrimSpace(value)
	}
	if len(headers) == 0 {
		return nil
	}
	return headers
}
