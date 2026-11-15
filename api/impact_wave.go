// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// Impact relay-leg termination (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#97). This file owns the frozen wave table:
// exactly the 13 W5 impact POSTs the versioned impact pin admits (the
// trace, blast-radius, change-surface, contracts, config-influence,
// exposure, and rest legs), with the two code-surface adapter legs on
// explicit relay hold. Behavior lives in impact_terminate.go; proofs
// in impact_conformance_test.go, impact_parity_test.go, and
// impact_gate_test.go; spec posture stays disclosed (no openapi port
// in this lane); the record in contracts/impact-termination.md.
//
// This lane is a NEW wave table, never an extension of any earlier
// wave: the Impact pin covers its own 13 rows only, and no other
// family may cite the Impact pin as coverage. Citing Entity,
// CodeGraph, IaC, or CoreReadContractsVersion here is void (those pins
// grant nothing to this family beyond the explicit TraceContext
// dual-pin cite, which this lane records rather than assumes: the
// trace legs' enrichment reads ride the Entity pin, and this lane
// forwards them opaquely like every other byte).
//
// Pre-edit gate (MUST read before editing): entry point is
// Server.routes in api/router.go, which calls registerImpactWave ahead
// of the generic proxy; phase order is pin record first, wave
// registration second, parity proofs third; data dependency is the
// Impact pin at 76240fda9f (recorded satellite-side in
// contracts/impact-read-admission.md plus the vendored upstream
// copies, v0, exact 14-method set, no transport constructor by design)
// plus the Entity pin for the two trace legs' TraceContext enrichment
// (dual-pin cite, contracts/entity-read-admission.md); re-trigger is
// dispatchNote/telemetry per family plus spec_scope and the disclosed
// absences that move with this lane.
//
// Production binding (carried from the D0 shim proof through the D2-D6
// waves, the Wave-2 Content lane, the W4 lanes, the Wave-7 G-a graph
// lane, the G-b relationship lane, and the W5 entity lane): dest owns
// the route (caller gate, family authorizer, response mapping) and
// calls the versioned core read API as its port transport — the
// shared-key credential and negotiation headers forwarded, no retry,
// no table/queue handles dest-side. Authz stays atomic core-side: dest
// allows only the shared-key (AllScopes) identity it can construct and
// core re-judges every forwarded call as backstop (tenant scope,
// permission catalog, governance mode — which is also where the live
// 403s and mode-dependent denials come from). The contract record is
// contracts/impact-termination.md.
//
// Transport pin: the versioned Impact pin, v0 (recorded at the W5 mint
// 76240fda9f), covers all 13 wave rows — the trace/blast-radius/
// change-surface/contracts/config-influence/exposure/rest reads. The
// pin carries no transport constructor by design: dest serves the reads
// through the existing graph reader, content reader, code-topic
// investigator, and live/ECS stores, the same backends cmd/api wiring
// builds today, and any other backend pairing needs a new arbiter
// ruling. Handler behavior stays satellite-side and unpinned: grant
// checks, grant-before-limit, scoped-token node-by-node filtering,
// fixed failure messages, QueryProfile. The change-surface adapter
// seams (FetchCodeSurface) and the trace seams
// (FetchServiceTraceContext, BuildServiceDeploymentOverview) stay
// core-side: the topic search the code-surface leg exercises IS frozen
// in the pin, and the service-story closure the trace legs pull IS
// frozen in the cited Entity pin.
//
// The routes stay inside RequireAuth: the wave gate answers every
// credential shape (shared-key terminate, foreign-class relay,
// credential-less 401), so no keyless caller gains a new public
// surface here. There is no public carve-out in this lane.

// impactServeGated terminates natively; impactServeRelayHold rides the
// explicit relay stack (never the catch-all) until the owed core seam
// lands.
const (
	impactServeGated     = "gated"
	impactServeRelayHold = "relay_hold"
)

// impactRoute is one frozen W5 impact entry: the exact surface this
// lane terminates or holds. Serve tells the router how this lane
// serves it; Fragment names the core-side Decision 5(c) fragment the
// follow-up spec-port lane will vendor — this lane ports no fragment
// (the spec absences stay disclosed, see
// contracts/impact-termination.md), so the field carries the core
// path, not a satellite one.
type impactRoute struct {
	Method   string
	Path     string
	Family   string
	Serve    string
	Fragment string
}

