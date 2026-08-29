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
	"sort"
	"strings"
)

// Skill represents a single skill discovered from a skills directory.
// A skill is a directory containing a SKILL.md file (Agent Skills open standard).
type Skill struct {
	// Name is the skill name, parsed from frontmatter or falling back to the
	// directory name.
	Name string
	// Description is a short summary, parsed from frontmatter or falling back
	// to the first non-empty line of SKILL.md.
	Description string
	// Path is the absolute path to the SKILL.md file.
	Path string
	// Dir is the absolute path to the skill directory.
	Dir string
}

// scanSkillsDir scans a single skills directory (e.g. ./skills or
// ~/.co-shell/skills) and returns the skills found. Each immediate
// subdirectory containing a SKILL.md is treated as a skill. Returns an empty
// slice when the directory does not exist or contains no skills.
func scanSkillsDir(dir string) []Skill {
	var skills []Skill
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skillDir := filepath.Join(dir, e.Name())
		skillPath := filepath.Join(skillDir, "SKILL.md")
		data, err := os.ReadFile(skillPath)
		if err != nil {
			continue
		}
		name, desc := parseSkillFrontmatter(string(data))
		if name == "" {
			name = e.Name()
		}
		if desc == "" {
			desc = firstNonEmptyLine(string(data))
		}
		skills = append(skills, Skill{
			Name:        name,
			Description: desc,
			Path:        skillPath,
			Dir:         skillDir,
		})
	}
	return skills
}

// scanSkills scans both the workspace-level skills directory and the global
// skills directory, merging the results. Workspace skills take precedence over
// global skills with the same name. The result is sorted by name.
func scanSkills(workspacePath, homeDir string) []Skill {
	byName := make(map[string]Skill)
	var order []string

	add := func(skills []Skill) {
		for _, s := range skills {
			if _, exists := byName[s.Name]; !exists {
				order = append(order, s.Name)
				byName[s.Name] = s
			}
		}
	}

	if workspacePath != "" {
		add(scanSkillsDir(filepath.Join(workspacePath, "skills")))
	}
	if homeDir != "" {
		add(scanSkillsDir(filepath.Join(homeDir, ".co-shell", "skills")))
	}

	sort.Strings(order)
	skills := make([]Skill, 0, len(order))
	for _, name := range order {
		skills = append(skills, byName[name])
	}
	return skills
}

// ScanSkills is the exported wrapper around scanSkills, used by the :skill
// command handler to list skills from both the workspace and global directories.
func ScanSkills(workspacePath, homeDir string) []Skill {
	return scanSkills(workspacePath, homeDir)
}

// parseSkillFrontmatter parses the YAML frontmatter of a SKILL.md file and
// returns the name and description fields. The frontmatter is delimited by
// "---" lines at the start of the file. Returns empty strings when there is no
// frontmatter or the fields are absent.
func parseSkillFrontmatter(content string) (name, description string) {
	trimmed := strings.TrimLeft(content, "\ufeff \t\r\n")
	if !strings.HasPrefix(trimmed, "---") {
		return "", ""
	}
	// Find the closing "---" delimiter.
	rest := trimmed[3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", ""
	}
	fm := rest[:end]
	for _, line := range strings.Split(fm, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name:") {
			name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
			name = strings.Trim(name, `"'`)
		} else if strings.HasPrefix(line, "description:") {
			description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
			description = strings.Trim(description, `"'`)
		}
	}
	return name, description
}

// firstNonEmptyLine returns the first non-empty, non-heading line of content.
// Used as a fallback description when frontmatter has no description.
func firstNonEmptyLine(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "---") {
			continue
		}
		return line
	}
	return ""
}

// buildSkillsIndex builds the SKILLS section body listing each skill as an
// index entry (name, description, path) without loading the SKILL.md content.
// Returns an empty string when there are no skills.
func buildSkillsIndex(skills []Skill) string {
	if len(skills) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, s := range skills {
		sb.WriteString("- ")
		sb.WriteString(s.Name)
		sb.WriteString(": ")
		sb.WriteString(s.Description)
		sb.WriteString(" (")
		sb.WriteString(s.Path)
		sb.WriteString(")\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}
