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

// D9 OIDC-login served-spec proof at the route level (Refs
// eshu-hq/eshu#9669, part of #6707). The openapi package tests pin the
// assembled document; these tests pin that the native openapi.json
// route serves it to keyed callers with the lane path keys present —
// for both a configured and an unconfigured core origin — while the
// key gate still fronts the route (the standing no-new-public-surface
// decision for the assembled spec).

// d9WaveSpecKeys maps every D9 OIDC lane path key to the methods the
// served document must carry for it. Both keys carry exactly the GET
// operation their fragment documents; the GitHub/SAML keys ride
// their own landed lane tables (d9GitHubWaveSpecKeys,
// d9SAMLWaveSpecKeys).
var d9WaveSpecKeys = map[string][]string{
	"/api/v0/auth/oidc/login":    {"get"},
	"/api/v0/auth/oidc/callback": {"get"},
}

func TestD9WaveSpecKeyCount(t *testing.T) {
	t.Parallel()

	if len(d9WaveSpecKeys) != 2 {
		t.Fatalf("d9WaveSpecKeys holds %d keys, want the 2 served OIDC path keys", len(d9WaveSpecKeys))
	}
}

// TestD9Native_ServesWaveSpecKeys pins that the native spec route
// serves the lane keys: authed callers get the dest-assembled document
// with every lane path and its pinned methods, with or without a core
// origin configured.
func TestD9Native_ServesWaveSpecKeys(t *testing.T) {
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
		for path, methods := range d9WaveSpecKeys {
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
