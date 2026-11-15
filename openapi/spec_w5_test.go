// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// W5 entity/impact/story/packet termination spec posture (Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#97). The 24 W5 routes
// mount natively (21 terminate; the entity context fallback leg and
// the two code-surface adapter legs hold on explicit relay), but
// their Decision 5(c) fragments stay core-side for this lane: the
// entity entries live in
// go/internal/query/openapi/paths/search/entities.go (plus the graph
// entities entry in
// go/internal/query/openapi/paths/search/graph_entities.go), the
// impact entries live in
// go/internal/query/openapi/paths/impact/routes.go, contract.go,
// deployment_config_influence.go, exposure.go, and rest.go, the story
// entry interpolates the shared schema.EvidenceBoundaries helper, and
// the packet entry lives in
// go/internal/query/openapi/paths/evidence/investigations.go.
// Porting any entry without its referenced blocks would break the
// verbatim-port rule scripts/check-parity.sh gates — so, like the
// gap-04 Compare, W6a Code-flow, W4 search, G-a graph, dead-code,
// divergence, G-b relationship, W8a, and drift lanes, the routes are
// disclosed spec absences (see contracts/entity-termination.md,
// contracts/impact-termination.md,
// contracts/repository-story-termination.md, and
// contracts/supply-chain-impact-packet-termination.md), not a port. A
// follow-up spec-port lane claims the keys when it ports the entries
// entry-shaped with their referenced schemas; until then no served
// key duplicates a relayed or terminated route, and no mounted route
// promises what the document does not carry. The story absence moves
// here from TestSpec_D4DisclosedOmissions with this lane: it is no
// longer relay-only, it is disclosed-absent-as-terminated. The packet
// absence moves here the same way: it is no longer relay-only
// (D4 disclosed omission), it is disclosed-absent-as-terminated with
// the Decision 2 corpus closed in-lane.

// TestSpec_W5StaysDisclosedAbsent pins the W5 spec posture: the served
// document carries none of the 24 W5 keys while the fragments stay
// core-side. Lifting any absence (porting the entries) belongs to the
// follow-up spec-port lane, not to the relay-leg termination.
func TestSpec_W5StaysDisclosedAbsent(t *testing.T) {
	t.Parallel()

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
		"/api/v0/graph/entities",
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
		if _, ok := served[path]; ok {
			t.Errorf("served spec carries %s, want it disclosed-absent until the follow-up spec-port lane", path)
		}
	}
}
