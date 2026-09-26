// Author: L.Shuang
// Created: 2026-09-19
//
// MIT License
//
// Copyright (c) 2026 L.Shuang

package gateway

// FEATURE-538: the hub no longer stores an agent's "职责描述" in its own
// registry. An agent's description lives in {workspace}/IDENTITY.md — the file
// co-shell edits under "身份与个性" — so for managed agents the hub reads and
// writes that file instead. The hub is a separate Go module and cannot import
// the root module's agent package, so this file implements the minimal
// IDENTITY.md handling the hub needs.
//
// The sections this file understands:
//
//	## agent-name
//	work
//
//	## agent-description
//	你是一个严谨、务实、目标驱动的通用智能体。
//
// Only `## agent-description` is read/written here: every other line (other
// sections such as agent-name / user-name, plus free-form text) is preserved
// verbatim.
//
// Author: L.Shuang
// MIT License - Copyright (c) 2026 L.Shuang

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// IdentityFileName is the workspace-root file holding the agent identity.
const IdentityFileName = "IDENTITY.md"

// IDENTITY.md section keys used by the hub.
const (
	identityDescriptionKey = "agent-description"
	identityNameKey        = "agent-name"
)

// IdentityPath returns the path of the IDENTITY.md file inside dir.
func IdentityPath(dir string) string {
	if dir == "" {
		return IdentityFileName
	}
	return filepath.Join(dir, IdentityFileName)
}

// ReadIdentityDescription returns the `agent-description` value of
// {dir}/IDENTITY.md. A missing file or a missing section yields "" (nothing
// configured yet), which the UI renders as "（未设置）".
func ReadIdentityDescription(dir string) string {
	v, err := readIdentityDescription(dir)
	if err != nil {
		return ""
	}
	return v
}

// readIdentityDescription is ReadIdentityDescription with the error surfaced
// (empty workspace or an unreadable file).
func readIdentityDescription(dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", fmt.Errorf("empty workspace")
	}
	data, err := os.ReadFile(IdentityPath(dir))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return identitySectionValue(string(data), identityDescriptionKey), nil
}

// WriteIdentityDescription stores value as the `## agent-description` section of
// {dir}/IDENTITY.md. dir must already exist (it is the agent's workspace); the
// file itself is created when absent, in which case `## agent-name` is
// initialised to agentID so co-shell has a name to start from. Other sections
// and free-form text are preserved verbatim.
func WriteIdentityDescription(dir, agentID, value string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return fmt.Errorf("cannot write %s: empty workspace", IdentityFileName)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return fmt.Errorf("workspace %q does not exist", dir)
	}
	path := IdentityPath(dir)
	data, err := os.ReadFile(path)
	text := string(data)
	switch {
	case os.IsNotExist(err):
		// First write into this workspace: give co-shell a name to fall back on.
		text = "## " + identityNameKey + "\n" + strings.TrimSpace(agentID) + "\n\n"
	case err != nil:
		return fmt.Errorf("read %s: %w", path, err)
	}
	text = writeIdentitySection(text, identityDescriptionKey, value)
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// WriteIdentityName stores name as the `## agent-name` section of
// {dir}/IDENTITY.md, creating the file when absent. FIX-558 uses it when the hub
// creates a managed agent: the agent's ID becomes its initial name, so the agent
// has something to identify itself with. Other sections and free-form text are
// preserved verbatim.
func WriteIdentityName(dir, name string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return fmt.Errorf("cannot write %s: empty workspace", IdentityFileName)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return fmt.Errorf("workspace %q does not exist", dir)
	}
	path := IdentityPath(dir)
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", path, err)
	}
	text := writeIdentitySection(string(data), identityNameKey, name)
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// identityLines splits file text into lines, tolerating CRLF and dropping the
// empty element produced by a trailing newline (so line indexes stay stable).
func identityLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	return lines
}

// identityHeadingKey normalizes a markdown heading line into its section key.
// It mirrors co-shell's parser: a "#"-prefixed single token is a key, while a
// heading containing spaces is free-form prose and never matches.
func identityHeadingKey(line string) (string, bool) {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, "#") {
		return "", false
	}
	t = strings.TrimLeft(t, "#")
	t = strings.TrimSpace(t)
	if t == "" || strings.ContainsAny(t, " \t") {
		return "", false
	}
	return strings.ToLower(t), true
}

// identitySectionBounds returns the body line range [start, end) of section key
// (end is the next heading line or the end of the file). ok reports whether the
// section heading exists.
func identitySectionBounds(lines []string, key string) (int, int, bool) {
	for i, line := range lines {
		if k, isHeading := identityHeadingKey(line); isHeading && k == key {
			end := len(lines)
			for j := i + 1; j < len(lines); j++ {
				if _, isH := identityHeadingKey(lines[j]); isH {
					end = j
					break
				}
			}
			return i + 1, end, true
		}
	}
	return 0, 0, false
}

// identitySectionValue returns the trimmed body text of section key ("" when the
// section is absent).
func identitySectionValue(text, key string) string {
	lines := identityLines(text)
	start, end, ok := identitySectionBounds(lines, key)
	if !ok {
		return ""
	}
	return strings.TrimSpace(strings.Join(lines[start:end], "\n"))
}

// identityValueLines renders value as body lines (a multi-line value is kept
// as-is; trailing blank lines are dropped).
func identityValueLines(value string) []string {
	v := strings.ReplaceAll(strings.TrimRight(value, " \t\r\n"), "\r\n", "\n")
	if v == "" {
		return nil
	}
	return strings.Split(v, "\n")
}

// trailingBlankLines returns the trailing blank lines of a section body; they
// are kept so the blank line separating two sections survives a rewrite.
func trailingBlankLines(body []string) []string {
	i := len(body)
	for i > 0 && strings.TrimSpace(body[i-1]) == "" {
		i--
	}
	return append([]string{}, body[i:]...)
}

// writeIdentitySection returns text with the body of section key replaced by
// value. A missing section is appended at the end (with a blank line
// separator); every other line is copied verbatim.
func writeIdentitySection(text, key, value string) string {
	lines := identityLines(text)
	valueLines := identityValueLines(value)
	start, end, ok := identitySectionBounds(lines, key)
	if !ok {
		for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
			lines = lines[:len(lines)-1]
		}
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, "## "+key)
		lines = append(lines, valueLines...)
		return joinIdentityLines(lines)
	}
	body := append(append([]string{}, valueLines...), trailingBlankLines(lines[start:end])...)
	out := make([]string, 0, len(lines))
	out = append(out, lines[:start]...)
	out = append(out, body...)
	out = append(out, lines[end:]...)
	return joinIdentityLines(out)
}

// joinIdentityLines renders lines back into file text (always newline-terminated).
func joinIdentityLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}
