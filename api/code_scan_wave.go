// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// Code-scan relay-leg termination (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#98, W6b). This file owns the frozen wave
// table: exactly the 4 W6b graph-scan POSTs the versioned scan pin
// admits (the complexity POST admits its by-name, by-ID, and ranked
// builders through the bounded traversal plus the by-ID enrichment
// hydration; the quality/inspect POST admits its bounded scan through
// the same traversal read; the imports/investigate POST admits its
// leaf dispatch through the same traversal read; the bundles POST
// admits its anchor plus version counts through the same traversal
// read with the public-only grant model). Behavior lives in
// code_scan_terminate.go; proofs in code_scan_conformance_test.go
// and the per-family suites; spec posture stays disclosed (no
// openapi port in this lane); the record in
// contracts/code-scan-termination.md.
//
// This lane is a NEW wave table, never an extension of codeGraphWave,
// codeDivergenceWave, deadCodeWave, or relationshipWave: the W6b pin
// covers its own 4 rows only, and no other family may cite the scan
// pin as coverage. Citing CodeGraphReadAdmission v0 as coverage for
// any row here is void (that pin seals the W6b scan legs out
// explicitly).
//
// Pre-edit gate (MUST read before editing): entry point is
// Server.routes in api/router.go, which calls
// registerCodeScanWave ahead of the generic proxy; phase order is
// pin record first, wave registration second, parity/latency proofs
// third; data dependency is the W6b pin at 81b756fd84 (recorded
// satellite-side in contracts/code-scan-read-admission.md plus the
// vendored upstream copies, v0, exact 2-method set, no transport
// constructor by design); re-trigger is
// dispatchNote/telemetry per family plus spec_scope and the disclosed
// absences that move with this lane.
//
// Production binding (carried from the D0 shim proof via the D2-D6
// waves, the Wave-2 Content lane, the W4 code-search lane, the Wave-7
// G-a graph lane, the G-b relationship lane, and the
// dead-code/divergence lanes): dest owns the route (caller gate,
// family authorizer, response mapping) and calls the versioned core
// read API as its port transport — the shared-key credential and
// negotiation headers forwarded, no retry, no table/queue handles
// dest-side. Authz stays atomic core-side: dest allows only the
// shared-key (AllScopes) identity it can construct and core re-judges
// every forwarded call as backstop (tenant scope, permission catalog,
// governance mode — which is also where the live 403s and
// mode-dependent denials come from). The contract record is
// contracts/code-scan-termination.md.
//
// Transport pin: the versioned W6b pin, v0 (recorded at the W6 mint
// 81b756fd84, merge of eshu-hq/eshu#10688), covers all four wave rows
// — the bounded traversal plus the by-ID enrichment the complexity
// by-ID branch hydrates through. The pin carries no transport
// constructor by design:
// dest serves the reads through the existing graph reader plus the
// guarded Postgres content reader, the same pair cmd/api wiring
// builds today, and any other backend pairing needs a new arbiter
// ruling. Handler behavior stays satellite-side and unpinned: grant
// checks, the grant-bound builders, the repository selector, fixed
// failure messages, QueryProfile. This lane cites no other pin, and
// no other family may cite this pin.
//
// The routes stay inside RequireAuth: the wave gate answers every
// credential shape (shared-key terminate, foreign-class relay,
// credential-less 401), so no keyless caller gains a new public
// surface here. There is no public carve-out in this lane.

// codeScanServeGated is the only serve kind in this lane.
const codeScanServeGated = "gated"

// codeScanRoute is one frozen W6b entry: the exact surface this
// lane terminates. Serve tells the router how this lane serves it;
// Fragment names the core-side Decision 5(c) fragment the follow-up
// spec-port lane will vendor — this lane ports no fragment (the spec
// absences stay disclosed, see
// contracts/code-scan-termination.md), so the field carries the core
// path, not a satellite one.
type codeScanRoute struct {
	Method   string
	Path     string
	Family   string
	Serve    string
	Fragment string
}

// codeScanWave is the frozen W6b termination target: exactly the 4
// scan POSTs behind the versioned scan pin, v0.
// TestCodeScanWaveFrozen pins it entry by entry; a wave change moves
// the ownership note, the fragment reference, and this mirror
// together.
var codeScanWave = []codeScanRoute{
	{Method: "POST", Path: "/api/v0/code/complexity", Family: "code_scan", Serve: codeScanServeGated, Fragment: "go/internal/query/openapi/paths/code/routes.go"},
	{Method: "POST", Path: "/api/v0/code/quality/inspect", Family: "code_scan", Serve: codeScanServeGated, Fragment: "go/internal/query/openapi/paths/code/quality.go"},
	{Method: "POST", Path: "/api/v0/code/imports/investigate", Family: "code_scan", Serve: codeScanServeGated, Fragment: "go/internal/query/openapi/paths/code/symbols.go"},
	{Method: "POST", Path: "/api/v0/code/bundles", Family: "code_scan", Serve: codeScanServeGated, Fragment: "go/internal/query/openapi/paths/code/graph.go"},
}

// registerCodeScanWave mounts the W6b code-scan wave gate ahead of
// the generic proxy: a valid shared key terminates on the native
// lane; a foreign credential class (scoped token, OIDC bearer,
// session cookie) relays explicitly on the relay lane so core judges
// it; credential-less callers — and junk-cookie carriers, per the D2
// follow-up lesson — get the standing 401. Each trailing-slash
// spelling stays on the wave route for the same never-split reason.
// Lives here (not in routes()) so the shared router stays under the
// file-size cap.
func (s *Server) registerCodeScanWave(mux *http.ServeMux) {
	for _, entry := range codeScanWave {
		pattern := entry.Method + " " + entry.Path
		gated := s.instrumented(viaNative, s.codeScanReadGate(http.HandlerFunc(s.serveCodeScanTerminated)))
		mux.Handle(pattern, gated)
		mux.Handle(pattern+"/", gated)
	}
}
