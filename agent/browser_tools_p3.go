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

// FEATURE-556 Phase 3: browser tools built on the additional CDP domains —
// console capture, file upload, PDF export, network observation, multi-tab
// handling and request mocking.
//
// Every capability is activated on demand: a collector is created (and its CDP
// domain enabled) the first time its tool runs, so browsing without these tools
// behaves exactly as before.

package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/idirect3d/co-shell/browser"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
)

// browserArgString reads an optional string argument.
func browserArgString(args map[string]interface{}, key string) string {
	value, _ := args[key].(string)
	return strings.TrimSpace(value)
}

// browserArgRawString reads an optional string argument without trimming it
// (used for mock response bodies).
func browserArgRawString(args map[string]interface{}, key string) string {
	value, _ := args[key].(string)
	return value
}

// browserArgBool reads an optional boolean argument.
func browserArgBool(args map[string]interface{}, key string) bool {
	value, _ := args[key].(bool)
	return value
}

// browserArgInt reads an optional integer argument.
func browserArgInt(args map[string]interface{}, key string) int {
	value, _ := args[key].(float64)
	return int(value)
}

// browserArgStringList reads an optional array-of-strings argument.
func browserArgStringList(args map[string]interface{}, key string) []string {
	raw, ok := args[key].([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if value, ok := item.(string); ok {
			out = append(out, value)
		}
	}
	return out
}

// browserArgStringMap reads an optional object-of-strings argument.
func browserArgStringMap(args map[string]interface{}, key string) map[string]string {
	raw, ok := args[key].(map[string]interface{})
	if !ok || len(raw) == 0 {
		return nil
	}
	out := make(map[string]string, len(raw))
	for name, value := range raw {
		if text, ok := value.(string); ok {
			out[name] = text
		}
	}
	return out
}

// browserToolError wraps a failure in the localized tool error prefix.
func browserToolError(detail interface{}) error {
	return fmt.Errorf(i18n.T(i18n.KeyBrowserToolError), detail)
}

// browserConsoleTool reports the console messages collected since the last call.
// Capture (Runtime.enable + Log.enable) is enabled on first use.
func (a *Agent) browserConsoleTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := a.ensureBrowserReady(ctx); err != nil {
		return "", err
	}

	manager, err := a.chromeMgr.EnsureConsoleCapture(ctx)
	if manager == nil {
		return "", browserToolError(err)
	}
	if err != nil {
		// Capture is best effort: reading an empty buffer is still valid.
		log.Warn("browser console capture: %v", err)
	}

	if browserArgBool(args, "clear") {
		manager.TakeMessages()
		return i18n.T(i18n.KeyBrowserConsoleEmpty), nil
	}

	messages := manager.TakeMessages()
	if len(messages) == 0 {
		return i18n.T(i18n.KeyBrowserConsoleEmpty), nil
	}

	lines := browser.ConsoleSummary(messages, browser.ConsoleSummaryEntries, browser.ConsoleSummaryChars)
	body := strings.Join(lines, "\n")
	if omitted := len(messages) - len(lines); omitted > 0 {
		body += "\n" + i18n.TF(i18n.KeyBrowserMoreEntries, omitted)
	}

	log.Info("Browser console: %d new message(s)", len(messages))
	return fmt.Sprintf(i18n.T(i18n.KeyBrowserConsoleResult), len(messages), len(lines), body), nil
}

// browserUploadTool attaches workspace files to a file input.
func (a *Agent) browserUploadTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := a.ensureBrowserReady(ctx); err != nil {
		return "", err
	}

	selector := browserSelector(args)
	if selector == "" {
		return "", browserToolError("selector is required")
	}
	files := browserArgStringList(args, "files")
	if len(files) == 0 {
		return "", browserToolError("files is required")
	}

	resolved, err := browser.WorkspaceFiles(a.workspacePath, files)
	if err != nil {
		return "", browserToolError(err)
	}

	if err := a.chromeMgr.Client().SetFileInputFiles(ctx, selector, resolved); err != nil {
		return "", browserToolError(err)
	}

	log.Info("Browser upload: %d file(s) -> %s", len(resolved), selector)
	return fmt.Sprintf(i18n.T(i18n.KeyBrowserUploadResult), len(resolved), selector, strings.Join(resolved, "\n")), nil
}

