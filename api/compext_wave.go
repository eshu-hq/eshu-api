// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// Component-extensions satellite termination (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#101, Wave-1b ruling: PROCEED on both
// legs). This file owns the frozen wave table: exactly the 2
// component-extension routes the versioned ComponentExtension
// read-admission pin admits. The behavior lives in
// compext_terminate.go; the proofs in
// compext_conformance_test.go, compext_gate_test.go,
// compext_parity_test.go, and compext_latency_test.go; the spec
// posture in openapi/spec_compext_test.go; the record in
// contracts/component-extension-termination.md.
//
// This lane is a NEW wave table, never an extension of w8bNarrowWave
// or any other table: the ComponentExtension pin covers its own 2
// rows only, and no other family may cite it as coverage. Citing any
// other pin as coverage here is void (the pin seals the other
// families out explicitly).
//
// The 6 no-port routes (readiness x2, fact-schema x2,
// surface-inventory, visualizations/derive) carry no core mint and
// stay on the relay (see contracts/w8b-narrow-termination.md).
//
// Pre-edit gate (MUST read before editing): entry point is
// Server.routes in api/router.go, which calls
// registerCompextWave ahead of the generic proxy; phase order is
// pin record first, wave registration second, parity/latency proofs
// third; data dependency is the ComponentExtension v0 pin recorded
// satellite-side in
// contracts/component-extension-termination.md plus the vendored
// upstream copy, exact method set, no transport constructor by
// design; re-trigger is dispatchNote/telemetry per family plus
// spec_scope and the disclosed absences that move with this lane.
//
// Production binding (carried from the Wave-1b ruling via the D0
// shim proof and the W8b-narrow lane): dest owns the route (caller
// gate, family authorizer, response mapping) and calls the versioned
// core read API as its port transport — the shared-key credential
// and negotiation headers forwarded, no retry, no table/queue
// handles dest-side. Authz stays atomic core-side: dest allows only
// the shared-key identity it can construct and core re-judges every
// forwarded call as backstop (tenant scope, permission catalog,
// governance mode — which is also where the live 403s and mode
// dependent denials come from). The contract record is
// contracts/component-extension-termination.md.
//
// Transport pin (recorded byte-identical at the core HEAD 9bf29e53,
// re-verified in-lane with zero drift since its mint): the
// ComponentExtension read admission v0, one re-declared read method
// (Readback). It carries no transport constructor by design: dest
// serves the reads through the versioned forward transport only, and
// any other backend pairing needs a new arbiter ruling. Handler
// behavior stays satellite-side and unpinned: grant checks, fixed
// failure messages, limit truncation, the component_id filter + 404,
// QueryProfile. This lane cites no other pin, and no other family
// may cite this pin.
//
// The routes stay inside RequireAuth: the wave gate answers every
// credential shape (shared-key terminate, foreign-class relay,
// credential-less 401), so no keyless caller gains a new public
// surface here. There is no public carve-out in this lane.
//
// Both legs classify to the shared "other" family in classifyFamily
// (no component prefix entry exists): the authorizer and telemetry
// pin that value until a family entry lands.

// compextServeGated is the only serve kind in this lane.
const compextServeGated = "gated"

// compextRoute is one frozen Wave-1b entry: the exact surface this
// lane terminates. Serve tells the router how this lane serves it;
// Fragment names the core-side Decision 5(c) fragment the
// follow-up spec-port lane will vendor — this lane ports no fragment
// (the spec absences stay disclosed, see
// contracts/component-extension-termination.md), so the field
// carries the core path, not a satellite one.
type compextRoute struct {
	Method   string
	Path     string
	Family   string
	Serve    string
	Fragment string
}

// compextWave is the frozen Wave-1b termination target: exactly the
// 2 component-extension routes behind the versioned pin, v0.
// TestCompextWaveFrozen pins it entry by entry; a wave change moves
// the ownership note, the fragment reference, and this mirror
// together.
var compextWave = []compextRoute{
	{Method: "GET", Path: "/api/v0/component-extensions", Family: "other", Serve: compextServeGated, Fragment: "go/internal/query/openapi/paths/catalog/component_extensions.go"},
	{Method: "GET", Path: "/api/v0/component-extensions/{component_id}/diagnostics", Family: "other", Serve: compextServeGated, Fragment: "go/internal/query/openapi/paths/catalog/component_extensions.go"},
}

// registerCompextWave mounts the Wave-1b wave gate ahead of the
// generic proxy: a valid shared key terminates on the native lane; a
// foreign credential class (scoped token, OIDC bearer, session
// cookie) relays explicitly on the relay lane so core judges it;
// credential-less callers — and junk-cookie carriers, per the D2
// follow-up lesson — get the standing 401. Each trailing-slash
// spelling registers on the same gate for the same never-split
// reason, diagnostics included: registerCompextWave mounts
// pattern+"/" for every entry.
// Lives here (not in routes()) so the shared router stays under the
// file-size cap.
func (s *Server) registerCompextWave(mux *http.ServeMux) {
	for _, entry := range compextWave {
		pattern := entry.Method + " " + entry.Path
		gated := s.instrumented(viaNative, s.compextReadGate(http.HandlerFunc(s.serveCompextTerminated)))
		mux.Handle(pattern, gated)
		mux.Handle(pattern+"/", gated)
	}
}
