// Package web - HTTP API tests (FEATURE-307c): workspace tree exclusions,
// upload save + traversal rejection, open/reveal path validation with
// injected fake launchers, and file read boundaries.
//
// Author: L.Shuang
// Created: 2026-08-17
// Last Modified: 2026-08-17
// MIT License - Copyright (c) 2026 L.Shuang

package web

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestServer builds a Server over a temp workspace pre-populated with a
// regular file, an output/ file and the excluded noise directories.
func newTestServer(t *testing.T) (*Server, *httptest.Server, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{".git", "node_modules", "db", "log", "tmp", "output"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "output", "result.md"), []byte("out"), 0644); err != nil {
		t.Fatal(err)
	}
	s := NewServer(root, ServerOptions{Lang: "zh", Version: "0.0.0", Build: "0"})
	ts := httptest.NewServer(s.mux)
	t.Cleanup(ts.Close)
	return s, ts, root
}

func getJSON(t *testing.T, url string, out interface{}) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("decode %s: %v", url, err)
	}
}

// TestTreeExclusions verifies the excluded noise directories are hidden and
// output/ is kept.
func TestTreeExclusions(t *testing.T) {
	_, ts, _ := newTestServer(t)
	var root treeNode
	getJSON(t, ts.URL+"/api/tree", &root)
	names := map[string]bool{}
	for _, c := range root.Children {
		names[c.Name] = c.Dir
	}
	for _, excluded := range []string{".git", "node_modules", "db", "log", "tmp"} {
		if _, ok := names[excluded]; ok {
			t.Errorf("tree contains excluded entry %q", excluded)
		}
	}
	if isDir, ok := names["output"]; !ok || !isDir {
		t.Errorf("tree must keep the output/ directory")
	}
	if _, ok := names["hello.txt"]; !ok {
		t.Errorf("tree missing hello.txt")
	}
}

// uploadFiles posts one multipart file to /api/upload and returns the
// response status and decoded body.
func uploadFiles(t *testing.T, url, field, filename, content string) (int, map[string]interface{}) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile(field, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(url, mw.FormDataContentType(), &buf)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	var body map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body
}

// TestUploadSavesFile verifies an upload lands in the target directory and
// the returned path is workspace-relative.
func TestUploadSavesFile(t *testing.T) {
	_, ts, root := newTestServer(t)
	status, body := uploadFiles(t, ts.URL+"/api/upload?dir=output", "file", "up.txt", "payload")
	if status != http.StatusOK {
		t.Fatalf("upload status = %d, body = %v", status, body)
	}
	data, err := os.ReadFile(filepath.Join(root, "output", "up.txt"))
	if err != nil || string(data) != "payload" {
		t.Errorf("uploaded file content = %q, err = %v", data, err)
	}
	paths, _ := body["paths"].([]interface{})
	if len(paths) != 1 || paths[0] != "output/up.txt" {
		t.Errorf("returned paths = %v, want [output/up.txt]", paths)
	}
}

// TestUploadTraversalRejected verifies a ../ target directory is refused.
func TestUploadTraversalRejected(t *testing.T) {
	_, ts, _ := newTestServer(t)
	status, _ := uploadFiles(t, ts.URL+"/api/upload?dir=../outside", "file", "evil.txt", "x")
	if status != http.StatusForbidden {
		t.Errorf("upload to ../ status = %d, want 403", status)
	}
}

// TestUploadAutoCreatesDir verifies a missing target directory (e.g. the
// message attachment dir "input") is auto-created on upload (FEATURE-469).
func TestUploadAutoCreatesDir(t *testing.T) {
	_, ts, root := newTestServer(t)
	status, body := uploadFiles(t, ts.URL+"/api/upload?dir=input", "file", "shot.png", "img")
	if status != http.StatusOK {
		t.Fatalf("upload status = %d, body = %v", status, body)
	}
	data, err := os.ReadFile(filepath.Join(root, "input", "shot.png"))
	if err != nil || string(data) != "img" {
		t.Errorf("uploaded file in auto-created dir: data = %q, err = %v", data, err)
	}
}

