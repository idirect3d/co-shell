// Author: L.Shuang
// Created: 2026-06-04
// Last Modified: 2026-06-04
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
// IMPLIED, BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package agent

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/browser"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
)

// ensureBrowserReady ensures the Chrome browser is started and connected.
// Returns the CDPClient or an error.
func (a *Agent) ensureBrowserReady(ctx context.Context) error {
	if !a.browserEnabled {
		return fmt.Errorf("browser is not enabled. Use .set browser-enabled on to enable")
	}
	if a.chromeMgr == nil {
		if err := a.EnsureBrowserStarted(); err != nil {
			return fmt.Errorf("cannot start browser: %w", err)
		}
	}
	_, err := a.chromeMgr.EnsurePageConnected(ctx)
	return err
}

// appendBrowserDialogNotes appends the JavaScript dialogs that were answered
// automatically during this browser tool call (FEATURE-556). Without it the
// model cannot tell that the page raised, for example, a confirm dialog that
// was dismissed on its behalf.
func (a *Agent) appendBrowserDialogNotes(result string) string {
	if a.chromeMgr == nil {
		return result
	}
	dialogs := a.chromeMgr.TakeDialogMessages()
	if len(dialogs) == 0 {
		return result
	}

	var builder strings.Builder
	builder.WriteString(result)
	for _, dialog := range dialogs {
		builder.WriteString("\n")
		builder.WriteString(i18n.TF(i18n.KeyBrowserDialogHandled, dialog.Type, dialog.Message, dialog.Handled))
	}
	return builder.String()
}

// browserElementTimeout bounds element lookup for selector-based tools.
const browserElementTimeout = 5 * time.Second

// browserMouseButton reads the optional mouse button argument (default: left).
func browserMouseButton(args map[string]interface{}) (string, error) {
	button, _ := args["button"].(string)
	return browser.NormalizeMouseButton(button)
}

// browserSelector reads the optional selector argument.
func browserSelector(args map[string]interface{}) string {
	selector, _ := args["selector"].(string)
	return strings.TrimSpace(selector)
}

// browserNavigateTool navigates the browser to a given URL.
func (a *Agent) browserNavigateTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := a.ensureBrowserReady(ctx); err != nil {
		return "", err
	}

	url, ok := args["url"].(string)
	if !ok || url == "" {
		return "", fmt.Errorf("url is required")
	}

	cdp := a.chromeMgr.Client()
	if _, err := cdp.Navigate(ctx, url); err != nil {
		return "", fmt.Errorf("cannot navigate to %s: %w", url, err)
	}

	currentURL, err := cdp.GetCurrentURL(ctx)
	if err != nil {
		currentURL = url
	}
	title, err := cdp.GetPageTitle(ctx)
	if err != nil {
		title = ""
	}

	log.Info("Browser navigate: %s -> %s (title: %s)", url, currentURL, title)

	return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_686), currentURL, title), nil
}

