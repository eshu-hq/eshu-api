// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// Code-cypher relay-leg termination (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#98, W6d). This file owns the frozen wave
// table: exactly the 2 W6d caller-program POSTs the versioned cypher
// pin admits (the cypher POST admits its read through the bounded
// traversal with raw result rows; the visualize POST admits its read
// through the same bounded traversal, projecting a bounded,
// renderable visualization packet from the returned graph entities,
// with row-window truncation marked and limited). Behavior lives in
// code_cypher_terminate.go; proofs in
// code_cypher_conformance_test.go and the per-family suites; spec
// posture stays disclosed (no openapi port in this lane); the record
// in contracts/code-cypher-termination.md.
//
// This lane is a NEW wave table, never an extension of codeGraphWave,
// deadCodeWave, codeDivergenceWave, relationshipWave, codeScanWave,
// codeInvestigationWave, or codeSecretsWave: the W6d pin covers its
// own 2 rows only, and no other family may cite the cypher pin as
// coverage. Citing any sibling-family pin as coverage for any row
// here is void (those pins seal the W6d legs out explicitly).
//
// Pre-edit gate (MUST read before editing): entry point is
// Server.routes in api/router.go, which calls
// registerCodeCypherWave ahead of the generic proxy; phase order is
// pin record first, wave registration second, parity proofs third;
// data dependency is the W6d pin at 81b756fd84 (recorded
// satellite-side in contracts/code-cypher-read-admission.md plus the
// vendored upstream copies, v0, exact 1-method set, no transport
// constructor by design); re-trigger is dispatchNote/telemetry per
// family plus spec_scope and the disclosed absences that move with
// this lane.
//
// Production binding (carried from the D0 shim proof via the D2-D6
// waves, the Wave-2 Content lane, the W4 code-search lane, the Wave-7
// G-a graph lane, the G-b relationship lane, the W6b scan lane, the
// W6c investigation and secret lanes, and the dead-code/divergence
// lanes): dest owns the route (caller gate, family authorizer,
// response mapping) and calls the versioned core read API as its port
// transport — the shared-key credential and negotiation headers
// forwarded, no retry, no table/queue handles dest-side. Authz stays
// atomic core-side: dest allows only the shared-key (AllScopes)
// identity it can construct and core re-judges every forwarded call
// as backstop (tenant scope, permission catalog, governance mode —
// which is also where the live 403s and mode-dependent denials come
// from). The contract record is
// contracts/code-cypher-termination.md.
//
// Transport pin: the versioned W6d pin, v0 (recorded at the W6 mint
// 81b756fd84, merge of eshu-hq/eshu#10688), covers both wave rows —
// the bounded traversal each route admits through, bounded by the
// read-only cypher shapers plus the route timeout. The pin carries
// no constructor surface by design: dest serves the reads through
// the concurrent-safe graph reader,
// the reader cmd/api wiring builds today, and any other backend
// pairing needs a new arbiter ruling. Handler behavior stays
// satellite-side and unpinned: the mutation-keyword rejection, the
// terminal LIMIT injection, the statement timeout, the grant-class
// gate (shared-key-only), the fixed failure messages, QueryProfile.
// This lane cites no other pin, and no other family may cite this
// pin.
//
// The routes stay inside RequireAuth: the wave gate answers every
// credential shape (shared-key terminate, foreign-class relay,
// credential-less 401), so no keyless caller gains a new public
// surface here. There is no public carve-out in this lane.

// codeCypherServeGated is the only serve kind in this lane.
const codeCypherServeGated = "gated"

// codeCypherRoute is one frozen W6d entry: the exact surface this
// lane terminates. Serve tells the router how this lane serves it;
// Fragment names the core-side Decision 5(c) fragment the follow-up
// spec-port lane will vendor — this lane ports no fragment (the spec
// absences stay disclosed, see
// contracts/code-cypher-termination.md), so the field carries the core
// path, not a satellite one.
type codeCypherRoute struct {
	Method   string
	Path     string
	Family   string
	Serve    string
	Fragment string
}

// codeCypherWave is the frozen W6d termination target: exactly the 2
// cypher POSTs behind the versioned cypher pin, v0.
// TestCodeCypherWaveFrozen pins it entry by entry; a wave change moves
// the ownership note, the fragment reference, and this mirror
// together.
var codeCypherWave = []codeCypherRoute{
	{Method: "POST", Path: "/api/v0/code/cypher", Family: "code_cypher", Serve: codeCypherServeGated, Fragment: "go/internal/query/openapi/paths/code/graph.go"},
	{Method: "POST", Path: "/api/v0/code/visualize", Family: "code_cypher", Serve: codeCypherServeGated, Fragment: "go/internal/query/openapi/paths/code/graph.go"},
}

// registerCodeCypherWave mounts the W6d code-cypher wave gate ahead
// of the generic proxy: a valid shared key terminates on the native
// lane; a foreign credential class (scoped token, OIDC bearer,
// session cookie) relays explicitly on the relay lane so core judges
// it — core refuses scoped and browser-session callers on these
// shared-key-only routes before any handler runs; credential-less
// callers — and junk-cookie carriers, per the D2 follow-up lesson —
// get the standing 401. Each trailing-slash spelling stays on the
// wave route for the same never-split reason. Lives here (not in
// routes()) so the shared router stays under the file-size cap.
func (s *Server) registerCodeCypherWave(mux *http.ServeMux) {
	for _, entry := range codeCypherWave {
		pattern := entry.Method + " " + entry.Path
		gated := s.instrumented(viaNative, s.codeCypherReadGate(http.HandlerFunc(s.serveCodeCypherTerminated)))
		mux.Handle(pattern, gated)
		mux.Handle(pattern+"/", gated)
	}
}
