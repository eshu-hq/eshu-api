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

// D7 admin-four served-spec proof at the route level (Refs
// eshu-hq/eshu#9669, part of #6707). The openapi package tests pin the
// assembled document; these tests pin that the native openapi.json
// route serves it to keyed callers with the lane path keys present —
// for both a configured and an unconfigured core origin — while the
// key gate still fronts the route (the standing no-new-public-surface
// decision for the assembled spec).

// d7WaveSpecKeys are the path keys the D7 lane adds to the served
// document: one entry per served lane route. The C-c2 recovery keys,
// the D3 admin query keys, and the mux-level openapi.json self entry
// keep their own posture: their keys are served by their own lanes,
// and no other lane's key duplicates a D7 key.
var d7WaveSpecKeys = []string{
	"/api/v0/admin/backfill",
	"/api/v0/admin/dead-letter",
	"/api/v0/admin/reindex",
	"/api/v0/admin/skip",
}

func TestD7WaveSpecKeyCount(t *testing.T) {
	t.Parallel()

	if len(d7WaveSpecKeys) != 4 {
		t.Fatalf("d7WaveSpecKeys holds %d keys, want the 4 served D7 keys", len(d7WaveSpecKeys))
	}
}

// TestD7Native_ServesWaveSpecKeys pins that the native spec route
// serves the lane keys: authed callers get the dest-assembled document
// with every lane path, with or without a core origin configured.
func TestD7Native_ServesWaveSpecKeys(t *testing.T) {
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
		for _, path := range d7WaveSpecKeys {
			item, ok := paths[path].(map[string]any)
			if !ok {
				t.Errorf("origin %q: served spec lacks lane key %s", origin, path)
				continue
			}
			if _, ok := item["post"]; !ok {
				t.Errorf("origin %q: %s serves no post operation", origin, path)
			}
		}
	}
}

// TestD7OpenAPIStillKeyGated pins that the assembled spec route stays
// behind the key gate: a credential-less caller gets the standing 401
// even though the document is fully dest-served.
func TestD7OpenAPIStillKeyGated(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v0/openapi.json", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want the standing 401", rec.Code)
	}
}
