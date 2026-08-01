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

	"github.com/idirect3d/co-shell/config"
)

// TestListModeDirsToRename_KnownModes verifies UC-0001: dirs whose name matches
// a known mode name are listed with a .YYYYMMDD new name.
func TestListModeDirsToRename_KnownModes(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"act", "plan", "research"} {
		if err := os.MkdirAll(filepath.Join(root, "mode", name), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
	}

	cfg := &config.Config{}
	entries := listModeDirsToRename(root, cfg, "20260101")
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d: %+v", len(entries), entries)
	}
	got := map[string]string{}
	for _, e := range entries {
		got[e.oldName] = e.newName
	}
	for _, name := range []string{"act", "plan", "research"} {
		if got[name] != name+".20260101" {
			t.Errorf("expected %s.%s, got %q", name, "20260101", got[name])
		}
	}
}

// TestListModeDirsToRename_UnknownDirsSkipped verifies UC-0002: dirs not matching
// known mode names are never listed.
func TestListModeDirsToRename_UnknownDirsSkipped(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"act_old", "claim", "research.v0", "account.v0"} {
		if err := os.MkdirAll(filepath.Join(root, "mode", name), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
	}

	cfg := &config.Config{}
	entries := listModeDirsToRename(root, cfg, "20260101")
	if len(entries) != 0 {
		t.Fatalf("expected no entries, got %+v", entries)
	}
}

// TestListModeDirsToRename_CustomMode verifies user-defined modes from cfg.WorkModes
// are also recognized.
func TestListModeDirsToRename_CustomMode(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "mode", "expert"), 0755); err != nil {
		t.Fatalf("mkdir expert: %v", err)
	}

	cfg := &config.Config{}
	cfg.WorkModes = append(cfg.WorkModes, config.WorkMode{Name: "expert"})
	entries := listModeDirsToRename(root, cfg, "20260101")
	if len(entries) != 1 || entries[0].oldName != "expert" || entries[0].newName != "expert.20260101" {
		t.Fatalf("expected expert -> expert.20260101, got %+v", entries)
	}
}

// TestListModeDirsToRename_ConflictSuffix verifies UC-0003: when the target name
// already exists, a numeric suffix is appended.
func TestListModeDirsToRename_ConflictSuffix(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "mode", "research"), 0755); err != nil {
		t.Fatalf("mkdir research: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "mode", "research.20260101"), 0755); err != nil {
		t.Fatalf("mkdir research.20260101: %v", err)
	}

	cfg := &config.Config{}
	entries := listModeDirsToRename(root, cfg, "20260101")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %+v", entries)
	}
	if entries[0].newName != "research.20260101.1" {
		t.Errorf("expected research.20260101.1, got %q", entries[0].newName)
	}
}

// TestRenameModeDirs_Success verifies UC-0005: rename moves the dir and keeps contents.
func TestRenameModeDirs_Success(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "mode", "research")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	identity := filepath.Join(src, "IDENTITY.md")
	if err := os.WriteFile(identity, []byte("hello"), 0644); err != nil {
		t.Fatalf("write IDENTITY: %v", err)
	}

	entries := []modeDirRenameEntry{{
		oldPath: src,
		newPath: filepath.Join(root, "mode", "research.20260101"),
		oldName: "research",
		newName: "research.20260101",
	}}
	renamed, failed := renameModeDirs(entries)
	if len(failed) != 0 || len(renamed) != 1 {
		t.Fatalf("expected 1 renamed, 0 failed; got renamed=%d failed=%d (%v)", len(renamed), len(failed), failed)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("source dir should not exist after rename")
	}
	if _, err := os.Stat(filepath.Join(root, "mode", "research.20260101", "IDENTITY.md")); err != nil {
		t.Errorf("renamed dir should keep IDENTITY.md: %v", err)
	}
}

// TestKnownModeNames verifies built-in + user modes are all known.
func TestKnownModeNames(t *testing.T) {
	cfg := &config.Config{}
	cfg.WorkModes = append(cfg.WorkModes, config.WorkMode{Name: "expert"})
	names := knownModeNames(cfg)
	for _, n := range []string{"act", "plan", "research", "expert"} {
		if !names[n] {
			t.Errorf("expected %q to be a known mode name", n)
		}
	}
	if names["act_old"] {
		t.Errorf("act_old should not be a known mode name")
	}
}