// TestOpenRevealPathValidation verifies open/reveal invoke the injected
// launcher for in-workspace paths and reject traversal without invoking it.
func TestOpenRevealPathValidation(t *testing.T) {
	_, ts, root := newTestServer(t)

	var opened, revealed []string
	oldOpen, oldReveal := openFileFunc, revealFileFunc
	openFileFunc = func(p string) error { opened = append(opened, p); return nil }
	revealFileFunc = func(p string) error { revealed = append(revealed, p); return nil }
	t.Cleanup(func() { openFileFunc, revealFileFunc = oldOpen, oldReveal })

	post := func(api, path string) int {
		body := strings.NewReader(`{"path":` + `"` + path + `"` + `}`)
		resp, err := http.Post(ts.URL+api, "application/json", body)
		if err != nil {
			t.Fatalf("POST %s: %v", api, err)
		}
		defer resp.Body.Close()
		io.Copy(io.Discard, resp.Body)
		return resp.StatusCode
	}

	if st := post("/api/open", "hello.txt"); st != http.StatusOK {
		t.Errorf("open hello.txt status = %d, want 200", st)
	}
	if len(opened) != 1 || opened[0] != filepath.Join(root, "hello.txt") {
		t.Errorf("open launcher got %v, want workspace hello.txt", opened)
	}
	if st := post("/api/reveal", "hello.txt"); st != http.StatusOK {
		t.Errorf("reveal hello.txt status = %d, want 200", st)
	}
	if len(revealed) != 1 {
		t.Errorf("reveal launcher got %v, want one call", revealed)
	}

	// Traversal must be rejected and the launcher must not run.
	if st := post("/api/open", "../../etc/passwd"); st != http.StatusForbidden {
		t.Errorf("open ../ status = %d, want 403", st)
	}
	if st := post("/api/reveal", ".."); st != http.StatusForbidden {
		t.Errorf("reveal .. status = %d, want 403", st)
	}
	if len(opened) != 1 || len(revealed) != 1 {
		t.Errorf("launcher invoked for traversal path: opened=%v revealed=%v", opened, revealed)
	}
}

// TestFileRead verifies /api/file serves workspace files and rejects both
// traversal and directories.
func TestFileRead(t *testing.T) {
	_, ts, _ := newTestServer(t)

	resp, err := http.Get(ts.URL + "/api/file?path=output/result.md")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(data) != "out" {
		t.Errorf("file read = (%d, %q), want (200, \"out\")", resp.StatusCode, data)
	}

	for _, path := range []string{"../../etc/passwd", "output"} {
		resp, err := http.Get(ts.URL + "/api/file?path=" + path)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		want := http.StatusForbidden
		if path == "output" {
			want = http.StatusNotFound
		}
		if resp.StatusCode != want {
			t.Errorf("file read %q status = %d, want %d", path, resp.StatusCode, want)
		}
	}
}

// TestBootstrap verifies the bootstrap payload carries the configured lang
// and the current git branch (from .git/HEAD in the workspace root).
func TestBootstrap(t *testing.T) {
	_, ts, root := newTestServer(t)
	// newTestServer already creates a .git dir; write a HEAD so the branch
	// is resolvable.
	if err := os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	var b map[string]interface{}
	getJSON(t, ts.URL+"/api/bootstrap", &b)
	if b["lang"] != "zh" {
		t.Errorf("bootstrap lang = %v, want zh", b["lang"])
	}
	if b["branch"] != "main" {
		t.Errorf("bootstrap branch = %v, want main", b["branch"])
	}
	// FEATURE-455: remote/download flags are booleans in the payload.
	if b["remote"] != false {
		t.Errorf("bootstrap remote = %v, want false (loopback bind)", b["remote"])
	}
	if b["downloadEnabled"] != false {
		t.Errorf("bootstrap downloadEnabled = %v, want false", b["downloadEnabled"])
	}
}

// TestGitBranch verifies branch detection from .git/HEAD: a normal branch
// ref, a missing HEAD (no repo), and a detached HEAD (raw commit hash).
func TestGitBranch(t *testing.T) {
	t.Run("branch ref", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".git"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if got := gitBranch(root); got != "main" {
			t.Errorf("gitBranch = %q, want main", got)
		}
	})
	t.Run("no repo", func(t *testing.T) {
		root := t.TempDir()
		if got := gitBranch(root); got != "" {
			t.Errorf("gitBranch = %q, want empty", got)
		}
	})
	t.Run("detached head", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".git"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("a1b2c3d4e5f6\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if got := gitBranch(root); got != "" {
			t.Errorf("gitBranch = %q, want empty", got)
		}
	})
}

// TestListenBind verifies Listen binds the configured address (FEATURE-430):
// the returned "host:port" reflects the ServerOptions.Bind value, defaulting
// to 127.0.0.1 when empty.
func TestListenBind(t *testing.T) {
	cases := []struct {
		name string
		bind string
		want string
	}{
		{"explicit loopback", "127.0.0.1", "127.0.0.1:"},
		{"all interfaces", "0.0.0.0", "0.0.0.0:"},
		{"default loopback", "", "127.0.0.1:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewServer(t.TempDir(), ServerOptions{Lang: "zh", Version: "0", Build: "0", Bind: tc.bind})
			addr, err := s.Listen(0)
			if err != nil {
				t.Fatalf("Listen(0): %v", err)
			}
			defer s.Close()
			if !strings.HasPrefix(addr, tc.want) {
				t.Errorf("Listen addr = %q, want prefix %q", addr, tc.want)
			}
		})
	}
}

