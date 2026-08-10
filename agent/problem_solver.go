// Author: L.Shuang
// Created: 2026-08-10
// Last Modified: 2026-08-10
//
// MIT License
//
// Copyright (c) 2026 L.Shuang
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

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
)

// ProblemType classifies the kind of problem sent to the problem solver model.
type ProblemType string

const (
	// ProblemTypeNoAnomaly: the model examined the signal and found no real problem.
	ProblemTypeNoAnomaly ProblemType = "no_anomaly"
	// ProblemTypeLoop: suspected loop (content or tool-call repetition).
	ProblemTypeLoop ProblemType = "loop"
	// ProblemTypeToolFormatError: malformed tool call arguments / XML structure.
	ProblemTypeToolFormatError ProblemType = "tool_format_error"
	// ProblemTypeContextOverflow: context window is over / near the limit.
	ProblemTypeContextOverflow ProblemType = "context_overflow"
	// ProblemTypeLLMConnectionError: connection-level / credential / model-not-found errors.
	ProblemTypeLLMConnectionError ProblemType = "llm_connection_error"
	// ProblemTypeUnknown: the model could not determine the nature of the signal.
	ProblemTypeUnknown ProblemType = "unknown"
)

// SuggestedAction is the action the problem model recommends the code take.
// The code MAY ignore the suggestion and always falls back to a safe default.
type SuggestedAction string

const (
	// ActionContinue: no anomaly, keep the normal flow.
	ActionContinue SuggestedAction = "continue"
	// ActionPromptFeedback: append guidance as a user message to the main model.
	ActionPromptFeedback SuggestedAction = "prompt_feedback"
	// ActionDeleteLastMsg: remove the last assistant tool-call message and retry.
	ActionDeleteLastMsg SuggestedAction = "delete_last_msg"
	// ActionCompactContext: replace the overflowing message with a reorganize hint.
	ActionCompactContext SuggestedAction = "compact_context"
	// ActionNotifyUser: surface the problem to the user.
	ActionNotifyUser SuggestedAction = "notify_user"
	// ActionRetry: just resend the context without feedback.
	ActionRetry SuggestedAction = "retry"
)

// ProblemReport is the structured payload the problem model returns via the
// report_problem tool (FEATURE-342). It replaces the free-text JSON output
// of judgeLoop with a schema-forced tool call.
type ProblemReport struct {
	// Type is the classified problem type.
	Type ProblemType `json:"type"`
	// ErrorDetail carries the raw error text / suspicious content (truncated).
	ErrorDetail string `json:"error_detail"`
	// Reason is the model's analysis of why this is (or is not) a problem.
	Reason string `json:"reason"`
	// Guidance is the constructive, self-contained instruction for the main model.
	// It must NOT reference deleted messages and must re-state the goal.
	Guidance string `json:"guidance"`
	// SuggestedAction is the recommended handling action.
	SuggestedAction SuggestedAction `json:"suggested_action"`
}

// IsLoop returns true when the report classifies the signal as a loop.
func (p *ProblemReport) IsLoop() bool {
	return p != nil && p.Type == ProblemTypeLoop
}

// IsNoAnomaly returns true when the model confirmed there is no real problem.
func (p *ProblemReport) IsNoAnomaly() bool {
	return p != nil && p.Type == ProblemTypeNoAnomaly
}

// reportProblemTool builds the single tool definition sent to the problem
// solver model. Only this one tool is provided, together with
// tool_choice→report_problem, forcing a structured call.
func reportProblemTool() llm.Tool {
	return llm.Tool{
		Name:        "report_problem",
		Description: "Report the current problem and decide the next step. You MUST call this tool with a structured report.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"type": map[string]interface{}{
					"type": "string",
					"enum": []string{
						string(ProblemTypeNoAnomaly),
						string(ProblemTypeLoop),
						string(ProblemTypeToolFormatError),
						string(ProblemTypeContextOverflow),
						string(ProblemTypeLLMConnectionError),
						string(ProblemTypeUnknown),
					},
					"description": "The classified problem type.",
				},
				"error_detail": map[string]interface{}{
					"type":        "string",
					"description": "The raw error text / suspicious content (truncated).",
				},
				"reason": map[string]interface{}{
					"type":        "string",
					"description": "Why you think this is (or is not) a problem.",
				},
				"guidance": map[string]interface{}{
					"type":        "string",
					"description": "Constructive, self-contained instruction for the main model. Re-state the goal. Do NOT reference messages that may be deleted.",
				},
				"suggested_action": map[string]interface{}{
					"type": "string",
					"enum": []string{
						string(ActionContinue),
						string(ActionPromptFeedback),
						string(ActionDeleteLastMsg),
						string(ActionCompactContext),
						string(ActionNotifyUser),
						string(ActionRetry),
					},
					"description": "Recommended handling action (the code may override).",
				},
			},
			"required": []string{"type", "reason", "guidance", "suggested_action"},
		},
	}
}

// resolveProblemAction normalizes a suggested action to a known value.
// Unknown/empty values fall back to notify_user (safe conservative default).
func resolveProblemAction(s string) SuggestedAction {
	switch SuggestedAction(s) {
	case ActionContinue, ActionPromptFeedback, ActionDeleteLastMsg,
		ActionCompactContext, ActionNotifyUser, ActionRetry:
		return SuggestedAction(s)
	default:
		return ActionNotifyUser
	}
}

