// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// Supply-chain impact packet relay-leg termination (Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#97). This file owns
// the frozen wave table: the W5f packet single the versioned packet
// pin admits (GET
// /api/v0/investigations/supply-chain/impact/packet). Behavior lives
// in packet_terminate.go; proofs in packet_conformance_test.go,
// packet_parity_test.go (including the 7-case Decision 2 suppression
// corpus), and packet_gate_test.go; spec posture stays disclosed (no
// openapi port in this lane); the record in
// contracts/supply-chain-impact-packet-termination.md.
//
// This lane is a NEW wave table, never an extension of any earlier
// wave: the packet pin covers this single row only, it does not widen
// the D6 pin, and the repository story single rides its own pin —
// neither route may cite the other's pin. No other family may cite
// the packet pin as coverage.
//
// Decision 2 proof closes in this lane: the owed satellite parity
// corpus (active winner, suppressed winner, expired-suppression
// winner, ambiguous scope, no-evidence scope, scoped-empty-grant
// scope-refusal packet, unconfigured-origin 503) is proven at leaf
// time in packet_parity_test.go, not frozen in the pin.
//
// Pre-edit gate (MUST read before editing): entry point is
// Server.routes in api/router.go, which calls registerPacketWave ahead
// of the generic proxy; phase order is pin record first, wave
// registration second, parity proofs third; data dependency is the
// packet pin at 76240fda9f (recorded satellite-side in
// contracts/supply-chain-impact-packet-read-admission.md plus the
// vendored upstream copies, v0, exact 5-method set, no transport
// constructor by design); re-trigger is dispatchNote/telemetry per
// family plus spec_scope and the disclosed absence that moves with
// this lane.
//
// Production binding (carried from the D0 shim proof through the D2-D6
// waves and the W4/W5 lanes): dest owns the route (caller gate,
// family authorizer, response mapping) and calls the versioned core
// read API as its port transport — the shared-key credential and
// negotiation headers forwarded, no retry, no table/queue handles
// dest-side. Authz stays atomic core-side: dest allows only the
// shared-key (AllScopes) identity it can construct and core re-judges
// every forwarded call as backstop. The contract record is
// contracts/supply-chain-impact-packet-termination.md.
//
// Transport pin: the versioned packet pin, v0 (recorded at the W5 mint
// 76240fda9f), covers this single row — the two reducer reads plus
// the shared selector closure the D6 pin explicitly excludes. The pin
// carries no transport constructor by design: dest serves the reads
// through the existing explanation/readiness stores plus the selector
// closure, the same backends cmd/api wiring builds today, and any
// other backend pairing needs a new arbiter ruling. Handler behavior
// stays satellite-side and unpinned: the responder, the composers, the
// capability gate, fixed failure messages. This lane cites no other
// pin, and no other route may cite this pin.
//
// The route stays inside RequireAuth: the wave gate answers every
// credential shape (shared-key terminate, foreign-class relay,
// credential-less 401), so no keyless caller gains a new public
// surface here. There is no public carve-out in this lane.

// packetServeGated is the only serve kind in this lane.
const packetServeGated = "gated"

// packetRoute is one frozen W5 packet entry: the exact surface this
// lane terminates. Serve tells the router how this lane serves it;
// Fragment names the core-side Decision 5(c) fragment the follow-up
// spec-port lane will vendor — this lane ports no fragment (the spec
// absence stays disclosed, see
// contracts/supply-chain-impact-packet-termination.md), so the field
// carries the core path, not a satellite one.
type packetRoute struct {
	Method   string
	Path     string
	Family   string
	Serve    string
	Fragment string
}

// packetWave is the frozen W5 packet termination target: exactly the
// packet single behind the versioned packet pin, v0.
// TestPacketWaveFrozen pins it entry by entry; a wave change moves the
// ownership note, the fragment reference, and this mirror together.
var packetWave = []packetRoute{
	{Method: "GET", Path: "/api/v0/investigations/supply-chain/impact/packet", Family: "supply_chain", Serve: packetServeGated, Fragment: "go/internal/query/openapi/paths/evidence/investigations.go"},
}

// registerPacketWave mounts the packet wave ahead of the generic
// proxy: a valid shared key terminates on the native lane; a foreign
// credential class (scoped token, OIDC bearer, session cookie) relays
// explicitly on the relay lane so core judges it; credential-less
// callers — and junk-cookie carriers, per the D2 follow-up lesson —
// get the standing 401. Each trailing-slash spelling stays on the wave
// route for the same never-split reason. Lives here (not in routes())
// so the shared router stays under the file-size cap.
func (s *Server) registerPacketWave(mux *http.ServeMux) {
	for _, entry := range packetWave {
		pattern := entry.Method + " " + entry.Path
		gated := s.instrumented(viaNative, s.packetReadGate(http.HandlerFunc(s.servePacketTerminated)))
		mux.Handle(pattern, gated)
		mux.Handle(pattern+"/", gated)
	}
}
