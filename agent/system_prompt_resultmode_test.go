// Author: L.Shuang
// Created: 2026-09-05
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
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
)

// FEATURE-472: ResultMode section staticization tests (UC-0001 ~ UC-0006).

// TestBuildResultModeSection_DefaultModes verifies the default three modes
// produce a static RESULT MODE section with the expected title, lead sentence
// and per-mode descriptions (UC-0001).
func TestBuildResultModeSection_DefaultModes(t *testing.T) {
	i18n.Init("en")
	cfg := config.DefaultConfig()
	cfg.WorkModes = nil // no user-defined modes
	cfg.LLM.WorkMode = "act"

	got := buildResultModeSection(cfg)
	if got == "" {
		t.Fatal("buildResultModeSection returned empty for default modes")
	}
	for _, want := range []string{
		"ACT MODE V.S. PLAN MODE V.S. RESEARCH MODE",
		"There are 3 modes:",
		"# ACT MODE",
		"# PLAN MODE",
		"# RESEARCH MODE",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("result mode section missing %q:\n%s", want, got)
		}
	}
	// Order: act before plan before research.
	actIdx := strings.Index(got, "# ACT MODE")
	planIdx := strings.Index(got, "# PLAN MODE")
	resIdx := strings.Index(got, "# RESEARCH MODE")
	if !(actIdx >= 0 && actIdx < planIdx && planIdx < resIdx) {
		t.Errorf("mode order wrong (act=%d plan=%d research=%d)", actIdx, planIdx, resIdx)
	}
}

// TestBuildResultModeSection_TrailingNote verifies the section ends with the
// --unload-mode note in both languages (FEATURE-503, UC-0001/UC-0002).
func TestBuildResultModeSection_TrailingNote(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		t.Run(lang, func(t *testing.T) {
			i18n.Init(lang)
			cfg := config.DefaultConfig()
			cfg.WorkModes = nil
			cfg.LLM.WorkMode = "act"

			got := buildResultModeSection(cfg)
			const note = "--unload-mode {mode}"
			if !strings.Contains(got, note) {
				t.Errorf("result mode section missing trailing note %q:\n%s", note, got)
			}
			// The note must come after the last mode description.
			noteIdx := strings.Index(got, note)
			resIdx := strings.Index(got, "# RESEARCH MODE")
			if !(resIdx >= 0 && resIdx < noteIdx) {
				t.Errorf("trailing note not after mode descriptions (research=%d note=%d)", resIdx, noteIdx)
			}
			// The note must be wrapped in parentheses.
			if !strings.Contains(got, "（") && !strings.Contains(got, "(") {
				t.Errorf("trailing note is not parenthesized:\n%s", got)
			}
		})
	}
}

// TestBuildResultModeSection_CustomMode verifies a user-defined mode is
// appended to the section (UC-0002).
func TestBuildResultModeSection_CustomMode(t *testing.T) {
	i18n.Init("en")
	cfg := config.DefaultConfig()
	cfg.WorkModes = []config.WorkMode{
		{Name: "review", Description: "Review mode - review code"},
	}
	cfg.LLM.WorkMode = "act"

	got := buildResultModeSection(cfg)
	for _, want := range []string{
		"ACT MODE V.S. PLAN MODE V.S. RESEARCH MODE V.S. REVIEW MODE",
		"There are 4 modes:",
		"# REVIEW MODE",
		"Review mode - review code",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("result mode section missing %q:\n%s", want, got)
		}
	}
}

// TestBuildResultModeSection_CustomOverridesBuiltin verifies a user-defined
// mode with the same name as a built-in overrides it without duplication
// (UC-0003).
func TestBuildResultModeSection_CustomOverridesBuiltin(t *testing.T) {
	i18n.Init("en")
	cfg := config.DefaultConfig()
	cfg.WorkModes = []config.WorkMode{
		{Name: "act", Description: "Custom act override"},
	}
	cfg.LLM.WorkMode = "act"

	got := buildResultModeSection(cfg)
	// act must not be duplicated in the title (title lists each mode once).
	if strings.Count(got, "ACT MODE V.S.") != 1 {
		t.Errorf("ACT MODE should appear once in the title, got %d:\n%s",
			strings.Count(got, "ACT MODE V.S."), got)
	}
	// The list entry must use the custom description, not the built-in one.
	if !strings.Contains(got, "# ACT MODE\n\nCustom act override") {
		t.Errorf("custom act description not used:\n%s", got)
	}
	if strings.Contains(got, "# ACT MODE\n\nIn this mode") {
		t.Errorf("built-in act description should be overridden:\n%s", got)
	}
}

// TestBuildResultModeSection_EmptyDescriptionFallback verifies a mode whose
// detailed i18n key is empty falls back to its WorkMode.Description (UC-0004).
func TestBuildResultModeSection_EmptyDescriptionFallback(t *testing.T) {
	i18n.Init("en")
	cfg := config.DefaultConfig()
	cfg.WorkModes = []config.WorkMode{
		{Name: "custom", Description: "Custom mode desc"},
	}
	cfg.LLM.WorkMode = "act"

	got := buildResultModeSection(cfg)
	if !strings.Contains(got, "# CUSTOM MODE\n\nCustom mode desc") {
		t.Errorf("custom mode description missing:\n%s", got)
	}
	// Every listed mode must carry a non-empty description: a heading must not
	// be immediately followed by another heading (which would mean empty body).
	lines := strings.Split(got, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "# ") {
			continue
		}
		// Look ahead past blank lines for the next non-blank line.
		j := i + 1
		for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
			j++
		}
		if j < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[j]), "# ") {
			t.Errorf("mode heading %q has empty description (next is another heading)", line)
		}
	}
}

