// Package web - FEATURE-547 tests: the workspace image preview is a non-modal
// view inside the central message area with a floating info bubble, instead of
// the full-screen #preview modal (which stays in place for attachments).
//
// Author: L.Shuang
// Created: 2026-09-21
// MIT License - Copyright (c) 2026 L.Shuang

package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// staticAsset fetches an embedded UI asset through the same handler the browser
// uses, so the structural assertions below run against what is actually served.
func staticAsset(t *testing.T, path string) string {
	t.Helper()
	handler := serveStatic()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", path, rec.Code)
	}
	return rec.Body.String()
}

// between returns the text from the first occurrence of start to the first
// occurrence of end after it (exclusive), or "" when either marker is absent.
func between(s, start, end string) string {
	i := strings.Index(s, start)
	if i < 0 {
		return ""
	}
	j := strings.Index(s[i:], end)
	if j < 0 {
		return ""
	}
	return s[i : i+j]
}

// funcBody returns the source of a JS function, from its signature to its
// matching closing brace, or "" when the function is missing.
func funcBody(src, signature string) string {
	i := strings.Index(src, signature)
	if i < 0 {
		return ""
	}
	depth := 0
	for j := i; j < len(src); j++ {
		switch src[j] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[i : j+1]
			}
		}
	}
	return ""
}

// TestFEATURE547ImageViewerIsNonModal is UC-0001: the image previewer lives in
// the central message area as a non-modal .file-viewer (same shape as the text
// previewer) and sits before the #preview modal in the document.
func TestFEATURE547ImageViewerIsNonModal(t *testing.T) {
	html := staticAsset(t, "/static/")
	idx := strings.Index(html, `id="imgViewer"`)
	if idx < 0 {
		t.Fatal(`#imgViewer missing — images would still open the #preview modal`)
	}
	tagStart := strings.LastIndex(html[:idx], "<div")
	tag := between(html[tagStart:], "<div", ">") + ">"
	if !strings.Contains(tag, `class="file-viewer hidden"`) {
		t.Errorf("imgViewer tag = %q, want class \"file-viewer hidden\"", tag)
	}
	// It must be part of the main-area section (closed by the next </section>),
	// not appended after the input row like the #preview modal.
	block := between(html, `id="imgViewer"`, "</section>")
	if !strings.Contains(block, `id="ivImg"`) {
		t.Error("imgViewer is not inside the main message-area section")
	}
	if p := strings.Index(html, `id="preview"`); p >= 0 && p < idx {
		t.Error("#imgViewer appears before #preview; expected it in the main area")
	}
}

// TestFEATURE547ImageViewerBubble is UC-0002: the viewer carries the shared
// floating info bubble with name / size / mtime / resolution and a close button.
func TestFEATURE547ImageViewerBubble(t *testing.T) {
	html := staticAsset(t, "/static/")
	block := between(html, `id="imgViewer"`, "</section>")
	if block == "" {
		t.Fatal("imgViewer block not found")
	}
	for _, want := range []string{
		`class="fv-titlebar"`, `id="ivName"`, `id="ivSize"`,
		`id="ivMtime"`, `id="ivRes"`, `id="ivClose"`,
	} {
		if !strings.Contains(block, want) {
			t.Errorf("imgViewer block missing %s", want)
		}
	}
	if strings.Contains(block, "preview-titlebar") {
		t.Error("imgViewer uses the modal title bar class instead of .fv-titlebar")
	}
}

// TestFEATURE547ImagePreviewNoLongerOpensModal is UC-0004: openImagePreview must
// fill the non-modal viewer (and toggle off on a second click) rather than
// un-hiding the #preview modal.
func TestFEATURE547ImagePreviewNoLongerOpensModal(t *testing.T) {
	js := staticAsset(t, "/static/app.js")
	body := funcBody(js, "function openImagePreview(node) {")
	if body == "" {
		t.Fatal("openImagePreview not found in app.js")
	}
	if strings.Contains(body, "preview.classList.remove") {
		t.Error("openImagePreview still opens the #preview modal")
	}
	for _, want := range []string{
		"imgViewer.classList.remove",
		"ivImg.src =",
		"ivImg.onload",
		"highlightTreeFile(node.path)",
		"if (ivPath === node.path) { closeImageViewer(); return; }",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("openImagePreview missing %q", want)
		}
	}
	// The text and image viewers must be mutually exclusive.
	if text := funcBody(js, "function openFilePreview(node) {"); !strings.Contains(text, "closeImageViewer();") {
		t.Error("openFilePreview does not close the image viewer (views would stack)")
	}
	if !strings.Contains(funcBody(js, "function closeImageViewer() {"), `imgViewer.classList.add("hidden")`) {
		t.Error("closeImageViewer does not hide the viewer")
	}
}

// TestFEATURE547ModalKeptForAttachments is UC-0003: the #preview modal and its
// click-outside-to-close binding stay, because the input-row attachment
// thumbnails still enlarge through it.
func TestFEATURE547ModalKeptForAttachments(t *testing.T) {
	html := staticAsset(t, "/static/")
	for _, want := range []string{`id="preview"`, `id="previewImg"`, `id="previewClose"`} {
		if !strings.Contains(html, want) {
			t.Errorf("index.html missing %s — the attachment enlarge path would break", want)
		}
	}
	js := staticAsset(t, "/static/app.js")
	for _, want := range []string{
		`previewClose.onclick = () => preview.classList.add("hidden")`,
		`if (e.target === preview) preview.classList.add("hidden")`,
		"function openAttachPreview(it)",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("app.js missing %q", want)
		}
	}
}
