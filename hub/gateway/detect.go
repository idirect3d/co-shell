package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// coShellVersionRe matches the "co-shell vX.Y.Z [BUILD-N]" version banner.
var coShellVersionRe = regexp.MustCompile(`v?(\d+\.\d+\.\d+)(?:\s+\[BUILD-(\d+)\])?`)

// coShellInfo describes one detected co-shell executable.
type coShellInfo struct {
	Path    string `json:"path"`
	Source  string `json:"source"` // "cwd" | "path"
	Version string `json:"version,omitempty"`
	Build   string `json:"build,omitempty"`
	OK      bool   `json:"ok"` // true if the file exists and is executable
}

// coShellPrefix is the file-name prefix of co-shell executables. Files whose
// name starts with this prefix (e.g. "co-shell", "co-shell-0.44.0.darwin.arm64",
// "co-shell-0.44.0.exe") are candidates for the dropdown.
const coShellPrefix = "co-shell"

// DetectCoShells finds co-shell executables in the current working directory
// and on PATH. It scans every executable whose file name starts with
// "co-shell" (including versioned names like co-shell-0.44.0.darwin.arm64),
// verifies each candidate by running "--version" (only real co-shell binaries
// are listed), and returns them in priority order (cwd first).
func DetectCoShells() []coShellInfo {
	var out []coShellInfo
	seen := map[string]bool{}

	add := func(path, source string) {
		if path == "" {
			return
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			abs = path
		}
		if seen[abs] {
			return
		}
		seen[abs] = true
		info := coShellInfo{Path: abs, Source: source}
		if st, err := os.Stat(abs); err == nil && !st.IsDir() && st.Mode()&0111 != 0 {
			// Verify the candidate is a real co-shell binary by running
			// "--version". Only candidates that report a version are listed, so
			// the UI dropdown never offers a path that cannot run.
			info.Version, info.Build = coShellVersion(abs)
			if info.Version != "" {
				info.OK = true
			}
		}
		if info.OK {
			out = append(out, info)
		}
	}

	// Current working directory: every executable starting with "co-shell".
	if cwd, err := os.Getwd(); err == nil {
		for _, p := range coShellCandidates(cwd) {
			add(p, "cwd")
		}
	}
	// PATH: every executable starting with "co-shell" in each PATH directory.
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		for _, p := range coShellCandidates(dir) {
			add(p, "path")
		}
	}
	return out
}

// coShellCandidates returns the paths of executable files in dir whose name
// starts with the co-shell prefix. On Windows only files with the .exe
// extension are considered (matching how the OS resolves executables).
func coShellCandidates(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, coShellPrefix) {
			continue
		}
		if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(name), ".exe") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.Mode()&0111 == 0 {
			continue
		}
		out = append(out, filepath.Join(dir, name))
	}
	return out
}

// coShellVersion runs "<path> --version" and parses the version/build banner.
func coShellVersion(path string) (version, build string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--version")
	out, err := cmd.Output()
	if err != nil {
		return "", ""
	}
	m := coShellVersionRe.FindStringSubmatch(string(out))
	if len(m) >= 2 {
		version = m[1]
		if len(m) >= 3 {
			build = m[2]
		}
	}
	return version, build
}

// configCandidate describes one detected config.json path.
type configCandidate struct {
	Path string `json:"path"`
	Note string `json:"note,omitempty"`
}

// DetectConfigCandidates returns config.json candidates for a local agent:
// ~/.co-shell/config.json, ./config.json, and {workspace}/config.json.
func DetectConfigCandidates(workspace string) []configCandidate {
	var out []configCandidate
	seen := map[string]bool{}
	add := func(path, note string) {
		if path == "" {
			return
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			abs = path
		}
		if seen[abs] {
			return
		}
		seen[abs] = true
		if _, err := os.Stat(abs); err == nil {
			out = append(out, configCandidate{Path: abs, Note: note})
		}
	}
	home, _ := os.UserHomeDir()
	add(filepath.Join(home, ".co-shell", "config.json"), "~/.co-shell/config.json")
	if cwd, err := os.Getwd(); err == nil {
		add(filepath.Join(cwd, "config.json"), "./config.json")
	}
	if workspace != "" {
		add(filepath.Join(workspace, "config.json"), "{workspace}/config.json")
	}
	return out
}

// DefaultWorkspace returns the default local-agent workspace directory
// (~/.co-shell/agents/agent-1). It does not create the directory.
func DefaultWorkspace() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".co-shell", "agents", "agent-1")
}

// DeriveID returns a unique agent id from a workspace path: the last path
// segment, suffixed with an incrementing number if it collides with existing
// ids.
func DeriveID(workspace string, existing []string) string {
	base := filepath.Base(filepath.Clean(workspace))
	if base == "." || base == string(filepath.Separator) || base == "" {
		base = "agent"
	}
	used := map[string]bool{}
	for _, id := range existing {
		used[id] = true
	}
	if !used[base] {
		return base
	}
	for i := 2; ; i++ {
		cand := fmt.Sprintf("%s-%d", base, i)
		if !used[cand] {
			return cand
		}
	}
}

// remoteVersion queries a co-shell instance's /api/bootstrap for its version
// and build. baseURL is like "http://127.0.0.1:12820".
func remoteVersion(baseURL string) (version, build string, err error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(baseURL + "/api/bootstrap")
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	var body struct {
		Version string `json:"version"`
		Build   string `json:"build"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", "", err
	}
	return body.Version, body.Build, nil
}

// wsURLToBase converts a ws://host:port/ws URL to its http base URL.
func wsURLToBase(wsURL string) string {
	u := strings.TrimPrefix(wsURL, "ws://")
	u = strings.TrimPrefix(u, "wss://")
	if i := strings.IndexByte(u, '/'); i >= 0 {
		u = u[:i]
	}
	return "http://" + u
}
