package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestNewClientForAPIType covers UC-0002: NewClient dispatch by api_type.
func TestNewClientForAPIType(t *testing.T) {
	tests := []struct {
		name    string
		apiType string
		wantRC  bool // true if a responsesClient is expected
	}{
		{name: "empty defaults to chat", apiType: "", wantRC: false},
		{name: "chat explicit", apiType: "chat", wantRC: false},
		{name: "responses", apiType: "responses", wantRC: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClientForAPIType("http://example.com/v1", "key", "m", 0, 0, tt.apiType, 10)
			_, isRC := c.(*responsesClient)
			if isRC != tt.wantRC {
				t.Errorf("NewClientForAPIType(apiType=%q) responsesClient=%v, want %v", tt.apiType, isRC, tt.wantRC)
			}
			c.Close()
		})
	}
}

// TestBuildResponsesInput covers UC-0003: Message → input conversion.
func TestBuildResponsesInput(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "thinking...", ReasoningContent: "rc"},
		{Role: "assistant", ToolCalls: []ToolCall{{ID: "call_1", Name: "get_weather", Arguments: `{"city":"beijing"}`}}},
		{Role: "tool", ToolCallID: "call_1", Content: "sunny"},
	}
	input := buildResponsesInput(messages)
	if len(input) != 5 {
		t.Fatalf("expected 5 input items, got %d", len(input))
	}

	// system → message
	if input[0].Type != "message" || input[0].Role != "system" || input[0].Content[0].Type != "input_text" || input[0].Content[0].Text != "sys" {
		t.Errorf("system item wrong: %+v", input[0])
	}
	// user → message
	if input[1].Type != "message" || input[1].Role != "user" || input[1].Content[0].Text != "hello" {
		t.Errorf("user item wrong: %+v", input[1])
	}
	// assistant text → message
	if input[2].Type != "message" || input[2].Role != "assistant" || input[2].Content[0].Text != "thinking..." {
		t.Errorf("assistant text item wrong: %+v", input[2])
	}
	// assistant tool call → function_call
	if input[3].Type != "function_call" || input[3].CallID != "call_1" || input[3].Name != "get_weather" || input[3].Arguments != `{"city":"beijing"}` {
		t.Errorf("function_call item wrong: %+v", input[3])
	}
	// tool → function_call_output
	if input[4].Type != "function_call_output" || input[4].CallID != "call_1" || input[4].Output != "sunny" {
		t.Errorf("function_call_output item wrong: %+v", input[4])
	}
}

// TestResponsesTextContentFallback verifies a user message whose text lives in
// ContentParts (Content empty — the agent's structured multi-part user turns)
// is converted with the concatenated text instead of an empty part (FEATURE-468).
func TestResponsesTextContentFallback(t *testing.T) {
	msg := Message{Role: "user"}
	msg.AppendTextPart("first part\n")
	msg.AppendTextPart("second part")
	input := buildResponsesInput([]Message{msg})
	if len(input) != 1 || input[0].Type != "message" || input[0].Role != "user" {
		t.Fatalf("parts user conversion wrong: %+v", input)
	}
	if got := input[0].Content[0].Text; got != "first part\n\nsecond part" {
		t.Errorf("parts text = %q, want parts joined by newline", got)
	}
}

// TestResponsesEmptyTextKeepsKey guards the LM Studio / OpenAI requirement that
// an input_text part always carries a "text" key (missing key fails the input
// union even when the text is empty).
func TestResponsesEmptyTextKeepsKey(t *testing.T) {
	input := buildResponsesInput([]Message{{Role: "user"}})
	body, err := json.Marshal(input[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(body), `"text":""`) {
		t.Errorf("input part must serialize an explicit empty text key, got: %s", body)
	}
}

// TestBuildResponsesTools covers UC-0004: Tool → flattened tools conversion.
func TestBuildResponsesTools(t *testing.T) {
	tools := []Tool{
		{
			Name:        "get_weather",
			Description: "Get weather",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"city": map[string]interface{}{"type": "string"},
				},
			},
		},
	}
	got := buildResponsesTools(tools)
	if len(got) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(got))
	}
	if got[0].Type != "function" || got[0].Name != "get_weather" || got[0].Description != "Get weather" {
		t.Errorf("tool wrong: %+v", got[0])
	}
	if got[0].Parameters == nil {
		t.Error("tool parameters must not be nil")
	}
}

