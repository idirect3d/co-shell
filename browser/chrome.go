// Author: L.Shuang
// Created: 2026-06-04
// Last Modified: 2026-06-09
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

package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"

	"github.com/idirect3d/co-shell/log"
)

// ChromeManager manages a Chrome/Chromium browser process lifecycle
// and provides CDP WebSocket connection endpoints.
type ChromeManager struct {
	cmd         *exec.Cmd
	port        int
	headless    bool
	cdpClient   *CDPClient
	started     bool
	userDataDir string // persistent user data directory
	dialogs     *DialogManager

	// FEATURE-556 Phase 3: per-session collectors, created on demand by the
	// first tool call that needs them.
	console *ConsoleManager
	network *NetworkCollector
	mock    *FetchMock

	// cfg holds the FEATURE-556 launch and session settings.
	cfg ChromeConfig
}

// NewChromeManager creates a new ChromeManager with the default configuration.
// userDataDir is the directory for Chrome's user data storage.
// If empty, a temporary directory under /tmp is used.
func NewChromeManager(port int, headless bool, userDataDir ...string) *ChromeManager {
	cfg := DefaultChromeConfig()
	if len(userDataDir) > 0 && userDataDir[0] != "" {
		cfg.ProfileDir = userDataDir[0]
	}
	return NewChromeManagerWithConfig(port, headless, cfg)
}

// NewChromeManagerWithConfig creates a new ChromeManager from an explicit
// browser configuration (FEATURE-556 Phase 2).
func NewChromeManagerWithConfig(port int, headless bool, cfg ChromeConfig) *ChromeManager {
	if port <= 0 {
		port = 9222
	}
	return &ChromeManager{
		port:     port,
		headless: headless,
		cfg:      cfg,
	}
}

// Start launches a Chrome/Chromium browser instance with remote debugging enabled.
// It searches for Chrome or Chromium in common installation paths.
// Returns the debugging URL (e.g., "http://localhost:9222").
func (m *ChromeManager) Start() (string, error) {
	if m.started {
		return fmt.Sprintf("http://localhost:%d", m.port), nil
	}

	chromePath := m.cfg.ChromePath
	if chromePath == "" {
		chromePath = findChromePath()
	}
	if chromePath == "" {
		return "", fmt.Errorf("cannot find Chrome/Chromium installation")
	}

	// Use a persistent user data directory when persistence is enabled, so
	// browser state (cookies, sessions) survives co-shell restarts; otherwise
	// a fresh temporary directory keeps every run clean (FEATURE-556).
	userDataDir := m.resolveUserDataDir()
	os.MkdirAll(userDataDir, 0755)
	m.userDataDir = userDataDir

	args := BuildChromeArgs(m.cfg, m.port, m.headless, userDataDir)

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

	return m.setClient(client), nil
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

	debugURL := fmt.Sprintf("http://localhost:%d", m.port)

	// Step 1: Try to get WebSocket URL from existing pages first
	wsURL, err := getFirstPageWebSocketURL(debugURL)
	if err == nil && wsURL != "" {
		client, err := NewCDPClient(wsURL)
		if err == nil {
			return m.setClient(client), nil
		}
		log.Debug("Cannot connect to existing page: %v, trying new tab", err)
	}

	// Step 2: Try creating a new tab via /json/new
	client, err := m.createNewPage(debugURL)
	if err == nil {
		return client, nil
	}

	// Step 3: Last fallback — try /json again (Chrome may have created a default page)
	log.Debug("Cannot create new page: %v, retrying /json", err)
	wsURL, err = getFirstPageWebSocketURL(debugURL)
	if err != nil || wsURL == "" {
		return nil, fmt.Errorf("cannot get any page from Chrome after all attempts")
	}
	client2, err := NewCDPClient(wsURL)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to CDP via WebSocket: %w", err)
	}
	return m.setClient(client2), nil
}

