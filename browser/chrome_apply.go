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
)

// CDPCaller is the minimal CDP surface needed to apply session settings.
// *CDPClient implements it; unit tests substitute a recording fake.
type CDPCaller interface {
	Call(ctx context.Context, method string, params map[string]interface{}) (map[string]interface{}, error)
}

// ApplySessionSettings replays the configured session-level browser settings on
// a freshly connected CDP session (FEATURE-556 Phase 2, "apply-on-connect").
//
// Only configured items produce a CDP call, so an all-default ChromeConfig
// issues no request at all. The returned slice holds the CDP methods that were
// applied, in order, which lets callers log the result and tests assert the
// sequence. Application stops at the first failure and returns the methods
// applied so far together with the error.
func ApplySessionSettings(ctx context.Context, c CDPCaller, cfg ChromeConfig) ([]string, error) {
	var applied []string
	apply := func(method string, params map[string]interface{}) error {
		if _, err := c.Call(ctx, method, params); err != nil {
			return fmt.Errorf("%s: %w", method, err)
		}
		applied = append(applied, method)
		return nil
	}

	if cfg.UserAgent != "" {
		if err := apply("Network.setUserAgentOverride", map[string]interface{}{
			"userAgent": cfg.UserAgent,
		}); err != nil {
			return applied, err
		}
	}
	if headers := ParseExtraHeaders(cfg.ExtraHTTPHeaders); len(headers) > 0 {
		if err := apply("Network.setExtraHTTPHeaders", map[string]interface{}{
			"headers": headers,
		}); err != nil {
			return applied, err
		}
	}
	if width, height, ok := ParseViewport(cfg.Viewport); ok {
		if err := apply("Emulation.setDeviceMetricsOverride", map[string]interface{}{
			"width":             width,
			"height":            height,
			"deviceScaleFactor": 1,
			"mobile":            false,
		}); err != nil {
			return applied, err
		}
	}
	if cfg.Language != "" {
		if err := apply("Emulation.setLocaleOverride", map[string]interface{}{
			"locale": cfg.Language,
		}); err != nil {
			return applied, err
		}
	}
	if cfg.Timezone != "" {
		if err := apply("Emulation.setTimezoneOverride", map[string]interface{}{
			"timezoneId": cfg.Timezone,
		}); err != nil {
			return applied, err
		}
	}
	if cfg.IgnoreCertErrors {
		if err := apply("Security.setIgnoreCertificateErrors", map[string]interface{}{
			"ignore": true,
		}); err != nil {
			return applied, err
		}
	}
	if cfg.DownloadDir != "" {
		if err := apply("Browser.setDownloadBehavior", map[string]interface{}{
			"behavior":      "allow",
			"downloadPath":  cfg.DownloadDir,
			"eventsEnabled": true,
		}); err != nil {
			return applied, err
		}
	}
	return applied, nil
}
