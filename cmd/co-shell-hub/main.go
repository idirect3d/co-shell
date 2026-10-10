// co-shell-hub is the new WebSocket aggregation gateway (FEATURE-484).
// It connects to multiple co-shell agents over WebSocket, exposes a TCP service
// with API Key authentication for mobile clients, and serves a Web UI for
// browser access (default localhost only, whitelist configurable).
//
// The gateway also manages co-shell agent lifecycles: it keeps an agent
// registry (persisted to a JSON file), can create workspaces and launch/stop
// managed co-shell --serve subprocesses, and connects to external agents.
//
// Usage:
//
//	co-shell-hub [flags]
//
// Flags:
//
//	--config PATH        Config file path (default: ./hub-gateway.json)
//	--tcp-addr ADDR      TCP listen address (default 127.0.0.1:27256; the
//	                     default port auto-increments when occupied)
//	--api-key KEY        API key required by TCP clients
//	--no-api-key         Disable TCP API-key auth for this run (also makes the Web UI
//	                     ignore its "require key" switch; config files are not modified)
//	--web-addr ADDR      Web UI listen address (default 127.0.0.1:28256; the
//	                     default port auto-increments when occupied)
//	--whitelist IPS      Web UI access whitelist (comma-separated IPs/CIDR)
//	--registry PATH      Agent registry file (default: ./hub-agents.json)
//	--co-shell-path PATH co-shell executable for managed agents (default: same dir as this binary)
//	--base-port N        First port for auto-allocating managed agents (default:
//	                     the Web UI port + 1)
//	--agent ID=WSURL     External agent endpoint (repeatable, added to registry)
//	--help               Show help
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/idirect3d/co-shell/hub/gateway"
	"github.com/idirect3d/co-shell/web"
)

// hubVersion and hubBuild identify this co-shell-hub build. They track the
// co-shell release they ship with (same version/build numbering).
const (
	hubVersion = "0.108.2"
	hubBuild   = "1285"
)

// Default listen ports for the hub. When neither the command line nor the
// config file specifies an address, the hub listens on defaultBindHost plus
// these ports, incrementing the port until a free one is found. An address that
// IS specified (on the command line or in the config file) is used as-is: when
// its port is occupied the bind fails and startup aborts.
const (
	defaultBindHost = "127.0.0.1"
	defaultTCPPort  = 27256
	defaultWebPort  = 28256
)

// config is the JSON config file shape.
type config struct {
	TCPAddr      string   `json:"tcp_addr"`
	APIKey       string   `json:"api_key"`
	WebAddr      string   `json:"web_addr"`
	Whitelist    []string `json:"whitelist,omitempty"`
	RegistryPath string   `json:"registry_path,omitempty"`
	CoShellPath  string   `json:"co_shell_path,omitempty"`
	BasePort     int      `json:"base_port,omitempty"`
	// SettingsPath is the remote-access settings file (TLS/whitelist/access
	// key). Default: ./hub-settings.json (same search order as the config).
	SettingsPath string `json:"settings_path,omitempty"`
	// BoardDBPath is the message board database (bbolt). Default:
	// hub-board.db next to the agent registry (FEATURE-530).
	BoardDBPath string `json:"board_db_path,omitempty"`
	// BoardRedeliverMax is how many times the hub re-sends a board message
	// whose reply window elapsed before it reports a board_timeout
	// (FEATURE-567). Unset = 1 re-delivery; 0 = never re-send, i.e. the
	// behaviour before FEATURE-567.
	BoardRedeliverMax *int `json:"board_redeliver_max,omitempty"`
	// Lang is the Web UI language ("zh"/"en", FEATURE-559). Empty means
	// auto-detect: --lang > LANG / LC_ALL > zh.
	Lang string `json:"lang,omitempty"`
	// Serve suppresses auto-opening the browser (headless/server mode).
	Serve bool `json:"serve,omitempty"`
}

