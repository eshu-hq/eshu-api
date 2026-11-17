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

// Compare served-spec proof at the route level (Refs eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#72 gap 04, D13 row-7 full-surface). The openapi package tests pin the assembled document (see openapi/spec_compare_test.go); these tests pin that the native openapi.json route serves it to keyed callers with the gap-04 key present, with or without a core origin configured.

// compareSpecKeys are the path keys the Compare lane adds to the served
// document: one entry per served lane route.
var compareSpecKeys = []string{
	"/api/v0/compare/environments",
}

func TestCompareWaveSpecKeyCount(t *testing.T) {
	t.Parallel()

	if len(compareSpecKeys) != 1 {
		t.Fatalf("compareSpecKeys holds %d keys, want the 1 served Compare keys", len(compareSpecKeys))
	}
}

// compareSpecKeysOperations pins the served method plus operationId per
// path key, mirroring the pinned core fragments.
var compareSpecKeysOperations = map[string]map[string]string{
	"/api/v0/compare/environments": {"post": "compareEnvironments"},
}

// TestCompareNative_ServesWaveSpecKeys pins that the native spec
// route serves the Compare keys: authed callers get the
// dest-assembled document with every lane path and its pinned
// operationId, with or without a core origin configured.
func TestCompareNative_ServesWaveSpecKeys(t *testing.T) {
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
		for _, path := range compareSpecKeys {
			if _, ok := paths[path]; !ok {
				t.Errorf("origin %q: served spec lacks Compare key %s", origin, path)
			}
		}
		for path, wantOps := range compareSpecKeysOperations {
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
