package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
)

// Agent type constants.
const (
	// AgentTypeManaged: hub creates the workspace and manages the co-shell
	// --serve subprocess lifecycle (start/stop/monitor).
	AgentTypeManaged = "managed"
	// AgentTypeExternal: an externally running co-shell service that hub only
	// connects to (no lifecycle management).
	AgentTypeExternal = "external"
)

// AgentSpec is the persisted configuration of one agent. FEATURE-538: the ID is
// the only identity (and the board's unique key); the former "职责描述" Name
// field is gone — that text now lives in {workspace}/IDENTITY.md.
type AgentSpec struct {
	ID   string `json:"id"`
	Type string `json:"type"` // "managed" | "external"

	// Managed fields.
	Workspace  string `json:"workspace,omitempty"`
	Port       int    `json:"port,omitempty"`
	ConfigFile string `json:"config_file,omitempty"` // optional empty config.json name
	CoShell    string `json:"co_shell,omitempty"`    // optional per-agent co-shell executable (defaults to manager's)
	ConfigPath string `json:"config_path,omitempty"` // optional explicit config.json path
	// UseSharedConfig: true uses ~/.co-shell/config.json; false uses
	// {workspace}/config.json (created empty if absent).
	UseSharedConfig bool `json:"use_shared_config,omitempty"`
	// ExtraArgs: extra command-line arguments appended when launching the
	// managed co-shell subprocess (in addition to the system-supplied ones).
	ExtraArgs string `json:"extra_args,omitempty"`

	// YOLO (FEATURE-528): when true the managed subprocess is started with the
	// --yolo flag, which auto-approves every tool call. Opt-in only: the flag
	// is never passed while this is false (the zero value, so existing registry
	// entries keep their behaviour).
	YOLO bool `json:"yolo,omitempty"`

	// DownloadEnabled (FEATURE-607): when true the managed subprocess is
	// started with the --download-enabled flag, which lets remote clients
	// (including mobile clients reaching the agent through this hub) download
	// workspace files from the Web UI. Opt-in only: the flag is never passed
	// while this is false (the zero value, so existing registry entries keep
	// their behaviour).
	DownloadEnabled bool `json:"download_enabled,omitempty"`

	// AutoStart (FEATURE-626): when true the hub starts this managed agent
	// automatically on hub startup, and the Web UI focuses the first such agent
	// in the list. Opt-in only (the zero value keeps existing behaviour).
	AutoStart bool `json:"auto_start,omitempty"`

	// External field.
	WSURL string `json:"ws_url,omitempty"`
}

// runningAgent tracks a live co-shell subprocess for a managed agent.
type runningAgent struct {
	spec   AgentSpec
	cmd    *exec.Cmd
	cancel context.CancelFunc
}

// Manager owns the agent registry (persisted to a JSON file) and the lifecycle
// of managed co-shell --serve subprocesses. It exposes the current set of
// agent specs to the Proxy so it can connect to each.
type Manager struct {
	mu       sync.Mutex
	path     string // config file path for persistence
	coShell  string // co-shell executable path
	basePort int    // starting port for auto-allocation
	agents   []AgentSpec
	running  map[string]*runningAgent
	ctx      context.Context
}

// NewManager loads the agent registry from path (creating it if absent).
func NewManager(ctx context.Context, path, coShell string, basePort int) (*Manager, error) {
	m := &Manager{
		path:     path,
		coShell:  coShell,
		basePort: basePort,
		running:  make(map[string]*runningAgent),
		ctx:      ctx,
	}
	if err := m.load(); err != nil {
		return nil, err
	}
	return m, nil
}

// load reads the registry JSON file if present.
func (m *Manager) load() error {
	data, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &m.agents)
}

// save persists the registry to the JSON file.
func (m *Manager) save() error {
	data, err := json.MarshalIndent(m.agents, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.path, data, 0644)
}

// CoShellPath returns the configured co-shell executable path used to launch
// managed agents.
func (m *Manager) CoShellPath() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.coShell
}

// Agents returns a copy of the current agent specs.
func (m *Manager) Agents() []AgentSpec {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]AgentSpec, len(m.agents))
	copy(out, m.agents)
	return out
}

