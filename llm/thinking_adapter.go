package llm

import (
	"encoding/json"
	"fmt"
)

type ThinkingMode string

const (
	ThinkingModeEnabled  ThinkingMode = "enabled"
	ThinkingModeDisabled ThinkingMode = "disabled"
	ThinkingModeDefault  ThinkingMode = ""
)

type ThinkingConfig struct {
	Mode            ThinkingMode
	ReasoningEffort string
}

type ThinkingAdapter interface {
	BuildAdditions(cfg ThinkingConfig) map[string]string
}

var thinkingAdapterRegistry = map[string]ThinkingAdapter{
	"deepseek":          &deepseekThinkingAdapter{},
	"qwen":              &qwenThinkingAdapter{},
	"zhipu":             &zhipuThinkingAdapter{},
	"minimax":           &minimaxThinkingAdapter{},
	"moonshot":          &moonshotThinkingAdapter{},
	"kimi":              &kimiThinkingAdapter{},
	"openai":            &openaiThinkingAdapter{},
	"openai-compatible": &fallbackThinkingAdapter{},
}

func GetThinkingAdapter(provider string) ThinkingAdapter {
	if a, ok := thinkingAdapterRegistry[provider]; ok {
		return a
	}
	return &fallbackThinkingAdapter{}
}

func ThinkingModeFromBool(enabled bool) ThinkingMode {
	if enabled {
		return ThinkingModeEnabled
	}
	return ThinkingModeDisabled
}

// ThinkingModeFromString converts config string values to ThinkingMode.
// "on" → Enabled, "off" → Disabled, anything else → Default (no params sent).
func ThinkingModeFromString(s string) ThinkingMode {
	switch s {
	case "on":
		return ThinkingModeEnabled
	case "off":
		return ThinkingModeDisabled
	default:
		return ThinkingModeDefault
	}
}

type deepseekThinkingAdapter struct{}

func (a *deepseekThinkingAdapter) BuildAdditions(cfg ThinkingConfig) map[string]string {
	switch cfg.Mode {
	case ThinkingModeDefault:
		return nil
	case ThinkingModeDisabled:
		return map[string]string{"thinking": `{"type":"disabled"}`}
	case ThinkingModeEnabled:
		r := map[string]string{"thinking": `{"type":"enabled"}`}
		if cfg.ReasoningEffort != "" {
			r["reasoning_effort"] = fmt.Sprintf(`"%s"`, cfg.ReasoningEffort)
		} else {
			r["reasoning_effort"] = `"high"`
		}
		return r
	}
	return nil
}

type qwenThinkingAdapter struct{}

func (a *qwenThinkingAdapter) BuildAdditions(cfg ThinkingConfig) map[string]string {
	switch cfg.Mode {
	case ThinkingModeDefault:
		return nil
	case ThinkingModeDisabled:
		return map[string]string{"extra_body": `{"chat_template_kwargs":{"enable_thinking":false}}`}
	case ThinkingModeEnabled:
		return map[string]string{"extra_body": `{"chat_template_kwargs":{"enable_thinking":true}}`}
	}
	return nil
}

type zhipuThinkingAdapter struct{}

func (a *zhipuThinkingAdapter) BuildAdditions(cfg ThinkingConfig) map[string]string {
	switch cfg.Mode {
	case ThinkingModeDefault:
		return nil
	case ThinkingModeDisabled:
		return map[string]string{"thinking": `{"type":"disabled"}`}
	case ThinkingModeEnabled:
		effort := cfg.ReasoningEffort
		if effort != "" {
			switch effort {
			case "low", "medium":
				effort = "high"
			case "xhigh":
				effort = "max"
			}
		} else {
			effort = "max"
		}
		return map[string]string{
			"thinking":         `{"type":"enabled"}`,
			"reasoning_effort": fmt.Sprintf(`"%s"`, effort),
		}
	}
	return nil
}

type minimaxThinkingAdapter struct{}

func (a *minimaxThinkingAdapter) BuildAdditions(cfg ThinkingConfig) map[string]string {
	switch cfg.Mode {
	case ThinkingModeDefault:
		return nil
	case ThinkingModeDisabled:
		return map[string]string{"extra_body": `{"reasoning_split":false}`}
	case ThinkingModeEnabled:
		return map[string]string{"extra_body": `{"reasoning_split":true}`}
	}
	return nil
}

type moonshotThinkingAdapter struct{}

func (a *moonshotThinkingAdapter) BuildAdditions(cfg ThinkingConfig) map[string]string {
	return kimiBuildAdditions(cfg)
}

type kimiThinkingAdapter struct{}

func (a *kimiThinkingAdapter) BuildAdditions(cfg ThinkingConfig) map[string]string {
	return kimiBuildAdditions(cfg)
}

// kimiBuildAdditions builds thinking additions for Kimi/Moonshot API.
// Kimi K3 always uses reasoning, K2.x supports configurable thinking mode.
func kimiBuildAdditions(cfg ThinkingConfig) map[string]string {
	switch cfg.Mode {
	case ThinkingModeDefault:
		return nil
	case ThinkingModeDisabled:
		return map[string]string{"thinking": `{"type":"disabled"}`}
	case ThinkingModeEnabled:
		r := map[string]string{
			"thinking": `{"type":"enabled","keep":"all"}`,
		}
		if cfg.ReasoningEffort != "" {
			r["reasoning_effort"] = fmt.Sprintf(`"%s"`, cfg.ReasoningEffort)
		} else {
			r["reasoning_effort"] = `"max"`
		}
		return r
	}
	return nil
}

type openaiThinkingAdapter struct{}

func (a *openaiThinkingAdapter) BuildAdditions(cfg ThinkingConfig) map[string]string {
	switch cfg.Mode {
	case ThinkingModeDefault:
		return nil
	case ThinkingModeDisabled:
		return map[string]string{"reasoning_effort": `"none"`}
	case ThinkingModeEnabled:
		effort := cfg.ReasoningEffort
		if effort == "" {
			effort = "medium"
		}
		return map[string]string{"reasoning_effort": fmt.Sprintf(`"%s"`, effort)}
	}
	return nil
}

type fallbackThinkingAdapter struct{}

func (a *fallbackThinkingAdapter) BuildAdditions(cfg ThinkingConfig) map[string]string {
	switch cfg.Mode {
	case ThinkingModeDefault:
		return nil
	case ThinkingModeDisabled:
		return map[string]string{"thinking": `{"type":"disabled"}`}
	case ThinkingModeEnabled:
		r := map[string]string{"thinking": `{"type":"enabled"}`}
		if cfg.ReasoningEffort != "" {
			r["reasoning_effort"] = fmt.Sprintf(`"%s"`, cfg.ReasoningEffort)
		}
		return r
	}
	return nil
}

var _ = json.Marshal
