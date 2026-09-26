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

// Multi-tab support built on the Target domain: tabs are listed, opened, closed
// and activated through CDP, while switching to a tab means reconnecting to
// that page's own WebSocket debugger URL — a page session is bound to exactly
// one page, so switching is a reconnect rather than a command (FEATURE-556
// Phase 3).
//
// Target.getTargets does not expose webSocketDebuggerUrl, so the URL needed for
// a switch is read from Chrome's HTTP debug endpoint (/json/list) and matched by
// target id.

package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// targetEndpointTimeout bounds the HTTP debug endpoint call.
	targetEndpointTimeout = 5 * time.Second
	// targetPageType is the CDP target type of a regular tab.
	targetPageType = "page"
)

// TargetInfo describes one debuggable target.
type TargetInfo struct {
	TargetID string
	Type     string
	Title    string
	URL      string
	WSURL    string
	Attached bool
}

// ParseTargets maps a Target.getTargets result into TargetInfo values.
func ParseTargets(result map[string]interface{}) []TargetInfo {
	infos, _ := result["targetInfos"].([]interface{})
	targets := make([]TargetInfo, 0, len(infos))
	for _, raw := range infos {
		info, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		targets = append(targets, TargetInfo{
			TargetID: eventString(info, "targetId"),
			Type:     eventString(info, "type"),
			Title:    eventString(info, "title"),
			URL:      eventString(info, "url"),
			Attached: boolField(info, "attached"),
		})
	}
	return targets
}

// PageTargets keeps the page targets of a target list.
func PageTargets(targets []TargetInfo) []TargetInfo {
	pages := make([]TargetInfo, 0, len(targets))
	for _, target := range targets {
		if target.Type == targetPageType {
			pages = append(pages, target)
		}
	}
	if len(pages) == 0 {
		return nil
	}
	return pages
}

// ListTargets enables target discovery and returns every known target.
func ListTargets(ctx context.Context, caller CDPCaller) ([]TargetInfo, error) {
	if _, err := caller.Call(ctx, "Target.setDiscoverTargets", map[string]interface{}{"discover": true}); err != nil {
		return nil, fmt.Errorf("cannot enable target discovery: %w", err)
	}
	result, err := caller.Call(ctx, "Target.getTargets", map[string]interface{}{})
	if err != nil {
		return nil, fmt.Errorf("cannot list browser targets: %w", err)
	}
	return ParseTargets(result), nil
}

// ListPageTargets returns the page targets of the browser.
func ListPageTargets(ctx context.Context, caller CDPCaller) ([]TargetInfo, error) {
	targets, err := ListTargets(ctx, caller)
	if err != nil {
		return nil, err
	}
	return PageTargets(targets), nil
}

// CreateTarget opens a new page target (about:blank when url is empty) and
// returns its target id.
func CreateTarget(ctx context.Context, caller CDPCaller, url string) (string, error) {
	if strings.TrimSpace(url) == "" {
		url = "about:blank"
	}
	result, err := caller.Call(ctx, "Target.createTarget", map[string]interface{}{"url": url})
	if err != nil {
		return "", fmt.Errorf("cannot open new tab: %w", err)
	}
	targetID := eventString(result, "targetId")
	if targetID == "" {
		return "", fmt.Errorf("new tab has no target id")
	}
	return targetID, nil
}

// CloseTarget closes a page target and reports whether Chrome confirmed it.
func CloseTarget(ctx context.Context, caller CDPCaller, targetID string) (bool, error) {
	if strings.TrimSpace(targetID) == "" {
		return false, fmt.Errorf("target id must not be empty")
	}
	result, err := caller.Call(ctx, "Target.closeTarget", map[string]interface{}{"targetId": targetID})
	if err != nil {
		return false, fmt.Errorf("cannot close tab %s: %w", targetID, err)
	}
	return boolField(result, "success"), nil
}

