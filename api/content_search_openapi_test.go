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

// W4 Content search served-spec proof at the route level (Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#111). The openapi
// package tests pin the assembled document; these tests pin that the
// native openapi.json route serves it to keyed callers with the wave
// path keys present — for both a configured and an unconfigured core
// origin — while the key gate still fronts the route (the standing
// no-new-public-surface decision for the assembled spec).

// contentSearchWaveSpecKeys are the path keys the W4 lane adds to the
// served document: one entry per served wave route.
var contentSearchWaveSpecKeys = []string{
	"/api/v0/content/files/search",
	"/api/v0/content/entities/search",
	"/api/v0/content/dockerfile-installs/search",
	"/api/v0/content/php-config/search",
}

func TestContentSearchWaveSpecKeyCount(t *testing.T) {
	t.Parallel()

	if len(contentSearchWaveSpecKeys) != 4 {
		t.Fatalf("contentSearchWaveSpecKeys holds %d keys, want the 4 served wave keys", len(contentSearchWaveSpecKeys))
	}
	if len(contentSearchWaveSpecKeys) != len(contentSearchWave) {
		t.Fatalf("contentSearchWaveSpecKeys holds %d keys, want the %d wave routes", len(contentSearchWaveSpecKeys), len(contentSearchWave))
	}
}

// TestContentSearchNative_ServesWaveSpecKeys pins that the native spec
// route serves the wave keys: authed callers get the dest-assembled
// document with every wave path, with or without a core origin
// configured.
func TestContentSearchNative_ServesWaveSpecKeys(t *testing.T) {
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
		for _, path := range contentSearchWaveSpecKeys {
			if _, ok := paths[path]; !ok {
				t.Errorf("origin %q: served spec lacks wave key %s", origin, path)
			}
		}
		for path, wantOp := range map[string]string{
			"/api/v0/content/files/search":               "searchFiles",
			"/api/v0/content/entities/search":            "searchEntities",
			"/api/v0/content/dockerfile-installs/search": "searchDockerfileInstalls",
			"/api/v0/content/php-config/search":          "searchPHPConfiguration",
		} {
			item, ok := paths[path].(map[string]any)
			if !ok {
				continue
			}
			post, ok := item["post"].(map[string]any)
			if !ok {
				t.Errorf("origin %q: %s serves no post operation", origin, path)
				continue
			}
			if got := post["operationId"]; got != wantOp {
				t.Errorf("origin %q: %s operationId = %v, want %q", origin, path, got, wantOp)
			}
		}
	}
}
