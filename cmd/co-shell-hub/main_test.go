package main

import (
	"net"
	"strconv"
	"testing"
)

// TestLastAuthFlag covers the FIX-606 rule: --no-api-key and --api-key may both
// be given; the one appearing later on the command line decides.
func TestLastAuthFlag(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"neither flag", []string{"--tcp-addr", "127.0.0.1:12801"}, ""},
		{"no-api-key only", []string{"--no-api-key"}, "none"},
		{"no-api-key single dash", []string{"-no-api-key"}, "none"},
		{"api-key with value", []string{"--api-key", "K"}, "key"},
		{"api-key equals form", []string{"--api-key=K"}, "key"},
		{"api-key then no-api-key", []string{"--api-key", "K", "--no-api-key"}, "none"},
		{"no-api-key then api-key", []string{"--no-api-key", "--api-key", "K"}, "key"},
		{"no-api-key followed by unrelated flag", []string{"--no-api-key", "--web-addr", "x"}, "none"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := lastAuthFlag(tc.args); got != tc.want {
				t.Fatalf("lastAuthFlag(%q) = %q, want %q", tc.args, got, tc.want)
			}
		})
	}
}

// TestAuthDisabledByCLI covers the effective decision used by main: the config
// file's api_key is dropped exactly when auth is decided to be disabled.
func TestAuthDisabledByCLI(t *testing.T) {
	cases := []struct {
		name     string
		noAPIKey bool
		args     []string
		want     bool
	}{
		{"switch off, api-key given", false, []string{"--api-key", "K"}, false},
		{"switch off, no auth flags", false, nil, false},
		{"switch off but flag parsed as set (unreachable in practice)", true, []string{"--tcp-addr", "x"}, true},
		{"switch on, no-api-key last", true, []string{"--api-key", "K", "--no-api-key"}, true},
		{"switch on, api-key last", true, []string{"--no-api-key", "--api-key", "K"}, false},
		{"switch on, api-key equals form last", true, []string{"--no-api-key", "--api-key=K"}, false},
		{"switch on, no-api-key only", true, []string{"--no-api-key"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := authDisabledByCLI(tc.noAPIKey, tc.args); got != tc.want {
				t.Fatalf("authDisabledByCLI(%v, %q) = %v, want %v", tc.noAPIKey, tc.args, got, tc.want)
			}
		})
	}
}

// TestDefaultPortConstants pins the built-in listen port defaults (FEATURE-609).
func TestDefaultPortConstants(t *testing.T) {
	if defaultTCPPort != 27256 {
		t.Errorf("defaultTCPPort = %d, want 27256", defaultTCPPort)
	}
	if defaultWebPort != 28256 {
		t.Errorf("defaultWebPort = %d, want 28256", defaultWebPort)
	}
	if defaultBindHost != "127.0.0.1" {
		t.Errorf("defaultBindHost = %q, want 127.0.0.1", defaultBindHost)
	}
}

// TestResolveListenAddrExplicit: an explicitly configured address is returned
// unchanged, so an occupied port makes the later bind fail (startup aborts).
func TestResolveListenAddrExplicit(t *testing.T) {
	addr := "127.0.0.1:12345"
	if got := resolveListenAddr(addr, defaultBindHost, defaultTCPPort, true); got != addr {
		t.Fatalf("explicit resolveListenAddr = %q, want %q", got, addr)
	}
}

// TestResolveListenAddrAutoFree: when the built-in default port is free, the
// default address is used as-is.
func TestResolveListenAddrAutoFree(t *testing.T) {
	port := freeLoopbackPort(t)
	want := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	if got := resolveListenAddr("", "127.0.0.1", port, false); got != want {
		t.Fatalf("auto resolveListenAddr = %q, want %q", got, want)
	}
}

// TestResolveListenAddrAutoIncrement: when the built-in default port is
// occupied, the resolver increments the port until one is free.
func TestResolveListenAddrAutoIncrement(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	busy := ln.Addr().(*net.TCPAddr).Port
	got := resolveListenAddr("", "127.0.0.1", busy, false)
	if got == net.JoinHostPort("127.0.0.1", strconv.Itoa(busy)) {
		t.Fatalf("resolveListenAddr returned the occupied port %s", got)
	}
	if p := addrPort(got, 0); p <= busy {
		t.Fatalf("resolveListenAddr = %s, want a port above %d", got, busy)
	}
	if addrInUse(got) {
		t.Fatalf("resolveListenAddr returned an occupied port %s", got)
	}
}

// TestAddrPort covers host:port parsing and its fallbacks.
func TestAddrPort(t *testing.T) {
	cases := []struct {
		name     string
		addr     string
		fallback int
		want     int
	}{
		{"ipv4", "127.0.0.1:28256", 1, 28256},
		{"wildcard", "0.0.0.0:1", 1, 1},
		{"ipv6", "[::1]:65535", 1, 65535},
		{"missing port", "127.0.0.1", 9999, 9999},
		{"non-numeric", "127.0.0.1:abc", 9999, 9999},
		{"zero", "127.0.0.1:0", 9999, 9999},
		{"out of range", "127.0.0.1:70000", 9999, 9999},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := addrPort(tc.addr, tc.fallback); got != tc.want {
				t.Fatalf("addrPort(%q) = %d, want %d", tc.addr, got, tc.want)
			}
		})
	}
}

// TestAgentBasePort: managed agents default to the Web UI port + 1.
func TestAgentBasePort(t *testing.T) {
	cases := []struct {
		name    string
		webAddr string
		want    int
	}{
		{"default web port", "127.0.0.1:28256", 28257},
		{"auto-incremented web port", "127.0.0.1:28257", 28258},
		{"unparsable falls back", "garbage", 28257},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := agentBasePort(tc.webAddr); got != tc.want {
				t.Fatalf("agentBasePort(%q) = %d, want %d", tc.webAddr, got, tc.want)
			}
		})
	}
}

// freeLoopbackPort returns a currently free loopback port.
func freeLoopbackPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}
