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

// Drift served-spec proof at the route level (Refs eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#117, D13 row-7 full-surface). The openapi package tests pin the assembled document (see openapi/spec_drift_test.go); these tests pin that the native openapi.json route serves it to keyed callers with the drift keys present, with or without a core origin configured.

// driftSpecKeys are the path keys the Drift lane adds to the served
// document: one entry per served lane route.
var driftSpecKeys = []string{
	"/api/v0/cloud/runtime-drift/findings",
	"/api/v0/investigations/drift/packet",
}

func TestDriftWaveSpecKeyCount(t *testing.T) {
	t.Parallel()

	if len(driftSpecKeys) != 2 {
		t.Fatalf("driftSpecKeys holds %d keys, want the 2 served Drift keys", len(driftSpecKeys))
	}
}

// driftSpecKeysOperations pins the served method plus operationId per
// path key, mirroring the pinned core fragments.
var driftSpecKeysOperations = map[string]map[string]string{
	"/api/v0/cloud/runtime-drift/findings": {"post": "listCloudRuntimeDriftFindings"},
	"/api/v0/investigations/drift/packet":  {"get": "getDriftPacket"},
}

// TestDriftNative_ServesWaveSpecKeys pins that the native spec
// route serves the Drift keys: authed callers get the
// dest-assembled document with every lane path and its pinned
// operationId, with or without a core origin configured.
func TestDriftNative_ServesWaveSpecKeys(t *testing.T) {
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
		body, _ := io.ReadAll(rec.Result().Body)
		var doc map[string]any
		if err := json.Unmarshal(body, &doc); err != nil {
			t.Fatalf("origin %q: served spec is not valid JSON: %v", origin, err)
		}
		paths, _ := doc["paths"].(map[string]any)
		for _, path := range driftSpecKeys {
			if _, ok := paths[path]; !ok {
				t.Errorf("origin %q: served spec lacks Drift key %s", origin, path)
			}
		}
		for path, wantOps := range driftSpecKeysOperations {
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
