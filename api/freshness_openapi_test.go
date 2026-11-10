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

// Wave-1 Freshness served-spec proof at the route level (Refs
// eshu-hq/eshu#9669, D13 tracker eshu-hq/eshu#10491, satellite
// eshu-hq/eshu-api#72). The openapi package tests pin the assembled
// document; these tests pin that the native openapi.json route serves
// it to keyed callers with the wave path keys present — for both a
// configured and an unconfigured core origin — while the key gate
// still fronts the route (the standing no-new-public-surface decision
// for the assembled spec).

// freshnessWaveSpecKeys are the path keys the Wave-1 lane adds to the
// served document: one entry per served wave route. No Wave-1 route
// stays relay, so there is no relayed-keys list for this lane; the
// earlier-lane freshness-path keys (repository freshness, documentation
// freshness, freshness causality) keep their own lanes and entries.
var freshnessWaveSpecKeys = []string{
	"/api/v0/freshness/changed-since",
	"/api/v0/freshness/generations",
	"/api/v0/freshness/services/changed-since",
}

func TestFreshnessWaveSpecKeyCount(t *testing.T) {
	t.Parallel()

	if len(freshnessWaveSpecKeys) != 3 {
		t.Fatalf("freshnessWaveSpecKeys holds %d keys, want the 3 served Wave-1 keys", len(freshnessWaveSpecKeys))
	}
	if len(freshnessWaveSpecKeys) != len(freshnessWave) {
		t.Fatalf("freshnessWaveSpecKeys holds %d keys, want the %d wave routes", len(freshnessWaveSpecKeys), len(freshnessWave))
	}
}

// TestFreshnessNative_ServesWaveSpecKeys pins that the native spec
// route serves the wave keys: authed callers get the dest-assembled
// document with every wave path, with or without a core origin
// configured.
func TestFreshnessNative_ServesWaveSpecKeys(t *testing.T) {
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
		for _, path := range freshnessWaveSpecKeys {
			if _, ok := paths[path]; !ok {
				t.Errorf("origin %q: served spec lacks wave key %s", origin, path)
			}
		}
		if item, ok := paths["/api/v0/freshness/changed-since"].(map[string]any); ok {
			if op, _ := item["get"].(map[string]any); op["operationId"] != "summarizeChangedSince" {
				t.Errorf("origin %q: changed-since operationId = %v, want summarizeChangedSince", origin, op["operationId"])
			}
		}
		if item, ok := paths["/api/v0/freshness/generations"].(map[string]any); ok {
			if op, _ := item["get"].(map[string]any); op["operationId"] != "listGenerationLifecycle" {
				t.Errorf("origin %q: generations operationId = %v, want listGenerationLifecycle", origin, op["operationId"])
			}
		}
		if item, ok := paths["/api/v0/freshness/services/changed-since"].(map[string]any); ok {
			if op, _ := item["get"].(map[string]any); op["operationId"] != "summarizeServiceChangedSince" {
				t.Errorf("origin %q: service changed-since operationId = %v, want summarizeServiceChangedSince", origin, op["operationId"])
			}
		}
	}
}

// TestFreshnessOpenAPIStillKeyGated pins that the assembled spec route
// stays behind the key gate: a credential-less caller gets the
// standing 401 even though the document is fully dest-served.
func TestFreshnessOpenAPIStillKeyGated(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v0/openapi.json", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want the standing 401", rec.Code)
	}
}