func main() {
	configPath := flag.String("config", "", "config file path (default: ./hub-gateway.json)")
	tcpAddr := flag.String("tcp-addr", "", "TCP listen address (default 127.0.0.1:27256; the default port auto-increments when occupied)")
	apiKey := flag.String("api-key", "", "API key required by TCP clients")
	noAPIKey := flag.Bool("no-api-key", false, "Disable TCP API-key auth for this run (also ignores the Web UI \"require key\" switch)")
	webAddr := flag.String("web-addr", "", "Web UI listen address (default 127.0.0.1:28256; the default port auto-increments when occupied)")
	lang := flag.String("lang", "", "Web UI language (zh/en, default auto-detect)")
	whitelist := flag.String("whitelist", "", "Web UI access whitelist (comma-separated IPs/CIDR, empty=loopback only)")
	registryPath := flag.String("registry", "", "agent registry file (default: ./hub-agents.json)")
	boardDB := flag.String("board-db", "", "message board database file (default: hub-board.db next to the registry)")
	coShellPath := flag.String("co-shell-path", "", "co-shell executable for managed agents (default: same dir as this binary)")
	basePort := flag.Int("base-port", 0, "first port for auto-allocating managed agents (default: the Web UI port + 1)")
	serve := flag.Bool("serve", false, "serve without auto-opening the browser (headless/server mode)")
	var agents multiFlag
	flag.Var(&agents, "agent", "external agent endpoint as ID=WSURL (repeatable)")
	showHelp := flag.Bool("help", false, "show help")
	flag.Parse()

	if *showHelp {
		printUsage()
		os.Exit(0)
	}

	cfg := loadConfig(*configPath)

	// Apply CLI overrides.
	if *tcpAddr != "" {
		cfg.TCPAddr = *tcpAddr
	}
	if *apiKey != "" {
		cfg.APIKey = *apiKey
	}
	// FIX-606: --no-api-key disables TCP auth for this run (and makes the Web UI
	// ignore its persisted "require key" switch). It wins over the config file;
	// when both --api-key and --no-api-key are given, the one appearing later on
	// the command line wins, e.g. "--api-key K --no-api-key" ends up keyless.
	authDisabled := authDisabledByCLI(*noAPIKey, os.Args[1:])
	if authDisabled {
		cfg.APIKey = ""
		log.Printf("WARNING: TCP auth disabled (--no-api-key); the Web UI \"require key\" switch is ignored for this run (config files are not modified)")
	}
	if *webAddr != "" {
		cfg.WebAddr = *webAddr
	}
	if *lang != "" {
		cfg.Lang = *lang
	}
	if *whitelist != "" {
		cfg.Whitelist = splitList(*whitelist)
	}
	if *registryPath != "" {
		cfg.RegistryPath = *registryPath
	}
	if *coShellPath != "" {
		cfg.CoShellPath = *coShellPath
	}
	if *basePort > 0 {
		cfg.BasePort = *basePort
	}
	if *boardDB != "" {
		cfg.BoardDBPath = *boardDB
	}
	if *serve {
		cfg.Serve = true
	}

	// Resolve the listen addresses. An address supplied on the command line or
	// in the config file counts as explicit and is used as-is: when its port is
	// occupied the bind below fails and startup aborts. An address left at the
	// built-in default auto-increments the port until a free one is found.
	tcpExplicit := cfg.TCPAddr != ""
	cfg.TCPAddr = resolveListenAddr(cfg.TCPAddr, defaultBindHost, defaultTCPPort, tcpExplicit)
	webExplicit := cfg.WebAddr != ""
	cfg.WebAddr = resolveListenAddr(cfg.WebAddr, defaultBindHost, defaultWebPort, webExplicit)
	if !tcpExplicit {
		log.Printf("hub-gateway: TCP address not set, using %s", cfg.TCPAddr)
	}
	if !webExplicit {
		log.Printf("hub-gateway: Web address not set, using %s", cfg.WebAddr)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Resolve defaults for the agent manager. The registry file follows the
	// same search order as the config: an explicit --registry path wins, then
	// ./hub-agents.json (process cwd), then ~/.co-shell/hub-agents.json. The
	// first existing file is used; if none exists, ./hub-agents.json is the
	// default write target.
	if cfg.RegistryPath == "" {
		cfg.RegistryPath = firstExisting("./hub-agents.json", filepath.Join(homeDir(), ".co-shell", "hub-agents.json"))
	}
	if cfg.CoShellPath == "" {
		cfg.CoShellPath = defaultCoShellPath()
	}
	// Resolve to an absolute path: managed co-shell subprocesses run with their
	// workspace as the working directory, so a relative path would break.
	if abs, err := filepath.Abs(cfg.CoShellPath); err == nil {
		cfg.CoShellPath = abs
	}
	// Managed agents default to the port right above the Web UI unless
	// --base-port (or base_port in the config file) explicitly overrides it.
	if cfg.BasePort <= 0 {
		cfg.BasePort = agentBasePort(cfg.WebAddr)
	}

	// Create the agent manager (loads the persisted registry).
	mgr, err := gateway.NewManager(ctx, cfg.RegistryPath, cfg.CoShellPath, cfg.BasePort)
	if err != nil {
		log.Fatalf("agent manager: %v", err)
	}
	defer mgr.StopAll()

	// Register any --agent external endpoints into the registry (idempotent).
	for _, a := range agents {
		id, url, ok := strings.Cut(a, "=")
		if !ok || id == "" || url == "" {
			log.Fatalf("invalid --agent %q (want ID=WSURL)", a)
		}
		if _, err := mgr.AddExternal(id, url); err != nil {
			log.Printf("note: --agent %s not added: %v", id, err)
		}
	}

	// FEATURE-626: bring up every agent marked "auto_start" so the Web UI can
	// focus the first of them on first load. Failures are logged, never fatal.
	if started, failed := mgr.StartAutoStartable(); started > 0 || len(failed) > 0 {
		log.Printf("auto-start: started %d agent(s), failed=%v", started, failed)
	}

	// Create an empty proxy and connect the registry's external agents (and any
	// managed agent that is already running). Managed agents are started on
	// demand through the Web UI.
	proxy := gateway.NewProxy(ctx, nil)
	defer proxy.Close()

	// Attach the agent message board (FEATURE-530): the hub arbitrates
	// agent-to-agent messaging, keeps the roster and persists board state.
	if cfg.BoardDBPath == "" {
		cfg.BoardDBPath = filepath.Join(filepath.Dir(cfg.RegistryPath), "hub-board.db")
	}
	boardStore, err := gateway.OpenBoardStore(cfg.BoardDBPath)
	if err != nil {
		log.Fatalf("board store: %v", err)
	}
	defer func() {
		if cerr := boardStore.Close(); cerr != nil {
			log.Printf("board store close: %v", cerr)
		}
	}()
	board, err := gateway.NewBoard(boardStore)
	if err != nil {
		log.Fatalf("board: %v", err)
	}
	// FEATURE-567: how often an expired reply obligation is re-sent to its
	// recipient before the message times out.
	if cfg.BoardRedeliverMax != nil {
		board.SetRedeliverMax(*cfg.BoardRedeliverMax)
	}
	// FEATURE-538: the roster "职责" is the agent's Agent描述, i.e. the
	// agent-description section of {workspace}/IDENTITY.md. Agents report it
	// themselves (board_register / board_identity); this resolver is only a
	// fallback for a managed agent that has not reported anything yet.
	board.SetDutyResolver(func(agentID string) (string, bool) {
		for _, spec := range mgr.Agents() {
			if spec.ID != agentID || spec.Type != gateway.AgentTypeManaged || spec.Workspace == "" {
				continue
			}
			if v := strings.TrimSpace(gateway.ReadIdentityDescription(spec.Workspace)); v != "" {
				return v, true
			}
		}
		return "", false
	})
	proxy.SetBoard(board)
	log.Printf("board: message board enabled (db=%s)", cfg.BoardDBPath)
	for _, spec := range mgr.Agents() {
		if spec.Type == gateway.AgentTypeExternal {
			if err := proxy.AddAgent(gateway.AgentConfig{ID: spec.ID, WSURL: spec.WSURL}); err != nil {
				log.Printf("gateway: connect external agent %s failed: %v", spec.ID, err)
			}
		} else if mgr.IsRunning(spec.ID) {
			if err := proxy.AddAgent(gateway.AgentConfig{ID: spec.ID, WSURL: gateway.WSURLForPort(spec.Port)}); err != nil {
				log.Printf("gateway: connect running agent %s failed: %v", spec.ID, err)
			}
		}
	}

	// Start the TCP gateway service.
	tcpCfg := gateway.DefaultConfig()
	tcpCfg.ListenAddr = cfg.TCPAddr
	tcpCfg.APIKey = cfg.APIKey
	tcpSrv := gateway.NewServer(tcpCfg, proxy)
	if err := tcpSrv.Listen(); err != nil {
		log.Fatalf("tcp listen: %v", err)
	}
	go func() {
		if err := tcpSrv.Serve(); err != nil {
			log.Printf("tcp serve error: %v", err)
		}
	}()
	log.Printf("hub-gateway: TCP service on %s (api-key=%v)", cfg.TCPAddr, cfg.APIKey != "")

	// Start the Web UI server (chat + agent management).
	webCfg := gateway.WebUIConfig{ListenAddr: cfg.WebAddr, Whitelist: cfg.Whitelist, Lang: cfg.Lang}
	// Load the remote-access settings (TLS/whitelist/access key). The settings
	// file follows the same search order as the config.
	if cfg.SettingsPath == "" {
		cfg.SettingsPath = firstExisting("./hub-settings.json", filepath.Join(homeDir(), ".co-shell", "hub-settings.json"))
	}
	settings := gateway.LoadSettings(cfg.SettingsPath)
	// A command-line --whitelist overrides the persisted settings whitelist so
	// the flag remains authoritative when supplied.
	if len(cfg.Whitelist) > 0 {
		settings.Whitelist = cfg.Whitelist
	}
	// FIX-606: --no-api-key also makes the Web UI ignore its persisted
	// "require key" switch for this process only. This is in-memory: the settings
	// file is never rewritten, and the whitelist / access key stay as configured
	// (an out-of-whitelist host still needs the access key).
	if authDisabled {
		settings.RequireKey = false
	}
	webUI := gateway.NewWebUI(webCfg, proxy, mgr, settings, hubVersion, hubBuild)
	// The Chat pane shows the board conversations (FEATURE-530).
	webUI.SetBoard(board)
	if err := webUI.Listen(); err != nil {
		log.Fatalf("web listen: %v", err)
	}
	// The Web UI scheme is https when TLS is enabled, else http.
	scheme := "http"
	if settings != nil && settings.TLSEnabled {
		scheme = "https"
	}
	go func() {
		if err := webUI.Serve(); err != nil {
			log.Printf("web serve error: %v", err)
		}
	}()
	log.Printf("hub-gateway: Web UI on %s://%s (whitelist=%v)", scheme, cfg.WebAddr, cfg.Whitelist)

	// Open the Web UI in the default browser (loopback URL so it is reachable
	// even when the listener is bound to 0.0.0.0). Skipped in --serve mode.
	if !cfg.Serve {
		if url := browserURL(cfg.WebAddr, scheme); url != "" {
			if err := web.OpenBrowser(url); err != nil {
				log.Printf("hub-gateway: open browser: %v", err)
			}
		}
	}

	// Wait for shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("hub-gateway: shutting down")
	// FIX-561: stop the managed agents first — the same path the Web UI takes
	// when an agent is closed from the list — so Ctrl+C never leaves co-shell
	// subprocesses running behind the hub.
	stopManagedAgents(mgr)
	tcpSrv.Close()
	webUI.Close()
}

// stopManagedAgents terminates every running managed agent the way the Web UI's
// close action does (Manager.Stop), logging each one. Registry entries are kept:
// shutting the hub down stops processes, it does not delete configured agents.
func stopManagedAgents(mgr *gateway.Manager) {
	for _, spec := range mgr.Agents() {
		if spec.Type != gateway.AgentTypeManaged || !mgr.IsRunning(spec.ID) {
			continue
		}
		if err := mgr.Stop(spec.ID); err != nil {
			log.Printf("hub-gateway: stop agent %s: %v", spec.ID, err)
			continue
		}
		log.Printf("hub-gateway: stopped managed agent %s", spec.ID)
	}
}

// defaultCoShellPath returns the co-shell executable next to this binary.
// browserURL converts a listen address (host:port) into a loopback URL the
// default browser can open. A wildcard/empty host (0.0.0.0, ::, "") maps to
// 127.0.0.1 so the page is reachable from the local machine. scheme is http or
// https (https when TLS is enabled).
func browserURL(addr, scheme string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		host = "127.0.0.1"
	}
	return scheme + "://" + net.JoinHostPort(host, port)
}

