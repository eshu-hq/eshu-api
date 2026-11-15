// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// W8b-narrow satellite termination (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#101). This file owns the frozen wave table:
// exactly the 15 narrow-family routes the fourteen versioned W8b
// narrow pins admit. The behavior lives in w8bnarrow_terminate.go;
// the proofs in w8bnarrow_conformance_test.go,
// w8bnarrow_gate_test.go, w8bnarrow_parity_test.go, and
// w8bnarrow_latency_test.go; the spec posture in
// openapi/spec_w8bnarrow_test.go; the record in
// contracts/w8b-narrow-termination.md.
//
// This lane is a NEW wave table, never an extension of w8aWave or
// driftWave: the W8b narrow pins cover their own 15 rows only, and
// no other family may cite a W8b narrow pin as coverage. Citing any
// W8a pin, the drift pin, or any code-search/graph pin as coverage
// here is void (each pin seals the other families out explicitly).
//
// The 2 drift routes (POST /api/v0/cloud/runtime-drift/findings,
// GET /api/v0/investigations/drift/packet) already terminated in the
// Wave-4 Drift lane under eshu-hq/eshu-api#117 and stay out of this
// table. The component-extensions pair and the 6 no-port routes
// (readiness x2, fact-schema x2, surface-inventory,
// visualizations/derive) carry no core mint and stay on the relay
// with the TODO naming the owed item (see
// contracts/w8b-narrow-termination.md).
//
// Pre-edit gate (MUST read before editing): entry point is
// Server.routes in api/router.go, which calls
// registerW8bNarrowWave ahead of the generic proxy; phase order is
// pin record first, wave registration second, parity/latency proofs
// third; data dependency is the fourteen v0 pins recorded
// satellite-side in contracts/w8b-narrow-termination.md plus the
// vendored upstream copies, each v0, exact method set, no transport
// constructor by design; re-trigger is dispatchNote/telemetry per
// family plus spec_scope and the disclosed absences that move with
// this lane.
//
// Production binding (carried from the D0 shim proof via the D2-D6
// waves, the Wave-7 G-a graph lane, and the W8a lane): dest owns the
// route (caller gate, family authorizer, response mapping) and calls
// the versioned core read API as its port transport — the shared-key
// credential and negotiation headers forwarded, no retry, no
// table/queue handles dest-side. Authz stays atomic core-side: dest
// allows only the shared-key (AllScopes) identity it can construct
// and core re-judges every forwarded call as backstop (tenant scope,
// permission catalog, governance mode — which is also where the live
// 403s and mode-dependent denials come from). The contract record is
// contracts/w8b-narrow-termination.md.
//
// Transport pins (recorded byte-identical at the core HEAD
// eshu-hq/eshu#76240fda9f, re-verified in-lane with zero drift since
// each mint): the fourteen narrow-family v0 pins — kubernetes and
// observability coverage correlations, work-item evidence, semantic
// evidence (covers BOTH semantic GETs), dependency, image, graph
// entity inventory, tag history, runtime declaration, cloud
// inventory, incident context, SSM host inventory, service catalog
// correlation, and codeowners ownership (see
// contracts/w8b-narrow-termination.md for the pin table). Each pin
// carries no transport constructor by design: dest serves the reads
// through the versioned forward transport only, and any other backend
// pairing needs a new arbiter ruling. Handler behavior stays
// satellite-side and unpinned: grant checks, fixed failure messages,
// packet bounds, QueryProfile. This lane cites no other pin, and no
// other family may cite these pins.
//
// The routes stay inside RequireAuth: the wave gate answers every
// credential shape (shared-key terminate, foreign-class relay,
// credential-less 401), so no keyless caller gains a new public
// surface here. There is no public carve-out in this lane.

// w8bNarrowServeGated is the only serve kind in this lane.
const w8bNarrowServeGated = "gated"

// w8bNarrowRoute is one frozen W8b-narrow entry: the exact surface
// this lane terminates. Serve tells the router how this lane serves
// it; Fragment names the core-side Decision 5(c) fragment the
// follow-up spec-port lane will vendor — this lane ports no fragment
// (the spec absences stay disclosed, see
// contracts/w8b-narrow-termination.md), so the field carries the core
// path, not a satellite one.
type w8bNarrowRoute struct {
	Method   string
	Path     string
	Family   string
	Serve    string
	Fragment string
}

// w8bNarrowWave is the frozen W8b-narrow termination target: exactly
// the 15 routes behind the fourteen versioned narrow pins, v0.
// TestW8bNarrowWaveFrozen pins it entry by entry; a wave change moves
// the ownership note, the fragment reference, and this mirror
// together.
var w8bNarrowWave = []w8bNarrowRoute{
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

// registerW8bNarrowWave mounts the W8b-narrow wave gate ahead of the
// generic proxy: a valid shared key terminates on the native lane; a
// foreign credential class (scoped token, OIDC bearer, session
// cookie) relays explicitly on the relay lane so core judges it;
// credential-less callers — and junk-cookie carriers, per the D2
// follow-up lesson — get the standing 401. Each trailing-slash
// spelling stays on the wave route for the same never-split reason,
// except the incident context route whose {incident_id} segment keeps
// a single spelling (the trailing-slash twin rides the same gate).
// Lives here (not in routes()) so the shared router stays under the
// file-size cap.
func (s *Server) registerW8bNarrowWave(mux *http.ServeMux) {
	for _, entry := range w8bNarrowWave {
		pattern := entry.Method + " " + entry.Path
		gated := s.instrumented(viaNative, s.w8bNarrowReadGate(http.HandlerFunc(s.serveW8bNarrowTerminated)))
		mux.Handle(pattern, gated)
		mux.Handle(pattern+"/", gated)
	}
}
