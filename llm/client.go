package llm

import (
	"context"
	"fmt"
	"io"

	"github.com/sashabaranov/go-openai"
)

// Message represents a chat message in the conversation.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ToolCall represents a function call requested by the LLM.
type ToolCall struct {
	ID       string
	Name     string
	Arguments string
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
	Content   string
	ToolCalls []ToolCall
}

// Client is the interface for LLM interactions.
type Client interface {
	// Chat sends a chat completion request and returns the response.
	Chat(ctx context.Context, messages []Message, tools []Tool) (*LLMResponse, error)

	// ChatStream sends a chat completion request with streaming response.
	ChatStream(ctx context.Context, messages []Message, tools []Tool) (<-chan StreamEvent, error)

	// Close cleans up any resources.
	Close() error
}

// StreamEvent represents an event in the streaming response.
type StreamEvent struct {
	Type    StreamEventType
	Content string
	Done    bool
	Err     error
}

// StreamEventType indicates the type of stream event.
type StreamEventType int

const (
	StreamEventContent StreamEventType = iota
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

// openAIClient implements Client using the OpenAI-compatible API.
type openAIClient struct {
	client *openai.Client
	model  string
	temperature float64
	maxTokens   int
}

// NewClient creates a new LLM client from configuration.
func NewClient(endpoint, apiKey, model string, temperature float64, maxTokens int) Client {
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = endpoint

	return &openAIClient{
		client:      openai.NewClientWithConfig(cfg),
		model:       model,
		temperature: temperature,
		maxTokens:   maxTokens,
	}
}

// toOpenAIMessages converts our Message type to OpenAI messages.
func toOpenAIMessages(messages []Message) []openai.ChatCompletionMessage {
	result := make([]openai.ChatCompletionMessage, len(messages))
	for i, msg := range messages {
		result[i] = openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}
	return result
}

// toOpenAITools converts our Tool type to OpenAI tools.
func toOpenAITools(tools []Tool) []openai.Tool {
	result := make([]openai.Tool, 0, len(tools))
	for _, t := range tools {
		params := t.Parameters
		if params == nil {
			params = map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}
		}
		result = append(result, openai.Tool{
			Type: "function",
			Function: &openai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
			},
		})
	}
	return result
}

// parseToolCalls extracts tool calls from the OpenAI response.
func parseToolCalls(choices []openai.ChatCompletionChoice) []ToolCall {
	var toolCalls []ToolCall
	for _, choice := range choices {
		if choice.Message.ToolCalls != nil {
			for _, tc := range choice.Message.ToolCalls {
				toolCalls = append(toolCalls, ToolCall{
					ID:        tc.ID,
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				})
			}
		}
	}
	return toolCalls
}

func (c *openAIClient) Chat(ctx context.Context, messages []Message, tools []Tool) (*LLMResponse, error) {
	req := openai.ChatCompletionRequest{
		Model:       c.model,
		Messages:    toOpenAIMessages(messages),
		Temperature: float32(c.temperature),
		MaxTokens:   c.maxTokens,
	}

	if len(tools) > 0 {
		req.Tools = toOpenAITools(tools)
	}

	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("chat completion failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := resp.Choices[0]
	return &LLMResponse{
		Content:   choice.Message.Content,
		ToolCalls: parseToolCalls(resp.Choices),
	}, nil
}

func (c *openAIClient) ChatStream(ctx context.Context, messages []Message, tools []Tool) (<-chan StreamEvent, error) {
	req := openai.ChatCompletionRequest{
		Model:       c.model,
		Messages:    toOpenAIMessages(messages),
		Temperature: float32(c.temperature),
		MaxTokens:   c.maxTokens,
		Stream:      true,
	}

	if len(tools) > 0 {
		req.Tools = toOpenAITools(tools)
	}

	stream, err := c.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("chat stream failed: %w", err)
	}

	eventCh := make(chan StreamEvent, 100)

	go func() {
		defer close(eventCh)
		defer stream.Close()

		for {
			response, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					eventCh <- StreamEvent{Type: StreamEventDone, Done: true}
					return
				}
				eventCh <- StreamEvent{Type: StreamEventError, Err: err}
				return
			}

			if len(response.Choices) == 0 {
				continue
			}

			delta := response.Choices[0].Delta
			if delta.Content != "" {
				eventCh <- StreamEvent{
					Type:    StreamEventContent,
					Content: delta.Content,
				}
			}

			if delta.ToolCalls != nil {
				for _, tc := range delta.ToolCalls {
					eventCh <- StreamEvent{
						Type: StreamEventToolCall,
						Content: tc.Function.Name,
					}
				}
			}
		}
	}()

	return eventCh, nil
}

func (c *openAIClient) Close() error {
	return nil
}
