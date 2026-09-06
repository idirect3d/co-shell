package gateway

import (
	"net/http"
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
