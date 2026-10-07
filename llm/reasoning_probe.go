package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
)

// Capability probing for OpenAI-compatible endpoints (FEATURE-620).
//
// A self-hosted engine (vLLM, LMStudio, Ollama, ...) presents one uniform API
// shell while the semantics vary per model and per engine version: which field
// carries thinking, whether thinking is on by default, and how tool calls come
// back. The wizard uses ProbeReasoning to answer these questions instead of
// asking the user to know them.

// ProbeLevel controls how much work a probe does.
type ProbeLevel string

const (
	// ProbeOff probes nothing: every value stays empty and the user fills the
	// form manually.
	ProbeOff ProbeLevel = "off"
	// ProbeLight sends a single streamed request, which reveals both the
	// response field name and whether thinking is on by default. This is the
	// default because it suits slow or concurrency-limited endpoints.
	ProbeLight ProbeLevel = "light"
	// ProbeFull additionally walks the request-side thinking protocols, looks
	// for inline chain-of-thought and probes the tool-call style.
	ProbeFull ProbeLevel = "full"
)

// Detected values reported by a probe.
const (
	// reasoningDefaultOn/Off describe whether the engine returns thinking
	// without any request-side thinking parameter.
	reasoningDefaultOn  = "on"
	reasoningDefaultOff = "off"
	// reasoningDefaultUnknown means the probe could not tell.
	reasoningDefaultUnknown = "unknown"

	// ToolCallStyle* describe how a model returns tool calls.
	ToolCallStyleNative = "native"
	ToolCallStyleText   = "text"
	ToolCallStyleNone   = "none"

	// EngineVLLM is reported when the endpoint identifies itself as vLLM.
	EngineVLLM = "vllm"
)

// probeItemTimeout bounds a single probe item so that a slow endpoint can
// never block the wizard. It is a variable so tests can shorten it.
var probeItemTimeout = 15 * time.Second

const (
	// probeChunkLimit is how many stream deltas the field probe inspects
	// before cancelling the request.
	probeChunkLimit = 12
	// probeMaxTokens keeps probe answers short.
	probeMaxTokens = 96
	// probeToolName is the no-op tool used to detect native tool calling.
	probeToolName = "probe_noop"
)

// thinkingProtocolProbeOrder is the fixed order in which the request-side
// protocols are tried when thinking is off by default. The order is part of the
// contract (tests assert it) so that a probe never sends a wild amount of
// requests and behaves deterministically.
var thinkingProtocolProbeOrder = []string{
	"deepseek", "qwen", "zhipu", "minimax", "kimi", "openai-compatible",
}

// ThinkingProtocolCandidates returns the request-side thinking protocol keys a
// model configuration may select, in canonical order. The first entry is the
// empty "auto" value meaning "derive the protocol from the provider".
func ThinkingProtocolCandidates() []string {
	return append([]string{""}, thinkingProtocolProbeOrder...)
}

// inlineTagCandidates are the tag names checked for inline chain-of-thought.
var inlineTagCandidates = []string{"thinking", "reasoning", "analysis", "think"}

// probeHTTPClient performs probe requests. It has no overall timeout: every
// request carries its own context deadline instead.
var probeHTTPClient = &http.Client{}

// DetectedProfile is the (read-only) report produced by ProbeReasoning. An
// empty value means "not detected"; nothing here is ever persisted unless the
// user accepts it.
type DetectedProfile struct {
	// ReasoningField is the response field that carries thinking:
	// ReasoningFieldContent, ReasoningFieldReasoning, ReasoningFieldInline or
	// ReasoningFieldNone.
	ReasoningField string `json:"reasoning_field,omitempty"`
	// ReasoningDefault is "on", "off" or "unknown": whether thinking arrives
	// without any request-side thinking parameter.
	ReasoningDefault string `json:"reasoning_default,omitempty"`
	// ThinkingProtocol is the request-side protocol that turned thinking on,
	// as a thinking adapter registry key. Empty when thinking is on by default
	// or when no protocol worked.
	ThinkingProtocol string `json:"thinking_protocol,omitempty"`
	// ToolCallStyle is "native", "text", "none" or empty (not probed).
	ToolCallStyle string `json:"tool_call_style,omitempty"`
	// ReasoningTags lists inline chain-of-thought tags found in content.
	ReasoningTags []string `json:"reasoning_tags,omitempty"`
	// Engine is the recognised inference engine ("vllm") or empty.
	Engine string `json:"engine,omitempty"`
}

