package gateway

// FIX-558 test helper: readSourceFile returns a source file of this package
// (go test runs with the package directory as working directory).

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readSourceFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

// TestFix558CreateFormDropsDescription pins A-001/A-002/A-012: the create form
// no longer collects an Agent描述 and the create request no longer carries one.
func TestFix558CreateFormDropsDescription(t *testing.T) {
	static := readSourceFile(t, "webui_static.go")
	if strings.Contains(static, `id="m-name"`) {
		t.Error(`create form still renders the Agent描述 input (id="m-name")`)
	}
	if strings.Contains(static, `agent_description: document.getElementById('m-name')`) {
		t.Error("create form still posts agent_description")
	}
	src := readSourceFile(t, "webui.go")
	if strings.Contains(src, `json:"agent_description,omitempty"`) {
		t.Error("createAgentRequest still declares AgentDescription")
	}
}

// TestFix558PutIdentityEndpointRemoved pins A-006/A-007: the hub no longer
// writes an agent's description (no PUT handler, no frontend call to it).
func TestFix558PutIdentityEndpointRemoved(t *testing.T) {
	src := readSourceFile(t, "webui.go")
	if strings.Contains(src, "handlePutAgentIdentity") {
		t.Error("PUT /api/agents/{id}/identity handler is still present")
	}
	if strings.Contains(src, `mux.HandleFunc("PUT /api/agents/{id}/identity"`) {
		t.Error(`route "PUT /api/agents/{id}/identity" is still registered`)
	}
	static := readSourceFile(t, "webui_static.go")
	if strings.Contains(static, `/identity', { agent_description`) {
		t.Error("edit form still PUTs agent_description to /identity")
	}
}

// TestFix558EditFormDescriptionIsReadonlyLast pins A-005: the description is the
// last item of the edit form and is a plain read-only element, not an input.
func TestFix558EditFormDescriptionIsReadonlyLast(t *testing.T) {
	static := readSourceFile(t, "webui_static.go")
	if strings.Contains(static, `<input id="d-name"`) {
		t.Error("edit form still renders an editable d-name input")
	}
	extra := strings.Index(static, `id="d-extra"`)
	desc := strings.Index(static, `class="ro-text" id="d-name"`)
	if extra == -1 || desc == -1 {
		t.Fatalf("edit-form markers missing (d-extra=%d, desc=%d)", extra, desc)
	}
	if desc < extra {
		t.Error("the Agent描述 must be rendered after every other field")
	}
}

// TestFix558WriteIdentityName pins A-003/A-004: WriteIdentityName creates the
// file when absent and otherwise rewrites only the `## agent-name` section.
func TestFix558WriteIdentityName(t *testing.T) {
	dir := t.TempDir()
	if err := WriteIdentityName(dir, "agent-7"); err != nil {
		t.Fatalf("WriteIdentityName (create): %v", err)
	}
	if got := readIdentity(t, dir); !strings.Contains(got, "## agent-name\nagent-7") {
		t.Errorf("created IDENTITY.md = %q, want `## agent-name` = agent-7", got)
	}

	dir2 := t.TempDir()
	writeIdentity(t, dir2, "intro line\n\n## user-name\n匿名\n\n## agent-description\nold\n")
	if err := WriteIdentityName(dir2, "agent-9"); err != nil {
		t.Fatalf("WriteIdentityName (update): %v", err)
	}
	got := readIdentity(t, dir2)
	for _, want := range []string{"## agent-name\nagent-9", "## user-name\n匿名", "## agent-description\nold", "intro line"} {
		if !strings.Contains(got, want) {
			t.Errorf("updated IDENTITY.md = %q, want it to keep %q", got, want)
		}
	}

	if err := WriteIdentityName("", "agent-1"); err == nil {
		t.Error("WriteIdentityName with an empty workspace must fail")
	}
	if err := WriteIdentityName(filepath.Join(dir, "missing"), "agent-1"); err == nil {
		t.Error("WriteIdentityName with a missing workspace must fail")
	}
}

