// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// Wave-2 Content point reads termination (Refs eshu-hq/eshu#9669, D13
// tracker eshu-hq/eshu#10491, satellite eshu-hq/eshu-api#72). This
// file owns the frozen wave table: exactly the 3 Content POSTs the
// merged core pin ContentPointReadAdmission v0 admits (core c987f73 —
// the bounded single-file content lookup, the bounded single-file
// line-range slice with its clamping and empty-range edges, and the
// bounded single-entity lookup by entity id). A wave change moves the
// ownership map, the spec fragments, and this mirror together. The
// behavior lives in content_terminate.go; the proofs in
// content_conformance_test.go and the per-family suites.
//
// Production binding (carried from the D0 shim proof via the D2/D4/D6
// waves and the Wave-1 Freshness lane): dest owns each route (caller
// gate, family authorizer, response mapping) and calls the versioned
// core read API as its port transport — the shared-key credential and
// negotiation headers forwarded, no retry, no table/queue handles
// dest-side. Authz stays atomic core-side: dest allows only the
// shared-key (AllScopes) identity it can construct and core re-judges
// every forwarded call as backstop (tenant scope, permission catalog,
// governance mode — which is also where the live 403s and
// mode-dependent denials come from). The contract record is
// contracts/content-point-reads-wave.md.
//
// Transport pin: ContentPointReadAdmission v0 (contracts/upstream/
// content_point_read_admission_contract_test.go at the Wave-2 merge
// SHA) covers all 3 wave rows — every row admits through its
// re-declared port method over the guarded read port. The minting rule
// forbids inventing pins beyond it; the search legs, the list legs,
// and the scoped-entity leg stay out per the pin record and stay relay
// with the reason disclosed per route in the contract.
//
// D-wave satellite serve kind. Every Wave-2 route is wave-gated:
// shared-key callers terminate, unresolvable credential classes relay
// explicitly, credential-less callers (and junk-cookie carriers, per
// the D2 follow-up lesson) keep the standing 401. There is no public
// carve-out in this lane — every Content point read is an authed
// reader-port read.

// contentServeGated is the only serve kind in this lane.
const contentServeGated = "gated"

// Content point-read route paths: the exact core route paths the pin
// admits. The wave table references these so the frozen test, the
// router registration, and the terminated-handler switch move
// together.
const (
	contentFilesReadPath    = "/api/v0/content/files/read"
	contentFilesLinesPath   = "/api/v0/content/files/lines"
	contentEntitiesReadPath = "/api/v0/content/entities/read"
)

// contentRoute is one frozen Wave-2 entry: the exact cross-repo
// surface this lane terminates. Method+Path+Family mirror the CUTOVER
// rows; Serve tells the router how this lane serves it; Fragment names
// the Decision 5(c) fragment that carries the route's spec entry.
type contentRoute struct {
	Method   string
	Path     string
	Family   string
	Serve    string
	Fragment string
}

// contentWave is the frozen Wave-2 termination target: exactly the 3
// Content POSTs behind ContentPointReadAdmission v0.
// TestContentPointReadsWaveFrozen pins it entry by entry; a wave change
// moves the ownership map, the fragments, and this mirror together.
var contentWave = []contentRoute{
	{Method: "POST", Path: contentFilesReadPath, Family: "content", Serve: contentServeGated, Fragment: "paths/search/content_point_reads.go"},
	{Method: "POST", Path: contentFilesLinesPath, Family: "content", Serve: contentServeGated, Fragment: "paths/search/content_point_reads.go"},
	{Method: "POST", Path: contentEntitiesReadPath, Family: "content", Serve: contentServeGated, Fragment: "paths/search/content_point_reads.go"},
}

// registerContentPointReads mounts the Wave-2 Content point reads
// termination (Refs eshu-hq/eshu#9669): the 3 pinned read routes
// register explicitly inside the wave gate, ahead of the generic
// proxy. A valid shared key terminates on the native lane; a foreign
// credential class (scoped token, OIDC bearer, session cookie) relays
// explicitly on the relay lane so core judges it; credential-less
// callers — and junk-cookie carriers, per the D2 follow-up lesson —
// get the standing 401. Each trailing-slash spelling stays on the wave
// route for the same never-split reason. Every other family still
// crosses its existing lane.
func (s *Server) registerContentPointReads(mux *http.ServeMux) {
	for _, entry := range contentWave {
		pattern := entry.Method + " " + entry.Path
		gated := s.instrumented(viaNative, s.contentReadGate(http.HandlerFunc(s.contentTerminatedHandler(entry.Path))))
		mux.Handle(pattern, gated)
		mux.Handle(pattern+"/", gated)
	}
}
