// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"

	apilibadmin "github.com/eshu-hq/eshu/go/apilib/admin"
)

// Admin decisions query native termination (satellite
// eshu-hq/eshu-api#325, the third D3 store-backed promotion). This
// file owns the serving contract for the one Admin POST the
// promotion lifts out of the D3 wave: the bounded decisions query
// with its per-decision evidence fan-out. The route rides the
// untouched D3 wave gate in d3_terminate.go (AUTH peer owns that
// file): a valid shared key terminates locally on the pinned apilib
// body served from the reader-role PG store, while foreign
// credential classes relay explicitly and credential-less callers
// keep the standing 401 — the same matrix the wave registration
// served, with the terminal swapped from the core forward to the
// local store read.
//
// Lane rule (arbiter D-wave ruling, decision 4, via the D3 wave):
// shared-key callers terminate locally with zero core contact;
// credential classes dest cannot resolve (scoped tokens, OIDC
// bearers, browser sessions) relay explicitly — core judges, core
// bytes come back untouched, dest invents no 401, 403, 404, or 500
// for them; credential-less callers get the standing dest 401. A
// missing store answers the apilib local 503 (never a forward); a
// failed store answers the apilib fixed 500 (never the backend
// text). The only other dest-synthesized errors on the terminated
// path are the gate 401, the family 403, and the permanent local 503
// when no core origin is configured (never Retry-After).
//
// Cookie rule (D2 follow-up lesson): only session-cookie-shaped
// credentials relay — the gate reuses hasSessionCookie, so the
// credential matrix is bit-identical to the wave loop this route
// leaves. An empty-valued session cookie stays credential-less
// (dest 401 with zero core contact), exactly as today.
//
// Store rule: the reader is SELECT-only over projection_decisions
// plus the per-decision projection_decision_evidence fan-out (see
// internal/adminread decisions.go). A nil store is the unconfigured
// lane, not a defect: it answers the apilib 503 locally and never
// forwards.
//
// Required fields (core parity, served by the pinned apilib v0.3.0,
// no dest-side fork): repository_id and source_run_id are required
// with the core-identical detail; an absent limit defaults to 100,
// like core.
//
// Admission pin: the apilib decisions contract v0 (the pinned
// external apilib v0.3.0, no replace). The served bytes are the
// apilib ServeDecisionsQuery rendering of the reader rows: {count,
// decisions} with core field/RFC3339/pointer-omission parity plus
// the joined evidence — the apilib handler owns that shape, this
// lane owns only the gate, the store handle, and the telemetry.

// adminDecisionsRoute is the promoted singleton: the decisions
// query POST. Registered ahead of the D3 loops with its
// trailing-slash twin on the same gate (never-split); deeper
// subtrees and non-POST spellings stay on the generic relay.
const adminDecisionsRoute = "POST /api/v0/admin/decisions/query"

// WithDecisionStore attaches store as this server's reader-role
// decision source and returns the server for chaining. A nil store
// is the unconfigured lane: shared-key calls answer the apilib local
// 503 and never forward. The binary wires this from the
// ESHU_API_ADMIN_READ_DSN environment; tests attach fakes.
func (s *Server) WithDecisionStore(store apilibadmin.DecisionStore) *Server {
	s.decisions = store
	return s
}

// registerAdminDecisionsQuery mounts the promoted route explicitly
// inside the D3 wave gate, ahead of the D3 loops and the generic
// proxy. Lives here (not in routes()) so the shared router stays
// under the file-size cap — the sign-in-policy precedent.
func (s *Server) registerAdminDecisionsQuery(mux *http.ServeMux) {
	gated := s.instrumented(viaNative, s.d3ReadGate(http.HandlerFunc(s.serveAdminDecisionsTerminated)))
	mux.Handle(adminDecisionsRoute, gated)
	mux.Handle(adminDecisionsRoute+"/", gated)
}

// serveAdminDecisionsTerminated terminates the decisions query for
// a shared-key caller: the family authorizer runs first, then the
// query serves locally from the reader-role store with no core
// contact.
func (s *Server) serveAdminDecisionsTerminated(w http.ResponseWriter, r *http.Request) {
	s.serveAdminDecisionsTerminatedAs(w, r, true)
}

// serveAdminDecisionsTerminatedAs is the testable core of the
// terminated reader: allScopes is the resolved caller scope. The
// wave gate always passes true today (the shared key maps to
// AllScopes in core); the false branch is the hookup point for the
// day dest gains a scoped/session resolver, and it stays a plain
// deny until then. Like dispatch, it fails closed with the permanent
// 503 when no core origin is configured, so the unconfigured edge
// keeps the wave behavior bit for bit.
func (s *Server) serveAdminDecisionsTerminatedAs(w http.ResponseWriter, r *http.Request, allScopes bool) {
	if s.core == nil {
		s.dispatchOutcome(r, viaNative, outcomeLocal503, 0, false)
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	// The family authorizer judgment counts on both branches, so the
	// unreachable deny branch stays observable from dest signals. A
	// deny never dispatches: it files the denied outcome on the
	// native lane. The authorizer is the shared D3 trust root (see
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
	// the apilib bounded-list rendering of the reader rows, including
	// the per-decision evidence fan-out when requested. A nil store
	// is the unconfigured lane: apilib answers its 503 locally and
	// the call never forwards. The required repository_id and
	// source_run_id validate inside the pinned apilib with the
	// core-identical 400, so this lane forks no validation of its
	// own.
	s.dispatchOutcome(r, viaNative, outcomeLocalOK, 0, false)
	apilibadmin.ServeDecisionsQuery(w, r, s.decisions, nil)
}
