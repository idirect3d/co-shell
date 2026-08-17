// Package web - OS integration helpers: open the default browser, open a
// file with its default application, reveal a file in the OS file manager
// (FEATURE-307c). Dispatched by runtime.GOOS (no build tags needed). The
// concrete launcher functions are package-level variables so tests can
// inject fakes instead of really spawning processes.
//
// Author: L.Shuang
// Created: 2026-08-17
// Last Modified: 2026-08-17
// MIT License - Copyright (c) 2026 L.Shuang

package web

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

// Injectable OS launchers (replaced by fakes in tests).
var (
	openBrowserFunc = openBrowserOS
	openFileFunc    = openFileOS
	revealFileFunc  = revealFileOS
)

// OpenBrowser opens the URL in the OS default browser.
func OpenBrowser(url string) error { return openBrowserFunc(url) }

// openBrowserOS is the platform-specific browser launcher.
func openBrowserOS(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

// openFileOS opens a file with its OS-default application.
func openFileOS(path string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", path).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}

// revealFileOS locates a file in the OS file manager.
func revealFileOS(path string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", "-R", path).Start()
	case "windows":
		return exec.Command("explorer", "/select,"+path).Start()
	default:
		// No portable "reveal" on Linux: open the containing directory.
		return exec.Command("xdg-open", filepath.Dir(path)).Start()
	}
}