// Reorder rewrites the registry order to match ids and persists it
// (FEATURE-613). The list must cover every registered agent exactly once —
// compared case-insensitively and trimmed, like findAgentLocked — so a stale
// or partial list from another browser can never drop a row. Any missing,
// unknown or duplicated id is rejected before the in-memory order changes,
// leaving the registry untouched.
func (m *Manager) Reorder(ids []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(ids) != len(m.agents) {
		return fmt.Errorf("%w: got %d ids for %d agents", ErrInvalidAgentOrder, len(ids), len(m.agents))
	}
	byID := make(map[string]AgentSpec, len(m.agents))
	for _, a := range m.agents {
		byID[strings.ToLower(a.ID)] = a
	}
	ordered := make([]AgentSpec, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		key := strings.ToLower(strings.TrimSpace(id))
		if seen[key] {
			return fmt.Errorf("%w: duplicate id %q", ErrInvalidAgentOrder, id)
		}
		a, ok := byID[key]
		if !ok {
			return fmt.Errorf("%w: unknown id %q", ErrInvalidAgentOrder, id)
		}
		seen[key] = true
		ordered = append(ordered, a)
	}
	m.agents = ordered
	return m.save()
}

// ID validation errors (FEATURE-538). Callers map ErrAgentExists to 409 and
// ErrInvalidAgentID to 400.
var (
	// ErrAgentExists marks a create/add rejected because the ID is taken.
	ErrAgentExists = errors.New("agent already exists")
	// ErrInvalidAgentID marks a create/add rejected because the ID is unusable.
	ErrInvalidAgentID = errors.New("invalid agent id")
	// ErrInvalidAgentOrder marks a reorder request whose id list does not
	// describe the registry exactly (FEATURE-613).
	ErrInvalidAgentOrder = errors.New("invalid agent order")
)

// idForbiddenChars are the characters an agent ID may not contain: they break
// filesystem paths, URLs and the ID's use as a single path segment.
const idForbiddenChars = `/\:*?"<>|`

// validateAgentID normalizes and validates an agent ID: it must be non-empty,
// contain no whitespace and none of idForbiddenChars. The trimmed ID is
// returned on success.
func validateAgentID(id string) (string, error) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return "", fmt.Errorf("%w: id is required", ErrInvalidAgentID)
	}
	for _, r := range trimmed {
		if unicode.IsSpace(r) {
			return "", fmt.Errorf("%w: id must not contain whitespace", ErrInvalidAgentID)
		}
		if strings.ContainsRune(idForbiddenChars, r) {
			return "", fmt.Errorf("%w: id must not contain %q", ErrInvalidAgentID, string(r))
		}
	}
	return trimmed, nil
}

// findAgentLocked returns the index of the agent with the given ID, comparing
// case-insensitively so "Agent-1" and "agent-1" cannot coexist. The caller must
// hold the manager lock.
func (m *Manager) findAgentLocked(id string) int {
	for i := range m.agents {
		if strings.EqualFold(m.agents[i].ID, id) {
			return i
		}
	}
	return -1
}

// Spec returns the agent spec with the given ID. The comparison is
// case-insensitive, matching findAgentLocked.
func (m *Manager) Spec(id string) (AgentSpec, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if i := m.findAgentLocked(id); i >= 0 {
		return m.agents[i], true
	}
	return AgentSpec{}, false
}

// portInUse reports whether the given TCP port on loopback is already bound by
// some process on the system (not just by a registered managed agent).
func portInUse(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return true
	}
	ln.Close()
	return false
}

// nextPort returns the next free port starting from basePort, skipping ports
// already registered to managed agents.
func (m *Manager) nextPort() int {
	used := map[int]bool{}
	for _, a := range m.agents {
		if a.Type == AgentTypeManaged && a.Port > 0 {
			used[a.Port] = true
		}
	}
	for p := m.basePort; ; p++ {
		if !used[p] {
			return p
		}
	}
}

// RecommendedPort returns a free port for a new managed agent: it scans upward
// from basePort, skipping both ports already registered to managed agents and
// ports actually bound on the system.
func (m *Manager) RecommendedPort() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	used := map[int]bool{}
	for _, a := range m.agents {
		if a.Type == AgentTypeManaged && a.Port > 0 {
			used[a.Port] = true
		}
	}
	for p := m.basePort; ; p++ {
		if !used[p] && !portInUse(p) {
			return p
		}
	}
}

