// Shared fixtures for the FEATURE-556 Phase 3 integration tests: fixture pages
// served by an inline httptest server plus the helpers that start a dedicated
// headless Chrome and poll for asynchronous results.
//
// Use-case: use-case/FEATURE-556/FEATURE-556-UC-0020.md (group B).

package browser

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const (
	uc556P3ConsolePage = `<!doctype html>
<html><head><meta charset="utf-8"><title>uc556-console</title></head>
<body>
<img src="/missing-404.png" alt="">
<pre id="out">loading</pre>
<script>
console.log("p3-log");
console.info("p3-info");
console.warn("p3-warn");
console.error("p3-error");
console.log("p3-long-" + new Array(301).join("x"));
setTimeout(function () { throw new Error("p3-uncaught"); }, 0);
document.getElementById("out").textContent = "ready";
</script>
</body></html>`

	uc556P3UploadPage = `<!doctype html>
<html><head><meta charset="utf-8"><title>uc556-upload</title></head>
<body>
<input id="files" type="file" multiple>
<pre id="out">ready</pre>
</body></html>`

	uc556P3PlainPage = `<!doctype html>
<html><head><meta charset="utf-8"><title>uc556-plain</title></head>
<body><pre id="out">plain</pre></body></html>`

	uc556P3NetworkPage = `<!doctype html>
<html><head><meta charset="utf-8"><title>uc556-network</title></head>
<body>
<img id="ok" src="/real.png" alt="">
<img id="gone" src="/missing-404.png" alt="">
<img id="bad" src="http://127.0.0.1:%d/unreachable.png" alt="">
<pre id="out">ready</pre>
</body></html>`

	// uc556P3MockPage takes an asset query value so every navigation requests a
	// distinct image URL: a cached image would never be paused by Fetch, and the
	// mocking assertions would silently pass without a mock.
	uc556P3MockPage = `<!doctype html>
<html><head><meta charset="utf-8"><title>uc556-mock</title></head>
<body><img id="pic" src="/real.png?asset=%s" alt=""><pre id="out">ready</pre></body></html>`
)

// uc556P3Fixtures serves the inline Phase 3 fixture pages.
type uc556P3Fixtures struct {
	server  *httptest.Server
	realPNG []byte // 3x3 image served from /real.png
}

// uc556P3Session starts a dedicated headless Chrome for one Phase 3 test. A
// fresh browser per test keeps collectors, tabs and mock rules isolated.
func uc556P3Session(t *testing.T) (*ChromeManager, *CDPClient) {
	t.Helper()
	return uc556PolicySession(t, "auto")
}

// uc556P3PNG renders a size x size PNG image.
func uc556P3PNG(t *testing.T, size int) []byte {
	t.Helper()

	var buffer bytes.Buffer
	if err := png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, size, size))); err != nil {
		t.Fatalf("encode %dx%d png: %v", size, size, err)
	}
	return buffer.Bytes()
}

// uc556P3StartServer serves the Phase 3 fixture pages plus a real image. Any
// other path (missing-404.png, …) falls through to the 404 handler.
func uc556P3StartServer(t *testing.T) *uc556P3Fixtures {
	t.Helper()

	fixtures := &uc556P3Fixtures{realPNG: uc556P3PNG(t, 3)}
	deadPort, err := uc556FreePort()
	if err != nil {
		t.Fatalf("cannot reserve dead port: %v", err)
	}

	writePage := func(w http.ResponseWriter, page string) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, page)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/real.png", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(fixtures.realPNG)
	})
	mux.HandleFunc("/console.html", func(w http.ResponseWriter, _ *http.Request) {
		writePage(w, uc556P3ConsolePage)
	})
	mux.HandleFunc("/upload.html", func(w http.ResponseWriter, _ *http.Request) {
		writePage(w, uc556P3UploadPage)
	})
	mux.HandleFunc("/plain.html", func(w http.ResponseWriter, _ *http.Request) {
		writePage(w, uc556P3PlainPage)
	})
	mux.HandleFunc("/network.html", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, uc556P3NetworkPage, deadPort)
	})
	mux.HandleFunc("/mock.html", func(w http.ResponseWriter, r *http.Request) {
		asset := r.URL.Query().Get("asset")
		if asset == "" {
			asset = "default"
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, uc556P3MockPage, asset)
	})

	fixtures.server = httptest.NewServer(mux)
	t.Cleanup(fixtures.server.Close)
	return fixtures
}

// uc556P3Eventually polls check until it succeeds or the timeout elapses.
func uc556P3Eventually(t *testing.T, timeout time.Duration, what string, check func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for {
		if check() {
			return
		}
		if !time.Now().Before(deadline) {
			t.Fatalf("timed out after %s waiting for %s", timeout, what)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
