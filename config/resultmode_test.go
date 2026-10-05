package config

import "testing"

// TestResultModeMachine verifies the machine level is parsed and stringified
// correctly (FEATURE-610), and that existing levels are unaffected.
func TestResultModeMachine(t *testing.T) {
	// Parse "machine" -> ResultModeMachine.
	m, ok := ParseResultMode("machine")
	if !ok {
		t.Fatal("ParseResultMode(\"machine\") should succeed")
	}
	if m != ResultModeMachine {
		t.Errorf("ParseResultMode(\"machine\") = %d, want %d", m, ResultModeMachine)
	}
	// String round-trip.
	if s := ResultModeString(ResultModeMachine); s != "machine" {
		t.Errorf("ResultModeString(ResultModeMachine) = %q, want \"machine\"", s)
	}
	// Existing levels still parse.
	for _, tc := range []struct {
		s    string
		want ResultMode
	}{
		{"minimal", ResultModeMinimal},
		{"explain", ResultModeExplain},
		{"analyze", ResultModeAnalyze},
		{"free", ResultModeFree},
	} {
		got, ok := ParseResultMode(tc.s)
		if !ok || got != tc.want {
			t.Errorf("ParseResultMode(%q) = %d,%v want %d,true", tc.s, got, ok, tc.want)
		}
	}
	// Invalid string returns false.
	if _, ok := ParseResultMode("bogus"); ok {
		t.Error("ParseResultMode(\"bogus\") should fail")
	}
	// ResultModeMachine is the 5th value (4), preserving 0-3 compatibility.
	if ResultModeMachine != 4 {
		t.Errorf("ResultModeMachine = %d, want 4", ResultModeMachine)
	}
}