// ProbeReasoning inspects an endpoint and reports how it exposes thinking and
// tool calls (FEATURE-620).
//
// It never returns nil and never fails as a whole: every item is bounded by
// probeItemTimeout and a failed item simply stays empty, so a broken or slow
// endpoint cannot block the wizard.
func ProbeReasoning(ctx context.Context, endpoint, apiKey, model string, level ProbeLevel) *DetectedProfile {
	profile := &DetectedProfile{ReasoningDefault: reasoningDefaultUnknown}
	if level == "" {
		level = ProbeLight
	}
	if level == ProbeOff {
		return profile
	}

	// Probe requests must stay traceable after the fact.
	wasOn := log.IsLLMInteractionEnabled()
	log.SetLLMInteractionEnabled(true)
	defer log.SetLLMInteractionEnabled(wasOn)

	// One request with no thinking parameters answers two questions at once:
	// whether thinking is on by default, and which field carries it.
	field, sawThinking, engine := probeStream(ctx, endpoint, apiKey, model, nil)
	profile.Engine = engine
	if sawThinking {
		profile.ReasoningField = field
		profile.ReasoningDefault = reasoningDefaultOn
	} else {
		profile.ReasoningDefault = reasoningDefaultOff
		profile.ReasoningField = ReasoningFieldNone
	}

	// Full probes continue from there.
	if level == ProbeFull && !sawThinking {
		// Thinking is off by default: find the request-side protocol that turns
		// it on. This is what keeps the probe useful on an engine whose
		// provider name says nothing about the model it serves.
		if f, protocol := probeThinkingProtocol(ctx, endpoint, apiKey, model); protocol != "" {
			profile.ReasoningField = f
			profile.ThinkingProtocol = protocol
		}
	}
	if level == ProbeFull && profile.ReasoningField == ReasoningFieldNone {
		// Inline chain-of-thought is reported as a hint only: stripping runs the
		// risk of cutting legitimate text out of an answer, so the wizard never
		// enables it on the user's behalf.
		profile.ReasoningTags = probeInlineTags(ctx, endpoint, apiKey, model)
	}
	if level == ProbeFull {
		profile.ToolCallStyle = probeToolCallStyle(ctx, endpoint, apiKey, model)
	}

	return profile
}

