// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-04-25
//
// # MIT License
//
// # Copyright (c) 2026 L.Shuang
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
package cmd

import (
	"fmt"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/mcp"
)

// MCPHandler handles the .mcp built-in command.
type MCPHandler struct {
	cfg    *config.Config
	mcpMgr *mcp.Manager
}

// NewMCPHandler creates a new MCPHandler.
func NewMCPHandler(cfg *config.Config, mcpMgr *mcp.Manager) *MCPHandler {
	return &MCPHandler{cfg: cfg, mcpMgr: mcpMgr}
}

// Handle processes .mcp commands.
func (h *MCPHandler) Handle(args []string) (string, error) {
	if len(args) == 0 {
		return h.listServers(), nil
	}

	subcommand := args[0]
	switch subcommand {
	case "add":
		return h.addServer(args[1:])
	case "remove":
		return h.removeServer(args[1:])
	case "list":
		return h.listServers(), nil
	case "enable":
		return h.enableServer(args[1:])
	case "disable":
		return h.disableServer(args[1:])
	default:
		return "", fmt.Errorf("unknown mcp subcommand: %s", subcommand)
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

	// Check for duplicates
	for _, s := range h.cfg.MCP.Servers {
		if s.Name == name {
			return "", fmt.Errorf("%s", i18n.TF(i18n.KeyMCPAlreadyExists, name))
		}
	}

	server := config.MCPServerConfig{
		Name:    name,
		Command: command,
		Args:    cmdArgs,
		Enabled: true,
	}
	h.cfg.MCP.Servers = append(h.cfg.MCP.Servers, server)

	if err := h.mcpMgr.AddServer(name, command, cmdArgs); err != nil {
		log.Warn("MCP server %s added to config but connection failed: %v", name, err)
	}

	if err := h.cfg.Save(); err != nil {
		return "", err
	}
	log.Info("MCP server added: %s", name)
	return i18n.TF(i18n.KeyMCPAdded, name), nil
}

func (h *MCPHandler) removeServer(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("usage: .mcp remove <name>")
	}
	name := args[0]

	index := -1
	for i, s := range h.cfg.MCP.Servers {
		if s.Name == name {
			index = i
			break
		}
	}
	if index == -1 {
		return "", fmt.Errorf("%s", i18n.TF(i18n.KeyMCPNotFound, name))
	}

	h.cfg.MCP.Servers = append(h.cfg.MCP.Servers[:index], h.cfg.MCP.Servers[index+1:]...)

	if err := h.mcpMgr.RemoveServer(name); err != nil {
		log.Warn("MCP server %s removed from config but disconnect error: %v", name, err)
	}

	if err := h.cfg.Save(); err != nil {
		return "", err
	}
	log.Info("MCP server removed: %s", name)
	return i18n.TF(i18n.KeyMCPRemoved, name), nil
}

func (h *MCPHandler) enableServer(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("usage: .mcp enable <name>")
	}
	name := args[0]

	for i, s := range h.cfg.MCP.Servers {
		if s.Name == name {
			h.cfg.MCP.Servers[i].Enabled = true
			if err := h.mcpMgr.AddServer(name, s.Command, s.Args); err != nil {
				log.Warn("MCP server %s enabled but connection failed: %v", name, err)
			}
			if err := h.cfg.Save(); err != nil {
				return "", err
			}
			log.Info("MCP server enabled: %s", name)
			return i18n.TF(i18n.KeyMCPEnabled, name), nil
		}
	}
	return "", fmt.Errorf("%s", i18n.TF(i18n.KeyMCPNotFound, name))
}

func (h *MCPHandler) disableServer(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("usage: .mcp disable <name>")
	}
	name := args[0]

	for i, s := range h.cfg.MCP.Servers {
		if s.Name == name {
			h.cfg.MCP.Servers[i].Enabled = false
			if err := h.mcpMgr.RemoveServer(name); err != nil {
				log.Warn("MCP server %s disabled but disconnect error: %v", name, err)
			}
			if err := h.cfg.Save(); err != nil {
				return "", err
			}
			log.Info("MCP server disabled: %s", name)
			return i18n.TF(i18n.KeyMCPDisabled, name), nil
		}
	}
	return "", fmt.Errorf("%s", i18n.TF(i18n.KeyMCPNotFound, name))
}

func (h *MCPHandler) listServers() string {
	if len(h.cfg.MCP.Servers) == 0 {
		return i18n.T(i18n.KeyMCPEmpty)
	}

	var result string
	result = i18n.T(i18n.KeyMCPListTitle) + "\n"
	for _, s := range h.cfg.MCP.Servers {
		status := i18n.T(i18n.KeyOff)
		if s.Enabled {
			status = i18n.T(i18n.KeyOn)
		}
		result += fmt.Sprintf("  [%s] %s: %s %v\n", status, s.Name, s.Command, s.Args)
	}
	return result
}
