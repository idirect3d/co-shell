// Package gateway - hub role tagging tests (FEATURE-530): the hub's agent link
// is dialled with role=hub so the agent keeps it in a separate slot from
// browser clients.
//
// Author: L.Shuang
// Created: 2026-09-17
// Last Modified: 2026-09-17
// MIT License - Copyright (c) 2026 L.Shuang

package gateway

import (
	"net/url"
	"testing"
)

// TestGatewayURLTagsRole verifies the hub adds role=hub at dial time while
// keeping any query the registry entry already carries, and leaves an
// unparsable URL untouched (dialWS then reports the parse error as before).
func TestGatewayURLTagsRole(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"plain", "ws://127.0.0.1:29241/ws", "ws://127.0.0.1:29241/ws?role=hub"},
		{"existing query", "ws://127.0.0.1:29241/ws?x=1", "ws://127.0.0.1:29241/ws?role=hub&x=1"},
		{"already tagged", "ws://127.0.0.1:29241/ws?role=hub", "ws://127.0.0.1:29241/ws?role=hub"},
		{"unparsable", "://bad", "://bad"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := gatewayURL(tc.raw)
			if got != tc.want {
				t.Fatalf("gatewayURL(%q) = %q, want %q", tc.raw, got, tc.want)
			}
			if u, err := url.Parse(got); err == nil && u.Scheme != "" {
				if u.Query().Get("role") != gatewayRole {
					t.Fatalf("role = %q, want %q", u.Query().Get("role"), gatewayRole)
				}
			}
		})
	}
}
