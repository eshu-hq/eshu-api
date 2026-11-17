// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// W5 entity/impact/story/packet full-surface spec proof (Refs eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#97). The 24 W5 routes terminate natively and their Decision 5(c) fragments (entities.go, the impact package, repository_story.go, investigation_packets.go) now ride the served document with their referenced schemas and the vendored schema helpers; the D13 row-7 full-surface lane claims the keys the lane disclosed absent-as-terminated.
//
// The lossless-assembly property holds for the lane keys: the
// served path objects equal the ported fragments parsed path
// objects, so the only reviewed delta to the pinned core
// fragments is the fragment files themselves (gated
// byte-identical or line-verbatim by scripts/check-parity.sh).
func TestSpec_ServesW5Waves(t *testing.T) {
	assertServedPaths(t, map[string]map[string]string{
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
	})

	fragment := decodePaths(t, `{"paths": {`+familyPaths()+`  }}`)
	served := decodePaths(t, Spec())
	for _, path := range []string{
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
	} {
		want, ok := fragment[path]
		if !ok {
			t.Fatalf("fragment lacks %s", path)
		}
		got, ok := served[path].(map[string]any)
		if !ok {
			t.Fatalf("served spec lacks %s", path)
		}
		if !jsonEqual(got, want) {
			t.Errorf("served %s differs from the ported fragment", path)
		}
	}
}
