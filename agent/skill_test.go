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

package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSkill creates a skill directory with a SKILL.md file under baseDir.
func writeSkill(t *testing.T, baseDir, name, content string) {
	t.Helper()
	dir := filepath.Join(baseDir, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
}

func TestParseSkillFrontmatter(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantName    string
		wantDesc    string
	}{
		{
			name: "with frontmatter",
			content: "---\nname: my-skill\ndescription: A test skill\n---\nUsage instructions...\n",
			wantName: "my-skill",
			wantDesc: "A test skill",
		},
		{
			name:     "no frontmatter",
			content:  "Just a plain skill file\n",
			wantName: "",
			wantDesc: "",
		},
		{
			name:     "frontmatter without name",
			content:  "---\ndescription: only desc\n---\nbody\n",
			wantName: "",
			wantDesc: "only desc",
		},
		{
			name:     "quoted values",
			content:  "---\nname: \"quoted-name\"\ndescription: 'quoted desc'\n---\nbody\n",
			wantName: "quoted-name",
			wantDesc: "quoted desc",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, desc := parseSkillFrontmatter(tt.content)
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if desc != tt.wantDesc {
				t.Errorf("desc = %q, want %q", desc, tt.wantDesc)
			}
		})
	}
}

func TestScanSkills(t *testing.T) {
	ws := t.TempDir()
	home := t.TempDir()

	// Workspace skill with frontmatter.
	writeSkill(t, filepath.Join(ws, "skills"), "with-fm", "---\nname: with-fm\ndescription: workspace skill\n---\nbody\n")
	// Workspace skill without frontmatter (name falls back to dir, desc to first line).
	writeSkill(t, filepath.Join(ws, "skills"), "no-fm", "no frontmatter skill\nbody\n")
	// Global skill.
	writeSkill(t, filepath.Join(home, ".co-shell", "skills"), "global-skill", "---\nname: global-skill\ndescription: global skill\n---\nbody\n")

	skills := scanSkills(ws, home)
	if len(skills) != 3 {
		t.Fatalf("got %d skills, want 3", len(skills))
	}

	byName := map[string]Skill{}
	for _, s := range skills {
		byName[s.Name] = s
	}

	if s, ok := byName["with-fm"]; !ok || s.Description != "workspace skill" {
		t.Errorf("with-fm not parsed correctly: %+v", byName["with-fm"])
	}
	if s, ok := byName["no-fm"]; !ok || s.Name != "no-fm" || s.Description != "no frontmatter skill" {
		t.Errorf("no-fm fallback not applied: %+v", byName["no-fm"])
	}
	if s, ok := byName["global-skill"]; !ok || s.Description != "global skill" {
		t.Errorf("global-skill not found: %+v", byName["global-skill"])
	}
}

func TestScanSkillsWorkspacePrecedence(t *testing.T) {
	ws := t.TempDir()
	home := t.TempDir()

	// Same skill name in both workspace and global; workspace should win.
	writeSkill(t, filepath.Join(ws, "skills"), "dup", "---\nname: dup\ndescription: workspace version\n---\nbody\n")
	writeSkill(t, filepath.Join(home, ".co-shell", "skills"), "dup", "---\nname: dup\ndescription: global version\n---\nbody\n")

	skills := scanSkills(ws, home)
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}
	if skills[0].Description != "workspace version" {
		t.Errorf("workspace precedence not applied, got %q", skills[0].Description)
	}
}

func TestScanSkillsEmpty(t *testing.T) {
	ws := t.TempDir()
	home := t.TempDir()
	skills := scanSkills(ws, home)
	if len(skills) != 0 {
		t.Errorf("expected no skills, got %d", len(skills))
	}
}

func TestBuildSkillsIndex(t *testing.T) {
	skills := []Skill{
		{Name: "alpha", Description: "first skill", Path: "/ws/skills/alpha/SKILL.md"},
		{Name: "beta", Description: "second skill", Path: "/ws/skills/beta/SKILL.md"},
	}
	index := buildSkillsIndex(skills)
	if !strings.Contains(index, "- alpha: first skill (/ws/skills/alpha/SKILL.md)") {
		t.Errorf("index missing alpha entry:\n%s", index)
	}
	if !strings.Contains(index, "- beta: second skill (/ws/skills/beta/SKILL.md)") {
		t.Errorf("index missing beta entry:\n%s", index)
	}

	if got := buildSkillsIndex(nil); got != "" {
		t.Errorf("empty index should be empty, got %q", got)
	}
}