// browserScreenshotTool captures a screenshot, saves it to ./download/screenshot/,
// and automatically loads it into image cache if the model supports vision.
func (a *Agent) browserScreenshotTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := a.ensureBrowserReady(ctx); err != nil {
		return "", err
	}

	cdp := a.chromeMgr.Client()

	quality := 80
	if q, ok := args["quality"].(float64); ok && q > 0 && q <= 100 {
		quality = int(q)
	}
	fullPage := false
	if fp, ok := args["full_page"].(bool); ok {
		fullPage = fp
	}

	// FEATURE-556: a selector captures just that element's box; without it the
	// historical viewport / full-page behaviour is kept.
	selector := browserSelector(args)
	var screenshotData string
	var err error
	if selector != "" {
		box, boxErr := cdp.ElementBox(ctx, selector, browserElementTimeout)
		if boxErr != nil {
			return "", boxErr
		}
		screenshotData, err = cdp.CaptureScreenshotClip(ctx, quality, box.Clip())
		if err != nil {
			return "", fmt.Errorf("cannot capture element screenshot: %w", err)
		}
		fullPage = false
	} else {
		screenshotData, err = cdp.CaptureScreenshot(ctx, quality, fullPage)
		if err != nil {
			return "", fmt.Errorf("cannot capture screenshot: %w", err)
		}
	}

	currentURL, _ := cdp.GetCurrentURL(ctx)
	title, _ := cdp.GetPageTitle(ctx)

	// Decode base64 and save to ./download/screenshot/
	screenshotBytes, err := base64.StdEncoding.DecodeString(screenshotData)
	if err != nil {
		return "", fmt.Errorf("cannot decode screenshot data: %w", err)
	}

	ts := time.Now().Format("20060102_150405")
	screenshotPath := filepath.Join(".", "download", "screenshot", fmt.Sprintf("browser_screenshot_%s.jpg", ts))
	if err := os.MkdirAll(filepath.Dir(screenshotPath), 0755); err != nil {
		return "", fmt.Errorf("cannot create screenshot directory: %w", err)
	}
	if err := os.WriteFile(screenshotPath, screenshotBytes, 0644); err != nil {
		return "", fmt.Errorf("cannot write screenshot file: %w", err)
	}

	// Cache in memory
	a.mu.Lock()
	a.browserScreenshotData = screenshotData
	a.mu.Unlock()

	log.Info("Browser screenshot saved to %s (quality=%d, fullPage=%v, selector=%q, size=%d bytes)", screenshotPath, quality, fullPage, selector, len(screenshotBytes))

	// Check vision support before auto-loading to image cache
	visionSupported := false
	if a.cfg != nil {
		visionSupported = a.cfg.LLM.VisionSupport
	}

	baseMsg := fmt.Sprintf(i18n.T(i18n.KeySettingCmd_687),
		screenshotPath, currentURL, title, quality, fullPage)

	if visionSupported {
		// FEATURE-346: carry the screenshot into the FEATURE-343 minimal
		// recognition round exactly like visual_analysis — set the pending
		// instruct (from the tool's instruct argument, with a localized
		// fallback) so buildContextMessages collapses to
		// [Identity-only, instruct+image] and the recognition result is
		// backfilled as this tool's return. FEATURE-447: the vision
		// instruction is the explicit "instruct" parameter, distinct from
		// meta.intent (which is the intent shown to the user).
		instruct, _ := args["instruct"].(string)
		if strings.TrimSpace(instruct) == "" {
			instruct = i18n.T(i18n.KeyBrowserScreenshotVisionIntentDefault)
		}
		a.mu.Lock()
		a.imagePaths = []string{screenshotPath}
		a.visionPendingIntent = instruct
		a.mu.Unlock()
		baseMsg += i18n.T(i18n.KeySettingCmd_688)
		baseMsg += i18n.T(i18n.KeySettingCmd_689)
	} else {
		baseMsg += i18n.T(i18n.KeySettingCmd_690)
	}

	return baseMsg, nil
}

// browserClickTool clicks at the specified coordinates.
func (a *Agent) browserClickTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := a.ensureBrowserReady(ctx); err != nil {
		return "", err
	}

	cdp := a.chromeMgr.Client()

	button, err := browserMouseButton(args)
	if err != nil {
		return "", err
	}
	clickCount := 1
	if cc, ok := args["click_count"].(float64); ok && cc >= 1 {
		clickCount = int(cc)
	}

	// FEATURE-556: an explicit selector wins over raw coordinates; coordinates
	// stay supported for backward compatibility.
	selector := browserSelector(args)
	var x, y float64
	if selector != "" {
		box, boxErr := cdp.ElementBox(ctx, selector, browserElementTimeout)
		if boxErr != nil {
			return "", boxErr
		}
		x, y = box.Center()
	} else {
		var okX, okY bool
		x, okX = args["x"].(float64)
		y, okY = args["y"].(float64)
		if !okX || !okY {
			return "", fmt.Errorf("either selector or x/y coordinates are required")
		}
	}

	if err := cdp.ClickEx(ctx, x, y, button, clickCount); err != nil {
		return "", fmt.Errorf("cannot click at (%f, %f): %w", x, y, err)
	}

	log.Info("Browser click at (%f, %f) (selector=%q, button=%s, clickCount=%d)", x, y, selector, button, clickCount)
	currentURL, _ := cdp.GetCurrentURL(ctx)

	return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_691), x, y, currentURL), nil
}

