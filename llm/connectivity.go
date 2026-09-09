// Model connectivity check (FEATURE-496): verifies that a configured model is
// actually available at its endpoint by calling the /models API and checking
// whether the target model name appears in the returned list.
//
// Author: L.Shuang
// Created: 2026-09-09
// MIT License - Copyright (c) 2026 L.Shuang

package llm

import (
	"context"
	"fmt"
	"time"
)

// CheckModelAvailable verifies that the given model is available at the
// endpoint by calling GET /models and checking whether model appears in the
// returned list. It returns true when the model is present, false when the
// endpoint is unreachable, the API key is invalid, or the model is not in the
// list. The returned error carries a human-readable reason (empty on success).
func CheckModelAvailable(endpoint, apiKey, model string, timeoutSeconds int) (bool, error) {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 15
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	client := NewClient(endpoint, apiKey, model, 0, 0, timeoutSeconds)
	models, err := client.ListModels(ctx)
	if err != nil {
		return false, fmt.Errorf("cannot reach model endpoint: %w", err)
	}
	for _, m := range models {
		if m.ID == model {
			return true, nil
		}
	}
	return false, fmt.Errorf("model %q not found in the endpoint's model list", model)
}
