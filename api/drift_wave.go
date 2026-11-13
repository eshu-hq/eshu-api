// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// Drift relay-leg termination (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#117). This file owns the frozen wave table:
// exactly the 2 drift routes the versioned drift pin admits (the
// findings POST admits its bounded readback through the page plus the
// truncation total; the packet GET admits its packet build through the
// page alone — the packet carries no total, so the count read stays
// unexercised there by construction). Behavior lives in
// drift_terminate.go; proofs in drift_conformance_test.go and the
// per-family suites; spec posture in openapi/spec_drift_test.go; the
// record in contracts/cloud-runtime-drift-termination.md.
//
// This lane is a NEW wave table, never an extension of w8aWave: the
// W8a pins cover their own 16 rows only, and no other family may cite
// the drift pin as coverage. Citing TerraformDriftReadAdmission v0 or
// IaCReadAdmission v0 as coverage for either route here is void (the
// pin seals that out explicitly).
//
// Pre-edit gate (MUST read before editing): entry point is
// Server.routes in api/router.go, which calls registerDriftWave ahead
// of the generic proxy; phase order is pin record first, wave
// registration second, parity/latency proofs third; data dependency
// is the W8b tier-1 pin at d81bd12802 (recorded satellite-side in
// contracts/cloud-runtime-drift-termination.md plus the vendored
// upstream copies, v0, exact 2-method set, no transport constructor by
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
// contract record is contracts/cloud-runtime-drift-termination.md.
//
// Transport pin: the versioned drift pin, v0 (recorded at the W8b
// mint d81bd12802), covers both wave rows — the provider-neutral
// findings readback plus the drift investigation packet the two
// routes admit through. The pin carries no transport constructor by
// design: dest serves the reads through the existing Postgres adapter
// the way cmd/api wiring builds it today, and any other backend
// pairing needs a new arbiter ruling. Handler behavior stays
// satellite-side and unpinned: grant checks, fixed failure messages,
// packet bounds, the truth envelope, QueryProfile. This lane cites no
// other pin, and no other family may cite this pin.
//
// The routes stay inside RequireAuth: the wave gate answers every
// credential shape (shared-key terminate, foreign-class relay,
// credential-less 401), so no keyless caller gains a new public
// surface here. There is no public carve-out in this lane.

// driftServeGated is the only serve kind in this lane.
const driftServeGated = "gated"

// driftRoute is one frozen drift entry: the exact cross-repo surface
// this lane terminates. Serve tells the router how this lane serves
// it; Fragment names the core-side Decision 5(c) fragment the
// follow-up spec-port lane will vendor — this lane ports no fragment
// (the spec absences stay disclosed, see
// contracts/cloud-runtime-drift-termination.md), so the field carries
// the core path, not a satellite one.
type driftRoute struct {
	Method   string
	Path     string
	Family   string
	Serve    string
	Fragment string
}

// driftWave is the frozen drift termination target: exactly the 2
// drift routes behind the versioned drift pin, v0.
// TestDriftWaveFrozen pins it entry by entry; a wave change moves the
// ownership note, the fragment reference, and this mirror together.
var driftWave = []driftRoute{
	{Method: "POST", Path: "/api/v0/cloud/runtime-drift/findings", Family: "iac", Serve: driftServeGated, Fragment: "go/internal/query/openapi/paths/cloud/runtime_drift.go"},
	{Method: "GET", Path: "/api/v0/investigations/drift/packet", Family: "evidence", Serve: driftServeGated, Fragment: "go/internal/query/openapi/paths/evidence/investigations.go"},
}

// registerDriftWave mounts the drift wave gate ahead of the generic
// proxy: a valid shared key terminates on the native lane; a foreign
// credential class (scoped token, OIDC bearer, session cookie) relays
// explicitly on the relay lane so core judges it; credential-less
// callers — and junk-cookie carriers, per the D2 follow-up lesson —
// get the standing 401. Each trailing-slash spelling stays on the
// wave route for the same never-split reason. Lives here (not in
// routes()) so the shared router stays under the file-size cap.
func (s *Server) registerDriftWave(mux *http.ServeMux) {
	for _, entry := range driftWave {
		pattern := entry.Method + " " + entry.Path
		gated := s.instrumented(viaNative, s.driftReadGate(http.HandlerFunc(s.serveDriftTerminated)))
		mux.Handle(pattern, gated)
		mux.Handle(pattern+"/", gated)
	}
}
