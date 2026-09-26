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

// File upload support: a CSS selector is resolved to a file input through the
// DOM domain and the workspace files are attached with DOM.setFileInputFiles,
// which is the only way to fill a file input without a native file dialog.
//
// Paths are always resolved against the workspace root and rejected when they
// escape it, so a tool can never upload files from outside the workspace
// (FEATURE-556 Phase 3).

package browser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WorkspaceFiles resolves files (absolute or workspace-relative) to absolute
// paths, keeping the caller's order and duplicates. It fails when a path
// escapes root, does not exist or is not a regular file.
func WorkspaceFiles(root string, files []string) ([]string, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("at least one file is required")
	}
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("workspace root must not be empty")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve workspace root %q: %w", root, err)
	}

	resolved := make([]string, 0, len(files))
	for _, file := range files {
		if strings.TrimSpace(file) == "" {
			return nil, fmt.Errorf("file path must not be empty")
		}
		path := file
		if !filepath.IsAbs(path) {
			path = filepath.Join(absRoot, path)
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve file %q: %w", file, err)
		}
		if !pathInside(absRoot, abs) {
			return nil, fmt.Errorf("file %q is outside the workspace", file)
		}
		info, err := os.Stat(abs)
		if err != nil {
			return nil, fmt.Errorf("file %q does not exist", file)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("%q is a directory, not a file", file)
		}
		resolved = append(resolved, abs)
	}
	return resolved, nil
}

// pathInside reports whether path is root itself or below it.
func pathInside(root, path string) bool {
	if path == root {
		return true
	}
	return strings.HasPrefix(path, root+string(filepath.Separator))
}

// UploadFiles attaches files to the file input matched by selector: the
// selector is resolved with DOM.querySelector and the files are set with
// DOM.setFileInputFiles. It returns an error when the selector matches nothing,
// in which case no file is set.
func UploadFiles(ctx context.Context, caller CDPCaller, selector string, files []string) error {
	if len(files) == 0 {
		return fmt.Errorf("at least one file is required")
	}
	nodeID, err := QuerySelectorNodeID(ctx, caller, selector)
	if err != nil {
		return err
	}
	if _, err := caller.Call(ctx, "DOM.setFileInputFiles", map[string]interface{}{
		"files":  files,
		"nodeId": nodeID,
	}); err != nil {
		return fmt.Errorf("cannot set files on %q: %w", selector, err)
	}
	return nil
}

// SetFileInputFiles attaches files to the file input matched by selector.
func (c *CDPClient) SetFileInputFiles(ctx context.Context, selector string, files []string) error {
	return UploadFiles(ctx, c, selector, files)
}
