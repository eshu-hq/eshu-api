// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"

	apilibadmin "github.com/eshu-hq/eshu/go/apilib/admin"
)

// Admin work-items query native termination (satellite
// eshu-hq/eshu-api#335, the first D3 store-backed promotion). This
// file owns the serving contract for the one Admin POST the
// promotion lifts out of the D3 wave: the bounded work-items query.
// The route rides the untouched D3 wave gate in d3_terminate.go
// (AUTH peer owns that file): a valid shared key terminates locally
// on the pinned apilib body served from the reader-role PG store,
// while foreign credential classes relay explicitly and
// credential-less callers keep the standing 401 — the same matrix
// the wave registration served, with the terminal swapped from the
// core forward to the local store read.
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
// Store rule: the reader is SELECT-only on fact_work_items (see
// internal/adminread). A nil store is the unconfigured lane, not a
// defect: it answers the apilib 503 locally and never forwards.
//
// Admission pin: the apilib work-items contract v0 (the pinned
// external apilib v0.1.0, no replace). The served bytes are the
// apilib ServeWorkItemsQuery rendering of the reader rows: {count,
// items} with core field/RFC3339/pointer-omission parity — the
// apilib handler owns that shape, this lane owns only the gate, the
// store handle, and the telemetry.

// adminWorkItemsRoute is the promoted singleton: the work-items
// query POST. Registered ahead of the D3 loops with its
// trailing-slash twin on the same gate (never-split); deeper
// subtrees and non-POST spellings stay on the generic relay.
const adminWorkItemsRoute = "POST /api/v0/admin/work-items/query"

// WithWorkItemStore attaches store as this server's reader-role
// work-items source and returns the server for chaining. A nil store
// is the unconfigured lane: shared-key calls answer the apilib local
// 503 and never forward. The binary wires this from the
// ESHU_API_ADMIN_READ_DSN environment; tests attach fakes.
func (s *Server) WithWorkItemStore(store apilibadmin.WorkItemStore) *Server {
	s.workItems = store
	return s
}

// registerAdminWorkItemsQuery mounts the promoted route explicitly
// inside the D3 wave gate, ahead of the D3 loops and the generic
// proxy. Lives here (not in routes()) so the shared router stays
// under the file-size cap — the sign-in-policy precedent.
func (s *Server) registerAdminWorkItemsQuery(mux *http.ServeMux) {
	gated := s.instrumented(viaNative, s.d3ReadGate(http.HandlerFunc(s.serveAdminWorkItemsTerminated)))
	mux.Handle(adminWorkItemsRoute, gated)
	mux.Handle(adminWorkItemsRoute+"/", gated)
}

// serveAdminWorkItemsTerminated terminates the work-items query for
// a shared-key caller: the family authorizer runs first, then the
// query serves locally from the reader-role store with no core
// contact.
func (s *Server) serveAdminWorkItemsTerminated(w http.ResponseWriter, r *http.Request) {
	s.serveAdminWorkItemsTerminatedAs(w, r, true)
}

// serveAdminWorkItemsTerminatedAs is the testable core of the
// terminated reader: allScopes is the resolved caller scope. The
// wave gate always passes true today (the shared key maps to
// AllScopes in core); the false branch is the hookup point for the
// day dest gains a scoped/session resolver, and it stays a plain
// deny until then. Like dispatch, it fails closed with the permanent
// 503 when no core origin is configured, so the unconfigured edge
// keeps the wave behavior bit for bit.
func (s *Server) serveAdminWorkItemsTerminatedAs(w http.ResponseWriter, r *http.Request, allScopes bool) {
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
	// the apilib bounded-list rendering of the reader rows. A nil
	// store is the unconfigured lane: apilib answers its 503 locally
	// and the call never forwards.
	s.dispatchOutcome(r, viaNative, outcomeLocalOK, 0, false)
	apilibadmin.ServeWorkItemsQuery(w, r, s.workItems, nil)
}
