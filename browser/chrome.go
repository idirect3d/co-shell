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
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"syscall"
	"time"

	"github.com/idirect3d/co-shell/log"
)

// ChromeManager manages a Chrome/Chromium browser process lifecycle
// and provides CDP WebSocket connection endpoints.
type ChromeManager struct {
	cmd       *exec.Cmd
	port      int
	headless  bool
	cdpClient *CDPClient
	started   bool
}

// NewChromeManager creates a new ChromeManager.
func NewChromeManager(port int, headless bool) *ChromeManager {
	if port <= 0 {
		port = 9222
	}
	return &ChromeManager{
		port:     port,
		headless: headless,
	}
}

// Start launches a Chrome/Chromium browser instance with remote debugging enabled.
// It searches for Chrome or Chromium in common installation paths.
// Returns the debugging URL (e.g., "http://localhost:9222").
func (m *ChromeManager) Start() (string, error) {
	if m.started {
		return fmt.Sprintf("http://localhost:%d", m.port), nil
	}

	chromePath := findChromePath()
	if chromePath == "" {
		return "", fmt.Errorf("cannot find Chrome/Chromium installation")
	}

	args := []string{
		fmt.Sprintf("--remote-debugging-port=%d", m.port),
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
	}

	if m.headless {
		args = append(args, "--headless=new")
	}

	// Use a temporary user data directory to avoid polluting the user's profile
	args = append(args, fmt.Sprintf("--user-data-dir=/tmp/co-shell-chrome-%d", time.Now().Unix()))

	args = append(args, "about:blank")

	m.cmd = exec.Command(chromePath, args...)

	// Discard stdout/stderr to avoid filling up buffers
	m.cmd.Stdout = io.Discard
	m.cmd.Stderr = io.Discard

	if err := m.cmd.Start(); err != nil {
		return "", fmt.Errorf("cannot start Chrome: %w", err)
	}

	m.started = true

	// Wait for Chrome to start listening
	debugURL := fmt.Sprintf("http://localhost:%d", m.port)
	if err := waitForEndpoint(debugURL, 15*time.Second); err != nil {
		m.Stop()
		return "", fmt.Errorf("Chrome started but not responding: %w", err)
	}

	log.Info("Chrome browser started (headless=%v, port=%d)", m.headless, m.port)

	return debugURL, nil
}

// Connect connects to an already running Chrome instance via CDP.
// Returns the CDPClient for the first available page/tab.
func (m *ChromeManager) Connect() (*CDPClient, error) {
	if m.cdpClient != nil {
		return m.cdpClient, nil
	}

	debugURL := fmt.Sprintf("http://localhost:%d", m.port)

	// Get the list of pages/tabs from the browser
	wsURL, err := getFirstPageWebSocketURL(debugURL)
	if err != nil {
		return nil, fmt.Errorf("cannot get page list from Chrome: %w", err)
	}

	client, err := NewCDPClient(wsURL)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to CDP: %w", err)
	}

	m.cdpClient = client
	return client, nil
}

// EnsurePageConnected ensures we have a valid CDP connection with an active page.
// If the current connection is dead, it creates a new tab and reconnects.
func (m *ChromeManager) EnsurePageConnected(ctx context.Context) (*CDPClient, error) {
	// Test current connection
	if m.cdpClient != nil {
		_, err := m.cdpClient.GetCurrentURL(ctx)
		if err == nil {
			return m.cdpClient, nil
		}
		log.Debug("CDP connection lost, reconnecting: %v", err)
		m.cdpClient.Close()
		m.cdpClient = nil
	}

	// Create a new page/tab
	debugURL := fmt.Sprintf("http://localhost:%d", m.port)
	newPageURL := debugURL + "/json/new"

	resp, err := http.Get(newPageURL)
	if err != nil {
		return nil, fmt.Errorf("cannot create new page: %w", err)
	}
	defer resp.Body.Close()

	var pageInfo struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pageInfo); err != nil {
		return nil, fmt.Errorf("cannot decode new page info: %w", err)
	}

	client, err := NewCDPClient(pageInfo.WebSocketDebuggerURL)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to new page: %w", err)
	}

	m.cdpClient = client
	return client, nil
}

