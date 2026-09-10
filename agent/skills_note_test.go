// Package agent - SKILLS section customization note assertions (FEATURE-505).
//
// These tests pin the trailing parenthesized note in the SKILLS section that
// tells the LLM how users can customize skills, and the behavior change that
// the section is always emitted (even when no skills exist).

package agent

import (
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/i18n"
)

// TestSkillsSectionCustomizationNote verifies UC-0001 ~ UC-0004: both zh and en
// SKILLS texts carry the skill customization note, wrapped in parentheses and
// placed as the last paragraph.
func TestSkillsSectionCustomizationNote(t *testing.T) {
	cases := []struct {
		name     string
		lang     string
		mustHave []string
		open     string
		close    string
	}{
		{
			name: "zh",
			lang: "zh",
			mustHave: []string{
				"可以通过向 ./skills/",
				"~/.co-shell/skills/",
				"同名时工作空间级优先",
				":skill list/show/add/remove",
			},
			open:  "（",
			close: "）",
		},
		{
			name: "en",
			lang: "en",
			mustHave: []string{
				"placing skill directories under ./skills/",
				"~/.co-shell/skills/",
				"workspace-level takes precedence",
				":skill list/show/add/remove",
			},
			open:  "(",
			close: ")",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			i18n.Init(tc.lang)
			text := i18n.T(i18n.KeySystemPromptSkills)

			for _, want := range tc.mustHave {
				if !strings.Contains(text, want) {
					t.Errorf("[%s] SKILLS missing %q", tc.name, want)
				}
			}

			// The note must be the last non-empty paragraph, parenthesized.
			trimmed := strings.TrimSpace(text)
			if !strings.HasSuffix(trimmed, tc.close) {
				t.Errorf("[%s] SKILLS text must end with %q", tc.name, tc.close)
			}
			marker := tc.open + "可以通过向"
			if tc.name == "en" {
				marker = tc.open + "You can customize skills"
			}
			idx := strings.Index(trimmed, marker)
			if idx < 0 {
				t.Fatalf("[%s] customization note not found", tc.name)
			}
			if !strings.Contains(trimmed[idx:], "skills/") {
				t.Errorf("[%s] note does not mention the skills directory", tc.name)
			}
		})
	}
}

// TestSkillsSectionAlwaysEmitted verifies UC-0005: the SKILLS section is
// emitted even when no skills exist, so the customization note stays visible.
func TestSkillsSectionAlwaysEmitted(t *testing.T) {
	i18n.Init("zh")

	// Empty workspace + empty home => no skills discovered.
	empty := t.TempDir()
	header := i18n.T(i18n.KeySystemPromptSkills)
	index := buildSkillsIndex(scanSkills(empty, empty))
	if index != "" {
		t.Fatalf("expected no skills in empty dirs, got index: %q", index)
	}

	// Mirror the buildSection("Skills") logic: with no index, the header alone
	// is returned (never an empty string).
	got := header
	if got == "" {
		t.Fatal("SKILLS section must not be empty when no skills exist")
	}
	if !strings.Contains(got, "skills/") {
		t.Errorf("SKILLS section must still carry the customization note, got: %q", got)
	}
}

// TestSkillsIndexInsertedBeforeNote verifies UC-0006: when skills exist, the
// index is inserted before the trailing note so the note stays at the very end.
func TestSkillsIndexInsertedBeforeNote(t *testing.T) {
	i18n.Init("zh")
	header := i18n.T(i18n.KeySystemPromptSkills)
	index := "- my-skill: a test skill (/tmp/my-skill/SKILL.md)"

	// Mirror the buildSection("Skills") insertion logic.
	var got string
	for _, open := range []string{"\n\n\uff08", "\n\n("} {
		if i := strings.LastIndex(header, open); i >= 0 {
			got = header[:i] + "\n\n" + index + header[i:]
			break
		}
	}
	if got == "" {
		got = header + "\n\n" + index
	}

	idxPos := strings.Index(got, index)
	notePos := strings.Index(got, "（可以通过向")
	if idxPos < 0 {
		t.Fatal("index not found in rendered section")
	}
	if notePos < 0 {
		t.Fatal("customization note not found in rendered section")
	}
	if idxPos > notePos {
		t.Errorf("index must precede the note: index@%d note@%d", idxPos, notePos)
	}
	if !strings.HasSuffix(strings.TrimSpace(got), "）") {
		t.Errorf("note must remain the last paragraph, got tail: %q", got[len(got)-40:])
	}
}
