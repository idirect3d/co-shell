// Agent personalization lives in workspace files instead of config.json
// (FEATURE-531) so that several agents sharing one config.json keep their own
// identity. This file implements the structured ./IDENTITY.md format:
//
//	## agent-name
//	work
//
//	## agent-description
//	你是一个严谨、务实、目标驱动的通用智能体。
//
//	## user-name
//	匿名
//
//	## user-description
//	（可选）
//
//	（未识别的标题与文本原样保留，作为附加自由文本）
//
// Supported keys: agent-name, agent-description, user-name, user-description.
// Values are read at prompt-build time (every new turn) so external edits made
// by the user take effect immediately.
//
// Author: L.Shuang
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
)

// IdentityFileName is the workspace-root file holding the agent identity.
const IdentityFileName = "IDENTITY.md"

// identityKeys lists the recognized IDENTITY.md section keys.
var identityKeys = map[string]bool{
	"agent-name":        true,
	"agent-description": true,
	"user-name":         true,
	"user-description":  true,
}

// Identity holds the agent personalization parsed from ./IDENTITY.md.
type Identity struct {
	AgentName        string
	AgentDescription string
	UserName         string
	UserDescription  string
	// Extra keeps unrecognized headings and free-form text verbatim; it is
	// appended to the IDENTITY section.
	Extra string
	// Found reports whether the file exists (even when it parses to no field).
	Found bool
}

// IdentityPath returns the path of the identity file inside dir.
func IdentityPath(dir string) string {
	if dir == "" {
		return IdentityFileName
	}
	return filepath.Join(dir, IdentityFileName)
}

// LoadIdentity parses {dir}/IDENTITY.md. A missing file yields a zero Identity
// with Found=false; callers apply their own defaults.
func LoadIdentity(dir string) Identity {
	if dir == "" {
		return Identity{}
	}
	data, err := os.ReadFile(IdentityPath(dir))
	if err != nil {
		return Identity{}
	}
	return parseIdentity(string(data))
}

// parseIdentity parses the markdown section format described above.
func parseIdentity(text string) Identity {
	id := Identity{Found: true}
	var (
		current string
		buf     []string
		extra   []string
	)
	flush := func() {
		v := strings.TrimSpace(strings.Join(buf, "\n"))
		switch current {
		case "agent-name":
			id.AgentName = v
		case "agent-description":
			id.AgentDescription = v
		case "user-name":
			id.UserName = v
		case "user-description":
			id.UserDescription = v
		}
		buf = buf[:0]
	}
	for _, line := range strings.Split(text, "\n") {
		key, isHeading := identityHeadingKey(line)
		if isHeading {
			flush()
			if identityKeys[key] {
				current = key
			} else {
				// Unrecognized heading: keep it (and its body) as free text.
				current = ""
				extra = append(extra, strings.TrimRight(line, " \t"))
			}
			continue
		}
		if current == "" {
			extra = append(extra, line)
			continue
		}
		buf = append(buf, line)
	}
	flush()
	id.Extra = strings.TrimSpace(strings.Join(extra, "\n"))
	return id
}

// identityHeadingKey reports whether line is a markdown heading and returns its
// normalized key (lower-case, trimmed). A heading whose text contains spaces is
// not treated as an identity key (it is free-form prose).
func identityHeadingKey(line string) (string, bool) {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, "#") {
		return "", false
	}
	t = strings.TrimLeft(t, "#")
	t = strings.TrimSpace(t)
	if t == "" || strings.ContainsAny(t, " \t") {
		return "", false
	}
	return strings.ToLower(t), true
}

// IdentityText renders the structured file content for the given identity.
// Free-form text is written first so that it round-trips as free-form text
// (text placed after a recognized section would be read back as part of that
// section's value).
func IdentityText(id Identity) string {
	var sb strings.Builder
	writeSection := func(key, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		sb.WriteString("## " + key + "\n")
		sb.WriteString(strings.TrimSpace(value) + "\n\n")
	}
	if extra := strings.TrimSpace(id.Extra); extra != "" {
		sb.WriteString(extra + "\n\n")
	}
	writeSection("agent-name", id.AgentName)
	writeSection("agent-description", id.AgentDescription)
	writeSection("user-name", id.UserName)
	writeSection("user-description", id.UserDescription)
	out := strings.TrimSpace(sb.String())
	if out == "" {
		return ""
	}
	return out + "\n"
}

// WriteIdentity writes the identity to {dir}/IDENTITY.md.
func WriteIdentity(dir string, id Identity) error {
	if dir == "" {
		return fmt.Errorf("cannot write %s: empty directory", IdentityFileName)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create %s: %w", dir, err)
	}
	path := IdentityPath(dir)
	if err := os.WriteFile(path, []byte(IdentityText(id)), 0644); err != nil {
		return fmt.Errorf("cannot write %s: %w", path, err)
	}
	return nil
}

