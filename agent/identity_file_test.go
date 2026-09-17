// Unit tests for the structured ./IDENTITY.md personalization file
// (FEATURE-531 follow-up: personalization lives in workspace files, never in
// config.json).
//
// Author: L.Shuang
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/i18n"
)

const identitySample = `# Agent identity

## agent-name
my-agent

## agent-description
A pragmatic helper.

## user-name
Alice

## user-description
Runs the release process.
`

func TestParseIdentityStructured(t *testing.T) {
	id := parseIdentity(identitySample)
	if !id.Found {
		t.Fatal("parsed identity should be marked as found")
	}
	if id.AgentName != "my-agent" {
		t.Errorf("AgentName = %q, want %q", id.AgentName, "my-agent")
	}
	if id.AgentDescription != "A pragmatic helper." {
		t.Errorf("AgentDescription = %q", id.AgentDescription)
	}
	if id.UserName != "Alice" {
		t.Errorf("UserName = %q", id.UserName)
	}
	if id.UserDescription != "Runs the release process." {
		t.Errorf("UserDescription = %q", id.UserDescription)
	}
	if !strings.Contains(id.Extra, "# Agent identity") {
		t.Errorf("unrecognized heading should be kept as free text, got %q", id.Extra)
	}
}

func TestIdentityTextRoundTrip(t *testing.T) {
	id := LoadIdentity("")
	id = parseIdentity(identitySample)
	text := IdentityText(id)
	if !strings.Contains(text, "## agent-name\nmy-agent") {
		t.Errorf("rendered text should contain the agent-name section, got:\n%s", text)
	}
	if !strings.Contains(text, "Runs the release process.") {
		t.Errorf("rendered text should contain the user-description, got:\n%s", text)
	}
	if !strings.HasPrefix(text, "# Agent identity") {
		t.Errorf("free-form text should be written before the structured sections, got:\n%s", text)
	}
	reparsed := parseIdentity(text)
	if reparsed.AgentName != id.AgentName || reparsed.AgentDescription != id.AgentDescription ||
		reparsed.UserName != id.UserName || reparsed.UserDescription != id.UserDescription {
		t.Errorf("round trip changed the identity: %+v vs %+v", reparsed, id)
	}
}

func TestWriteAndLoadIdentity(t *testing.T) {
	dir := t.TempDir()
	want := Identity{AgentName: "file-agent", AgentDescription: "desc", UserName: "bob"}
	if err := WriteIdentity(dir, want); err != nil {
		t.Fatalf("WriteIdentity: %v", err)
	}
	got := LoadIdentity(dir)
	if got.AgentName != want.AgentName || got.AgentDescription != want.AgentDescription || got.UserName != want.UserName {
		t.Errorf("LoadIdentity = %+v, want %+v", got, want)
	}
	if !got.Found {
		t.Error("loaded identity should be marked as found")
	}
}

func TestLoadIdentityMissingFile(t *testing.T) {
	got := LoadIdentity(t.TempDir())
	if got.Found {
		t.Error("missing file must yield Found=false")
	}
	if got.AgentName != "" {
		t.Errorf("missing file must yield an empty name, got %q", got.AgentName)
	}
	if LoadIdentity("") != (Identity{}) {
		t.Error("empty dir must yield a zero Identity")
	}
}

func TestUpdateIdentityFieldPreservesOtherContent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(IdentityPath(dir), []byte(identitySample), 0644); err != nil {
		t.Fatal(err)
	}
	if err := UpdateIdentityField(dir, "agent-description", "New description."); err != nil {
		t.Fatalf("UpdateIdentityField: %v", err)
	}
	got := LoadIdentity(dir)
	if got.AgentDescription != "New description." {
		t.Errorf("AgentDescription = %q, want %q", got.AgentDescription, "New description.")
	}
	// Every other field and the free-form text must survive the update.
	if got.AgentName != "my-agent" || got.UserName != "Alice" || got.UserDescription != "Runs the release process." {
		t.Errorf("update dropped other fields: %+v", got)
	}
	if !strings.Contains(got.Extra, "# Agent identity") {
		t.Errorf("update dropped the free-form text: %q", got.Extra)
	}
	raw, err := os.ReadFile(IdentityPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "## user-name\nAlice") {
		t.Errorf("file on disk lost the user-name section:\n%s", raw)
	}
}

func TestUpdateIdentityFieldCreatesFile(t *testing.T) {
	dir := t.TempDir()
	if err := UpdateIdentityField(dir, "agent-name", "fresh"); err != nil {
		t.Fatalf("UpdateIdentityField: %v", err)
	}
	if got := LoadIdentity(dir).AgentName; got != "fresh" {
		t.Errorf("AgentName = %q, want %q", got, "fresh")
	}
}

func TestUpdateIdentityFieldUnknownKey(t *testing.T) {
	if err := UpdateIdentityField(t.TempDir(), "nope", "x"); err == nil {
		t.Error("unknown identity field should return an error")
	}
}

