// Author: L.Shuang
// Created: 2026-10-02
// Last Modified: 2026-10-02
//
// MIT License - Copyright (c) 2026 L.Shuang
//
// FIX-588: the SKILLS section now demands the "unload as soon as it is no longer
// needed" discipline (task done / by-product consumed / later steps no longer
// depend on it) plus a pre-delivery self-check; the same guidance is carried by
// the skill_load / skill_unload tool descriptions, the tool usage blocks and the
// pinned note emitted by skill_load.

package agent

import (
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/i18n"
)

// fix588UrgencyWords returns the words that make an unload instruction urgent in
// the given language - an instruction without any of them is only a passive
// option, which is exactly what FIX-588 removes.
func fix588UrgencyWords(lang string) []string {
	if lang == "zh" {
		return []string{"立即", "及时", "用完即卸"}
	}
	return []string{"as soon as", "when done", "promptly"}
}

// fix588ContainsAny reports whether s contains any of the given substrings.
func fix588ContainsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// fix588UnloadRuleLine returns the SKILLS rule line that talks about skill_unload.
func fix588UnloadRuleLine(section string) string {
	for _, ln := range strings.Split(section, "\n") {
		if strings.Contains(ln, "skill_unload") {
			return strings.TrimSpace(ln)
		}
	}
	return ""
}

// TestFIX588SkillsSectionDemandsTimelyUnload locks the new SKILLS rule: unloading
// must be demanded ("as soon as it is no longer needed"), not merely offered.
func TestFIX588SkillsSectionDemandsTimelyUnload(t *testing.T) {
	cases := []struct {
		lang     string
		mustHave []string
		banned   []string
	}{
		{
			lang: "zh",
			mustHave: []string{
				"一旦不再需要就应立即调用 skill_unload",
				"用完即卸",
				"任务已完成",
				"中间产物",
				"交付前自检",
			},
			banned: []string{"- 卸载：不再需要时调用 skill_unload"},
		},
		{
			lang: "en",
			mustHave: []string{
				"call skill_unload (parameter name) as soon as you no longer need it",
				"unload as soon as it is no longer needed",
				"task is done",
				"by-product",
				"Self-check before delivering",
			},
			banned: []string{"- Unload: call skill_unload (parameter name) to release it when it is no longer needed"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.lang, func(t *testing.T) {
			i18n.Init(tc.lang)
			section := i18n.T(i18n.KeySystemPromptSkills)

			for _, want := range tc.mustHave {
				if !strings.Contains(section, want) {
					t.Errorf("[%s] SKILLS section must state %q", tc.lang, want)
				}
			}
			for _, bad := range tc.banned {
				if strings.Contains(section, bad) {
					t.Errorf("[%s] SKILLS section still carries the old passive wording %q", tc.lang, bad)
				}
			}

			// The customization note must stay the trailing paragraph.
			trimmed := strings.TrimSpace(section)
			if !strings.HasSuffix(trimmed, "）") && !strings.HasSuffix(trimmed, ")") {
				t.Errorf("[%s] SKILLS section must still end with the customization note", tc.lang)
			}

			line := fix588UnloadRuleLine(trimmed)
			if line == "" {
				t.Fatalf("[%s] no unload rule line found", tc.lang)
			}
			if !fix588ContainsAny(line, fix588UrgencyWords(tc.lang)) {
				t.Errorf("[%s] unload rule must be urgent, got %q", tc.lang, line)
			}
			t.Logf("FIX588_SKILLS|%s=%q", tc.lang, line)
		})
	}
}

// TestFIX588SkillTextsCarryUnloadGuidance verifies the guidance reaches every
// text the model reads about skills: both skill tool descriptions, both tool
// usage blocks and the pinned note.
func TestFIX588SkillTextsCarryUnloadGuidance(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		t.Run(lang, func(t *testing.T) {
			i18n.Init(lang)
			texts := []struct {
				name string
				key  string
			}{
				{"tool_load_desc", i18n.KeySkillToolLoadDesc},
				{"tool_unload_desc", i18n.KeySkillToolUnloadDesc},
				{"usage_load", i18n.KeyToolUsageSkillLoad},
				{"usage_unload", i18n.KeyToolUsageSkillUnload},
				{"pinned_note", i18n.KeySkillLoadPinnedNote},
			}

			for _, item := range texts {
				text := i18n.T(item.key)
				if !strings.Contains(text, "skill_unload") {
					t.Errorf("[%s] %s must mention skill_unload, got %q", lang, item.name, text)
					continue
				}
				if !fix588ContainsAny(text, fix588UrgencyWords(lang)) {
					t.Errorf("[%s] %s must carry an urgent unload hint, got %q", lang, item.name, text)
				}
				t.Logf("FIX588_TEXT|%s|%s|%q", lang, item.name, text)
			}
		})
	}
}