// ActivateTarget brings a page target to the front.
func ActivateTarget(ctx context.Context, caller CDPCaller, targetID string) error {
	if strings.TrimSpace(targetID) == "" {
		return fmt.Errorf("target id must not be empty")
	}
	if _, err := caller.Call(ctx, "Target.activateTarget", map[string]interface{}{"targetId": targetID}); err != nil {
		return fmt.Errorf("cannot activate tab %s: %w", targetID, err)
	}
	return nil
}

// SelectTarget resolves the requested page target: a 1-based index into pages or
// an explicit target id. The error describes what is available.
func SelectTarget(pages []TargetInfo, index int, targetID string) (TargetInfo, error) {
	if targetID != "" {
		for _, target := range pages {
			if target.TargetID == targetID {
				return target, nil
			}
		}
		return TargetInfo{}, fmt.Errorf("no page target with id %q (available: %s)", targetID, availableTargetIDs(pages))
	}
	if index > 0 {
		if index > len(pages) {
			return TargetInfo{}, fmt.Errorf("tab index %d is out of range (available: 1..%d)", index, len(pages))
		}
		return pages[index-1], nil
	}
	return TargetInfo{}, fmt.Errorf("either index or target_id is required")
}

// availableTargetIDs renders the known target ids for error messages.
func availableTargetIDs(pages []TargetInfo) string {
	if len(pages) == 0 {
		return "none"
	}
	ids := make([]string, 0, len(pages))
	for _, target := range pages {
		ids = append(ids, target.TargetID)
	}
	return strings.Join(ids, ", ")
}

// SwitchTargetURL returns the WebSocket debugger URL to reconnect to in order to
// switch to target.
func SwitchTargetURL(target TargetInfo) (string, error) {
	if strings.TrimSpace(target.WSURL) == "" {
		return "", fmt.Errorf("page target %q has no webSocketDebuggerUrl", target.TargetID)
	}
	return target.WSURL, nil
}

// CurrentTargetIndex returns the index of the page matching url (then title), or
// -1. Only the first match is returned so the "current" marker stays unique.
func CurrentTargetIndex(pages []TargetInfo, url, title string) int {
	if url != "" {
		for i, target := range pages {
			if target.URL == url {
				return i
			}
		}
	}
	if title != "" {
		for i, target := range pages {
			if target.Title == title {
				return i
			}
		}
	}
	return -1
}

// DebugTargets reads Chrome's HTTP debug endpoint (/json/list) and returns every
// target together with its WebSocket debugger URL. It is only used to enrich
// target lists, because CDP target info has no such URL.
func DebugTargets(debugURL string) ([]TargetInfo, error) {
	client := &http.Client{Timeout: targetEndpointTimeout}
	resp, err := client.Get(strings.TrimSuffix(debugURL, "/") + "/json/list")
	if err != nil {
		return nil, fmt.Errorf("cannot fetch target list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("target list returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read target list: %w", err)
	}

	var raw []struct {
		ID                   string `json:"id"`
		Type                 string `json:"type"`
		Title                string `json:"title"`
		URL                  string `json:"url"`
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("cannot parse target list: %w", err)
	}

	targets := make([]TargetInfo, 0, len(raw))
	for _, item := range raw {
		targets = append(targets, TargetInfo{
			TargetID: item.ID,
			Type:     item.Type,
			Title:    item.Title,
			URL:      item.URL,
			WSURL:    item.WebSocketDebuggerURL,
		})
	}
	return targets, nil
}

// AttachDebugURLs copies the WebSocket debugger URLs of the HTTP endpoint into
// the CDP target list (matched by target id) so a page target can be switched to.
func AttachDebugURLs(targets []TargetInfo, debugTargets []TargetInfo) []TargetInfo {
	urls := make(map[string]string, len(debugTargets))
	for _, debug := range debugTargets {
		if debug.WSURL != "" {
			urls[debug.TargetID] = debug.WSURL
		}
	}

	out := make([]TargetInfo, 0, len(targets))
	for _, target := range targets {
		target.WSURL = urls[target.TargetID]
		out = append(out, target)
	}
	return out
}
