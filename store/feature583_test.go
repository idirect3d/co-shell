// Author: L.Shuang
// Created: 2026-10-02
//
// MIT License - Copyright (c) 2026 L.Shuang
//
// FEATURE-583 UC-0018: session records written before the feature (no
// loaded_skills field) stay readable and mean "no skill loaded".

package store

import (
	"encoding/json"
	"testing"
)

func TestFeature583UC0018LegacySessionEntry(t *testing.T) {
	legacy := []byte(`{"id":"sess-1","title":"old","keywords":"","messages":"W10=","message_count":0}`)
	var e SessionEntry
	if err := json.Unmarshal(legacy, &e); err != nil {
		t.Fatalf("legacy record must stay readable: %v", err)
	}
	if e.LoadedSkills != nil {
		t.Errorf("legacy LoadedSkills = %v, want nil (empty set)", e.LoadedSkills)
	}
	if e.ID != "sess-1" || e.Title != "old" {
		t.Errorf("legacy fields lost: %+v", e)
	}

	// The new field round-trips and is omitted when empty.
	e.LoadedSkills = []string{"demo"}
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back SessionEntry
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(back.LoadedSkills) != 1 || back.LoadedSkills[0] != "demo" {
		t.Errorf("LoadedSkills round trip = %v, want [demo]", back.LoadedSkills)
	}
	empty, err := json.Marshal(SessionEntry{ID: "s2"})
	if err != nil {
		t.Fatalf("marshal empty: %v", err)
	}
	if string(empty) == "" || bytesContains(empty, "loaded_skills") {
		t.Errorf("empty LoadedSkills must be omitted: %s", empty)
	}
}

func bytesContains(b []byte, sub string) bool {
	return len(sub) > 0 && len(b) >= len(sub) && string(b) != "" &&
		indexOf(string(b), sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
