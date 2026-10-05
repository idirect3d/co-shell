// Package web - FIX-608 tests: downloading a workspace file must never
// navigate the top-level document (that unloaded the SPA and, on iOS
// WKWebView, left a blank page), and the iOS client must hand attachment
// responses to WKDownload so the file is saved while the page stays.
//
// Author: L.Shuang
// Created: 2026-10-05
// MIT License - Copyright (c) 2026 L.Shuang
package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFIX608FrontendDownloadWithoutNavigation is UC-01/UC-02/UC-03: the
// download trigger stays off the top-level navigation path and goes through a
// hidden iframe instead, keeping the workspace page in place.
func TestFIX608FrontendDownloadWithoutNavigation(t *testing.T) {
	js := staticAsset(t, "/static/app.js")

	start := strings.Index(js, "function downloadFile(path)")
	if start < 0 {
		t.Fatal("app.js has no downloadFile(path) function")
	}
	end := strings.Index(js[start:], "\n}")
	if end < 0 {
		t.Fatal("cannot locate the end of downloadFile(path)")
	}
	body := js[start : start+end]

	for _, banned := range []string{"window.location.href", "location.assign", "location.replace"} {
		if strings.Contains(body, banned) {
			t.Errorf("downloadFile must not navigate the document, found %q", banned)
		}
	}
	for _, want := range []string{
		`"api/download?path=" + encodeURIComponent(path)`,
		`document.createElement("iframe")`,
		`method: "HEAD"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("downloadFile is missing %q", want)
		}
	}
}

// TestFIX608FrontendDownloadFailureMessage is UC-04: an error status (403/404)
// surfaces a localized message instead of a blank page.
func TestFIX608FrontendDownloadFailureMessage(t *testing.T) {
	js := staticAsset(t, "/static/app.js")
	for _, want := range []string{
		`downloadFailed: "下载失败：%s"`,
		`downloadFailed: "Download failed: %s"`,
		"alert(T.downloadFailed.replace",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("app.js is missing %q", want)
		}
	}
}

// TestFIX608IOSDownloadSupport is UC-07/UC-08/UC-09: the iOS web view declares
// the download policy, wires WKDownload and writes into its own sandbox
// directory. The real-device behaviour is covered by use-case/FIX-608.
func TestFIX608IOSDownloadSupport(t *testing.T) {
	path := filepath.Join("..", "mobile", "ios", "Runner", "WebViewController.swift")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}
	src := string(data)

	for _, want := range []string{
		"WKNavigationResponsePolicy",
		"decidePolicyFor navigationResponse",
		"Content-Disposition",
		"decisionHandler(.download)",
		"download.delegate = self",
		"extension WebViewController: WKDownloadDelegate",
		"decideDestinationUsing",
		"UIActivityViewController",
		"NSURLErrorCancelled",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("WebViewController.swift is missing %q", want)
		}
	}
}
