// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// Code-divergence relay-leg termination (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#125). This file owns the frozen wave
// table: exactly the 3 divergence POSTs the versioned divergence pin
// admits (the findings POST admits its bounded content reads through
// the five staying-port methods plus the wrapper and outlier graph
// tracks through the bounded Run; the investigate POST admits the
// same reads for one addressed finding; the report POST admits them
// for the rollup scan). Behavior lives in
// code_divergence_terminate.go; proofs in
// code_divergence_conformance_test.go and the per-family suites;
// spec posture in openapi/spec_divergence_test.go; the record in
// contracts/code-divergence-termination.md.
//
// This lane is a NEW wave table, never an extension of codeGraphWave
// or any other wave: the divergence pin covers its own 3 rows only,
// and no other family may cite the divergence pin as coverage.
//
// Pre-edit gate (MUST read before editing): entry point is
// Server.routes in api/router.go, which calls
// registerCodeDivergenceWave ahead of the generic proxy; phase order
// is pin record first, wave registration second, parity/latency
// proofs third; data dependency is the divergence pin at d584e38
// (recorded satellite-side in
// contracts/code-divergence-read-admission.md plus the vendored
// upstream copies, v0, exact 6-method set, no transport constructor
// by design); re-trigger is dispatchNote/telemetry per family plus
// spec_scope and the disclosed absences that move with this lane.
//
// Production binding (carried from the D0 shim proof via the D2-D6
// waves, the Wave-2 Content lane, the W4 code-search lane, and the
// Wave-7 G-a graph lane): dest owns the route (caller gate, family
// authorizer, response mapping) and calls the versioned core read
// API as its port transport — the shared-key credential and
// negotiation headers forwarded, no retry, no table/queue handles
// dest-side. Authz stays atomic core-side: dest allows only the
// shared-key (AllScopes) identity it can construct and core
// re-judges every forwarded call as backstop (tenant scope,
// permission catalog, governance mode — which is also where the
// live 403s and mode-dependent denials come from). The contract
// record is contracts/code-divergence-termination.md.
//
// Transport pin: the versioned divergence pin, v0 (recorded at the
// d584e38 core merge of eshu-hq/eshu#10650), covers all three wave
// rows — the five staying content reads plus the bounded graph
// traversal the wrapper-bypass and convention-outlier tracks read
// through. The pin carries no transport constructor by design: dest
// serves the reads through the existing graph reader plus the
// guarded Postgres content reader, the same pair cmd/api wiring
// builds today, and any other backend pairing needs a new arbiter
// ruling. Handler behavior stays satellite-side and unpinned: grant
// checks, the fixed failure messages, finding assembly, next-step
// synthesis, rollup caps, suppression and truncation rules,
// QueryProfile. This lane cites no other pin, and no other family
// may cite this pin.
//
// The routes stay inside RequireAuth: the wave gate answers every
// credential shape (shared-key terminate, foreign-class relay,
// credential-less 401), so no keyless caller gains a new public
// surface here. There is no public carve-out in this lane.

// codeDivergenceServeGated is the only serve kind in this lane.
const codeDivergenceServeGated = "gated"

// codeDivergenceRoute is one frozen divergence entry: the exact
// cross-repo surface this lane terminates. Serve tells the router how
// this lane serves it; Fragment names the core-side Decision 5(c)
// fragment the follow-up spec-port lane will vendor — this lane
// ports no fragment (the spec absences stay disclosed, see
// contracts/code-divergence-termination.md), so the field carries
// the core path, not a satellite one.
type codeDivergenceRoute struct {
	Method   string
	Path     string
	Family   string
	Serve    string
	Fragment string
}

// codeDivergenceWave is the frozen termination target: exactly the 3
// divergence POSTs behind the versioned divergence pin, v0.
// TestCodeDivergenceWaveFrozen pins it entry by entry; a wave change
// moves the ownership note, the fragment reference, and this mirror
// together.
var codeDivergenceWave = []codeDivergenceRoute{
	{Method: "POST", Path: "/api/v0/code/divergence/findings", Family: "code_divergence", Serve: codeDivergenceServeGated, Fragment: "go/internal/query/openapi/paths/code/divergence.go"},
	{Method: "POST", Path: "/api/v0/code/divergence/investigate", Family: "code_divergence", Serve: codeDivergenceServeGated, Fragment: "go/internal/query/openapi/paths/code/divergence.go"},
	{Method: "POST", Path: "/api/v0/code/divergence/report", Family: "code_divergence", Serve: codeDivergenceServeGated, Fragment: "go/internal/query/openapi/paths/code/divergence.go"},
}

// registerCodeDivergenceWave mounts the divergence wave gate ahead of
// the generic proxy: a valid shared key terminates on the native
// lane; a foreign credential class (scoped token, OIDC bearer,
// session cookie) relays explicitly on the relay lane so core judges
// it; credential-less callers — and junk-cookie carriers, per the D2
// follow-up lesson — get the standing 401. Each trailing-slash
// spelling stays on the wave route for the same never-split reason.
// Lives here (not in routes()) so the shared router stays under the
// file-size cap.
func (s *Server) registerCodeDivergenceWave(mux *http.ServeMux) {
	for _, entry := range codeDivergenceWave {
		pattern := entry.Method + " " + entry.Path
		gated := s.instrumented(viaNative, s.codeDivergenceReadGate(http.HandlerFunc(s.serveCodeDivergenceTerminated)))
		mux.Handle(pattern, gated)
		mux.Handle(pattern+"/", gated)
	}
}
