// Author: L.Shuang
// Created: 2026-08-01
// Last Modified: 2026-08-01
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

package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRenameModeDirs_PartialFailure verifies UC-0006: when one rename fails,
// the others still succeed and the failed entry is reported (not fatal).
func TestRenameModeDirs_PartialFailure(t *testing.T) {
	root := t.TempDir()
	// Create two source dirs.
	srcGood := filepath.Join(root, "mode", "research")
	if err := os.MkdirAll(srcGood, 0755); err != nil {
		t.Fatalf("mkdir research: %v", err)
	}
	srcBad := filepath.Join(root, "mode", "plan")
	if err := os.MkdirAll(srcBad, 0755); err != nil {
		t.Fatalf("mkdir plan: %v", err)
	}
	// Make the bad target impossible: create a FILE at the destination path so
	// os.Rename (dir -> existing file) fails on most systems.
	badTarget := filepath.Join(root, "mode", "plan.20260101")
	if err := os.WriteFile(badTarget, []byte("x"), 0644); err != nil {
		t.Fatalf("write bad target: %v", err)
	}

	entries := []modeDirRenameEntry{
		{
			oldPath: srcGood,
			newPath: filepath.Join(root, "mode", "research.20260101"),
			oldName: "research",
			newName: "research.20260101",
		},
		{
			oldPath: srcBad,
			newPath: badTarget,
			oldName: "plan",
			newName: "plan.20260101",
		},
	}
	renamed, failed := renameModeDirs(entries)
	if len(renamed) != 1 {
		t.Errorf("expected 1 renamed, got %d (%+v)", len(renamed), renamed)
	}
	if len(failed) != 1 {
		t.Fatalf("expected 1 failed, got %d (%+v)", len(failed), failed)
	}
	if failed[0].oldName != "plan" || failed[0].err == nil {
		t.Errorf("failed entry should be plan with an error, got %+v", failed[0])
	}
	// The good dir must have been renamed.
	if _, err := os.Stat(srcGood); !os.IsNotExist(err) {
		t.Errorf("research dir should have been moved")
	}
	if _, err := os.Stat(filepath.Join(root, "mode", "research.20260101")); err != nil {
		t.Errorf("research.20260101 should exist: %v", err)
	}
}