// homeDir returns the current user's home directory (empty on error).
func homeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}

// firstExisting returns the first candidate path that exists on disk; if none
// exist it returns the first candidate (the default write target).
func firstExisting(candidates ...string) string {
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return ""
}

// resolveListenAddr returns the address the hub should listen on. An explicit
// address (from the command line or the config file) is returned unchanged, so
// an occupied port makes the subsequent bind fail and startup abort. Otherwise
// the address is built from host and defaultPort, incrementing the port until a
// free one is found.
func resolveListenAddr(addr, host string, defaultPort int, explicit bool) string {
	if explicit {
		return addr
	}
	for port := defaultPort; port <= 65535; port++ {
		candidate := net.JoinHostPort(host, strconv.Itoa(port))
		if !addrInUse(candidate) {
			return candidate
		}
	}
	return net.JoinHostPort(host, strconv.Itoa(defaultPort))
}

// addrInUse reports whether a TCP address is already bound on the system.
func addrInUse(addr string) bool {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return true
	}
	if err := ln.Close(); err != nil {
		log.Printf("warning: close probe listener on %s: %v", addr, err)
	}
	return false
}

// agentBasePort returns the default first port for auto-allocating managed
// agents: one above the hub's Web UI port. An unparsable address falls back to
// the built-in default Web port.
func agentBasePort(webAddr string) int {
	return addrPort(webAddr, defaultWebPort) + 1
}

