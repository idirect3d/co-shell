// Package agent - unified user interaction model (FEATURE-388).
//
// This file defines the structured Interaction model that standardizes all
// user-facing interactions (tool confirmation, question/selection, free
// input) so that TUI and Web UI can render them uniformly from the same
// structured data, instead of each tool callback hand-assembling raw
// UserIO primitives (Print*/ReadLine/ReadKey).
//
// Author: L.Shuang
// Created: 2026-08-21
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"context"
	"strconv"
	"strings"

	"github.com/idirect3d/co-shell/i18n"
)

// InteractionKind identifies the type of a user interaction.
type InteractionKind string

const (
	// InteractionConfirm is a tool-confirmation prompt (approve/cancel/
	// approve-all/disable/approve-N-times/free input).
	InteractionConfirm InteractionKind = "confirm"
	// InteractionSelect is a question with a fixed set of options.
	InteractionSelect InteractionKind = "select"
	// InteractionInput is a free-form text input (no options).
	InteractionInput InteractionKind = "input"
	// InteractionKey is a single-key confirmation.
	InteractionKey InteractionKind = "key"
)

// InteractionResultAction is the structured action a user chose.
type InteractionResultAction string

const (
	// ActionApprove approves and executes.
	ActionApprove InteractionResultAction = "approve"
	// ActionApproveAll approves all tools for this request.
	ActionApproveAll InteractionResultAction = "approve_all"
	// ActionApproveCount approves the next N calls of this tool.
	ActionApproveCount InteractionResultAction = "approve_count"
	// ActionApproveG approves and disables confirmation for this tool.
	ActionApproveG InteractionResultAction = "approve_g"
	// ActionApproveD permanently disables this tool.
	ActionApproveD InteractionResultAction = "approve_d"
	// ActionCancel cancels the current operation.
	ActionCancel InteractionResultAction = "cancel"
	// ActionModify holds execution and sends supplementary input to the LLM.
	ActionModify InteractionResultAction = "modify"
	// ActionSelect selects one of the offered options.
	ActionSelect InteractionResultAction = "select"
	// ActionInput returns free-form text input.
	ActionInput InteractionResultAction = "input"
)

// KeyOption is a single key/button option offered to the user.
type KeyOption struct {
	Label string `json:"label"` // display label (e.g. "Approve", "Cancel")
	Key   string `json:"key"`   // trigger key ("" = Enter)
	Value string `json:"value"` // value returned to the backend
	Hint  string `json:"hint,omitempty"` // hover hint (Web)
}

// Interaction is a complete declaration of one user interaction.
type Interaction struct {
	Kind      InteractionKind `json:"kind"`                // interaction type
	Title     string          `json:"title,omitempty"`     // title (tool summary, question)
	Body      string          `json:"body,omitempty"`      // body (risk warning, extra note)
	Options   []string        `json:"options,omitempty"`   // select options
	Keys      []KeyOption     `json:"keys,omitempty"`      // key/button options (confirm/key)
	Default   string          `json:"default,omitempty"`   // default value (Enter)
	AllowFree bool            `json:"allow_free,omitempty"` // allow free-form input
	// Presets carries preset values for an action (e.g. approve_count [3,10,50]).
	Presets []string `json:"presets,omitempty"`
}

// InteractionResult is the structured outcome of a user interaction.
type InteractionResult struct {
	Action InteractionResultAction `json:"action"` // action chosen
	Value  string                  `json:"value"` // value (approve count, selected option, free text)
	Raw    string                  `json:"raw"`   // raw input (preserved)
}

// InteractionManager handles user interactions uniformly. Tool callbacks
// construct an Interaction and hand it to the manager instead of calling
// UserIO primitives directly. Implementations render the request and parse
// the response per channel (TUI via UserIO, Web via WebSocket).
type InteractionManager interface {
	// Ask presents an interaction and returns the structured result.
	Ask(ctx context.Context, in Interaction) (InteractionResult, error)
}

// TerminalInteractionManager renders interactions through a UserIO and
// parses the user's typed response. It composes (rather than extends) UserIO,
// so StdioIO/EnhancedIO/WebIO remain unchanged.
type TerminalInteractionManager struct {
	io UserIO
}

// NewTerminalInteractionManager creates a TUI interaction manager over the
// given UserIO.
func NewTerminalInteractionManager(io UserIO) *TerminalInteractionManager {
	return &TerminalInteractionManager{io: io}
}

// Ask renders the interaction and parses the user's response.
func (m *TerminalInteractionManager) Ask(ctx context.Context, in Interaction) (InteractionResult, error) {
	switch in.Kind {
	case InteractionConfirm:
		return m.askConfirm(in)
	case InteractionSelect:
		return m.askSelect(in)
	case InteractionInput:
		return m.askInput(in)
	case InteractionKey:
		return m.askKey(in)
	default:
		return InteractionResult{}, nil
	}
}

