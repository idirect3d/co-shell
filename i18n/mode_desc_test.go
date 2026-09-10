// Package i18n - work-mode description assertions (FEATURE-502).
//
// These tests pin the behavioral discipline promised by the PLAN and RESEARCH
// mode system prompts, so the wording cannot silently regress.

package i18n

import (
	"strings"
	"testing"
)

// TestWorkModePlanDescRequirementDiscovery verifies UC-001 ~ UC-003: the PLAN
// MODE description stresses uncovering the user's real requirements, repeated
// confirmation of ambiguities, and never deciding for the user by guessing.
func TestWorkModePlanDescRequirementDiscovery(t *testing.T) {
	cases := []struct {
		name     string
		lang     Lang
		mustHave []string
	}{
		{
			name: "zh",
			lang: LangZH,
			mustHave: []string{
				"挖掘用户的真实需求",
				"反复用 ask_followup_question 与用户确认",
				"不要仅凭猜测替用户做决定",
			},
		},
		{
			name: "en",
			lang: LangEN,
			mustHave: []string{
				"uncovering the user's real requirements",
				"repeatedly confirm with the user via the ask_followup_question tool",
				"Do NOT make decisions on the user's behalf based on guesswork",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			Init(string(tc.lang))
			got := T(KeyWorkModePlan)
			for _, want := range tc.mustHave {
				if !strings.Contains(got, want) {
					t.Errorf("PLAN MODE description (%s) missing %q\ngot:\n%s", tc.name, want, got)
				}
			}
		})
	}
}

// TestWorkModeResearchDescHighConfidenceEvidence verifies UC-004 ~ UC-006: the
// RESEARCH MODE description requires high-confidence evidence for every
// conclusion, forbids fabrication, and lists curl as an internet research tool.
func TestWorkModeResearchDescHighConfidenceEvidence(t *testing.T) {
	cases := []struct {
		name     string
		lang     Lang
		mustHave []string
	}{
		{
			name: "zh",
			lang: LangZH,
			mustHave: []string{
				"或浏览器与curl在互联网开展调研",
				"所有结论都必须有高置信度证据支撑",
				"严禁凭空想像",
			},
		},
		{
			name: "en",
			lang: LangEN,
			mustHave: []string{
				"or the browser and curl to conduct research on the internet",
				"Every conclusion MUST be supported by high-confidence evidence",
				"NEVER fabricate",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			Init(string(tc.lang))
			got := T(KeyWorkModeResearch)
			for _, want := range tc.mustHave {
				if !strings.Contains(got, want) {
					t.Errorf("RESEARCH MODE description (%s) missing %q\ngot:\n%s", tc.name, want, got)
				}
			}
		})
	}
}

// TestWorkModeDescriptionsRenderNonEmpty verifies UC-007: both mode
// descriptions resolve to non-empty, placeholder-free text in both languages.
func TestWorkModeDescriptionsRenderNonEmpty(t *testing.T) {
	for _, lang := range []Lang{LangZH, LangEN} {
		Init(string(lang))
		for _, key := range []string{KeyWorkModePlan, KeyWorkModeResearch} {
			got := T(key)
			if strings.TrimSpace(got) == "" {
				t.Errorf("T(%q) with lang=%s is empty", key, lang)
			}
			if got == key {
				t.Errorf("T(%q) with lang=%s returned the key itself (missing translation)", key, lang)
			}
		}
	}
}