// createNewPage creates a new page/tab via Chrome's /json/new HTTP API.
func (m *ChromeManager) createNewPage(debugURL string) (*CDPClient, error) {
	newPageURL := debugURL + "/json/new"
	resp, err := http.Get(newPageURL)
	if err != nil {
		return nil, fmt.Errorf("cannot create new page: %w", err)
	}
	defer resp.Body.Close()

	// Check HTTP status code — Chrome may return 500 or an HTML error page
	// if the debugging endpoint is temporarily unavailable.
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create new page returned HTTP %d: %s", resp.StatusCode, truncateString(string(body), 200))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read new page response: %w", err)
	}

	var pageInfo struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.Unmarshal(body, &pageInfo); err != nil {
		return nil, fmt.Errorf("cannot decode new page info (HTTP %d, body=%q): %w", resp.StatusCode, truncateString(string(body), 200), err)
	}

	if pageInfo.WebSocketDebuggerURL == "" {
		return nil, fmt.Errorf("new page has no WebSocket debugger URL")
	}

	client, err := NewCDPClient(pageInfo.WebSocketDebuggerURL)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to new page: %w", err)
	}

	return m.setClient(client), nil
}

// Stop terminates the Chrome browser process.
func (m *ChromeManager) Stop() {
	if m.dialogs != nil {
		m.dialogs.Close()
		m.dialogs = nil
	}

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

// setClient stores a freshly connected CDP client and prepares its session:
// Page domain events are enabled and JavaScript dialogs are answered
// automatically.
func (m *ChromeManager) setClient(client *CDPClient) *CDPClient {
	m.cdpClient = client
	m.attachSession(client)
	return client
}

// attachSession enables Page events, (re)starts dialog handling and replays the
// configured session-level settings for a connection. It never fails: a missing
// Page domain only degrades event driven waiting, which falls back to
// readyState polling.
func (m *ChromeManager) attachSession(client *CDPClient) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.EnablePageEvents(ctx); err != nil {
		log.Debug("cannot enable Page events: %v", err)
	}

	if m.dialogs != nil {
		m.dialogs.Close()
	}
	policy, ok := ParseDialogPolicy(m.cfg.DialogPolicy)
	if !ok {
		log.Debug("unknown dialog policy %q, falling back to %q", m.cfg.DialogPolicy, policy)
	}
	m.dialogs = NewDialogManager(client, policy)

	// FEATURE-556 Phase 3: the collectors belong to the previous connection, so
	// they are stopped here and recreated on demand for this session.
	m.resetCollectors()

	// FEATURE-556: replay the configured session-level settings (user agent,
	// headers, viewport, locale, timezone, TLS exceptions, download behaviour)
	// so a reconnect or a co-shell restart keeps the configured browser state.
	// Nothing is sent when no such setting is configured.
	if m.cfg.ApplyOnConnect {
		applied, err := ApplySessionSettings(ctx, client, m.cfg)
		if err != nil {
			log.Warn("cannot apply browser session settings (applied=%v): %v", applied, err)
		} else if len(applied) > 0 {
			log.Info("Browser session settings applied on connect: %v", applied)
		}
	}
}

// resolveUserDataDir returns Chrome's user data directory: the configured
// profile when persistence is on, otherwise a fresh temporary directory so that
// every run starts from a clean state (FEATURE-556 Phase 2).
func (m *ChromeManager) resolveUserDataDir() string {
	if m.cfg.Persist && m.cfg.ProfileDir != "" {
		return m.cfg.ProfileDir
	}
	return fmt.Sprintf("/tmp/co-shell-chrome-%d", time.Now().UnixNano())
}

// DialogManager returns the dialog handler of the current session, or nil.
func (m *ChromeManager) DialogManager() *DialogManager {
	return m.dialogs
}

// TakeDialogMessages returns the JavaScript dialogs answered so far for this
// browser session (and clears them).
func (m *ChromeManager) TakeDialogMessages() []DialogInfo {
	if m.dialogs == nil {
		return nil
	}
	return m.dialogs.TakeMessages()
}

