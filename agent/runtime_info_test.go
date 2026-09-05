// FEATURE-481: runtime environment awareness tests.
//
// Author: L.Shuang
// Created: 2026-09-05
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"strings"
	"testing"
)

// TestBuildRuntimeInfo_NotInjected verifies that when SetRuntimeInfo was never
// called, buildRuntimeInfo returns "" so the envelope stays backward compatible
// (FEATURE-481 UC-0001).
func TestBuildRuntimeInfo_NotInjected(t *testing.T) {
	a := &Agent{}
	if got := a.buildRuntimeInfo(); got != "" {
		t.Errorf("buildRuntimeInfo without injection = %q, want empty", got)
	}
}

// TestBuildRuntimeInfo_Stdio verifies stdio service mode output (UC-0002).
func TestBuildRuntimeInfo_Stdio(t *testing.T) {
	a := &Agent{}
	a.SetRuntimeInfo(RuntimeInfo{PID: 1234, Version: "0.36.0", Build: "842", ServiceMode: "stdio"})
	got := a.buildRuntimeInfo()
	for _, want := range []string{
		"<runtime_info>", "<pid>1234</pid>", "<version>0.36.0</version>",
		"<build>842</build>", "<service_mode>stdio</service_mode>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("stdio runtime_info missing %q, got:\n%s", want, got)
		}
	}
	// stdio mode must NOT report serve-only fields.
	for _, notWant := range []string{"<serve_port>", "<serve_bind>", "<serve_whitelist>"} {
		if strings.Contains(got, notWant) {
			t.Errorf("stdio runtime_info should not contain %q, got:\n%s", notWant, got)
		}
	}
}

// TestBuildRuntimeInfo_ModelName verifies the <model_name> tag is rendered when
// a model name is injected and omitted when empty (FEATURE-482 UC-0006).
func TestBuildRuntimeInfo_ModelName(t *testing.T) {
	a := &Agent{}
	a.SetRuntimeInfo(RuntimeInfo{PID: 1, Version: "0.36.0", Build: "842", ModelName: "deepseek-chat", ServiceMode: "stdio"})
	got := a.buildRuntimeInfo()
	if !strings.Contains(got, "<model_name>deepseek-chat</model_name>") {
		t.Errorf("runtime_info missing model_name, got:\n%s", got)
	}

	// Empty model name must omit the tag entirely.
	a2 := &Agent{}
	a2.SetRuntimeInfo(RuntimeInfo{PID: 2, Version: "0.36.0", Build: "842", ServiceMode: "stdio"})
	got2 := a2.buildRuntimeInfo()
	if strings.Contains(got2, "<model_name>") {
		t.Errorf("runtime_info should omit model_name when empty, got:\n%s", got2)
	}
}

// TestBuildRuntimeInfo_Enhanced verifies enhanced service mode output (UC-0003).
func TestBuildRuntimeInfo_Enhanced(t *testing.T) {
	a := &Agent{}
	a.SetRuntimeInfo(RuntimeInfo{PID: 5678, Version: "0.36.0", Build: "842", ServiceMode: "enhanced"})
	got := a.buildRuntimeInfo()
	if !strings.Contains(got, "<service_mode>enhanced</service_mode>") {
		t.Errorf("enhanced runtime_info missing service_mode, got:\n%s", got)
	}
	for _, notWant := range []string{"<serve_port>", "<serve_bind>", "<serve_whitelist>"} {
		if strings.Contains(got, notWant) {
			t.Errorf("enhanced runtime_info should not contain %q, got:\n%s", notWant, got)
		}
	}
}

// TestBuildRuntimeInfo_Serve verifies serve mode output with port/bind/whitelist
// (UC-0004).
func TestBuildRuntimeInfo_Serve(t *testing.T) {
	a := &Agent{}
	a.SetRuntimeInfo(RuntimeInfo{
		PID: 9012, Version: "0.36.0", Build: "842", ServiceMode: "serve",
		ServePort: 8399, ServeBind: "127.0.0.1", ServeWhitelist: []string{"192.168.1.100", "192.168.1.0/24"},
	})
	got := a.buildRuntimeInfo()
	for _, want := range []string{
		"<service_mode>serve</service_mode>", "<serve_port>8399</serve_port>",
		"<serve_bind>127.0.0.1</serve_bind>",
		"<serve_whitelist>192.168.1.100,192.168.1.0/24</serve_whitelist>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("serve runtime_info missing %q, got:\n%s", want, got)
		}
	}
}

// TestBuildRuntimeInfo_ServeNoWhitelist verifies serve mode omits serve_whitelist
// when the whitelist is empty (UC-0005).
func TestBuildRuntimeInfo_ServeNoWhitelist(t *testing.T) {
	a := &Agent{}
	a.SetRuntimeInfo(RuntimeInfo{
		PID: 9012, Version: "0.36.0", Build: "842", ServiceMode: "serve",
		ServePort: 8399, ServeBind: "0.0.0.0",
	})
	got := a.buildRuntimeInfo()
	if !strings.Contains(got, "<serve_port>8399</serve_port>") {
		t.Errorf("serve runtime_info missing serve_port, got:\n%s", got)
	}
	if strings.Contains(got, "<serve_whitelist>") {
		t.Errorf("serve runtime_info should omit serve_whitelist when empty, got:\n%s", got)
	}
}
