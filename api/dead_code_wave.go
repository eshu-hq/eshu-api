// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// Dead-code relay-leg termination (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#124). This file owns the frozen wave
// table: exactly the 3 dead-code POSTs the versioned dead-code pin
// admits (the scan POST admits its candidate scan plus incoming-edge
// probe plus hydration plus coverage through the pin reads; the
// cross-repo POST admits its producer scan plus consumer-evidence,
// coverage, and root-path reads plus the boundary-evidence read; the
// investigate POST admits its candidate scan plus hydration plus
// coverage plus maturity reads through the same pin). Behavior lives in
// dead_code_terminate.go; proofs in dead_code_conformance_test.go
// and the per-family suites; spec posture in
// openapi/spec_dead_code_test.go; the record in
// contracts/code-dead-code-termination.md.
//
// This lane is a NEW wave table, never an extension of w8aWave: the
// W8a pins cover their own 16 rows only, and no other family may cite
// the dead-code pin as coverage. Citing CodeGraphReadAdmission v0,
// CodeSearchReadAdmission v0, or ContentSearchReadAdmission v0 as
// coverage here is void (those pins seal the dead-code graph and
// content legs out explicitly).
//
// Pre-edit gate (MUST read before editing): entry point is
// Server.routes in api/router.go, which calls registerDeadCodeWave
// ahead of the generic proxy; phase order is pin record first, wave
// registration second, parity/latency proofs third; data dependency
// is the dead-code pin at 589d55e (recorded satellite-side in
// contracts/code-dead-code-read-admission.md plus the vendored upstream
// copies, v0, exact 14-method set, no transport constructor by
// design); re-trigger is dispatchNote/telemetry per family plus
// spec_scope and the disclosed absences that move with this lane.
//
// Production binding (carried from the D0 shim proof via the D2-D6
// waves, the Wave-2 Content lane, the W4 code-search lane, and the
// Wave-7 G-a lane): dest owns the route (caller gate, family
// authorizer, response mapping) and calls the versioned core read API
// as its port transport — the shared-key credential and negotiation
// headers forwarded, no retry, no table/queue handles dest-side. Authz
// stays atomic core-side: dest allows only the shared-key (AllScopes)
// identity it can construct and core re-judges every forwarded call
// as backstop (tenant scope, permission catalog, governance mode —
// which is also where the live 403s and mode-dependent denials come
// from). The contract record is
// contracts/code-dead-code-termination.md.
//
// Transport pin: the versioned dead-code pin, v0 (recorded at the
// dead-code mint 589d55e), covers all three wave rows — the graph run
// behind the SQL-routine EXECUTES probes plus the candidate,
// incoming-edge, hydration, coverage, index-freshness, cross-repo
// consumer-evidence, and boundary-evidence reads the three routes
// admit through. The pin carries no transport constructor by design:
// dest serves the reads through the existing graph reader plus the
// guarded Postgres content reader, the same pair cmd/api wiring
// builds today, and any other backend pairing needs a new arbiter
// ruling. Handler behavior stays satellite-side and unpinned: grant
// checks, the repository selector, fixed failure messages, rerank,
// and QueryProfile. This lane cites no other pin, and no other family
// may cite this pin.
//
// The routes stay inside RequireAuth: the wave gate answers every
// credential shape (shared-key terminate, foreign-class relay,
// credential-less 401), so no keyless caller gains a new public
// surface here. There is no public carve-out in this lane.

// deadCodeServeGated is the only serve kind in this lane.
const deadCodeServeGated = "gated"

// deadCodeRoute is one frozen dead-code entry: the exact surface
// this lane terminates. Serve tells the router how this lane serves
// it; Fragment names the core-side Decision 5(c) fragment the
// follow-up spec-port lane will vendor — this lane ports no fragment
// (the spec absences stay disclosed, see
// contracts/code-dead-code-termination.md), so the field carries the
// core path, not a satellite one.
type deadCodeRoute struct {
	Method   string
	Path     string
	Family   string
	Serve    string
	Fragment string
}

// deadCodeWave is the frozen dead-code termination target: exactly the 3
// dead-code POSTs behind the versioned dead-code pin, v0.
// TestDeadCodeWaveFrozen pins it entry by entry; a wave change moves
// the ownership note, the fragment reference, and this mirror
// together.
var deadCodeWave = []deadCodeRoute{
	{Method: "POST", Path: "/api/v0/code/dead-code", Family: "code_dead_code", Serve: deadCodeServeGated, Fragment: "go/internal/query/openapi/paths/code/dead/scan.go"},
	{Method: "POST", Path: "/api/v0/code/dead-code/cross-repo", Family: "code_dead_code", Serve: deadCodeServeGated, Fragment: "go/internal/query/openapi/paths/code/dead/cross_repo.go"},
	{Method: "POST", Path: "/api/v0/code/dead-code/investigate", Family: "code_dead_code", Serve: deadCodeServeGated, Fragment: "go/internal/query/openapi/paths/code/dead/investigation.go"},
}

// registerDeadCodeWave mounts the dead-code wave gate ahead of the
// generic proxy: a valid shared key terminates on the native lane; a
// foreign credential class (scoped token, OIDC bearer, session cookie)
// relays explicitly on the relay lane so core judges it;
// credential-less callers — and junk-cookie carriers, per the D2
// follow-up lesson — get the standing 401. Each trailing-slash
// spelling stays on the wave route for the same never-split reason.
// Lives here (not in routes()) so the shared router stays under the
// file-size cap.
func (s *Server) registerDeadCodeWave(mux *http.ServeMux) {
	for _, entry := range deadCodeWave {
		pattern := entry.Method + " " + entry.Path
		gated := s.instrumented(viaNative, s.deadCodeReadGate(http.HandlerFunc(s.serveDeadCodeTerminated)))
		mux.Handle(pattern, gated)
		mux.Handle(pattern+"/", gated)
	}
}
