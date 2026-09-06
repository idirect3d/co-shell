package gateway

import (
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// agentDefaults is the JSON returned by GET /api/agent-defaults. It carries the
// values the create-agent form can pre-fill so the user only needs to enter a
// workspace (or a remote URL).
type agentDefaults struct {
	DefaultWorkspace string            `json:"default_workspace"`
	DefaultID        string            `json:"default_id"`
	CoShells         []coShellInfo     `json:"co_shells"`
	ConfigCandidates []configCandidate `json:"config_candidates"`
	RecommendedPort  int               `json:"recommended_port"`
}

// handleAgentDefaults returns detection results + derived defaults for the
// create-agent form.
func (w *WebUI) handleAgentDefaults(rw http.ResponseWriter, _ *http.Request) {
	ws := DefaultWorkspace()
	existing := make([]string, 0, len(w.manager.Agents()))
	for _, a := range w.manager.Agents() {
		existing = append(existing, a.ID)
	}
	writeJSON(rw, http.StatusOK, agentDefaults{
		DefaultWorkspace: ws,
		DefaultID:        DeriveID(ws, existing),
		CoShells:         DetectCoShells(),
		ConfigCandidates: DetectConfigCandidates(ws),
		RecommendedPort:  w.manager.RecommendedPort(),
	})
}

// handleCoShellLocations returns the detected co-shell executables.
func (w *WebUI) handleCoShellLocations(rw http.ResponseWriter, _ *http.Request) {
	writeJSON(rw, http.StatusOK, map[string]interface{}{"co_shells": DetectCoShells()})
}

// handleConfigCandidates returns config.json candidates for a workspace given
// via the ?workspace= query parameter.
func (w *WebUI) handleConfigCandidates(rw http.ResponseWriter, r *http.Request) {
	ws := r.URL.Query().Get("workspace")
	writeJSON(rw, http.StatusOK, map[string]interface{}{"config_candidates": DetectConfigCandidates(ws)})
}

// agentVersionResult is the JSON returned by GET /api/agent-version.
type agentVersionResult struct {
	Kind    string `json:"kind"` // "local" | "remote"
	Version string `json:"version,omitempty"`
	Build   string `json:"build,omitempty"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
}

// handleAgentVersion reports the co-shell version for a local executable
// (?kind=local&path=...) or a remote instance (?kind=remote&url=...). It also
// flags whether the version is compatible with the hub (same major.minor).
func (w *WebUI) handleAgentVersion(rw http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	kind := q.Get("kind")
	res := agentVersionResult{Kind: kind}
	switch kind {
	case "local":
		path := q.Get("path")
		if path == "" {
			writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "path required"})
			return
		}
		res.Version, res.Build = coShellVersion(path)
		res.OK = res.Version != ""
		if !res.OK {
			res.Error = "无法读取 co-shell 版本（文件不存在或不可执行）"
		}
	case "remote":
		url := q.Get("url")
		if url == "" {
			writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "url required"})
			return
		}
		base := wsURLToBase(url)
		v, b, err := remoteVersion(base)
		if err != nil {
			res.Error = "无法连接远程 co-shell: " + err.Error()
		} else {
			res.Version, res.Build, res.OK = v, b, true
		}
	default:
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "kind must be local or remote"})
		return
	}
	writeJSON(rw, http.StatusOK, res)
}

// remoteDefaults is the JSON returned by GET /api/remote-defaults.
type remoteDefaults struct {
	RecommendedPort int   `json:"recommended_port"` // 0 = none free in the scan window
	UsedPorts       []int `json:"used_ports"`       // ports already used by registered agents for this host
}

// remotePortBase is the first port scanned for a free remote-agent port.
const remotePortBase = 28256

// remotePortScan is how many consecutive ports are scanned from remotePortBase.
const remotePortScan = 10

// handleRemoteDefaults returns a recommended free port for a remote agent on
// the given host (?host=...). It scans remotePortBase..+remotePortScan and picks
// the first port whose host:port combination is not already used by a
// registered agent. If none is free it returns recommended_port=0 (the user
// must pick one manually).
func (w *WebUI) handleRemoteDefaults(rw http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	used := w.usedRemotePorts(host)
	rec := 0
	for p := remotePortBase; p < remotePortBase+remotePortScan; p++ {
		if !used[p] {
			rec = p
			break
		}
	}
	writeJSON(rw, http.StatusOK, remoteDefaults{RecommendedPort: rec, UsedPorts: usedPortList(used)})
}

// usedRemotePorts returns the set of ports already used by registered agents
// whose host matches the given host (or all ports when host is empty).
func (w *WebUI) usedRemotePorts(host string) map[int]bool {
	used := map[int]bool{}
	host = strings.ToLower(strings.TrimSpace(host))
	for _, a := range w.manager.Agents() {
		if a.Type == AgentTypeExternal && a.WSURL != "" {
			if u, err := url.Parse(a.WSURL); err == nil {
				aHost := strings.ToLower(u.Hostname())
				if host == "" || aHost == host {
					if p := u.Port(); p != "" {
						if n, err := strconv.Atoi(p); err == nil {
							used[n] = true
						}
					}
				}
			}
		}
	}
	return used
}

// usedPortList converts a used-port set into a sorted slice.
func usedPortList(used map[int]bool) []int {
	out := make([]int, 0, len(used))
	for p := range used {
		out = append(out, p)
	}
	sort.Ints(out)
	return out
}

// versionCompatible reports whether two co-shell versions share the same
// major.minor (a coarse compatibility check). Empty versions are treated as
// compatible (unknown).
func versionCompatible(a, b string) bool {
	if a == "" || b == "" {
		return true
	}
	am := strings.SplitN(a, ".", 3)
	bm := strings.SplitN(b, ".", 3)
	if len(am) < 2 || len(bm) < 2 {
		return true
	}
	return am[0] == bm[0] && am[1] == bm[1]
}
