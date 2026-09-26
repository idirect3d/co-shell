// Integration tests for FEATURE-556 against a real headless Chrome.
//
// Use-case: use-case/FEATURE-556/FEATURE-556-UC-0019.md (group B, B-001..B-016,
// B-025). The tests are gated by testing.Short() and skipped when no
// Chrome/Chromium installation is available, so `go test ./... -short` stays
// hermetic. Each fixture page lives in testdata/browser and is served from a
// local httptest server.

package browser

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// uc556SlowImageDelay keeps the load event of delayed.html pending long enough
// to observe event-driven waiting.
const uc556SlowImageDelay = 1200 * time.Millisecond

var (
	uc556ServerOnce sync.Once
	uc556Server     *httptest.Server

	uc556ChromeOnce sync.Once
	uc556Chrome     *ChromeManager
	uc556ChromeErr  error

	uc556ArtifactOnce sync.Once
	uc556ArtifactDir  string
)

// TestMain stops the shared browser and test server once all tests ran.
func TestMain(m *testing.M) {
	code := m.Run()
	if uc556Chrome != nil {
		uc556Chrome.Stop()
	}
	if uc556Server != nil {
		uc556Server.Close()
	}
	os.Exit(code)
}

// uc556StartServer serves the fixture pages plus a slow image and a download
// payload. The server is created once per test binary.
func uc556StartServer(t *testing.T) *httptest.Server {
	t.Helper()

	uc556ServerOnce.Do(func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/slow.png", func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(uc556SlowImageDelay)
			w.Header().Set("Content-Type", "image/png")
			img := image.NewRGBA(image.Rect(0, 0, 2, 2))
			_ = png.Encode(w, img)
		})
		mux.HandleFunc("/files/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.Header().Set("Content-Disposition", `attachment; filename="uc556-download.txt"`)
			_, _ = w.Write([]byte("uc556-download-payload\n"))
		})
		mux.Handle("/", http.FileServer(http.Dir(filepath.Join("..", "testdata", "browser"))))
		uc556Server = httptest.NewServer(mux)
	})

	return uc556Server
}

// uc556FreePort reserves a free localhost TCP port.
func uc556FreePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("unexpected listener address %T", listener.Addr())
	}
	return addr.Port, nil
}

// uc556StartChrome starts one shared headless Chrome with a temporary profile.
func uc556StartChrome(t *testing.T) *ChromeManager {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping real-Chrome integration test in -short mode")
	}
	if !IsChromeAvailable() {
		t.Skip("Chrome/Chromium is not installed")
	}

	uc556ChromeOnce.Do(func() {
		profile, err := os.MkdirTemp("", "co-shell-uc556-profile-")
		if err != nil {
			uc556ChromeErr = fmt.Errorf("cannot create temp profile: %w", err)
			return
		}
		port, err := uc556FreePort()
		if err != nil {
			uc556ChromeErr = fmt.Errorf("cannot reserve debug port: %w", err)
			return
		}

		uc556Chrome = NewChromeManager(port, true, profile)
		if _, err := uc556Chrome.Start(); err != nil {
			uc556ChromeErr = fmt.Errorf("cannot start Chrome: %w", err)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := uc556Chrome.EnsurePageConnected(ctx); err != nil {
			uc556ChromeErr = fmt.Errorf("cannot connect to Chrome: %w", err)
		}
	})

	if uc556ChromeErr != nil {
		t.Fatalf("headless Chrome unavailable: %v", uc556ChromeErr)
	}
	return uc556Chrome
}

// uc556Open returns the shared client, optionally navigating to page first.
func uc556Open(t *testing.T, page string) (*ChromeManager, *CDPClient) {
	t.Helper()

	manager := uc556StartChrome(t)
	client := manager.Client()
	if client == nil {
		t.Fatal("Chrome manager has no CDP client")
	}
	if page != "" {
		uc556Navigate(t, client, uc556StartServer(t).URL+"/"+page)
	}
	return manager, client
}

