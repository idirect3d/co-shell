// Author: L.Shuang
// Created: 2026-10-02
// Last Modified: 2026-10-02
//
// MIT License - Copyright (c) 2026 L.Shuang
//
// FIX-587: the skill_load header (and the Web UI loaded-skills row) show only
// the skill name plus a right-aligned source label again; the file path, the
// byte/token estimate and the sha256 prefix were dropped, and the pinned note
// line now carries the skill card description.

package agent

import (
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
)

// TestFIX587HeaderKeepsNameAndRightAlignsSource locks the new header contract:
// name + source label, with the label ending exactly at skillHeaderLabelColumn.
func TestFIX587HeaderKeepsNameAndRightAlignsSource(t *testing.T) {
	i18nInit()
	a := &Agent{cfg: config.DefaultConfig()}
	ls := LoadedSkill{
		Name:        "image-generate",
		Path:        "/tmp/ws/skills/image-generate/SKILL.md",
		Dir:         "/tmp/ws/skills/image-generate",
		Bytes:       3895,
		SHA256:      "7a611b09deadbeef",
		Source:      SkillSourceWorkspace,
		Description: "本地 AI 文生图（画图）能力",
	}

	out := a.skillLoadResultText(ls, "BODY")
	line := strings.SplitN(out, "\n", 2)[0]
	label := i18nT(i18n.KeySkillSrcWorkspace)
	if !strings.Contains(line, "SKILL LOADED: image-generate") {
		t.Fatalf("header must carry the skill name, got %q", line)
	}
	idx := strings.Index(line, label)
	if idx < 0 {
		t.Fatalf("header must carry the source label %q, got %q", label, line)
	}
	if end := displayWidth(line[:idx]) + displayWidth(label); end != skillHeaderLabelColumn {
		t.Errorf("source label must end at display column %d, got %d: %q", skillHeaderLabelColumn, end, line)
	}
	for _, banned := range []string{"sha256", "3895", "tokens", ls.Path, "KB"} {
		if strings.Contains(out, banned) {
			t.Errorf("FIX-587: result must not carry %q:\n%s", banned, out)
		}
	}

	// A global skill renders the global label instead (same alignment rule).
	ls.Source = SkillSourceGlobal
	global := a.skillLoadResultText(ls, "BODY")
	globalLine := strings.SplitN(global, "\n", 2)[0]
	if gLabel := i18nT(i18n.KeySkillSrcGlobal); !strings.Contains(globalLine, gLabel) {
		t.Errorf("global source must render %q, got %q", gLabel, globalLine)
	}
	t.Logf("FIX587_HEADER|ws=%q|global=%q", line, globalLine)
}

// TestFIX587NoteCarriesDescription verifies the pinned note line gains the skill
// description (and stays untouched when the skill has none).
func TestFIX587NoteCarriesDescription(t *testing.T) {
	i18nInit()
	a := &Agent{cfg: config.DefaultConfig()}
	const desc = "helper for demos"

	withDesc := LoadedSkill{Name: "demo", Bytes: 10, Source: SkillSourceWorkspace, Description: desc}
	out := a.skillLoadResultText(withDesc, "BODY")
	lines := strings.SplitN(out, "\n", 3)
	if len(lines) < 2 {
		t.Fatalf("result must have a header and a note line:\n%s", out)
	}
	if want := i18n.TF(i18n.KeySkillDescSuffix, desc); !strings.HasSuffix(lines[1], want) {
		t.Errorf("note line must end with %q, got %q", want, lines[1])
	}
	if !strings.HasPrefix(lines[1], i18n.T(i18n.KeySkillLoadPinnedNote)) {
		t.Errorf("note line must keep the pinned note text, got %q", lines[1])
	}

	noDesc := LoadedSkill{Name: "demo", Bytes: 10, Source: SkillSourceGlobal}
	out2 := a.skillLoadResultText(noDesc, "BODY")
	lines2 := strings.SplitN(out2, "\n", 3)
	if strings.Contains(lines2[1], desc) {
		t.Errorf("note line must not carry a description when there is none, got %q", lines2[1])
	}
	t.Logf("FIX587_NOTE|with_desc=%q|without=%q", lines[1], lines2[1])
}
