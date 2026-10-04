package gateway

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
)

// FEATURE-607: the hub reverse proxy stamps every forwarded request with the
// real client address (X-Client-IP), and each managed agent can opt in to the
// download feature, which is rendered as the --download-enabled flag.

// TestBuildArgsDownloadEnabled verifies the per-agent download switch is a pure
// boolean flag: absent while opted out, present once opted in.
func TestBuildArgsDownloadEnabled(t *testing.T) {
	base := &AgentSpec{ID: "a1", Type: AgentTypeManaged, Workspace: "/tmp/ws", Port: 28256}
	off := buildArgs(base)
	if slices.Contains(off, "--download-enabled") {
		t.Errorf("download off args must not contain --download-enabled: %v", off)
	}

	on := *base
	on.DownloadEnabled = true
	got := buildArgs(&on)
	if !slices.Contains(got, "--download-enabled") {
		t.Errorf("download on args = %v, want --download-enabled", got)
	}
}

// TestManagerDownloadEnabledPersisted covers the DownloadEnabled round trip
// through create, a patch that omits the field, and an explicit disable.
func TestManagerDownloadEnabledPersisted(t *testing.T) {
	m := newTestManager(t)
	spec, err := m.CreateManaged("a1", filepath.Join(t.TempDir(), "ws"), "", "", false, true, 0, "--accept-license", false, true)
	if err != nil {
		t.Fatalf("CreateManaged: %v", err)
	}
	if !spec.DownloadEnabled {
		t.Errorf("created DownloadEnabled = false, want true")
	}
	persisted, ok := findAgent(readRegistry(t, m), "a1")
	if !ok {
		t.Fatal("agent a1 missing from persisted registry")
	}
	if !persisted.DownloadEnabled {
		t.Errorf("persisted DownloadEnabled = false, want true")
	}

	// A patch without the field must leave it untouched.
	if _, err := m.Update("a1", AgentPatch{ExtraArgs: strPtr("--note")}); err != nil {
		t.Fatalf("Update(extra args): %v", err)
	}
	persisted, _ = findAgent(readRegistry(t, m), "a1")
	if !persisted.DownloadEnabled {
		t.Errorf("nil download patch cleared the flag, want unchanged (true)")
	}

	// An explicit false must be applied and persisted.
	if _, err := m.Update("a1", AgentPatch{DownloadEnabled: boolPtr(false)}); err != nil {
		t.Fatalf("Update(download=false): %v", err)
	}
	persisted, _ = findAgent(readRegistry(t, m), "a1")
	if persisted.DownloadEnabled {
		t.Errorf("persisted DownloadEnabled = true after disabling, want false")
	}
}

// TestProxyInjectsClientIP verifies the reverse proxy stamps X-Client-IP with
// the real client address and therefore overwrites anything the client sent.
func TestProxyInjectsClientIP(t *testing.T) {
	var got string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("X-Client-IP")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	u, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}

	m := newTestManager(t)
	m.agents = append(m.agents, AgentSpec{ID: "a1", Type: AgentTypeManaged, Workspace: t.TempDir(), Port: port})

	p := newHTTPReverseProxy(m)
	mux := http.NewServeMux()
	mux.Handle(agentPrefix, p)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// A forged client header must not survive the proxy.
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/agent/a1/api/bootstrap", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Client-IP", "1.2.3.4:9999")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("proxy status = %d, want 200", resp.StatusCode)
	}
	if got == "" || got == "1.2.3.4:9999" {
		t.Fatalf("upstream X-Client-IP = %q, want the real client address", got)
	}
	if _, _, err := net.SplitHostPort(got); err != nil {
		t.Errorf("upstream X-Client-IP = %q, want IP:port", got)
	}
}
