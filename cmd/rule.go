package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/idirect3d/co-shell/config"
)

// RuleHandler handles the .rule built-in command.
type RuleHandler struct {
	cfg *config.Config
}

// NewRuleHandler creates a new RuleHandler.
func NewRuleHandler(cfg *config.Config) *RuleHandler {
	return &RuleHandler{cfg: cfg}
}

// Handle processes .rule commands.
// Syntax:
//
//	.rule                    - list all rules
//	.rule add <text>         - add a new rule
//	.rule remove <index>     - remove a rule by index
//	.rule clear              - clear all rules
func (h *RuleHandler) Handle(args []string) (string, error) {
	if len(args) == 0 {
		return h.listRules(), nil
	}

	subcommand := args[0]
	switch subcommand {
	case "add":
		return h.addRule(args[1:])

	case "remove", "rm":
		return h.removeRule(args[1:])

	case "clear":
		return h.clearRules()

	default:
		return "", fmt.Errorf("unknown subcommand: %s\n\nAvailable commands:\n  add <text>      - Add a new rule\n  remove <index>  - Remove a rule by index\n  clear           - Clear all rules", subcommand)
	}
}

func (h *RuleHandler) addRule(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("usage: .rule add <rule text>")
	}

	rule := strings.Join(args, " ")
	h.cfg.Rules = append(h.cfg.Rules, rule)
	if err := h.cfg.Save(); err != nil {
		return "", err
	}

	return fmt.Sprintf("✅ Rule added: %s", rule), nil
}

func (h *RuleHandler) removeRule(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("usage: .rule remove <index>")
	}

	index, err := strconv.Atoi(args[0])
	if err != nil {
		return "", fmt.Errorf("invalid index: %s", args[0])
	}

	if index < 0 || index >= len(h.cfg.Rules) {
		return "", fmt.Errorf("index out of range: %d (0-%d)", index, len(h.cfg.Rules)-1)
	}

	removed := h.cfg.Rules[index]
	h.cfg.Rules = append(h.cfg.Rules[:index], h.cfg.Rules[index+1:]...)
	if err := h.cfg.Save(); err != nil {
		return "", err
	}

	return fmt.Sprintf("✅ Rule removed: %s", removed), nil
}

func (h *RuleHandler) clearRules() (string, error) {
	count := len(h.cfg.Rules)
	h.cfg.Rules = []string{}
	if err := h.cfg.Save(); err != nil {
		return "", err
	}

	return fmt.Sprintf("✅ Cleared %d rules", count), nil
}

func (h *RuleHandler) listRules() string {
	if len(h.cfg.Rules) == 0 {
		return "No rules defined.\n\nAdd one with: .rule add <rule text>"
	}

	var sb strings.Builder
	sb.WriteString("Global Rules:\n")
	for i, rule := range h.cfg.Rules {
		sb.WriteString(fmt.Sprintf("  [%d] %s\n", i, rule))
	}
	return sb.String()
}

// Help returns the help text for the rule command.
func (h *RuleHandler) Help() string {
	return `Rule Management (.rule)

Usage:
  .rule                    List all rules
  .rule add <text>         Add a new rule
  .rule remove <index>     Remove a rule by index
  .rule clear              Clear all rules

Examples:
  .rule add "Always confirm before deleting files"
  .rule add "Use English for all responses"
  .rule remove 0`
}

// GetRules returns the current rules as a formatted string for the system prompt.
func (h *RuleHandler) GetRules() string {
	if len(h.cfg.Rules) == 0 {
		return ""
	}
	return strings.Join(h.cfg.Rules, "\n")
}