// addrPort returns the port part of a host:port address, or fallback when the
// address cannot be parsed.
func addrPort(addr string, fallback int) int {
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return fallback
	}
	p, err := strconv.Atoi(portStr)
	if err != nil || p <= 0 || p > 65535 {
		return fallback
	}
	return p
}

func defaultCoShellPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "co-shell"
	}
	name := "co-shell"
	if runtime.GOOS == "windows" {
		name = "co-shell.exe"
	}
	return filepath.Join(filepath.Dir(exe), name)
}

// multiFlag collects repeated string flags.
type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}

// loadConfig reads the JSON config file if present, else returns defaults.
// Config search priority: an explicit --config path > ./hub-gateway.json
// (process cwd) > ~/.co-shell/hub-gateway.json. The first existing file wins;
// if none exists a default config is returned.
func loadConfig(path string) *config {
	// No built-in defaults here: an empty address means "not specified" and is
	// resolved later (default port, auto-incrementing when occupied).
	cfg := &config{}
	var candidates []string
	if path != "" {
		// An explicit --config is authoritative: use it alone (no fallback).
		candidates = append(candidates, path)
	} else {
		candidates = append(candidates, "./hub-gateway.json")
		if home, err := os.UserHomeDir(); err == nil {
			candidates = append(candidates, filepath.Join(home, ".co-shell", "hub-gateway.json"))
		}
	}
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err != nil {
			continue // not found; try the next candidate
		}
		if err := json.Unmarshal(data, cfg); err != nil {
			log.Printf("warning: parse config %s: %v", p, err)
		}
		break
	}
	return cfg
}

