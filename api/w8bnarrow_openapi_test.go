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

// W8b-narrow served-spec proof at the route level (Refs eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#101, D13 row-7 full-surface). The openapi package tests pin the assembled document (see openapi/spec_w8bnarrow_test.go); these tests pin that the native openapi.json route serves it to keyed callers with the narrow-family keys present, with or without a core origin configured.

// w8bnarrowSpecKeys are the path keys the W8bNarrow lane adds to the served
// document: one entry per served lane route.
var w8bnarrowSpecKeys = []string{
	"/api/v0/kubernetes/correlations",
	"/api/v0/observability/coverage/correlations",
	"/api/v0/work-items/evidence",
	"/api/v0/semantic/code-hints",
	"/api/v0/semantic/documentation-observations",
	"/api/v0/dependencies",
	"/api/v0/graph/entities",
	"/api/v0/images",
	"/api/v0/images/tag-history",
	"/api/v0/incidents/{incident_id}/context",
	"/api/v0/runtime-declarations/query",
	"/api/v0/cloud/inventory",
	"/api/v0/cloud/ssm-host-inventory",
	"/api/v0/service-catalog/correlations",
	"/api/v0/codeowners/ownership",
}

func TestW8bNarrowWaveSpecKeyCount(t *testing.T) {
	t.Parallel()

	if len(w8bnarrowSpecKeys) != 15 {
		t.Fatalf("w8bnarrowSpecKeys holds %d keys, want the 15 served W8bNarrow keys", len(w8bnarrowSpecKeys))
	}
}

// w8bnarrowSpecKeysOperations pins the served method plus operationId per
// path key, mirroring the pinned core fragments.
var w8bnarrowSpecKeysOperations = map[string]map[string]string{
	"/api/v0/kubernetes/correlations":             {"get": "listKubernetesCorrelations"},
	"/api/v0/observability/coverage/correlations": {"get": "listObservabilityCoverageCorrelations"},
	"/api/v0/work-items/evidence":                 {"get": "listWorkItemEvidence"},
	"/api/v0/semantic/code-hints":                 {"get": "listSemanticCodeHints"},
	"/api/v0/semantic/documentation-observations": {"get": "listSemanticDocumentationObservations"},
	"/api/v0/dependencies":                        {"get": "listDependencies"},
	"/api/v0/graph/entities":                      {"get": "browseGraphEntityInventory"},
	"/api/v0/images":                              {"get": "listContainerImages"},
	"/api/v0/images/tag-history":                  {"get": "listContainerImageTagHistory"},
	"/api/v0/incidents/{incident_id}/context":     {"get": "getIncidentContext"},
	"/api/v0/runtime-declarations/query":          {"post": "listRuntimeVersionDeclarations"},
	"/api/v0/cloud/inventory":                     {"get": "listCloudResourceInventory"},
	"/api/v0/cloud/ssm-host-inventory":            {"get": "listSSMHostInventory"},
	"/api/v0/service-catalog/correlations":        {"get": "listServiceCatalogCorrelations"},
	"/api/v0/codeowners/ownership":                {"get": "listCodeownersOwnership"},
}

// TestW8bNarrowNative_ServesWaveSpecKeys pins that the native spec
// route serves the W8bNarrow keys: authed callers get the
// dest-assembled document with every lane path and its pinned
// operationId, with or without a core origin configured.
func TestW8bNarrowNative_ServesWaveSpecKeys(t *testing.T) {
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
		for _, path := range w8bnarrowSpecKeys {
			if _, ok := paths[path]; !ok {
				t.Errorf("origin %q: served spec lacks W8bNarrow key %s", origin, path)
			}
		}
		for path, wantOps := range w8bnarrowSpecKeysOperations {
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
