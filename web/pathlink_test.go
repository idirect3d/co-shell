// Package web - FEATURE-576 tests: a local file path found in the conversation
// stream (LLM body, tool result, tool arguments) renders as a clickable link
// that opens the same preview panel the workspace tree uses; the read-only
// preview API stays confined to the workspace.
//
// Author: L.Shuang
// Created: 2026-10-02
// MIT License - Copyright (c) 2026 L.Shuang

package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// uiAsset fetches a UI asset through the static handler, trying the layouts the
// server may serve it under, so the assertions below run against what the
// browser actually receives.
func uiAsset(t *testing.T, names ...string) string {
	t.Helper()
	h := serveStatic()
	for _, n := range names {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, n, nil))
		if rec.Code == http.StatusOK {
			return rec.Body.String()
		}
	}
	t.Fatalf("static asset not served: %v", names)
	return ""
}

// TestPathLinksInMdRenderer verifies the markdown renderer turns supported local
// paths into links and that every text outlet goes through the same builder
// (so LLM body, tool result and tool arguments are all covered: they all render
// through mdRender).
func TestPathLinksInMdRenderer(t *testing.T) {
	src := uiAsset(t, "/md.js", "/static/md.js", "md.js")

	link := funcBody(src, "function pathLinkNodes(text) {")
	if link == "" {
		t.Fatal("md.js: pathLinkNodes is missing")
	}
	for _, want := range []string{
		`a.className = "md-path"`,
		"a.dataset.path = target.path",
		"a.textContent = m[1]",
		"window.openPathPreview",
	} {
		if !strings.Contains(link, want) {
			t.Errorf("pathLinkNodes: missing %q", want)
		}
	}
	if strings.Contains(link, "innerHTML") {
		t.Error("pathLinkNodes must build DOM nodes, never innerHTML")
	}
	if !strings.Contains(src, "function pushPathNodes(nodes, text) {") {
		t.Error("md.js: pushPathNodes helper is missing")
	}

	outlets := map[string]string{
		"inline text": funcBody(src, "function mdInline(text) {"),
		"fenced code": between(src, `const pre = document.createElement("pre");`, "pre.appendChild(code);"),
	}
	for name, body := range outlets {
		if body == "" {
			t.Fatalf("md.js: %s outlet not found", name)
		}
		if !strings.Contains(body, "pathLinkNodes(") {
			t.Errorf("md.js: %s outlet does not linkify paths", name)
		}
	}
}

// TestPathPreviewEntryPoints verifies the app-side validation: only previewable
// local files with a directory part are linked, URLs are refused, and an
// absolute path is accepted only inside the workspace (converted to the
// workspace-relative form every preview API expects).
func TestPathPreviewEntryPoints(t *testing.T) {
	src := uiAsset(t, "/app.js", "/static/app.js", "app.js")

	pred := funcBody(src, "function isPreviewablePath(p) {")
	if pred == "" {
		t.Fatal("app.js: isPreviewablePath is missing")
	}
	if !strings.Contains(pred, "IMAGE_EXT.test(p) || TEXT_EXT.test(p)") {
		t.Error("isPreviewablePath must reuse the tree's two extension sets")
	}
	norm := funcBody(src, "function previewPathOf(raw) {")
	if norm == "" {
		t.Fatal("app.js: previewPathOf is missing")
	}
	for _, want := range []string{
		`rawStr.indexOf("://") >= 0`, // URLs are never paths
		`wsRoot + "/"`,          // absolute path must be in the workspace
		"isPreviewablePath(s)",
	} {
		if !strings.Contains(norm, want) {
			t.Errorf("previewPathOf: missing %q", want)
		}
	}
	for name, sig := range map[string]string{
		"openPathPreview":   "function openPathPreview(path, line) {",
		"jumpToPreviewLine": "async function jumpToPreviewLine(path, line) {",
	} {
		if funcBody(src, sig) == "" {
			t.Errorf("app.js: %s is missing", name)
		}
	}
	if !strings.Contains(src, "wsRoot = b.workspace.replace") {
		t.Error("app.js: wsRoot is not taken from the /api/bootstrap workspace root")
	}
}

// TestPreviewStaysInsideWorkspace verifies the read-only preview API does not
// read outside the workspace: a path from the conversation that resolves
// outside (traversal or an absolute path elsewhere on disk) is never served,
// while the workspace-relative form of a workspace file still is.
func TestPreviewStaysInsideWorkspace(t *testing.T) {
	_, ts, root := newTestServer(t)

	// A real file next to the workspace: it must not be reachable, no matter
	// whether it is requested as an absolute path or through traversal.
	outside := filepath.Join(filepath.Dir(root), "outside-feature576.txt")
	if err := os.WriteFile(outside, []byte("secret\n"), 0644); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(outside) }()

	for _, tc := range []struct{ name, path string }{
		{"traversal", "../../etc/passwd"},
		{"absolute outside", outside},
		{"absolute system", "/etc/hosts"},
	} {
		resp, err := http.Get(ts.URL + "/api/file?path=" + url.QueryEscape(tc.path))
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			t.Errorf("%s: /api/file?path=%s = 200, want the preview API to stay inside the workspace", tc.name, tc.path)
		}
	}

	// Positive control: the workspace-relative form is still readable (this is
	// the form previewPathOf converts an in-workspace absolute path to).
	rel := "feature576-preview.txt"
	if err := os.WriteFile(filepath.Join(root, rel), []byte("ok\n"), 0644); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(filepath.Join(root, rel)) }()
	resp, err := http.Get(ts.URL + "/api/file?path=" + url.QueryEscape(rel) + "&start=1&end=10")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("workspace-relative read = %d, want 200", resp.StatusCode)
	}
}
