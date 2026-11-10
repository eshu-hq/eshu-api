// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// Wave-1 Freshness reads termination (Refs eshu-hq/eshu#9669, D13
// tracker eshu-hq/eshu#10491, satellite eshu-hq/eshu-api#72). This
// file owns the frozen wave table: exactly the 3 Freshness GETs the
// merged core pin FreshnessReadAdmission v0 admits (core 51b84b2 —
// the bounded generation-lifecycle page plus the two bounded
// changed-since delta summaries). A wave change moves the ownership
// map, the spec fragments, and this mirror together. The behavior
// lives in freshness_terminate.go; the proofs in
// freshness_conformance_test.go and the per-family suites.
//
// Production binding (carried from the D0 shim proof via the D2/D4/D6
// waves): dest owns each route (caller gate, family authorizer,
// response mapping) and calls the versioned core read API as its port
// transport — the shared-key credential and negotiation headers
// forwarded, no retry, no table/queue handles dest-side. Authz stays
// atomic core-side: dest allows only the shared-key (AllScopes)
// identity it can construct and core re-judges every forwarded call as
// backstop (tenant scope, permission catalog, governance mode — which
// is also where the live 403s and mode-dependent denials come from).
// The contract record is contracts/freshness-wave.md.
//
// Transport pin: FreshnessReadAdmission v0 (contracts/upstream/
// freshness_read_admission_contract_test.go at the Wave-1 merge SHA)
// covers all 3 wave rows — every row admits through its re-declared
// port method over the guarded read port. The minting rule forbids
// inventing pins beyond it; the ledger, summary-reader, and snapshot
// legs stay out per the pin record and stay relay with the reason
// disclosed per route in the contract.
//
// D-wave satellite serve kind. Every Wave-1 route is wave-gated:
// shared-key callers terminate, unresolvable credential classes relay
// explicitly, credential-less callers (and junk-cookie carriers, per
// the D2 follow-up lesson) keep the standing 401. There is no public
// carve-out in this lane — every Freshness read is an authed
// reader-port read.

// freshnessServeGated is the only serve kind in this lane.
const freshnessServeGated = "gated"

// Freshness wave route paths: the exact core route paths the pin
// admits. The wave table references these so the frozen test, the
// router registration, and the terminated-handler switch move
// together.
const (
	freshnessChangedSincePath        = "/api/v0/freshness/changed-since"
	freshnessGenerationsPath         = "/api/v0/freshness/generations"
	freshnessServiceChangedSincePath = "/api/v0/freshness/services/changed-since"
)

// freshnessRoute is one frozen Wave-1 entry: the exact cross-repo
// surface this lane terminates. Method+Path+Family mirror the CUTOVER
// rows; Serve tells the router how this lane serves it; Fragment names
// the Decision 5(c) fragment that carries the route's spec entry.
type freshnessRoute struct {
	Method   string
	Path     string
	Family   string
	Serve    string
	Fragment string
}

// freshnessWave is the frozen Wave-1 termination target: exactly the
// 3 Freshness GETs behind FreshnessReadAdmission v0.
// TestFreshnessWaveFrozen pins it entry by entry; a wave change moves
// the ownership map, the fragments, and this mirror together.
var freshnessWave = []freshnessRoute{
	{Method: "GET", Path: freshnessChangedSincePath, Family: "freshness", Serve: freshnessServeGated, Fragment: "paths/freshness/changed_since.go"},
	{Method: "GET", Path: freshnessGenerationsPath, Family: "freshness", Serve: freshnessServeGated, Fragment: "paths/freshness/generations.go"},
	{Method: "GET", Path: freshnessServiceChangedSincePath, Family: "freshness", Serve: freshnessServeGated, Fragment: "paths/freshness/service_changed_since.go"},
}

// registerFreshnessWave mounts the Wave-1 Freshness reads termination
// (Refs eshu-hq/eshu#9669): the 3 pinned read routes register
// explicitly inside the wave gate, ahead of the generic proxy. A valid
// shared key terminates on the native lane; a foreign credential class
// (scoped token, OIDC bearer, session cookie) relays explicitly on the
// relay lane so core judges it; credential-less callers — and
// junk-cookie carriers, per the D2 follow-up lesson — get the standing
// 401. Each trailing-slash spelling stays on the wave route for the
// same never-split reason. Every other family still crosses its
// existing lane.
func (s *Server) registerFreshnessWave(mux *http.ServeMux) {
	for _, entry := range freshnessWave {
		pattern := entry.Method + " " + entry.Path
		gated := s.instrumented(viaNative, s.freshnessReadGate(http.HandlerFunc(s.freshnessTerminatedHandler(entry.Path))))
		mux.Handle(pattern, gated)
		mux.Handle(pattern+"/", gated)
	}
}
