// Author: L.Shuang
// Created: 2026-08-30
// Last Modified: 2026-08-30
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
// IMPLIED, BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/store"
	"github.com/idirect3d/co-shell/workspace"
)

// newTestSkillHandler builds a SkillHandler whose workspace is a temp dir.
func newTestSkillHandler(t *testing.T) (*SkillHandler, string) {
	t.Helper()
	i18n.Init("zh")
	ws := t.TempDir()
	wsObj, err := workspace.New(ws)
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	boltStore, err := store.NewStore(wsObj)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = boltStore.Close() })
	ds := store.NewDualStore(boltStore, nil)
	ag := agent.New(nil, nil, ds, "")
	ag.SetWorkspacePath(ws)
	return NewSkillHandler(ag), ws
}

// writeWorkspaceSkill creates a skill under the workspace ./skills/ dir.
func writeWorkspaceSkill(t *testing.T, ws, name, content string) {
	t.Helper()
	dir := filepath.Join(ws, "skills", name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestSkillHandlerList(t *testing.T) {
	h, ws := newTestSkillHandler(t)
	writeWorkspaceSkill(t, ws, "my-skill", "---\nname: my-skill\ndescription: a test skill\n---\nbody\n")

	out, err := h.Handle([]string{"list"})
	if err != nil {
		t.Fatalf("list error: %v", err)
	}
	if !strings.Contains(out, "my-skill") || !strings.Contains(out, "a test skill") {
		t.Errorf("list output missing skill info:\n%s", out)
	}
}

func TestSkillHandlerListEmpty(t *testing.T) {
	h, _ := newTestSkillHandler(t)
	out, err := h.Handle([]string{"list"})
	if err != nil {
		t.Fatalf("list error: %v", err)
	}
	if !strings.Contains(out, i18n.T(i18n.KeySkillCmdNoSkills)) {
		t.Errorf("expected no-skills message, got:\n%s", out)
	}
}

func TestSkillHandlerShow(t *testing.T) {
	h, ws := newTestSkillHandler(t)
	writeWorkspaceSkill(t, ws, "my-skill", "---\nname: my-skill\ndescription: a test skill\n---\nDetailed usage here.\n")

	out, err := h.Handle([]string{"show", "my-skill"})
	if err != nil {
		t.Fatalf("show error: %v", err)
	}
	if !strings.Contains(out, "Detailed usage here.") {
		t.Errorf("show output missing SKILL.md content:\n%s", out)
	}
}

func TestSkillHandlerShowNotFound(t *testing.T) {
	h, _ := newTestSkillHandler(t)
	_, err := h.Handle([]string{"show", "missing"})
	if err == nil {
		t.Fatal("expected error for missing skill")
	}
}

func TestSkillHandlerAdd(t *testing.T) {
	h, ws := newTestSkillHandler(t)

	// Create a source skill directory.
	src := filepath.Join(t.TempDir(), "new-skill")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("---\nname: new-skill\ndescription: added skill\n---\nbody\n"), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := h.Handle([]string{"add", src})
	if err != nil {
		t.Fatalf("add error: %v", err)
	}
	if !strings.Contains(out, "new-skill") {
		t.Errorf("add output missing skill name:\n%s", out)
	}
	// Verify the skill was copied into workspace ./skills/.
	if _, err := os.Stat(filepath.Join(ws, "skills", "new-skill", "SKILL.md")); err != nil {
		t.Errorf("skill not copied to workspace: %v", err)
	}
}

func TestSkillHandlerAddSourceNotExist(t *testing.T) {
	h, _ := newTestSkillHandler(t)
	_, err := h.Handle([]string{"add", "/nonexistent/path"})
	if err == nil {
		t.Fatal("expected error for nonexistent source")
	}
}

func TestSkillHandlerRemove(t *testing.T) {
	h, ws := newTestSkillHandler(t)
	writeWorkspaceSkill(t, ws, "my-skill", "---\nname: my-skill\ndescription: a test skill\n---\nbody\n")

	out, err := h.Handle([]string{"remove", "my-skill"})
	if err != nil {
		t.Fatalf("remove error: %v", err)
	}
	if !strings.Contains(out, "my-skill") {
		t.Errorf("remove output missing skill name:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(ws, "skills", "my-skill")); !os.IsNotExist(err) {
		t.Errorf("skill dir not removed: %v", err)
	}
}

func TestSkillHandlerRemoveNotFound(t *testing.T) {
	h, _ := newTestSkillHandler(t)
	_, err := h.Handle([]string{"remove", "missing"})
	if err == nil {
		t.Fatal("expected error for missing skill")
	}
}
