package gateway

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// FEATURE-613: hub agent list reordering. The registry order is the display
// order, so a drag has to reach the registry file (and come back through
// GET /api/agents) — the tests below cover the Manager, the HTTP endpoint and
// the embedded page's long-press sort implementation.

// newReorderManager returns a Manager whose registry holds the given ids in the
// given order. It seeds m.agents directly: these tests are about the registry
// order, not about the agent lifecycle that CreateManaged would touch.
func newReorderManager(t *testing.T, ids ...string) *Manager {
	t.Helper()
	m := newTestManager(t)
	for _, id := range ids {
		m.agents = append(m.agents, AgentSpec{ID: id, Type: AgentTypeManaged})
	}
	if err := m.save(); err != nil {
		t.Fatalf("save registry: %v", err)
	}
	return m
}

func agentIDsOf(specs []AgentSpec) []string {
	out := make([]string, 0, len(specs))
	for _, s := range specs {
		out = append(out, s.ID)
	}
	return out
}

func equalIDs(got []string, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// TestReorderPersistsNewOrder pins A-001 / A-007: Reorder rewrites both the
// in-memory order and the registry file, and a fresh Manager reading that file
// sees the same order (a hub restart keeps the user's arrangement).
func TestReorderPersistsNewOrder(t *testing.T) {
	m := newReorderManager(t, "a", "b", "c")
	if err := m.Reorder([]string{"c", "a", "b"}); err != nil {
		t.Fatalf("Reorder: %v", err)
	}
	if got := agentIDsOf(m.Agents()); !equalIDs(got, "c", "a", "b") {
		t.Fatalf("Agents() order = %v, want [c a b]", got)
	}
	if got := agentIDsOf(readRegistry(t, m)); !equalIDs(got, "c", "a", "b") {
		t.Fatalf("persisted order = %v, want [c a b]", got)
	}

	reloaded, err := NewManager(context.Background(), m.path, "/bin/true", 28600)
	if err != nil {
		t.Fatalf("reload registry: %v", err)
	}
	if got := agentIDsOf(reloaded.Agents()); !equalIDs(got, "c", "a", "b") {
		t.Fatalf("reloaded order = %v, want [c a b]", got)
	}
}

// TestReorderRejectsMismatchedLists pins A-002..A-004 and A-006: a list that
// does not cover the registry exactly once is rejected before anything changes,
// so a stale or partial list from another browser can never drop a row.
func TestReorderRejectsMismatchedLists(t *testing.T) {
	cases := []struct {
		name string
		ids  []string
	}{
		{"too few ids", []string{"a", "b"}},
		{"unknown id", []string{"a", "b", "zzz"}},
		{"duplicate id", []string{"a", "a", "b"}},
		{"empty list", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := newReorderManager(t, "a", "b", "c")
			before := agentIDsOf(m.Agents())
			err := m.Reorder(c.ids)
			if err == nil {
				t.Fatalf("Reorder(%v) succeeded, want an error", c.ids)
			}
			if !errors.Is(err, ErrInvalidAgentOrder) {
				t.Fatalf("error = %v, want ErrInvalidAgentOrder", err)
			}
			if got := agentIDsOf(m.Agents()); !equalIDs(got, before...) {
				t.Fatalf("in-memory order changed on a rejected request: %v", got)
			}
			if got := agentIDsOf(readRegistry(t, m)); !equalIDs(got, before...) {
				t.Fatalf("disk order changed on a rejected request: %v", got)
			}
		})
	}
}

// TestReorderIsCaseInsensitive pins A-005: the id comparison matches
// findAgentLocked, so a browser that sends a differently-cased id still works.
func TestReorderIsCaseInsensitive(t *testing.T) {
	m := newReorderManager(t, "Agent-1", "agent-2")
	if err := m.Reorder([]string{" agent-2 ", "AGENT-1"}); err != nil {
		t.Fatalf("Reorder: %v", err)
	}
	if got := agentIDsOf(m.Agents()); !equalIDs(got, "agent-2", "Agent-1") {
		t.Fatalf("order = %v, want [agent-2 Agent-1]", got)
	}
}

// reorderCall drives PUT /api/agents/order with a raw body through the handler.
func reorderCall(w *WebUI, body string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	w.handleReorderAgents(rr, httptest.NewRequest(http.MethodPut, "/api/agents/order", bytes.NewReader([]byte(body))))
	return rr
}

