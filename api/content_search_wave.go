// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// W4 Content search termination (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#111). This file owns the frozen wave table: exactly
// the 4 Content search POSTs the W4 pin ContentSearchReadAdmission v0
// admits (core merge eshu-hq/eshu#10540 at e504e328a7 — the files and
// entities scoped, fallback, corpus, and unscoped reads plus the
// dockerfile-install and php-config entity pages). Behavior lives in
// content_search_terminate.go; proofs in
// content_search_conformance_test.go and the per-family suites; spec
// posture in openapi/spec_content_search_test.go; the record in
// contracts/content-search-termination.md.
//
// This lane is a NEW wave table, never an extension of contentWave:
// the Wave-2 point pin covers point reads only, and claiming search
// under it would break the pin record and trip the Wave-2 parity
// hygiene ban on search symbols in the point-read files.
//
// Pre-edit gate: entry point is Server.routes in api/router.go, which
// calls registerContentSearchWave ahead of the generic proxy; phase
// order is pin record first, wave registration second, parity/latency
// proofs third; data dependency is the W4 pin at e504e328a7 (contract
// text, v0, exact 10-method set, transport constructor); re-trigger is
// dispatchNote/telemetry per family plus spec_scope and the served
// fragment that move with this wave.
//
// Production binding (carried from the Wave-2 lane): dest owns the
// route (caller gate, family authorizer, response mapping) and calls
// the versioned core read API as its port transport — the shared-key
// credential and negotiation headers forwarded, no retry, no
// table/queue handles dest-side. Authz stays atomic core-side: dest
// allows only the shared-key (AllScopes) identity it can construct and
// core re-judges every forwarded call as backstop (tenant scope,
// permission catalog, governance mode — which is also where the live
// 403s and mode-dependent denials come from). The contract record is
// contracts/content-search-termination.md.
//
// Transport pin: ContentSearchReadAdmission v0
// (contracts/content-search-read-admission.md, recorded at the W4 base
// e504e328a7) covers all 4 wave rows — every row rides its re-declared
// port method over the guarded read port. Handler behavior stays
// satellite-side and unpinned: grant checks, selector resolution,
// fixed messages, QueryProfile, and hybrid rerank. This lane cites no
// other pin, and no other family may cite this pin.
//
// The routes stay inside RequireAuth: the wave gate answers every
// credential shape (shared-key terminate, foreign-class relay,
// credential-less 401), so no keyless caller gains a new public
// surface here. There is no public carve-out in this lane.

// contentSearchServeGated is the only serve kind in this lane.
const contentSearchServeGated = "gated"

// contentSearchRoute is one frozen W4 entry: the exact cross-repo
// surface this lane terminates. Serve tells the router how this lane
// serves it; Fragment names the Decision 5(c) fragment that carries the
// route's spec entry (the pinned core content fragment, ported
// line-verbatim into openapi/paths/search/content_search.go).
type contentSearchRoute struct {
	Method   string
	Path     string
	Family   string
	Serve    string
	Fragment string
}

// contentSearchWave is the frozen W4 termination target: exactly the 4
// Content search POSTs behind ContentSearchReadAdmission v0.
// TestContentSearchWaveFrozen pins it entry by entry; a wave change
// moves the ownership map, the fragment, and this mirror together.
var contentSearchWave = []contentSearchRoute{
	{Method: "POST", Path: "/api/v0/content/files/search", Family: "content", Serve: contentSearchServeGated, Fragment: "go/internal/query/openapi/paths/search/content.go"},
	{Method: "POST", Path: "/api/v0/content/entities/search", Family: "content", Serve: contentSearchServeGated, Fragment: "go/internal/query/openapi/paths/search/content.go"},
	{Method: "POST", Path: "/api/v0/content/dockerfile-installs/search", Family: "content", Serve: contentSearchServeGated, Fragment: "go/internal/query/openapi/paths/search/content.go"},
	{Method: "POST", Path: "/api/v0/content/php-config/search", Family: "content", Serve: contentSearchServeGated, Fragment: "go/internal/query/openapi/paths/search/content.go"},
}

// registerContentSearchWave mounts the W4 Content search wave gate
// ahead of the generic proxy: a valid shared key terminates on the
// native lane; a foreign credential class (scoped token, OIDC bearer,
// session cookie) relays explicitly on the relay lane so core judges
// it; credential-less callers — and junk-cookie carriers, per the D2
// follow-up lesson — get the standing 401. Each trailing-slash
// spelling stays on the wave route for the same never-split reason.
// Lives here (not in routes()) so the shared router stays under the
// file-size cap.
func (s *Server) registerContentSearchWave(mux *http.ServeMux) {
	for _, entry := range contentSearchWave {
		pattern := entry.Method + " " + entry.Path
		gated := s.instrumented(viaNative, s.contentSearchReadGate(http.HandlerFunc(s.serveContentSearchTerminated)))
		mux.Handle(pattern, gated)
		mux.Handle(pattern+"/", gated)
	}
}