// UpdateIdentityField applies a single field change to {dir}/IDENTITY.md,
// preserving every other field and the free-form text.
// Supported keys: agent-name, agent-description, user-name, user-description.
func UpdateIdentityField(dir, key, value string) error {
	if !identityKeys[key] {
		return fmt.Errorf("unknown identity field: %s", key)
	}
	id := LoadIdentity(dir)
	switch key {
	case "agent-name":
		id.AgentName = value
	case "agent-description":
		id.AgentDescription = value
	case "user-name":
		id.UserName = value
	case "user-description":
		id.UserDescription = value
	}
	id.Found = true
	return WriteIdentity(dir, id)
}

// ResolveAgentName returns the agent name for dir (IDENTITY.md value, else the
// built-in default).
func ResolveAgentName(dir string) string {
	if n := strings.TrimSpace(LoadIdentity(dir).AgentName); n != "" {
		return n
	}
	return "co-shell"
}

// ResolveAgentDescription returns the agent description for dir (IDENTITY.md
// value, else the i18n default).
func ResolveAgentDescription(dir string) string {
	if d := strings.TrimSpace(LoadIdentity(dir).AgentDescription); d != "" {
		return d
	}
	if d := i18n.T(i18n.KeyAgentDefaultDescription); d != "" && d != string(i18n.KeyAgentDefaultDescription) {
		return d
	}
	return ""
}

// ResolveUserName returns the user name for dir: IDENTITY.md value, else the
// given fallback (legacy config value), else the anonymous i18n default.
func ResolveUserName(dir, fallback string) string {
	if n := strings.TrimSpace(LoadIdentity(dir).UserName); n != "" {
		return n
	}
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}
	return i18n.T(i18n.KeyAnonymousUser)
}

// ResolveUserDescription returns the user description for dir (may be empty).
func ResolveUserDescription(dir string) string {
	return strings.TrimSpace(LoadIdentity(dir).UserDescription)
}

// identityExtra returns the free-form text kept in IDENTITY.md.
func identityExtra(dir string) string {
	return strings.TrimSpace(LoadIdentity(dir).Extra)
}

// PersonalizationDir resolves the directory that holds the agent
// personalization files (IDENTITY.md / PRINCIPLES.md / RULES.md /
// CAPABILITIES.md): the given workspace path when non-empty, otherwise the
// process working directory. Returns "" when neither is available.
func PersonalizationDir(workspacePath string) string {
	if workspacePath != "" {
		return workspacePath
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return ""
}

// WritePersonalizationFile writes content to {dir}/{name} (e.g. PRINCIPLES.md,
// RULES.md, CAPABILITIES.md), creating the directory when needed.
func WritePersonalizationFile(dir, name, content string) error {
	if dir == "" {
		return fmt.Errorf("cannot write %s: empty directory", name)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create %s: %w", dir, err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("cannot write %s: %w", path, err)
	}
	return nil
}

// IdentityDir returns the directory holding this agent's personalization files
// (IDENTITY.md, PRINCIPLES.md, RULES.md, CAPABILITIES.md).
func (a *Agent) IdentityDir() string {
	return a.identityDir()
}

// IdentityName returns the agent name stored in ./IDENTITY.md, falling back to
// the built-in default when the file is missing or has no agent-name field.
func (a *Agent) IdentityName() string {
	return ResolveAgentName(a.identityDir())
}

// IdentityDescription returns the description stored in ./IDENTITY.md,
// falling back to the i18n default.
func (a *Agent) IdentityDescription() string {
	return ResolveAgentDescription(a.identityDir())
}

// IdentityFilePath returns the path of this agent's IDENTITY.md.
func (a *Agent) IdentityFilePath() string {
	return IdentityPath(a.identityDir())
}

// SetIdentityField updates a single field of ./IDENTITY.md (FEATURE-531).
// Supported keys: agent-name, agent-description, user-name, user-description.
// A name or description change is announced to the hub board right away
// (FEATURE-538) so the roster shows the new value without waiting for the
// heartbeat.
func (a *Agent) SetIdentityField(key, value string) error {
	if err := UpdateIdentityField(a.identityDir(), key, value); err != nil {
		return err
	}
	if key == "agent-name" || key == "agent-description" {
		a.announceBoardIdentity()
	}
	return nil
}

// announceBoardIdentity pushes the current identity (IDENTITY.md agent-name and
// agent-description) to the hub board as a board_identity frame (FEATURE-538).
// It is a no-op when no hub link is installed (the board is off or the agent is
// not connected to a hub).
func (a *Agent) announceBoardIdentity() {
	send := a.boardLink()
	if send == nil {
		return
	}
	if err := send(map[string]interface{}{
		"type": "board_identity",
		"name": a.IdentityName(),
		"duty": a.IdentityDescription(),
	}); err != nil {
		log.Info("identity: board_identity not sent: %v", err)
	}
}

// WritePersonalizationFile writes content into the personalization file {name}
// (e.g. PRINCIPLES.md, RULES.md) inside this agent's identity directory.
func (a *Agent) WritePersonalizationFile(name, content string) error {
	return WritePersonalizationFile(a.identityDir(), name, content)
}

// ReadPersonalizationFile returns the content of the personalization file
// {name} (e.g. RULES.md), or "" when it does not exist.
func (a *Agent) ReadPersonalizationFile(name string) string {
	dir := a.identityDir()
	if dir == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