// browserTypeTool sends text input to the focused element.
func (a *Agent) browserTypeTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := a.ensureBrowserReady(ctx); err != nil {
		return "", err
	}

	cdp := a.chromeMgr.Client()

	text, ok := args["text"].(string)
	if !ok {
		return "", fmt.Errorf("text is required")
	}

	// FEATURE-556: an optional selector focuses the target element first.
	selector := browserSelector(args)
	if selector != "" {
		box, boxErr := cdp.ElementBox(ctx, selector, browserElementTimeout)
		if boxErr != nil {
			return "", boxErr
		}
		cx, cy := box.Center()
		if err := cdp.ClickEx(ctx, cx, cy, browser.MouseButtonLeft, 1); err != nil {
			return "", fmt.Errorf("cannot focus %q: %w", selector, err)
		}
	}

	clear := false
	if cl, ok := args["clear"].(bool); ok {
		clear = cl
	}
	if clear {
		_, _ = cdp.Evaluate(ctx, `document.activeElement?.select?.()`)
		_, _ = cdp.Evaluate(ctx, `document.execCommand('delete')`)
	}

	if err := cdp.Type(ctx, text); err != nil {
		return "", fmt.Errorf("cannot type text: %w", err)
	}

	log.Info("Browser type: %q (clear=%v, selector=%q)", text, clear, selector)
	return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_692), text), nil
}

// browserEvaluateTool executes JavaScript in the browser.
func (a *Agent) browserEvaluateTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := a.ensureBrowserReady(ctx); err != nil {
		return "", err
	}

	cdp := a.chromeMgr.Client()

	expression, ok := args["expression"].(string)
	if !ok || expression == "" {
		return "", fmt.Errorf("expression is required")
	}

	result, err := cdp.Evaluate(ctx, expression)
	if err != nil {
		return "", fmt.Errorf("JavaScript execution error: %w", err)
	}

	log.Info("Browser evaluate: %q", expression)
	if result == nil {
		return i18n.T(i18n.KeySettingCmd_693), nil
	}
	return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_694), result), nil
}

// browserGetHTMLTool returns the rendered DOM HTML of the current browser page.
// The HTML is obtained from Chrome's live DOM tree via CDP (DOM.getDocument +
// DOM.getOuterHTML), meaning it reflects the final state after all JavaScript
// has executed — including SPA framework output, dynamic content, and all DOM
// modifications. This is NOT the raw source HTML. There is no need to separately
// download JS, JSON, or other resources referenced in the page source.
// The HTML is always saved to ./download/html/ for data integrity, regardless
// of size. If it exceeds the configured max size (default 10KB), the file
// path is returned instead of the full content. If it fits within the limit,
// the full HTML is returned along with the file path for later reference.
func (a *Agent) browserGetHTMLTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := a.ensureBrowserReady(ctx); err != nil {
		return "", err
	}

	cdp := a.chromeMgr.Client()

	nodeID, err := cdp.GetDocument(ctx)
	if err != nil {
		return "", fmt.Errorf("cannot get DOM document: %w", err)
	}
	html, err := cdp.GetOuterHTML(ctx, nodeID)
	if err != nil {
		return "", fmt.Errorf("cannot get HTML: %w", err)
	}

	log.Info("Browser get HTML (%d bytes)", len(html))

	// Count lines for reference
	lineCount := strings.Count(html, "\n") + 1

	// Determine max HTML size: config override or default 10KB
	maxHTMLSize := 10240 // 10KB default
	if a.cfg != nil && a.cfg.LLM.BrowserMaxHTMLSize > 0 {
		maxHTMLSize = a.cfg.LLM.BrowserMaxHTMLSize
	}

	// Always save to file for data integrity, regardless of size
	ts := time.Now().Format("20060102_150405")
	htmlPath := filepath.Join(".", "download", "html", fmt.Sprintf("browser_html_%s.html", ts))
	if err := os.MkdirAll(filepath.Dir(htmlPath), 0755); err != nil {
		return "", fmt.Errorf("cannot create html download directory: %w", err)
	}
	if err := os.WriteFile(htmlPath, []byte(html), 0644); err != nil {
		return "", fmt.Errorf("cannot write html file: %w", err)
	}

	log.Info("Browser HTML saved to %s (%d bytes)", htmlPath, len(html))

	if len(html) <= maxHTMLSize {
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_695),
			lineCount, len(html), html, htmlPath), nil
	}

	return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_696),
		lineCount, len(html), htmlPath, maxHTMLSize, maxHTMLSize/1024), nil
}

