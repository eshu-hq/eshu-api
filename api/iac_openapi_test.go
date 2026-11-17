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

// W3 IaC served-spec proof at the route level (Refs eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#102, D13 row-7 full-surface). The openapi package tests pin the assembled document (see openapi/spec_iac_test.go); these tests pin that the native openapi.json route serves it to keyed callers with the lane keys present, with or without a core origin configured.

// iacSpecKeys are the path keys the Iac lane adds to the served
// document: one entry per served lane route.
var iacSpecKeys = []string{
	"/api/v0/iac/dead",
	"/api/v0/iac/unmanaged-resources",
	"/api/v0/iac/terraform-import-plan/candidates",
	"/api/v0/iac/management-status",
	"/api/v0/iac/management-status/explain",
	"/api/v0/iac/resources",
}

func TestIacWaveSpecKeyCount(t *testing.T) {
	t.Parallel()

	if len(iacSpecKeys) != 6 {
		t.Fatalf("iacSpecKeys holds %d keys, want the 6 served Iac keys", len(iacSpecKeys))
	}
}

// iacSpecKeysOperations pins the served method plus operationId per
// path key, mirroring the pinned core fragments.
var iacSpecKeysOperations = map[string]map[string]string{
	"/api/v0/iac/dead":                             {"post": "findDeadIaC"},
	"/api/v0/iac/unmanaged-resources":              {"post": "findUnmanagedResources"},
	"/api/v0/iac/terraform-import-plan/candidates": {"post": "proposeTerraformImportPlanCandidates"},
	"/api/v0/iac/management-status":                {"post": "getIaCManagementStatus"},
	"/api/v0/iac/management-status/explain":        {"post": "explainIaCManagementStatus"},
	"/api/v0/iac/resources":                        {"get": "listIaCResources"},
}

// TestIacNative_ServesWaveSpecKeys pins that the native spec
// route serves the Iac keys: authed callers get the
// dest-assembled document with every lane path and its pinned
// operationId, with or without a core origin configured.
func TestIacNative_ServesWaveSpecKeys(t *testing.T) {
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
		for _, path := range iacSpecKeys {
			if _, ok := paths[path]; !ok {
				t.Errorf("origin %q: served spec lacks Iac key %s", origin, path)
			}
		}
		for path, wantOps := range iacSpecKeysOperations {
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
