// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"

	apilibadmin "github.com/eshu-hq/eshu/go/apilib/admin"
)

// Admin shared-projection tuning-report native termination
// (satellite eshu-hq/eshu-api#171, part of the D3 Gap 02 trio
// promotion). This file owns the serving contract for the one Admin
// GET the promotion lifts out of the D3 trio wave: the static
// shared-projection tuning report. The route rides the untouched D3
// wave gate in d3_terminate.go (AUTH peer owns that file): a valid
// shared key terminates locally on the pinned apilib static body,
// while foreign credential classes relay explicitly and
// credential-less callers keep the standing 401 — the same matrix
// the trio registration served, with the terminal swapped from the
// core forward to the local apilib write.
//
// Lane rule (arbiter D-wave ruling, decision 4, via the D3 wave):
// shared-key callers terminate locally with zero core and zero store
// contact; credential classes dest cannot resolve (scoped tokens,
// OIDC bearers, browser sessions) relay explicitly — core judges,
// core bytes come back untouched, dest invents no 401, 403, 404, or
// 500 for them; credential-less callers get the standing dest 401.
// The only dest-synthesized errors on the terminated path are the
// gate 401 and the permanent local 503 when no core origin is
// configured (never Retry-After).
//
// Cookie rule (D2 follow-up lesson): only session-cookie-shaped
// credentials relay — the gate reuses hasSessionCookie, so the
// credential matrix is bit-identical to the trio loop this route
// leaves. An empty-valued session cookie stays credential-less
// (dest 401 with zero core contact), exactly as today.
//
// Admission pin: SharedProjectionTuningReport v0
// (SharedProjectionTuningReportContractVersion, in lockstep with the
// core pin at eshu-hq/eshu#10412). The served bytes are byte-identical
// to the core handler's body: same pinned constants, same sorted-key
// map encoding, same HTML-escaping setting, same trailing newline —
// the apilib WriteTuningReport writer owns that shape, this lane
// owns only the gate and the telemetry.

// adminTuningReportRoute is the promoted singleton: the static
// shared-projection tuning-report GET. Registered ahead of the D3
// loops with its trailing-slash twin on the same gate (never-split);
// deeper subtrees and non-GET spellings stay on the generic relay.
const adminTuningReportRoute = "GET /api/v0/admin/shared-projection/tuning-report"

// registerAdminTuningReport mounts the promoted route explicitly
// inside the D3 wave gate, ahead of the D3 loops and the generic
// proxy. Lives here (not in routes()) so the shared router stays
// under the file-size cap — the sign-in-policy precedent.
func (s *Server) registerAdminTuningReport(mux *http.ServeMux) {
	gated := s.instrumented(viaNative, s.d3ReadGate(http.HandlerFunc(s.serveAdminTuningTerminated)))
	mux.Handle(adminTuningReportRoute, gated)
	mux.Handle(adminTuningReportRoute+"/", gated)
}

// serveAdminTuningTerminated terminates the tuning report for a
// shared-key caller: the family authorizer runs first, then the
// static body serves locally with no core and no store contact.
func (s *Server) serveAdminTuningTerminated(w http.ResponseWriter, r *http.Request) {
	s.serveAdminTuningTerminatedAs(w, r, true)
}

// serveAdminTuningTerminatedAs is the testable core of the terminated
// reader: allScopes is the resolved caller scope. The wave gate
// always passes true today (the shared key maps to AllScopes in
// core); the false branch is the hookup point for the day dest gains
// a scoped/session resolver, and it stays a plain deny until then.
// Like dispatch, it fails closed with the permanent 503 when no core
// origin is configured, so the unconfigured edge keeps the trio
// behavior bit for bit.
func (s *Server) serveAdminTuningTerminatedAs(w http.ResponseWriter, r *http.Request, allScopes bool) {
	if s.core == nil {
		s.dispatchOutcome(r, viaNative, outcomeLocal503, 0, false)
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	// The family authorizer judgment counts on both branches, so the
	// unreachable deny branch stays observable from dest signals. A
	// deny never dispatches: it files the denied outcome on the native
	// lane. The authorizer is the shared D3 trust root (see
	// api/d3_terminate.go): one trust root for the admin read family
	// until D8 resolves per-class handling.
	family := classifyFamily(r.URL.Path)
	if !authorizeD3SharedKey(allScopes) {
		if tel := s.telemetry; tel != nil {
			tel.observeAuthorizer(family, decisionDeny)
		}
		s.dispatchOutcome(r, viaNative, outcomeDenied, 0, false)
		writeError(w, r, http.StatusForbidden, http.StatusText(http.StatusForbidden),
			d3FamilyDenyMessage)
		return
	}
	if tel := s.telemetry; tel != nil {
		tel.observeAuthorizer(family, decisionAllow)
	}
	// Local termination: the lane files local_ok (never a core
	// outcome — no core answered) with zero core latency, then serves
	// the pinned apilib bytes. No pool checkout, no store span.
	s.dispatchOutcome(r, viaNative, outcomeLocalOK, 0, false)
	apilibadmin.WriteTuningReport(w, r)
}
