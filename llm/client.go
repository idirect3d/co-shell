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
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Message represents a chat message in the conversation.
type Message struct {
	Role             string     `json:"role"`
	Content          string     `json:"content"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
}

// ToolCall represents a function call requested by the LLM.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Type      string `json:"type,omitempty"`
	Index     int    `json:"index,omitempty"` // index in stream delta chunks
}

// Tool defines a function that the LLM can call.
type Tool struct {
	Name        string
	Description string
	Parameters  map[string]interface{}
	Callback    func(ctx context.Context, args map[string]interface{}) (string, error)
}

// ToolResult holds the result of a tool execution.
type ToolResult struct {
	ToolCallID string
	Name       string
	Content    string
}

// LLMResponse is the parsed response from the LLM.
type LLMResponse struct {
	Content          string
	ReasoningContent string
	ToolCalls        []ToolCall
}

// Client is the interface for LLM interactions.
type Client interface {
	// Chat sends a chat completion request and returns the response.
	Chat(ctx context.Context, messages []Message, tools []Tool) (*LLMResponse, error)

	// ChatStream sends a chat completion request with streaming response.
	ChatStream(ctx context.Context, messages []Message, tools []Tool) (<-chan StreamEvent, error)

	// ListModels retrieves the list of available models from the API.
	ListModels(ctx context.Context) ([]string, error)

	// Close cleans up any resources.
	Close() error
}

// StreamEvent represents an event in the streaming response.
type StreamEvent struct {
	Type         StreamEventType
	Content      string
	ToolCall     *ToolCall // accumulated tool call from stream deltas
	FinishReason string    // finish_reason from the stream (e.g. "stop", "tool_calls")
	Done         bool
	Err          error
}

// StreamEventType indicates the type of stream event.
type StreamEventType int

const (
	StreamEventContent StreamEventType = iota
	StreamEventReasoning
	StreamEventToolCall
	StreamEventDone
	StreamEventError
)

// OpenAIError represents an error from the OpenAI API.
type OpenAIError struct {
	StatusCode int
	Message    string
}

func (e *OpenAIError) Error() string {
	return fmt.Sprintf("OpenAI API error (status %d): %s", e.StatusCode, e.Message)
}

// chatMessageJSON is the JSON structure for a single message in the request.
type chatMessageJSON struct {
	Role             string         `json:"role"`
	Content          string         `json:"content"`
	ToolCallID       string         `json:"tool_call_id,omitempty"`
	ToolCalls        []toolCallJSON `json:"tool_calls,omitempty"`
	ReasoningContent string         `json:"reasoning_content,omitempty"`
}

// toolCallJSON is the JSON structure for a tool call in messages.
type toolCallJSON struct {
	ID       string           `json:"id,omitempty"`
	Type     string           `json:"type,omitempty"`
	Function functionCallJSON `json:"function,omitempty"`
}

// functionCallJSON is the JSON structure for a function call.
type functionCallJSON struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// toolJSON is the JSON structure for a tool definition.
type toolJSON struct {
	Type     string                 `json:"type"`
	Function functionDefinitionJSON `json:"function"`
}

// functionDefinitionJSON is the JSON structure for a function definition.
type functionDefinitionJSON struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

// chatRequestJSON is the JSON structure for the chat completion request.
type chatRequestJSON struct {
	Model           string            `json:"model"`
	Messages        []chatMessageJSON `json:"messages"`
	Temperature     float32           `json:"temperature,omitempty"`
	MaxTokens       int               `json:"max_tokens,omitempty"`
	Tools           []toolJSON        `json:"tools,omitempty"`
	Stream          bool              `json:"stream,omitempty"`
	Thinking        *thinkingConfig   `json:"thinking,omitempty"`
	ReasoningEffort string            `json:"reasoning_effort,omitempty"`
}

// thinkingConfig represents the DeepSeek thinking mode configuration.
type thinkingConfig struct {
	Type string `json:"type"`
}

// chatResponseJSON is the JSON structure for the chat completion response.
type chatResponseJSON struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []choiceJSON       `json:"choices"`
	Usage   *usageJSON         `json:"usage,omitempty"`
	Error   *responseErrorJSON `json:"error,omitempty"`
}

// choiceJSON is the JSON structure for a response choice.
type choiceJSON struct {
	Index        int                 `json:"index"`
	Message      responseMessageJSON `json:"message"`
	FinishReason string              `json:"finish_reason"`
}

// responseMessageJSON is the JSON structure for a response message.
type responseMessageJSON struct {
	Role             string         `json:"role"`
	Content          string         `json:"content"`
	ReasoningContent string         `json:"reasoning_content,omitempty"`
	ToolCalls        []toolCallJSON `json:"tool_calls,omitempty"`
}

// usageJSON is the JSON structure for token usage.
type usageJSON struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// responseErrorJSON is the JSON structure for an API error response.
type responseErrorJSON struct {
	Message string `json:"message"`
	Type    string `json:"type,omitempty"`
	Code    string `json:"code,omitempty"`
}

// openAIClient implements Client using the OpenAI-compatible API.
type openAIClient struct {
	httpClient   *http.Client
	streamClient *http.Client // separate client for streaming (no timeout, relies on context)
	baseURL      string
	apiKey       string
	model        string
	temperature  float64
	maxTokens    int
}

// NewClient creates a new LLM client from configuration.
func NewClient(endpoint, apiKey, model string, temperature float64, maxTokens int) Client {
	// Ensure endpoint ends without trailing slash
	baseURL := endpoint
	for len(baseURL) > 0 && baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}

	return &openAIClient{
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		// Stream client has no timeout - relies on context.Context for cancellation.
		// This is necessary because streaming responses can take a long time
		// (e.g., DeepSeek thinking mode, large context processing).
		streamClient: &http.Client{},
		baseURL:      baseURL,
		apiKey:       apiKey,
		model:        model,
		temperature:  temperature,
		maxTokens:    maxTokens,
	}

}

// buildMessages converts our Message type to the JSON-serializable format.
func buildMessages(messages []Message) []chatMessageJSON {
	result := make([]chatMessageJSON, 0, len(messages))
	for _, msg := range messages {
		m := chatMessageJSON{
			Role:             msg.Role,
			Content:          msg.Content,
			ToolCallID:       msg.ToolCallID,
			ReasoningContent: msg.ReasoningContent,
		}

		// Map ToolCalls if present
		if len(msg.ToolCalls) > 0 {
			m.ToolCalls = make([]toolCallJSON, 0, len(msg.ToolCalls))
			for _, tc := range msg.ToolCalls {
				tcj := toolCallJSON{
					ID:   tc.ID,
					Type: "function",
					Function: functionCallJSON{
						Name:      tc.Name,
						Arguments: tc.Arguments,
					},
				}
				m.ToolCalls = append(m.ToolCalls, tcj)
			}
		}

		result = append(result, m)
	}
	return result
}

// buildTools converts our Tool type to the JSON-serializable format.
func buildTools(tools []Tool) []toolJSON {
	result := make([]toolJSON, 0, len(tools))
	for _, t := range tools {
		params := t.Parameters
		if params == nil {
			params = map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}
		}
		result = append(result, toolJSON{
			Type: "function",
			Function: functionDefinitionJSON{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
			},
		})
	}
	return result
}

// parseResponseChoices extracts tool calls and content from the response.
func parseResponseChoices(choices []choiceJSON) (string, string, []ToolCall) {
	var content, reasoningContent string
	var toolCalls []ToolCall

	if len(choices) == 0 {
		return "", "", nil
	}

	choice := choices[0]
	content = choice.Message.Content
	reasoningContent = choice.Message.ReasoningContent

	if choice.Message.ToolCalls != nil {
		for _, tc := range choice.Message.ToolCalls {
			toolCalls = append(toolCalls, ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
				Type:      tc.Type,
			})
		}
	}

	return content, reasoningContent, toolCalls
}

// isThinkingModel checks if the model name suggests thinking/reasoning capability.
func isThinkingModel(model string) bool {
	// DeepSeek models with thinking support
	thinkingModels := []string{"deepseek-v4", "deepseek-r1", "deepseek-reasoner"}
	for _, prefix := range thinkingModels {
		if len(model) >= len(prefix) && model[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

func (c *openAIClient) Chat(ctx context.Context, messages []Message, tools []Tool) (*LLMResponse, error) {
	// Build request body
	reqBody := chatRequestJSON{
		Model:       c.model,
		Messages:    buildMessages(messages),
		Temperature: float32(c.temperature),
		MaxTokens:   c.maxTokens,
	}

	// Add tools if present
	if len(tools) > 0 {
		reqBody.Tools = buildTools(tools)
	}

	// Enable thinking mode for supported models
	if isThinkingModel(c.model) {
		reqBody.Thinking = &thinkingConfig{Type: "enabled"}
		reqBody.ReasoningEffort = "high"
		// Thinking mode doesn't support temperature
		reqBody.Temperature = 0
	}

	// Serialize request
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal request: %w", err)
	}

	// Create HTTP request
	apiURL := c.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("cannot create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("chat request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read response: %w", err)
	}

	// Parse response
	var chatResp chatResponseJSON
	if err := json.Unmarshal(respBytes, &chatResp); err != nil {
		return nil, fmt.Errorf("cannot parse response: %w", err)
	}

	// Check for API error
	if chatResp.Error != nil {
		return nil, &OpenAIError{
			StatusCode: resp.StatusCode,
			Message:    chatResp.Error.Message,
		}
	}

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	// Parse response content and tool calls
	content, reasoningContent, toolCalls := parseResponseChoices(chatResp.Choices)

	return &LLMResponse{
		Content:          content,
		ReasoningContent: reasoningContent,
		ToolCalls:        toolCalls,
	}, nil
}

func (c *openAIClient) ChatStream(ctx context.Context, messages []Message, tools []Tool) (<-chan StreamEvent, error) {
	// Build request body
	reqBody := chatRequestJSON{
		Model:       c.model,
		Messages:    buildMessages(messages),
		Temperature: float32(c.temperature),
		MaxTokens:   c.maxTokens,
		Stream:      true,
	}

	// Add tools if present
	if len(tools) > 0 {
		reqBody.Tools = buildTools(tools)
	}

	// Enable thinking mode for supported models
	if isThinkingModel(c.model) {
		reqBody.Thinking = &thinkingConfig{Type: "enabled"}
		reqBody.ReasoningEffort = "high"
		reqBody.Temperature = 0
	}

	// Serialize request
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal request: %w", err)
	}

	// Create HTTP request
	apiURL := c.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("cannot create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	// Use streamClient (no timeout) for streaming requests.
	// The httpClient has a 60s timeout which would cause streaming requests
	// to fail when the LLM takes a long time to respond (e.g., thinking mode,
	// large context processing). The streamClient relies on context.Context
	// for cancellation instead.
	resp, err := c.streamClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("chat stream request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	eventCh := make(chan StreamEvent, 100)

	go func() {
		defer close(eventCh)
		defer resp.Body.Close()

		reader := NewStreamReader(resp.Body)
		// Accumulate tool calls from stream deltas.
		// SSE tool_calls are sent in chunks: first chunk has ID+Name,
		// subsequent chunks have arguments fragments for the same index.
		accumulatedToolCalls := make(map[int]*ToolCall)
		finishReason := ""

		for {
			line, err := reader.Read()
			if err != nil {
				if err == io.EOF {
					// Send accumulated tool calls before done
					for _, tc := range accumulatedToolCalls {
						eventCh <- StreamEvent{
							Type:     StreamEventToolCall,
							ToolCall: tc,
						}
					}
					eventCh <- StreamEvent{
						Type:         StreamEventDone,
						FinishReason: finishReason,
						Done:         true,
					}
					return
				}
				eventCh <- StreamEvent{Type: StreamEventError, Err: err}
				return
			}

			// Parse the SSE line
			event, parseErr := parseSSELine(line)
			if parseErr != nil {
				continue
			}

			if event == nil {
				continue
			}

			if event.ReasoningContent != "" {
				eventCh <- StreamEvent{
					Type:    StreamEventReasoning,
					Content: event.ReasoningContent,
				}
			}

			if event.Content != "" {
				eventCh <- StreamEvent{
					Type:    StreamEventContent,
					Content: event.Content,
				}
			}

			if len(event.ToolCalls) > 0 {
				for _, tc := range event.ToolCalls {
					// Use index to identify which tool call this delta belongs to
					// If no index, treat as a complete tool call
					if tc.Index < 0 {
						// Complete tool call (non-streaming fallback)
						tcCopy := tc
						eventCh <- StreamEvent{
							Type:     StreamEventToolCall,
							ToolCall: &tcCopy,
						}
					} else {
						// Delta chunk - accumulate
						existing, exists := accumulatedToolCalls[tc.Index]
						if !exists {
							accumulatedToolCalls[tc.Index] = &ToolCall{
								ID:   tc.ID,
								Name: tc.Name,
							}
						} else {
							if tc.ID != "" {
								existing.ID = tc.ID
							}
							if tc.Name != "" {
								existing.Name = tc.Name
							}
							if tc.Arguments != "" {
								existing.Arguments += tc.Arguments
							}
						}
					}
				}
			}

			// Track finish_reason from the stream
			if event.FinishReason != "" {
				finishReason = event.FinishReason
			}
		}
	}()

	return eventCh, nil
}

// streamEventJSON represents a single SSE event from the streaming response.
type streamEventJSON struct {
	Content          string     `json:"content,omitempty"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	FinishReason     string     `json:"finish_reason,omitempty"`
}

