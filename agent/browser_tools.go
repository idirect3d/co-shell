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
	"time"

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

	return fmt.Sprintf("已导航到页面:\nURL: %s\n标题: %s\n\n现在你可以使用 browser_screenshot 查看页面内容，或使用 browser_get_interactive_elements 查看可交互元素。", currentURL, title), nil
}

// browserScreenshotTool captures a screenshot, saves it to ./screenshot/,
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

	screenshotData, err := cdp.CaptureScreenshot(ctx, quality, fullPage)
	if err != nil {
		return "", fmt.Errorf("cannot capture screenshot: %w", err)
	}

	currentURL, _ := cdp.GetCurrentURL(ctx)
	title, _ := cdp.GetPageTitle(ctx)

	// Decode base64 and save to ./screenshot/
	screenshotBytes, err := base64.StdEncoding.DecodeString(screenshotData)
	if err != nil {
		return "", fmt.Errorf("cannot decode screenshot data: %w", err)
	}

	ts := time.Now().Format("20060102_150405")
	screenshotPath := filepath.Join(".", "screenshot", fmt.Sprintf("browser_screenshot_%s.jpg", ts))
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

	log.Info("Browser screenshot saved to %s (quality=%d, fullPage=%v, size=%d bytes)", screenshotPath, quality, fullPage, len(screenshotBytes))

	// Check vision support before auto-loading to image cache
	visionSupported := false
	if a.cfg != nil {
		visionSupported = a.cfg.LLM.VisionSupport
	}

	baseMsg := fmt.Sprintf("页面截图已保存到: %s\nURL: %s\n标题: %s\n截图质量: %d\n全页截图: %v\n",
		screenshotPath, currentURL, title, quality, fullPage)

	if visionSupported {
		addResult, _ := a.AddImages(screenshotPath)
		baseMsg += "\n" + addResult
		baseMsg += "\n\n截图已加载到图片缓存，后续请求将发送到视觉模型进行分析。你可以结合 browser_get_interactive_elements 获取页面可交互元素信息来进行精确操作。"
	} else {
		baseMsg += "\n\n⚠️ **当前模型不支持视觉识别**，无法对截图内容进行分析。\n截图已保存到文件系统中，你可以：\n1. 通过 `.set vision on` 启用多模态支持（需模型支持）\n2. 切换到支持视觉的多模态大模型后再试\n3. 手动使用 add_images 工具加载截图"
	}

	return baseMsg, nil
}

// browserClickTool clicks at the specified coordinates.
func (a *Agent) browserClickTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := a.ensureBrowserReady(ctx); err != nil {
		return "", err
	}

	cdp := a.chromeMgr.Client()

	x, okX := args["x"].(float64)
	y, okY := args["y"].(float64)
	if !okX || !okY {
		return "", fmt.Errorf("x and y coordinates are required")
	}

	if err := cdp.Click(ctx, x, y); err != nil {
		return "", fmt.Errorf("cannot click at (%f, %f): %w", x, y, err)
	}

	log.Info("Browser click at (%f, %f)", x, y)
	currentURL, _ := cdp.GetCurrentURL(ctx)

	return fmt.Sprintf("已在坐标 (%.0f, %.0f) 处执行点击。\n当前URL: %s\n\n请使用 browser_screenshot 查看点击后的页面变化。", x, y, currentURL), nil
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

	log.Info("Browser type: %q (clear=%v)", text, clear)
	return fmt.Sprintf("已输入文本: %s", text), nil
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
		return "JavaScript 执行成功，无返回值", nil
	}
	return fmt.Sprintf("JavaScript 执行结果:\n%s", result), nil
}

// browserGetHTMLTool returns the page's HTML.
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
	return fmt.Sprintf("页面 HTML（%d 个字符）:\n%s", len(html), html), nil
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
	direction := "向下"
	if deltaY < 0 {
		direction = "向上"
	}
	return fmt.Sprintf("已%s滚动 %.0f 像素。请使用 browser_screenshot 查看滚动后的页面内容。", direction, deltaY), nil
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

	return fmt.Sprintf("页面可交互元素信息:\n%s\n\n你可以根据这些元素的位置坐标（centerX, centerY）使用 browser_click 工具进行点击，或使用 browser_type 工具输入文本。如果页面布局发生变化，可以重新调用此工具获取最新信息。", elementsJSON), nil
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

	return fmt.Sprintf("已返回上一页。\n当前URL: %s\n标题: %s", currentURL, title), nil
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

	return fmt.Sprintf("已前进到下一页。\n当前URL: %s\n标题: %s", currentURL, title), nil
}

// browserCloseTool closes the browser and cleans up.
func (a *Agent) browserCloseTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if a.chromeMgr == nil {
		return "浏览器未启动", nil
	}

	a.chromeMgr.Stop()
	a.chromeMgr = nil
	a.browserEnabled = false

	log.Info("Browser closed by tool call")
	return "浏览器已关闭。", nil
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