// Stop terminates the Chrome browser process.
func (m *ChromeManager) Stop() {
	if m.cdpClient != nil {
		m.cdpClient.Close()
		m.cdpClient = nil
	}

	if m.cmd != nil && m.started {
		// Try to terminate gracefully
		m.cmd.Process.Signal(terminateSignal())
		done := make(chan struct{}, 1)
		go func() {
			m.cmd.Wait()
			close(done)
		}()

		select {
		case <-done:
			log.Info("Chrome browser process terminated")
		case <-time.After(5 * time.Second):
			m.cmd.Process.Kill()
			log.Warn("Chrome browser process killed (graceful shutdown timeout)")
		}

		m.started = false
		m.cmd = nil
	}
}

// IsRunning returns whether the browser process is running.
func (m *ChromeManager) IsRunning() bool {
	return m.started && m.cmd != nil && m.cmd.Process != nil
}

// Client returns the current CDP client, or nil if not connected.
func (m *ChromeManager) Client() *CDPClient {
	return m.cdpClient
}

// Port returns the debugging port.
func (m *ChromeManager) Port() int {
	return m.port
}

// findChromePath searches for Chrome or Chromium in common installation locations.
func findChromePath() string {
	paths := getChromePaths()
	for _, path := range paths {
		if pathExists(path) {
			return path
		}
	}
	return ""
}

// getChromePaths returns common Chrome/Chromium installation paths per OS.
func getChromePaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"google-chrome",
			"chromium",
			"chrome",
		}
	case "linux":
		return []string{
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/snap/bin/chromium",
			"google-chrome",
			"chromium",
			"chromium-browser",
		}
	case "windows":
		return []string{
			"C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe",
			"C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe",
			"C:\\Program Files\\Chromium\\Application\\chrome.exe",
			"chrome",
		}
	default:
		return []string{"google-chrome", "chromium", "chrome"}
	}
}

// pathExists checks if a file exists at the given path.
func pathExists(path string) bool {
	// If it's a simple command name (no path separators), assume it exists in PATH
	if path == "google-chrome" || path == "chromium" || path == "chromium-browser" || path == "chrome" {
		_, err := exec.LookPath(path)
		return err == nil
	}
	cmd := exec.Command("test", "-f", path)
	return cmd.Run() == nil
}

// terminateSignal returns the appropriate signal for graceful process termination on the current OS.
func terminateSignal() syscall.Signal {
	if runtime.GOOS == "windows" {
		return syscall.SIGKILL
	}
	return syscall.SIGTERM
}

// waitForEndpoint polls the given HTTP endpoint until it responds or times out.
func waitForEndpoint(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url + "/json/version")
		if err == nil {
			resp.Body.Close()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("endpoint %s did not respond within %v", url, timeout)
}

// getFirstPageWebSocketURL retrieves the WebSocket URL for the first available page tab.
func getFirstPageWebSocketURL(debugURL string) (string, error) {
	resp, err := http.Get(debugURL + "/json")
	if err != nil {
		return "", fmt.Errorf("cannot fetch page list: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("cannot read page list: %w", err)
	}

	var pages []struct {
		ID                   string `json:"id"`
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
		URL                  string `json:"url"`
	}

	if err := json.Unmarshal(body, &pages); err != nil {
		return "", fmt.Errorf("cannot parse page list: %w", err)
	}

	if len(pages) == 0 {
		// No pages found, create one via /json/new
		newResp, err := http.Get(debugURL + "/json/new")
		if err != nil {
			return "", fmt.Errorf("cannot create new page: %w", err)
		}
		defer newResp.Body.Close()

		var newPage struct {
			WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
		}
		if err := json.NewDecoder(newResp.Body).Decode(&newPage); err != nil {
			return "", fmt.Errorf("cannot decode new page: %w", err)
		}
		return newPage.WebSocketDebuggerURL, nil
	}

	// Return the first page's WebSocket URL (prefer "about:blank" or first non-blank)
	for _, p := range pages {
		if p.WebSocketDebuggerURL != "" {
			return p.WebSocketDebuggerURL, nil
		}
	}

	return pages[0].WebSocketDebuggerURL, nil
}

// IsChromeAvailable checks if Chrome/Chromium is installed on the system.
func IsChromeAvailable() bool {
	return findChromePath() != ""
}