// browserPdfTool exports the current page as PDF and saves it to disk.
func (a *Agent) browserPdfTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := a.ensureBrowserReady(ctx); err != nil {
		return "", err
	}

	cdp := a.chromeMgr.Client()

	options := browser.PDFOptions{}
	if landscape, ok := args["landscape"].(bool); ok {
		options.Landscape = &landscape
	}
	if printBackground, ok := args["print_background"].(bool); ok {
		options.PrintBackground = &printBackground
	}
	if paperWidth, ok := args["paper_width"].(float64); ok {
		options.PaperWidth = &paperWidth
	}
	if paperHeight, ok := args["paper_height"].(float64); ok {
		options.PaperHeight = &paperHeight
	}
	if scale, ok := args["scale"].(float64); ok {
		options.Scale = &scale
	}

	data, err := browser.PrintToPDF(ctx, cdp, options)
	if err != nil {
		return "", browserToolError(err)
	}

	path := browserArgString(args, "path")
	if path == "" {
		title, titleErr := cdp.GetPageTitle(ctx)
		if titleErr != nil {
			title = ""
		}
		path = filepath.Join(a.workspacePath, browser.PDFDefaultDir, browser.PDFFileName(title))
	} else if !filepath.IsAbs(path) {
		path = filepath.Join(a.workspacePath, path)
	}

	size, err := browser.SavePDF(data, path)
	if err != nil {
		return "", browserToolError(err)
	}

	log.Info("Browser PDF saved to %s (%d bytes)", path, size)
	return fmt.Sprintf(i18n.T(i18n.KeyBrowserPdfResult), path, size), nil
}

// browserNetworkTool reports the observed network requests.
func (a *Agent) browserNetworkTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := a.ensureBrowserReady(ctx); err != nil {
		return "", err
	}

	collector, err := a.chromeMgr.EnsureNetworkCollector(ctx)
	if collector == nil {
		return "", browserToolError(err)
	}
	if err != nil {
		log.Warn("browser network observation: %v", err)
	}

	onlyFailures := browserArgBool(args, "only_failures")
	entries := collector.Entries(onlyFailures, browserArgInt(args, "limit"))
	if len(entries) == 0 {
		if onlyFailures {
			return i18n.T(i18n.KeyBrowserNetworkNoFailure), nil
		}
		return i18n.T(i18n.KeyBrowserNetworkEmpty), nil
	}

	lines := browser.NetworkSummary(entries, len(entries), browser.NetworkSummaryChars)
	body := strings.Join(lines, "\n")

	// Failures observed so far are summarised separately, capped at three lines.
	observed := collector.Entries(false, browser.NetworkMaxLimit)
	if failures, omitted := browser.NetworkFailureLines(observed, browser.NetworkSummaryEntries, browser.NetworkSummaryChars); len(failures) > 0 {
		body += "\n" + i18n.T(i18n.KeyBrowserNetworkFailures) + "\n" + strings.Join(failures, "\n")
		if omitted > 0 {
			body += "\n" + i18n.TF(i18n.KeyBrowserMoreEntries, omitted)
		}
	}

	log.Info("Browser network: %d entry(ies) reported", len(entries))
	return fmt.Sprintf(i18n.T(i18n.KeyBrowserNetworkResult), len(entries), len(lines), body), nil
}

// browserTabTool lists, opens, closes or switches browser tabs.
func (a *Agent) browserTabTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := a.ensureBrowserReady(ctx); err != nil {
		return "", err
	}

	action := strings.ToLower(browserArgString(args, "action"))
	if action == "" {
		action = "list"
	}
	cdp := a.chromeMgr.Client()

	switch action {
	case "list":
		pages, err := a.chromeMgr.PageTargets(ctx)
		if err != nil {
			return "", browserToolError(err)
		}
		currentURL, _ := cdp.GetCurrentURL(ctx)
		title, _ := cdp.GetPageTitle(ctx)
		current := browser.CurrentTargetIndex(pages, currentURL, title)

		lines := make([]string, 0, len(pages))
		for i, page := range pages {
			marker := "  "
			if i == current {
				marker = "* "
			}
			lines = append(lines, fmt.Sprintf("%s%d. %s | %s | %s", marker, i+1, page.TargetID, page.Title, page.URL))
		}
		return fmt.Sprintf(i18n.T(i18n.KeyBrowserTabList), len(pages), strings.Join(lines, "\n")), nil

	case "open":
		url := browserArgString(args, "url")
		targetID, err := browser.CreateTarget(ctx, cdp, url)
		if err != nil {
			return "", browserToolError(err)
		}
		if url == "" {
			url = "about:blank"
		}
		log.Info("Browser tab opened: %s (%s)", targetID, url)
		return fmt.Sprintf(i18n.T(i18n.KeyBrowserTabOpened), targetID, url), nil

	case "switch":
		target, err := a.selectBrowserTarget(ctx, args)
		if err != nil {
			return "", browserToolError(err)
		}
		wsURL, err := browser.SwitchTargetURL(target)
		if err != nil {
			return "", browserToolError(err)
		}
		if err := a.chromeMgr.SelectTarget(wsURL); err != nil {
			return "", browserToolError(err)
		}
		_ = browser.ActivateTarget(ctx, a.chromeMgr.Client(), target.TargetID)

		currentURL, _ := a.chromeMgr.Client().GetCurrentURL(ctx)
		title, _ := a.chromeMgr.Client().GetPageTitle(ctx)
		log.Info("Browser tab switched to %s (%s)", target.TargetID, currentURL)
		return fmt.Sprintf(i18n.T(i18n.KeyBrowserTabSwitched), target.TargetID, currentURL, title), nil

	case "close":
		target, err := a.selectBrowserTarget(ctx, args)
		if err != nil {
			return "", browserToolError(err)
		}
		isCurrent := a.isCurrentTarget(ctx, target.TargetID)

		if _, err := browser.CloseTarget(ctx, cdp, target.TargetID); err != nil {
			return "", browserToolError(err)
		}
		log.Info("Browser tab closed: %s (current=%v)", target.TargetID, isCurrent)

		if isCurrent {
			// The session can no longer use the closed page: drop the connection
			// so the next call reconnects to an available tab.
			a.chromeMgr.DropConnection()
			return fmt.Sprintf("%s\n%s",
				fmt.Sprintf(i18n.T(i18n.KeyBrowserTabClosed), target.TargetID),
				i18n.T(i18n.KeyBrowserTabClosedCurrent)), nil
		}
		return fmt.Sprintf(i18n.T(i18n.KeyBrowserTabClosed), target.TargetID), nil

	default:
		return "", browserToolError(fmt.Sprintf("unknown action %q (valid: list, open, switch, close)", action))
	}
}

