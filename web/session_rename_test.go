// Package web - renameSession title-lock/unlock tests (FEATURE-500).
//
// Author: L.Shuang
// Created: 2026-09-10
// MIT License - Copyright (c) 2026 L.Shuang

package web

import (
	"strings"
	"testing"
	"time"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/store"
)

// newRenameFixture builds a WebSession with a current named session whose
// title is set to the given title, and returns the session + agent.
func newRenameFixture(t *testing.T, title string) (*WebSession, *agent.Agent) {
	t.Helper()
	_, sess, ag, _ := newSessionFixture(t)
	ws, ok := sess.(*WebSession)
	if !ok {
		t.Fatalf("session is not *WebSession")
	}
	now := time.Now()
	entry := &store.SessionEntry{
		ID:           "sess-rename",
		Title:        title,
		Keywords:     "",
		Messages:     []byte("[]"),
		MessageCount: 0,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := ag.Store().SaveNamedSession(entry); err != nil {
		t.Fatalf("SaveNamedSession: %v", err)
	}
	ag.SetCurrentSessionID("sess-rename")
	return ws, ag
}

// loadTitle loads the current session's title from the store.
func loadTitle(t *testing.T, ag *agent.Agent) string {
	t.Helper()
	entry, found, err := ag.Store().LoadNamedSession("sess-rename")
	if err != nil || !found || entry == nil {
		t.Fatalf("LoadNamedSession: found=%v err=%v", found, err)
	}
	return entry.Title
}

// TestRenameSessionLocksWithDollar verifies a non-empty manual title is
// persisted with a "$" prefix (FEATURE-500, UC-001).
func TestRenameSessionLocksWithDollar(t *testing.T) {
	ws, _ := newRenameFixture(t, "新会话1")
	ws.renameSession("我的项目")
	if got := loadTitle(t, ws.ag); got != "$我的项目" {
		t.Errorf("title = %q, want %q", got, "$我的项目")
	}
}

// TestRenameSessionStripsExistingDollar verifies re-editing a locked title
// does not stack "$" prefixes (FEATURE-500, UC-005).
func TestRenameSessionStripsExistingDollar(t *testing.T) {
	ws, _ := newRenameFixture(t, "$我的项目")
	ws.renameSession("$我的项目")
	if got := loadTitle(t, ws.ag); got != "$我的项目" {
		t.Errorf("title = %q, want %q", got, "$我的项目")
	}
}

// TestRenameSessionClearRestoresDefault verifies clearing the title restores
// the default placeholder title and unlocks it (FEATURE-500, UC-003).
func TestRenameSessionClearRestoresDefault(t *testing.T) {
	ws, _ := newRenameFixture(t, "$我的项目")
	ws.renameSession("")
	got := loadTitle(t, ws.ag)
	if strings.HasPrefix(got, "$") {
		t.Errorf("title = %q, want no $ prefix (unlocked)", got)
	}
	if !strings.HasPrefix(got, "新会话") && !strings.HasPrefix(got, "New session") {
		t.Errorf("title = %q, want default placeholder title", got)
	}
}

// TestRenameSessionEmptyKeepsDefault verifies renaming with an empty title on
// an already-default session keeps a default placeholder (no crash, no $).
func TestRenameSessionEmptyKeepsDefault(t *testing.T) {
	ws, _ := newRenameFixture(t, "新会话1")
	ws.renameSession("")
	got := loadTitle(t, ws.ag)
	if strings.HasPrefix(got, "$") {
		t.Errorf("title = %q, want no $ prefix", got)
	}
}
