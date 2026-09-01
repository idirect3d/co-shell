// Author: co-shell
// Created: 2026-09-01
//
// MIT License
//
// Copyright (c) 2026 co-shell
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

package agent

import (
	"testing"

	"github.com/idirect3d/co-shell/config"
)

// TestGetModelIDForCallVisionFallback verifies FIX-465: when vision is required
// and the current work mode has no VisionModelID binding, getModelIDForCall must
// NOT return the mode's ModelID if that model does not support vision. Instead it
// returns empty so selectModelForCall falls back to the global vision model.
func TestGetModelIDForCallVisionFallback(t *testing.T) {
	textModelID := "text-model"
	visionModelID := "vision-model"
	nonVisionModelID := "non-vision-model"

	cfg := &config.Config{
		LLM: config.LLMConfig{WorkMode: "act"},
		Models: []*config.ModelConfig{
			{ID: textModelID, Enabled: true, Capabilities: config.ModelCapability{Vision: false}},
			{ID: visionModelID, Enabled: true, Capabilities: config.ModelCapability{Vision: true}},
			{ID: nonVisionModelID, Enabled: true, Capabilities: config.ModelCapability{Vision: false}},
		},
		WorkModes: []config.WorkMode{
			{Name: "act", ModelID: &textModelID}, // no VisionModelID binding
		},
	}

	t.Run("vision required, mode ModelID lacks vision -> empty (fallback to global)", func(t *testing.T) {
		a := &Agent{cfg: cfg, imagePaths: []string{"/tmp/fake.png"}}
		if got := a.getModelIDForCall(); got != "" {
			t.Fatalf("getModelIDForCall() = %q, want empty (fallback to global vision model)", got)
		}
	})

	t.Run("vision required, mode binds VisionModelID -> returns it", func(t *testing.T) {
		cfg2 := *cfg
		cfg2.WorkModes = []config.WorkMode{
			{Name: "act", ModelID: &textModelID, VisionModelID: &visionModelID},
		}
		a := &Agent{cfg: &cfg2, imagePaths: []string{"/tmp/fake.png"}}
		if got := a.getModelIDForCall(); got != visionModelID {
			t.Fatalf("getModelIDForCall() = %q, want %q", got, visionModelID)
		}
	})

	t.Run("vision required, mode ModelID supports vision -> returns it", func(t *testing.T) {
		cfg2 := *cfg
		cfg2.WorkModes = []config.WorkMode{
			{Name: "act", ModelID: &visionModelID}, // ModelID itself supports vision
		}
		a := &Agent{cfg: &cfg2, imagePaths: []string{"/tmp/fake.png"}}
		if got := a.getModelIDForCall(); got != visionModelID {
			t.Fatalf("getModelIDForCall() = %q, want %q", got, visionModelID)
		}
	})

	t.Run("no vision, mode ModelID -> returns it", func(t *testing.T) {
		a := &Agent{cfg: cfg} // no imagePaths
		if got := a.getModelIDForCall(); got != textModelID {
			t.Fatalf("getModelIDForCall() = %q, want %q", got, textModelID)
		}
	})
}

// TestSelectModelForCallVisionFallback verifies FIX-465 end-to-end: when vision
// is required and the mode's ModelID lacks vision, selectModelForCall falls back
// to the global highest-priority vision model.
func TestSelectModelForCallVisionFallback(t *testing.T) {
	textModelID := "text-model"
	globalVisionID := "global-vision-model"

	cfg := &config.Config{
		LLM: config.LLMConfig{WorkMode: "act"},
		Models: []*config.ModelConfig{
			{ID: textModelID, Enabled: true, Priority: 30, Capabilities: config.ModelCapability{Vision: false}},
			{ID: globalVisionID, Enabled: true, Priority: 20, Capabilities: config.ModelCapability{Vision: true}},
		},
		WorkModes: []config.WorkMode{
			{Name: "act", ModelID: &textModelID}, // no VisionModelID binding
		},
	}

	mm := &config.ModelManager{}
	for _, m := range cfg.Models {
		if err := mm.AddModel(m); err != nil {
			t.Fatalf("AddModel(%s): %v", m.ID, err)
		}
	}

	a := &Agent{cfg: cfg, modelManager: mm, imagePaths: []string{"/tmp/fake.png"}}
	selected := a.selectModelForCall()
	if selected == nil {
		t.Fatal("selectModelForCall() returned nil, want global vision model")
	}
	if selected.ID != globalVisionID {
		t.Fatalf("selectModelForCall() = %q, want global vision model %q", selected.ID, globalVisionID)
	}
}