// impactWave is the frozen W5 impact termination target: exactly the
// 13 impact POSTs behind the versioned impact pin, v0, 11 terminating
// natively and 2 (the code-surface adapter legs) on relay hold.
// TestImpactWaveFrozen pins it entry by entry; a wave change moves the
// ownership note, the fragment reference, and this mirror together.
var impactWave = []impactRoute{
	{Method: "POST", Path: "/api/v0/impact/trace-deployment-chain", Family: "impact", Serve: impactServeGated, Fragment: "go/internal/query/openapi/paths/impact/routes.go"},
	{Method: "POST", Path: "/api/v0/impact/blast-radius", Family: "impact", Serve: impactServeGated, Fragment: "go/internal/query/openapi/paths/impact/routes.go"},
	{Method: "POST", Path: "/api/v0/impact/change-surface", Family: "impact", Serve: impactServeRelayHold, Fragment: "go/internal/query/openapi/paths/impact/routes.go"},
	{Method: "POST", Path: "/api/v0/impact/change-surface/investigate", Family: "impact", Serve: impactServeRelayHold, Fragment: "go/internal/query/openapi/paths/impact/routes.go"},
	{Method: "POST", Path: "/api/v0/impact/contracts", Family: "impact", Serve: impactServeGated, Fragment: "go/internal/query/openapi/paths/impact/contract.go"},
	{Method: "POST", Path: "/api/v0/impact/deployment-config-influence", Family: "impact", Serve: impactServeGated, Fragment: "go/internal/query/openapi/paths/impact/deployment_config_influence.go"},
	{Method: "POST", Path: "/api/v0/impact/trace-exposure-path", Family: "impact", Serve: impactServeGated, Fragment: "go/internal/query/openapi/paths/impact/exposure.go"},
	{Method: "POST", Path: "/api/v0/impact/pre-change", Family: "impact", Serve: impactServeGated, Fragment: "go/internal/query/openapi/paths/impact/rest.go"},
	{Method: "POST", Path: "/api/v0/impact/developer-change-plan", Family: "impact", Serve: impactServeGated, Fragment: "go/internal/query/openapi/paths/impact/rest.go"},
	{Method: "POST", Path: "/api/v0/impact/entity-map", Family: "impact", Serve: impactServeGated, Fragment: "go/internal/query/openapi/paths/impact/rest.go"},
	{Method: "POST", Path: "/api/v0/impact/resource-investigation", Family: "impact", Serve: impactServeGated, Fragment: "go/internal/query/openapi/paths/impact/rest.go"},
	{Method: "POST", Path: "/api/v0/impact/trace-resource-to-code", Family: "impact", Serve: impactServeGated, Fragment: "go/internal/query/openapi/paths/impact/rest.go"},
	{Method: "POST", Path: "/api/v0/impact/explain-dependency-path", Family: "impact", Serve: impactServeGated, Fragment: "go/internal/query/openapi/paths/impact/rest.go"},
}

// registerImpactWave mounts the impact wave ahead of the generic
// proxy: gated rows terminate on the native lane for a valid shared
// key and relay explicitly for foreign credential classes; the two
// relay_hold adapter rows ride the explicit relay stack on the relay
// lane for every caller shape (pinned by TestImpactWaveAheadOfRelay),
// with the standing 401 for credential-less callers. Each
// trailing-slash spelling stays on the wave route for the same
// never-split reason. Lives here (not in routes()) so the shared
// router stays under the file-size cap.
func (s *Server) registerImpactWave(mux *http.ServeMux) {
	for _, entry := range impactWave {
		pattern := entry.Method + " " + entry.Path
		if entry.Serve == impactServeRelayHold {
			// TODO-eshu-9669-w5: terminate POST
			// /api/v0/impact/change-surface and POST
			// /api/v0/impact/change-surface/investigate natively once
			// core provides a satellite-consumable seam for
			// ChangeSurfaceCodeBackend.FetchCodeSurface; until then
			// these legs relay explicitly (never the catch-all) so
			// the holds stay observable in telemetry and the spec
			// absences stay disclosed.
			relayed := s.instrumented(viaRelay, RequireAuth(s.apiKey, http.HandlerFunc(s.dispatch)))
			mux.Handle(pattern, relayed)
			mux.Handle(pattern+"/", relayed)
			continue
		}
		gated := s.instrumented(viaNative, s.impactReadGate(http.HandlerFunc(s.serveImpactTerminated)))
		mux.Handle(pattern, gated)
		mux.Handle(pattern+"/", gated)
	}
}
