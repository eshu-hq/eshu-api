// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// Code-graph relay-leg termination (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#121). This file owns the frozen wave
// table: exactly the 4 G-a graph POSTs the versioned graph pin
// admits (the call-chain POST admits its bounded traversal through
// the bounded read plus the cross_repo endpoint selectors; the
// compare POST admits its bounded path enumeration through the same
// traversal read plus the truncation/visited budget; the metrics POST
// admits its exact metric build through the bounded edge scan with
// the 422 fail-closed bound; the routes/callers POST admits its
// exact route trace through the same traversal read plus the
// 409-ambiguity selector rule). Behavior lives in
// code_graph_terminate.go; proofs in code_graph_conformance_test.go
// and the per-family suites; spec posture in
// openapi/spec_graph_test.go; the record in
// contracts/code-graph-termination.md.
//
// This lane is a NEW wave table, never an extension of w8aWave: the
// W8a pins cover their own 16 rows only, and no other family may cite
// the graph pin as coverage. Citing MetricsReadAdmission v0 as
// coverage for the metrics POST here is void (that pin seals the
// graph leg out explicitly).
//
// Pre-edit gate (MUST read before editing): entry point is
// Server.routes in api/router.go, which calls registerCodeGraphWave
// ahead of the generic proxy; phase order is pin record first, wave
// registration second, parity/latency proofs third; data dependency
// is the G-a pin at 0fd6db6 (recorded satellite-side in
// contracts/code-graph-read-admission.md plus the vendored upstream
// copies, v0, exact 3-method set, no transport constructor by
// design); re-trigger is dispatchNote/telemetry per family plus
// spec_scope and the disclosed absences that move with this lane.
//
// Production binding (carried from the D0 shim proof via the D2-D6
// waves, the Wave-2 Content lane, and the W4 code-search lane): dest
// owns the route (caller gate, family authorizer, response mapping)
// and calls the versioned core read API as its port transport — the
// shared-key credential and negotiation headers forwarded, no retry,
// no table/queue handles dest-side. Authz stays atomic core-side:
// dest allows only the shared-key (AllScopes) identity it can
// construct and core re-judges every forwarded call as backstop
// (tenant scope, permission catalog, governance mode — which is also
// where the live 403s and mode-dependent denials come from). The
// contract record is contracts/code-graph-termination.md.
//
// Transport pin: the versioned graph pin, v0 (recorded at the G-a
// mint 0fd6db6), covers all four wave rows — the bounded traversal
// plus the entity-name resolution the four routes admit through. The
// pin carries no transport constructor by design: dest serves the
// reads through the existing graph reader plus the guarded Postgres
// content reader, the same pair cmd/api wiring builds today, and any
// other backend pairing needs a new arbiter ruling. Handler behavior
// stays satellite-side and unpinned: grant checks, the
// unsupported-language short-circuit, fixed failure messages,
// endpoint disambiguation, packet bounds, the truth envelope,
// QueryProfile. This lane cites no other pin, and no other family
// may cite this pin.
//
// The routes stay inside RequireAuth: the wave gate answers every
// credential shape (shared-key terminate, foreign-class relay,
// credential-less 401), so no keyless caller gains a new public
// surface here. There is no public carve-out in this lane.

// codeGraphServeGated is the only serve kind in this lane.
const codeGraphServeGated = "gated"

// codeGraphRoute is one frozen G-a entry: the exact cross-repo
// surface this lane terminates. Serve tells the router how this lane
// serves it; Fragment names the core-side Decision 5(c) fragment the
// follow-up spec-port lane will vendor — this lane ports no fragment
// (the spec absences stay disclosed, see
// contracts/code-graph-termination.md), so the field carries the core
// path, not a satellite one.
type codeGraphRoute struct {
	Method   string
	Path     string
	Family   string
	Serve    string
	Fragment string
}

// codeGraphWave is the frozen G-a termination target: exactly the 4
// graph POSTs behind the versioned graph pin, v0.
// TestCodeGraphWaveFrozen pins it entry by entry; a wave change moves
// the ownership note, the fragment reference, and this mirror
// together.
var codeGraphWave = []codeGraphRoute{
	{Method: "POST", Path: "/api/v0/code/call-chain", Family: "code_graph", Serve: codeGraphServeGated, Fragment: "go/internal/query/openapi/paths/code/routes.go"},
	{Method: "POST", Path: "/api/v0/code/call-chain/compare", Family: "code_graph", Serve: codeGraphServeGated, Fragment: "go/internal/query/openapi/paths/code/routes.go"},
	{Method: "POST", Path: "/api/v0/code/call-graph/metrics", Family: "code_graph", Serve: codeGraphServeGated, Fragment: "go/internal/query/openapi/paths/code/symbols.go"},
	{Method: "POST", Path: "/api/v0/code/routes/callers", Family: "code_graph", Serve: codeGraphServeGated, Fragment: "go/internal/query/openapi/paths/code/route_to_caller.go"},
}

// registerCodeGraphWave mounts the G-a code-graph wave gate ahead of
// the generic proxy: a valid shared key terminates on the native
// lane; a foreign credential class (scoped token, OIDC bearer,
// session cookie) relays explicitly on the relay lane so core judges
// it; credential-less callers — and junk-cookie carriers, per the D2
// follow-up lesson — get the standing 401. Each trailing-slash
// spelling stays on the wave route for the same never-split reason.
// Lives here (not in routes()) so the shared router stays under the
// file-size cap.
func (s *Server) registerCodeGraphWave(mux *http.ServeMux) {
	for _, entry := range codeGraphWave {
		pattern := entry.Method + " " + entry.Path
		gated := s.instrumented(viaNative, s.codeGraphReadGate(http.HandlerFunc(s.serveCodeGraphTerminated)))
		mux.Handle(pattern, gated)
		mux.Handle(pattern+"/", gated)
	}
}