// selectBrowserTarget resolves the tab requested by index or target id.
func (a *Agent) selectBrowserTarget(ctx context.Context, args map[string]interface{}) (browser.TargetInfo, error) {
	pages, err := a.chromeMgr.PageTargets(ctx)
	if err != nil {
		return browser.TargetInfo{}, err
	}
	return browser.SelectTarget(pages, browserArgInt(args, "index"), browserArgString(args, "target_id"))
}

// isCurrentTarget reports whether targetID is the page the session is attached to.
func (a *Agent) isCurrentTarget(ctx context.Context, targetID string) bool {
	pages, err := a.chromeMgr.PageTargets(ctx)
	if err != nil {
		return false
	}
	currentURL, _ := a.chromeMgr.Client().GetCurrentURL(ctx)
	title, _ := a.chromeMgr.Client().GetPageTitle(ctx)

	index := browser.CurrentTargetIndex(pages, currentURL, title)
	return index >= 0 && pages[index].TargetID == targetID
}

// browserMockTool manages the in-memory request mocking rules.
func (a *Agent) browserMockTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := a.ensureBrowserReady(ctx); err != nil {
		return "", err
	}

	action := strings.ToLower(browserArgString(args, "action"))
	mock, err := a.chromeMgr.Mock()
	if err != nil {
		return "", browserToolError(err)
	}

	switch action {
	case "add":
		rule, err := browserMockRule(args)
		if err != nil {
			return "", browserToolError(err)
		}
		if err := mock.AddRule(ctx, rule); err != nil {
			return "", browserToolError(err)
		}
		rules := mock.Rules()
		log.Info("Browser mock rule added (%d active): %s", len(rules), rule.Describe())
		return fmt.Sprintf(i18n.T(i18n.KeyBrowserMockRules), len(rules), browserMockRuleLines(rules)), nil

	case "list":
		rules := mock.Rules()
		if len(rules) == 0 {
			return i18n.T(i18n.KeyBrowserMockEmpty), nil
		}
		return fmt.Sprintf(i18n.T(i18n.KeyBrowserMockRules), len(rules), browserMockRuleLines(rules)), nil

	case "clear", "off":
		if err := mock.Off(ctx); err != nil {
			return "", browserToolError(err)
		}
		log.Info("Browser mocks cleared (action=%s)", action)
		return i18n.T(i18n.KeyBrowserMockCleared), nil

	default:
		return "", browserToolError(fmt.Sprintf("unknown action %q (valid: add, list, clear, off)", action))
	}
}

// browserMockRule builds a rule from the tool arguments. The rule action
// defaults to fulfill, or to fail when an error_reason is given.
func browserMockRule(args map[string]interface{}) (browser.MockRule, error) {
	errorReason := browserArgString(args, "error_reason")
	ruleAction := strings.ToLower(browserArgString(args, "rule_action"))
	if ruleAction == "" {
		if errorReason != "" {
			ruleAction = browser.MockActionFail
		} else {
			ruleAction = browser.MockActionFulfill
		}
	}

	status := browserArgInt(args, "status")
	if status == 0 {
		status = 200
	}

	rule := browser.MockRule{
		Match:       browserArgString(args, "match"),
		Glob:        browserArgBool(args, "glob"),
		Action:      ruleAction,
		Status:      status,
		Headers:     browserArgStringMap(args, "headers"),
		Body:        browserArgRawString(args, "body"),
		Base64Body:  browserArgBool(args, "base64_body"),
		ErrorReason: errorReason,
	}
	if err := rule.Validate(); err != nil {
		return browser.MockRule{}, err
	}
	return rule, nil
}

// browserMockRuleLines renders the active rules for tool output.
func browserMockRuleLines(rules []browser.MockRule) string {
	lines := make([]string, 0, len(rules))
	for i, rule := range rules {
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, rule.Describe()))
	}
	return strings.Join(lines, "\n")
}
