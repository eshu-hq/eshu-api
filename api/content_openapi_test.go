// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Wave-2 Content served-spec proof at the route level (Refs
// eshu-hq/eshu#9669, D13 tracker eshu-hq/eshu#10491, satellite
// eshu-hq/eshu-api#72). The openapi package tests pin the assembled
// document; these tests pin that the native openapi.json route serves
// it to keyed callers with the wave path keys present — for both a
// configured and an unconfigured core origin — while the key gate
// still fronts the route (the standing no-new-public-surface decision
// for the assembled spec).

// contentWaveSpecKeys are the path keys the Wave-2 lane adds to the
// served document: one entry per served wave route. No wave route
// stays relay, so there is no relayed-keys list for this lane; the
// remaining out-of-pin content legs (list, scoped-entity) keep
// relaying with their fragments core-side, disclosed in
// contracts/content-point-reads-wave.md. The search legs now ride the
// W4 lane (contracts/content-search-termination.md).
var contentWaveSpecKeys = []string{
	"/api/v0/content/files/read",
	"/api/v0/content/files/lines",
	"/api/v0/content/entities/read",
}

func TestContentPointReadsWaveSpecKeyCount(t *testing.T) {
	t.Parallel()

	if len(contentWaveSpecKeys) != 3 {
		t.Fatalf("contentWaveSpecKeys holds %d keys, want the 3 served wave keys", len(contentWaveSpecKeys))
	}
	if len(contentWaveSpecKeys) != len(contentWave) {
		t.Fatalf("contentWaveSpecKeys holds %d keys, want the %d wave routes", len(contentWaveSpecKeys), len(contentWave))
	}
}

// TestContentPointReadsNative_ServesWaveSpecKeys pins that the native
// spec route serves the wave keys: authed callers get the dest-assembled
// document with every wave path, with or without a core origin
// configured.
func TestContentPointReadsNative_ServesWaveSpecKeys(t *testing.T) {
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
		for _, path := range contentWaveSpecKeys {
			if _, ok := paths[path]; !ok {
				t.Errorf("origin %q: served spec lacks wave key %s", origin, path)
			}
		}
		for path, wantOp := range map[string]string{
			"/api/v0/content/files/read":    "readFile",
			"/api/v0/content/files/lines":   "readFileLines",
			"/api/v0/content/entities/read": "readEntity",
		} {
			item, ok := paths[path].(map[string]any)
			if !ok {
				continue
			}
			op, _ := item["post"].(map[string]any)
			if op["operationId"] != wantOp {
				t.Errorf("origin %q: %s operationId = %v, want %s", origin, path, op["operationId"], wantOp)
			}
		}
	}
}

// TestContentPointReadsOpenAPIStillKeyGated pins that the assembled
// spec route stays behind the key gate: a credential-less caller gets
// the standing 401 even though the document is fully dest-served.
func TestContentPointReadsOpenAPIStillKeyGated(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v0/openapi.json", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want the standing 401", rec.Code)
	}
}

// TestContentPointReadsSearchKeysNowServed pins the W4 inversion: the
// search keys the point pin sealed out now ride the W4 lane, so the
// served document carries all four search paths with their pinned
// operationIds. The fragment ports entry-shaped with the lane
// (contracts/content-search-termination.md), so no served key
// duplicates a relayed route.
func TestContentPointReadsSearchKeysNowServed(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest(http.MethodGet, "/api/v0/openapi.json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body, _ := io.ReadAll(rec.Result().Body)
	for path, wantOp := range map[string]string{
		"/api/v0/content/files/search":               "searchFiles",
		"/api/v0/content/entities/search":            "searchEntities",
		"/api/v0/content/dockerfile-installs/search": "searchDockerfileInstalls",
		"/api/v0/content/php-config/search":          "searchPHPConfiguration",
	} {
		if !strings.Contains(string(body), `"`+path+`"`) {
			t.Errorf("served spec lacks %s, want it served by the W4 search lane", path)
		}
		if !strings.Contains(string(body), `"operationId":"`+wantOp+`"`) && !strings.Contains(string(body), `"operationId": "`+wantOp+`"`) {
			t.Errorf("served spec lacks operationId %s for %s", wantOp, path)
		}
	}
}