// CreateManaged registers a new managed agent. If workspace does not exist it
// is created. useSharedConfig selects the config source: true uses
// ~/.co-shell/config.json (shared); false uses {workspace}/config.json, which
// is copied from ~/.co-shell/config.json when absent, or created empty if the
// shared config does not exist either (a per-agent config). coShell and
// configPath are optional per-agent overrides (empty = use the manager's
// defaults). port > 0 uses the caller-specified port (which must be free);
// port == 0 auto-allocates.
func (m *Manager) CreateManaged(id, workspace, coShell, configPath string, createConfig bool, useSharedConfig bool, port int, extraArgs string, yolo bool, downloadEnabled bool) (AgentSpec, error) {
	id, err := validateAgentID(id)
	if err != nil {
		return AgentSpec{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.findAgentLocked(id) >= 0 {
		return AgentSpec{}, fmt.Errorf("%w: %q", ErrAgentExists, id)
	}
	if err := os.MkdirAll(workspace, 0755); err != nil {
		return AgentSpec{}, fmt.Errorf("create workspace: %w", err)
	}
	if port == 0 {
		port = m.nextPort()
	} else if portInUse(port) {
		return AgentSpec{}, fmt.Errorf("port %d is already in use", port)
	}
	spec := AgentSpec{
		ID:              id,
		Type:            AgentTypeManaged,
		Workspace:       workspace,
		Port:            port,
		CoShell:         coShell,
		ConfigPath:      configPath,
		UseSharedConfig: useSharedConfig,
		ExtraArgs:       extraArgs,
		YOLO:            yolo,
		DownloadEnabled: downloadEnabled,
	}
	// Per-agent config: ensure {workspace}/config.json exists (never overwrite
	// an existing one). When absent, first try to copy ~/.co-shell/config.json
	// (the shared config) so the per-agent config inherits the user's settings;
	// if that shared config does not exist either, create an empty file.
	// Shared config needs no file.
	if !useSharedConfig {
		cfgPath := filepath.Join(workspace, "config.json")
		if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
			if err := copySharedConfig(cfgPath); err != nil {
				return AgentSpec{}, fmt.Errorf("create config: %w", err)
			}
		}
		spec.ConfigFile = "config.json"
	}
	m.agents = append(m.agents, spec)
	if err := m.save(); err != nil {
		return AgentSpec{}, err
	}
	return spec, nil
}

// AddExternal registers an external agent reachable at wsURL. The ID follows the
// same rules as a managed agent (FEATURE-538).
func (m *Manager) AddExternal(id, wsURL string) (AgentSpec, error) {
	id, err := validateAgentID(id)
	if err != nil {
		return AgentSpec{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.findAgentLocked(id) >= 0 {
		return AgentSpec{}, fmt.Errorf("%w: %q", ErrAgentExists, id)
	}
	spec := AgentSpec{ID: id, Type: AgentTypeExternal, WSURL: wsURL}
	m.agents = append(m.agents, spec)
	if err := m.save(); err != nil {
		return AgentSpec{}, err
	}
	return spec, nil
}

// ErrAgentNotFound is returned by Update when the target agent ID is unknown.
var ErrAgentNotFound = errors.New("agent not found")

// ErrInvalidAgent marks an Update rejected because the supplied values are not
// usable (blank workspace/ws_url, out-of-range port). Callers map it to a 400.
var ErrInvalidAgent = errors.New("invalid agent config")

// AgentPatch carries the editable fields of an agent for Manager.Update. A nil
// field is left unchanged; the agent's ID and type are immutable.
type AgentPatch struct {
	Workspace       *string
	Port            *int
	CoShell         *string
	UseSharedConfig *bool
	ExtraArgs       *string
	YOLO            *bool
	DownloadEnabled *bool
	AutoStart       *bool
	WSURL           *string
}

// Update applies patch to the agent with the given ID and persists the registry.
// It validates the workspace and the port (rejecting a port already bound on the
// system) and keeps {workspace}/config.json in place when a per-agent config is
// selected. A running managed subprocess is deliberately left untouched: port /
// workspace / co-shell / extra-args changes take effect on the next start.
func (m *Manager) Update(id string, patch AgentPatch) (AgentSpec, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	idx := m.findAgentLocked(id)
	if idx < 0 {
		return AgentSpec{}, fmt.Errorf("%w: %q", ErrAgentNotFound, id)
	}
	spec := m.agents[idx]

	if spec.Type == AgentTypeExternal {
		if patch.WSURL != nil {
			wsURL := strings.TrimSpace(*patch.WSURL)
			if wsURL == "" {
				return AgentSpec{}, fmt.Errorf("%w: ws_url is required", ErrInvalidAgent)
			}
			spec.WSURL = wsURL
		}
	} else {
		if patch.Workspace != nil {
			workspace := strings.TrimSpace(*patch.Workspace)
			if workspace == "" {
				return AgentSpec{}, fmt.Errorf("%w: workspace is required", ErrInvalidAgent)
			}
			if err := os.MkdirAll(workspace, 0755); err != nil {
				return AgentSpec{}, fmt.Errorf("create workspace: %w", err)
			}
			spec.Workspace = workspace
		}
		if patch.Port != nil {
			port := *patch.Port
			if port < 1 || port > 65535 {
				return AgentSpec{}, fmt.Errorf("%w: invalid port %d", ErrInvalidAgent, port)
			}
			// The port in use by this very agent must stay acceptable.
			if port != spec.Port && portInUse(port) {
				return AgentSpec{}, fmt.Errorf("port %d is already in use", port)
			}
			spec.Port = port
		}
		if patch.CoShell != nil {
			spec.CoShell = strings.TrimSpace(*patch.CoShell)
		}
		if patch.ExtraArgs != nil {
			spec.ExtraArgs = strings.TrimSpace(*patch.ExtraArgs)
		}
		if patch.YOLO != nil {
			spec.YOLO = *patch.YOLO
		}
		if patch.DownloadEnabled != nil {
			spec.DownloadEnabled = *patch.DownloadEnabled
		}
		if patch.AutoStart != nil {
			spec.AutoStart = *patch.AutoStart
		}
		if patch.UseSharedConfig != nil {
			spec.UseSharedConfig = *patch.UseSharedConfig
		}
		// Per-agent config: ensure {workspace}/config.json exists when a
		// non-shared config is selected (never overwrite). When absent, first
		// try to copy ~/.co-shell/config.json (the shared config) so the
		// per-agent config inherits the user's settings; if that shared config
		// does not exist either, create an empty file.
		if !spec.UseSharedConfig {
			cfgPath := filepath.Join(spec.Workspace, "config.json")
			if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
				if err := copySharedConfig(cfgPath); err != nil {
					return AgentSpec{}, fmt.Errorf("create config: %w", err)
				}
			}
			spec.ConfigFile = "config.json"
		}
	}

	m.agents[idx] = spec
	if err := m.save(); err != nil {
		return AgentSpec{}, err
	}
	return spec, nil
}

// SetPort updates a managed agent's serve port.
func (m *Manager) SetPort(id string, port int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.agents {
		if m.agents[i].ID == id && m.agents[i].Type == AgentTypeManaged {
			m.agents[i].Port = port
			return m.save()
		}
	}
	return fmt.Errorf("agent %q not found", id)
}

// Remove deletes an agent spec (stopping it first if running).
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.running[id]; ok {
		r.cancel()
		if r.cmd != nil && r.cmd.Process != nil {
			_ = r.cmd.Process.Kill()
		}
		delete(m.running, id)
	}
	for i := range m.agents {
		if m.agents[i].ID == id {
			m.agents = append(m.agents[:i], m.agents[i+1:]...)
			return m.save()
		}
	}
	return fmt.Errorf("agent %q not found", id)
}

// buildArgs assembles the co-shell command line for a managed agent spec. The
// YOLO (FEATURE-528) and download (FEATURE-607) flags are pure boolean
// switches: each is appended only when the agent explicitly opted in, and they
// sit between the system-supplied arguments and the user's extra arguments.
func buildArgs(spec *AgentSpec) []string {
	args := []string{"--serve", "--port", fmt.Sprintf("%d", spec.Port), "--bind", "127.0.0.1", "-w", spec.Workspace}
	// Config source: shared (~/.co-shell/config.json) or per-agent
	// ({workspace}/config.json). An explicit ConfigPath wins over both.
	if spec.ConfigPath != "" {
		args = append(args, "-c", spec.ConfigPath)
	} else if spec.UseSharedConfig {
		if home, err := os.UserHomeDir(); err == nil {
			args = append(args, "-c", filepath.Join(home, ".co-shell", "config.json"))
		}
	} else if spec.ConfigFile != "" {
		args = append(args, "-c", filepath.Join(spec.Workspace, spec.ConfigFile))
	}
	if spec.YOLO {
		args = append(args, "--yolo")
	}
	if spec.DownloadEnabled {
		args = append(args, "--download-enabled")
	}
	// Append user-supplied extra arguments (whitespace-separated). The default
	// --accept-license is added when the user left the field empty; --serve is
	// already supplied above so it is never duplicated.
	extra := strings.TrimSpace(spec.ExtraArgs)
	if extra == "" {
		extra = "--accept-license"
	}
	for _, a := range strings.Fields(extra) {
		args = append(args, a)
	}
	return args
}

// Start launches the co-shell --serve subprocess for a managed agent and
// returns its WS URL. It is a no-op if the agent is already running.
func (m *Manager) Start(id string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if r, ok := m.running[id]; ok && r.cmd != nil && r.cmd.Process != nil {
		return WSURLForPort(r.spec.Port), nil
	}
	var spec *AgentSpec
	for i := range m.agents {
		if m.agents[i].ID == id {
			spec = &m.agents[i]
			break
		}
	}
	if spec == nil {
		return "", fmt.Errorf("agent %q not found", id)
	}
	if spec.Type != AgentTypeManaged {
		return "", fmt.Errorf("agent %q is not managed", id)
	}

	coShell := spec.CoShell
	if coShell == CoShellLatest {
		// FEATURE-527: a "latest" agent re-scans the search paths on EVERY
		// start and runs the highest-versioned co-shell found. A miss is
		// reported to the caller instead of silently falling back to a
		// different executable.
		resolved, err := ResolveLatestCoShell()
		if err != nil {
			return "", fmt.Errorf("agent %q: %w", id, err)
		}
		coShell = resolved
	} else if coShell == "" {
		coShell = m.coShell
	}
	cmd := exec.Command(coShell, buildArgs(spec)...)
	cmd.Dir = spec.Workspace
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start co-shell: %w", err)
	}
	_, cancel := context.WithCancel(m.ctx)
	m.running[id] = &runningAgent{spec: *spec, cmd: cmd, cancel: cancel}

	// Monitor process exit.
	go func(id string, cmd *exec.Cmd, cancel context.CancelFunc) {
		_ = cmd.Wait()
		cancel()
		m.mu.Lock()
		if r, ok := m.running[id]; ok && r.cmd == cmd {
			delete(m.running, id)
		}
		m.mu.Unlock()
		log.Printf("gateway: managed agent %s exited", id)
	}(id, cmd, cancel)

	log.Printf("gateway: started managed agent %s on port %d", id, spec.Port)
	return WSURLForPort(spec.Port), nil
}