// classifyConnectionError maps an LLM API error to a hard-coded problem type
// when the status code is a sufficient condition (FEATURE-342). It returns
// (type, ok): ok=false means the error is ambiguous and the problem model
// should be consulted.
func classifyConnectionError(err error) (ProblemType, bool) {
	if err == nil {
		return "", false
	}
	// llm.OpenAIError carries the HTTP status code.
	var apiErr *llm.OpenAIError
	if errors.As(err, &apiErr) && apiErr != nil {
		switch apiErr.StatusCode {
		case 401, 403, 404, 429:
			return ProblemTypeLLMConnectionError, true
		default:
			if apiErr.StatusCode >= 500 {
				return ProblemTypeLLMConnectionError, true
			}
		}
		return "", false
	}
	// Text-based sufficient conditions (defensive).
	msg := err.Error()
	low := strings.ToLower(msg)
	for _, needle := range []string{
		"invalid api key", "unauthorized", "authentication", "api key",
		"model not found", "not found", "model does not exist",
	} {
		if strings.Contains(low, needle) {
			return ProblemTypeLLMConnectionError, true
		}
	}
	return "", false
}

// callProblemSolver invokes the problem model with only the report_problem
// tool and parses its tool-call arguments into a ProblemReport. The call
// follows the main model's tool-call mechanism (openai function calling or
// XML tags) as requested in FEATURE-342.
//
// Judge model resolution: getLoopJudgeModel() uses getProblemModelID() which
// resolves the full FEATURE-342 chain (mode ProblemModelID >
// default-problem-model > default-tool-model > mode ModelID > active model).
func (a *Agent) callProblemSolver(ctx context.Context, prompt string) (*ProblemReport, error) {
	modelCfg := a.getLoopJudgeModel()
	if modelCfg == nil {
		return nil, fmt.Errorf("no problem solver model available")
	}

	judgeTimeout := 60
	if a.cfg != nil && a.cfg.LLM.LoopJudgeTimeout > 0 {
		judgeTimeout = a.cfg.LLM.LoopJudgeTimeout
	} else if a.cfg != nil && a.cfg.LLM.LoopJudgeTimeout == 0 {
		judgeTimeout = 0
	}

	judgeClient := llm.NewClient(
		modelCfg.Endpoint, modelCfg.APIKey, modelCfg.Model,
		0.3, 8192, judgeTimeout,
	)
	if judgeClient != nil {
		defer judgeClient.Close()
	}
	judgeClient.SetThinkingEnabled(false)
	if modelCfg.Temperature != nil {
		judgeClient.SetTemperature(*modelCfg.Temperature)
	}

	// Force the model to call report_problem via body additions
	// (tool_choice={"type":"function","function":{"name":"report_problem"}}).
	judgeClient.SetBodyAdditions(map[string]string{
		"tool_choice": `{"type":"function","function":{"name":"report_problem"}}`,
	})

	messages := []llm.Message{
		{Role: "system", Content: i18n.T(i18n.KeyProblemSolverSystemPrompt)},
		{Role: "user", Content: prompt},
	}

	ctxTimeout := judgeTimeout + 5
	if judgeTimeout <= 0 {
		ctxTimeout = 0
	}
	var cctx context.Context
	var cancel context.CancelFunc
	if ctxTimeout > 0 {
		cctx, cancel = context.WithTimeout(ctx, time.Duration(ctxTimeout)*time.Second)
	} else {
		cctx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	resp, err := judgeClient.Chat(cctx, messages, []llm.Tool{reportProblemTool()})
	if err != nil {
		return nil, fmt.Errorf("problem solver call failed: %w", err)
	}

	// OpenAI mode: the call is returned in resp.ToolCalls.
	for _, tc := range resp.ToolCalls {
		if tc.Name == "report_problem" {
			return parseProblemReport(tc.Arguments)
		}
	}

	// XML mode fallback: the model emits XML tags in the content. Parse the
	// first report_problem call from the content.
	if resp.Content != "" {
		calls := ParseXMLToolCalls(resp.Content)
		for _, tc := range calls {
			if tc.Name == "report_problem" {
				return parseProblemReport(tc.Arguments)
			}
		}
		// Last resort: the model may have returned pure JSON (judgeLoop style).
		return parseProblemReport(resp.Content)
	}

	return nil, fmt.Errorf("problem solver returned no report_problem call")
}

// parseProblemReport unmarshals report_problem arguments (JSON) into a
// ProblemReport. It is lenient about extra fields and normalizes the action.
func parseProblemReport(args string) (*ProblemReport, error) {
	var report ProblemReport
	if err := json.Unmarshal([]byte(args), &report); err != nil {
		// Try to extract JSON object from a surrounding text response.
		s := strings.TrimSpace(args)
		if idx := strings.Index(s, "{"); idx >= 0 {
			s = s[idx:]
		}
		if idx := strings.LastIndex(s, "}"); idx >= 0 {
			s = s[:idx+1]
		}
		if err2 := json.Unmarshal([]byte(s), &report); err2 != nil {
			return nil, fmt.Errorf("cannot parse report_problem args: %v", err)
		}
	}
	// Normalize: default type to unknown when empty; default action to notify.
	if report.Type == "" {
		report.Type = ProblemTypeUnknown
	}
	report.SuggestedAction = resolveProblemAction(string(report.SuggestedAction))
	return &report, nil
}