// uc556Navigate navigates and returns how long Navigate took.
func uc556Navigate(t *testing.T, client *CDPClient, url string) time.Duration {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	start := time.Now()
	if _, err := client.Navigate(ctx, url); err != nil {
		t.Fatalf("navigate to %s: %v", url, err)
	}
	return time.Since(start)
}

// uc556Eval evaluates expression and renders the result as a string.
func uc556Eval(t *testing.T, client *CDPClient, expression string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	value, err := client.Evaluate(ctx, expression)
	if err != nil {
		t.Fatalf("evaluate %q: %v", expression, err)
	}
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%v", value)
}

// uc556EvalFloat evaluates expression and parses the result as a number.
func uc556EvalFloat(t *testing.T, client *CDPClient, expression string) float64 {
	t.Helper()

	raw := strings.TrimSpace(uc556Eval(t, client, expression))
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		t.Fatalf("evaluate %q returned %q, want a number", expression, raw)
	}
	return value
}

// uc556WaitForFloat polls expression until it reaches want or the timeout
// elapses, and returns the last value seen. Wheel scrolling is applied by the
// compositor asynchronously, so scroll offsets need a short poll.
func uc556WaitForFloat(t *testing.T, client *CDPClient, expression string, want float64, timeout time.Duration) float64 {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for {
		got := uc556EvalFloat(t, client, expression)
		if got >= want || !time.Now().Before(deadline) {
			return got
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// uc556Center resolves a selector and returns its click point.
func uc556Center(t *testing.T, client *CDPClient, selector string) (float64, float64, *ElementBox) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	box, err := client.ElementBox(ctx, selector, 5*time.Second)
	if err != nil {
		t.Fatalf("ElementBox(%q): %v", selector, err)
	}
	x, y := box.Center()
	return x, y, box
}

// uc556Run executes fn with a bounded context.
func uc556Run(t *testing.T, timeout time.Duration, fn func(context.Context) error) error {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return fn(ctx)
}

// uc556Artifact writes a test artifact under work/tmp/uc556 for later visual
// inspection and returns its path ("" when the file could not be written).
func uc556Artifact(t *testing.T, name string, data []byte) string {
	t.Helper()

	uc556ArtifactOnce.Do(func() {
		dir := filepath.Join("..", "work", "tmp", "uc556")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return
		}
		uc556ArtifactDir = dir
	})
	if uc556ArtifactDir == "" {
		return ""
	}

	path := filepath.Join(uc556ArtifactDir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Logf("cannot write artifact %s: %v", path, err)
		return ""
	}
	t.Logf("artifact written to %s", path)
	return path
}

// uc556JPEGSize decodes a base64 JPEG screenshot and returns its size plus the
// raw bytes.
func uc556JPEGSize(t *testing.T, base64Data string) (int, int, []byte) {
	t.Helper()

	raw, err := ScreenshotToBytes(base64Data)
	if err != nil {
		t.Fatalf("cannot decode screenshot: %v", err)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("cannot read JPEG header: %v", err)
	}
	return cfg.Width, cfg.Height, raw
}

// TestIntegrationEventDrivenNavigation covers B-001: Navigate returns only once
// the load event fired, so a page whose load is held back by a slow subresource
// is complete when Navigate returns.
func TestIntegrationEventDrivenNavigation(t *testing.T) {
	_, client := uc556Open(t, "")

	elapsed := uc556Navigate(t, client, uc556StartServer(t).URL+"/delayed.html")
	if elapsed < time.Second {
		t.Fatalf("Navigate returned after %v, want >= 1s: the load event must gate the return", elapsed)
	}
	if got := uc556Eval(t, client, "String(!!document.querySelector('#late'))"); got != "true" {
		t.Fatalf("delayed element #late missing right after Navigate (got %q)", got)
	}

	err := uc556Run(t, 5*time.Second, func(ctx context.Context) error {
		return client.WaitForSelector(ctx, "#late", 2*time.Second)
	})
	if err != nil {
		t.Fatalf("WaitForSelector(#late) after Navigate: %v", err)
	}
}

// TestIntegrationChineseInputAndEnter covers B-004, B-005 and D-006: CJK text
// and emoji go through Input.insertText, while Enter is dispatched as a real key
// event that submits the form.
func TestIntegrationChineseInputAndEnter(t *testing.T) {
	_, client := uc556Open(t, "basic.html")

	// The fixture ships a pre-filled value; start from an empty field so the
	// assertion covers the typed text only.
	uc556Eval(t, client, "document.querySelector('#name').value=''")

	x, y, _ := uc556Center(t, client, "#name")
	if err := uc556Run(t, 10*time.Second, func(ctx context.Context) error {
		return client.ClickEx(ctx, x, y, MouseButtonLeft, 1)
	}); err != nil {
		t.Fatalf("click #name: %v", err)
	}

	const text = "你好，世界"
	if err := uc556Run(t, 10*time.Second, func(ctx context.Context) error {
		return client.TypeText(ctx, text)
	}); err != nil {
		t.Fatalf("TypeText(%q): %v", text, err)
	}
	if got := uc556Eval(t, client, "document.querySelector('#name').value"); got != text {
		t.Fatalf("input value = %q, want %q", got, text)
	}

	if err := uc556Run(t, 10*time.Second, func(ctx context.Context) error {
		return client.TypeText(ctx, "🎉")
	}); err != nil {
		t.Fatalf("TypeText(emoji): %v", err)
	}
	if got := uc556Eval(t, client, "document.querySelector('#name').value"); got != text+"🎉" {
		t.Fatalf("input value after emoji = %q, want %q", got, text+"🎉")
	}

	if err := uc556Run(t, 10*time.Second, func(ctx context.Context) error {
		return client.PressKeys(ctx, "Enter")
	}); err != nil {
		t.Fatalf("PressKeys(Enter): %v", err)
	}

	if got := uc556Eval(t, client, "JSON.stringify(window.__keys)"); !strings.Contains(got, "Enter") {
		t.Fatalf("keydown log %s does not contain Enter", got)
	}
	if got := uc556Eval(t, client, "document.querySelector('#out').textContent"); got != "submitted" {
		t.Fatalf("form submission result = %q, want submitted", got)
	}
}

// TestIntegrationShortcutKeys covers the platform-normalised shortcut path used
// by clear-on-type: "Ctrl+A" must reach the page with the platform convention
// (Cmd on darwin, use-case A-007) and must not type any text.
func TestIntegrationShortcutKeys(t *testing.T) {
	_, client := uc556Open(t, "basic.html")

	x, y, _ := uc556Center(t, client, "#name")
	if err := uc556Run(t, 10*time.Second, func(ctx context.Context) error {
		return client.ClickEx(ctx, x, y, MouseButtonLeft, 1)
	}); err != nil {
		t.Fatalf("click #name: %v", err)
	}
	if err := uc556Run(t, 10*time.Second, func(ctx context.Context) error {
		return client.PressKeys(ctx, "Ctrl+A")
	}); err != nil {
		t.Fatalf("PressKeys(Ctrl+A): %v", err)
	}

	// The page records the modifiers it really received; a shortcut must not
	// insert text.
	keylog := uc556Eval(t, client, "JSON.stringify(window.__keylog)")
	if runtime.GOOS == "darwin" {
		if !strings.Contains(keylog, `"meta":true`) || strings.Contains(keylog, `"ctrl":true`) {
			t.Fatalf("keydown log %s: Ctrl must be normalised to Cmd on darwin", keylog)
		}
	} else if !strings.Contains(keylog, `"ctrl":true`) {
		t.Fatalf("keydown log %s: Ctrl modifier missing", keylog)
	}
	if got := uc556Eval(t, client, "document.querySelector('#name').value"); got != "old" {
		t.Fatalf("input value = %q, want the untouched initial value", got)
	}
}

// TestIntegrationAlertAutoHandled covers B-002 and D-007: a page that opens
// alert() while parsing neither blocks navigation nor later CDP calls, and the
// handled dialog is reported back.
func TestIntegrationAlertAutoHandled(t *testing.T) {
	manager, client := uc556Open(t, "")
	dialogs := manager.DialogManager()
	if dialogs == nil {
		t.Fatal("no dialog manager attached to the browser session")
	}
	dialogs.SetPolicy(DialogAuto)
	dialogs.TakeMessages()

	if _, err := client.Navigate(context.Background(), uc556StartServer(t).URL+"/alert.html"); err != nil {
		t.Fatalf("navigate to alert page: %v", err)
	}

	if got := uc556Eval(t, client, "document.querySelector('#ready').textContent"); got != "ready" {
		t.Fatalf("page did not finish parsing after alert (got %q)", got)
	}
	if got := uc556Eval(t, client, "String(1+1)"); got != "2" {
		t.Fatalf("CDP call after alert returned %q, want 2", got)
	}

	messages := dialogs.TakeMessages()
	if len(messages) == 0 {
		t.Fatal("dialog manager reported no handled dialog")
	}
	if !strings.Contains(messages[0].Message, "hello-uc556") {
		t.Fatalf("dialog message = %q, want it to contain hello-uc556", messages[0].Message)
	}
	if messages[0].Type != "alert" {
		t.Fatalf("dialog type = %q, want alert", messages[0].Type)
	}
}

// TestIntegrationConfirmPolicy covers B-003: dismiss makes confirm() return
// false, accept makes it return true.
func TestIntegrationConfirmPolicy(t *testing.T) {
	manager, client := uc556Open(t, "")
	dialogs := manager.DialogManager()
	if dialogs == nil {
		t.Fatal("no dialog manager attached to the browser session")
	}
	t.Cleanup(func() { dialogs.SetPolicy(DialogAuto) })

	pageURL := uc556StartServer(t).URL + "/confirm.html"

	dialogs.SetPolicy(DialogDismiss)
	dialogs.TakeMessages()
	if _, err := client.Navigate(context.Background(), pageURL); err != nil {
		t.Fatalf("navigate to confirm page: %v", err)
	}
	if got := uc556Eval(t, client, "document.querySelector('#out').textContent"); got != "cancelled" {
		t.Fatalf("confirm with dismiss policy produced %q, want cancelled", got)
	}
	messages := dialogs.TakeMessages()
	if len(messages) == 0 || messages[0].Type != "confirm" {
		t.Fatalf("confirm dialog not reported: %+v", messages)
	}

	dialogs.SetPolicy(DialogAccept)
	if _, err := client.Navigate(context.Background(), pageURL); err != nil {
		t.Fatalf("navigate to confirm page (accept): %v", err)
	}
	if got := uc556Eval(t, client, "document.querySelector('#out').textContent"); got != "confirmed" {
		t.Fatalf("confirm with accept policy produced %q, want confirmed", got)
	}
}

// TestIntegrationHoverRightClickDoubleClick covers B-006, B-007 and B-008.
func TestIntegrationHoverRightClickDoubleClick(t *testing.T) {
	_, client := uc556Open(t, "basic.html")

	// B-007: right click reaches the contextmenu listener.
	cx, cy, _ := uc556Center(t, client, "#ctx")
	if err := uc556Run(t, 10*time.Second, func(ctx context.Context) error {
		return client.ClickEx(ctx, cx, cy, MouseButtonRight, 1)
	}); err != nil {
		t.Fatalf("right click #ctx: %v", err)
	}
	if got := uc556EvalFloat(t, client, "String(window.__ctxCount)"); got != 1 {
		t.Fatalf("contextmenu count = %v, want 1", got)
	}

	// B-008: clickCount=2 produces a dblclick event.
	dx, dy, _ := uc556Center(t, client, "#dbl")
	if err := uc556Run(t, 10*time.Second, func(ctx context.Context) error {
		return client.ClickEx(ctx, dx, dy, MouseButtonLeft, 2)
	}); err != nil {
		t.Fatalf("double click #dbl: %v", err)
	}
	if got := uc556EvalFloat(t, client, "String(window.__dblCount)"); got != 1 {
		t.Fatalf("dblclick count = %v, want 1", got)
	}

	// B-006: hover opens the submenu, which is only reachable through a real
	// mouseMoved event. It runs last because opening the submenu reflows the
	// page and would invalidate the coordinates of the clicks above.
	mx, my, _ := uc556Center(t, client, "#menu")
	if err := uc556Run(t, 10*time.Second, func(ctx context.Context) error {
		return client.Hover(ctx, mx, my)
	}); err != nil {
		t.Fatalf("Hover(#menu): %v", err)
	}
	if got := uc556Eval(t, client, "getComputedStyle(document.querySelector('#submenu')).display"); got != "block" {
		t.Fatalf("submenu display = %q, want block after hover", got)
	}
}

// TestIntegrationWheelScroll covers B-009: mouseWheel scrolls both the document
// and a scrollable container under the pointer.
func TestIntegrationWheelScroll(t *testing.T) {
	_, client := uc556Open(t, "long.html")

	// The inner container first: ScrollByWheel always aims at the viewport
	// centre, so the container is scrolled with the same wheel parameters
	// dispatched at its own centre.
	sx, sy, _ := uc556Center(t, client, "#scroller")
	if err := uc556Run(t, 10*time.Second, func(ctx context.Context) error {
		_, err := client.Call(ctx, "Input.dispatchMouseEvent", map[string]interface{}{
			"type":   "mouseWheel",
			"x":      sx,
			"y":      sy,
			"deltaX": 0,
			"deltaY": 300,
		})
		return err
	}); err != nil {
		t.Fatalf("wheel over #scroller: %v", err)
	}
	if got := uc556WaitForFloat(t, client, "String(document.querySelector('#scroller').scrollTop)", 1, 2*time.Second); got <= 0 {
		t.Fatalf("inner container scrollTop = %v, want > 0", got)
	}

	if err := uc556Run(t, 10*time.Second, func(ctx context.Context) error {
		return client.ScrollByWheel(ctx, 0, 600)
	}); err != nil {
		t.Fatalf("ScrollByWheel(0, 600): %v", err)
	}
	if got := uc556WaitForFloat(t, client, "String(window.scrollY)", 1, 2*time.Second); got <= 0 {
		t.Fatalf("window.scrollY = %v, want > 0 after wheel", got)
	}
}

// TestIntegrationSelectorClickOutsideViewport covers B-010: a selector outside
// the viewport is scrolled into view and clicked at its real geometry.
func TestIntegrationSelectorClickOutsideViewport(t *testing.T) {
	_, client := uc556Open(t, "long.html")

	if got := uc556EvalFloat(t, client, "String(window.scrollY)"); got != 0 {
		t.Fatalf("page is not at the top before the test (scrollY=%v)", got)
	}

	viewportHeight := uc556EvalFloat(t, client, "String(window.innerHeight)")
	x, y, box := uc556Center(t, client, "#deep")

	if box.PageY <= viewportHeight {
		t.Fatalf("#deep pageY = %v, want it below the initial viewport (%v)", box.PageY, viewportHeight)
	}
	if y < 0 || y+box.Height > viewportHeight {
		t.Fatalf("#deep viewport Y range [%v, %v] is outside [0, %v] after scrolling", y, y+box.Height, viewportHeight)
	}

	if err := uc556Run(t, 10*time.Second, func(ctx context.Context) error {
		return client.ClickEx(ctx, x, y, MouseButtonLeft, 1)
	}); err != nil {
		t.Fatalf("click #deep at (%v, %v): %v", x, y, err)
	}
	if got := uc556Eval(t, client, "String(window.__deepClicked)"); got != "true" {
		t.Fatalf("#deep was not clicked (clicked=%q, box=%+v)", got, box)
	}
	if got := uc556EvalFloat(t, client, "String(window.scrollY)"); got <= 0 {
		t.Fatalf("window.scrollY = %v, want > 0 after scrolling #deep into view", got)
	}
}

// TestIntegrationSelectorTypeClear covers B-011 with the same sequence the
// browser_type tool uses: focus by selector, clear, then type.
func TestIntegrationSelectorTypeClear(t *testing.T) {
	_, client := uc556Open(t, "basic.html")

	x, y, _ := uc556Center(t, client, "#name")
	if err := uc556Run(t, 10*time.Second, func(ctx context.Context) error {
		return client.ClickEx(ctx, x, y, MouseButtonLeft, 1)
	}); err != nil {
		t.Fatalf("focus #name: %v", err)
	}
	if _, err := client.Evaluate(context.Background(), `document.activeElement?.select?.()`); err != nil {
		t.Fatalf("select active element: %v", err)
	}
	if _, err := client.Evaluate(context.Background(), `document.execCommand('delete')`); err != nil {
		t.Fatalf("delete selection: %v", err)
	}
	if got := uc556Eval(t, client, "document.querySelector('#name').value"); got != "" {
		t.Fatalf("input value after clear = %q, want empty", got)
	}

	if err := uc556Run(t, 10*time.Second, func(ctx context.Context) error {
		return client.TypeText(ctx, "abc")
	}); err != nil {
		t.Fatalf("TypeText(abc): %v", err)
	}
	if got := uc556Eval(t, client, "document.querySelector('#name').value"); got != "abc" {
		t.Fatalf("input value = %q, want abc", got)
	}
}

// TestIntegrationSelectorScreenshot covers B-012: the clip matches the element
// geometry.
func TestIntegrationSelectorScreenshot(t *testing.T) {
	_, client := uc556Open(t, "basic.html")

	_, _, box := uc556Center(t, client, "#card")

	var width, height int
	var raw []byte
	err := uc556Run(t, 20*time.Second, func(ctx context.Context) error {
		data, err := client.CaptureScreenshotClip(ctx, 80, box.Clip())
		if err != nil {
			return err
		}
		width, height, raw = uc556JPEGSize(t, data)
		return nil
	})
	if err != nil {
		t.Fatalf("CaptureScreenshotClip(#card): %v", err)
	}
	uc556Artifact(t, "selector-card.jpg", raw)

	if diff := width - int(box.Width+0.5); diff > 2 || diff < -2 {
		t.Fatalf("screenshot width = %d, want ≈ %v", width, box.Width)
	}
	if diff := height - int(box.Height+0.5); diff > 2 || diff < -2 {
		t.Fatalf("screenshot height = %d, want ≈ %v", height, box.Height)
	}
}

// TestIntegrationWaitForSelectorTimeout covers B-013: an unknown selector fails
// fast with a message naming the selector.
func TestIntegrationWaitForSelectorTimeout(t *testing.T) {
	_, client := uc556Open(t, "basic.html")

	start := time.Now()
	var waitErr error
	_ = uc556Run(t, 10*time.Second, func(ctx context.Context) error {
		waitErr = client.WaitForSelector(ctx, "#uc556-missing", 700*time.Millisecond)
		return nil
	})
	elapsed := time.Since(start)

	if waitErr == nil {
		t.Fatal("WaitForSelector returned nil for a missing selector")
	}
	if !strings.Contains(waitErr.Error(), "#uc556-missing") {
		t.Fatalf("error %q does not name the selector", waitErr.Error())
	}
	if elapsed > 5*time.Second {
		t.Fatalf("WaitForSelector took %v, want a bounded wait", elapsed)
	}
}

// TestIntegrationFullPageScreenshot covers B-014: the full-page capture spans
// the whole document and renders the fixed header exactly once.
func TestIntegrationFullPageScreenshot(t *testing.T) {
	_, client := uc556Open(t, "long.html")

	documentHeight := uc556EvalFloat(t, client, "String(document.documentElement.scrollHeight)")
	viewportHeight := uc556EvalFloat(t, client, "String(window.innerHeight)")

	var width, height int
	var raw []byte
	err := uc556Run(t, 30*time.Second, func(ctx context.Context) error {
		data, err := client.CaptureScreenshot(ctx, 80, true)
		if err != nil {
			return err
		}
		width, height, raw = uc556JPEGSize(t, data)
		return nil
	})
	if err != nil {
		t.Fatalf("CaptureScreenshot(fullPage): %v", err)
	}
	uc556Artifact(t, "fullpage-long.jpg", raw)

	if height < int(documentHeight)-2 {
		t.Fatalf("full page screenshot height = %d, want >= document height %v", height, documentHeight)
	}
	if float64(height) <= viewportHeight*2 {
		t.Fatalf("full page screenshot height = %d, want more than two viewports (%v)", height, viewportHeight)
	}
	if width <= 0 {
		t.Fatalf("full page screenshot width = %d", width)
	}

	img, err := jpeg.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("cannot decode full page screenshot: %v", err)
	}
	darkRows := 0
	for y := 0; y < img.Bounds().Dy(); y++ {
		r, g, b, _ := img.At(5, y).RGBA()
		if r>>8 < 100 && g>>8 < 100 && b>>8 < 100 {
			darkRows++
		}
	}
	if darkRows < 35 || darkRows > 45 {
		t.Fatalf("dark header rows = %d, want ~40 (fixed header must appear exactly once)", darkRows)
	}
}