// lastAuthFlag reports which auth flag appears last in args: "none" for
// --no-api-key, "key" for --api-key and "" when neither is present. Both flags
// may be supplied; the later one wins (FIX-606), so "--api-key K --no-api-key"
// disables auth while "--no-api-key --api-key K" keeps requiring key K.
// authDisabledByCLI reports whether TCP auth must be disabled for this run:
// --no-api-key turns it off, and when both auth flags are present the one that
// appears later in args wins (FIX-606).
func authDisabledByCLI(noAPIKey bool, args []string) bool {
	return noAPIKey && lastAuthFlag(args) != "key"
}

func lastAuthFlag(args []string) string {
	last := ""
	for _, arg := range args {
		switch {
		case arg == "--no-api-key" || arg == "-no-api-key":
			last = "none"
		case strings.HasPrefix(arg, "--api-key") || strings.HasPrefix(arg, "-api-key"):
			last = "key"
		}
	}
	return last
}

// splitList splits a comma-separated string into trimmed non-empty parts.
func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func printUsage() {
	fmt.Println(`co-shell-hub - WebSocket 聚合网关 (FEATURE-484)

Usage:
  co-shell-hub [flags]

Flags:
  --config PATH        Config file path (default: ./hub-gateway.json)
  --tcp-addr ADDR      TCP listen address (default 127.0.0.1:27256; the default
                       port auto-increments when occupied; an explicitly set
                       address aborts startup when its port is occupied)
  --api-key KEY        API key required by TCP clients
  --no-api-key         Disable TCP API-key auth for this run (highest priority; also
                       ignores the Web UI "require key" switch; config files unchanged)
  --web-addr ADDR      Web UI listen address (default 127.0.0.1:28256; the
                       default port auto-increments when occupied; an explicitly
                       set address aborts startup when its port is occupied)
  --whitelist IPS      Web UI access whitelist (comma-separated IPs/CIDR, empty=loopback only)
  --lang CODE          Web UI language: zh/en (default: auto-detect from LANG/LC_ALL, fallback zh)
  --registry PATH      Agent registry file (default: ./hub-agents.json)
  --co-shell-path PATH co-shell executable for managed agents (default: same dir as this binary)
  --base-port N        First port for auto-allocating managed agents (default:
                       the Web UI port + 1)
  --serve              Serve without auto-opening the browser (headless/server mode)
  --agent ID=WSURL     External agent endpoint (repeatable, added to registry)
  --help               Show help

Examples:
  # Serve Web UI on localhost; manage agents through the UI
  co-shell-hub

  # Connect one external agent and serve Web UI on localhost
  co-shell-hub --agent default=ws://127.0.0.1:8399/ws

  # Temporarily disable the TCP API key (config files are left untouched)
  co-shell-hub --no-api-key

  # TCP service with API key + remote Web UI whitelist
  co-shell-hub --api-key secret \
    --web-addr 0.0.0.0:28256 --whitelist 192.168.1.100,192.168.1.0/24`)
}
