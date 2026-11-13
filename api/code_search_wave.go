// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// Code-search relay-leg termination (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#113). This file owns the frozen wave
// table: exactly the 2 code search POSTs the W4 pin
// CodeSearchReadAdmission v0 admits (core merge eshu-hq/eshu#10540 at
// e504e328a7 — the repo-selected graph lane plus the global and
// fallback content reads for /code/search, and the symbol-aware read
// plus the plain-name fallback for /code/symbols/search). Behavior
// lives in code_search_terminate.go; proofs in
// code_search_conformance_test.go and the per-family suites; spec
// posture in openapi/spec_code_search_test.go; the record in
// contracts/code-search-termination.md.
//
// This lane is a NEW wave table, never an extension of codeFlowWave:
// the W6a flow pin covers flow reads only, and no other family may
// cite the code search pin as coverage.
//
// Pre-edit gate: entry point is Server.routes in api/router.go, which
// calls registerCodeSearchWave ahead of the generic proxy; phase order
// is pin record first, wave registration second, parity/latency proofs
// third; data dependency is the W4 pin at e504e328a7 (recorded
// satellite-side in contracts/code-search-read-admission.md, v0, exact
// 9-method set, no transport constructor by design); re-trigger is
// dispatchNote/telemetry per family plus spec_scope and the disclosed
// absences that move with this wave.
//
// Production binding (carried from the D0 shim proof via the D2-D6
// waves, the Wave-2 Content lane, and the W6a Code-flow lane): dest
// owns the route (caller gate, family authorizer, response mapping)
// and calls the versioned core read API as its port transport — the
// shared-key credential and negotiation headers forwarded, no retry, no
// table/queue handles dest-side. Authz stays atomic core-side: dest
// allows only the shared-key (AllScopes) identity it can construct and
// core re-judges every forwarded call as backstop (tenant scope,
// permission catalog, governance mode — which is also where the live
// 403s and mode-dependent denials come from). The contract record is
// contracts/code-search-termination.md.
//
// Transport pin: CodeSearchReadAdmission v0
// (contracts/code-search-read-admission.md, recorded at the W4 base
// e504e328a7) covers both wave rows — the graph lane plus the guarded
// content-reader legs the two routes admit through. The pin carries no
// transport constructor by design: dest serves the reads through the
// existing graph reader (satisfying Run) plus the guarded Postgres
// content reader (satisfying the eight content reads), the same pair
// cmd/api wiring builds today; any other backend pairing needs a new
// arbiter ruling. Handler behavior
// stays satellite-side and unpinned: grant checks, repository
// selector, fixed failure messages, hybrid rerank, semantic overlay,
// QueryProfile. This lane cites no other pin, and no other family may
// cite this pin.
//
// The routes stay inside RequireAuth: the wave gate answers every
// credential shape (shared-key terminate, foreign-class relay,
// credential-less 401), so no keyless caller gains a new public
// surface here. There is no public carve-out in this lane.

// codeSearchServeGated is the only serve kind in this lane.
const codeSearchServeGated = "gated"

// codeSearchRoute is one frozen W4 entry: the exact cross-repo surface
// this lane terminates. Serve tells the router how this lane serves
// it; Fragment names the core-side Decision 5(c) fragment the
// follow-up spec-port lane will vendor — this lane ports no fragment
// (the spec absences stay disclosed, see
// contracts/code-search-termination.md), so the field carries the core
// path, not a satellite one.
type codeSearchRoute struct {
	Method   string
	Path     string
	Family   string
	Serve    string
	Fragment string
}

// codeSearchWave is the frozen W4 termination target: exactly the 2
// code search POSTs behind CodeSearchReadAdmission v0.
// TestCodeSearchWaveFrozen pins it entry by entry; a wave change moves
// the ownership note, the fragment reference, and this mirror
// together.
var codeSearchWave = []codeSearchRoute{
	{Method: "POST", Path: "/api/v0/code/search", Family: "code_search", Serve: codeSearchServeGated, Fragment: "go/internal/query/openapi/paths/code/routes.go"},
	{Method: "POST", Path: "/api/v0/code/symbols/search", Family: "code_search", Serve: codeSearchServeGated, Fragment: "go/internal/query/openapi/paths/code/symbols.go"},
}

// registerCodeSearchWave mounts the W4 code search wave gate ahead of
// the generic proxy: a valid shared key terminates on the native lane;
// a foreign credential class (scoped token, OIDC bearer, session
// cookie) relays explicitly on the relay lane so core judges it;
// credential-less callers — and junk-cookie carriers, per the D2
// follow-up lesson — get the standing 401. Each trailing-slash
// spelling stays on the wave route for the same never-split reason.
// Lives here (not in routes()) so the shared router stays under the
// file-size cap.
func (s *Server) registerCodeSearchWave(mux *http.ServeMux) {
	for _, entry := range codeSearchWave {
		pattern := entry.Method + " " + entry.Path
		gated := s.instrumented(viaNative, s.codeSearchReadGate(http.HandlerFunc(s.serveCodeSearchTerminated)))
		mux.Handle(pattern, gated)
		mux.Handle(pattern+"/", gated)
	}
}