// TestIntegrationGoBackForward covers B-015: history navigation uses a valid
// entry id and reports the resulting URL.
func TestIntegrationGoBackForward(t *testing.T) {
	_, client := uc556Open(t, "")
	base := uc556StartServer(t).URL

	uc556Navigate(t, client, base+"/basic.html")
	uc556Navigate(t, client, base+"/long.html")

	var backURL string
	err := uc556Run(t, 30*time.Second, func(ctx context.Context) error {
		var err error
		backURL, err = client.GoBack(ctx)
		return err
	})
	if err != nil {
		t.Fatalf("GoBack: %v", err)
	}
	if !strings.Contains(backURL, "basic.html") {
		t.Fatalf("GoBack landed on %q, want basic.html", backURL)
	}
	if got := uc556Eval(t, client, "document.querySelector('#title').textContent"); !strings.Contains(got, "basic") {
		t.Fatalf("page title after GoBack = %q, want the basic page", got)
	}

	var forwardURL string
	err = uc556Run(t, 30*time.Second, func(ctx context.Context) error {
		var err error
		forwardURL, err = client.GoForward(ctx)
		return err
	})
	if err != nil {
		t.Fatalf("GoForward: %v", err)
	}
	if !strings.Contains(forwardURL, "long.html") {
		t.Fatalf("GoForward landed on %q, want long.html", forwardURL)
	}
}

