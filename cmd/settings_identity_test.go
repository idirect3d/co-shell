// Unit tests for the Web UI identity & personality form fields (FEATURE-534):
// the user-name / user-description fields exposed by IdentityJSON and persisted
// by SaveIdentity into ./IDENTITY.md.
//
// Author: L.Shuang
// MIT License - Copyright (c) 2026 L.Shuang

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/store"
	"github.com/idirect3d/co-shell/workspace"
)

// setupIdentityTestHandler creates a handler whose agent workspace is a temp
// dir, so IdentityJSON / SaveIdentity read and write ./IDENTITY.md there.
func setupIdentityTestHandler(t *testing.T) (*SettingsHandler, string) {
	t.Helper()
	i18n.Init("zh")

	ws := t.TempDir()
	wsObj, err := workspace.New(ws)
	if err != nil {
		t.Fatalf("cannot init workspace: %v", err)
	}
	boltStore, err := store.NewStore(wsObj)
	if err != nil {
		t.Fatalf("cannot init bbolt store: %v", err)
	}
	cfg := config.DefaultConfig()
	// A real agent is required because identityDir() resolves the workspace
	// through it.
	ag := agent.New(nil, nil, store.NewDualStore(boltStore, nil), "")
	ag.SetWorkspacePath(ws)
	ag.SetConfig(cfg)

	return &SettingsHandler{cfg: cfg, agent: ag}, ws
}

// TestIdentityJSONExposesUserFields verifies the identity form receives the two
// ./IDENTITY.md user fields in the order the UI groups them (UC-0001).
func TestIdentityJSONExposesUserFields(t *testing.T) {
	h, ws := setupIdentityTestHandler(t)
	if err := agent.UpdateIdentityField(ws, "user-name", "梁爽"); err != nil {
		t.Fatalf("write user-name: %v", err)
	}
	if err := agent.UpdateIdentityField(ws, "user-description", "后端工程师"); err != nil {
		t.Fatalf("write user-description: %v", err)
	}

	fields := h.IdentityJSON()
	wantKeys := []string{"name", "description", "user-name", "user-description", "principles", "capabilities", "rules"}
	if len(fields) != len(wantKeys) {
		t.Fatalf("expected %d fields, got %d: %+v", len(wantKeys), len(fields), fields)
	}
	for i, key := range wantKeys {
		if fields[i].Key != key {
			t.Errorf("field %d: expected key %q, got %q", i, key, fields[i].Key)
		}
	}

	byKey := map[string]IdentityField{}
	for _, f := range fields {
		byKey[f.Key] = f
	}
	if got := byKey["user-name"].Value; got != "梁爽" {
		t.Errorf("user-name value = %q, want %q", got, "梁爽")
	}
	if got := byKey["user-description"].Value; got != "后端工程师" {
		t.Errorf("user-description value = %q, want %q", got, "后端工程师")
	}
	if got := byKey["user-name"].Type; got != "text" {
		t.Errorf("user-name type = %q, want text", got)
	}
	if got := byKey["user-description"].Type; got != "textarea" {
		t.Errorf("user-description type = %q, want textarea", got)
	}
	for _, key := range []string{"user-name", "user-description"} {
		if l := byKey[key].Label; l == "" || l == i18n.KeyIdentityUserName || l == i18n.KeyIdentityUserDesc {
			t.Errorf("%s label is not a localized string: %q", key, l)
		}
	}
}

// TestSaveIdentityPersistsUserFields verifies both user fields are written to
// ./IDENTITY.md and resolve back (UC-0002).
func TestSaveIdentityPersistsUserFields(t *testing.T) {
	h, ws := setupIdentityTestHandler(t)
	cases := []struct {
		key     string
		value   string
		section string
	}{
		{"user-name", "梁爽", "## user-name"},
		{"user-description", "后端工程师", "## user-description"},
	}
	for _, tc := range cases {
		if err := h.SaveIdentity(tc.key, tc.value); err != nil {
			t.Fatalf("SaveIdentity(%s): %v", tc.key, err)
		}
		data, err := os.ReadFile(filepath.Join(ws, "IDENTITY.md"))
		if err != nil {
			t.Fatalf("read IDENTITY.md: %v", err)
		}
		if !strings.Contains(string(data), tc.section) || !strings.Contains(string(data), tc.value) {
			t.Errorf("IDENTITY.md missing %s = %q:\n%s", tc.section, tc.value, data)
		}
	}

	// Round-trip through the resolvers used when rendering the form.
	if got := agent.ResolveUserName(ws, ""); got != "梁爽" {
		t.Errorf("ResolveUserName = %q, want %q", got, "梁爽")
	}
	if got := agent.ResolveUserDescription(ws); got != "后端工程师" {
		t.Errorf("ResolveUserDescription = %q, want %q", got, "后端工程师")
	}

	// The two fields must not leak into each other's section.
	if err := h.SaveIdentity("principles", "只做最小改动"); err != nil {
		t.Fatalf("SaveIdentity(principles): %v", err)
	}
	if got := agent.ResolveUserName(ws, ""); got != "梁爽" {
		t.Errorf("user-name changed after saving principles: %q", got)
	}
}

// TestFeature535IdentityRulesFallback verifies the "rules" field of the identity
// form: ./RULES.md wins when present, otherwise the built-in default is shown, so
// the Web UI never renders an empty rules box (FEATURE-535, UC-0007).
func TestFeature535IdentityRulesFallback(t *testing.T) {
	h, ws := setupIdentityTestHandler(t)

	rulesValue := func() string {
		for _, f := range h.IdentityJSON() {
			if f.Key == "rules" {
				return f.Value
			}
		}
		t.Fatal("IdentityJSON has no rules field")
		return ""
	}

	want := strings.TrimSpace(i18n.T(i18n.KeySystemPromptRules))
	if want == "" {
		t.Fatal("built-in RULES default is empty")
	}
	got := rulesValue()
	if got != want {
		t.Errorf("rules value without ./RULES.md = %q, want the built-in default", got)
	}
	// The form shows the raw template (what --init exports): no placeholder may
	// be substituted away.
	for _, token := range []string{"{CUSTOM_RULES}", "{CONTEXT_REORGANIZE_THRESHOLD}"} {
		if strings.Contains(want, token) && !strings.Contains(got, token) {
			t.Errorf("rules value must keep %s literal, got: %q", token, got)
		}
	}

	if err := os.WriteFile(filepath.Join(ws, "RULES.md"), []byte("我的规则"), 0o644); err != nil {
		t.Fatalf("write RULES.md: %v", err)
	}
	if got := rulesValue(); got != "我的规则" {
		t.Errorf("rules value with ./RULES.md = %q, want the file content", got)
	}
}
