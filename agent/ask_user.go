// Package agent - the ask_user tool (FEATURE-512).
//
// ask_user collects the answers to one or more questions in a single
// interaction round. It replaces the former single-choice ask_user
// tool: every question may be single-choice or multi-choice, the user may attach
// a supplementary note to the options they pick, and a question without options
// expects free-form text.
//
// Author: L.Shuang
// Created: 2026-09-13
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
)

// askUserToolName is the registered name of the multi-question tool.
const askUserToolName = "ask_user"

// buildAskUserTool declares the ask_user tool (always available).
func (a *Agent) buildAskUserTool() llm.Tool {
	return llm.Tool{
		Name: askUserToolName,
		Description: "Ask the user one or more questions to gather information needed to complete the task. Use this when requirements are ambiguous, a decision is needed, or more details are required. " +
			"Each question may offer options (single-choice by default; set multi=true to let the user pick several) and the user can attach a supplementary note to the option(s) they pick. A question without options expects free-form text. " +
			"Prefer collecting all pending questions in a single call instead of asking them one by one.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"meta": map[string]interface{}{
					"type":        "object",
					"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
				},
				"questions": map[string]interface{}{
					"type": "array",
					"description": "The questions to ask, in order. Each item is an object: title (required, the question text), " +
						"options (optional array of 2-5 strings the user can choose from), multi (optional bool, default false — set true for multi-choice), " +
						"allow_note (optional bool, default true — whether the user may add a supplementary note to a chosen option), " +
						"required (optional bool, default false — whether the user must answer this question before submitting). " +
						"Omit options to ask for free-form text.",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"title": map[string]interface{}{
								"type":        "string",
								"description": "The question text shown to the user.",
							},
							"options": map[string]interface{}{
								"type":        "array",
								"items":       map[string]interface{}{"type": "string"},
								"description": "The 2-5 options for this question. Omit to ask for free-form text.",
							},
							"multi": map[string]interface{}{
								"type":        "boolean",
								"description": "Whether the user may select several options. Default false (single choice).",
							},
							"allow_note": map[string]interface{}{
								"type":        "boolean",
								"description": "Whether the user may attach a supplementary note to a chosen option. Default true.",
							},
							"required": map[string]interface{}{
								"type":        "boolean",
								"description": "Whether the user must answer this question before the form can be submitted. Default false.",
							},
						},
						"required": []string{"title"},
					},
				},
			},
			"required": []string{"meta"},
		},
		Callback: a.askUserTool,
	}
}

// askUserTool collects the user's answers to one or more questions. The
// questions argument is normalised from the accepted shapes (see
// normalizeQuestions); the legacy question/options pair is accepted as a
// single-question form. The formatted answers are stored as the user reply,
// which is what the LLM sees on the next iteration.
func (a *Agent) askUserTool(ctx context.Context, args map[string]interface{}) (string, error) {
	questions := normalizeQuestions(args)
	if len(questions) == 0 {
		return "", fmt.Errorf("questions is required — provide at least one question with a title")
	}

	in := Interaction{
		Kind:      InteractionQuestions,
		Questions: questions,
		// Fixed key shared by every question: abort the whole form. The former
		// "+" (more options) key is covered by the free-form note input.
		Keys: []KeyOption{
			{Label: i18n.T(i18n.KeyAskFollowupThinkExit), Key: "-", Value: "think_exit"},
		},
	}

	res, err := a.interactionManager().Ask(ctx, in)
	if err != nil {
		return "", fmt.Errorf("failed to read user input: %w", err)
	}

	switch res.Action {
	case ActionCancel:
		return "", fmt.Errorf("CANCEL_AGENT")
	case ActionSubmit:
		// FIX-513: a channel may deliver only a plain value (older browser
		// client, free-form submit) instead of structured answers. Fall back to
		// Value so the LLM still receives the reply, and never return an empty
		// tool result — an empty answer means the LLM would see nothing at all.
		if len(res.Answers) > 0 {
			a.storeUserReply(formatQuestionAnswers(res.Answers))
			return i18n.T(i18n.KeySettingCmd_609), nil
		}
		if v := strings.TrimSpace(res.Value); v != "" {
			a.storeUserReply(v)
			return i18n.T(i18n.KeySettingCmd_609), nil
		}
		return i18n.T(i18n.KeyToolAskUserNoAnswer), nil
	case ActionInput:
		// Free-form fallback (e.g. a single question without options answered
		// through a channel that only supports text input).
		a.storeUserReply(res.Value)
		return i18n.T(i18n.KeySettingCmd_609), nil
	case ActionSelect:
		// Fixed-key answer (e.g. "-" = think it over, exit for now): pass the
		// user-readable label to the LLM so it can decide how to proceed.
		label := res.Value
		if label == "think_exit" {
			label = i18n.T(i18n.KeyAskFollowupThinkExit)
		}
		a.storeUserReply(label)
		return i18n.T(i18n.KeySettingCmd_609), nil
	default:
		// FIX-513: an unrecognised action must not look like a silent success.
		return i18n.T(i18n.KeyToolAskUserNoAnswer), nil
	}
}