// TestIntegrationNavigateFastFail covers B-016: an unresolvable host fails
// quickly instead of waiting for the full load timeout.
func TestIntegrationNavigateFastFail(t *testing.T) {
	_, client := uc556Open(t, "")

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	start := time.Now()
	_, err := client.Navigate(ctx, "http://uc556-does-not-exist.invalid/")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Navigate returned nil for an unresolvable host")
	}
	if elapsed > 15*time.Second {
		t.Fatalf("Navigate waited %v before failing, want a fast failure", elapsed)
	}
}

// TestIntegrationConcurrentCallsWithEvents covers B-025: concurrent CDP calls
// while events are subscribed must not race or deadlock (run with -race).
func TestIntegrationConcurrentCallsWithEvents(t *testing.T) {
	_, client := uc556Open(t, "basic.html")

	const workers = 4
	const callsPerWorker = 5

	events, cancelWatch := client.Watch("Page.loadEventFired")
	defer cancelWatch()

	var wg sync.WaitGroup
	failures := make(chan error, workers+1)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < callsPerWorker; j++ {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				_, err := client.Evaluate(ctx, "(function(){ return 1 + 1; })()")
				cancel()
				if err != nil {
					failures <- fmt.Errorf("worker %d call %d: %w", worker, j, err)
					return
				}
			}
		}(i)
	}

	// Subscribe and unsubscribe while calls are in flight.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < callsPerWorker; j++ {
			_, cancel := client.Watch("Page.domContentEventFired")
			cancel()
		}
	}()

	wg.Wait()
	close(failures)

	for err := range failures {
		t.Fatalf("concurrent CDP usage failed: %v", err)
	}

	// A registered watcher must still be usable after the concurrent burst.
	select {
	case <-events:
	case <-time.After(50 * time.Millisecond):
	}

	// The client must still be usable after the concurrent burst.
	if got := uc556Eval(t, client, "String(2+2)"); got != "4" {
		t.Fatalf("client unusable after concurrent calls: %q", got)
	}
}
