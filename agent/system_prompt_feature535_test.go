// FEATURE-535 unit tests: identity injection into the Environment section and
// the explicit two-layer RULES semantics ({DEFAULT_RULES} expansion, missing
// {CUSTOM_RULES} warning).
//
// Author: L.Shuang
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
)

// feature535Build builds a system prompt for the current working directory: the
// builder resolves ./RULES.md and the identity files from os.Getwd(), which the
// tests point at a temp dir via t.Chdir.
func feature535Build(t *testing.T, id promptIdentity, rules string) string {
	t.Helper()
	return buildSystemPromptWithMode(nil, rules, config.ResultModeMinimal, false, id, "web", "", "", true)
}

// feature535EnvSection returns the Environment part of the prompt. Environment is
// the last section in config.DefaultSections(), so the marker "SYSTEM INFORMATION"
// reaches the end of the prompt.
func feature535EnvSection(t *testing.T, prompt string) string {
	t.Helper()
	idx := strings.Index(prompt, "SYSTEM INFORMATION")
	if idx < 0 {
		t.Fatalf("system prompt has no SYSTEM INFORMATION section:\n%s", prompt)
	}
	section := prompt[idx:]
	if !strings.Contains(section, "</system_info>") {
		t.Fatalf("SYSTEM INFORMATION section is not closed:\n%s", section)
	}
	return section
}

// feature535RulesProbe returns a distinctive plain-text line of the built-in
// RULES body so assertions survive wording changes elsewhere in the default text.
func feature535RulesProbe(t *testing.T) string {
	t.Helper()
	for _, line := range strings.Split(i18n.T(i18n.KeySystemPromptRules), "\n") {
		if l := strings.TrimSpace(line); len(l) > 20 && !strings.Contains(l, "{") {
			return l
		}
	}
	t.Fatal("built-in RULES body has no plain-text line to probe")
	return ""
}

// feature535WriteRulesDir creates {workspace}/.rules/team.md and returns the
// text the builder injects at {CUSTOM_RULES}.
func feature535WriteRulesDir(t *testing.T, dir string) string {
	t.Helper()
	rulesDir := filepath.Join(dir, ".rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatalf("mkdir .rules: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rulesDir, "team.md"), []byte("team rule"), 0o644); err != nil {
		t.Fatalf("write .rules/team.md: %v", err)
	}
	return loadRulesDir(rulesDir)
}

// UC-0001: Environment injects <user>/<user_description> and <channel> carries
// the channel name only.
func TestFeature535IdentityInEnvironment(t *testing.T) {
	i18n.Init("en")
	dir := t.TempDir()
	t.Chdir(dir)

	if err := UpdateIdentityField(dir, "user-name", "梁爽"); err != nil {
		t.Fatalf("write user-name: %v", err)
	}
	if err := UpdateIdentityField(dir, "user-description", "后端工程师"); err != nil {
		t.Fatalf("write user-description: %v", err)
	}

	out := feature535Build(t, promptIdentity{
		UserName:        ResolveUserName(dir, ""),
		UserDescription: ResolveUserDescription(dir),
	}, "")

	section := feature535EnvSection(t, out)
	for _, want := range []string{
		"<user>梁爽</user>",
		"<user_description>后端工程师</user_description>",
		"<channel>web</channel>",
	} {
		if !strings.Contains(section, want) {
			t.Errorf("Environment section missing %q:\n%s", want, section)
		}
	}
	if strings.Contains(out, "梁爽 @ web") {
		t.Errorf("user name must no longer be folded into <channel>, got %q in prompt", "梁爽 @ web")
	}
}

