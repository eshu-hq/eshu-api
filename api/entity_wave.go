// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// Entity relay-leg termination (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#97). This file owns the frozen wave table:
// exactly the 9 W5 entity routes the versioned entity pin admits:
// 7 terminate natively (the resolve POST, the workload/service context
// GETs, the workload/service story GETs, and the service api-surface
// and downstream-consumers GETs) plus 2 explicit relay holds (the
// entity context fallback leg, whose builder seam core has not made
// satellite-consumable, and the investigations service leg, pending a
// pin extension). Behavior lives in
// entity_terminate.go; proofs in entity_conformance_test.go,
// entity_parity_test.go, and entity_gate_test.go; spec posture stays
// disclosed (no openapi port in this lane); the record in
// contracts/entity-termination.md.
//
// This lane is a NEW wave table, never an extension of any earlier
// wave: the Entity pin covers its own 9 rows (plus the cited W5c-e
// TraceContext leg, which rides the impact lane's dual-pin cite), and
// no other family may cite the Entity pin as coverage. Citing
// CoreReadContractsVersion or SupplyChainReadAdmission as W5 coverage
// here is void (those pins grant nothing to this family).
//
// Pre-edit gate (MUST read before editing): entry point is
// Server.routes in api/router.go, which calls registerEntityWave ahead
// of the generic proxy; phase order is pin record first, wave
// registration second, parity proofs third; data dependency is the
// Entity pin at 76240fda9f (recorded satellite-side in
// contracts/entity-read-admission.md plus the vendored upstream
// copies, v0, exact 27-method set, no transport constructor by design);
// re-trigger is dispatchNote/telemetry per family plus spec_scope and
// the disclosed absences that move with this lane.
//
// Production binding (carried from the D0 shim proof through the D2-D6
// waves, the Wave-2 Content lane, the W4 lanes, the Wave-7 G-a graph
// lane, and the G-b relationship lane): dest owns the route (caller
// gate, family authorizer, response mapping) and calls the versioned
// core read API as its port transport — the shared-key credential and
// negotiation headers forwarded, no retry, no table/queue handles
// dest-side. Authz stays atomic core-side: dest allows only the
// shared-key (AllScopes) identity it can construct and core re-judges
// every forwarded call as backstop (tenant scope, permission catalog,
// governance mode — which is also where the live 403s and
// mode-dependent denials come from). The contract record is
// contracts/entity-termination.md.
//
// Transport pin: the versioned Entity pin, v0 (recorded at the W5 mint
// 76240fda9f), covers all 9 wave rows — the resolve POST, the
// context/story GETs, the service enrichment GETs, and the graph
// entities GET. The pin carries no transport constructor by design:
// dest serves the reads through the existing graph reader plus the
// guarded Postgres content reader plus the reducer-owned stores, the
// same backends cmd/api wiring builds today, and any other backend
// pairing needs a new arbiter ruling. Handler behavior stays
// satellite-side and unpinned: grant checks, the repository selector,
// fixed failure messages, QueryProfile. This lane cites no other pin,
// and no other family may cite this pin (beyond the explicit impact
// TraceContext dual-pin cite recorded in both pins).
//
// The routes stay inside RequireAuth: the wave gate answers every
// credential shape (shared-key terminate, foreign-class relay,
// credential-less 401), so no keyless caller gains a new public
// surface here. There is no public carve-out in this lane.

// entityServeGated terminates natively; entityServeRelayHold rides the
// explicit relay stack (never the catch-all) until the owed core
// satisfier lands.
const (
	entityServeGated     = "gated"
	entityServeRelayHold = "relay_hold"
)

// entityRoute is one frozen W5 entity entry: the exact surface this
// lane terminates or holds. Serve tells the router how this lane
// serves it; Fragment names the core-side Decision 5(c) fragment the
// follow-up spec-port lane will vendor — this lane ports no fragment
// (the spec absences stay disclosed, see
// contracts/entity-termination.md), so the field carries the core
// path, not a satellite one.
type entityRoute struct {
	Method   string
	Path     string
	Family   string
	Serve    string
	Fragment string
}