// EnsureConsoleCapture returns the console collector of this session, creating
// and enabling it on first use. Console capture stays off until a tool asks for
// it, so ordinary browsing sends no Runtime.enable/Log.enable call.
func (m *ChromeManager) EnsureConsoleCapture(ctx context.Context) (*ConsoleManager, error) {
	if m.cdpClient == nil {
		return nil, fmt.Errorf("browser is not connected")
	}
	if m.console != nil {
		return m.console, nil
	}
	manager, err := NewConsoleManager(ctx, m.cdpClient)
	m.console = manager
	return manager, err
}

// ConsoleManager returns the console collector of this session, or nil when
// capture was never started.
func (m *ChromeManager) ConsoleManager() *ConsoleManager {
	return m.console
}

// EnsureNetworkCollector returns the network collector of this session,
// creating and enabling it on first use. CDP does not replay network events, so
// this must run before the traffic of interest.
func (m *ChromeManager) EnsureNetworkCollector(ctx context.Context) (*NetworkCollector, error) {
	if m.cdpClient == nil {
		return nil, fmt.Errorf("browser is not connected")
	}
	if m.network != nil {
		return m.network, nil
	}
	collector, err := NewNetworkCollector(ctx, m.cdpClient)
	m.network = collector
	return collector, err
}

// NetworkCollector returns the network collector of this session, or nil.
func (m *ChromeManager) NetworkCollector() *NetworkCollector {
	return m.network
}

// Mock returns the request mocker of this session, creating it on first use.
// Nothing is intercepted until a rule is added.
func (m *ChromeManager) Mock() (*FetchMock, error) {
	if m.cdpClient == nil {
		return nil, fmt.Errorf("browser is not connected")
	}
	if m.mock == nil {
		m.mock = NewFetchMock(m.cdpClient)
	}
	return m.mock, nil
}

// resetCollectors stops the per-session collectors; it is called whenever the
// CDP connection changes.
func (m *ChromeManager) resetCollectors() {
	if m.console != nil {
		m.console.Close()
		m.console = nil
	}
	if m.network != nil {
		m.network.Close()
		m.network = nil
	}
	if m.mock != nil {
		m.mock.Close()
		m.mock = nil
	}
}

// DebugURL returns Chrome's HTTP debug endpoint base URL.
func (m *ChromeManager) DebugURL() string {
	return fmt.Sprintf("http://localhost:%d", m.port)
}

// PageTargets returns the page targets of the browser, enriched with the
// WebSocket debugger URL needed to switch to them.
func (m *ChromeManager) PageTargets(ctx context.Context) ([]TargetInfo, error) {
	if m.cdpClient == nil {
		return nil, fmt.Errorf("browser is not connected")
	}
	pages, err := ListPageTargets(ctx, m.cdpClient)
	if err != nil {
		return nil, err
	}

	debugTargets, debugErr := DebugTargets(m.DebugURL())
	if debugErr != nil {
		// Listing still works without the HTTP endpoint; only switching needs it.
		log.Debug("cannot read Chrome debug endpoint: %v", debugErr)
		return pages, nil
	}
	return AttachDebugURLs(pages, debugTargets), nil
}

// SelectTarget reconnects the session to another page target: a page session is
// bound to exactly one page, so switching means closing the current connection
// and connecting to the target's own WebSocket debugger URL.
func (m *ChromeManager) SelectTarget(wsURL string) error {
	if wsURL == "" {
		return fmt.Errorf("target has no webSocketDebuggerUrl")
	}
	client, err := NewCDPClient(wsURL)
	if err != nil {
		return fmt.Errorf("cannot connect to target: %w", err)
	}

	previous := m.cdpClient
	m.setClient(client)
	if previous != nil {
		previous.Close()
	}
	return nil
}

