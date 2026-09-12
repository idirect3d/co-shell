package i18n

import (
	"strings"
	"testing"
)

// FIX-510: the XML-mode meta section carries the {NO_META_TOOLS} placeholder
// (filled at runtime from the tool schemas) and the hint template must accept
// the tool list through %s.
func TestMetaExemptToolsPlaceholders(t *testing.T) {
	Init("en")
	defer Init("zh")

	metaXML := T(KeySystemPromptToolUsageMetaXML)
	if !strings.Contains(metaXML, "{NO_META_TOOLS}") {
		t.Error("XML meta section must contain the {NO_META_TOOLS} placeholder")
	}

	// OpenAI mode ships the tool schema itself, so no textual list is needed.
	metaOpenAI := T(KeySystemPromptToolUsageMetaOpenAI)
	if strings.Contains(metaOpenAI, "{NO_META_TOOLS}") {
		t.Error("OpenAI meta section must not contain an unreplaced placeholder")
	}

	tmpl := T(KeySystemPromptMetaExemptTools)
	if !strings.Contains(tmpl, "%s") {
		t.Errorf("exempt-tool hint template must contain %%s, got %q", tmpl)
	}
}