// entityWave is the frozen W5 entity termination target: exactly the 9
// entity routes behind the versioned entity pin, v0, 7 terminating
// natively and 2 on explicit relay hold (the context fallback leg and
// the investigations service leg, pending the owed core seams).
// TestEntityWaveFrozen pins it entry by entry; a wave change moves the
// ownership note, the fragment reference, and this mirror together.
var entityWave = []entityRoute{
	{Method: "POST", Path: "/api/v0/entities/resolve", Family: "entities", Serve: entityServeGated, Fragment: "go/internal/query/openapi/paths/search/entities.go"},
	{Method: "GET", Path: "/api/v0/entities/{entity_id}/context", Family: "entities", Serve: entityServeRelayHold, Fragment: "go/internal/query/openapi/paths/search/entities.go"},
	{Method: "GET", Path: "/api/v0/workloads/{workload_id}/context", Family: "workloads", Serve: entityServeGated, Fragment: "go/internal/query/openapi/paths/search/entities.go"},
	{Method: "GET", Path: "/api/v0/workloads/{workload_id}/story", Family: "workloads", Serve: entityServeGated, Fragment: "go/internal/query/openapi/paths/search/entities.go"},
	{Method: "GET", Path: "/api/v0/services/{service_name}/context", Family: "service_intel", Serve: entityServeGated, Fragment: "go/internal/query/openapi/paths/search/entities.go"},
	{Method: "GET", Path: "/api/v0/services/{service_name}/story", Family: "service_intel", Serve: entityServeGated, Fragment: "go/internal/query/openapi/paths/search/entities.go"},
	{Method: "GET", Path: "/api/v0/services/{service_name}/api-surface/endpoints", Family: "service_intel", Serve: entityServeGated, Fragment: "go/internal/query/openapi/paths/search/entities.go"},
	{Method: "GET", Path: "/api/v0/services/{service_name}/downstream-consumers", Family: "service_intel", Serve: entityServeGated, Fragment: "go/internal/query/openapi/paths/search/entities.go"},
	{Method: "GET", Path: "/api/v0/investigations/services/{service_name}", Family: "service_intel", Serve: entityServeRelayHold, Fragment: "go/internal/query/openapi/paths/search/entities.go"},
}

// registerEntityWave mounts the entity wave ahead of the generic
// proxy: gated rows terminate on the native lane for a valid shared
// key and relay explicitly for foreign credential classes; the
// relay_hold row rides the explicit relay stack on the relay lane for
// every caller shape (pinned by TestEntityWaveAheadOfRelay), with the
// standing 401 for credential-less callers. Each trailing-slash
// spelling stays on the wave route for the same never-split reason.
// Lives here (not in routes()) so the shared router stays under the
// file-size cap.
func (s *Server) registerEntityWave(mux *http.ServeMux) {
	for _, entry := range entityWave {
		pattern := entry.Method + " " + entry.Path
		if entry.Serve == entityServeRelayHold {
			// TODO-eshu-9669-w5: terminate GET
			// /api/v0/entities/{entity_id}/context natively once core
			// provides a satellite-consumable satisfier for
			// ContentRelationshipBuilder.BuildContentRelationships, and
			// GET /api/v0/investigations/services/{service_name} once
			// core extends the versioned entity pin with the three
			// investigations reads (ListRepoEntitiesByTypes,
			// RepositoryReadModelSummary, SearchFileReferenceAnyRepo);
			// until then these legs relay explicitly (never the
			// catch-all) so each hold stays observable in telemetry
			// and the spec absence stays disclosed.
			relayed := s.instrumented(viaRelay, RequireAuth(s.apiKey, http.HandlerFunc(s.dispatch)))
			mux.Handle(pattern, relayed)
			mux.Handle(pattern+"/", relayed)
			continue
		}
		gated := s.instrumented(viaNative, s.entityReadGate(http.HandlerFunc(s.serveEntityTerminated)))
		mux.Handle(pattern, gated)
		mux.Handle(pattern+"/", gated)
	}
}
