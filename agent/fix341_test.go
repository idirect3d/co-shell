package agent

import (
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
)

// collectInfoEvents collects EventInfo messages for assertion.
func collectInfoEvents(t *testing.T) (StreamCallback, *[]string) {
	t.Helper()
	var events []string
	cb := func(evt string, msg string) {
		if evt == EventInfo {
			events = append(events, msg)
		}
	}
	return cb, &events
}

func TestFIX341_EmitParseErrorRaw_SwitchOn(t *testing.T) {
	a := &Agent{cfg: &config.Config{LLM: config.LLMConfig{ShowParseErrorRaw: true}}}
	cb, events := collectInfoEvents(t)
	a.emitParseErrorRaw(cb, `{}`)
	if len(*events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(*events))
	}
	// The i18n template contains a %s placeholder; assert on the literal
	// prefix (before the placeholder) and the raw detail separately.
	label := strings.TrimSuffix(i18n.T(i18n.KeyXMLParseErrorRaw), "\n")
	label = strings.ReplaceAll(label, "%s", "")
	label = strings.TrimSpace(label)
	if !strings.Contains((*events)[0], label) {
		t.Errorf("want label %q, got %q", label, (*events)[0])
	}
	if !strings.Contains((*events)[0], "{}") {
		t.Errorf("want raw detail, got %q", (*events)[0])
	}
}

func TestFIX341_EmitParseErrorRaw_SwitchOff(t *testing.T) {
	a := &Agent{cfg: &config.Config{LLM: config.LLMConfig{ShowParseErrorRaw: false}}}
	cb, events := collectInfoEvents(t)
	a.emitParseErrorRaw(cb, `{"path":"foo"}`)
	if len(*events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(*events))
	}
}

func TestFIX341_EmitParseErrorRaw_RawEmpty(t *testing.T) {
	a := &Agent{cfg: &config.Config{LLM: config.LLMConfig{ShowParseErrorRaw: true}}}
	cb, events := collectInfoEvents(t)
	a.emitParseErrorRaw(cb, "")
	if len(*events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(*events))
	}
}

func TestFIX341_EmitParseErrorRaw_NilCfg(t *testing.T) {
	a := &Agent{}
	cb, events := collectInfoEvents(t)
	a.emitParseErrorRaw(cb, `{"path":"foo"}`)
	if len(*events) != 0 {
		t.Fatalf("expected 0 events with nil cfg, got %d", len(*events))
	}
}