// probeStreamChunk is one streamed delta as the probe sees it. Unlike the
// client's streamEventJSON it keeps "reasoning" and "reasoning_content" apart,
// because telling them apart is exactly what the probe is for.
type probeStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content          string            `json:"content"`
			ReasoningContent string            `json:"reasoning_content"`
			Reasoning        string            `json:"reasoning"`
			ToolCalls        []json.RawMessage `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
	SystemFingerprint string `json:"system_fingerprint"`
}

// probeStream sends one streamed request with the given (optional) body
// additions and reports which reasoning field appeared, whether thinking was
// returned at all, and the engine recognised from system_fingerprint.
//
// It reads at most probeChunkLimit deltas and stops as soon as thinking shows
// up, so a chatty endpoint cannot make the probe slow.
func probeStream(ctx context.Context, endpoint, apiKey, model string, additions map[string]string) (field string, sawThinking bool, engine string) {
	reqCtx, cancel := context.WithTimeout(ctx, probeItemTimeout)
	defer cancel()

	body := map[string]interface{}{
		"model":      model,
		"stream":     true,
		"max_tokens": probeMaxTokens,
		"messages": []map[string]string{
			{"role": "user", "content": i18n.T(i18n.KeyLLMProbeReasoningPrompt)},
		},
	}
	// Adapter additions are pre-serialized JSON values; inline them as JSON so
	// nested objects (a "thinking" object, chat_template_kwargs) survive.
	for k, v := range additions {
		var parsed interface{}
		if err := json.Unmarshal([]byte(v), &parsed); err != nil {
			log.Warn("FEATURE-620 probe: skipping non-JSON body addition %s: %v", k, err)
			continue
		}
		body[k] = parsed
	}

	resp, err := probePost(reqCtx, endpoint, apiKey, body)
	if err != nil {
		log.Warn("FEATURE-620 probe: streaming request failed: %v", err)
		return "", false, ""
	}
	defer resp.Body.Close()
	defer log.WriteLLMInteractionEnd()

	sawContent, sawReasoning := false, false
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	chunks := 0
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk probeStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.SystemFingerprint != "" {
			engine = engineFromFingerprint(chunk.SystemFingerprint)
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta
		if delta.ReasoningContent != "" {
			sawContent = true
		}
		if delta.Reasoning != "" {
			sawReasoning = true
		}
		chunks++
		if sawContent || sawReasoning || chunks >= probeChunkLimit {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		log.Debug("FEATURE-620 probe: stream read stopped: %v", err)
	}

	switch {
	case sawContent:
		return ReasoningFieldContent, true, engine
	case sawReasoning:
		return ReasoningFieldReasoning, true, engine
	default:
		return "", false, engine
	}
}

// engineFromFingerprint maps a system_fingerprint to a known engine name.
func engineFromFingerprint(fingerprint string) string {
	if strings.HasPrefix(fingerprint, "vllm-") {
		return EngineVLLM
	}
	return ""
}

// probePost performs one POST of a JSON body and returns the response. The
// body is sent verbatim so callers control the exact request shape.
func probePost(ctx context.Context, endpoint, apiKey string, body interface{}) (*http.Response, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(endpoint, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := probeHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	// Probe traffic must stay traceable for later diagnosis.
	log.WriteLLMInteraction("PROBE][request", string(payload))
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		resp.Body.Close()
		log.Warn("FEATURE-620 probe: POST %s status=%d body=%s", url, resp.StatusCode, strings.TrimSpace(string(raw)))
		return nil, &probeStatusError{url: url, status: resp.StatusCode, body: strings.TrimSpace(string(raw))}
	}
	return resp, nil
}

// probeStatusError reports a non-200 probe response.
type probeStatusError struct {
	url    string
	status int
	body   string
}

func (e *probeStatusError) Error() string {
	return "probe request to " + e.url + " failed with status " + strconv.Itoa(e.status) + ": " + e.body
}

// probeThinkingProtocol walks the known request-side protocols in a fixed order
// and returns the first one that makes the engine return thinking, together
// with the response field that appeared.
//
// Without this, a self-hosted engine whose provider name is a generic
// "openai-compatible" would be probed with the wrong request-side parameters
// and the model would look as if it could not think at all.
func probeThinkingProtocol(ctx context.Context, endpoint, apiKey, model string) (field, protocol string) {
	for _, candidate := range thinkingProtocolProbeOrder {
		additions := GetThinkingAdapter(candidate).BuildAdditions(ThinkingConfig{
			Mode: ThinkingModeEnabled,
		})
		if len(additions) == 0 {
			continue
		}
		field, sawThinking, _ := probeStream(ctx, endpoint, apiKey, model, additions)
		if sawThinking {
			return field, candidate
		}
	}
	return "", ""
}

// probeInlineTags looks for chain-of-thought tags wrapped inside content, which
// is how a model without a dedicated reasoning field exposes its thinking.
func probeInlineTags(ctx context.Context, endpoint, apiKey, model string) []string {
	body := map[string]interface{}{
		"model":      model,
		"max_tokens": probeMaxTokens,
		"messages": []map[string]string{
			{"role": "user", "content": i18n.T(i18n.KeyLLMProbeReasoningPrompt)},
		},
	}
	content, _, err := probeNonStreaming(ctx, endpoint, apiKey, body)
	if err != nil {
		log.Warn("FEATURE-620 probe: inline chain-of-thought probe failed: %v", err)
		return nil
	}

	var tags []string
	for _, tag := range inlineTagCandidates {
		if strings.Contains(content, "<"+tag+">") && strings.Contains(content, "</"+tag+">") {
			tags = append(tags, tag)
		}
	}
	return tags
}

// probeToolCallStyle reports how the model returns tool calls: natively via
// tool_calls, as text that merely mentions the tool, or not at all.
func probeToolCallStyle(ctx context.Context, endpoint, apiKey, model string) string {
	body := map[string]interface{}{
		"model":      model,
		"max_tokens": probeMaxTokens,
		"messages": []map[string]string{
			{"role": "user", "content": i18n.T(i18n.KeyLLMProbeToolPrompt)},
		},
		"tools": []map[string]interface{}{
			{
				"type": "function",
				"function": map[string]interface{}{
					"name":        probeToolName,
					"description": i18n.T(i18n.KeyLLMProbeToolDescription),
					"parameters": map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{},
					},
				},
			},
		},
	}

	content, toolCalls, err := probeNonStreaming(ctx, endpoint, apiKey, body)
	if err != nil {
		log.Warn("FEATURE-620 probe: tool-call probe failed: %v", err)
		return ""
	}
	if toolCalls > 0 {
		return ToolCallStyleNative
	}
	if strings.Contains(content, probeToolName) {
		return ToolCallStyleText
	}
	return ToolCallStyleNone
}

// probeNonStreaming sends one non-streaming request and returns the assistant
// message content plus how many tool calls it carried.
func probeNonStreaming(ctx context.Context, endpoint, apiKey string, body map[string]interface{}) (string, int, error) {
	reqCtx, cancel := context.WithTimeout(ctx, probeItemTimeout)
	defer cancel()

	resp, err := probePost(reqCtx, endpoint, apiKey, body)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", 0, err
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content   string            `json:"content"`
				ToolCalls []json.RawMessage `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", 0, err
	}
	if len(parsed.Choices) == 0 {
		return "", 0, nil
	}
	return parsed.Choices[0].Message.Content, len(parsed.Choices[0].Message.ToolCalls), nil
}
