// Package i18n - RULES section .rules/ customization note assertions (FEATURE-504).
//
// These tests pin the trailing parenthesized note in the RULES section that
// tells the LLM how users can customize rules via the .rules/ directory.

package i18n

import (
	"strings"
	"testing"
)

// TestRulesDirCustomizationNote verifies UC-0001 ~ UC-0005: both zh and en RULES
// texts carry the .rules/ customization note, wrapped in parentheses, placed
// after the last rule and before the {CUSTOM_RULES} placeholder.
func TestRulesDirCustomizationNote(t *testing.T) {
	cases := []struct {
		name     string
		lang     Lang
		mustHave []string
		open     string
		close    string
	}{
		{
			name: "zh",
			lang: LangZH,
			mustHave: []string{
				"可以通过向 .rules/ 下放规则文件的方式",
				"文件名将被当作各节标题",
				"子文件夹将被列出（作为索引），但不会再遍历子文件夹",
			},
			open:  "（",
			close: "）",
		},
		{
			name: "en",
			lang: LangEN,
			mustHave: []string{
				"placing rule files under .rules/",
				"file names are used as section titles",
				"subfolders are listed as an index but are not traversed further",
			},
			open:  "(",
			close: ")",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			Init(string(tc.lang))
			text := T(KeySystemPromptRules)

			for _, want := range tc.mustHave {
				if !strings.Contains(text, want) {
					t.Errorf("[%s] RULES missing %q", tc.name, want)
				}
			}

			// The note must be parenthesized.
			start := strings.Index(text, tc.open+"可以通过向 .rules/")
			if tc.name == "en" {
				start = strings.Index(text, tc.open+"You can customize rules")
			}
			if start == -1 {
				t.Fatalf("[%s] customization note not found", tc.name)
			}
			end := strings.Index(text[start:], tc.close)
			if end == -1 {
				t.Fatalf("[%s] customization note is not closed with %q", tc.name, tc.close)
			}

			// The note must precede the {CUSTOM_RULES} placeholder.
			placeholder := strings.Index(text, "{CUSTOM_RULES}")
			if placeholder == -1 {
				t.Fatalf("[%s] {CUSTOM_RULES} placeholder missing", tc.name)
			}
			if start > placeholder {
				t.Errorf("[%s] customization note must precede {CUSTOM_RULES}", tc.name)
			}

			// The note must come after the last rule (context window rule).
			lastRule := "管理上下文窗口"
			if tc.name == "en" {
				lastRule = "Managing the context window"
			}
			lastRuleIdx := strings.Index(text, lastRule)
			if lastRuleIdx == -1 {
				t.Fatalf("[%s] last rule %q missing", tc.name, lastRule)
			}
			if start < lastRuleIdx {
				t.Errorf("[%s] customization note must come after the last rule", tc.name)
			}
		})
	}
}