// TestWorkModeResourcesFilled verifies the detailed mode description resources
// are non-empty in both zh and en (UC-0005).
func TestWorkModeResourcesFilled(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		i18n.Init(lang)
		for _, key := range []string{
			i18n.KeyWorkModeAct,
			i18n.KeyWorkModePlan,
			i18n.KeyWorkModeResearch,
		} {
			if v := i18n.T(key); v == "" || v == key {
				t.Errorf("[%s] mode description resource %q is empty", lang, key)
			}
		}
	}
}

// TestResultModeSectionStaticAcrossModes verifies the RESULT MODE section is
// identical regardless of the currently active mode (UC-0006).
func TestResultModeSectionStaticAcrossModes(t *testing.T) {
	i18n.Init("en")
	cfg := config.DefaultConfig()
	cfg.WorkModes = nil
	cfg.LLM.WorkMode = "act"
	base := buildResultModeSection(cfg)
	if base == "" {
		t.Fatal("base result mode section empty")
	}
	for _, mode := range []string{"plan", "research"} {
		cfg.LLM.WorkMode = mode
		if got := buildResultModeSection(cfg); got != base {
			t.Errorf("result mode section differs in %s mode:\nbase:\n%s\n%s:\n%s", mode, base, mode, got)
		}
	}
}

// TestRulesContextThresholdInjected verifies the RULES section carries the
// {CONTEXT_REORGANIZE_THRESHOLD} placeholder and that buildSectionWithPlaceholders
// replaces it with the configured value (FEATURE-472 optimization).
func TestRulesContextThresholdInjected(t *testing.T) {
	i18n.Init("en")
	cfg := config.DefaultConfig()
	cfg.LLM.ContextReorganizeThreshold = 80

	// The RULES i18n resource must reference the placeholder.
	rulesText := i18n.T(i18n.KeySystemPromptRules)
	if !strings.Contains(rulesText, "{CONTEXT_REORGANIZE_THRESHOLD}") {
		t.Errorf("RULES resource missing {CONTEXT_REORGANIZE_THRESHOLD} placeholder")
	}

	// buildSectionWithPlaceholders must substitute the configured value.
	env := &promptEnv{contextReorganizeThreshold: "80"}
	got := buildSectionWithPlaceholders(rulesText, env)
	if strings.Contains(got, "{CONTEXT_REORGANIZE_THRESHOLD}") {
		t.Errorf("placeholder not substituted:\n%s", got)
	}
	if !strings.Contains(got, "approaches 80%") {
		t.Errorf("threshold value 80 not injected:\n%s", got)
	}
}

// TestModeDescriptionsNoPlanModeRespond verifies the mode descriptions no longer
// reference the non-existent plan_mode_respond tool (FEATURE-472 optimization).
func TestModeDescriptionsNoPlanModeRespond(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		i18n.Init(lang)
		for _, key := range []string{
			i18n.KeyWorkModeAct,
			i18n.KeyWorkModePlan,
			i18n.KeyWorkModeResearch,
		} {
			if v := i18n.T(key); strings.Contains(v, "plan_mode_respond") {
				t.Errorf("[%s] mode description %q still references plan_mode_respond", lang, key)
			}
		}
	}
}

// TestResearchRulesMovedFromGlobalRules verifies the research-specific rules were
// removed from the global RULES section (they now live in RESEARCH MODE).
func TestResearchRulesMovedFromGlobalRules(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		i18n.Init(lang)
		rulesText := i18n.T(i18n.KeySystemPromptRules)
		for _, forbidden := range []string{"pdf2png.py", "GB/T 7714", "research/", "做调查研究"} {
			if strings.Contains(rulesText, forbidden) {
				t.Errorf("[%s] global RULES still contains research-specific %q", lang, forbidden)
			}
		}
	}
}

// TestDefaultSectionsStaticAcrossModes verifies the three built-in modes share
// the same default section list (all include ResultMode) so the system prompt
// stays static across mode switches (FEATURE-472 staticization).
func TestDefaultSectionsStaticAcrossModes(t *testing.T) {
	i18n.Init("en")
	act := config.DefaultActSections()
	plan := config.DefaultPlanSections()
	research := config.DefaultResearchSections()
	if len(act) != len(plan) || len(act) != len(research) {
		t.Fatalf("section list lengths differ: act=%d plan=%d research=%d", len(act), len(plan), len(research))
	}
	for i := range act {
		if act[i] != plan[i] || act[i] != research[i] {
			t.Errorf("section %d differs: act=%q plan=%q research=%q", i, act[i], plan[i], research[i])
		}
	}
	// ResultMode must be present in the shared default list.
	found := false
	for _, s := range act {
		if s == "ResultMode" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("default section list missing ResultMode: %v", act)
	}
}

// TestAgentDescriptionDefaultUnified verifies the default agent description is
// no longer mode-specific (all modes fall back to the same global default).
func TestAgentDescriptionDefaultUnified(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		i18n.Init(lang)
		// The mode-specific default keys must be empty (removed).
		for _, key := range []string{
			i18n.KeyAgentDefaultDescriptionAct,
			i18n.KeyAgentDefaultDescriptionPlan,
			i18n.KeyAgentDefaultDescriptionResearch,
		} {
			if v := i18n.T(key); v != "" && v != key {
				t.Errorf("[%s] mode-specific default %q should be empty, got %q", lang, key, v)
			}
		}
		// The global default must be the unified general-purpose description.
		global := i18n.T(i18n.KeyAgentDefaultDescription)
		if global == "" || global == i18n.KeyAgentDefaultDescription {
			t.Errorf("[%s] global default description is empty", lang)
		}
	}
}
