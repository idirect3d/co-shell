package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestFix592DefaultValues covers UC-0001 case A: the FIX-592 default baseline.
func TestFix592DefaultValues(t *testing.T) {
	def := DefaultConfig()
	if got := def.LLM.StreamWindowMaxBlocks; got != 128 {
		t.Errorf("StreamWindowMaxBlocks = %d, want 128", got)
	}
	if got := def.LLM.StreamWindowMaxNodes; got != 12800 {
		t.Errorf("StreamWindowMaxNodes = %d, want 12800", got)
	}
	if got := def.LLM.SearchMaxResultBytes; got != 128000 {
		t.Errorf("SearchMaxResultBytes = %d, want 128000", got)
	}
	if got := def.LLM.Temperature; got != 0.5 {
		t.Errorf("Temperature = %v, want 0.5", got)
	}
	if !def.BoardEnabled {
		t.Error("BoardEnabled = false, want true (board-enabled defaults to on)")
	}
}

// TestFix592BrowserHeadlessPlatforms covers UC-0001 case B: the default is
// platform dependent (off on Linux, on elsewhere) and reflects the host OS.
func TestFix592BrowserHeadlessPlatforms(t *testing.T) {
	cases := []struct {
		goos string
		want bool
	}{
		{"linux", false},
		{"darwin", true},
		{"windows", true},
		{"freebsd", true},
	}
	for _, tc := range cases {
		if got := defaultBrowserHeadless(tc.goos); got != tc.want {
			t.Errorf("defaultBrowserHeadless(%q) = %v, want %v", tc.goos, got, tc.want)
		}
	}
	if got, want := DefaultConfig().LLM.BrowserHeadless, runtime.GOOS != "linux"; got != want {
		t.Errorf("default BrowserHeadless on %s = %v, want %v", runtime.GOOS, got, want)
	}
}

// TestFix592WorkModesInheritGlobalTemperature covers UC-0001 case D: the default
// modes define no temperature, so they inherit the global default; an explicit
// mode temperature still wins (merge order mirrors agent.applyWorkModeConfig).
func TestFix592WorkModesInheritGlobalTemperature(t *testing.T) {
	modes := DefaultWorkModes()
	if len(modes) == 0 {
		t.Fatal("DefaultWorkModes returned no modes")
	}
	global := DefaultConfig().LLM.Temperature
	for _, m := range modes {
		if m.Temperature != nil {
			t.Errorf("mode %q Temperature = %v, want nil (inherit global default)", m.Name, *m.Temperature)
			continue
		}
		// global -> model -> mode (mode wins only when explicitly set)
		effective := global
		if m.Temperature != nil {
			effective = *m.Temperature
		}
		if effective != 0.5 {
			t.Errorf("mode %q effective temperature = %v, want 0.5", m.Name, effective)
		}
	}

	explicit := 0.9
	modes[0].Temperature = &explicit
	effective := global
	if modes[0].Temperature != nil {
		effective = *modes[0].Temperature
	}
	if effective != 0.9 {
		t.Errorf("explicit mode temperature ignored: got %v, want 0.9", effective)
	}
}

// TestFix592ExplicitConfigWins covers UC-0001 case C: values written in
// config.json (even the old ones) win over the new defaults, and an explicit
// board_enabled=false survives a Save/Load round trip (the omitempty fix).
func TestFix592ExplicitConfigWins(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	raw := []byte(`{
  "llm": {
    "temperature": 0.25,
    "stream_window_max_blocks": 300,
    "stream_window_max_nodes": 30000,
    "search_max_result_bytes": 65536,
    "browser_headless": false
  },
  "board_enabled": false
}`)
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	cfg, _, err := LoadFromFile(path, nil)
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	if cfg.LLM.Temperature != 0.25 {
		t.Errorf("Temperature = %v, want the explicit 0.25", cfg.LLM.Temperature)
	}
	if cfg.LLM.StreamWindowMaxBlocks != 300 {
		t.Errorf("StreamWindowMaxBlocks = %d, want the explicit 300", cfg.LLM.StreamWindowMaxBlocks)
	}
	if cfg.LLM.StreamWindowMaxNodes != 30000 {
		t.Errorf("StreamWindowMaxNodes = %d, want the explicit 30000", cfg.LLM.StreamWindowMaxNodes)
	}
	if cfg.LLM.SearchMaxResultBytes != 65536 {
		t.Errorf("SearchMaxResultBytes = %d, want the explicit 65536", cfg.LLM.SearchMaxResultBytes)
	}
	if cfg.LLM.BrowserHeadless {
		t.Error("explicit browser_headless=false was overridden by the platform default")
	}
	if cfg.BoardEnabled {
		t.Error("explicit board_enabled=false was overridden by the new default")
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	var keys map[string]any
	if err := json.Unmarshal(data, &keys); err != nil {
		t.Fatalf("saved config is not valid JSON: %v", err)
	}
	if _, ok := keys["board_enabled"]; !ok {
		t.Error("board_enabled missing from the saved config: an explicit false must be persisted")
	}

	reloaded, _, err := LoadFromFile(path, nil)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.BoardEnabled {
		t.Error("board_enabled=false did not survive Save/Load (omitempty regression)")
	}
	if reloaded.LLM.BrowserHeadless {
		t.Error("browser_headless=false did not survive Save/Load")
	}
	if reloaded.LLM.Temperature != 0.25 {
		t.Errorf("reloaded Temperature = %v, want 0.25", reloaded.LLM.Temperature)
	}
}
