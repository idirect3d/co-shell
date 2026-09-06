// co-shell-hub-gateway is the new WebSocket aggregation gateway (FEATURE-484).
// It connects to multiple co-shell agents over WebSocket, exposes a TCP service
// with API Key authentication for mobile clients, and serves a Web UI for
// browser access (default localhost only, whitelist configurable).
//
// Usage:
//
//	co-shell-hub-gateway [flags]
//
// Flags:
//
//	--config PATH        Config file path (default: ./hub-gateway.json)
//	--tcp-addr ADDR      TCP listen address (default 127.0.0.1:12801)
//	--api-key KEY        API key required by TCP clients
//	--web-addr ADDR      Web UI listen address (default 127.0.0.1:12802)
//	--whitelist IPS      Web UI access whitelist (comma-separated IPs/CIDR)
//	--agent ID=WSURL     co-shell agent endpoint (repeatable)
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
	"strings"
	"syscall"

	"github.com/idirect3d/co-shell/hub/gateway"
)

// config is the JSON config file shape.
type config struct {
	TCPAddr   string              `json:"tcp_addr"`
	APIKey    string              `json:"api_key"`
	WebAddr   string              `json:"web_addr"`
	Whitelist []string            `json:"whitelist,omitempty"`
	Agents    []gateway.AgentConfig `json:"agents"`
}

func main() {
	configPath := flag.String("config", "", "config file path (default: ./hub-gateway.json)")
	tcpAddr := flag.String("tcp-addr", "", "TCP listen address (default 127.0.0.1:12801)")
	apiKey := flag.String("api-key", "", "API key required by TCP clients")
	webAddr := flag.String("web-addr", "", "Web UI listen address (default 127.0.0.1:12802)")
	whitelist := flag.String("whitelist", "", "Web UI access whitelist (comma-separated IPs/CIDR, empty=loopback only)")
	var agents multiFlag
	flag.Var(&agents, "agent", "co-shell agent endpoint as ID=WSURL (repeatable)")
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
	for _, a := range agents {
		id, url, ok := strings.Cut(a, "=")
		if !ok || id == "" || url == "" {
			log.Fatalf("invalid --agent %q (want ID=WSURL)", a)
		}
		cfg.Agents = append(cfg.Agents, gateway.AgentConfig{ID: id, Name: id, WSURL: url})
	}

	if len(cfg.Agents) == 0 {
		log.Println("warning: no agents configured; use --agent ID=WSURL")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to agents via the proxy.
	proxy := gateway.NewProxy(ctx, cfg.Agents)
	defer proxy.Close()

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

	// Start the Web UI server.
	webCfg := gateway.WebUIConfig{ListenAddr: cfg.WebAddr, Whitelist: cfg.Whitelist}
	webUI := gateway.NewWebUI(webCfg, proxy)
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

// multiFlag collects repeated string flags.
type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}

// loadConfig reads the JSON config file if present, else returns defaults.
func loadConfig(path string) *config {
	cfg := &config{
		TCPAddr: "127.0.0.1:12801",
		WebAddr: "127.0.0.1:12802",
	}
	if path == "" {
		path = "./hub-gateway.json"
	}
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, cfg); err != nil {
			log.Printf("warning: parse config %s: %v", path, err)
		}
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
	fmt.Println(`co-shell-hub-gateway - WebSocket 聚合网关 (FEATURE-484)

Usage:
  co-shell-hub-gateway [flags]

Flags:
  --config PATH        Config file path (default: ./hub-gateway.json)
  --tcp-addr ADDR      TCP listen address (default 127.0.0.1:12801)
  --api-key KEY        API key required by TCP clients
  --web-addr ADDR      Web UI listen address (default 127.0.0.1:12802)
  --whitelist IPS      Web UI access whitelist (comma-separated IPs/CIDR, empty=loopback only)
  --agent ID=WSURL     co-shell agent endpoint (repeatable, e.g. --agent a=ws://127.0.0.1:8399/ws)
  --help               Show help

Examples:
  # Connect one agent and serve Web UI on localhost
  co-shell-hub-gateway --agent default=ws://127.0.0.1:8399/ws

  # Two agents + TCP service with API key + remote Web UI whitelist
  co-shell-hub-gateway --api-key secret \
    --agent a=ws://127.0.0.1:8399/ws --agent b=ws://127.0.0.1:8400/ws \
    --web-addr 0.0.0.0:12802 --whitelist 192.168.1.100,192.168.1.0/24`)
}
