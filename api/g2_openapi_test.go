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

// G2 served-spec proof at the route level (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#101 RE:G2V2, D13 row-7 full-surface).
// The 7 G2 routes mount natively in the G2 lane and their Decision
// 5(c) fragments now ride the served document (see
// openapi/spec_g2_test.go). The openapi package tests pin the
// assembled document; these tests pin that the native openapi.json
// route serves it to keyed callers with the wave keys present — for
// both a configured and an unconfigured core origin — while the key
// gate still fronts the route (the standing no-new-public-surface
// decision for the assembled spec).

// g2WaveSpecKeys are the path keys the G2 lane adds to the served
// document: one entry per mounted wave route.
var g2WaveSpecKeys = []string{
	"/api/v0/collector-extraction-readiness",
	"/api/v0/collector-extraction-readiness/{family}",
	"/api/v0/fact-schema-versions",
	"/api/v0/fact-schema-versions/{fact_kind}",
	"/api/v0/surface-inventory",
	"/api/v0/visualizations/derive",
	"/api/v0/repository/manifest-dependencies",
}

func TestG2WaveSpecKeyCount(t *testing.T) {
	t.Parallel()

	if len(g2WaveSpecKeys) != 7 {
		t.Fatalf("g2WaveSpecKeys holds %d keys, want the 7 served G2 keys", len(g2WaveSpecKeys))
	}
	if len(g2WaveSpecKeys) != len(g2Wave) {
		t.Fatalf("g2WaveSpecKeys holds %d keys, want the %d wave routes", len(g2WaveSpecKeys), len(g2Wave))
	}
}

// g2WaveSpecOperations pins the served method plus operationId per G2
// path key, mirroring the pinned core fragments.
var g2WaveSpecOperations = map[string]map[string]string{
	"/api/v0/collector-extraction-readiness":          {"get": "listCollectorExtractionReadiness"},
	"/api/v0/collector-extraction-readiness/{family}": {"get": "getCollectorExtractionReadiness"},
	"/api/v0/fact-schema-versions":                    {"get": "listFactSchemaVersions"},
	"/api/v0/fact-schema-versions/{fact_kind}":        {"get": "getFactSchemaVersion"},
	"/api/v0/surface-inventory":                       {"get": "listSurfaceInventory"},
	"/api/v0/visualizations/derive":                   {"post": "deriveVisualizationPacket"},
	"/api/v0/repository/manifest-dependencies":        {"get": "listRepositoryManifestDependencies"},
}

// TestG2Native_ServesWaveSpecKeys pins that the native spec route
// serves the G2 keys: authed callers get the dest-assembled document
// with every wave path and its pinned operationId, with or without a
// core origin configured.
func TestG2Native_ServesWaveSpecKeys(t *testing.T) {
	t.Parallel()

	for _, origin := range []string{"http://core.invalid", ""} {
		srv := NewServer(origin, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		req := g2AuthedReq(http.MethodGet, "/api/v0/openapi.json", "")
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
		for _, path := range g2WaveSpecKeys {
			if _, ok := paths[path]; !ok {
				t.Errorf("origin %q: served spec lacks G2 wave key %s", origin, path)
			}
		}
		for path, wantOps := range g2WaveSpecOperations {
			item, ok := paths[path].(map[string]any)
			if !ok {
				continue
			}
			for method, wantID := range wantOps {
				op, _ := item[method].(map[string]any)
				if got := op["operationId"]; got != wantID {
					t.Errorf("origin %q: %s %s operationId = %v, want %q", origin, path, method, got, wantID)
				}
			}
		}
	}
}

// TestG2OpenAPIServesPublicly pins the eshu-hq/eshu-api#164 outcome:
// the assembled spec route serves pre-auth, so a credential-less
// caller gets 200 with the document instead of the standing 401.
func TestG2OpenAPIServesPublicly(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v0/openapi.json", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for the keyless spec caller", rec.Code)
	}
}
