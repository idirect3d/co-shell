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
	"fmt"
	"strings"

	"github.com/idirect3d/co-shell/i18n"
)

// MetaCapability represents a single meta-capability of co-shell.
// Meta-capabilities are the "hidden" abilities that are not exposed as tools
// but as strategies/behaviors the agent can adopt (self-modification, model
// routing, problem-solving strategies, sub-agent collaboration, context
// management, self-configuration, etc.).
//
// Each capability has a stable unique ID (independent of language), a category,
// a name, a short description, and a full detail text. The knowledge base is
// stored in i18n resources (zh/en) so it is built into the binary and evolves
// with development.
type MetaCapability struct {
	// ID is the stable unique identifier (e.g. "cap.self-modify").
	// It does NOT change with language.
	ID string
	// Category groups related capabilities (e.g. "自我改造", "模型调度").
	Category string
	// Name is the capability name (localized).
	Name string
	// Description is a short summary (localized).
	Description string
	// Detail is the full explanation of how to use the capability (localized).
	Detail string
}

// metaCapabilityIDs lists all meta-capability IDs in display order.
// The knowledge base content is loaded from i18n resources keyed by these IDs.
var metaCapabilityIDs = []string{
	"cap.self-modify",
	"cap.model-routing",
	"cap.problem-strategies",
	"cap.subagent-collab",
	"cap.context-management",
	"cap.self-config",
}

// metaCapabilityCategory returns the localized category label for a capability ID.
func metaCapabilityCategory(id string) string {
	switch id {
	case "cap.self-modify", "cap.self-config":
		return i18n.T(i18n.KeyCapCategorySelf)
	case "cap.model-routing":
		return i18n.T(i18n.KeyCapCategoryModel)
	case "cap.problem-strategies":
		return i18n.T(i18n.KeyCapCategoryProblem)
	case "cap.subagent-collab":
		return i18n.T(i18n.KeyCapCategoryCollab)
	case "cap.context-management":
		return i18n.T(i18n.KeyCapCategoryContext)
	default:
		return ""
	}
}

// metaCapabilityName returns the localized name for a capability ID.
func metaCapabilityName(id string) string {
	switch id {
	case "cap.self-modify":
		return i18n.T(i18n.KeyCapSelfModifyName)
	case "cap.model-routing":
		return i18n.T(i18n.KeyCapModelRoutingName)
	case "cap.problem-strategies":
		return i18n.T(i18n.KeyCapProblemStrategiesName)
	case "cap.subagent-collab":
		return i18n.T(i18n.KeyCapSubagentCollabName)
	case "cap.context-management":
		return i18n.T(i18n.KeyCapContextManagementName)
	case "cap.self-config":
		return i18n.T(i18n.KeyCapSelfConfigName)
	default:
		return id
	}
}

// metaCapabilityDescription returns the localized short description for a capability ID.
func metaCapabilityDescription(id string) string {
	switch id {
	case "cap.self-modify":
		return i18n.T(i18n.KeyCapSelfModifyDesc)
	case "cap.model-routing":
		return i18n.T(i18n.KeyCapModelRoutingDesc)
	case "cap.problem-strategies":
		return i18n.T(i18n.KeyCapProblemStrategiesDesc)
	case "cap.subagent-collab":
		return i18n.T(i18n.KeyCapSubagentCollabDesc)
	case "cap.context-management":
		return i18n.T(i18n.KeyCapContextManagementDesc)
	case "cap.self-config":
		return i18n.T(i18n.KeyCapSelfConfigDesc)
	default:
		return ""
	}
}

// metaCapabilityDetail returns the localized full detail text for a capability ID.
func metaCapabilityDetail(id string) string {
	switch id {
	case "cap.self-modify":
		return i18n.T(i18n.KeyCapSelfModifyDetail)
	case "cap.model-routing":
		return i18n.T(i18n.KeyCapModelRoutingDetail)
	case "cap.problem-strategies":
		return i18n.T(i18n.KeyCapProblemStrategiesDetail)
	case "cap.subagent-collab":
		return i18n.T(i18n.KeyCapSubagentCollabDetail)
	case "cap.context-management":
		return i18n.T(i18n.KeyCapContextManagementDetail)
	case "cap.self-config":
		return i18n.T(i18n.KeyCapSelfConfigDetail)
	default:
		return ""
	}
}

// allMetaCapabilities returns all meta-capabilities in display order.
func allMetaCapabilities() []MetaCapability {
	caps := make([]MetaCapability, 0, len(metaCapabilityIDs))
	for _, id := range metaCapabilityIDs {
		caps = append(caps, MetaCapability{
			ID:          id,
			Category:    metaCapabilityCategory(id),
			Name:        metaCapabilityName(id),
			Description: metaCapabilityDescription(id),
			Detail:      metaCapabilityDetail(id),
		})
	}
	return caps
}

