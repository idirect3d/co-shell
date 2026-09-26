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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// uc556DownloadWait bounds how long the download is given to land on disk.
const uc556DownloadWait = 20 * time.Second

// TestIntegrationDownloadLanding covers B-021: with a configured download
// directory, apply-on-connect issues Browser.setDownloadBehavior and a page
// triggered save really lands in that directory (size > 0).
func TestIntegrationDownloadLanding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-Chrome integration test in -short mode")
	}
	if !IsChromeAvailable() {
		t.Skip("Chrome/Chromium is not installed")
	}

	// The download directory intentionally does not exist yet: Chrome (or the
	// caller) must be able to create it, otherwise a default
	// workspace/download/browser target would never work on a fresh workspace.
	root := t.TempDir()
	downloadDir := filepath.Join(root, "browser-downloads")
	profile, err := os.MkdirTemp("", "co-shell-uc556-dl-profile-")
	if err != nil {
		t.Fatalf("cannot create temp profile: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(profile) })

	port, err := uc556FreePort()
	if err != nil {
		t.Fatalf("cannot reserve debug port: %v", err)
	}

	cfg := DefaultChromeConfig()
	cfg.ProfileDir = profile
	cfg.Persist = true
	cfg.DownloadDir = downloadDir
	cfg.ApplyOnConnect = true

	manager := NewChromeManagerWithConfig(port, true, cfg)
	t.Cleanup(func() { manager.Stop() })

	if _, err := manager.Start(); err != nil {
		t.Fatalf("cannot start Chrome: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := manager.EnsurePageConnected(ctx); err != nil {
		t.Fatalf("cannot connect to Chrome: %v", err)
	}

	client := manager.Client()
	if client == nil {
		t.Fatal("Chrome manager has no CDP client")
	}

	uc556Navigate(t, client, uc556StartServer(t).URL+"/download.html")

	// Trigger the save exactly like the browser_click tool does: resolve the
	// element geometry, then click its centre.
	x, y, _ := uc556Center(t, client, "#dl")
	if err := uc556Run(t, 15*time.Second, func(ctx context.Context) error {
		return client.ClickEx(ctx, x, y, MouseButtonLeft, 1)
	}); err != nil {
		t.Fatalf("click #dl at (%v, %v): %v", x, y, err)
	}

	path, size := uc556WaitForDownload(t, downloadDir, uc556DownloadWait)
	if path == "" {
		t.Fatalf("no file landed in %s within %v", downloadDir, uc556DownloadWait)
	}
	if size <= 0 {
		t.Fatalf("downloaded file %s has size %d, want > 0", path, size)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read downloaded file %s: %v", path, err)
	}
	if !strings.Contains(string(data), "uc556-download-payload") {
		t.Fatalf("downloaded file %s has unexpected content %q", path, string(data))
	}
	t.Logf("download landed: %s (%d bytes)", path, size)
}

// uc556WaitForDownload polls dir until a regular non-empty file (ignoring
// Chrome's own *.crdownload temp files) appears, and returns its path and size.
func uc556WaitForDownload(t *testing.T, dir string, timeout time.Duration) (string, int64) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for {
		entries, err := os.ReadDir(dir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() || strings.HasSuffix(entry.Name(), ".crdownload") {
					continue
				}
				info, statErr := entry.Info()
				if statErr != nil {
					continue
				}
				return filepath.Join(dir, entry.Name()), info.Size()
			}
		}
		if !time.Now().Before(deadline) {
			return "", 0
		}
		time.Sleep(100 * time.Millisecond)
	}
}
