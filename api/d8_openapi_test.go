// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// D8 session-resolver served-spec proof at the route level (Refs
// eshu-hq/eshu#9669, part of #6707). The openapi package tests pin the
// assembled document; these tests pin that the native openapi.json
// route serves it to keyed callers with the lane path keys present —
// for both a configured and an unconfigured core origin — while the
// key gate still fronts the route (the standing no-new-public-surface
// decision for the assembled spec).

// d8WaveSpecKeys maps every D8 lane path key to the methods the served
// document must carry for it. The invitations key carries both landed
// methods (the D3 GET plus the D8 POST merged into the same fragment);
// every other key carries exactly the methods its fragment documents.
var d8WaveSpecKeys = map[string][]string{
	"/api/v0/auth/browser-session":                    {"post", "get", "delete"},
	"/api/v0/auth/browser-session/context":            {"patch"},
	"/api/v0/auth/sessions":                           {"get"},
	"/api/v0/auth/profile":                            {"get"},
	"/api/v0/auth/local/bootstrap":                    {"post"},
	"/api/v0/auth/local/login":                        {"post"},
	"/api/v0/auth/local/invitations":                  {"post", "get"},
	"/api/v0/auth/local/invitations/accept":           {"post"},
	"/api/v0/auth/local/users/{user_id}/password":     {"post"},
	"/api/v0/auth/local/password/rotate":              {"post"},
	"/api/v0/auth/local/users/{user_id}/mfa-reset":    {"post"},
	"/api/v0/auth/local/users/{user_id}/disable":      {"post"},
	"/api/v0/auth/local/break-glass":                  {"post"},
	"/api/v0/auth/local/break-glass/session":          {"post"},
	"/api/v0/auth/local/api-tokens":                   {"get", "post"},
	"/api/v0/auth/local/api-tokens/{token_id}/revoke": {"post"},
	"/api/v0/auth/local/api-tokens/{token_id}/rotate": {"post"},
	"/api/v0/auth/local/mfa/totp/begin":               {"post"},
	"/api/v0/auth/local/mfa/totp/confirm":             {"post"},
}

func TestD8WaveSpecKeyCount(t *testing.T) {
	t.Parallel()

	if len(d8WaveSpecKeys) != 19 {
		t.Fatalf("d8WaveSpecKeys holds %d keys, want the 19 served D8 path keys", len(d8WaveSpecKeys))
	}
}

// TestD8Native_ServesWaveSpecKeys pins that the native spec route
// serves the lane keys: authed callers get the dest-assembled document
// with every lane path and its pinned methods, with or without a core
// origin configured.
func TestD8Native_ServesWaveSpecKeys(t *testing.T) {
	t.Parallel()

	for _, origin := range []string{"http://core.invalid", ""} {
		srv := NewServer(origin, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		req := authedRequest(http.MethodGet, "/api/v0/openapi.json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("origin %q: status = %d, want 200", origin, rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
			t.Errorf("origin %q: Content-Type = %q, want the JSON document shape", origin, got)
		}
		body, _ := io.ReadAll(rec.Result().Body)
		var doc map[string]any
		if err := json.Unmarshal(body, &doc); err != nil {
			t.Fatalf("origin %q: served spec is not valid JSON: %v", origin, err)
		}
		paths, _ := doc["paths"].(map[string]any)
		for path, methods := range d8WaveSpecKeys {
			item, ok := paths[path].(map[string]any)
			if !ok {
				t.Errorf("origin %q: served spec lacks lane key %s", origin, path)
				continue
			}
			if len(item) != len(methods) {
				t.Errorf("origin %q: %s serves %d methods, want %d (%v)", origin, path, len(item), len(methods), methods)
				continue
			}
			for _, method := range methods {
				if _, ok := item[method]; !ok {
					t.Errorf("origin %q: %s serves no %s operation", origin, path, method)
				}
			}
		}
	}
}