// DropConnection closes the current CDP connection without stopping Chrome, so
// the next browser tool call reconnects to a live page. It is used after
// closing the page the session was attached to.
func (m *ChromeManager) DropConnection() {
	if m == nil {
		return
	}
	if m.cdpClient != nil {
		m.cdpClient.Close()
		m.cdpClient = nil
	}
	m.resetCollectors()
	if m.dialogs != nil {
		m.dialogs.Close()
		m.dialogs = nil
	}
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
// It filters out Chrome extension background pages and prefers real tabs.
// If no real tab exists, it creates a new one via /json/new.
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
		Type                 string `json:"type"`
	}

	if err := json.Unmarshal(body, &pages); err != nil {
		return "", fmt.Errorf("cannot parse page list: %w", err)
	}

	if len(pages) == 0 {
		// No pages found, create one via /json/new
		body, err = createNewPageViaHTTP(debugURL)
		if err != nil {
			return "", err
		}
		var newPage struct {
			WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
		}
		if err := json.Unmarshal(body, &newPage); err != nil {
			return "", fmt.Errorf("cannot decode new page info: %w", err)
		}
		return newPage.WebSocketDebuggerURL, nil
	}

	// Prefer a real page tab (not a Chrome extension background page).
	// Extension background pages are not navigable — connecting to them
	// causes all navigation/screenshot calls to operate on the extension
	// rather than the user's intended page.
	var fallback string
	for _, p := range pages {
		if p.WebSocketDebuggerURL == "" {
			continue
		}
		// Skip extension background pages
		if isExtensionURL(p.URL) {
			continue
		}
		// Prefer "page" type over "background_page" or "other"
		if p.Type == "page" {
			return p.WebSocketDebuggerURL, nil
		}
		// Save as fallback
		if fallback == "" {
			fallback = p.WebSocketDebuggerURL
		}
	}

	// Fallback: use the first non-extension WebSocket URL
	if fallback != "" {
		return fallback, nil
	}

	// All pages are extension pages — create a new real tab
	body, err = createNewPageViaHTTP(debugURL)
	if err != nil {
		return "", err
	}
	var newPage struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.Unmarshal(body, &newPage); err != nil {
		return "", fmt.Errorf("cannot decode new page info: %w", err)
	}
	if newPage.WebSocketDebuggerURL == "" {
		return "", fmt.Errorf("new page has no WebSocket debugger URL")
	}
	return newPage.WebSocketDebuggerURL, nil
}

// isExtensionURL checks if a URL is a Chrome extension URL.
func isExtensionURL(url string) bool {
	return len(url) >= 22 && url[:22] == "chrome-extension://"
}

// createNewPageViaHTTP calls Chrome's /json/new API and returns the raw response body.
func createNewPageViaHTTP(debugURL string) ([]byte, error) {
	newResp, err := http.Get(debugURL + "/json/new")
	if err != nil {
		return nil, fmt.Errorf("cannot create new page: %w", err)
	}
	defer newResp.Body.Close()

	body, err := io.ReadAll(newResp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read new page response: %w", err)
	}
	return body, nil
}

// truncateString truncates a string to the given maximum length,
// appending "..." if it was truncated.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// SetStarted marks the ChromeManager as started without actually launching Chrome.
// This is used when reusing an already-running Chrome instance detected on the
// configured remote debugging port. After calling SetStarted(), subsequent
// Start() calls will return immediately without spawning a new process.
func (m *ChromeManager) SetStarted() {
	m.started = true
}

// IsEndpointAvailable checks if a Chrome DevTools Protocol endpoint is already
// responding on the given URL. This is used to detect an existing Chrome instance
// before attempting to start a new one.
func IsEndpointAvailable(debugURL string) bool {
	resp, err := http.Get(debugURL + "/json/version")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

// IsChromeAvailable checks if Chrome/Chromium is installed on the system.
func IsChromeAvailable() bool {
	return findChromePath() != ""
}
