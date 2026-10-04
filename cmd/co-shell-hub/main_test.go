package main

import "testing"

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