// askConfirm renders a tool-confirmation prompt and parses the response.
// Key mapping (fixed, not user-configurable):
//   - Enter: approve
//   - c: cancel
//   - a: approve all
//   - g: approve and disable confirmation for this tool
//   - d: permanently disable this tool
//   - N (positive integer): approve the next N calls of this tool
//   - other: supplementary instructions for the LLM to re-evaluate
func (m *TerminalInteractionManager) askConfirm(in Interaction) (InteractionResult, error) {
	m.io.Println()
	if in.Title != "" {
		m.io.Println(in.Title)
	}
	if in.Body != "" {
		m.io.Println(in.Body)
	}
	m.io.Println()

	for {
		m.io.Printf("%s", i18n.T(i18n.KeyCmdConfirmPrompt))

		response, err := m.io.ReadLine()
		if err != nil {
			return InteractionResult{Action: ActionCancel}, err
		}
		response = strings.TrimSpace(response)

		if response == "" {
			return InteractionResult{Action: ActionApprove}, nil
		}

		lower := strings.ToLower(response)
		switch lower {
		case "c":
			return InteractionResult{Action: ActionCancel}, nil
		case "a":
			return InteractionResult{Action: ActionApproveAll}, nil
		case "g":
			return InteractionResult{Action: ActionApproveG}, nil
		case "d":
			return InteractionResult{Action: ActionApproveD}, nil
		}

		// Positive integer: approve the next N calls of this tool.
		if n, err := strconv.Atoi(response); err == nil && n > 0 {
			return InteractionResult{Action: ActionApproveCount, Value: strconv.Itoa(n), Raw: response}, nil
		}

		// Any other input is treated as supplementary instructions.
		return InteractionResult{Action: ActionModify, Value: response, Raw: response}, nil
	}
}

// askSelect renders a question with options and parses the user's choice.
// The option list always ends with a fixed "supplementary info" option
// (len(options)+1) and a cancel option (len(options)+2). Selecting the
// supplementary option, typing a leading space, or typing any non-option text
// enters free-form input that is sent directly to the LLM.
func (m *TerminalInteractionManager) askSelect(in Interaction) (InteractionResult, error) {
	suppIdx := len(in.Options) + 1
	cancelIdx := len(in.Options) + 2

	for {
		m.io.Println()
		if in.Title != "" {
			m.io.Printf("❓ %s\n", in.Title)
		}

		if len(in.Options) > 0 {
			m.io.Println()
			m.io.Println(i18n.T(i18n.KeySettingCmd_601))
			for i, opt := range in.Options {
				m.io.Printf("    [%d] %s\n", i+1, opt)
			}
			// Fixed supplementary-info option (FEATURE-438).
			m.io.Printf(i18n.T(i18n.KeySettingCmd_776), suppIdx)
			m.io.Printf(i18n.T(i18n.KeySettingCmd_602), cancelIdx)
		}
		// Fixed key options (FEATURE-452): rendered as [Key] Label, e.g. [-]
		// and [+]. Parsed by matching the typed key.
		for _, k := range in.Keys {
			m.io.Printf("    [%s] %s\n", k.Key, k.Label)
		}
		if len(in.Options) > 0 || len(in.Keys) > 0 {
			m.io.Println()
		}

		m.io.Printf(i18n.T(i18n.KeySettingCmd_603))

		input, err := m.io.ReadLine()
		if err != nil {
			return InteractionResult{}, err
		}

		// A leading space means the user pressed space to enter supplementary
		// info directly (FEATURE-438).
		if strings.HasPrefix(input, " ") {
			note := strings.TrimSpace(input)
			if note == "" {
				m.io.Println(i18n.T(i18n.KeySettingCmd_604))
				continue
			}
			return InteractionResult{Action: ActionInput, Value: note, Raw: note}, nil
		}

		input = strings.TrimSpace(input)

		// Empty input: if there are options, prompt to re-choose; otherwise
		// accept empty input.
		if input == "" {
			if len(in.Options) > 0 {
				m.io.Println(i18n.T(i18n.KeySettingCmd_604))
				continue
			}
			return InteractionResult{Action: ActionInput}, nil
		}

		// Match a fixed key option (e.g. "-" or "+") before parsing numbers.
		if len(in.Keys) > 0 {
			for _, k := range in.Keys {
				if input == k.Key {
					return InteractionResult{Action: ActionSelect, Value: k.Value, Raw: input}, nil
				}
			}
		}

		// Parse the first token as a potential option number.
		fields := strings.Fields(input)
		firstToken := fields[0]

		if idx, err := strconv.Atoi(firstToken); err == nil {
			if len(in.Options) > 0 {
				if idx == cancelIdx {
					m.io.Println(i18n.T(i18n.KeySettingCmd_605))
					return InteractionResult{Action: ActionCancel}, nil
				}
				if idx == suppIdx {
					// Enter supplementary-info input mode.
					m.io.Printf(i18n.T(i18n.KeySettingCmd_777))
					supp, err := m.io.ReadLine()
					if err != nil {
						return InteractionResult{}, err
					}
					supp = strings.TrimSpace(supp)
					if supp == "" {
						m.io.Println(i18n.T(i18n.KeySettingCmd_604))
						continue
					}
					return InteractionResult{Action: ActionInput, Value: supp, Raw: supp}, nil
				}
				if idx >= 1 && idx <= len(in.Options) {
					selected := in.Options[idx-1]
					remaining := strings.TrimSpace(input[len(firstToken):])
					if remaining != "" {
						m.io.Printf(i18n.T(i18n.KeySettingCmd_606), selected)
						m.io.Printf(i18n.T(i18n.KeySettingCmd_607), remaining)
						return InteractionResult{Action: ActionSelect, Value: selected, Raw: input}, nil
					}
					m.io.Printf(i18n.T(i18n.KeySettingCmd_606), selected)
					return InteractionResult{Action: ActionSelect, Value: selected, Raw: input}, nil
				}
				m.io.Printf(i18n.T(i18n.KeySettingCmd_608), idx)
				continue
			}
			// No options: return the number input as free text.
			return InteractionResult{Action: ActionInput, Value: input, Raw: input}, nil
		}

		// Input doesn't start with a valid number: free text.
		return InteractionResult{Action: ActionInput, Value: input, Raw: input}, nil
	}
}

