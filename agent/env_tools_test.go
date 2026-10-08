//
// FEATURE-623 tests: the <tools> block is split into a global section
// (~/.co-shell/bin) and a workspace section (<cwd>/bin). Each section prints its
// absolute path exactly once and lists bare file names ("name time size") below
// it.
//
// Author: L.Shuang
// MIT License - Copyright (c) 2026 L.Shuang
//

package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTool creates one dummy tool file inside dir.
func writeTool(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// TestBuildToolsBlockTwoSections is the core FEATURE-623 shape: global section
// first, workspace second, one absolute path each, bare names below.
func TestBuildToolsBlockTwoSections(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	globalDir := filepath.Join(home, ".co-shell", "bin")
	writeTool(t, globalDir, "global_tool.py")

	ws := t.TempDir()
	wsBin := filepath.Join(ws, "bin")
	writeTool(t, wsBin, "space_tool.py")

	block := buildToolsBlock(ws)

	gi, wi := strings.Index(block, "<global>"), strings.Index(block, "<workspace>")
	if gi < 0 || wi < 0 || gi > wi {
		t.Fatalf("global section must come first, got:\n%s", block)
	}
	if n := strings.Count(block, globalDir); n != 1 {
		t.Fatalf("global path must appear exactly once, got %d:\n%s", n, block)
	}
	if n := strings.Count(block, wsBin); n != 1 {
		t.Fatalf("workspace path must appear exactly once, got %d:\n%s", n, block)
	}
	for _, want := range []string{"global_tool.py", "space_tool.py"} {
		if !strings.Contains(block, want) {
			t.Fatalf("missing %q in:\n%s", want, block)
		}
	}
	// No path prefix on tool rows (the pre-FEATURE-623 form was "bin/xxx.py …").
	if strings.Contains(block, "bin/global_tool.py") || strings.Contains(block, "bin/space_tool.py") {
		t.Fatalf("tool rows must not carry a path prefix:\n%s", block)
	}
	// Every section keeps the path on its own line, directly under its tag.
	if !strings.HasPrefix(block, "<global>\n"+globalDir+"\n") {
		t.Fatalf("global section must start with its absolute path line:\n%s", block)
	}
	if !strings.Contains(block, "</global>\n<workspace>\n"+wsBin+"\n") {
		t.Fatalf("workspace section must follow the global one with its own path line:\n%s", block)
	}
	t.Logf("TOOLS|global=%s|workspace=%s|bytes=%d", globalDir, wsBin, len(block))
}

// TestBuildToolsBlockEmptyDirs keeps both path lines when the directories are
// missing or empty, so the reader can tell the section is empty.
func TestBuildToolsBlockEmptyDirs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	globalDir := filepath.Join(home, ".co-shell", "bin") // never created

	ws := t.TempDir()
	wsBin := filepath.Join(ws, "bin")
	if err := os.MkdirAll(wsBin, 0o755); err != nil { // exists but empty
		t.Fatal(err)
	}

	block := buildToolsBlock(ws)
	for _, want := range []string{"<global>\n" + globalDir, "<workspace>\n" + wsBin, "</global>", "</workspace>"} {
		if !strings.Contains(block, want) {
			t.Fatalf("missing %q in empty-dirs block:\n%s", want, block)
		}
	}
	if strings.Count(block, "\n") > 6 {
		t.Fatalf("empty sections must not add tool rows:\n%s", block)
	}
	t.Logf("TOOLS_EMPTY|bytes=%d", len(block))
}

// TestBuildToolsBlockSkipsGlobalWithoutHome drops the whole global section when
// the home directory cannot be resolved (user-chosen behaviour).
func TestBuildToolsBlockSkipsGlobalWithoutHome(t *testing.T) {
	t.Setenv("HOME", "")
	ws := t.TempDir()
	writeTool(t, filepath.Join(ws, "bin"), "space_tool.py")

	block := buildToolsBlock(ws)
	if strings.Contains(block, "<global>") {
		t.Fatalf("global section must be omitted without HOME:\n%s", block)
	}
	if !strings.Contains(block, "<workspace>") {
		t.Fatalf("workspace section must stay:\n%s", block)
	}
}

// TestEnvToolsChangeDetectCoversGlobal proves the change-detection hash sees the
// global directory too: an unchanged rebuild collapses to the placeholder, while
// a global-only change triggers a fresh full listing.
func TestEnvToolsChangeDetectCoversGlobal(t *testing.T) {
	a := testAgent(t)
	a.cfg.LLM.EnvHistoryPrune = false // sub-switches only apply with the trim off
	a.cfg.LLM.EnvDirChangeDetect = true

	first := a.envStaticBlocks(1, "f", "<global>\n/g\n</global>", "", true, true, false)
	if !strings.Contains(first, "<global>") {
		t.Fatalf("first call must emit the full block, got: %q", first)
	}
	second := a.envStaticBlocks(2, "f", "<global>\n/g\n</global>", "", true, true, false)
	if !strings.Contains(second, "unchanged since msg#1") {
		t.Fatalf("unchanged rebuild must collapse to the placeholder, got: %q", second)
	}
	third := a.envStaticBlocks(3, "f", "<global>\n/g\ntool.py 1 1B\n</global>", "", true, true, false)
	if strings.Contains(third, "unchanged") {
		t.Fatalf("a global-only change must re-emit the block, got: %q", third)
	}
	t.Logf("TOOLS_DETECT|first=%dB|second=%dB|third=%dB", len(first), len(second), len(third))
}

// TestEnvToolsSelfContainedWithTrim: with the history trim on, the block is
// always emitted in full (no placeholder) — FEATURE-622 behaviour preserved.
func TestEnvToolsSelfContainedWithTrim(t *testing.T) {
	a := testAgent(t) // EnvHistoryPrune = true
	a.cfg.LLM.EnvDirChangeDetect = true

	first := a.envStaticBlocks(1, "f", "<global>\n/g\n</global>", "", true, true, false)
	second := a.envStaticBlocks(2, "f", "<global>\n/g\n</global>", "", true, true, false)
	if strings.Contains(second, "unchanged") || !strings.Contains(second, "<global>") {
		t.Fatalf("trim on must keep re-emitting the full block, got: %q", second)
	}
	t.Logf("TOOLS_TRIM|first=%dB|second=%dB", len(first), len(second))
}
