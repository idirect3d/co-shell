// Package cmd - model wizard cancel/exit tests (FEATURE-422). Verifies that
// the wizard can exit from any step via Q/QUIT and that a failing ReadLine
// (e.g. the Web client cancelled) maps to wizardCancel so the wizard does not
// spin forever on a closed input.
//
// Author: L.Shuang
// Created: 2026-08-23
// MIT License - Copyright (c) 2026 L.Shuang

package cmd

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/store"
	"github.com/idirect3d/co-shell/workspace"
)

// wizardTestIO is a scripted UserIO for wizard tests. It queues line inputs
// and can be told to fail ReadLine (simulating a cancelled Web client).
type wizardTestIO struct {
	inputs    []string
	idx       int
	failRead  bool
	out       strings.Builder
}

func (w *wizardTestIO) Print(args ...interface{})            { w.out.WriteString(strings.TrimSpace(fmtSprintAll(args...)) + " ") }
func (w *wizardTestIO) Printf(f string, args ...interface{}) { w.out.WriteString(strings.TrimSpace(fmtSprintfAll(f, args...)) + " ") }
func (w *wizardTestIO) Println(args ...interface{})          { w.out.WriteString(strings.TrimSpace(fmtSprintAll(args...)) + "\n") }
func (w *wizardTestIO) ErrPrintf(f string, args ...interface{}) {
	w.out.WriteString(strings.TrimSpace(fmtSprintfAll(f, args...)) + "\n")
}
func (w *wizardTestIO) ReadLine() (string, error) {
	if w.failRead {
		return "", errors.New("cancelled")
	}
	if w.idx >= len(w.inputs) {
		return "", nil
	}
	v := w.inputs[w.idx]
	w.idx++
	return v, nil
}
func (w *wizardTestIO) ReadKey() (byte, error) { return 0, nil }
func (w *wizardTestIO) IsReading() bool        { return false }

func fmtSprintAll(args ...interface{}) string {
	var sb strings.Builder
	for _, a := range args {
		sb.WriteString(a.(string))
	}
	return sb.String()
}

func fmtSprintfAll(f string, args ...interface{}) string {
	return fmt.Sprintf(f, args...)
}

// newWizardHandler builds a ModelHandler with the given scripted IO installed
// on a fresh agent backed by a temp bbolt store.
func newWizardHandler(t *testing.T, io agent.UserIO) *ModelHandler {
	t.Helper()
	ws, err := workspace.New(t.TempDir())
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	boltStore, err := store.NewStore(ws)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = boltStore.Close() })
	ag := agent.New(nil, nil, store.NewDualStore(boltStore, nil), "")
	ag.SetIO(io)
	return NewModelHandler(&config.Config{}, ag)
}

// TestWizardPromptBoolCancel verifies wizardPromptBool returns cancelled=true
// when the user types Q (FEATURE-422).
func TestWizardPromptBoolCancel(t *testing.T) {
	h := newWizardHandler(t, &wizardTestIO{inputs: []string{"q"}})
	val, cancelled := h.wizardPromptBool("enable?", true)
	if !cancelled {
		t.Errorf("wizardPromptBool(Q) cancelled = %v, want true", cancelled)
	}
	if val {
		t.Errorf("wizardPromptBool(Q) val = %v, want false", val)
	}
}

// TestWizardPromptBoolReadLineError verifies wizardPromptBool returns
// cancelled=true when ReadLine fails (Web client cancelled) (FEATURE-422).
func TestWizardPromptBoolReadLineError(t *testing.T) {
	h := newWizardHandler(t, &wizardTestIO{failRead: true})
	_, cancelled := h.wizardPromptBool("enable?", true)
	if !cancelled {
		t.Errorf("wizardPromptBool(failRead) cancelled = %v, want true", cancelled)
	}
}

// TestWizardPromptStringWithDefaultCancel verifies wizardPromptStringWithDefault
// returns wizardCancel when the user types Q (FEATURE-422).
func TestWizardPromptStringWithDefaultCancel(t *testing.T) {
	h := newWizardHandler(t, &wizardTestIO{inputs: []string{"Q"}})
	got := h.wizardPromptStringWithDefault("endpoint?", "https://x", "q")
	if got != wizardCancel {
		t.Errorf("wizardPromptStringWithDefault(Q) = %q, want %q", got, wizardCancel)
	}
}

// TestWizardPromptStringWithDefaultReadLineError verifies
// wizardPromptStringWithDefault returns wizardCancel when ReadLine fails
// (FEATURE-422).
func TestWizardPromptStringWithDefaultReadLineError(t *testing.T) {
	h := newWizardHandler(t, &wizardTestIO{failRead: true})
	got := h.wizardPromptStringWithDefault("endpoint?", "https://x", "q")
	if got != wizardCancel {
		t.Errorf("wizardPromptStringWithDefault(failRead) = %q, want %q", got, wizardCancel)
	}
}

// TestWizardPromptSecretCancel verifies wizardPromptSecret returns wizardCancel
// when the user types Q (FEATURE-422).
func TestWizardPromptSecretCancel(t *testing.T) {
	h := newWizardHandler(t, &wizardTestIO{inputs: []string{"Q"}})
	got := h.wizardPromptSecret("api key?", "")
	if got != wizardCancel {
		t.Errorf("wizardPromptSecret(Q) = %q, want %q", got, wizardCancel)
	}
}

// TestWizardSelectCapabilitiesCancel verifies wizardSelectCapabilities returns
// wizardCancel when the user types Q (FEATURE-422).
func TestWizardSelectCapabilitiesCancel(t *testing.T) {
	h := newWizardHandler(t, &wizardTestIO{inputs: []string{"Q"}})
	_, nav := h.wizardSelectCapabilities(config.ModelCapability{})
	if nav != wizardCancel {
		t.Errorf("wizardSelectCapabilities(Q) nav = %q, want %q", nav, wizardCancel)
	}
}

// TestWizardSelectCapabilitiesReadLineError verifies wizardSelectCapabilities
// returns wizardCancel when ReadLine fails (FEATURE-422).
func TestWizardSelectCapabilitiesReadLineError(t *testing.T) {
	h := newWizardHandler(t, &wizardTestIO{failRead: true})
	_, nav := h.wizardSelectCapabilities(config.ModelCapability{})
	if nav != wizardCancel {
		t.Errorf("wizardSelectCapabilities(failRead) nav = %q, want %q", nav, wizardCancel)
	}
}
