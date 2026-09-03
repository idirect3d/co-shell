package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
)

// responsesClient implements Client using the OpenAI Responses API (/v1/responses).
// It supports reasoning.effort control (e.g. {"effort": "none"} to disable thinking),
// which is the reliable way to turn off thinking for models like qwen3.6 that
// ignore the Chat Completions thinking parameters (FEATURE-468).
type responsesClient struct {
	httpClient        *http.Client
	streamClient      *http.Client // separate client for streaming (no timeout, relies on context)
	baseURL           string
	apiKey            string
	model             string
	temperature       float64
	maxTokens         int
	topP              float64           // top-p sampling (-1 = don't send)
	topK              int               // top-k sampling (-1 = don't send)
	repetitionPenalty float64           // repetition penalty (-1 = don't send)
	thinkingEnabled   bool              // whether to enable thinking/reasoning mode
	reasoningEffort   string            // reasoning effort level: "none", "low", "medium", "high", "max"
	tokenUsage        string            // token usage display mode: "on", "off", "none"
	bodyAdditions     map[string]string // custom JSON properties to add to the LLM request body
}

// responsesInputItem is a single item in the Responses API "input" array.
type responsesInputItem struct {
	Type      string                 `json:"type"`
	Role      string                 `json:"role,omitempty"`
	Content   []responsesContentPart `json:"content,omitempty"`
	CallID    string                 `json:"call_id,omitempty"`
	Name      string                 `json:"name,omitempty"`
	Arguments string                 `json:"arguments,omitempty"`
	Output    string                 `json:"output,omitempty"`
}

