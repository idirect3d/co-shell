// Package config - workspace tree exclusion list (FEATURE-553).
//
// The Web UI workspace tree (GET /api/tree) hides the folders listed in
// Config.TreeExcludes. An entry without a slash matches a folder NAME at any
// depth ("db" hides every db folder); an entry with a slash matches that
// workspace-relative PATH exactly ("docs/private"). Only folders are matched,
// so a file sharing a listed name stays visible.
//
// nil means "field absent from config.json" and falls back to
// DefaultTreeExcludes; an explicitly empty list means "hide nothing".
//
// Author: L.Shuang
// Created: 2026-09-25
// Last Modified: 2026-09-25
// MIT License - Copyright (c) 2026 L.Shuang

package config

import "strings"

// DefaultTreeExcludes is the built-in exclusion list: the local bbolt store
// and the git metadata directory. It applies only when a config has no
// tree_excludes field (nil).
var DefaultTreeExcludes = []string{"db", ".git"}

// normalizeTreeExclude cleans one entry: surrounding whitespace and slashes are
// trimmed, so "/docs/private/" and "docs/private" are the same entry. It
// returns "" when the entry is unusable (empty, containing a "." / ".."
// segment or a backslash).
func normalizeTreeExclude(entry string) string {
	cleaned := strings.Trim(strings.TrimSpace(entry), "/")
	if cleaned == "" || strings.Contains(cleaned, "\\") {
		return ""
	}
	for _, seg := range strings.Split(cleaned, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return ""
		}
	}
	return cleaned
}

// NormalizeTreeExcludes cleans and de-duplicates an exclusion list, preserving
// the user's order. rejected holds the entries that were dropped (invalid or
// duplicated) so callers can report them: the config loader ignores them, the
// settings panel rejects the save.
func NormalizeTreeExcludes(list []string) (cleaned []string, rejected []string) {
	seen := make(map[string]bool, len(list))
	cleaned = make([]string, 0, len(list))
	for _, entry := range list {
		item := normalizeTreeExclude(entry)
		if item == "" || seen[item] {
			rejected = append(rejected, entry)
			continue
		}
		seen[item] = true
		cleaned = append(cleaned, item)
	}
	return cleaned, rejected
}

// EffectiveTreeExcludes returns the exclusion list to apply. A nil list (the
// field is absent from config.json) falls back to DefaultTreeExcludes; an
// empty but non-nil list means nothing is hidden.
func (c *Config) EffectiveTreeExcludes() []string {
	if c == nil || c.TreeExcludes == nil {
		return DefaultTreeExcludes
	}
	return c.TreeExcludes
}

// TreeExcludeSets compiles the effective list into a folder-NAME set (entries
// without a slash, matched at any depth) and a workspace-relative PATH set
// (entries with a slash, matched exactly). The Web server reads this per tree
// request, so a settings change takes effect without a restart.
func (c *Config) TreeExcludeSets() (names map[string]bool, paths map[string]bool) {
	names = map[string]bool{}
	paths = map[string]bool{}
	for _, entry := range c.EffectiveTreeExcludes() {
		if strings.Contains(entry, "/") {
			paths[entry] = true
			continue
		}
		names[entry] = true
	}
	return names, paths
}
