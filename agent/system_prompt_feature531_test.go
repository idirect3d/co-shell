package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
)

// FEATURE-531: mode strategies live in {cwd}/mode/{MODE}.md and win over the
// built-in i18n defaults.
func TestModeStrategyFileOverridesI18n(t *testing.T) {
	i18n.Init("en")
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "mode"), 0755); err != nil {
		t.Fatalf("mkdir mode: %v", err)
	}
	custom := "CUSTOM PLAN STRATEGY {AGENT_NAME}"
	if err := os.WriteFile(filepath.Join(dir, "mode", "PLAN.md"), []byte(custom), 0644); err != nil {
		t.Fatalf("write mode file: %v", err)
	}

	got := buildResultModeSection(dir, nil)
	if !strings.Contains(got, custom) {
		t.Errorf("PLAN strategy file not used, got:\n%s", got)
	}
	// Other modes keep their i18n default.
	if def := i18n.T(i18n.KeyWorkModeAct); def != "" && !strings.Contains(got, def) {
		t.Errorf("ACT strategy should still come from i18n, got:\n%s", got)
	}
}

// A missing or blank mode strategy file falls back to the i18n default.
func TestModeStrategyFileMissingFallsBack(t *testing.T) {
	i18n.Init("en")
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "mode"), 0755); err != nil {
		t.Fatalf("mkdir mode: %v", err)
	}
	// Blank file must be treated as absent.
	if err := os.WriteFile(filepath.Join(dir, "mode", "PLAN.md"), []byte("   \n"), 0644); err != nil {
		t.Fatalf("write blank mode file: %v", err)
	}
	got := buildResultModeSection(dir, nil)
	if want := i18n.T(i18n.KeyWorkModePlan); want != "" && !strings.Contains(got, want) {
		t.Errorf("blank strategy file should fall back to i18n, got:\n%s", got)
	}
}

// ResultMode text is identical for every current work mode, with or without
// strategy files present.
func TestResultModeStaticAcrossModesWithFiles(t *testing.T) {
	i18n.Init("en")
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "mode"), 0755); err != nil {
		t.Fatalf("mkdir mode: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mode", "ACT.md"), []byte("ACT STRATEGY"), 0644); err != nil {
		t.Fatalf("write ACT file: %v", err)
	}
	base := buildResultModeSection(dir, nil)
	for _, mode := range []string{"act", "plan", "research"} {
		cfg := &config.Config{}
		cfg.LLM.WorkMode = mode
		if got := buildResultModeSection(dir, cfg); got != base {
			t.Errorf("ResultMode differs for mode %s:\n%s\n--- vs ---\n%s", mode, got, base)
		}
	}
}

// Only the three exported sections can be overridden from the workspace root;
// other section files must be ignored.
func TestRootSectionOverrideOnlyForExportedSections(t *testing.T) {
	i18n.Init("en")
	dir := t.TempDir()
	// FEATURE-531 follow-up: IDENTITY.md is structured, so the Identity section is
	// rendered from the built-in template with the resolved values; only the
	// unrecognized (free-form) text is appended verbatim.
	identity := "CUSTOM IDENTITY\n\n## agent-name\nfile-agent"
	if err := os.WriteFile(filepath.Join(dir, "IDENTITY.md"), []byte(identity), 0644); err != nil {
		t.Fatalf("write IDENTITY.md: %v", err)
	}
	// Must NOT be picked up.
	if err := os.WriteFile(filepath.Join(dir, "TOOL_USAGE.md"), []byte("SHOULD NOT APPEAR"), 0644); err != nil {
		t.Fatalf("write TOOL_USAGE.md: %v", err)
	}

	env := &promptEnv{cwd: dir, agentName: "co-shell", agentDescription: "ENV DESCRIPTION"}
	cfg := &config.Config{}

	gotIdentity := buildNamedSection("Identity", env, cfg, false, nil, false)
	if !strings.HasPrefix(gotIdentity, "Your name is") {
		t.Errorf("Identity section should still use the built-in template, got: %q", gotIdentity)
	}
	if !strings.Contains(gotIdentity, "ENV DESCRIPTION") {
		t.Errorf("Identity section should carry the resolved description, got: %q", gotIdentity)
	}
	if !strings.Contains(gotIdentity, "CUSTOM IDENTITY") {
		t.Errorf("free-form IDENTITY.md text should be appended, got: %q", gotIdentity)
	}
	if strings.Contains(gotIdentity, "file-agent") {
		t.Errorf("structured field value must not be injected verbatim, got: %q", gotIdentity)
	}
	gotToolUsage := buildNamedSection("ToolUsage", env, cfg, false, nil, false)
	if strings.Contains(gotToolUsage, "SHOULD NOT APPEAR") {
		t.Errorf("TOOL_USAGE.md must not override the section, got: %q", gotToolUsage)
	}
}

// --init exports the three section files plus one strategy file per mode and
// backs up pre-existing files as .bak.
func TestExportSectionsWritesFilesAndBackups(t *testing.T) {
	i18n.Init("en")
	dir := t.TempDir()
	t.Chdir(dir)

	// Pre-existing file must be backed up, not lost.
	if err := os.WriteFile(filepath.Join(dir, "RULES.md"), []byte("USER EDITED RULES"), 0644); err != nil {
		t.Fatalf("seed RULES.md: %v", err)
	}

	cfg := &config.Config{}
	cfg.WorkModes = []config.WorkMode{{Name: "audit", Description: "audit strategy"}}

	written, err := ExportSections(cfg)
	if err != nil {
		t.Fatalf("ExportSections: %v", err)
	}
	for _, name := range []string{"IDENTITY.md", "CAPABILITIES.md", "RULES.md", "mode/ACT.md", "mode/PLAN.md", "mode/RESEARCH.md", "mode/AUDIT.md"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name))); err != nil {
			t.Errorf("expected exported file %s: %v", name, err)
		}
	}
	if len(written) < 7 {
		t.Errorf("expected at least 7 exported files, got %d: %v", len(written), written)
	}
	// Backup of the user-edited file keeps the user's content.
	bak, err := os.ReadFile(filepath.Join(dir, "RULES.md.bak"))
	if err != nil {
		t.Fatalf("RULES.md.bak missing: %v", err)
	}
	if string(bak) != "USER EDITED RULES" {
		t.Errorf("backup content = %q, want user content", string(bak))
	}
	// No legacy per-mode sub-directory is created.
	if _, err := os.Stat(filepath.Join(dir, "mode", "act")); !os.IsNotExist(err) {
		t.Errorf("mode/act/ sub-directory must not be created")
	}
}

// Work modes no longer carry their own section list.
func TestWorkModesHaveNoSectionsField(t *testing.T) {
	i18n.Init("en")
	modes := config.DefaultWorkModes()
	if len(modes) == 0 {
		t.Fatal("DefaultWorkModes returned no modes")
	}
	Sections := config.DefaultSections()
	if len(Sections) != 9 || Sections[2] != "ResultMode" || Sections[3] != "Objective" {
		t.Errorf("unexpected global section list: %v", Sections)
	}
}
