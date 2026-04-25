// Author: L.Shuang
package cmd

import (
	"fmt"
	"strings"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/mcp"
)

// MCPHandler handles the .mcp built-in command.
type MCPHandler struct {
	cfg     *config.Config
	manager *mcp.Manager
}

// NewMCPHandler creates a new MCPHandler.
func NewMCPHandler(cfg *config.Config, manager *mcp.Manager) *MCPHandler {
	return &MCPHandler{cfg: cfg, manager: manager}
}

// Handle processes .mcp commands.
// Syntax:
//
//	.mcp                          - list all MCP servers
//	.mcp add <name> <cmd> [args]  - add a new MCP server
//	.mcp remove <name>            - remove an MCP server
//	.mcp list                     - list all MCP servers and their tools
//	.mcp enable <name>            - enable an MCP server
//	.mcp disable <name>           - disable an MCP server
func (h *MCPHandler) Handle(args []string) (string, error) {
	if len(args) == 0 {
		return h.listServers(), nil
	}

	subcommand := args[0]
	switch subcommand {
	case "add":
		return h.addServer(args[1:])

	case "remove", "rm":
		return h.removeServer(args[1:])

	case "list", "ls":
		return h.listServers(), nil

	case "enable":
		return h.enableServer(args[1:])

	case "disable":
		return h.disableServer(args[1:])

	default:
		return "", fmt.Errorf("unknown subcommand: %s\n\nAvailable commands:\n  add <name> <cmd> [args]  - Add a new MCP server\n  remove <name>            - Remove an MCP server\n  list                     - List all MCP servers\n  enable <name>            - Enable an MCP server\n  disable <name>           - Disable an MCP server", subcommand)
	}
}

func (h *MCPHandler) addServer(args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("usage: .mcp add <name> <command> [args...]")
	}

	name := args[0]
	command := args[1]
	var cmdArgs []string
	if len(args) > 2 {
		cmdArgs = args[2:]
	}

	// Add to config
	serverCfg := config.MCPServerConfig{
		Name:    name,
		Command: command,
		Args:    cmdArgs,
		Enabled: true,
	}
	h.cfg.MCP.Servers = append(h.cfg.MCP.Servers, serverCfg)
	if err := h.cfg.Save(); err != nil {
		return "", err
	}

	// Connect to the server
	if err := h.manager.AddServer(name, command, cmdArgs); err != nil {
		return "", fmt.Errorf("added to config but failed to connect: %w", err)
	}

	return fmt.Sprintf("✅ MCP server %q added and connected", name), nil
}

func (h *MCPHandler) removeServer(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("usage: .mcp remove <name>")
	}

	name := args[0]

	// Remove from config
	var updatedServers []config.MCPServerConfig
	for _, s := range h.cfg.MCP.Servers {
		if s.Name != name {
			updatedServers = append(updatedServers, s)
		}
	}
	h.cfg.MCP.Servers = updatedServers
	if err := h.cfg.Save(); err != nil {
		return "", err
	}

	// Disconnect
	if err := h.manager.RemoveServer(name); err != nil {
		return fmt.Sprintf("⚠️  Removed from config but disconnect had error: %v", err), nil
	}

	return fmt.Sprintf("✅ MCP server %q removed", name), nil
}

func (h *MCPHandler) listServers() string {
	servers := h.manager.ListServers()
	if len(servers) == 0 {
		return "No MCP servers connected.\n\nAdd one with: .mcp add <name> <command> [args...]"
	}

	var sb strings.Builder
	sb.WriteString("MCP Servers:\n")
	for _, s := range servers {
		sb.WriteString(fmt.Sprintf("\n  📡 %s\n", s.Name))
		if len(s.Tools) == 0 {
			sb.WriteString("    No tools available\n")
		} else {
			sb.WriteString("    Tools:\n")
			for _, t := range s.Tools {
				sb.WriteString(fmt.Sprintf("      • %s", t.Name))
				if t.Description != "" {
					sb.WriteString(fmt.Sprintf(" - %s", t.Description))
				}
				sb.WriteString("\n")
			}
		}
	}
	return sb.String()
}

func (h *MCPHandler) enableServer(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("usage: .mcp enable <name>")
	}

	name := args[0]
	for i, s := range h.cfg.MCP.Servers {
		if s.Name == name {
			h.cfg.MCP.Servers[i].Enabled = true
			if err := h.cfg.Save(); err != nil {
				return "", err
			}
			// Try to connect if not already connected
			if err := h.manager.AddServer(name, s.Command, s.Args); err != nil {
				return fmt.Sprintf("⚠️  Enabled in config but failed to connect: %v", err), nil
			}
			return fmt.Sprintf("✅ MCP server %q enabled", name), nil
		}
	}
	return "", fmt.Errorf("server %q not found in config", name)
}

func (h *MCPHandler) disableServer(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("usage: .mcp disable <name>")
	}

	name := args[0]
	for i, s := range h.cfg.MCP.Servers {
		if s.Name == name {
			h.cfg.MCP.Servers[i].Enabled = false
			if err := h.cfg.Save(); err != nil {
				return "", err
			}
			h.manager.RemoveServer(name)
			return fmt.Sprintf("✅ MCP server %q disabled", name), nil
		}
	}
	return "", fmt.Errorf("server %q not found in config", name)
}

// Help returns the help text for the MCP command.
func (h *MCPHandler) Help() string {
	return `MCP Server Management (.mcp)

Usage:
  .mcp                              List all MCP servers
  .mcp add <name> <cmd> [args...]   Add a new MCP server
  .mcp remove <name>                Remove an MCP server
  .mcp list                         List all MCP servers and their tools
  .mcp enable <name>                Enable an MCP server
  .mcp disable <name>               Disable an MCP server

Examples:
  .mcp add filesystem npx @modelcontextprotocol/server-filesystem /tmp
  .mcp add github npx @modelcontextprotocol/server-github
  .mcp list`
}
