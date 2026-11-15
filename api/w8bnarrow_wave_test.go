// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"testing"
)

// TestW8bNarrowWaveFrozen pins the frozen W8b-narrow termination
// target entry by entry (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#101): exactly the 15 narrow-family routes behind
// the fourteen versioned narrow pins, v0. A wave change moves the
// ownership note, the fragment reference, and this mirror together.
// The 2 drift routes stay out (Wave-4 Drift lane,
// eshu-hq/eshu-api#117); the component-extensions pair and the 6
// no-port routes stay out (no core mint, relay with a TODO).
func TestW8bNarrowWaveFrozen(t *testing.T) {
	t.Parallel()

	want := []w8bNarrowRoute{
		{Method: "GET", Path: "/api/v0/kubernetes/correlations", Family: "kubernetes", Serve: w8bNarrowServeGated, Fragment: "go/internal/query/openapi/paths/infrastructure/kubernetes.go"},
		{Method: "GET", Path: "/api/v0/observability/coverage/correlations", Family: "observability", Serve: w8bNarrowServeGated, Fragment: "go/internal/query/openapi/paths/infrastructure/observability_coverage.go"},
		{Method: "GET", Path: "/api/v0/work-items/evidence", Family: "work_items", Serve: w8bNarrowServeGated, Fragment: "go/internal/query/openapi/paths/evidence/work_item.go"},
		{Method: "GET", Path: "/api/v0/semantic/code-hints", Family: "semantic", Serve: w8bNarrowServeGated, Fragment: "go/internal/query/openapi/paths/search/semantic_evidence.go"},
		{Method: "GET", Path: "/api/v0/semantic/documentation-observations", Family: "semantic", Serve: w8bNarrowServeGated, Fragment: "go/internal/query/openapi/paths/search/semantic_evidence.go"},
		{Method: "GET", Path: "/api/v0/dependencies", Family: "dependencies", Serve: w8bNarrowServeGated, Fragment: "go/internal/query/openapi/paths/repository/dependencies.go"},
		{Method: "GET", Path: "/api/v0/graph/entities", Family: "graph_entities", Serve: w8bNarrowServeGated, Fragment: "go/internal/query/openapi/paths/search/graph_entities.go"},
		{Method: "GET", Path: "/api/v0/images", Family: "images", Serve: w8bNarrowServeGated, Fragment: "go/internal/query/openapi/paths/supply/chain/images.go"},
		{Method: "GET", Path: "/api/v0/images/tag-history", Family: "images", Serve: w8bNarrowServeGated, Fragment: "go/internal/query/openapi/paths/supply/chain/tag_history.go"},
		{Method: "GET", Path: "/api/v0/incidents/{incident_id}/context", Family: "incidents", Serve: w8bNarrowServeGated, Fragment: "go/internal/query/openapi/paths/evidence/incident_context.go"},
		{Method: "POST", Path: "/api/v0/runtime-declarations/query", Family: "runtime_declarations", Serve: w8bNarrowServeGated, Fragment: "go/internal/query/openapi/paths/repository/runtime_declarations.go"},
		{Method: "GET", Path: "/api/v0/cloud/inventory", Family: "cloud_inventory", Serve: w8bNarrowServeGated, Fragment: "go/internal/query/openapi/paths/cloud/inventory.go"},
		{Method: "GET", Path: "/api/v0/cloud/ssm-host-inventory", Family: "cloud_inventory", Serve: w8bNarrowServeGated, Fragment: "go/internal/query/openapi/paths/cloud/ssm_host_inventory.go"},
		{Method: "GET", Path: "/api/v0/service-catalog/correlations", Family: "service_catalog", Serve: w8bNarrowServeGated, Fragment: "go/internal/query/openapi/paths/service/catalog.go"},
		{Method: "GET", Path: "/api/v0/codeowners/ownership", Family: "codeowners", Serve: w8bNarrowServeGated, Fragment: "go/internal/query/openapi/paths/code/owners.go"},
	}

	if len(w8bNarrowWave) != len(want) {
		t.Fatalf("w8bNarrowWave rows = %d, want %d: the wave is frozen, move it deliberately", len(w8bNarrowWave), len(want))
	}
	for i, w := range want {
		if w8bNarrowWave[i] != w {
			t.Errorf("w8bNarrowWave[%d] = %+v, want %+v: move the wave, the ownership note, and the fragment together", i, w8bNarrowWave[i], w)
		}
	}
}
