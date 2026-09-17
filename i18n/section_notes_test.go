// Package i18n - per-section trailing note assertions (FEATURE-533).
//
// These tests pin WHERE each editable-file note lives in the system prompt:
// ./IDENTITY.md at the end of the Identity section, ./CAPABILITIES.md at the end
// of the CAPABILITIES text, ./RULES.md merged into the RULES section note, and
// ./mode/{MODE}.md alone in the ResultMode note.
//
// Author: L.Shuang
// MIT License - Copyright (c) 2026 L.Shuang

package i18n

import (
	"strings"
	"testing"
)

// TestPerSectionFileNotes verifies each section carries its own file note at the
// very end of the section text (UC-0001).
func TestPerSectionFileNotes(t *testing.T) {
	cases := []struct {
		name string
		lang Lang
		key  string
		// file is the editable file the note must point at.
		file string
		// lastContent is the last content line that must precede the note.
		lastContent string
	}{
		{
			name:        "zh identity",
			lang:        LangZH,
			key:         KeySystemPromptIdentity,
			file:        "./IDENTITY.md",
			lastContent: "{AGENT_PRINCIPLES}",
		},
		{
			name:        "en identity",
			lang:        LangEN,
			key:         KeySystemPromptIdentity,
			file:        "./IDENTITY.md",
			lastContent: "{AGENT_PRINCIPLES}",
		},
		{
			name:        "zh capabilities",
			lang:        LangZH,
			key:         KeySystemPromptCapabilities,
			file:        "./CAPABILITIES.md",
			lastContent: "4. **目标驱动**",
		},
		{
			name:        "en capabilities",
			lang:        LangEN,
			key:         KeySystemPromptCapabilities,
			file:        "./CAPABILITIES.md",
			lastContent: "4. **Goal-Driven**",
		},
		{
			name:        "zh rules",
			lang:        LangZH,
			key:         KeySystemPromptRules,
			file:        "./RULES.md",
			lastContent: "管理上下文窗口",
		},
		{
			name:        "en rules",
			lang:        LangEN,
			key:         KeySystemPromptRules,
			file:        "./RULES.md",
			lastContent: "Managing the context window",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			Init(string(tc.lang))
			text := T(tc.key)

			note := strings.LastIndex(text, tc.file)
			if note == -1 {
				t.Fatalf("[%s] section %s does not mention %s", tc.name, tc.key, tc.file)
			}
			if !strings.Contains(text[note:], "--init") {
				t.Errorf("[%s] %s note does not mention --init", tc.name, tc.file)
			}
			// The note belongs to this section, i.e. it must follow the last
			// content line (for Rules the note sits right before {CUSTOM_RULES}).
			anchor := strings.Index(text, tc.lastContent)
			if anchor == -1 {
				t.Fatalf("[%s] anchor %q missing", tc.name, tc.lastContent)
			}
			if note < anchor {
				t.Errorf("[%s] %s note must come after %q", tc.name, tc.file, tc.lastContent)
			}
		})
	}
}

// TestRulesNoteMergesRulesFile verifies the pre-existing .rules/ customization
// note was merged with the ./RULES.md hint instead of being duplicated
// (UC-0001).
func TestRulesNoteMergesRulesFile(t *testing.T) {
	for _, tc := range []struct {
		lang Lang
		note string
	}{
		{LangZH, "（可以通过向 .rules/ 下放规则文件的方式"},
		{LangEN, "(You can customize rules/specifications by placing rule files under .rules/"},
	} {
		Init(string(tc.lang))
		text := T(KeySystemPromptRules)

		start := strings.Index(text, tc.note)
		if start == -1 {
			t.Fatalf("[%s] .rules/ note not found", tc.lang)
		}
		if !strings.Contains(text[start:], "./RULES.md") || !strings.Contains(text[start:], "--init") {
			t.Errorf("[%s] RULES note must mention ./RULES.md and --init", tc.lang)
		}
		if n := strings.Count(text, tc.note); n != 1 {
			t.Errorf("[%s] .rules/ note repeated %d times", tc.lang, n)
		}
	}
}

// TestResultModeNoteOnlyModeFile verifies the ResultMode note no longer lists
// the other three editable files; they now live in their own sections
// (UC-0001).
func TestResultModeNoteOnlyModeFile(t *testing.T) {
	for _, lang := range []Lang{LangZH, LangEN} {
		Init(string(lang))
		note := T(KeySystemPromptResultModeNote)

		if !strings.Contains(note, "./mode/{MODE}.md") || !strings.Contains(note, "--init") {
			t.Errorf("[%s] ResultMode note must mention ./mode/{MODE}.md and --init, got %q", lang, note)
		}
		for _, other := range []string{"./IDENTITY.md", "./CAPABILITIES.md", "./RULES.md"} {
			if strings.Contains(note, other) {
				t.Errorf("[%s] ResultMode note must not mention %s, got %q", lang, other, note)
			}
		}
	}
}
