// Author: L.Shuang
// Created: 2026-08-31
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
	"strings"

	"github.com/idirect3d/co-shell/llm"
)

// SupScenario identifies one of the three SUP LLM-interaction scenarios whose
// prompt and streaming reply can be exposed to the frontend (FEATURE-460).
type SupScenario string

const (
	// SupScenarioProblemSolver: the problem solver model (report_problem).
	SupScenarioProblemSolver SupScenario = "problem_solver"
	// SupScenarioLoopJudge: the loop judge model (report_problem, loop path).
	SupScenarioLoopJudge SupScenario = "loop_judge"
	// SupScenarioSupervisor: the supervisor model (submit_review).
	SupScenarioSupervisor SupScenario = "supervisor"
)

// supTitle returns the SUP block title for a scenario (e.g. "SUP·问题解决").
func (s SupScenario) supTitle() string {
	switch s {
	case SupScenarioProblemSolver:
		return "SUP·问题解决"
	case SupScenarioLoopJudge:
		return "SUP·循环判定"
	case SupScenarioSupervisor:
		return "SUP·监督"
	default:
		return "SUP"
	}
}

// supPromptEnabled reports whether the prompt should be exposed for the
// scenario (FEATURE-460). The loop judge reuses the problem solver model, so
// it shares the same switch.
func (a *Agent) supPromptEnabled(s SupScenario) bool {
	if a.cfg == nil {
		return false
	}
	return a.cfg.LLM.Supervisor.ShowSupPrompt
}

// supStreamEnabled reports whether the streaming reply should be exposed for
// the scenario (FEATURE-460).
func (a *Agent) supStreamEnabled(s SupScenario) bool {
	if a.cfg == nil {
		return false
	}
	return a.cfg.LLM.Supervisor.ShowSupStream
}

// emitSupPrompt exposes the prompt sent to the LLM as a SUP block when the
// show-sup-prompt switch is on (FEATURE-460). It emits a content_chunk event
// on the supervisor channel carrying the scenario title in Meta, so the
// frontend renders it as a SUP block.
func (a *Agent) emitSupPrompt(s SupScenario, prompt string) {
	if !a.supPromptEnabled(s) || prompt == "" {
		return
	}
	a.mu.Lock()
	cb := a.streamCb
	a.mu.Unlock()
	if cb == nil {
		return
	}
	ev := NewStreamEvent(EventContentChunk, ChannelSupervisor, LevelInfo, prompt)
	ev.Meta = map[string]string{MetaKeySupScenario: s.supTitle()}
	cb(ev)
}

// streamSupReply streams the LLM reply for a scenario to the frontend as a
// SUP block when the show-sup-stream switch is on (FEATURE-460). It consumes
// the ChatStream channel, forwarding content chunks to the frontend and
// accumulating the full content plus any tool calls. It returns the
// accumulated content and tool calls so the caller can parse the structured
// result (report_problem / submit_review) from the streamed response.
func (a *Agent) streamSupReply(ctx context.Context, s SupScenario, eventCh <-chan llm.StreamEvent) (string, []llm.ToolCall, error) {
	stream := a.supStreamEnabled(s)
	a.mu.Lock()
	cb := a.streamCb
	a.mu.Unlock()

	var sb strings.Builder
	var toolCalls []llm.ToolCall
	var streamErr error

	for {
		select {
		case <-ctx.Done():
			return sb.String(), toolCalls, ctx.Err()
		case ev, ok := <-eventCh:
			if !ok {
				return sb.String(), toolCalls, streamErr
			}
			switch ev.Type {
			case llm.StreamEventContent:
				if ev.Content != "" {
					sb.WriteString(ev.Content)
					if stream && cb != nil {
						se := NewStreamEvent(EventContentChunk, ChannelSupervisor, LevelInfo, ev.Content)
						se.Meta = map[string]string{MetaKeySupScenario: s.supTitle()}
						cb(se)
					}
				}
			case llm.StreamEventReasoning:
				// Reasoning is not exposed to the SUP block; it is still
				// accumulated so the structured result can be parsed.
				if ev.Content != "" {
					sb.WriteString(ev.Content)
				}
			case llm.StreamEventToolCall:
				if ev.ToolCall != nil {
					toolCalls = append(toolCalls, *ev.ToolCall)
				}
			case llm.StreamEventError:
				if ev.Err != nil {
					streamErr = ev.Err
				}
			case llm.StreamEventDone:
				return sb.String(), toolCalls, streamErr
			}
		}
	}
}
