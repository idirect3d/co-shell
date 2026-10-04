// Package agent - FEATURE-602: degrade to the next lower-priority model when an
// LLM call fails for a connection, service or model-internal reason.
//
// Author: L.Shuang
// Created: 2026-10-04
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/log"
)

// resetModelFallback clears the per-turn degradation state. It is called at the
// start of every RunStream so a new user turn starts from the normal model
// selection (mode binding / highest priority).
func (a *Agent) resetModelFallback() {
	a.mu.Lock()
	a.fallbackModelID = ""
	a.fallbackTried = nil
	a.mu.Unlock()
}

// autoSelectEnabled reports whether the user turned the automatic selection
// (degrade-on-failure) switch on (FEATURE-602). It is off by default.
func (a *Agent) autoSelectEnabled() bool {
	if a.cfg == nil {
		return false
	}
	return a.cfg.LLM.AutoSelectEnabled()
}

// fallbackCandidates returns the models in descending priority order — the same
// order the Web UI model list shows. The model manager owns that order; when no
// manager is wired (tests, headless) the config slice is sorted here.
func (a *Agent) fallbackCandidates() []*config.ModelConfig {
	if a.modelManager != nil {
		return a.modelManager.GetAllModels()
	}
	if a.cfg == nil {
		return nil
	}
	out := make([]*config.ModelConfig, len(a.cfg.Models))
	copy(out, a.cfg.Models)
	// Insertion sort by priority, descending (the list is tiny).
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Priority > out[j-1].Priority; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// markFallbackTried records that the model id has already been attempted in
// this turn.
func (a *Agent) markFallbackTried(id string) {
	if id == "" {
		return
	}
	a.mu.Lock()
	if a.fallbackTried == nil {
		a.fallbackTried = make(map[string]bool)
	}
	a.fallbackTried[id] = true
	a.mu.Unlock()
}

// isFallbackTried reports whether the model id was already attempted in this
// turn.
func (a *Agent) isFallbackTried(id string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.fallbackTried[id]
}

// nextFallbackModel returns the next enabled model to try after the current one
// failed (FEATURE-602). Only models ranked *after* the current one in the
// priority-descending list qualify (the strategy degrades downwards, it never
// jumps back up to a higher-priority model), models already attempted in this
// turn are skipped, and in a vision turn only vision-capable models count.
// nil means every eligible model has been tried.
func (a *Agent) nextFallbackModel() *config.ModelConfig {
	if a.cfg == nil && a.modelManager == nil {
		return nil
	}
	visionRequired := len(a.imagePaths) > 0
	cur := a.selectModelForCall()
	if cur != nil {
		a.markFallbackTried(cur.ID)
	}
	list := a.fallbackCandidates()

	// Start right after the current model so only lower-ranked models are
	// considered.
	start := 0
	if cur != nil {
		for i, m := range list {
			if m != nil && m.ID == cur.ID {
				start = i + 1
				break
			}
		}
	}
	for _, m := range list[start:] {
		if m == nil || !m.Enabled || m.ID == "" {
			continue
		}
		if visionRequired && !m.Capabilities.Vision {
			continue
		}
		if a.isFallbackTried(m.ID) {
			continue
		}
		return m
	}
	return nil
}

// applyFallbackModel pins the turn to the given model and records it as tried,
// so later iterations keep using it and never fail back to a higher-priority
// model that already failed.
func (a *Agent) applyFallbackModel(m *config.ModelConfig) {
	if m == nil {
		return
	}
	a.mu.Lock()
	a.fallbackModelID = m.ID
	if a.fallbackTried == nil {
		a.fallbackTried = make(map[string]bool)
	}
	a.fallbackTried[m.ID] = true
	a.mu.Unlock()
	log.Info("Agent: FEATURE-602 degraded to model %q", m.ID)
}