// normalizeQuestions extracts the questions of an ask_user call from the
// accepted argument shapes:
//
//   - JSON / OpenAI mode: "questions" is an array of objects.
//   - XML mode: parseXMLChildrenToJSON wraps repeated <item> tags, so the array
//     arrives as {"item": [...]} (or as a bare string list for titles only).
//   - Legacy single-question mode: "question" (+ "options"), where the options
//     may also arrive under the XML "item" key.
//
// Questions without a usable title are dropped. Unset multi defaults to false
// (single choice) and unset allow_note defaults to true, so notes stay
// available unless the caller opts out explicitly. Unset required defaults to
// false, so only the questions the caller marks must be answered.
func normalizeQuestions(args map[string]interface{}) []Question {
	// Legacy single-question form: question + options.
	if q, ok := args["question"].(string); ok && strings.TrimSpace(q) != "" {
		return []Question{{
			Title:     strings.TrimSpace(q),
			Options:   optionList(args["options"], args["item"]),
			AllowNote: true,
		}}
	}

	items := itemList(args["questions"])
	questions := make([]Question, 0, len(items))
	for _, item := range items {
		switch v := item.(type) {
		case string:
			title := strings.TrimSpace(v)
			if title == "" {
				continue
			}
			questions = append(questions, Question{Title: title, AllowNote: true})
		case map[string]interface{}:
			title := argString(v, "title")
			if title == "" {
				title = argString(v, "question")
			}
			title = strings.TrimSpace(title)
			if title == "" {
				continue
			}
			multi, _ := boolValue(v["multi"])
			allowNote := true
			if b, ok := boolValue(v["allow_note"]); ok {
				allowNote = b
			}
			required, _ := boolValue(v["required"])
			questions = append(questions, Question{
				Title:     title,
				Options:   optionList(v["options"], v["item"]),
				Multi:     multi,
				AllowNote: allowNote,
				Required:  required,
			})
		}
	}
	return questions
}

// itemList normalises an array-typed argument into a slice. XML parsing wraps
// repeated <item> tags under an "item" key, so a map with only that key is
// unwrapped as well.
func itemList(v interface{}) []interface{} {
	switch t := v.(type) {
	case []interface{}:
		return t
	case map[string]interface{}:
		if inner, ok := t["item"].([]interface{}); ok {
			return inner
		}
	}
	return nil
}

// optionList builds the option list of one question from the "options"
// argument, falling back to the XML "item" key (single-question legacy shape).
func optionList(values ...interface{}) []string {
	for _, v := range values {
		items := itemList(v)
		if len(items) == 0 {
			continue
		}
		options := make([]string, 0, len(items))
		for _, item := range items {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				options = append(options, strings.TrimSpace(s))
			}
		}
		if len(options) > 0 {
			return options
		}
	}
	return nil
}

// boolValue converts a JSON/XML argument value into a bool. It reports false
// when the value is absent or not convertible, letting callers apply defaults.
func boolValue(v interface{}) (bool, bool) {
	switch t := v.(type) {
	case bool:
		return t, true
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "true", "1", "yes":
			return true, true
		case "false", "0", "no":
			return false, true
		}
	}
	return false, false
}

// askUserSummaryText builds the summary text shown for an ask_user call: the
// question titles one per line, so a multi-question call stays readable. The
// legacy single-question argument shape is supported as well.
func askUserSummaryText(args map[string]interface{}) string {
	questions := normalizeQuestions(args)
	if len(questions) == 0 {
		return argString(args, "question")
	}
	lines := make([]string, 0, len(questions))
	for _, q := range questions {
		if strings.TrimSpace(q.Title) == "" {
			continue
		}
		lines = append(lines, q.Title)
	}
	return strings.Join(lines, "\n")
}
