// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// Code-flow relay-leg termination (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#98, W6a). This file owns the frozen wave table:
// the four Code-flow POSTs the W6a pin covers. Behavior lives in
// code_flow_terminate.go; proofs in code_flow_conformance_test.go,
// code_flow_auth_test.go, code_flow_failclosed_test.go, and
// code_flow_latency_test.go; spec posture in
// openapi/spec_code_flow_test.go; the record in
// contracts/code-flow-termination.md.
//
// Pre-edit gate: entry point is Server.routes in api/router.go, which
// calls registerCodeFlowWave ahead of the generic proxy; phase order
// is pin record first, wave registration second, parity/latency
// proofs third; data dependency is the core W6a pin at e504e328a7
// (contract text, v0, exact 1-method set, transport constructor);
// re-trigger is dispatchNote/telemetry per family plus spec_scope and
// openapi disclosed-absence pins that move with this wave.
//
// Production binding (carried from the D0 shim proof via the D2-D6
// waves and the gap-04 lane): dest owns the route (caller gate,
// family authorizer, response mapping) and calls the versioned core
// read API as its port transport — bounded client, forwarded
// credential and negotiation headers, no retry, no table/queue
// handles dest-side. Authz stays atomic core-side: dest allows only
// the shared-key (AllScopes) identity it can construct and core
// re-judges every forwarded call as backstop.
//
// Transport pin: CodeFlowReadAdmission v0
// (contracts/code-flow-read-admission.md, recorded at core merge
// eshu-hq/eshu#10552, f23e83111b, source e504e328a7) covers the four
// routes' bounded ListCodeFlow reads (Kind cfg_summary, pdg_summary,
// reaching_def, taint_path). Handler behavior stays satellite-side
// and unpinned: grant checks, repo selector, unsupported-language
// short-circuit, fixed messages, QueryProfile. This lane cites no
// other pin, and no other family may cite this pin.
//
// The routes stay inside RequireAuth: the wave gate answers every
// credential shape (shared-key terminate, foreign-class relay,
// credential-less 401), so no keyless caller gains a new public
// surface here. There is no public carve-out in this lane.

// CodeFlow satellite serve kind. The routes are wave-gated: a shared
// key terminates, unresolvable credential classes relay explicitly,
// credential-less callers (and junk-cookie carriers) keep the
// standing dest 401.
const codeFlowServeGated = "gated"

// codeFlowRoute is one frozen W6a wave entry: the exact cross-repo
// surface this lane terminates. Serve tells the router how this lane
// serves it; Fragment names the core-side Decision 5(c) fragment the
// follow-up spec-port lane will vendor — this lane ports no fragment
// (the spec absences stay disclosed, see
// contracts/code-flow-termination.md), so the field carries the core
// path, not a satellite one.
type codeFlowRoute struct {
	Method   string
	Path     string
	Family   string
	Serve    string
	Fragment string
}

// codeFlowWave is the frozen W6a termination target: the four
// Code-flow POSTs behind CodeFlowReadAdmission v0.
// TestCodeFlowWaveFrozen pins it entry by entry; a wave change moves
// the ownership note, the fragment reference, and this mirror
// together.
var codeFlowWave = []codeFlowRoute{
	{Method: "POST", Path: "/api/v0/code/flow/cfg-summary", Family: "code_flow", Serve: codeFlowServeGated, Fragment: "go/internal/query/openapi/paths/code/flow.go"},
	{Method: "POST", Path: "/api/v0/code/flow/pdg-summary", Family: "code_flow", Serve: codeFlowServeGated, Fragment: "go/internal/query/openapi/paths/code/flow.go"},
	{Method: "POST", Path: "/api/v0/code/flow/reaching-def", Family: "code_flow", Serve: codeFlowServeGated, Fragment: "go/internal/query/openapi/paths/code/flow.go"},
	{Method: "POST", Path: "/api/v0/code/flow/taint-path", Family: "code_flow", Serve: codeFlowServeGated, Fragment: "go/internal/query/openapi/paths/code/flow.go"},
}

// registerCodeFlowWave mounts the W6a Code-flow wave gate ahead of
// the generic proxy: shared-key callers terminate, foreign classes
// relay explicitly, credential-less callers keep the standing 401,
// and each slash twin rides along never-split. Lives here (not in
// routes()) so the shared router stays under the file-size cap.
func (s *Server) registerCodeFlowWave(mux *http.ServeMux) {
	for _, entry := range codeFlowWave {
		pattern := entry.Method + " " + entry.Path
		gated := s.instrumented(viaNative, s.codeFlowReadGate(http.HandlerFunc(s.serveCodeFlowTerminated)))
		mux.Handle(pattern, gated)
		mux.Handle(pattern+"/", gated)
	}
}
