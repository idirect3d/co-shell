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
//	--tcp-addr ADDR      TCP listen address (default 127.0.0.1:12801)
//	--api-key KEY        API key required by TCP clients
//	--web-addr ADDR      Web UI listen address (default 127.0.0.1:12802)
//	--whitelist IPS      Web UI access whitelist (comma-separated IPs/CIDR)
//	--registry PATH      Agent registry file (default: ./hub-agents.json)
//	--co-shell-path PATH co-shell executable for managed agents (default: same dir as this binary)
//	--base-port N        First port for auto-allocating managed agents (default 28256)
//	--agent ID=WSURL     External agent endpoint (repeatable, added to registry)
//	--help               Show help
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/idirect3d/co-shell/hub/gateway"
)

// config is the JSON config file shape.
type config struct {
	TCPAddr      string `json:"tcp_addr"`
	APIKey       string `json:"api_key"`
	WebAddr      string `json:"web_addr"`
	Whitelist    []string `json:"whitelist,omitempty"`
	RegistryPath string `json:"registry_path,omitempty"`
	CoShellPath  string `json:"co_shell_path,omitempty"`
	BasePort     int    `json:"base_port,omitempty"`
}

func main() {
	configPath := flag.String("config", "", "config file path (default: ./hub-gateway.json)")
	tcpAddr := flag.String("tcp-addr", "", "TCP listen address (default 127.0.0.1:12801)")
	apiKey := flag.String("api-key", "", "API key required by TCP clients")
	webAddr := flag.String("web-addr", "", "Web UI listen address (default 127.0.0.1:12802)")
	whitelist := flag.String("whitelist", "", "Web UI access whitelist (comma-separated IPs/CIDR, empty=loopback only)")
	registryPath := flag.String("registry", "", "agent registry file (default: ./hub-agents.json)")
	coShellPath := flag.String("co-shell-path", "", "co-shell executable for managed agents (default: same dir as this binary)")
	basePort := flag.Int("base-port", 0, "first port for auto-allocating managed agents (default 28256)")
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
	if *webAddr != "" {
		cfg.WebAddr = *webAddr
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Resolve defaults for the agent manager.
	if cfg.RegistryPath == "" {
		cfg.RegistryPath = "./hub-agents.json"
	}
	if cfg.CoShellPath == "" {
		cfg.CoShellPath = defaultCoShellPath()
	}
	// Resolve to an absolute path: managed co-shell subprocesses run with their
	// workspace as the working directory, so a relative path would break.
	if abs, err := filepath.Abs(cfg.CoShellPath); err == nil {
		cfg.CoShellPath = abs
	}
	if cfg.BasePort == 0 {
		cfg.BasePort = 28256
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
		if _, err := mgr.AddExternal(id, id, url); err != nil {
			log.Printf("note: --agent %s not added: %v", id, err)
		}
	}

	// Create an empty proxy and connect the registry's external agents (and any
	// managed agent that is already running). Managed agents are started on
	// demand through the Web UI.
	proxy := gateway.NewProxy(ctx, nil)
	defer proxy.Close()
	for _, spec := range mgr.Agents() {
		if spec.Type == gateway.AgentTypeExternal {
			if err := proxy.AddAgent(gateway.AgentConfig{ID: spec.ID, Name: spec.Name, WSURL: spec.WSURL}); err != nil {
				log.Printf("gateway: connect external agent %s failed: %v", spec.ID, err)
			}
		} else if mgr.IsRunning(spec.ID) {
			if err := proxy.AddAgent(gateway.AgentConfig{ID: spec.ID, Name: spec.Name, WSURL: gateway.WSURLForPort(spec.Port)}); err != nil {
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
	webCfg := gateway.WebUIConfig{ListenAddr: cfg.WebAddr, Whitelist: cfg.Whitelist}
	webUI := gateway.NewWebUI(webCfg, proxy, mgr)
	if err := webUI.Listen(); err != nil {
		log.Fatalf("web listen: %v", err)
	}
	go func() {
		if err := webUI.Serve(); err != nil {
			log.Printf("web serve error: %v", err)
		}
	}()
	log.Printf("hub-gateway: Web UI on http://%s (whitelist=%v)", cfg.WebAddr, cfg.Whitelist)

	// Wait for shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("hub-gateway: shutting down")
	tcpSrv.Close()
	webUI.Close()
}

// defaultCoShellPath returns the co-shell executable next to this binary.
func defaultCoShellPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "co-shell"
	}
	return filepath.Join(filepath.Dir(exe), "co-shell")
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
	cfg := &config{
		TCPAddr: "127.0.0.1:12801",
		WebAddr: "127.0.0.1:12802",
	}
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
  --tcp-addr ADDR      TCP listen address (default 127.0.0.1:12801)
  --api-key KEY        API key required by TCP clients
  --web-addr ADDR      Web UI listen address (default 127.0.0.1:12802)
  --whitelist IPS      Web UI access whitelist (comma-separated IPs/CIDR, empty=loopback only)
  --registry PATH      Agent registry file (default: ./hub-agents.json)
  --co-shell-path PATH co-shell executable for managed agents (default: same dir as this binary)
  --base-port N        First port for auto-allocating managed agents (default 28256)
  --agent ID=WSURL     External agent endpoint (repeatable, added to registry)
  --help               Show help

Examples:
  # Serve Web UI on localhost; manage agents through the UI
  co-shell-hub

  # Connect one external agent and serve Web UI on localhost
  co-shell-hub --agent default=ws://127.0.0.1:8399/ws

  # TCP service with API key + remote Web UI whitelist
  co-shell-hub --api-key secret \
    --web-addr 0.0.0.0:12802 --whitelist 192.168.1.100,192.168.1.0/24`)
}
