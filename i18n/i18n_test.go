package i18n

import "testing"

func TestTMissingKeyReturnsKey(t *testing.T) {
	const missingKey = "no_such_key_xyz"
	if got := T(missingKey); got != missingKey {
		t.Errorf("T(%q) = %q, want %q", missingKey, got, missingKey)
	}
}
