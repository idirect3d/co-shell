package taskplan

import (
	"encoding/json"
	"testing"

	"github.com/idirect3d/co-shell/store"
	"github.com/idirect3d/co-shell/workspace"
)

// newTestManager creates a Manager backed by a temp-workspace store.
func newTestManager(t *testing.T) *Manager {
	t.Helper()
	wsDir := t.TempDir()
	ws, err := workspace.New(wsDir)
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	boltStore, err := store.NewStore(ws)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = boltStore.Close() })
	return NewManager(store.NewDualStore(boltStore, nil))
}

// TestPlanKeySessionIsolation verifies that planKey returns a per-session key
// when a session is bound, and the global key when not (UC-0006).
func TestPlanKeySessionIsolation(t *testing.T) {
	m := newTestManager(t)

	if got := m.planKey(); got != currentPlanKey {
		t.Fatalf("planKey() with no session = %q, want %q", got, currentPlanKey)
	}

	m.SetSessionID("sess-A")
	if got := m.planKey(); got != "current:sess-A" {
		t.Fatalf("planKey() with session sess-A = %q, want %q", got, "current:sess-A")
	}

	m.SetSessionID("sess-B")
	if got := m.planKey(); got != "current:sess-B" {
		t.Fatalf("planKey() with session sess-B = %q, want %q", got, "current:sess-B")
	}
}

// TestSessionPlanIsolation verifies that each session has an independent plan
// and switching sessions loads the target session's plan (UC-0001/0002/0003).
func TestSessionPlanIsolation(t *testing.T) {
	m := newTestManager(t)

	// Session A creates a plan.
	m.SetSessionID("sess-A")
	planA, err := m.UpdateSteps("Plan A", "desc A", []StepInput{
		{Description: "step A1", Status: "[ ]"},
		{Description: "step A2", Status: "[X]"},
	})
	if err != nil {
		t.Fatalf("UpdateSteps A: %v", err)
	}
	if planA == nil || planA.Title != "Plan A" {
		t.Fatalf("plan A not created: %+v", planA)
	}

	// Switch to session B: plan should be empty (UC-0001).
	m.SetSessionID("sess-B")
	planB, err := m.GetCurrent()
	if err != nil {
		t.Fatalf("GetCurrent B: %v", err)
	}
	if planB != nil {
		t.Fatalf("session B should have no plan, got %+v", planB)
	}

	// Session B creates its own plan.
	planB, err = m.UpdateSteps("Plan B", "desc B", []StepInput{
		{Description: "step B1", Status: "[=]"},
	})
	if err != nil {
		t.Fatalf("UpdateSteps B: %v", err)
	}
	if planB == nil || planB.Title != "Plan B" {
		t.Fatalf("plan B not created: %+v", planB)
	}

	// Switch back to session A: plan A should be restored (UC-0002).
	m.SetSessionID("sess-A")
	planA2, err := m.GetCurrent()
	if err != nil {
		t.Fatalf("GetCurrent A: %v", err)
	}
	if planA2 == nil || planA2.Title != "Plan A" || len(planA2.Steps) != 2 {
		t.Fatalf("plan A not restored: %+v", planA2)
	}
	if planA2.Steps[1].Status != StatusCompleted {
		t.Fatalf("plan A step 2 status = %q, want completed", planA2.Steps[1].Status)
	}

	// Switch to B again: plan B intact (UC-0003).
	m.SetSessionID("sess-B")
	planB2, err := m.GetCurrent()
	if err != nil {
		t.Fatalf("GetCurrent B2: %v", err)
	}
	if planB2 == nil || planB2.Title != "Plan B" {
		t.Fatalf("plan B not intact: %+v", planB2)
	}
}

// TestUpdateInOneSessionDoesNotAffectAnother verifies that updating a plan in
// one session does not affect another session's plan (UC-0004).
func TestUpdateInOneSessionDoesNotAffectAnother(t *testing.T) {
	m := newTestManager(t)

	m.SetSessionID("sess-A")
	if _, err := m.UpdateSteps("Plan A", "", []StepInput{{Description: "a1", Status: "[ ]"}}); err != nil {
		t.Fatalf("create A: %v", err)
	}

	m.SetSessionID("sess-B")
	if _, err := m.UpdateSteps("Plan B", "", []StepInput{{Description: "b1", Status: "[ ]"}}); err != nil {
		t.Fatalf("create B: %v", err)
	}

	// Update A's plan (mark step completed).
	m.SetSessionID("sess-A")
	if _, err := m.UpdateSteps("Plan A", "", []StepInput{{Description: "a1", Status: "[X]"}}); err != nil {
		t.Fatalf("update A: %v", err)
	}

	// B's plan must be unaffected.
	m.SetSessionID("sess-B")
	planB, err := m.GetCurrent()
	if err != nil {
		t.Fatalf("GetCurrent B: %v", err)
	}
	if planB == nil || planB.Steps[0].Status != StatusPending {
		t.Fatalf("session B plan affected by A update: %+v", planB)
	}
}

