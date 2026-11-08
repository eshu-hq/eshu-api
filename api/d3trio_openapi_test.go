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

// D3 Gap 02 trio served-spec proof at the route level (Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#70). The openapi
// package tests pin the assembled document; these tests pin that the
// native openapi.json route serves it to keyed callers with the trio
// path keys present — for both a configured and an unconfigured core
// origin — while the key gate still fronts the route (the standing
// no-new-public-surface decision for the assembled spec). The trio
// fragments carry no operationIds (the pinned core entries have
// none), so the D5 operationId probe does not watch them: this table
// is the claim TestD5Wiring_NoUndocumentedGETs unions instead.

// d3TrioSpecKeys are the path keys the Gap 02 trio adds to the served
// document: the delta-union completeness GET, the reindex progress
// GET, and the shared-projection tuning-report GET.
var d3TrioSpecKeys = []string{
	"/api/v0/admin/delta-union/completeness",
	"/api/v0/admin/reindex/progress",
	"/api/v0/admin/shared-projection/tuning-report",
}

// TestD3TrioWaveSpecKeyCount pins the trio claim surface: exactly the
// three Gap 02 routes, no more (the D7 reindex mutation POST keeps
// its own key and stays out of this lane).
func TestD3TrioWaveSpecKeyCount(t *testing.T) {
	t.Parallel()

	if len(d3TrioSpecKeys) != len(d3TrioWave) {
		t.Fatalf("d3TrioSpecKeys holds %d keys, want the %d trio routes", len(d3TrioSpecKeys), len(d3TrioWave))
	}
	seen := map[string]bool{}
	for _, path := range d3TrioSpecKeys {
		if seen[path] {
			t.Errorf("d3TrioSpecKeys duplicates %s", path)
		}
		seen[path] = true
	}
	for _, entry := range d3TrioWave {
		if !seen[entry.Path] {
			t.Errorf("d3TrioSpecKeys lacks trio route %s", entry.Path)
		}
	}
}

// TestD3TrioNative_ServesWaveSpecKeys pins that the native spec route
// serves the trio keys: authed callers get the dest-assembled
// document with every trio path, in both negotiated shapes, with or
// without a core origin configured.
func TestD3TrioNative_ServesWaveSpecKeys(t *testing.T) {
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
		for _, path := range d3TrioSpecKeys {
			item, ok := paths[path].(map[string]any)
			if !ok {
				t.Errorf("origin %q: served spec lacks trio key %s", origin, path)
				continue
			}
			if _, ok := item["get"].(map[string]any); !ok {
				t.Errorf("origin %q: served spec lacks get under trio key %s", origin, path)
			}
		}
	}
}
