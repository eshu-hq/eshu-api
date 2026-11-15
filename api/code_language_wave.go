// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// Code-language relay-leg termination (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#98, W6c). This file owns the frozen wave
// table: exactly the one W6c language-query POST the versioned
// language pin admits (the route admits its reads through the
// bounded traversal for the graph-backed, graph-first-content, and
// directory branches, plus the three content reads for the
// content-only branch and the two enrichments, with the entity-type
// dispatch, the per-repo fallback and page-read fallback, and the
// page-clamped id set). Behavior lives in
// code_language_terminate.go; proofs in
// code_language_conformance_test.go and the per-family suites; spec
// posture stays disclosed (no openapi port in this lane); the record
// in contracts/code-language-termination.md.
//
// This lane is a NEW wave table, never an extension of codeGraphWave,
// deadCodeWave, codeDivergenceWave, relationshipWave, codeScanWave,
// codeInvestigationWave, codeSecretsWave, or codeCypherWave: the W6c
// pin covers its own row only, and no other family may cite the
// language pin as coverage. Citing any sibling-family pin as coverage
// for this row is void (those pins seal the language leg out
// explicitly).
//
// Pre-edit gate (MUST read before editing): entry point is
// Server.routes in api/router.go, which calls
// registerCodeLanguageWave ahead of the generic proxy; phase order is
// pin record first, wave registration second, parity proofs third;
// data dependency is the W6c pin at 81b756fd84 (recorded
// satellite-side in contracts/code-language-read-admission.md plus the
// vendored upstream copies, v0, exact 4-method set, no constructor
// surface by design); re-trigger is dispatchNote/telemetry per family
// plus spec_scope and the disclosed absences that move with this
// lane.
//
// Production binding (carried from the D0 shim proof via the D2-D6
// waves, the Wave-2 Content lane, the W4 code-search lane, the Wave-7
// G-a graph lane, the G-b relationship lane, the W6b scan lane, the
// W6c investigation and secret lanes, the W6d cypher lane, and the
// dead-code/divergence lanes): dest owns the route (caller gate,
// family authorizer, response mapping) and calls the versioned core
// read API as its port transport — the shared-key credential and
// negotiation headers forwarded, no retry, no table/queue handles
// dest-side. Authz stays atomic core-side: dest allows only the
// shared-key (AllScopes) identity it can construct and core re-judges
// every forwarded call as backstop (tenant scope, permission catalog,
// governance mode — which is also where the live 403s and
// mode-dependent denials come from). The contract record is
// contracts/code-language-termination.md.
//
// Transport pin: the versioned W6c pin, v0 (recorded at the W6 mint
// 81b756fd84, merge of eshu-hq/eshu#10688), covers the wave row —
// the bounded traversal plus the three content reads the route admits
// through. The pin lives in package language, not codequery, and
// carries no constructor surface by design: dest serves the reads
// through the concurrent-safe graph reader plus the guarded Postgres
// content reader, the readers cmd/api wiring builds today, and any
// other backend pairing needs a new arbiter ruling. Handler behavior
// stays satellite-side and unpinned: grant checks, the
// entity-type-family dispatch, the per-repo fallback loop, the
// page-read fallback, the fixed failure messages, QueryProfile. This
// lane cites no other pin, and no other family may cite this pin.
//
// The route stays inside RequireAuth: the wave gate answers every
// credential shape (shared-key terminate, foreign-class relay,
// credential-less 401), so no keyless caller gains a new public
// surface here. There is no public carve-out in this lane.

// codeLanguageServeGated is the only serve kind in this lane.
const codeLanguageServeGated = "gated"

// codeLanguageRoute is one frozen W6c entry: the exact surface this
// lane terminates. Serve tells the router how this lane serves it;
// Fragment names the core-side Decision 5(c) fragment the follow-up
// spec-port lane will vendor — this lane ports no fragment (the spec
// absences stay disclosed, see
// contracts/code-language-termination.md), so the field carries the
// core path, not a satellite one.
type codeLanguageRoute struct {
	Method   string
	Path     string
	Family   string
	Serve    string
	Fragment string
}

// codeLanguageWave is the frozen W6c termination target: exactly the
// one language-query POST behind the versioned language pin, v0.
// TestCodeLanguageWaveFrozen pins it entry by entry; a wave change
// moves the ownership note, the fragment reference, and this mirror
// together.
var codeLanguageWave = []codeLanguageRoute{
	{Method: "POST", Path: "/api/v0/code/language-query", Family: "code_language", Serve: codeLanguageServeGated, Fragment: "go/internal/query/openapi/paths/code/routes.go"},
}

// registerCodeLanguageWave mounts the W6c code-language wave gate
// ahead of the generic proxy: a valid shared key terminates on the
// native lane; a foreign credential class (scoped token, OIDC bearer,
// session cookie) relays explicitly on the relay lane so core judges
// it; credential-less callers — and junk-cookie carriers, per the D2
// follow-up lesson — get the standing 401. Each trailing-slash
// spelling stays on the wave route for the same never-split reason.
// Lives here (not in routes()) so the shared router stays under the
// file-size cap.
func (s *Server) registerCodeLanguageWave(mux *http.ServeMux) {
	for _, entry := range codeLanguageWave {
		pattern := entry.Method + " " + entry.Path
		gated := s.instrumented(viaNative, s.codeLanguageReadGate(http.HandlerFunc(s.serveCodeLanguageTerminated)))
		mux.Handle(pattern, gated)
		mux.Handle(pattern+"/", gated)
	}
}
