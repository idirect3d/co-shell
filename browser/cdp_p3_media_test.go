// Author: L.Shuang
// Created: 2026-09-26
// Last Modified: 2026-09-26
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
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTestFile creates a file with the given content below dir.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestUploadParams covers A-019: the selector is resolved first and the files
// are then set on that node.
func TestUploadParams(t *testing.T) {
	session := newMockSession()
	session.results["DOM.getDocument"] = map[string]interface{}{
		"root": map[string]interface{}{"nodeId": float64(1)},
	}
	session.results["DOM.querySelector"] = map[string]interface{}{"nodeId": float64(42)}

	files := []string{"/tmp/a.txt", "/tmp/b.txt"}
	if err := UploadFiles(context.Background(), session, "#f", files); err != nil {
		t.Fatalf("UploadFiles: %v", err)
	}

	want := []string{"DOM.getDocument", "DOM.querySelector", "DOM.setFileInputFiles"}
	got := session.methods()
	if len(got) != len(want) {
		t.Fatalf("calls = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("calls = %v, want %v", got, want)
		}
	}

	query := session.paramsOf("DOM.querySelector")
	if query["selector"] != "#f" {
		t.Errorf("selector = %v, want #f", query["selector"])
	}

	params := session.paramsOf("DOM.setFileInputFiles")
	if nodeID, ok := params["nodeId"].(int); !ok || nodeID != 42 {
		t.Errorf("nodeId = %v, want 42", params["nodeId"])
	}
	uploaded, ok := params["files"].([]string)
	if !ok || len(uploaded) != 2 || uploaded[0] != files[0] || uploaded[1] != files[1] {
		t.Errorf("files = %v, want %v", params["files"], files)
	}
}

// TestWorkspaceFiles covers A-020 and A-022: resolution, validation and order.
func TestWorkspaceFiles(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "input", "a.txt"), "a")
	writeTestFile(t, filepath.Join(root, "input", "b.txt"), "b")

	resolved, err := WorkspaceFiles(root, []string{"input/a.txt", filepath.Join(root, "input", "b.txt")})
	if err != nil {
		t.Fatalf("WorkspaceFiles: %v", err)
	}
	if len(resolved) != 2 {
		t.Fatalf("resolved %d files, want 2", len(resolved))
	}
	if resolved[0] != filepath.Join(root, "input", "a.txt") {
		t.Errorf("relative path resolved to %q", resolved[0])
	}
	if resolved[1] != filepath.Join(root, "input", "b.txt") {
		t.Errorf("absolute path resolved to %q", resolved[1])
	}

	ordered, err := WorkspaceFiles(root, []string{"input/b.txt", "input/a.txt", "input/a.txt"})
	if err != nil {
		t.Fatalf("WorkspaceFiles (order): %v", err)
	}
	if len(ordered) != 3 || ordered[0] != resolved[1] || ordered[1] != resolved[0] || ordered[2] != resolved[0] {
		t.Errorf("order or duplicates were not preserved: %v", ordered)
	}

	if _, err := WorkspaceFiles(root, []string{"input/missing.txt"}); err == nil {
		t.Error("a missing file must be rejected")
	}
	if _, err := WorkspaceFiles(root, []string{"../outside.txt"}); err == nil {
		t.Error("a path escaping the workspace must be rejected")
	}
	if _, err := WorkspaceFiles(root, []string{filepath.Join(root, "input")}); err == nil {
		t.Error("a directory must be rejected")
	}
	if _, err := WorkspaceFiles(root, nil); err == nil {
		t.Error("an empty file list must be rejected")
	}
}

