// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// Divergence relay-leg termination (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#125). This file owns the frozen wave
// table: exactly the 3 divergence POSTs the versioned divergence pin
// admits (the findings POST admits its phase-one stat scan plus member
// hydration plus the wrapper and outlier graph tracks through the pin
// reads; the investigate POST admits its single-finding lookup plus
// the wrapper-target and outlier-cohort qualification through the
// same pin; the report POST admits its per-kind counts plus top
// findings plus both graph tracks through the same pin). Behavior
// lives in divergence_terminate.go; proofs in
// divergence_conformance_test.go and the per-family suites; spec
// posture in openapi/spec_divergence_test.go; the record in
// contracts/code-divergence-termination.md.
//
// This lane is a NEW wave table, never an extension of
// codeGraphWave: the G-a pin covers its own 4 rows only, and no other
// family may cite the divergence pin as coverage. Citing
// CodeGraphReadAdmission v0, CodeSearchReadAdmission v0, or
// ContentSearchReadAdmission v0 as coverage here is void (those pins
// seal the divergence content and graph legs out explicitly).
//
// Pre-edit gate (MUST read before editing): entry point is
// Server.routes in api/router.go, which calls registerDivergenceWave
// ahead of the generic proxy; phase order is pin record first, wave
// registration second, parity/latency proofs third; data dependency
// is the divergence pin at d584e38 (recorded satellite-side in
// contracts/code-divergence-read-admission.md plus the vendored
// upstream copies, v0, exact 6-method set, no transport constructor
// by design); re-trigger is dispatchNote/telemetry per family plus
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
// contracts/code-divergence-termination.md.
//
// Transport pin: the versioned divergence pin, v0 (recorded at the
// divergence mint d584e38), covers all three wave rows — the five
// content reads (group stats, members, members-by-entity,
// drifted stats, drifted rows) plus the bounded graph traversal (Run)
// the wrapper-bypass and convention-outlier tracks read through. The
// pin carries no transport constructor by design: dest serves the
// reads through the existing graph reader plus the guarded Postgres
// content reader, the same pair cmd/api wiring builds today, and any
// other backend pairing needs a new arbiter ruling. Handler behavior
// stays satellite-side and unpinned: grant checks, the repository
// selector, fixed failure messages, suppression assembly, and
// QueryProfile. This lane cites no other pin, and no other family
// may cite this pin.
//
// The routes stay inside RequireAuth: the wave gate answers every
// credential shape (shared-key terminate, foreign-class relay,
// credential-less 401), so no keyless caller gains a new public
// surface here. There is no public carve-out in this lane.

// divergenceServeGated is the only serve kind in this lane.
const divergenceServeGated = "gated"

// divergenceRoute is one frozen divergence entry: the exact surface
// this lane terminates. Serve tells the router how this lane serves
// it; Fragment names the core-side Decision 5(c) fragment the
// follow-up spec-port lane will vendor — this lane ports no fragment
// (the spec absences stay disclosed, see
// contracts/code-divergence-termination.md), so the field carries the
// core path, not a satellite one.
type divergenceRoute struct {
	Method   string
	Path     string
	Family   string
	Serve    string
	Fragment string
}

// divergenceWave is the frozen divergence termination target:
// exactly the 3 divergence POSTs behind the versioned divergence
// pin, v0. TestDivergenceWaveFrozen pins it entry by entry; a wave
// change moves the ownership note, the fragment reference, and this
// mirror together.
var divergenceWave = []divergenceRoute{
	{Method: "POST", Path: "/api/v0/code/divergence/findings", Family: "code_divergence", Serve: divergenceServeGated, Fragment: "go/internal/query/openapi/paths/code/divergence.go"},
	{Method: "POST", Path: "/api/v0/code/divergence/investigate", Family: "code_divergence", Serve: divergenceServeGated, Fragment: "go/internal/query/openapi/paths/code/divergence.go"},
	{Method: "POST", Path: "/api/v0/code/divergence/report", Family: "code_divergence", Serve: divergenceServeGated, Fragment: "go/internal/query/openapi/paths/code/divergence.go"},
}

// registerDivergenceWave mounts the divergence wave gate ahead of the
// generic proxy: a valid shared key terminates on the native lane; a
// foreign credential class (scoped token, OIDC bearer, session cookie)
// relays explicitly on the relay lane so core judges it;
// credential-less callers — and junk-cookie carriers, per the D2
// follow-up lesson — get the standing 401. Each trailing-slash
// spelling stays on the wave route for the same never-split reason.
// Lives here (not in routes()) so the shared router stays under the
// file-size cap.
func (s *Server) registerDivergenceWave(mux *http.ServeMux) {
	for _, entry := range divergenceWave {
		pattern := entry.Method + " " + entry.Path
		gated := s.instrumented(viaNative, s.divergenceReadGate(http.HandlerFunc(s.serveDivergenceTerminated)))
		mux.Handle(pattern, gated)
		mux.Handle(pattern+"/", gated)
	}
}