// parseSSELine parses a single SSE line from the streaming response.
func parseSSELine(line []byte) (*streamEventJSON, error) {
	// Skip non-data lines
	if len(line) == 0 || line[0] != 'd' {
		return nil, nil
	}

	// Check for "data: " prefix
	if len(line) < 6 || string(line[:6]) != "data: " {
		return nil, nil
	}

	data := line[6:]

	// Skip [DONE] signal
	if string(data) == "[DONE]" {
		return nil, nil
	}

	// Parse JSON
	var chunk struct {
		Choices []struct {
			Delta struct {
				Content          string `json:"content,omitempty"`
				ReasoningContent string `json:"reasoning_content,omitempty"`
				ToolCalls        []struct {
					Index    *int   `json:"index,omitempty"`
					ID       string `json:"id,omitempty"`
					Type     string `json:"type,omitempty"`
					Function struct {
						Name      string `json:"name,omitempty"`
						Arguments string `json:"arguments,omitempty"`
					} `json:"function,omitempty"`
				} `json:"tool_calls,omitempty"`
			} `json:"delta"`
			FinishReason string `json:"finish_reason,omitempty"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(data, &chunk); err != nil {
		return nil, err
	}

	if len(chunk.Choices) == 0 {
		return nil, nil
	}

	choice := chunk.Choices[0]
	delta := choice.Delta
	event := &streamEventJSON{
		Content:          delta.Content,
		ReasoningContent: delta.ReasoningContent,
		FinishReason:     choice.FinishReason,
	}

	if delta.ToolCalls != nil {
		for _, tc := range delta.ToolCalls {
			tcIndex := -1
			if tc.Index != nil {
				tcIndex = *tc.Index
			}
			event.ToolCalls = append(event.ToolCalls, ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
				Index:     tcIndex,
			})
		}
	}

	return event, nil

}

// StreamReader reads SSE (Server-Sent Events) from a stream.
type StreamReader struct {
	reader    *bytes.Buffer
	rawReader io.Reader
}

// NewStreamReader creates a new StreamReader.
func NewStreamReader(r io.Reader) *StreamReader {
	return &StreamReader{
		reader:    &bytes.Buffer{},
		rawReader: r,
	}
}

// Read reads the next SSE line from the stream.
func (sr *StreamReader) Read() ([]byte, error) {
	for {
		line, err := sr.reader.ReadBytes('\n')
		if err == nil {
			// Remove trailing \r if present
			if len(line) > 1 && line[len(line)-2] == '\r' {
				line = append(line[:len(line)-2], '\n')
			}
			return line[:len(line)-1], nil
		}

		if err != io.EOF {
			return nil, err
		}

		// Need to read more data
		buf := make([]byte, 4096)
		n, readErr := sr.rawReader.Read(buf)
		if n > 0 {
			sr.reader.Write(buf[:n])
			continue
		}

		if readErr != nil {
			// Return any remaining data in buffer
			if sr.reader.Len() > 0 {
				line := sr.reader.Bytes()
				sr.reader.Reset()
				return line, nil
			}
			return nil, readErr
		}
	}
}

// ListModels retrieves the list of available models from the API.
// Uses the OpenAI-compatible GET /models endpoint.
func (c *openAIClient) ListModels(ctx context.Context) ([]string, error) {
	apiURL := c.baseURL + "/models"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("list models request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	var modelsResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read response: %w", err)
	}

	if err := json.Unmarshal(respBytes, &modelsResp); err != nil {
		return nil, fmt.Errorf("cannot parse response: %w", err)
	}

	models := make([]string, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		models = append(models, m.ID)
	}

	return models, nil
}

func (c *openAIClient) Close() error {
	c.httpClient.CloseIdleConnections()
	c.streamClient.CloseIdleConnections()
	return nil
}
