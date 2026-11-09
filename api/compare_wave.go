// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// Compare relay-leg termination (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#72, gap 04). This file owns the frozen wave table:
// the single Compare POST the deliberate later pin covers. The
// behavior lives in compare_terminate.go; the proofs in
// compare_conformance_test.go, compare_auth_test.go,
// compare_failclosed_test.go, and compare_latency_test.go; the spec
// posture in openapi/spec_compare_test.go; the record in
// contracts/compare-termination.md.
//
// Production binding (carried from the D0 shim proof via the D2-D6
// waves): dest owns the route (caller gate, family authorizer,
// response mapping) and calls the versioned core read API as its port
// transport — bounded client, forwarded credential and negotiation
// headers, no retry, no table/queue handles dest-side. Authz stays
// atomic core-side: dest allows only the shared-key (AllScopes)
// identity it can construct and core re-judges every forwarded call
// as backstop (tenant scope, permission catalog, governance mode).
//
// Transport pin: CompareReadAdmission v0
// (contracts/compare-read-admission.md, recorded at the core merge
// eshu-hq/eshu#10425) covers this route's workload and
// environment-snapshot graph reads (Run, RunSingle) and its
// service-evidence content reads (ListRepoFiles, GetFileContent).
// The minting rule forbids pinning the shared graph/content ports
// directly; the pin re-declares the four reads in this
// Compare-scoped shape instead, so no other family can cite it as
// coverage — and this lane cites no other pin.
//
// The route stays inside RequireAuth: the wave gate below answers
// every credential shape (shared-key terminate, foreign-class relay,
// credential-less 401), so no keyless caller gains a new public
// surface here. There is no public carve-out in this lane — the
// Compare POST is an authed read.

// Compare satellite serve kind. The route is wave-gated: a shared
// key terminates, unresolvable credential classes relay explicitly,
// credential-less callers (and junk-cookie carriers) keep the
// standing dest 401.
const compareServeGated = "gated"

// compareRoute is one frozen Compare wave entry: the exact
// cross-repo surface this lane terminates. Serve tells the router
// how this lane serves it; Fragment names the core-side Decision
// 5(c) fragment the follow-up spec-port lane will vendor — this
// lane ports no fragment (the spec absence stays disclosed, see
// contracts/compare-termination.md), so the field carries the core
// path, not a satellite one.
type compareRoute struct {
	Method   string
	Path     string
	Family   string
	Serve    string
	Fragment string
}

// compareWave is the frozen Compare termination target: the single
// Compare POST behind CompareReadAdmission v0.
// TestCompareWaveFrozen pins it entry by entry; a wave change moves
// the ownership note, the fragment reference, and this mirror
// together.
var compareWave = []compareRoute{
	{Method: "POST", Path: "/api/v0/compare/environments", Family: "compare", Serve: compareServeGated, Fragment: "go/internal/query/openapi/paths/status/compare.go"},
}

// registerCompareWave mounts the gap-04 Compare wave gate ahead of the
// generic proxy: shared-key callers terminate, foreign classes relay
// explicitly, credential-less callers keep the standing 401, and the
// slash twin rides along never-split. Lives here (not in routes())
// so the shared router stays under the file-size cap.
func (s *Server) registerCompareWave(mux *http.ServeMux) {
	for _, entry := range compareWave {
		pattern := entry.Method + " " + entry.Path
		gated := s.instrumented(viaNative, s.compareReadGate(http.HandlerFunc(s.serveCompareTerminated)))
		mux.Handle(pattern, gated)
		mux.Handle(pattern+"/", gated)
	}
}
