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

// D9 GitHub-login served-spec proof at the route level (Refs
// eshu-hq/eshu#9669, part of #6707). The openapi package tests pin the
// assembled document; these tests pin that the native openapi.json
// route serves it to keyed callers with the lane path keys present —
// for both a configured and an unconfigured core origin — while the
// key gate still fronts the route (the standing no-new-public-surface
// decision for the assembled spec).

// d9GitHubWaveSpecKeys maps every D9 GitHub lane path key to the
// methods the served document must carry for it. Both keys carry exactly
// the GET operation their fragment documents; the OIDC keys stay in
// their own lane table and the SAML keys stay out until their own
// sequential lane.
var d9GitHubWaveSpecKeys = map[string][]string{
	"/api/v0/auth/github/login":    {"get"},
	"/api/v0/auth/github/callback": {"get"},
}

func TestD9GitHubWaveSpecKeyCount(t *testing.T) {
	t.Parallel()

	if len(d9GitHubWaveSpecKeys) != 2 {
		t.Fatalf("d9GitHubWaveSpecKeys holds %d keys, want the 2 served GitHub path keys", len(d9GitHubWaveSpecKeys))
	}
}

// TestD9GitHubNative_ServesWaveSpecKeys pins that the native spec route
// serves the lane keys: authed callers get the dest-assembled document
// with every lane path and its pinned methods, with or without a core
// origin configured.
func TestD9GitHubNative_ServesWaveSpecKeys(t *testing.T) {
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
			t.Fatalf("origin %q: served spec is not valid JSON: %v", origin, body)
		}
		paths, _ := doc["paths"].(map[string]any)
		for path, methods := range d9GitHubWaveSpecKeys {
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