// TestHandleReorderAgents pins A-008..A-010: a valid list is applied and
// persisted; malformed JSON and lists that do not match the registry are 400
// and leave the stored order alone.
func TestHandleReorderAgents(t *testing.T) {
	m := newReorderManager(t, "a", "b")
	w := &WebUI{manager: m}

	rr := reorderCall(w, `{"ids":["b","a"]}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("valid reorder status = %d, want 200 (%s)", rr.Code, rr.Body.String())
	}
	if got := agentIDsOf(readRegistry(t, m)); !equalIDs(got, "b", "a") {
		t.Fatalf("persisted order = %v, want [b a]", got)
	}

	if rr := reorderCall(w, `{`); rr.Code != http.StatusBadRequest {
		t.Fatalf("malformed JSON status = %d, want 400", rr.Code)
	}
	for _, body := range []string{`{"ids":["a"]}`, `{"ids":["a","zzz"]}`, `{"ids":["b","b"]}`} {
		if rr := reorderCall(w, body); rr.Code != http.StatusBadRequest {
			t.Fatalf("status for %s = %d, want 400", body, rr.Code)
		}
	}
	if got := agentIDsOf(readRegistry(t, m)); !equalIDs(got, "b", "a") {
		t.Fatalf("a rejected request changed the stored order: %v", got)
	}
}

// TestReorderRouteBeatsIDPattern pins A-011: "/api/agents/order" is a literal
// pattern, so it wins over "/api/agents/{id}" — otherwise a reorder PUT would
// be read as an update of an agent literally named "order". The two rules below
// mirror the ones NewWebUI registers.
func TestReorderRouteBeatsIDPattern(t *testing.T) {
	var hit string
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /api/agents/order", func(http.ResponseWriter, *http.Request) { hit = "reorder" })
	mux.HandleFunc("PUT /api/agents/{id}", func(http.ResponseWriter, *http.Request) { hit = "update" })

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPut, "/api/agents/order", nil))
	if hit != "reorder" {
		t.Fatalf("PUT /api/agents/order reached %q, want \"reorder\"", hit)
	}
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPut, "/api/agents/agent-1", nil))
	if hit != "update" {
		t.Fatalf("PUT /api/agents/agent-1 reached %q, want \"update\"", hit)
	}
}

// TestWebUIRegistersReorderRoute keeps the mirror above honest: the route has
// to exist in webui.go, otherwise the comparison would pass on a stale copy.
func TestWebUIRegistersReorderRoute(t *testing.T) {
	src := feat559ReadFile(t, "webui.go")
	if !strings.Contains(src, `mux.HandleFunc("PUT /api/agents/order", w.handleReorderAgents)`) {
		t.Error(`webui.go does not register "PUT /api/agents/order"`)
	}
}

// TestFeature613FrontendSortable pins B-001..B-006 against the embedded page:
// the long-press sort implementation exists, it is gated on a ~300ms hold, the
// 3s poll stands down while it runs, and a rejected upload rolls the order back.
func TestFeature613FrontendSortable(t *testing.T) {
	page := feat559ReadFile(t, feat559StaticPath)
	body := uiFuncBody(page, "function attachSwipe(card)")
	if body == "" {
		t.Fatal("attachSwipe not found in the embedded page")
	}

	// B-001: the implementation and its upload call exist.
	for _, want := range []string{
		"function startSort()",
		"function sortMove(y)",
		"function cancelHold()",
		"function saveOrder()",
		"'/api/agents/order'",
		".agent-wrap.sorting",
		"wrap.dataset.id = a.id;",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the embedded page is missing %q", want)
		}
	}

	// B-002: a ~300ms hold (with a small drift allowance) gates the sort, and
	// the existing swipe release path is untouched.
	// The hold constants live next to DELETE_FADE_PX, outside attachSwipe.
	for _, want := range []string{"SORT_HOLD_MS = 300", "SORT_HOLD_PX = 8"} {
		if !strings.Contains(page, want) {
			t.Errorf("the embedded page is missing %q", want)
		}
	}
	for _, want := range []string{
		"holdTimer = setTimeout(startSort, SORT_HOLD_MS);",
		"if (sorting){ sortMove(y); return; }",
		"setSwipe(card, dx < -32);",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("attachSwipe is missing %q", want)
		}
	}
	// B-003: the 3s poll must not rebuild the list during a sort drag.
	if !strings.Contains(page, "if (sig !== lastAgentsSig && !listSorting){") {
		t.Error("refresh() does not stand down while a sort drag is running")
	}
	for _, want := range []string{"listSorting = true;", "listSorting = false;"} {
		if !strings.Contains(body, want) {
			t.Errorf("attachSwipe does not toggle the flag: %q", want)
		}
	}

	// B-004: a failed upload restores the previous order instead of keeping it.
	for _, want := range []string{"agents = prev;", "alertOrderFailed"} {
		if !strings.Contains(page, want) {
			t.Errorf("the rollback path is missing %q", want)
		}
	}

	// B-005: the poll signature is built in list order, so another browser's
	// reorder (or this one's) is detected instead of being polled over.
	for _, want := range []string{
		"return [a.id, a.name, a.connected, a.running, a.busy, a.type, a.board_state].join('\\u0001');",
		"}).join('\\u0002') + '\\u0003' + (cur || '');",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("agentsSignature is not order-sensitive: missing %q", want)
		}
	}

	// B-006: drifting beyond the allowance, or releasing, cancels the hold.
	for _, want := range []string{
		"if (Math.abs(mx) > SORT_HOLD_PX || Math.abs(my) > SORT_HOLD_PX) cancelHold();",
		"if (holdTimer){ clearTimeout(holdTimer); holdTimer = null; }",
		"cancelHold();\n      if (sorting){ finishSort(); return; }",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the hold-cancel path is missing %q", want)
		}
	}

	// Releasing without ever swapping a row must not fire a request.
	if !strings.Contains(body, "if (swapped) saveOrder();") {
		t.Error("finishSort() uploads even when nothing moved")
	}
}