// TestIPAllowed verifies the whitelist IP/CIDR matching logic (FEATURE-431).
func TestIPAllowed(t *testing.T) {
	cases := []struct {
		name   string
		remote string
		list   []string
		want   bool
	}{
		{"exact IP match", "192.168.1.100:1234", []string{"192.168.1.100"}, true},
		{"exact IP no match", "192.168.1.101:1234", []string{"192.168.1.100"}, false},
		{"CIDR subnet match", "192.168.1.50:1234", []string{"192.168.1.0/24"}, true},
		{"CIDR subnet no match", "192.168.2.50:1234", []string{"192.168.1.0/24"}, false},
		{"empty list denies", "127.0.0.1:1234", nil, false},
		{"invalid remote denied", "not-an-addr", []string{"192.168.1.0/24"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nets := parseWhitelist(tc.list)
			if got := ipAllowed(tc.remote, nets); got != tc.want {
				t.Errorf("ipAllowed(%q, %v) = %v, want %v", tc.remote, tc.list, got, tc.want)
			}
		})
	}
}

// TestIsRemote verifies the remote-access detection (FEATURE-455): a bind
// address other than loopback means the UI is served to remote clients.
func TestIsRemote(t *testing.T) {
	cases := []struct {
		name string
		bind string
		want bool
	}{
		{"default loopback", "127.0.0.1", false},
		{"localhost", "localhost", false},
		{"ipv6 loopback", "::1", false},
		{"empty", "", false},
		{"all interfaces", "0.0.0.0", true},
		{"lan ip", "192.168.1.5", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewServer(t.TempDir(), ServerOptions{Lang: "zh", Version: "0", Build: "0", Bind: tc.bind})
			if got := s.isRemote(); got != tc.want {
				t.Errorf("isRemote(bind=%q) = %v, want %v", tc.bind, got, tc.want)
			}
		})
	}
}

// TestDownload verifies the /api/download endpoint (FEATURE-455): it is only
// available when the UI is served to a remote address AND download is enabled;
// otherwise it returns 403. Path traversal and directories are rejected.
func TestDownload(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0755); err != nil {
		t.Fatal(err)
	}

	// Local bind + download disabled: always 403.
	sLocal := NewServer(root, ServerOptions{Lang: "zh", Version: "0", Build: "0", Bind: "127.0.0.1"})
	tsLocal := httptest.NewServer(sLocal.mux)
	defer tsLocal.Close()
	if st := getStatus(t, tsLocal.URL+"/api/download?path=hello.txt"); st != http.StatusForbidden {
		t.Errorf("local+disabled status = %d, want 403", st)
	}

	// Remote bind + download disabled: still 403 (feature not exposed).
	sRemoteOff := NewServer(root, ServerOptions{Lang: "zh", Version: "0", Build: "0", Bind: "0.0.0.0"})
	tsRemoteOff := httptest.NewServer(sRemoteOff.mux)
	defer tsRemoteOff.Close()
	if st := getStatus(t, tsRemoteOff.URL+"/api/download?path=hello.txt"); st != http.StatusForbidden {
		t.Errorf("remote+disabled status = %d, want 403", st)
	}

	// Remote bind + download enabled: normal file downloads.
	sRemote := NewServer(root, ServerOptions{Lang: "zh", Version: "0", Build: "0", Bind: "0.0.0.0", DownloadEnabled: true})
	tsRemote := httptest.NewServer(sRemote.mux)
	defer tsRemote.Close()

	resp, err := http.Get(tsRemote.URL + "/api/download?path=hello.txt")
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("download status = %d, want 200", resp.StatusCode)
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment") {
		t.Errorf("Content-Disposition = %q, want attachment", cd)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hi" {
		t.Errorf("download body = %q, want hi", body)
	}

	// Traversal must be rejected.
	if st := getStatus(t, tsRemote.URL+"/api/download?path=../../etc/passwd"); st != http.StatusForbidden {
		t.Errorf("download traversal status = %d, want 403", st)
	}

	// Directories must be rejected.
	if st := getStatus(t, tsRemote.URL+"/api/download?path=sub"); st != http.StatusNotFound {
		t.Errorf("download dir status = %d, want 404", st)
	}

	// FEATURE-455: on remote access, the OS-local open/reveal actions are
	// disabled (they would act on the server's local apps/file manager).
	post := func(api string) int {
		resp, err := http.Post(tsRemote.URL+api, "application/json", strings.NewReader(`{"path":"hello.txt"}`))
		if err != nil {
			t.Fatalf("POST %s: %v", api, err)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}
	if st := post("/api/open"); st != http.StatusForbidden {
		t.Errorf("remote open status = %d, want 403", st)
	}
	if st := post("/api/reveal"); st != http.StatusForbidden {
		t.Errorf("remote reveal status = %d, want 403", st)
	}
}

// getStatus performs a GET and returns only the status code.
func getStatus(t *testing.T, url string) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}
