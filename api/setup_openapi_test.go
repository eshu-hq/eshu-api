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

// Setup served-spec proof at the route level (Refs eshu-hq/eshu#9669, eshu-api#44 revisit, D13 row-7 full-surface). The openapi package tests pin the assembled document (see openapi/spec_setup_test.go); these tests pin that the native openapi.json route serves it to keyed callers with the setup keys present, with or without a core origin configured. Posture (public markers, key gate, carve-out) stays pinned by TestSetupSpecPosture_StaysCoreSide.

// setupSpecKeys are the path keys the Setup lane adds to the served
// document: one entry per served lane route.
var setupSpecKeys = []string{
	"/api/v0/auth/setup-state",
	"/api/v0/auth/setup/claim",
	"/api/v0/auth/setup/admin",
	"/api/v0/auth/setup/mfa",
}

func TestSetupWaveSpecKeyCount(t *testing.T) {
	t.Parallel()

	if len(setupSpecKeys) != 4 {
		t.Fatalf("setupSpecKeys holds %d keys, want the 4 served Setup keys", len(setupSpecKeys))
	}
}

// setupSpecKeysOperations pins the served method plus operationId per
// path key, mirroring the pinned core fragments.
var setupSpecKeysOperations = map[string]map[string]string{
	"/api/v0/auth/setup-state": {"get": "getAuthSetupState"},
	"/api/v0/auth/setup/claim": {"post": "claimSetup"},
	"/api/v0/auth/setup/admin": {"post": "createSetupAdmin"},
	"/api/v0/auth/setup/mfa":   {"post": "completeSetupMFA"},
}

// TestSetupNative_ServesWaveSpecKeys pins that the native spec
// route serves the Setup keys: authed callers get the
// dest-assembled document with every lane path and its pinned
// operationId, with or without a core origin configured.
func TestSetupNative_ServesWaveSpecKeys(t *testing.T) {
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
		for _, path := range setupSpecKeys {
			if _, ok := paths[path]; !ok {
				t.Errorf("origin %q: served spec lacks Setup key %s", origin, path)
			}
		}
		for path, wantOps := range setupSpecKeysOperations {
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