// Stop terminates a running managed agent.
func (m *Manager) Stop(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.running[id]
	if !ok {
		return fmt.Errorf("agent %q is not running", id)
	}
	r.cancel()
	if r.cmd != nil && r.cmd.Process != nil {
		_ = r.cmd.Process.Kill()
	}
	delete(m.running, id)
	return nil
}

// IsRunning reports whether a managed agent subprocess is live.
func (m *Manager) IsRunning(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.running[id]
	return ok && r.cmd != nil && r.cmd.Process != nil
}

// copySharedConfig ensures dst exists by first copying the shared config
// (~/.co-shell/config.json) when it is present, otherwise creating an empty
// config file. It never overwrites an existing dst (callers only invoke it
// after confirming dst is absent).
func copySharedConfig(dst string) error {
	if home, err := os.UserHomeDir(); err == nil {
		src := filepath.Join(home, ".co-shell", "config.json")
		if data, err := os.ReadFile(src); err == nil {
			return os.WriteFile(dst, data, 0644)
		}
	}
	return os.WriteFile(dst, []byte("{}\n"), 0644)
}

// WSURLForPort builds the local WebSocket URL for a serve port.
func WSURLForPort(port int) string {
	return fmt.Sprintf("ws://127.0.0.1:%d/ws", port)
}

// StopAll terminates every running managed agent.
// StartAutoStartable starts every agent whose spec has AutoStart set and that is
// not already running (FEATURE-626). It returns how many agents it started and
// the IDs that failed, so the caller can log them. A single failure never
// prevents the remaining agents (or the hub itself) from coming up.
func (m *Manager) StartAutoStartable() (int, []string) {
	m.mu.Lock()
	var ids []string
	for _, a := range m.agents {
		if a.AutoStart {
			ids = append(ids, a.ID)
		}
	}
	m.mu.Unlock()

	started := 0
	var failed []string
	for _, id := range ids {
		if m.IsRunning(id) {
			started++
			continue
		}
		if _, err := m.Start(id); err != nil {
			failed = append(failed, id)
			continue
		}
		started++
	}
	return started, failed
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, r := range m.running {
		r.cancel()
		if r.cmd != nil && r.cmd.Process != nil {
			_ = r.cmd.Process.Kill()
		}
		delete(m.running, id)
	}
}
