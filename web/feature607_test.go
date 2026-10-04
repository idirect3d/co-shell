package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FEATURE-607: the hub reverse proxy injects the real client address into the
// X-Client-IP header, so a loopback-bound co-shell can tell a remote client
// (mobile / LAN, reaching it through the hub) apart from a browser opened on
// the same machine. Download is offered to remote clients only, and only when
// the operator enabled it with --download-enabled.

// newXClientServer starts a server with the given bind/download settings.
func newXClientServer(t *testing.T, root, bind string, download bool) *httptest.Server {
	t.Helper()
	s := NewServer(root, ServerOptions{Lang: "zh", Version: "0", Build: "0", Bind: bind, DownloadEnabled: download})
	ts := httptest.NewServer(s.mux)
	t.Cleanup(ts.Close)
	return ts
}

func doWithClientIP(t *testing.T, method, url, clientIP string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if clientIP != "" {
		req.Header.Set("X-Client-IP", clientIP)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp
}

// TestIsRemoteClientXHeader covers the header parsing: only a well-formed,
// non-loopback address counts as a remote client.
func TestIsRemoteClientXHeader(t *testing.T) {
	cases := []struct {
		name   string
		header string
		want   bool
	}{
		{"absent", "", false},
		{"lan ip with port", "192.168.3.39:51000", true},
		{"bare lan ip", "10.0.0.9", true},
		{"loopback with port", "127.0.0.1:52345", false},
		{"bare loopback", "127.0.0.1", false},
		{"ipv6 loopback", "[::1]:52345", false},
		{"ipv6 remote", "[fe80::1]:52345", true},
		{"garbage", "not-an-ip", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewServer(t.TempDir(), ServerOptions{Lang: "zh", Version: "0", Build: "0", Bind: "127.0.0.1"})
			r := httptest.NewRequest(http.MethodGet, "/api/download", nil)
			if tc.header != "" {
				r.Header.Set("X-Client-IP", tc.header)
			}
			if got := s.isRemoteClient(r); got != tc.want {
				t.Errorf("isRemoteClient(%q) = %v, want %v", tc.header, got, tc.want)
			}
		})
	}
}

// TestDownloadXClientIP verifies the download gate for a loopback-bound server:
// direct local access and the hub-forwarded local browser are refused, while a
// remote client marked by the hub is served the file.
func TestDownloadXClientIP(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	ts := newXClientServer(t, root, "127.0.0.1", true)

	cases := []struct {
		name     string
		clientIP string
		want     int
	}{
		{"local direct (no header)", "", http.StatusForbidden},
		{"local browser via hub", "127.0.0.1:52345", http.StatusForbidden},
		{"ipv6 loopback via hub", "[::1]:52345", http.StatusForbidden},
		{"unparsable header", "not-an-ip", http.StatusForbidden},
		{"remote client via hub", "192.168.3.39:51000", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := doWithClientIP(t, http.MethodGet, ts.URL+"/api/download?path=hello.txt", tc.clientIP)
			defer resp.Body.Close()
			if resp.StatusCode != tc.want {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.want)
			}
			if tc.want != http.StatusOK {
				return
			}
			if cd := resp.Header.Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment") {
				t.Errorf("Content-Disposition = %q, want attachment", cd)
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != "hi" {
				t.Errorf("body = %q, want hi", body)
			}
		})
	}

	// The master switch still wins: with --download-enabled off, even a remote
	// client is refused.
	tsOff := newXClientServer(t, root, "127.0.0.1", false)
	respOff := doWithClientIP(t, http.MethodGet, tsOff.URL+"/api/download?path=hello.txt", "192.168.3.39:51000")
	respOff.Body.Close()
	if respOff.StatusCode != http.StatusForbidden {
		t.Errorf("remote client with switch off = %d, want 403", respOff.StatusCode)
	}

	// A remote bind keeps working without any header (direct remote access).
	tsRemoteBind := newXClientServer(t, root, "0.0.0.0", true)
	respRB := doWithClientIP(t, http.MethodGet, tsRemoteBind.URL+"/api/download?path=hello.txt", "")
	respRB.Body.Close()
	if respRB.StatusCode != http.StatusOK {
		t.Errorf("remote bind without header = %d, want 200", respRB.StatusCode)
	}

	// Path traversal and directories stay rejected for remote clients too.
	respTrav := doWithClientIP(t, http.MethodGet, ts.URL+"/api/download?path=../../etc/passwd", "192.168.3.39:51000")
	respTrav.Body.Close()
	if respTrav.StatusCode != http.StatusForbidden {
		t.Errorf("traversal = %d, want 403", respTrav.StatusCode)
	}
	respDir := doWithClientIP(t, http.MethodGet, ts.URL+"/api/download?path=sub", "192.168.3.39:51000")
	respDir.Body.Close()
	if respDir.StatusCode != http.StatusNotFound {
		t.Errorf("directory = %d, want 404", respDir.StatusCode)
	}
}

// TestBootstrapXClientIP verifies the flags the frontend reads: a request from
// a non-loopback client (as stamped by the hub) reports remote+downloadable,
// while local access reports neither.
func TestBootstrapXClientIP(t *testing.T) {
	root := t.TempDir()
	ts := newXClientServer(t, root, "127.0.0.1", true)

	cases := []struct {
		name           string
		clientIP       string
		wantRemote     bool
		wantDownloadOn bool
	}{
		{"local direct", "", false, false},
		{"local browser via hub", "127.0.0.1:52345", false, false},
		{"remote client via hub", "192.168.3.39:51000", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := doWithClientIP(t, http.MethodGet, ts.URL+"/api/bootstrap", tc.clientIP)
			defer resp.Body.Close()
			var b map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&b); err != nil {
				t.Fatalf("decode bootstrap: %v", err)
			}
			if b["remote"] != tc.wantRemote {
				t.Errorf("bootstrap remote = %v, want %v", b["remote"], tc.wantRemote)
			}
			if b["downloadEnabled"] != tc.wantDownloadOn {
				t.Errorf("bootstrap downloadEnabled = %v, want %v", b["downloadEnabled"], tc.wantDownloadOn)
			}
		})
	}

	// Remote access with the switch off: still remote, but not downloadable.
	tsOff := newXClientServer(t, root, "127.0.0.1", false)
	resp := doWithClientIP(t, http.MethodGet, tsOff.URL+"/api/bootstrap", "192.168.3.39:51000")
	defer resp.Body.Close()
	var b map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&b); err != nil {
		t.Fatal(err)
	}
	if b["remote"] != true || b["downloadEnabled"] != false {
		t.Errorf("switch off: remote=%v downloadEnabled=%v, want true/false", b["remote"], b["downloadEnabled"])
	}
}

// TestOpenRevealXClientIP verifies the OS-local open/reveal actions are refused
// for a client the hub marked as non-loopback.
func TestOpenRevealXClientIP(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}
	ts := newXClientServer(t, root, "127.0.0.1", true)
	for _, api := range []string{"/api/open", "/api/reveal"} {
		resp := doWithClientIP(t, http.MethodPost, ts.URL+api, "192.168.3.39:51000")
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("POST %s from remote client = %d, want 403", api, resp.StatusCode)
		}
	}
}
