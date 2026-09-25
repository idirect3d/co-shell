// Package config - workspace tree exclusion list tests (FEATURE-553): default
// list, nil (absent field) vs empty list, cleaning and de-duplication.
//
// Author: L.Shuang
// Created: 2026-09-25
// Last Modified: 2026-09-25
// MIT License - Copyright (c) 2026 L.Shuang

package config

import (
	"reflect"
	"testing"
)

// TestDefaultTreeExcludes verifies the built-in list is db + .git.
func TestDefaultTreeExcludes(t *testing.T) {
	if !reflect.DeepEqual(DefaultTreeExcludes, []string{"db", ".git"}) {
		t.Fatalf("DefaultTreeExcludes = %v, want [db .git]", DefaultTreeExcludes)
	}
}

// TestEffectiveTreeExcludes verifies nil falls back to the default while an
// explicit empty list means "hide nothing".
func TestEffectiveTreeExcludes(t *testing.T) {
	nilCfg := &Config{}
	if got := nilCfg.EffectiveTreeExcludes(); !reflect.DeepEqual(got, DefaultTreeExcludes) {
		t.Errorf("nil list = %v, want the default list", got)
	}
	emptyCfg := &Config{TreeExcludes: []string{}}
	if got := emptyCfg.EffectiveTreeExcludes(); len(got) != 0 {
		t.Errorf("empty list = %v, want nothing hidden", got)
	}
}

// TestNormalizeTreeExcludes verifies cleaning (padding and surrounding slashes
// are trimmed, so "/.git/" becomes ".git"), de-duplication, and the entries
// reported as rejected.
func TestNormalizeTreeExcludes(t *testing.T) {
	cleaned, rejected := NormalizeTreeExcludes([]string{
		" db ", "/.git/", "docs/private", "docs/private", "", "../escape", "a/../b",
	})
	if !reflect.DeepEqual(cleaned, []string{"db", ".git", "docs/private"}) {
		t.Errorf("cleaned = %v", cleaned)
	}
	if len(rejected) != 4 {
		t.Errorf("rejected = %v, want 4 entries (dup, empty, ../escape, a/../b)", rejected)
	}
}

// TestTreeExcludeSets verifies the compiled sets: names without a slash, paths
// with a slash.
func TestTreeExcludeSets(t *testing.T) {
	cfg := &Config{TreeExcludes: []string{"db", "docs/private"}}
	names, paths := cfg.TreeExcludeSets()
	if !names["db"] || len(names) != 1 {
		t.Errorf("names = %v, want only db", names)
	}
	if !paths["docs/private"] || len(paths) != 1 {
		t.Errorf("paths = %v, want only docs/private", paths)
	}
}
