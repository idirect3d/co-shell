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

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
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
//
// FIX-584: `description` is often written as a YAML block scalar
// ("description: >-", ">", "|", "|-"), whose value lives on the following
// indented lines. Those lines are folded (or kept verbatim for "|") instead of
// being dropped, so the field no longer degrades to the literal indicator.
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
	lines := strings.Split(rest[:end], "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(strings.TrimRight(lines[i], "\r"))
		switch {
		case strings.HasPrefix(line, "name:"):
			name = unquoteScalar(strings.TrimSpace(strings.TrimPrefix(line, "name:")))
		case strings.HasPrefix(line, "description:"):
			value := strings.TrimSpace(strings.TrimPrefix(line, "description:"))
			if isBlockScalarHeader(value) {
				var last int
				description, last = readBlockScalar(lines, i+1, value)
				i = last
				continue
			}
			description = unquoteScalar(value)
		}
	}
	return name, description
}

// isBlockScalarHeader reports whether a YAML scalar value is a block scalar
// indicator: ">" (folded) or "|" (literal), optionally followed by a chomping
// or indentation modifier such as "-", "+" or a digit.
func isBlockScalarHeader(value string) bool {
	if value == "" || (value[0] != '>' && value[0] != '|') {
		return false
	}
	for _, r := range value[1:] {
		if r != '+' && r != '-' && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

// readBlockScalar consumes the body of a block scalar: the lines after a
// "key: >-"-style header, which must be indented deeper than the key. It returns
// the resulting value and the index of the last consumed line. A folded header
// (">") joins lines with a space (a blank line becomes a line break), a literal
// header ("|") keeps the line breaks. The common block indentation is removed
// and the result trimmed, so the chomping modifier needs no extra handling for
// a one-line description.
func readBlockScalar(lines []string, start int, header string) (string, int) {
	end, indent := start, -1
	for end < len(lines) {
		line := strings.TrimRight(lines[end], "\r")
		if strings.TrimSpace(line) == "" {
			end++
			continue
		}
		lead := len(line) - len(strings.TrimLeft(line, " \t"))
		if lead == 0 {
			break // a new top-level key ends the block
		}
		if indent < 0 || lead < indent {
			indent = lead
		}
		end++
	}
	if indent < 0 {
		return "", start - 1
	}
	parts := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		line := strings.TrimRight(lines[i], "\r")
		if strings.TrimSpace(line) == "" {
			parts = append(parts, "")
			continue
		}
		if len(line) >= indent {
			line = line[indent:]
		}
		parts = append(parts, strings.TrimRight(line, " \t"))
	}
	var value string
	if strings.HasPrefix(header, ">") {
		var b strings.Builder
		blank := true
		for _, p := range parts {
			if p == "" {
				b.WriteString("\n")
				blank = true
				continue
			}
			if !blank {
				b.WriteString(" ")
			}
			b.WriteString(p)
			blank = false
		}
		value = b.String()
	} else {
		value = strings.Join(parts, "\n")
	}
	return strings.TrimSpace(value), end - 1
}

// unquoteScalar strips a pair of matching surrounding quotes from a plain
// (single-line) YAML scalar.
func unquoteScalar(value string) string {
	if len(value) >= 2 {
		first, last := value[0], value[len(value)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}

// filterEnabledSkills drops the skills the user disabled in the settings
// (FEATURE-583, D-D): a disabled skill is invisible to the model and cannot be
// loaded. A nil predicate keeps every skill.
func filterEnabledSkills(skills []Skill, enabled func(string) bool) []Skill {
	if enabled == nil {
		return skills
	}
	out := make([]Skill, 0, len(skills))
	for _, s := range skills {
		if enabled(s.Name) {
			out = append(out, s)
		}
	}
	return out
}

// buildSkillsEnvBlock renders the <skills> element injected into the NEWEST
// <environment_details> of a request (FEATURE-583). It lists one line per
// available skill — name + description, with the loaded marker in front of the
// skills whose full text is already pinned — and skips disabled skills. It
// returns "" when no skill is available.
//
// The block is built per request and injected only into the last envelope (see
// injectSkillsEnvToLast), never frozen into a message: loading or unloading a
// skill then changes the tail of the request only, so the system prompt and the
// earlier history stay byte-identical and keep their prompt-prefix cache.
func (a *Agent) buildSkillsEnvBlock() string {
	ws, home := a.skillScanDirs()
	skills := ScanSkills(ws, home)
	if a.cfg != nil {
		skills = filterEnabledSkills(skills, a.cfg.SkillEnabled)
	}
	if len(skills) == 0 {
		return ""
	}
	loaded := a.loadedSkillSet()
	tag := i18n.T(i18n.KeySystemPromptSkillsLoadedTag)
	if tag == "" || tag == i18n.KeySystemPromptSkillsLoadedTag {
		tag = "[loaded]"
	}
	var sb strings.Builder
	sb.WriteString("<skills>")
	for _, s := range skills {
		sb.WriteString("\n- ")
		if loaded[s.Name] {
			sb.WriteString(tag)
			sb.WriteString(" ")
		}
		sb.WriteString(s.Name)
		sb.WriteString(": ")
		sb.WriteString(s.Description)
	}
	sb.WriteString("\n</skills>")
	return sb.String()
}

// injectSkillsEnvToLast injects the <skills> block into the envelope of the
// newest message carrying an <environment_details> block, so the
// available-skill list appears exactly once per request — in its last envelope
// (FEATURE-583).
func (a *Agent) injectSkillsEnvToLast(msgs []llm.Message) {
	block := a.buildSkillsEnvBlock()
	if block == "" {
		return
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		if injectEnvBlock(&msgs[i], block) {
			return
		}
	}
}

// injectEnvBlock inserts block just before the closing tag of the message's
// envelope and reports whether one was found. The content parts are copied
// first: the message values share their backing array with the stored history,
// which must stay untouched.
func injectEnvBlock(msg *llm.Message, block string) bool {
	const closeTag = "</environment_details>"
	for i := len(msg.ContentParts) - 1; i >= 0; i-- {
		idx := strings.LastIndex(msg.ContentParts[i].Text, closeTag)
		if idx < 0 {
			continue
		}
		parts := make([]llm.ContentPart, len(msg.ContentParts))
		copy(parts, msg.ContentParts)
		parts[i].Text = parts[i].Text[:idx] + block + "\n" + parts[i].Text[idx:]
		msg.ContentParts = parts
		return true
	}
	if idx := strings.LastIndex(msg.Content, closeTag); idx >= 0 {
		msg.Content = msg.Content[:idx] + block + "\n" + msg.Content[idx:]
		return true
	}
	return false
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