// TestUploadSelectorNotFound covers A-021: nothing is set when no element matches.
func TestUploadSelectorNotFound(t *testing.T) {
	session := newMockSession()
	session.results["DOM.getDocument"] = map[string]interface{}{
		"root": map[string]interface{}{"nodeId": float64(1)},
	}
	session.results["DOM.querySelector"] = map[string]interface{}{"nodeId": float64(0)}

	err := UploadFiles(context.Background(), session, "#missing", []string{"/tmp/a.txt"})
	if err == nil {
		t.Fatal("a selector without match must fail")
	}
	if !strings.Contains(err.Error(), "no element matches selector") {
		t.Errorf("unexpected error: %v", err)
	}
	if session.called("DOM.setFileInputFiles") {
		t.Errorf("files must not be set: %v", session.methods())
	}
}

// TestPDFParams covers A-023: printBackground defaults to true and every other
// option is only sent when set.
func TestPDFParams(t *testing.T) {
	params := PrintToPDFParams(PDFOptions{})
	if params["printBackground"] != true {
		t.Errorf("printBackground = %v, want true by default", params["printBackground"])
	}
	if len(params) != 1 {
		t.Errorf("optional parameters must be omitted, got %v", params)
	}

	landscape := true
	printBackground := false
	paperWidth := 8.5
	paperHeight := 11.0
	scale := 1.5
	params = PrintToPDFParams(PDFOptions{
		Landscape:       &landscape,
		PrintBackground: &printBackground,
		PaperWidth:      &paperWidth,
		PaperHeight:     &paperHeight,
		Scale:           &scale,
	})
	if params["printBackground"] != false {
		t.Errorf("printBackground = %v, want false", params["printBackground"])
	}
	if params["landscape"] != true {
		t.Errorf("landscape = %v, want true", params["landscape"])
	}
	if params["paperWidth"] != 8.5 || params["paperHeight"] != 11.0 {
		t.Errorf("paper size = %v x %v", params["paperWidth"], params["paperHeight"])
	}
	if params["scale"] != 1.5 {
		t.Errorf("scale = %v, want 1.5", params["scale"])
	}
}

// TestPrintToPDF covers A-023/A-024: the data is passed through and written.
func TestPrintToPDF(t *testing.T) {
	session := newMockSession()
	payload := "%PDF-1.4 test payload"
	session.results["Page.printToPDF"] = map[string]interface{}{
		"data": base64.StdEncoding.EncodeToString([]byte(payload)),
	}

	data, err := PrintToPDF(context.Background(), session, PDFOptions{})
	if err != nil {
		t.Fatalf("PrintToPDF: %v", err)
	}

	path := filepath.Join(t.TempDir(), "nested", "page.pdf")
	size, err := SavePDF(data, path)
	if err != nil {
		t.Fatalf("SavePDF: %v", err)
	}
	if size != len(payload) {
		t.Errorf("size = %d, want %d", size, len(payload))
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(content) != payload {
		t.Errorf("file content = %q, want %q", string(content), payload)
	}
}

// TestSavePDFInvalidData covers A-024: invalid base64 leaves no file behind.
func TestSavePDFInvalidData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.pdf")
	if _, err := SavePDF("not-base64!!", path); err == nil {
		t.Fatal("invalid base64 must fail")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("no file must be created, stat error: %v", err)
	}
}

// TestPDFFileName covers A-025: illegal characters are replaced and a missing
// extension or title falls back to a usable name.
func TestPDFFileName(t *testing.T) {
	cases := []struct{ title, want string }{
		{"Simple Title", "Simple Title.pdf"},
		{`a/b:c*d?e"f<g>h|i`, "a-b-c-d-e-f-g-h-i.pdf"},
		{"trailing. ", "trailing.pdf"},
		{"", "page.pdf"},
		{"report.PDF", "report.PDF"},
	}
	for _, tc := range cases {
		if got := PDFFileName(tc.title); got != tc.want {
			t.Errorf("PDFFileName(%q) = %q, want %q", tc.title, got, tc.want)
		}
	}
	if PDFDefaultDir != "download/browser" {
		t.Errorf("PDFDefaultDir = %q, want download/browser", PDFDefaultDir)
	}
}