// askInput renders a free-form input prompt and returns the typed text.
func (m *TerminalInteractionManager) askInput(in Interaction) (InteractionResult, error) {
	m.io.Println()
	if in.Title != "" {
		m.io.Printf("❓ %s\n", in.Title)
	}
	m.io.Printf(i18n.T(i18n.KeySettingCmd_603))

	input, err := m.io.ReadLine()
	if err != nil {
		return InteractionResult{}, err
	}
	input = strings.TrimSpace(input)
	return InteractionResult{Action: ActionInput, Value: input, Raw: input}, nil
}

// promptErrorConfirmation asks the user how to handle repeated errors via the
// unified Interaction model (FEATURE-388). Options: Enter=continue, C=cancel,
// A=ignore all. Returns the structured result so callers can branch.
func promptErrorConfirmation(mgr InteractionManager, title, body string) (InteractionResult, error) {
	return mgr.Ask(context.Background(), Interaction{
		Kind:  InteractionConfirm,
		Title: title,
		Body:  body,
		Keys: []KeyOption{
			{Label: i18n.T(i18n.KeyErrActionEnter), Key: "", Value: string(ActionApprove)},
			{Label: i18n.T(i18n.KeyErrActionCancel), Key: "c", Value: string(ActionCancel)},
			{Label: i18n.T(i18n.KeyErrActionIgnore), Key: "a", Value: string(ActionApproveAll)},
		},
	})
}

// askKey renders a single-key confirmation and returns the pressed key.
func (m *TerminalInteractionManager) askKey(in Interaction) (InteractionResult, error) {
	m.io.Println()
	if in.Title != "" {
		m.io.Println(in.Title)
	}
	if in.Body != "" {
		m.io.Println(in.Body)
	}
	m.io.Println()

	for {
		m.io.Printf("%s", i18n.T(i18n.KeyCmdConfirmPrompt))

		response, err := m.io.ReadLine()
		if err != nil {
			return InteractionResult{Action: ActionCancel}, err
		}
		response = strings.TrimSpace(response)

		if response == "" {
			return InteractionResult{Action: ActionApprove}, nil
		}
		lower := strings.ToLower(response)
		switch lower {
		case "c":
			return InteractionResult{Action: ActionCancel}, nil
		case "a":
			return InteractionResult{Action: ActionApproveAll}, nil
		case "g":
			return InteractionResult{Action: ActionApproveG}, nil
		case "d":
			return InteractionResult{Action: ActionApproveD}, nil
		}
		if n, err := strconv.Atoi(response); err == nil && n > 0 {
			return InteractionResult{Action: ActionApproveCount, Value: strconv.Itoa(n), Raw: response}, nil
		}
		return InteractionResult{Action: ActionModify, Value: response, Raw: response}, nil
	}
}