func TestResolveAgentNameFallsBackToDefault(t *testing.T) {
	dir := t.TempDir()
	if got := ResolveAgentName(dir); got != "co-shell" {
		t.Errorf("ResolveAgentName(empty) = %q, want %q", got, "co-shell")
	}
	if err := UpdateIdentityField(dir, "agent-name", "custom-agent"); err != nil {
		t.Fatal(err)
	}
	if got := ResolveAgentName(dir); got != "custom-agent" {
		t.Errorf("ResolveAgentName(file) = %q, want %q", got, "custom-agent")
	}
}

func TestResolveAgentDescriptionFallsBackToI18nDefault(t *testing.T) {
	i18n.Init("en")
	dir := t.TempDir()
	if got := ResolveAgentDescription(dir); got != i18n.T(i18n.KeyAgentDefaultDescription) {
		t.Errorf("ResolveAgentDescription(empty) = %q, want the i18n default", got)
	}
	if err := UpdateIdentityField(dir, "agent-description", "file description"); err != nil {
		t.Fatal(err)
	}
	if got := ResolveAgentDescription(dir); got != "file description" {
		t.Errorf("ResolveAgentDescription(file) = %q, want %q", got, "file description")
	}
}

func TestResolveUserNamePriority(t *testing.T) {
	i18n.Init("en")
	dir := t.TempDir()
	// 1) fallback value when the file has no user-name.
	if got := ResolveUserName(dir, "fallback-user"); got != "fallback-user" {
		t.Errorf("ResolveUserName(fallback) = %q, want %q", got, "fallback-user")
	}
	// 2) i18n anonymous default when neither file nor fallback is set.
	if got := ResolveUserName(dir, ""); got != i18n.T(i18n.KeyAnonymousUser) {
		t.Errorf("ResolveUserName(anonymous) = %q, want the anonymous default", got)
	}
	// 3) the file wins over the fallback.
	if err := UpdateIdentityField(dir, "user-name", "file-user"); err != nil {
		t.Fatal(err)
	}
	if got := ResolveUserName(dir, "fallback-user"); got != "file-user" {
		t.Errorf("ResolveUserName(file) = %q, want %q", got, "file-user")
	}
}

func TestAgentIdentityAccessorsAndPersonalizationFiles(t *testing.T) {
	dir := t.TempDir()
	a := &Agent{workspacePath: dir}

	if got := a.IdentityDir(); got != dir {
		t.Errorf("IdentityDir() = %q, want %q", got, dir)
	}
	if got := a.IdentityFilePath(); got != IdentityPath(dir) {
		t.Errorf("IdentityFilePath() = %q, want %q", got, IdentityPath(dir))
	}
	if err := a.SetIdentityField("agent-name", "accessor-agent"); err != nil {
		t.Fatalf("SetIdentityField: %v", err)
	}
	if got := a.IdentityName(); got != "accessor-agent" {
		t.Errorf("IdentityName() = %q", got)
	}
	if err := a.SetIdentityField("agent-description", "accessor description"); err != nil {
		t.Fatal(err)
	}
	if got := a.IdentityDescription(); got != "accessor description" {
		t.Errorf("IdentityDescription() = %q", got)
	}

	if got := a.ReadPersonalizationFile("PRINCIPLES.md"); got != "" {
		t.Errorf("ReadPersonalizationFile(missing) = %q, want empty", got)
	}
	if err := a.WritePersonalizationFile("PRINCIPLES.md", "my principles\n"); err != nil {
		t.Fatalf("WritePersonalizationFile: %v", err)
	}
	if got := a.ReadPersonalizationFile("PRINCIPLES.md"); got != "my principles" {
		t.Errorf("ReadPersonalizationFile = %q, want %q", got, "my principles")
	}
	if _, err := os.Stat(filepath.Join(dir, "PRINCIPLES.md")); err != nil {
		t.Errorf("PRINCIPLES.md not written into the identity dir: %v", err)
	}
}

func TestWritePersonalizationFileRejectsEmptyDir(t *testing.T) {
	if err := WritePersonalizationFile("", "RULES.md", "x"); err == nil {
		t.Error("empty dir should return an error")
	}
	if err := WriteIdentity("", Identity{AgentName: "x"}); err == nil {
		t.Error("empty dir should return an error for WriteIdentity too")
	}
}

func TestPersonalizationDirFallsBackToCwd(t *testing.T) {
	if got := PersonalizationDir("/tmp/ws"); got != "/tmp/ws" {
		t.Errorf("PersonalizationDir(ws) = %q, want %q", got, "/tmp/ws")
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if got := PersonalizationDir(""); got != cwd {
		t.Errorf("PersonalizationDir(empty) = %q, want the working directory %q", got, cwd)
	}
}

func TestIdentityHeadingKey(t *testing.T) {
	cases := []struct {
		line  string
		key   string
		isKey bool
	}{
		{"## agent-name", "agent-name", true},
		{"### User-Name", "user-name", true},
		{"## free form heading", "", false},
		{"## ", "", false},
		{"plain text", "", false},
	}
	for _, c := range cases {
		key, ok := identityHeadingKey(c.line)
		if ok != c.isKey || key != c.key {
			t.Errorf("identityHeadingKey(%q) = (%q, %v), want (%q, %v)", c.line, key, ok, c.key, c.isKey)
		}
	}
}
