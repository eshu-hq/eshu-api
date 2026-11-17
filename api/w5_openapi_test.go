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

// W5 entity/impact/story/packet served-spec proof at the route level (Refs eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#97, D13 row-7 full-surface). The openapi package tests pin the assembled document (see openapi/spec_w5_test.go); these tests pin that the native openapi.json route serves it to keyed callers with the W5 keys present, with or without a core origin configured.

// w5SpecKeys are the path keys the W5 lane adds to the served
// document: one entry per served lane route.
var w5SpecKeys = []string{
	"/api/v0/entities/resolve",
	"/api/v0/entities/{entity_id}/context",
	"/api/v0/workloads/{workload_id}/context",
	"/api/v0/workloads/{workload_id}/story",
	"/api/v0/services/{service_name}/context",
	"/api/v0/services/{service_name}/story",
	"/api/v0/services/{service_name}/api-surface/endpoints",
	"/api/v0/services/{service_name}/downstream-consumers",
	"/api/v0/investigations/services/{service_name}",
	"/api/v0/impact/trace-deployment-chain",
	"/api/v0/impact/blast-radius",
	"/api/v0/impact/change-surface",
	"/api/v0/impact/change-surface/investigate",
	"/api/v0/impact/contracts",
	"/api/v0/impact/deployment-config-influence",
	"/api/v0/impact/trace-exposure-path",
	"/api/v0/impact/pre-change",
	"/api/v0/impact/developer-change-plan",
	"/api/v0/impact/entity-map",
	"/api/v0/impact/resource-investigation",
	"/api/v0/impact/trace-resource-to-code",
	"/api/v0/impact/explain-dependency-path",
	"/api/v0/repositories/{repo_id}/story",
	"/api/v0/investigations/supply-chain/impact/packet",
}

func TestW5WaveSpecKeyCount(t *testing.T) {
	t.Parallel()

	if len(w5SpecKeys) != 24 {
		t.Fatalf("w5SpecKeys holds %d keys, want the 24 served W5 keys", len(w5SpecKeys))
	}
}

// w5SpecKeysOperations pins the served method plus operationId per
// path key, mirroring the pinned core fragments.
var w5SpecKeysOperations = map[string]map[string]string{
	"/api/v0/entities/resolve":                              {"post": "resolveEntity"},
	"/api/v0/entities/{entity_id}/context":                  {"get": "getEntityContext"},
	"/api/v0/workloads/{workload_id}/context":               {"get": "getWorkloadContext"},
	"/api/v0/workloads/{workload_id}/story":                 {"get": "getWorkloadStory"},
	"/api/v0/services/{service_name}/context":               {"get": "getServiceContext"},
	"/api/v0/services/{service_name}/story":                 {"get": "getServiceStory"},
	"/api/v0/services/{service_name}/api-surface/endpoints": {"get": "listServiceAPIEndpoints"},
	"/api/v0/services/{service_name}/downstream-consumers":  {"get": "listServiceDownstreamConsumers"},
	"/api/v0/investigations/services/{service_name}":        {"get": "investigateService"},
	"/api/v0/impact/trace-deployment-chain":                 {"post": "traceDeploymentChain"},
	"/api/v0/impact/blast-radius":                           {"post": "findBlastRadius"},
	"/api/v0/impact/change-surface":                         {"post": "findChangeSurface"},
	"/api/v0/impact/change-surface/investigate":             {"post": "investigateChangeSurface"},
	"/api/v0/impact/contracts":                              {"post": "investigateContractImpact"},
	"/api/v0/impact/deployment-config-influence":            {"post": "investigateDeploymentConfigInfluence"},
	"/api/v0/impact/trace-exposure-path":                    {"post": "traceExposurePath"},
	"/api/v0/impact/pre-change":                             {"post": "analyzePreChangeImpact"},
	"/api/v0/impact/developer-change-plan":                  {"post": "planDeveloperChange"},
	"/api/v0/impact/entity-map":                             {"post": "entityMap"},
	"/api/v0/impact/resource-investigation":                 {"post": "investigateResource"},
	"/api/v0/impact/trace-resource-to-code":                 {"post": "traceResourceToCode"},
	"/api/v0/impact/explain-dependency-path":                {"post": "explainDependencyPath"},
	"/api/v0/repositories/{repo_id}/story":                  {"get": "getRepositoryStory"},
	"/api/v0/investigations/supply-chain/impact/packet":     {"get": "getSupplyChainImpactPacket"},
}

// TestW5Native_ServesWaveSpecKeys pins that the native spec
// route serves the W5 keys: authed callers get the
// dest-assembled document with every lane path and its pinned
// operationId, with or without a core origin configured.
func TestW5Native_ServesWaveSpecKeys(t *testing.T) {
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
		for _, path := range w5SpecKeys {
			if _, ok := paths[path]; !ok {
				t.Errorf("origin %q: served spec lacks W5 key %s", origin, path)
			}
		}
		for path, wantOps := range w5SpecKeysOperations {
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