// UC-0002: an empty user description leaves no empty tag line behind.
func TestFeature535EmptyUserDescriptionOmitted(t *testing.T) {
	i18n.Init("en")
	dir := t.TempDir()
	t.Chdir(dir)

	if err := UpdateIdentityField(dir, "user-name", "梁爽"); err != nil {
		t.Fatalf("write user-name: %v", err)
	}
	if got := ResolveUserDescription(dir); got != "" {
		t.Fatalf("precondition failed: user description = %q, want empty", got)
	}

	out := feature535Build(t, promptIdentity{
		UserName:        ResolveUserName(dir, ""),
		UserDescription: ResolveUserDescription(dir),
	}, "")

	section := feature535EnvSection(t, out)
	if !strings.Contains(section, "<user>梁爽</user>") {
		t.Errorf("Environment section missing the user field:\n%s", section)
	}
	if strings.Contains(section, "<user_description>") || strings.Contains(section, "</user_description>") {
		t.Errorf("empty user_description must be dropped entirely:\n%s", section)
	}
}

// UC-0003: changing user-description and rebuilding the prompt reflects the new
// value (the field is no longer a dead end).
func TestFeature535UserDescriptionRebuild(t *testing.T) {
	i18n.Init("en")
	dir := t.TempDir()
	t.Chdir(dir)

	build := func() string {
		return feature535Build(t, promptIdentity{
			UserName:        ResolveUserName(dir, ""),
			UserDescription: ResolveUserDescription(dir),
		}, "")
	}

	if err := UpdateIdentityField(dir, "user-description", "后端工程师"); err != nil {
		t.Fatalf("write user-description: %v", err)
	}
	if out := build(); !strings.Contains(out, "<user_description>后端工程师</user_description>") {
		t.Errorf("first build does not carry the initial description:\n%s", feature535EnvSection(t, out))
	}

	if err := UpdateIdentityField(dir, "user-description", "运维负责人"); err != nil {
		t.Fatalf("update user-description: %v", err)
	}
	out := build()
	section := feature535EnvSection(t, out)
	if !strings.Contains(section, "<user_description>运维负责人</user_description>") {
		t.Errorf("rebuilt prompt does not carry the new description:\n%s", section)
	}
	if strings.Contains(section, "后端工程师") {
		t.Errorf("rebuilt prompt still carries the stale description:\n%s", section)
	}
}

// UC-0004: without ./RULES.md the built-in template stays in place and the
// .rules/ content lands at {CUSTOM_RULES}.
func TestFeature535RulesDefaultPlusCustom(t *testing.T) {
	i18n.Init("en")
	dir := t.TempDir()
	t.Chdir(dir)
	rules := feature535WriteRulesDir(t, dir)

	out := feature535Build(t, promptIdentity{}, rules)

	if probe := feature535RulesProbe(t); !strings.Contains(out, probe) {
		t.Errorf("built-in RULES body is missing from the prompt (probe %q)", probe)
	}
	if !strings.Contains(out, "====\n\nteam\n\nteam rule") {
		t.Errorf(".rules/ content is not injected at {CUSTOM_RULES}:\n%s", out)
	}
	if strings.Contains(out, "{CONTEXT_REORGANIZE_THRESHOLD}") {
		t.Errorf("{CONTEXT_REORGANIZE_THRESHOLD} was left unsubstituted:\n%s", out)
	}
	// The explanatory note spells the placeholder tokens literally, so the
	// resolved section may only contain as many occurrences as the note itself.
	note := i18n.T(i18n.KeySystemPromptRulesNote)
	for _, token := range []string{"{DEFAULT_RULES}", "{CUSTOM_RULES}"} {
		if got, want := strings.Count(out, token), strings.Count(note, token); got != want {
			t.Errorf("unresolved %s: prompt has %d occurrence(s), note explains %d", token, got, want)
		}
	}
}

