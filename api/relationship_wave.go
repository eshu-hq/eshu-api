// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// Relationships relay-leg termination (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#122). This file owns the frozen wave
// table: exactly the 2 G-b relationship POSTs the versioned
// relationship pin admits (the relationships POST admits its bounded
// graph traversal plus entity and repository reads plus the content
// fallback plus the transitive CALLS walk; the story POST admits the
// same reads plus the class_hierarchy and overrides enrichment).
// Behavior lives in relationship_terminate.go; proofs in
// relationship_conformance_test.go and relationship_parity_test.go;
// spec posture stays disclosed (no openapi port in this lane); the
// record in contracts/code-relationships-termination.md.
//
// This lane is a NEW wave table, never an extension of codeGraphWave,
// deadCodeWave, or codeDivergenceWave: the G-b pin covers its own 2
// rows only, and no other family may cite the G-b pin as coverage.
// Citing CodeGraphReadAdmission v0, CodeSearchReadAdmission v0, or
// ContentSearchReadAdmission v0 as coverage here is void (those pins
// seal the G-b relationship legs out explicitly).
//
// Pre-edit gate (MUST read before editing): entry point is
// Server.routes in api/router.go, which calls
// registerRelationshipWave ahead of the generic proxy; phase order is
// pin record first, wave registration second, parity proofs third;
// data dependency is the G-b pin at 8ee32698 (recorded
// satellite-side in contracts/code-relationships-read-admission.md
// plus the vendored upstream copies, v0, exact 17-method set, no
// transport constructor by design); re-trigger is
// dispatchNote/telemetry per family plus spec_scope and the disclosed
// absences that move with this lane.
//
// Production binding (carried from the D0 shim proof via the D2-D6
// waves, the Wave-2 Content lane, the W4 code-search lane, the Wave-7
// G-a graph lane, and the dead-code/divergence lanes): dest owns the
// route (caller gate, family authorizer, response mapping) and calls
// the versioned core read API as its port transport — the shared-key
// credential and negotiation headers forwarded, no retry, no
// table/queue handles dest-side. Authz stays atomic core-side: dest
// allows only the shared-key (AllScopes) identity it can construct
// and core re-judges every forwarded call as backstop (tenant scope,
// permission catalog, governance mode — which is also where the live
// 403s and mode-dependent denials come from). The contract record is
// contracts/code-relationships-termination.md.
//
// Transport pin: the versioned G-b pin, v0 (recorded at the G-b mint
// 8ee32698, merge of eshu-hq/eshu#10669), covers both wave rows — the
// bounded graph traversal through Run and RunSingle, the twelve
// staying content reads the resolution, repo-identity, enrichment,
// fallback, and builder-closure paths exercise, and the three
// grant-scoped reads scoped callers take. The pin carries no
// transport constructor by design: dest serves the reads through the
// existing graph reader plus the guarded Postgres content reader, the
// same pair cmd/api wiring builds today, and any other backend
// pairing needs a new arbiter ruling. Handler behavior stays
// satellite-side and unpinned: grant checks, the repository selector,
// fixed failure messages, finding assembly, truncation rules,
// QueryProfile. This lane cites no other pin, and no other family may
// cite this pin.
//
// The routes stay inside RequireAuth: the wave gate answers every
// credential shape (shared-key terminate, foreign-class relay,
// credential-less 401), so no keyless caller gains a new public
// surface here. There is no public carve-out in this lane.

// relationshipServeGated is the only serve kind in this lane.
const relationshipServeGated = "gated"

// relationshipRoute is one frozen G-b entry: the exact surface this
// lane terminates. Serve tells the router how this lane serves it;
// Fragment names the core-side Decision 5(c) fragment the follow-up
// spec-port lane will vendor — this lane ports no fragment (the spec
// absences stay disclosed, see
// contracts/code-relationships-termination.md), so the field carries
// the core path, not a satellite one.
type relationshipRoute struct {
	Method   string
	Path     string
	Family   string
	Serve    string
	Fragment string
}

// relationshipWave is the frozen G-b termination target: exactly the
// 2 relationship POSTs behind the versioned G-b pin, v0.
// TestRelationshipWaveFrozen pins it entry by entry; a wave change
// moves the ownership note, the fragment reference, and this mirror
// together.
var relationshipWave = []relationshipRoute{
	{Method: "POST", Path: "/api/v0/code/relationships", Family: "code_relationships", Serve: relationshipServeGated, Fragment: "go/internal/query/openapi/paths/code/routes.go"},
	{Method: "POST", Path: "/api/v0/code/relationships/story", Family: "code_relationships", Serve: relationshipServeGated, Fragment: "go/internal/query/openapi/paths/code/routes.go"},
}

// registerRelationshipWave mounts the G-b wave gate ahead of the
// generic proxy: a valid shared key terminates on the native lane; a
// foreign credential class (scoped token, OIDC bearer, session cookie)
// relays explicitly on the relay lane so core judges it;
// credential-less callers — and junk-cookie carriers, per the D2
// follow-up lesson — get the standing 401. Each trailing-slash
// spelling stays on the wave route for the same never-split reason.
// Lives here (not in routes()) so the shared router stays under the
// file-size cap.
func (s *Server) registerRelationshipWave(mux *http.ServeMux) {
	for _, entry := range relationshipWave {
		pattern := entry.Method + " " + entry.Path
		gated := s.instrumented(viaNative, s.relationshipReadGate(http.HandlerFunc(s.serveRelationshipTerminated)))
		mux.Handle(pattern, gated)
		mux.Handle(pattern+"/", gated)
	}
}