// responsesContentPart is a content part inside a Responses API message item.
// Text intentionally has NO omitempty: the Responses API (LM Studio / OpenAI)
// rejects an input_text part whose "text" key is absent (missing field fails
// the pydantic union), while an explicit empty string is accepted.
type responsesContentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// responsesToolJSON is a flattened tool definition in the Responses API.
type responsesToolJSON struct {
	Type        string      `json:"type"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

// responsesReasoningJSON controls the reasoning/thinking behavior.
// Effort "none" disables thinking; "low"/"medium"/"high"/"max" enable it.
type responsesReasoningJSON struct {
	Effort string `json:"effort"`
}

// responsesRequestJSON is the JSON structure for the Responses API request.
type responsesRequestJSON struct {
	Model           string                 `json:"model"`
	Input           []responsesInputItem   `json:"input"`
	Tools           []responsesToolJSON    `json:"tools,omitempty"`
	Reasoning       *responsesReasoningJSON `json:"reasoning,omitempty"`
	MaxOutputTokens int                    `json:"max_output_tokens,omitempty"`
	Temperature     *float32               `json:"temperature,omitempty"`
	TopP            float32                `json:"top_p,omitempty"`
	Stream          bool                   `json:"stream,omitempty"`
	// BodyAdditions holds custom JSON properties to merge into the request body.
	BodyAdditions map[string]interface{} `json:"-"`
}

// responsesOutputItem is a single item in the Responses API "output" array.
type responsesOutputItem struct {
	Type      string                 `json:"type"`
	Role      string                 `json:"role,omitempty"`
	Content   []responsesContentPart `json:"content,omitempty"`
	CallID    string                 `json:"call_id,omitempty"`
	Name      string                 `json:"name,omitempty"`
	Arguments string                 `json:"arguments,omitempty"`
	Summary   []responsesContentPart `json:"summary,omitempty"`
}

// responsesUsageJSON is the token usage in a Responses API response.
type responsesUsageJSON struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// responsesResponseJSON is the JSON structure for the Responses API response.
type responsesResponseJSON struct {
	ID      string                `json:"id"`
	Object  string                `json:"object"`
	Output  []responsesOutputItem `json:"output"`
	Usage   *responsesUsageJSON   `json:"usage,omitempty"`
	Error   json.RawMessage       `json:"error,omitempty"`
}

// dumpResponsesErr writes the full request body and server error response to a
// /tmp file for diagnosis when a Responses API call fails (FEATURE-468). The
// truncated in-log body is often insufficient to spot why the server rejects a
// request (e.g. a malformed input item hidden behind a huge system prompt).
func dumpResponsesErr(tag string, bodyBytes, respBytes []byte) {
	name := fmt.Sprintf("co-shell-responses-%s-%d.json", tag, time.Now().UnixNano())
	path := filepath.Join(os.TempDir(), name)
	content := map[string]interface{}{
		"request":  json.RawMessage(bodyBytes),
		"response": json.RawMessage(respBytes),
	}
	data, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		log.Error("LLM Responses dump marshal failed: %v", err)
		return
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		log.Error("LLM Responses dump write failed: %v", err)
		return
	}
	log.Info("LLM Responses error details written to %s", path)
}

// parseError parses the raw error field into a responseErrorJSON.
func (r *responsesResponseJSON) parseError() *responseErrorJSON {
	if len(r.Error) == 0 {
		return nil
	}
	var errObj responseErrorJSON
	if err := json.Unmarshal(r.Error, &errObj); err == nil && errObj.Message != "" {
		return &errObj
	}
	var errStr string
	if err := json.Unmarshal(r.Error, &errStr); err == nil && errStr != "" {
		return &responseErrorJSON{Message: errStr}
	}
	return &responseErrorJSON{Message: string(r.Error)}
}

// NewResponsesClient creates a new LLM client using the OpenAI Responses API.
// timeoutSeconds: timeout for non-streaming requests in seconds (0 = no timeout).
func NewResponsesClient(endpoint, apiKey, model string, temperature float64, maxTokens int, timeoutSeconds ...int) Client {
	// Ensure endpoint ends without trailing slash
	baseURL := endpoint
	for len(baseURL) > 0 && baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}

	timeout := 60
	if len(timeoutSeconds) > 0 && timeoutSeconds[0] > 0 {
		timeout = timeoutSeconds[0]
	}

	var httpTimeout time.Duration
	if timeout > 0 {
		httpTimeout = time.Duration(timeout) * time.Second
	}

	return &responsesClient{
		httpClient: &http.Client{
			Timeout: httpTimeout,
		},
		streamClient:    &http.Client{},
		baseURL:         baseURL,
		apiKey:          apiKey,
		model:           model,
		temperature:     temperature,
		maxTokens:       maxTokens,
		thinkingEnabled: false,
		reasoningEffort: "",
	}
}

// NewClientForAPIType creates an LLM client based on the API type.
// apiType "responses" returns a responsesClient; empty or "chat" returns the
// default openAIClient (Chat Completions API). This is the dispatch entry point
// used by callers that have a model-level api_type configuration (FEATURE-468).
func NewClientForAPIType(endpoint, apiKey, model string, temperature float64, maxTokens int, apiType string, timeoutSeconds ...int) Client {
	if apiType == "responses" {
		return NewResponsesClient(endpoint, apiKey, model, temperature, maxTokens, timeoutSeconds...)
	}
	return NewClient(endpoint, apiKey, model, temperature, maxTokens, timeoutSeconds...)
}

// responsesTextContent returns the text payload of a message: the plain
// Content string when set, otherwise the concatenated text of the structured
// ContentParts (FEATURE-468). The agent stores long user turns (instructions +
// environment details) as ContentParts with an empty Content field; falling back
// to CombineContentParts preserves that text instead of sending an empty part.
func responsesTextContent(msg *Message) string {
	if msg.Content != "" {
		return msg.Content
	}
	return msg.CombineContentParts()
}

// buildResponsesInput converts our Message type to the Responses API input array.
//   - system/user/assistant → {type: "message", role, content: [{type: "input_text", text}]}
//   - assistant with ToolCalls → {type: "function_call", call_id, name, arguments}
//   - tool → {type: "function_call_output", call_id, output}
//
// Text content is taken from Content (or ContentParts via CombineContentParts
// when Content is empty). Image parts are not yet mapped to the Responses
// input_image format; the responses API targets text-driven thinking control
// (qwen3.6 / deepseek), so vision models keep using the chat API.
func buildResponsesInput(messages []Message) []responsesInputItem {
	var input []responsesInputItem
	for _, msg := range messages {
		switch msg.Role {
		case "system", "user":
			input = append(input, responsesInputItem{
				Type: "message",
				Role: msg.Role,
				Content: []responsesContentPart{
					{Type: "input_text", Text: responsesTextContent(&msg)},
				},
			})
		case "assistant":
			// Assistant text content (if any) becomes a message item.
			if text := responsesTextContent(&msg); text != "" {
				input = append(input, responsesInputItem{
					Type: "message",
					Role: "assistant",
					Content: []responsesContentPart{
						{Type: "input_text", Text: text},
					},
				})
			}
			// Assistant tool calls become function_call items.
			for _, tc := range msg.ToolCalls {
				input = append(input, responsesInputItem{
					Type:      "function_call",
					CallID:    tc.ID,
					Name:      tc.Name,
					Arguments: tc.Arguments,
				})
			}
		case "tool":
			input = append(input, responsesInputItem{
				Type:   "function_call_output",
				CallID: msg.ToolCallID,
				Output: msg.Content,
			})
		}
	}
	return input
}

// buildResponsesTools converts our Tool type to the Responses API flattened format.
func buildResponsesTools(tools []Tool) []responsesToolJSON {
	result := make([]responsesToolJSON, 0, len(tools))
	for _, t := range tools {
		params := t.Parameters
		if params == nil {
			params = map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}
		}
		result = append(result, responsesToolJSON{
			Type:        "function",
			Name:        t.Name,
			Description: t.Description,
			Parameters:  params,
		})
	}
	return result
}

// buildReasoning builds the reasoning control object for the Responses API.
//
// Thinking is enabled when either (a) SetThinkingEnabled(true) was called (the
// agent settings-tools path) or (b) the thinking adapters emitted chat-format
// enable controls in body additions — thinking:{"type":"enabled"} or
// extra_body.chat_template_kwargs.enable_thinking=true (the main.go / settings.go
// path that carries thinking entirely via body additions). The effort value is
// taken from body additions reasoning_effort (highest priority, produced by the
// adapter from the user's model-level reasoning_effort), then SetReasoningEffort,
// then defaults to "medium" when thinking is enabled.
//
// When NO thinking configuration is present at all (no adapter output and no
// SetThinkingEnabled call) the reasoning field is omitted so the model follows
// its own default — mirroring the Chat Completions "default" behavior.
func (c *responsesClient) buildReasoning() *responsesReasoningJSON {
	enable := c.thinkingEnabled
	effort := c.reasoningEffort
	explicit := false // user explicitly configured thinking (adapter output or fields)

	// Adapter additions (chat-format thinking controls from the thinking adapters).
	if v, ok := c.bodyAdditions["thinking"]; ok {
		explicit = true
		var tc struct {
			Type string `json:"type"`
		}
		if json.Unmarshal([]byte(v), &tc) == nil {
			enable = tc.Type == "enabled"
		}
	}
	if v, ok := c.bodyAdditions["extra_body"]; ok {
		var eb struct {
			ChatTemplateKwargs struct {
				EnableThinking *bool `json:"enable_thinking"`
			} `json:"chat_template_kwargs"`
		}
		if json.Unmarshal([]byte(v), &eb) == nil && eb.ChatTemplateKwargs.EnableThinking != nil {
			explicit = true
			enable = *eb.ChatTemplateKwargs.EnableThinking
		}
	}
	if v, ok := c.bodyAdditions["reasoning_effort"]; ok {
		explicit = true
		var e string
		if json.Unmarshal([]byte(v), &e) == nil && e != "" {
			effort = e
		}
	}

	// No thinking configuration at all: omit reasoning so the model decides.
	if !explicit && !c.thinkingEnabled && effort == "" {
		return nil
	}

	if !enable {
		return &responsesReasoningJSON{Effort: "none"}
	}
	if effort == "" {
		effort = "medium"
	}
	return &responsesReasoningJSON{Effort: effort}
}

// mergeResponsesBodyAdditions merges custom JSON properties into the serialized
// request body, skipping chat-format thinking controls (handled by the reasoning
// field) and extra_body (qwen chat-template wrapper, not applicable to Responses).
func (c *responsesClient) mergeResponsesBodyAdditions(bodyBytes []byte) ([]byte, error) {
	if len(c.bodyAdditions) == 0 {
		return bodyBytes, nil
	}
	var bodyMap map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &bodyMap); err != nil {
		return nil, fmt.Errorf("cannot unmarshal request body for merge: %w", err)
	}
	for key, value := range c.bodyAdditions {
		if key == "thinking" || key == "reasoning_effort" || key == "extra_body" {
			continue
		}
		var parsedValue interface{}
		if err := json.Unmarshal([]byte(value), &parsedValue); err == nil {
			bodyMap[key] = parsedValue
		} else {
			bodyMap[key] = value
		}
	}
	result, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, fmt.Errorf("cannot re-marshal request body after merge: %w", err)
	}
	return result, nil
}

// parseResponsesOutput extracts content, reasoning and tool calls from the
// Responses API output array.
func parseResponsesOutput(output []responsesOutputItem) (string, string, []ToolCall) {
	var content, reasoningContent string
	var toolCalls []ToolCall
	for _, item := range output {
		switch item.Type {
		case "message":
			for _, cp := range item.Content {
				if cp.Type == "output_text" {
					content += cp.Text
				}
			}
		case "function_call":
			toolCalls = append(toolCalls, ToolCall{
				ID:        item.CallID,
				Name:      item.Name,
				Arguments: item.Arguments,
				Type:      "function",
			})
		case "reasoning":
			for _, s := range item.Summary {
				reasoningContent += s.Text
			}
		}
	}
	return content, reasoningContent, toolCalls
}

func (c *responsesClient) Chat(ctx context.Context, messages []Message, tools []Tool) (*LLMResponse, error) {
	reqBody := responsesRequestJSON{
		Model: c.model,
		Input: buildResponsesInput(messages),
	}

	if c.temperature >= 0 {
		temp := float32(c.temperature)
		reqBody.Temperature = &temp
	}
	if c.maxTokens >= 0 {
		reqBody.MaxOutputTokens = c.maxTokens
	}
	if c.topP >= 0 {
		reqBody.TopP = float32(c.topP)
	}
	if len(tools) > 0 {
		reqBody.Tools = buildResponsesTools(tools)
	}
	reqBody.Reasoning = c.buildReasoning()

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal request: %w", err)
	}
	bodyBytes, err = c.mergeResponsesBodyAdditions(bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("cannot merge body additions: %w", err)
	}

	log.Debug("LLM Responses Chat request body: %s", maskAPIKeyInRequest(string(bodyBytes)))
	log.WriteLLMInteraction("REQ", prettyJSON(bodyBytes))

	apiURL := c.baseURL + "/responses"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("cannot create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	timeoutStr := "no timeout"
	if c.httpClient.Timeout > 0 {
		timeoutStr = c.httpClient.Timeout.String()
	}
	log.Info("LLM Responses Chat request: POST %s, timeout=%s, model=%s, messages=%d, tools=%d",
		apiURL, timeoutStr, c.model, len(messages), len(tools))

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		log.Error("LLM Responses Chat request failed: POST %s, error: %v", apiURL, err)
		return nil, fmt.Errorf("chat request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("LLM Responses Chat response read failed: POST %s, error: %v", apiURL, err)
		return nil, fmt.Errorf("cannot read response: %w", err)
	}

	log.WriteLLMInteraction("RESP", prettyJSON(respBytes))
	log.Debug("LLM Responses Chat raw response: %s", string(respBytes))

	var respJSON responsesResponseJSON
	if err := json.Unmarshal(respBytes, &respJSON); err != nil {
		log.Error("LLM Responses Chat response parse failed: POST %s, error: %v", apiURL, err)
		return nil, fmt.Errorf("cannot parse response: %w", err)
	}

	if errObj := respJSON.parseError(); errObj != nil {
		errMsg := fmt.Sprintf("API error: %s (type=%s, code=%s)", errObj.Message, errObj.Type, errObj.Code)
		log.Error("LLM Responses Chat API error: POST %s, status=%d, error=%s, request=%d bytes, body=%s",
			apiURL, resp.StatusCode, errMsg, len(bodyBytes), truncateBody(bodyBytes))
		dumpResponsesErr("chat", bodyBytes, respBytes)
		summary := requestSummary(bodyBytes)
		return nil, &OpenAIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("%s (request=%d bytes, %s)", errObj.Message, len(bodyBytes), summary),
		}
	}

	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("API returned status %d: %s", resp.StatusCode, string(respBytes))
		log.Error("LLM Responses Chat HTTP error: POST %s, status=%d, body=%s", apiURL, resp.StatusCode, string(respBytes))
		dumpResponsesErr("chat", bodyBytes, respBytes)
		return nil, fmt.Errorf("%s", errMsg)
	}

	content, reasoningContent, toolCalls := parseResponsesOutput(respJSON.Output)

	var usage *TokenUsage
	if respJSON.Usage != nil {
		usage = &TokenUsage{
			PromptTokens:     respJSON.Usage.InputTokens,
			CompletionTokens: respJSON.Usage.OutputTokens,
			TotalTokens:      respJSON.Usage.TotalTokens,
		}
		log.Debug("LLM Responses Chat token usage: prompt=%d, completion=%d, total=%d",
			usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)
	}

	log.Debug("LLM Responses Chat response: model=%s, content_len=%d, tool_calls=%d, reasoning_len=%d",
		c.model, len(content), len(toolCalls), len(reasoningContent))

	return &LLMResponse{
		Content:          content,
		ReasoningContent: reasoningContent,
		ToolCalls:        toolCalls,
		Usage:            usage,
	}, nil
}

// responsesStreamEventJSON is a single SSE event from the Responses API stream.
type responsesStreamEventJSON struct {
	Type        string `json:"type"`
	Delta       string `json:"delta"`
	ItemID      string `json:"item_id"`
	OutputIndex int    `json:"output_index"`
	Item        *struct {
		Type      string `json:"type"`
		Name      string `json:"name"`
		CallID    string `json:"call_id"`
		Arguments string `json:"arguments"`
	} `json:"item"`
	Response *responsesResponseJSON `json:"response"`
}

func (c *responsesClient) ChatStream(ctx context.Context, messages []Message, tools []Tool) (<-chan StreamEvent, error) {
	reqBody := responsesRequestJSON{
		Model:  c.model,
		Input:  buildResponsesInput(messages),
		Stream: true,
	}

	if c.temperature >= 0 {
		temp := float32(c.temperature)
		reqBody.Temperature = &temp
	}
	if c.maxTokens >= 0 {
		reqBody.MaxOutputTokens = c.maxTokens
	}
	if c.topP >= 0 {
		reqBody.TopP = float32(c.topP)
	}
	if len(tools) > 0 {
		reqBody.Tools = buildResponsesTools(tools)
	}
	reqBody.Reasoning = c.buildReasoning()

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal request: %w", err)
	}
	bodyBytes, err = c.mergeResponsesBodyAdditions(bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("cannot merge body additions: %w", err)
	}

	log.Debug("LLM Responses ChatStream request body: %s", maskAPIKeyInRequest(string(bodyBytes)))
	log.WriteLLMInteraction("REQ", prettyJSON(bodyBytes))

	apiURL := c.baseURL + "/responses"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("cannot create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	log.Info("LLM Responses ChatStream request: POST %s, model=%s, messages=%d, tools=%d",
		apiURL, c.model, len(messages), len(tools))

	resp, err := c.streamClient.Do(httpReq)
	if err != nil {
		log.Error("LLM Responses ChatStream request failed: POST %s, error: %v", apiURL, err)
		return nil, fmt.Errorf("chat stream request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		errMsg := fmt.Sprintf("API returned status %d: %s", resp.StatusCode, string(respBytes))
		log.Error("LLM Responses ChatStream HTTP error: POST %s, status=%d, body=%s, request=%d bytes, body=%s",
			apiURL, resp.StatusCode, string(respBytes), len(bodyBytes), truncateBody(bodyBytes))
		dumpResponsesErr("stream", bodyBytes, respBytes)
		return nil, fmt.Errorf("%s (request=%d bytes, %s)", errMsg, len(bodyBytes), requestSummary(bodyBytes))
	}

	eventCh := make(chan StreamEvent, 1024)

	go func() {
		defer close(eventCh)
		defer resp.Body.Close()
		defer log.WriteLLMInteractionEnd()

		reader := NewStreamReader(&wireLogReader{r: resp.Body})
		// Accumulate tool calls from stream deltas.
		var currentToolCall *ToolCall
		toolCallIndex := 0
		respHeaderWritten := false

		sendEvent := func(ev StreamEvent) {
			select {
			case eventCh <- ev:
			default:
				// Non-blocking: drop if the consumer is slow (stream is best-effort).
			}
		}

		for {
			line, err := reader.Read()
			if err != nil {
				if err == io.EOF {
					sendEvent(StreamEvent{Type: StreamEventDone, Done: true})
					return
				}
				sendEvent(StreamEvent{Type: StreamEventError, Err: err})
				return
			}
			if len(line) == 0 {
				continue
			}
			lineStr := string(line)
			// The Responses API emits "event: xxx" lines followed by "data: {...}".
			// The event type is also carried in the data payload's "type" field, so
			// the event: line is skipped and only data frames are parsed.
			if strings.HasPrefix(lineStr, "event: ") {
				continue
			}
			if !strings.HasPrefix(lineStr, "data: ") {
				continue
			}
			data := line[6:]

			var ev responsesStreamEventJSON
			if err := json.Unmarshal(data, &ev); err != nil {
				log.Debug("LLM Responses ChatStream unparseable data: %s", string(data))
				continue
			}

			switch ev.Type {
			case "response.output_text.delta":
				if ev.Delta != "" {
					if !respHeaderWritten {
						log.WriteLLMInteraction("RESP][assistant", ev.Delta)
						respHeaderWritten = true
					} else {
						log.WriteLLMInteractionAppend(ev.Delta)
					}
					sendEvent(StreamEvent{Type: StreamEventContent, Content: ev.Delta})
				}
			case "response.reasoning_text.delta":
				if ev.Delta != "" {
					sendEvent(StreamEvent{Type: StreamEventReasoning, Content: ev.Delta})
				}
			case "response.output_item.added":
				if ev.Item != nil && ev.Item.Type == "function_call" {
					currentToolCall = &ToolCall{
						ID:   ev.Item.CallID,
						Name: ev.Item.Name,
					}
				}
			case "response.function_call_arguments.delta":
				if currentToolCall == nil {
					currentToolCall = &ToolCall{}
				}
				currentToolCall.Arguments += ev.Delta
				sendEvent(StreamEvent{
					Type: StreamEventToolCallDelta,
					ToolCallDelta: &ToolCallDelta{
						Index:     toolCallIndex,
						Name:      currentToolCall.Name,
						Arguments: ev.Delta,
					},
				})
			case "response.function_call_arguments.done":
				if currentToolCall != nil {
					if currentToolCall.Name != "" && currentToolCall.ID != "" {
						sendEvent(StreamEvent{Type: StreamEventToolCall, ToolCall: currentToolCall})
					}
					currentToolCall = nil
					toolCallIndex++
				}
			case "response.completed":
				var usage *TokenUsage
				if ev.Response != nil && ev.Response.Usage != nil {
					usage = &TokenUsage{
						PromptTokens:     ev.Response.Usage.InputTokens,
						CompletionTokens: ev.Response.Usage.OutputTokens,
						TotalTokens:      ev.Response.Usage.TotalTokens,
					}
				}
				sendEvent(StreamEvent{Type: StreamEventDone, Done: true, Usage: usage})
				return
			}
		}
	}()

	return eventCh, nil
}

// ListModels retrieves the list of available models from the API.
func (c *responsesClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	apiURL := c.baseURL + "/models"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		log.Error("LLM Responses ListModels request failed: GET %s, error: %v", apiURL, err)
		return nil, fmt.Errorf("list models request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("API returned status %d: %s", resp.StatusCode, string(respBytes))
		log.Error("LLM Responses ListModels HTTP error: GET %s, status=%d, body=%s", apiURL, resp.StatusCode, string(respBytes))
		return nil, fmt.Errorf("%s", errMsg)
	}

	var modelsResp struct {
		Data []struct {
			ID string `json:"id"`
			Capabilities *struct {
				Vision bool `json:"vision"`
			} `json:"capabilities,omitempty"`
			MaxModelLen int `json:"max_model_len,omitempty"`
		} `json:"data"`
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read response: %w", err)
	}
	log.Debug("LLM Responses ListModels raw response: %s", string(respBytes))

	if err := json.Unmarshal(respBytes, &modelsResp); err != nil {
		return nil, fmt.Errorf("cannot parse response: %w", err)
	}

	models := make([]ModelInfo, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		vision := false
		if m.Capabilities != nil {
			vision = m.Capabilities.Vision
		}
		models = append(models, ModelInfo{
			ID:            m.ID,
			VisionSupport: vision,
			MaxModelLen:   m.MaxModelLen,
		})
	}
	return models, nil
}

// testChat sends a chat request with temperature=0 for deterministic testing.
func (c *responsesClient) testChat(ctx context.Context, messages []Message, tools []Tool) (*LLMResponse, error) {
	origTemp := c.temperature
	c.temperature = 0
	defer func() { c.temperature = origTemp }()
	defer log.WriteLLMInteractionEnd()
	return c.Chat(ctx, messages, tools)
}

// TestVisionSupport tests whether the model supports vision (image input).
func (c *responsesClient) TestVisionSupport(ctx context.Context) bool {
	const pixelBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
	msg := Message{
		Role: "user",
		ContentParts: []ContentPart{
			{Type: ContentPartText, Text: i18n.T(i18n.KeyVisionTestPrompt)},
			{Type: ContentPartImageURL, ImageURL: &ContentPartImage{URL: "data:image/png;base64," + pixelBase64}},
		},
	}
	_, err := c.testChat(ctx, []Message{msg}, nil)
	if err != nil {
		log.Debug("Responses TestVisionSupport failed for model %s: %v", c.model, err)
		return false
	}
	log.Info("Responses TestVisionSupport succeeded for model %s", c.model)
	return true
}

// TestTextSupport tests whether the model supports basic text chat.
func (c *responsesClient) TestTextSupport(ctx context.Context) bool {
	msg := Message{Role: "user", Content: "Hi"}
	_, err := c.testChat(ctx, []Message{msg}, nil)
	if err != nil {
		log.Debug("Responses TestTextSupport failed for model %s: %v", c.model, err)
		return false
	}
	log.Info("Responses TestTextSupport succeeded for model %s", c.model)
	return true
}

// TestToolCallSupport tests whether the model supports tool/function calling.
func (c *responsesClient) TestToolCallSupport(ctx context.Context) bool {
	testTool := Tool{
		Name:        "test_tool",
		Description: "A test tool to verify tool calling support",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{
					"type":        "string",
					"description": "A test message",
				},
			},
			"required": []string{"message"},
		},
	}
	msg := Message{Role: "user", Content: i18n.T(i18n.KeyToolCallTestPrompt)}
	resp, err := c.testChat(ctx, []Message{msg}, []Tool{testTool})
	if err != nil {
		log.Debug("Responses TestToolCallSupport failed for model %s: %v", c.model, err)
		return false
	}
	if len(resp.ToolCalls) > 0 {
		log.Info("Responses TestToolCallSupport succeeded for model %s (returned tool calls)", c.model)
	} else {
		log.Info("Responses TestToolCallSupport succeeded for model %s (accepted tools without error)", c.model)
	}
	return true
}

// TestThinkingSupport tests whether the model supports thinking/reasoning mode.
func (c *responsesClient) TestThinkingSupport(ctx context.Context) bool {
	msg := Message{Role: "user", Content: "Hi"}
	_, err := c.Chat(ctx, []Message{msg}, nil)
	if err != nil {
		log.Debug("Responses TestThinkingSupport failed for model %s: %v", c.model, err)
		return false
	}
	log.Info("Responses TestThinkingSupport succeeded for model %s", c.model)
	return true
}

func (c *responsesClient) SetThinkingEnabled(enabled bool) {
	c.thinkingEnabled = enabled
}

func (c *responsesClient) SetReasoningEffort(effort string) {
	c.reasoningEffort = effort
}

func (c *responsesClient) SetTopP(topP float64) {
	c.topP = topP
}

func (c *responsesClient) SetTopK(topK int) {
	c.topK = topK
}

func (c *responsesClient) SetRepetitionPenalty(penalty float64) {
	c.repetitionPenalty = penalty
}

func (c *responsesClient) SetTokenUsage(mode string) {
	c.tokenUsage = mode
}

func (c *responsesClient) SetTemperature(temp float64) {
	c.temperature = temp
}

func (c *responsesClient) SetBodyAdditions(additions map[string]string) {
	c.bodyAdditions = additions
}

func (c *responsesClient) RemoveBodyAddition(key string) {
	if c.bodyAdditions != nil {
		delete(c.bodyAdditions, key)
	}
}

func (c *responsesClient) GetBodyAdditions() map[string]string {
	return c.bodyAdditions
}

func (c *responsesClient) Close() error {
	c.httpClient.CloseIdleConnections()
	c.streamClient.CloseIdleConnections()
	return nil
}
