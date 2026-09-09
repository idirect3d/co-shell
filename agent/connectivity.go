// Model connectivity pre-check (FEATURE-496): before sending a request to the
// LLM, optionally verify that the active model is reachable and available at
// its endpoint by calling the /models API. When the model is found unavailable,
// the task is aborted and the user is informed instead of waiting for a doomed
// request to time out.
//
// Author: L.Shuang
// Created: 2026-09-09
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"fmt"

	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
)

// checkModelConnectivity verifies that the model which would be used for the
// next LLM call is available. It consults cfg.LLM.ModelConnectivityCheck:
//   - "off" or empty: no check (returns nil).
//   - "on_submit" / "on_send": check the active model.
//
// It returns an error describing the unavailable model when the check fails,
// or nil when the check passes or is not required. When no model config can be
// resolved, the check is skipped (returns nil) to avoid blocking.
func (a *Agent) checkModelConnectivity() error {
	if a.cfg == nil {
		return nil
	}
	mode := a.cfg.LLM.ModelConnectivityCheck
	if mode == "" || mode == "off" {
		return nil
	}
	modelCfg := a.selectModelForCall()
	if modelCfg == nil {
		return nil
	}
	ok, err := llm.CheckModelAvailable(modelCfg.Endpoint, modelCfg.APIKey, modelCfg.Model, 15)
	if ok {
		return nil
	}
	reason := "unavailable"
	if err != nil {
		reason = err.Error()
	}
	log.Warn("Agent.checkModelConnectivity: model %q (%s) %s", modelCfg.ID, modelCfg.Model, reason)
	return fmt.Errorf("model %q (%s) is not available: %s", modelCfg.ID, modelCfg.Model, reason)
}