// TestFix558CreateAgentWritesIDAsName pins A-003 end to end: creating a managed
// agent writes its ID into {workspace}/IDENTITY.md as the agent name.
func TestFix558CreateAgentWritesIDAsName(t *testing.T) {
	mgr, err := NewManager(context.Background(), filepath.Join(t.TempDir(), "hub-agents.json"), "/bin/echo", 30200)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() { mgr.StopAll() })
	w := &WebUI{manager: mgr}

	ws := t.TempDir()
	body, err := json.Marshal(createAgentRequest{ID: "fix558-a", Workspace: ws, CoShell: "/bin/echo", Port: 30201})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	rr := httptest.NewRecorder()
	w.handleCreateAgent(rr, httptest.NewRequest(http.MethodPost, "/api/agents", bytes.NewReader(body)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("handleCreateAgent status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if got := readIdentity(t, ws); !strings.Contains(got, "## agent-name\nfix558-a") {
		t.Errorf("IDENTITY.md = %q, want `## agent-name` = fix558-a", got)
	}
}

// TestFix558IdentityComesFromBoardReport pins A-008..A-011: the description the
// hub shows is the value the agent reported when it last registered, it is
// always read-only, a hub-side fallback value counts as "never reported", and a
// WebUI without a board degrades to empty instead of panicking.
func TestFix558IdentityComesFromBoardReport(t *testing.T) {
	b, err := NewBoard(nil)
	if err != nil {
		t.Fatalf("NewBoard: %v", err)
	}
	t.Cleanup(func() {
		if cerr := b.Close(); cerr != nil {
			t.Errorf("board close: %v", cerr)
		}
	})
	w := &WebUI{board: b}

	// A-008: never registered -> empty and read-only.
	if v := w.identityFor(AgentSpec{ID: "never", Type: AgentTypeManaged}); v.AgentDescription != "" || !v.Readonly {
		t.Errorf("never-registered view = %+v, want empty read-only", v)
	}

	// A-009: the reported value is shown as-is.
	b.Register("reported", "reported", "上报的描述")
	if v := w.identityFor(AgentSpec{ID: "reported", Type: AgentTypeManaged}); v.AgentDescription != "上报的描述" || !v.Readonly {
		t.Errorf("reported view = %+v, want 上报的描述 read-only", v)
	}

	// A-010: a roster duty filled in by the hub-side fallback is not an agent
	// report, so the form keeps showing "未上报".
	b.SetDutyResolver(func(string) (string, bool) { return "文件里的描述", true })
	if info, _ := b.Register("fallback", "fallback", ""); info.DutyFrom != DutySourceHub {
		t.Fatalf("fallback duty source = %q, want %q", info.DutyFrom, DutySourceHub)
	}
	if v := w.identityFor(AgentSpec{ID: "fallback", Type: AgentTypeManaged}); v.AgentDescription != "" {
		t.Errorf("fallback view = %+v, want empty (never reported)", v)
	}

	// A-011: external agents behave the same way and stay read-only.
	if v := w.identityFor(AgentSpec{ID: "ext", Type: AgentTypeExternal}); v.AgentDescription != "" || !v.Readonly {
		t.Errorf("unregistered external view = %+v, want empty read-only", v)
	}
	b.Register("ext", "ext", "外部描述")
	if v := w.identityFor(AgentSpec{ID: "ext", Type: AgentTypeExternal}); v.AgentDescription != "外部描述" || !v.Readonly {
		t.Errorf("external view = %+v, want 外部描述 read-only", v)
	}

	// No board at all: empty, read-only, no panic.
	if v := (&WebUI{}).identityFor(AgentSpec{ID: "x", Type: AgentTypeManaged}); v.AgentDescription != "" || !v.Readonly {
		t.Errorf("board-less view = %+v, want empty read-only", v)
	}
}