// browserScrollTool scrolls the page by the specified delta.
func (a *Agent) browserScrollTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := a.ensureBrowserReady(ctx); err != nil {
		return "", err
	}

	cdp := a.chromeMgr.Client()

	deltaX := 0.0
	deltaY := 500.0
	if dx, ok := args["delta_x"].(float64); ok {
		deltaX = dx
	}
	if dy, ok := args["delta_y"].(float64); ok {
		deltaY = dy
	}

	if err := cdp.ScrollBy(ctx, deltaX, deltaY); err != nil {
		return "", fmt.Errorf("cannot scroll: %w", err)
	}

	log.Info("Browser scroll (deltaX=%f, deltaY=%f)", deltaX, deltaY)
	direction := i18n.T(i18n.KeySettingCmd_697)
	if deltaY < 0 {
		direction = i18n.T(i18n.KeySettingCmd_704)
	}
	return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_698), direction, deltaY), nil
}

// browserGetInteractiveElementsTool returns interactive elements info.
func (a *Agent) browserGetInteractiveElementsTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := a.ensureBrowserReady(ctx); err != nil {
		return "", err
	}

	cdp := a.chromeMgr.Client()

	elementsJSON, err := cdp.GetInteractiveElements(ctx)
	if err != nil {
		return "", fmt.Errorf("cannot get interactive elements: %w", err)
	}

	log.Info("Browser get interactive elements (%d bytes)", len(elementsJSON))

	return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_699), elementsJSON), nil
}

// browserGoBackTool navigates back in history.
func (a *Agent) browserGoBackTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := a.ensureBrowserReady(ctx); err != nil {
		return "", err
	}

	cdp := a.chromeMgr.Client()

	if _, err := cdp.GoBack(ctx); err != nil {
		return "", fmt.Errorf("cannot go back: %w", err)
	}

	currentURL, _ := cdp.GetCurrentURL(ctx)
	title, _ := cdp.GetPageTitle(ctx)

	return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_700), currentURL, title), nil
}

// browserGoForwardTool navigates forward in history.
func (a *Agent) browserGoForwardTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := a.ensureBrowserReady(ctx); err != nil {
		return "", err
	}

	cdp := a.chromeMgr.Client()

	if _, err := cdp.GoForward(ctx); err != nil {
		return "", fmt.Errorf("cannot go forward: %w", err)
	}

	currentURL, _ := cdp.GetCurrentURL(ctx)
	title, _ := cdp.GetPageTitle(ctx)

	return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_701), currentURL, title), nil
}

// browserCloseTool closes the browser and cleans up.
// It disconnects the CDP WebSocket but keeps the Chrome process running.
// Do NOT Stop() the Chrome process here — doing so would cause the next
// browser tool call to start a completely new Chrome instance (producing
// an unwanted new blank window). Instead, just close the WebSocket;
// EnsurePageConnected will detect the dead connection, close it, and
// create a new tab via /json/new to reconnect.
// Also do NOT set browserEnabled=false — that removes all browser tools
// from the tool list, making it impossible for the LLM to continue.
func (a *Agent) browserCloseTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if a.chromeMgr == nil {
		return i18n.T(i18n.KeySettingCmd_702), nil
	}

	// Close the CDP WebSocket connection to disconnect from the current page.
	// The Chrome process stays alive so the next tool call can reconnect.
	if client := a.chromeMgr.Client(); client != nil {
		client.Close()
	}

	log.Info("Browser CDP connection closed by tool call (Chrome process kept alive)")
	return i18n.T(i18n.KeySettingCmd_703), nil
}

// getBrowserScreenshotData returns and clears the cached screenshot data.
func (a *Agent) getBrowserScreenshotData() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	data := a.browserScreenshotData
	a.browserScreenshotData = ""
	return data
}

// hasBrowserScreenshot checks if screenshot data is cached.
func (a *Agent) hasBrowserScreenshot() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.browserScreenshotData != ""
}