// findMetaCapabilityByID returns the capability with the given ID, or nil.
func findMetaCapabilityByID(id string) *MetaCapability {
	for _, c := range allMetaCapabilities() {
		if c.ID == id {
			cp := c
			return &cp
		}
	}
	return nil
}

// searchMetaCapabilities returns capabilities whose ID, name, description, or
// category matches ALL of the given keywords (AND logic, case-insensitive).
// Returns an empty slice when no capability matches.
func searchMetaCapabilities(keywords []string) []MetaCapability {
	if len(keywords) == 0 {
		return nil
	}
	var results []MetaCapability
	for _, c := range allMetaCapabilities() {
		haystack := strings.ToLower(c.ID + " " + c.Category + " " + c.Name + " " + c.Description)
		allMatch := true
		for _, kw := range keywords {
			if kw == "" {
				continue
			}
			if !strings.Contains(haystack, strings.ToLower(kw)) {
				allMatch = false
				break
			}
		}
		if allMatch {
			results = append(results, c)
		}
	}
	return results
}

// argStringSlice returns the string slice value of a named array argument,
// or nil if absent or not an array of strings.
func argStringSlice(args map[string]interface{}, key string) []string {
	v, ok := args[key]
	if !ok {
		return nil
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	var out []string
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// introspectCapabilityTool handles the "introspect_capability" tool call from
// the LLM (FEATURE-466). It queries the meta-capability knowledge base:
//   - with an "id" argument: returns the full detail for that capability
//   - with a "keywords" array: returns capabilities matching ALL keywords
//   - with no arguments: returns the full capability index
func (a *Agent) introspectCapabilityTool(ctx context.Context, args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)
	keywords := argStringSlice(args, "keywords")

	// Priority 1: exact ID lookup.
	if id != "" {
		if c := findMetaCapabilityByID(id); c != nil {
			return formatCapabilityDetail(*c), nil
		}
		return "", fmt.Errorf("capability %q not found. Use introspect_capability with no arguments to list all capabilities.", id)
	}

	// Priority 2: keyword fuzzy search.
	if len(keywords) > 0 {
		results := searchMetaCapabilities(keywords)
		if len(results) == 0 {
			return "No capability matches the given keywords. Use introspect_capability with no arguments to list all capabilities.", nil
		}
		return formatCapabilityList(results), nil
	}

	// Priority 3: full index.
	return buildMetaCapabilityIndex(), nil
}

// formatCapabilityDetail formats a single capability's full detail text.
func formatCapabilityDetail(c MetaCapability) string {
	var sb strings.Builder
	sb.WriteString("ID: " + c.ID + "\n")
	sb.WriteString("Category: " + c.Category + "\n")
	sb.WriteString("Name: " + c.Name + "\n")
	sb.WriteString("Description: " + c.Description + "\n\n")
	sb.WriteString(c.Detail)
	return sb.String()
}

// formatCapabilityList formats a list of capabilities as an index (ID + name +
// description), grouped by category.
func formatCapabilityList(caps []MetaCapability) string {
	var sb strings.Builder
	var catOrder []string
	byCat := make(map[string][]MetaCapability)
	for _, c := range caps {
		if _, seen := byCat[c.Category]; !seen {
			catOrder = append(catOrder, c.Category)
		}
		byCat[c.Category] = append(byCat[c.Category], c)
	}
	for _, cat := range catOrder {
		sb.WriteString("【" + cat + "】\n")
		for _, c := range byCat[cat] {
			sb.WriteString("- " + c.ID + ": " + c.Name + " — " + c.Description + "\n")
		}
		sb.WriteString("\n")
	}
	// FEATURE-482: append the static environment-awareness section describing
	// capabilities perceivable directly from <environment_details> (service
	// mode, user dynamic events, model parameters) without a tool call.
	if env := i18n.T(i18n.KeyCapEnvAwareness); env != "" && env != i18n.KeyCapEnvAwareness {
		sb.WriteString(env)
	}
	return strings.TrimRight(sb.String(), "\n")
}

// buildMetaCapabilityIndex builds the CAPABILITIES index section body listing
// each meta-capability grouped by category (ID + name + description) without
// loading the full detail text. Returns an empty string when there are no
// capabilities.
func buildMetaCapabilityIndex() string {
	caps := allMetaCapabilities()
	if len(caps) == 0 {
		return ""
	}
	var sb strings.Builder
	// Group by category, preserving first-appearance order.
	var catOrder []string
	byCat := make(map[string][]MetaCapability)
	for _, c := range caps {
		if _, seen := byCat[c.Category]; !seen {
			catOrder = append(catOrder, c.Category)
		}
		byCat[c.Category] = append(byCat[c.Category], c)
	}
	for _, cat := range catOrder {
		sb.WriteString("【" + cat + "】\n")
		for _, c := range byCat[cat] {
			sb.WriteString("- " + c.ID + ": " + c.Name + " — " + c.Description + "\n")
		}
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}