// UC-0005: ./RULES.md may pull in the built-in rules via {DEFAULT_RULES}; the
// expanded default section brings {CUSTOM_RULES} back, so .rules/ is still injected.
func TestFeature535DefaultRulesPlaceholder(t *testing.T) {
	i18n.Init("en")
	dir := t.TempDir()
	t.Chdir(dir)
	rules := feature535WriteRulesDir(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "RULES.md"), []byte("我的规则\n\n{DEFAULT_RULES}"), 0o644); err != nil {
		t.Fatalf("write RULES.md: %v", err)
	}

	out := feature535Build(t, promptIdentity{}, rules)

	if !strings.Contains(out, "我的规则") {
		t.Errorf("user RULES.md content is missing:\n%s", out)
	}
	if probe := feature535RulesProbe(t); !strings.Contains(out, probe) {
		t.Errorf("{DEFAULT_RULES} was not expanded to the built-in body (probe %q)", probe)
	}
	if !strings.Contains(out, "team rule") {
		t.Errorf(".rules/ content must still be injected via the expanded {CUSTOM_RULES}:\n%s", out)
	}
	// The note is not appended for a user-provided RULES.md, so no placeholder
	// token may survive.
	for _, token := range []string{"{DEFAULT_RULES}", "{CUSTOM_RULES}", "{CONTEXT_REORGANIZE_THRESHOLD}"} {
		if strings.Contains(out, token) {
			t.Errorf("unresolved placeholder %s in the prompt:\n%s", token, out)
		}
	}
}

// UC-0006: ./RULES.md without {CUSTOM_RULES} drops the .rules/ content and the
// builder reports it (the log.Warn condition is asserted here; the warn itself
// goes to the global logger which unit tests do not initialize).
func TestFeature535MissingCustomRulesWarns(t *testing.T) {
	i18n.Init("en")
	dir := t.TempDir()
	t.Chdir(dir)
	rules := feature535WriteRulesDir(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "RULES.md"), []byte("只按我的规则来"), 0o644); err != nil {
		t.Fatalf("write RULES.md: %v", err)
	}

	out := feature535Build(t, promptIdentity{}, rules)

	if !strings.Contains(out, "只按我的规则来") {
		t.Errorf("user RULES.md content is missing:\n%s", out)
	}
	if strings.Contains(out, "team rule") {
		t.Errorf(".rules/ content must not be injected when {CUSTOM_RULES} is absent:\n%s", out)
	}

	// The resolved RULES text (before placeholder substitution) is what the
	// builder inspects: no {CUSTOM_RULES} => warn.
	resolved := loadSectionText(dir, "Rules", func() string { return i18n.T(i18n.KeySystemPromptRules) })
	if !rulesPlaceholderMissing(resolved, rules) {
		t.Errorf("missing {CUSTOM_RULES} must be reported, resolved text:\n%s", resolved)
	}
	// A section that pulls the default rules in indirectly keeps the placeholder
	// after expansion => no false-positive warning (the UC-0005 scenario, text
	// expanded in the same order as the builder does it).
	indirect := strings.ReplaceAll("我的规则\n\n{DEFAULT_RULES}", "{DEFAULT_RULES}", i18n.T(i18n.KeySystemPromptRules))
	if rulesPlaceholderMissing(indirect, rules) {
		t.Errorf("expanded {DEFAULT_RULES} restores {CUSTOM_RULES}, it must not warn")
	}
	// Nothing to warn about when .rules/ is empty.
	if rulesPlaceholderMissing(resolved, "") {
		t.Errorf("an empty .rules/ must not warn")
	}
}

// UC-0008: the vision recognition round builds the Identity section only, so it
// never carries the user identity fields.
func TestFeature535VisionPromptUnaffected(t *testing.T) {
	i18n.Init("en")
	dir := t.TempDir()

	if err := UpdateIdentityField(dir, "user-name", "梁爽"); err != nil {
		t.Fatalf("write user-name: %v", err)
	}
	if err := UpdateIdentityField(dir, "user-description", "后端工程师"); err != nil {
		t.Fatalf("write user-description: %v", err)
	}

	out := buildVisionIdentityPrompt(config.DefaultConfig(), dir)
	for _, forbidden := range []string{"<user>", "<user_description>", "<system_info>"} {
		if strings.Contains(out, forbidden) {
			t.Errorf("vision prompt must not contain %s:\n%s", forbidden, out)
		}
	}
	if strings.Contains(out, "后端工程师") {
		t.Errorf("vision prompt must not carry the user description:\n%s", out)
	}
}