// TestParseResponsesOutput covers UC-0005: output[] → LLMResponse extraction.
func TestParseResponsesOutput(t *testing.T) {
	output := []responsesOutputItem{
		{Type: "reasoning", Summary: []responsesContentPart{{Type: "summary_text", Text: "let me think"}}},
		{Type: "message", Content: []responsesContentPart{{Type: "output_text", Text: "The answer"}}},
		{Type: "function_call", CallID: "call_2", Name: "search", Arguments: `{"q":"x"}`},
	}
	content, reasoning, calls := parseResponsesOutput(output)
	if content != "The answer" {
		t.Errorf("content = %q, want %q", content, "The answer")
	}
	if reasoning != "let me think" {
		t.Errorf("reasoning = %q, want %q", reasoning, "let me think")
	}
	if len(calls) != 1 || calls[0].ID != "call_2" || calls[0].Name != "search" || calls[0].Arguments != `{"q":"x"}` {
		t.Errorf("tool calls wrong: %+v", calls)
	}
}

// TestResponsesChatRequestAndResponse covers UC-0003/0004/0005/0007 end-to-end
// against a mock /responses endpoint: verifies the request body format and the
// parsed LLMResponse.
func TestResponsesChatRequestAndResponse(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": "resp_1",
			"object": "response",
			"output": [
				{"type": "message", "role": "assistant", "content": [{"type": "output_text", "text": "hi there"}]}
			],
			"usage": {"input_tokens": 10, "output_tokens": 5, "total_tokens": 15}
		}`))
	}))
	defer srv.Close()

	c := NewResponsesClient(srv.URL, "key", "test-model", 0.7, 2048, 10)
	defer c.Close()

	// UC-0007: thinking disabled → reasoning.effort=none
	c.SetThinkingEnabled(false)
	c.SetReasoningEffort("none")

	resp, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}
	if resp.Content != "hi there" {
		t.Errorf("content = %q, want %q", resp.Content, "hi there")
	}
	if resp.Usage == nil || resp.Usage.PromptTokens != 10 || resp.Usage.CompletionTokens != 5 || resp.Usage.TotalTokens != 15 {
		t.Errorf("usage wrong: %+v", resp.Usage)
	}

	// Verify request body format.
	if gotBody["model"] != "test-model" {
		t.Errorf("model = %v", gotBody["model"])
	}
	if gotBody["max_output_tokens"] != float64(2048) {
		t.Errorf("max_output_tokens = %v", gotBody["max_output_tokens"])
	}
	reasoning, ok := gotBody["reasoning"].(map[string]interface{})
	if !ok || reasoning["effort"] != "none" {
		t.Errorf("reasoning = %v, want effort=none", gotBody["reasoning"])
	}
	input, ok := gotBody["input"].([]interface{})
	if !ok || len(input) != 1 {
		t.Fatalf("input = %v", gotBody["input"])
	}
	first := input[0].(map[string]interface{})
	if first["type"] != "message" || first["role"] != "user" {
		t.Errorf("input[0] = %v", first)
	}
}

// TestResponsesChatStream covers UC-0006: streaming event parsing.
func TestResponsesChatStream(t *testing.T) {
	stream := "" +
		"event: response.created\n" +
		"data: {\"type\":\"response.created\"}\n\n" +
		"event: response.reasoning_text.delta\n" +
		"data: {\"type\":\"response.reasoning_text.delta\",\"delta\":\"think\"}\n\n" +
		"event: response.output_text.delta\n" +
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"Hello\"}\n\n" +
		"event: response.output_text.delta\n" +
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\" world\"}\n\n" +
		"event: response.output_item.added\n" +
		"data: {\"type\":\"response.output_item.added\",\"item\":{\"type\":\"function_call\",\"call_id\":\"call_9\",\"name\":\"tool_a\"}}\n\n" +
		"event: response.function_call_arguments.delta\n" +
		"data: {\"type\":\"response.function_call_arguments.delta\",\"delta\":\"{\\\"a\\\":\"}\n\n" +
		"event: response.function_call_arguments.delta\n" +
		"data: {\"type\":\"response.function_call_arguments.delta\",\"delta\":\"1}\"}\n\n" +
		"event: response.function_call_arguments.done\n" +
		"data: {\"type\":\"response.function_call_arguments.done\"}\n\n" +
		"event: response.completed\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":20,\"output_tokens\":8,\"total_tokens\":28}}}\n\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(stream))
	}))
	defer srv.Close()

	c := NewResponsesClient(srv.URL, "key", "test-model", 0, 0, 10)
	defer c.Close()

	eventCh, err := c.ChatStream(context.Background(), []Message{{Role: "user", Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("ChatStream error: %v", err)
	}

	var content, reasoning string
	var toolCall *ToolCall
	var doneUsage *TokenUsage
	done := false
	for ev := range eventCh {
		switch ev.Type {
		case StreamEventContent:
			content += ev.Content
		case StreamEventReasoning:
			reasoning += ev.Content
		case StreamEventToolCall:
			toolCall = ev.ToolCall
		case StreamEventDone:
			done = true
			doneUsage = ev.Usage
		case StreamEventError:
			t.Fatalf("stream error: %v", ev.Err)
		}
	}

	if !done {
		t.Fatal("stream did not end with Done event")
	}
	if content != "Hello world" {
		t.Errorf("content = %q, want %q", content, "Hello world")
	}
	if reasoning != "think" {
		t.Errorf("reasoning = %q, want %q", reasoning, "think")
	}
	if toolCall == nil || toolCall.Name != "tool_a" || toolCall.Arguments != `{"a":1}` {
		t.Errorf("tool call = %+v", toolCall)
	}
	if doneUsage == nil || doneUsage.PromptTokens != 20 || doneUsage.CompletionTokens != 8 {
		t.Errorf("done usage = %+v", doneUsage)
	}
}

// TestResponsesBuildReasoning covers UC-0007: reasoning.effort derivation from
// thinking settings and chat-format body additions.
func TestResponsesBuildReasoning(t *testing.T) {
	tests := []struct {
		name            string
		thinkingEnabled bool
		effort          string
		additions       map[string]string
		want            string // effort value; "" means reasoning must be omitted (nil)
	}{
		{name: "no config omits reasoning", thinkingEnabled: false, effort: "", want: ""},
		{name: "disabled forces none", thinkingEnabled: false, effort: "high", want: "none"},
		{name: "enabled uses effort", thinkingEnabled: true, effort: "low", want: "low"},
		{name: "enabled empty defaults medium", thinkingEnabled: true, effort: "", want: "medium"},
		{name: "enabled with effort none closes thinking", thinkingEnabled: true, effort: "none", want: "none"},
		{name: "reasoning_effort addition overrides", thinkingEnabled: true, effort: "low", additions: map[string]string{"reasoning_effort": `"high"`}, want: "high"},
		{name: "thinking disabled addition forces none", thinkingEnabled: true, effort: "high", additions: map[string]string{"thinking": `{"type":"disabled"}`}, want: "none"},
		{name: "qwen extra_body disable forces none", thinkingEnabled: true, effort: "high", additions: map[string]string{"extra_body": `{"chat_template_kwargs":{"enable_thinking":false}}`}, want: "none"},
		// Adapter-enabled paths (thinking carried solely via body additions, no
		// SetThinkingEnabled call — main.go / cmd/settings.go style).
		{name: "thinking enabled addition enables", thinkingEnabled: false, effort: "", additions: map[string]string{"thinking": `{"type":"enabled"}`}, want: "medium"},
		{name: "qwen extra_body enable defaults medium", thinkingEnabled: false, effort: "", additions: map[string]string{"extra_body": `{"chat_template_kwargs":{"enable_thinking":true}}`}, want: "medium"},
		{name: "adapter reasoning_effort none closes", thinkingEnabled: false, effort: "", additions: map[string]string{"extra_body": `{"chat_template_kwargs":{"enable_thinking":true}}`, "reasoning_effort": `"none"`}, want: "none"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &responsesClient{thinkingEnabled: tt.thinkingEnabled, reasoningEffort: tt.effort, bodyAdditions: tt.additions}
			r := c.buildReasoning()
			if tt.want == "" {
				if r != nil {
					t.Errorf("buildReasoning() = %+v, want nil (omit reasoning)", r)
				}
				return
			}
			if r == nil || r.Effort != tt.want {
				t.Errorf("buildReasoning() = %+v, want effort=%q", r, tt.want)
			}
		})
	}
}

// TestResponsesMergeBodyAdditions verifies that chat-format thinking controls
// are skipped and other additions are merged.
func TestResponsesMergeBodyAdditions(t *testing.T) {
	c := &responsesClient{bodyAdditions: map[string]string{
		"thinking":         `{"type":"disabled"}`,
		"reasoning_effort": `"none"`,
		"extra_body":       `{"x":1}`,
		"custom_field":     `{"a":1}`,
	}}
	body, err := c.mergeResponsesBodyAdditions([]byte(`{"model":"m","input":[]}`))
	if err != nil {
		t.Fatalf("merge error: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if _, ok := m["thinking"]; ok {
		t.Error("thinking must be skipped")
	}
	if _, ok := m["reasoning_effort"]; ok {
		t.Error("reasoning_effort must be skipped")
	}
	if _, ok := m["extra_body"]; ok {
		t.Error("extra_body must be skipped")
	}
	if _, ok := m["custom_field"]; !ok {
		t.Error("custom_field must be merged")
	}
}

// TestResponsesChatError verifies API error handling returns OpenAIError.
func TestResponsesChatError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"message":"bad request","type":"invalid_request_error"}}`))
	}))
	defer srv.Close()

	c := NewResponsesClient(srv.URL, "key", "m", 0, 0, 10)
	defer c.Close()

	_, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "bad request") {
		t.Errorf("error = %q, want contains 'bad request'", err.Error())
	}
}
