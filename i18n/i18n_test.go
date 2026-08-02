package i18n

import "testing"

func TestTEmptyTranslationReturnsEmpty(t *testing.T) {
	orig := GetLang()
	defer SetLang(string(orig))

	if !SetLang("zh") {
		t.Fatal("SetLang zh failed")
	}
	if got := T(KeySystemPromptToolUsage); got != "" {
		t.Errorf("T(tool_usage) zh = %q, want empty", got)
	}

	if !SetLang("en") {
		t.Fatal("SetLang en failed")
	}
	if got := T(KeySystemPromptToolUsage); got != "" {
		t.Errorf("T(tool_usage) en = %q, want empty", got)
	}
}

func TestTMissingKeyReturnsKey(t *testing.T) {
	const missingKey = "no_such_key_xyz"
	if got := T(missingKey); got != missingKey {
		t.Errorf("T(%q) = %q, want %q", missingKey, got, missingKey)
	}
}
