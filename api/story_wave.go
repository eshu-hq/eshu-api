// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// Repository story relay-leg termination (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#97). This file owns the frozen wave
// table: the W5f story single the versioned story pin admits (GET
// /api/v0/repositories/{repo_id}/story). Behavior lives in
// story_terminate.go; proofs in story_conformance_test.go,
// story_parity_test.go, and story_gate_test.go; spec posture stays
// disclosed (no openapi port in this lane); the record in
// contracts/repository-story-termination.md.
//
// This lane is a NEW wave table, never an extension of d4Wave: the
// story pin covers this single row only, and the D4 transport record
// explicitly excludes story (a whole-Repositories pin would
// retroactively widen D4, which stays forbidden). No other family may
// cite the story pin as coverage.
//
// The story GET left the D4 wave (its spec entry waited on the
// schema.EvidenceBoundaries helper port) and rode the explicit D4
// relay leg (d4RelayLegs, carried by
// TestD4StoryRelayLegOnGenericProxy). This lane retires that leg: the
// route terminates natively behind the story pin, and the relay test
// is superseded by the native parity corpus here.
//
// Pre-edit gate (MUST read before editing): entry point is
// Server.routes in api/router.go, which calls registerStoryWave ahead
// of the generic proxy; phase order is pin record first, wave
// registration second, parity proofs third; data dependency is the
// story pin at 76240fda9f (recorded satellite-side in
// contracts/repository-story-read-admission.md plus the vendored
// upstream copies, v0, exact 15-method set, no transport constructor
// by design); re-trigger is dispatchNote/telemetry per family plus
// spec_scope and the disclosed absence that moves with this lane.
//
// Production binding (carried from the D0 shim proof through the D2-D6
// waves and the W4/W5 lanes): dest owns the route (caller gate,
// family authorizer, response mapping) and calls the versioned core
// read API as its port transport — the shared-key credential and
// negotiation headers forwarded, no retry, no table/queue handles
// dest-side. Authz stays atomic core-side: dest allows only the
// shared-key (AllScopes) identity it can construct and core re-judges
// every forwarded call as backstop. The contract record is
// contracts/repository-story-termination.md.
//
// Transport pin: the versioned story pin, v0 (recorded at the W5 mint
// 76240fda9f), covers this single row. The pin carries no transport
// constructor by design: dest serves the reads through the existing
// graph reader, content reader, and run-correlation store, the same
// backends cmd/api wiring builds today, and any other backend pairing
// needs a new arbiter ruling. Handler behavior stays satellite-side
// and unpinned: the repository selector, fixed failure messages, the
// responder and writer seams. This lane cites no other pin, and no
// other route may cite this pin.
//
// The route stays inside RequireAuth: the wave gate answers every
// credential shape (shared-key terminate, foreign-class relay,
// credential-less 401), so no keyless caller gains a new public
// surface here. There is no public carve-out in this lane.

// storyServeGated is the only serve kind in this lane.
const storyServeGated = "gated"

// storyRoute is one frozen W5 story entry: the exact surface this
// lane terminates. Serve tells the router how this lane serves it;
// Fragment names the core-side Decision 5(c) fragment the follow-up
// spec-port lane will vendor — this lane ports no fragment (the spec
// absence stays disclosed, see
// contracts/repository-story-termination.md), so the field carries the
// core path, not a satellite one.
type storyRoute struct {
	Method   string
	Path     string
	Family   string
	Serve    string
	Fragment string
}

// storyWave is the frozen W5 story termination target: exactly the
// story single behind the versioned story pin, v0.
// TestStoryWaveFrozen pins it entry by entry; a wave change moves the
// ownership note, the fragment reference, and this mirror together.
var storyWave = []storyRoute{
	{Method: "GET", Path: "/api/v0/repositories/{repo_id}/story", Family: "repositories", Serve: storyServeGated, Fragment: "go/internal/query/openapi/paths/repository/routes.go"},
}

// registerStoryWave mounts the story wave ahead of the generic proxy:
// a valid shared key terminates on the native lane; a foreign
// credential class (scoped token, OIDC bearer, session cookie) relays
// explicitly on the relay lane so core judges it; credential-less
// callers — and junk-cookie carriers, per the D2 follow-up lesson —
// get the standing 401. The explicit registration shadows the list
// route's slash subtree (which would otherwise swallow the story path
// back under the D4 gate), the same shadowing the retired relay leg
// relied on. Each trailing-slash spelling stays on the wave route for
// the same never-split reason. Lives here (not in routes()) so the
// shared router stays under the file-size cap.
func (s *Server) registerStoryWave(mux *http.ServeMux) {
	for _, entry := range storyWave {
		pattern := entry.Method + " " + entry.Path
		gated := s.instrumented(viaNative, s.storyReadGate(http.HandlerFunc(s.serveStoryTerminated)))
		mux.Handle(pattern, gated)
		mux.Handle(pattern+"/", gated)
	}
}