// TestMigrateLegacyPlan verifies that a legacy global plan (stored under the
// bare "current" key) is migrated to the current session on first SetSessionID
// (UC-0005).
func TestMigrateLegacyPlan(t *testing.T) {
	m := newTestManager(t)

	// Simulate a legacy global plan stored under the bare "current" key.
	legacy := &TaskPlan{ID: 1, Title: "Legacy", Steps: []TaskStep{{ID: 1, Description: "old", Status: StatusPending}}}
	if err := m.saveCurrent(legacy); err != nil {
		t.Fatalf("save legacy: %v", err)
	}
	// saveCurrent uses planKey() which is the global key when no session bound.
	if m.sessionID != "" {
		t.Fatalf("expected no session bound")
	}

	// First SetSessionID triggers migration.
	m.SetSessionID("sess-A")

	plan, err := m.GetCurrent()
	if err != nil {
		t.Fatalf("GetCurrent after migrate: %v", err)
	}
	if plan == nil || plan.Title != "Legacy" {
		t.Fatalf("legacy plan not migrated: %+v", plan)
	}

	// The global key should be removed after migration.
	if _, found, _ := m.store.GetContext(currentPlanKey); found {
		t.Fatalf("legacy global key should be removed after migration")
	}
}

// TestMigrateLegacyPlanKeepsExistingSessionPlan verifies that when the current
// session already has its own plan, the legacy global plan is dropped without
// overwriting the session plan.
func TestMigrateLegacyPlanKeepsExistingSessionPlan(t *testing.T) {
	// Build a shared store so two managers see the same data.
	wsDir := t.TempDir()
	ws, err := workspace.New(wsDir)
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	boltStore, err := store.NewStore(ws)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = boltStore.Close() })
	ds := store.NewDualStore(boltStore, nil)

	// Create a session plan first.
	m := NewManager(ds)
	m.SetSessionID("sess-A")
	if _, err := m.UpdateSteps("Session Plan", "", []StepInput{{Description: "s1", Status: "[ ]"}}); err != nil {
		t.Fatalf("create session plan: %v", err)
	}

	// Now inject a legacy global plan (simulate upgrade with existing session plan).
	legacy := &TaskPlan{ID: 99, Title: "Legacy", Steps: []TaskStep{{ID: 1, Description: "old", Status: StatusPending}}}
	data, _ := json.Marshal(legacy)
	if err := ds.SaveContext(currentPlanKey, data); err != nil {
		t.Fatalf("save legacy: %v", err)
	}

	// Re-trigger migration via a fresh manager bound to sess-A on the same store.
	m2 := NewManager(ds)
	m2.SetSessionID("sess-A")

	plan, err := m2.GetCurrent()
	if err != nil {
		t.Fatalf("GetCurrent: %v", err)
	}
	if plan == nil || plan.Title != "Session Plan" {
		t.Fatalf("session plan should be kept, got %+v", plan)
	}
}

// TestArchiveOnlyAffectsCurrentSession verifies that archiving/deleting a plan
// only affects the current session (UC-0007).
func TestArchiveOnlyAffectsCurrentSession(t *testing.T) {
	m := newTestManager(t)

	m.SetSessionID("sess-A")
	if _, err := m.UpdateSteps("Plan A", "", []StepInput{{Description: "a1", Status: "[ ]"}}); err != nil {
		t.Fatalf("create A: %v", err)
	}
	m.SetSessionID("sess-B")
	if _, err := m.UpdateSteps("Plan B", "", []StepInput{{Description: "b1", Status: "[ ]"}}); err != nil {
		t.Fatalf("create B: %v", err)
	}

	// Archive/delete A's plan.
	m.SetSessionID("sess-A")
	if _, err := m.UpdateSteps("", "", nil); err != nil {
		t.Fatalf("archive A: %v", err)
	}
	if plan, _ := m.GetCurrent(); plan != nil {
		t.Fatalf("session A plan should be deleted, got %+v", plan)
	}

	// B's plan must be unaffected.
	m.SetSessionID("sess-B")
	planB, err := m.GetCurrent()
	if err != nil {
		t.Fatalf("GetCurrent B: %v", err)
	}
	if planB == nil || planB.Title != "Plan B" {
		t.Fatalf("session B plan affected by A archive: %+v", planB)
	}
}
