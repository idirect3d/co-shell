package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// coShellVersionRe matches the "co-shell vX.Y.Z [BUILD-N]" version banner.
var coShellVersionRe = regexp.MustCompile(`v?(\d+\.\d+\.\d+)(?:\s+\[BUILD-(\d+)\])?`)

// coShellInfo describes one detected co-shell executable.
type coShellInfo struct {
	Path    string `json:"path"`
	Source  string `json:"source"` // "cwd" | "path" | "hubdir"
	Version string `json:"version,omitempty"`
	Build   string `json:"build,omitempty"`
	OK      bool   `json:"ok"` // true if the file exists and is executable
}

// coShellPrefix is the file-name prefix of co-shell executables. Files whose
// name starts with this prefix (e.g. "co-shell", "co-shell-0.44.0.darwin.arm64",
// "co-shell-0.44.0.exe") are candidates for the dropdown.
const coShellPrefix = "co-shell"

// CoShellLatest is the sentinel stored in AgentSpec.CoShell when the user picks
// the "use the latest version" entry in the hub UI (FEATURE-527). A managed
// agent carrying it resolves its executable on EVERY start: the hub directory
// wins outright, otherwise the highest version across the current directory and
// PATH is used (see ResolveLatestCoShell).
const CoShellLatest = "latest"

// DetectCoShells finds co-shell executables in the current working directory,
// on PATH, and next to the hub executable itself. It scans every executable
// whose file name starts with "co-shell" (including versioned names like
// co-shell-0.44.0.darwin.arm64), verifies each candidate by running
// "--version" (only real co-shell binaries are listed), and returns them in
// priority order (cwd, then PATH, then the hub's own directory).
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
	// Hub's own directory: managed agents are launched with the co-shell that
	// sits next to this binary (see defaultCoShellPath), so expose that copy
	// too. Scanned last so existing CWD/PATH candidates keep their precedence.
	if dir, err := hubDir(); err == nil {
		for _, p := range coShellCandidates(dir) {
			add(p, "hubdir")
		}
	}
	return out
}

// hubDir returns the directory holding the running hub executable, resolving
// symlinks so a linked launcher still maps to the real install directory.
func hubDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(exe)
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	return dir, nil
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

// ResolveLatestCoShell picks the co-shell executable a "latest" agent must run
// (FEATURE-527). It is called on EVERY start, so the choice always reflects the
// executables present at that moment. Selection rules:
//
//	1. a usable co-shell next to the hub binary wins outright (highest version
//	   in that directory);
//	2. otherwise the highest version across the current working directory and
//	   every PATH directory is used.
//
// No usable candidate anywhere is an error: the caller reports it instead of
// silently falling back to a different executable.
func ResolveLatestCoShell() (string, error) {
	hub, _ := hubDir()
	cwd, _ := os.Getwd()
	return resolveLatestCoShell(hub, cwd, filepath.SplitList(os.Getenv("PATH")))
}

// resolveLatestCoShell implements the selection rules with injectable search
// paths, so they can be unit-tested without touching the real environment.
func resolveLatestCoShell(hubDir, cwd string, pathDirs []string) (string, error) {
	if best := bestCoShellIn(hubDir); best != "" {
		return best, nil
	}
	seen := map[string]bool{}
	best, bestVer := "", ""
	for _, dir := range append([]string{cwd}, pathDirs...) {
		if dir == "" || seen[dir] {
			continue
		}
		seen[dir] = true
		for _, p := range coShellCandidates(dir) {
			ver, _ := coShellVersion(p)
			if ver == "" {
				continue // same-prefix file that is not a co-shell binary
			}
			if best == "" || compareCoShellVersion(ver, bestVer) > 0 {
				best, bestVer = p, ver
			}
		}
	}
	if best == "" {
		return "", errors.New("no usable co-shell executable found in the hub directory, the current directory or PATH")
	}
	return best, nil
}

// bestCoShellIn returns the highest-versioned usable co-shell executable in dir
// ("" when the directory holds none).
func bestCoShellIn(dir string) string {
	if dir == "" {
		return ""
	}
	best, bestVer := "", ""
	for _, p := range coShellCandidates(dir) {
		ver, _ := coShellVersion(p)
		if ver == "" {
			continue
		}
		if best == "" || compareCoShellVersion(ver, bestVer) > 0 {
			best, bestVer = p, ver
		}
	}
	return best
}

// compareCoShellVersion compares two "X.Y.Z" version strings numerically
// (returns -1, 0 or 1), so 0.9.0 sorts below 0.10.0 — a plain string compare
// would get that backwards.
func compareCoShellVersion(a, b string) int {
	pa, pb := coShellVersionParts(a), coShellVersionParts(b)
	for i := range pa {
		switch {
		case pa[i] < pb[i]:
			return -1
		case pa[i] > pb[i]:
			return 1
		}
	}
	return 0
}

// coShellVersionParts splits a version string into its three numeric parts;
// missing or non-numeric parts count as 0.
func coShellVersionParts(v string) [3]int {
	var out [3]int
	for i, f := range strings.Split(v, ".") {
		if i >= len(out) {
			break
		}
		if n, err := strconv.Atoi(strings.TrimSpace(f)); err == nil {
			out[i] = n
		}
	}
	return out
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

// agentBusy queries a co-shell instance's /api/status for whether it is
// currently executing a task (FEATURE-499). baseURL is like
// "http://127.0.0.1:12820". Returns false on any error (treat as idle).
func agentBusy(baseURL string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(baseURL + "/api/status")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var body struct {
		Busy bool `json:"busy"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return false
	}
	return body.Busy
}
