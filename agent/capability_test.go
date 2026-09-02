// Author: L.Shuang
// Created: 2026-09-02
// Last Modified: 2026-09-02
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
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
)

// TestMetaCapabilityIndex verifies that the meta-capability index lists all
// capabilities with stable IDs and localized names/descriptions.
func TestMetaCapabilityIndex(t *testing.T) {
	i18n.Init("zh")
	index := buildMetaCapabilityIndex()
	if index == "" {
		t.Fatal("meta-capability index should not be empty")
	}
	// All capability IDs must appear in the index.
	for _, id := range metaCapabilityIDs {
		if !strings.Contains(index, id) {
			t.Errorf("index missing capability ID %q:\n%s", id, index)
		}
	}
	// Index must not contain full detail text (only name + description).
	if strings.Contains(index, "## 适用场景") {
		t.Errorf("index should not contain full detail text:\n%s", index)
	}
}

// TestFindMetaCapabilityByID verifies exact ID lookup.
func TestFindMetaCapabilityByID(t *testing.T) {
	i18n.Init("zh")
	c := findMetaCapabilityByID("cap.self-modify")
	if c == nil {
		t.Fatal("cap.self-modify should be found")
	}
	if c.ID != "cap.self-modify" {
		t.Errorf("ID = %q, want cap.self-modify", c.ID)
	}
	if c.Detail == "" {
		t.Error("detail should not be empty")
	}
	// Unknown ID returns nil.
	if findMetaCapabilityByID("cap.nonexistent") != nil {
		t.Error("unknown ID should return nil")
	}
}

// TestSearchMetaCapabilities verifies multi-condition keyword fuzzy search.
func TestSearchMetaCapabilities(t *testing.T) {
	i18n.Init("zh")
	// Single keyword matching name.
	results := searchMetaCapabilities([]string{"模型"})
	if len(results) == 0 {
		t.Fatal("search for 模型 should return results")
	}
	// Multi-keyword AND match.
	results = searchMetaCapabilities([]string{"模型", "调度"})
	if len(results) == 0 {
		t.Fatal("search for [模型 调度] should return results")
	}
	// No match returns empty.
	results = searchMetaCapabilities([]string{"不存在的关键字xyz"})
	if len(results) != 0 {
		t.Errorf("search for nonexistent keyword should return empty, got %d", len(results))
	}
}

// TestIntrospectCapabilityTool verifies the introspect_capability tool callback.
func TestIntrospectCapabilityTool(t *testing.T) {
	i18n.Init("zh")
	a := &Agent{}

	// By ID.
	out, err := a.introspectCapabilityTool(context.Background(), map[string]interface{}{
		"id": "cap.self-modify",
	})
	if err != nil {
		t.Fatalf("by ID: unexpected error: %v", err)
	}
	if !strings.Contains(out, "cap.self-modify") || !strings.Contains(out, "自我改造") {
		t.Errorf("by ID output missing capability info:\n%s", out)
	}

	// Unknown ID returns error.
	_, err = a.introspectCapabilityTool(context.Background(), map[string]interface{}{
		"id": "cap.nonexistent",
	})
	if err == nil {
		t.Error("unknown ID should return error")
	}

	// By keywords.
	out, err = a.introspectCapabilityTool(context.Background(), map[string]interface{}{
		"keywords": []interface{}{"模型", "调度"},
	})
	if err != nil {
		t.Fatalf("by keywords: unexpected error: %v", err)
	}
	if !strings.Contains(out, "cap.model-routing") {
		t.Errorf("by keywords output missing model-routing:\n%s", out)
	}

	// No args returns full index.
	out, err = a.introspectCapabilityTool(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("no args: unexpected error: %v", err)
	}
	if !strings.Contains(out, "cap.self-modify") {
		t.Errorf("no args output missing index:\n%s", out)
	}
}

// TestCapabilitiesSectionInjection verifies that the CAPABILITIES section
// injects the meta-capability index when meta-capability awareness is enabled,
// and does not when disabled.
func TestCapabilitiesSectionInjection(t *testing.T) {
	i18n.Init("zh")
	env := &promptEnv{cwd: t.TempDir()}

	// Enabled: index should be injected.
	enabledCfg := &config.Config{}
	enabledCfg.LLM.MetaCapabilityEnabled = true
	out := buildNamedSection("Capabilities", env, enabledCfg, false, nil, true)
	if !strings.Contains(out, "cap.self-modify") {
		t.Errorf("enabled: CAPABILITIES should contain meta-capability index:\n%s", out)
	}

	// Disabled: index should NOT be injected.
	disabledCfg := &config.Config{}
	disabledCfg.LLM.MetaCapabilityEnabled = false
	out = buildNamedSection("Capabilities", env, disabledCfg, false, nil, true)
	if strings.Contains(out, "cap.self-modify") {
		t.Errorf("disabled: CAPABILITIES should NOT contain meta-capability index:\n%s", out)
	}
}

// TestArgStringSlice verifies the keyword array parsing helper.
func TestArgStringSlice(t *testing.T) {
	got := argStringSlice(map[string]interface{}{
		"keywords": []interface{}{"a", "b", "c"},
	}, "keywords")
	if len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Errorf("argStringSlice = %v, want [a b c]", got)
	}
	// Non-array returns nil.
	if argStringSlice(map[string]interface{}{"keywords": "not-array"}, "keywords") != nil {
		t.Error("non-array should return nil")
	}
	// Absent returns nil.
	if argStringSlice(map[string]interface{}{}, "keywords") != nil {
		t.Error("absent should return nil")
	}
}
