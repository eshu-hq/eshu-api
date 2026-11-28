// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"

	apilibadmin "github.com/eshu-hq/eshu/go/apilib/admin"
)

// Admin reindex-progress native termination (satellite
// eshu-hq/eshu-api#170, the third D3 store-backed promotion, lifting
// the last Gap 02 trio GET out of the D3 wave). This file owns the
// serving contract for the one Admin GET the promotion lifts out of
// the D3 trio wave: the fleet reindex-progress read. The route rides
// the untouched D3 wave gate in d3_terminate.go (AUTH peer owns that
// file): a valid shared key terminates locally on the pinned apilib
// body served from the reader-role PG store, while foreign
// credential classes relay explicitly and credential-less callers
// keep the standing 401 — the same matrix the trio registration
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
// credential matrix is bit-identical to the trio loop this route
// leaves. An empty-valued session cookie stays credential-less
// (dest 401 with zero core contact), exactly as today.
//
// Store rule: the reader is SELECT-only over runtime_ingester_control,
// repository_reindex_requests, scope_generations, and
// ingestion_scopes for the fleet and per-scope watermarks (see
// internal/adminread/reindexprogress.go). A nil store is the
// unconfigured lane, not a defect: it answers the apilib 503 locally
// and never forwards.
//
// Progress validation (core parity, preserved dest-side by the
// pinned apilib v0.3.0): scope_id is required (1-100 scope IDs, none
// blank, trimmed, deduplicated, sorted) and fleet_requested_at must
// be RFC 3339 when present — the apilib renders the core-identical
// detail strings, this lane owns only the gate, the store handle, and
// the telemetry.
//
// Admission pin: the apilib reindex-progress contract v0 (the pinned
// external apilib v0.3.0, no replace). The served bytes are the
// apilib ServeReindexProgress rendering of the reader rows:
// {status, fleet_requested_at, counts, scopes, truncated} with core
// field/RFC3339 parity — the apilib handler owns that shape, this
// lane owns only the gate, the store handle, and the telemetry.

// adminReindexProgressRoute is the promoted singleton: the
// reindex-progress GET. Registered ahead of the D3 loops with its
// trailing-slash twin on the same gate (never-split); deeper
// subtrees and non-GET spellings stay on the generic relay.
const adminReindexProgressRoute = "GET /api/v0/admin/reindex/progress"

// WithReindexProgress attaches store as this server's reader-role
// reindex-progress source and returns the server for chaining. A nil
// store is the unconfigured lane: shared-key calls answer the apilib
// local 503 and never forward. The binary wires this from the
// ESHU_API_ADMIN_READ_DSN environment; tests attach fakes.
func (s *Server) WithReindexProgress(store apilibadmin.MaintenanceStore) *Server {
	s.reindexProgress = store
	return s
}

// registerAdminReindexProgress mounts the promoted route explicitly
// inside the D3 wave gate, ahead of the D3 loops and the generic
// proxy. Lives here (not in routes()) so the shared router stays
// under the file-size cap — the sign-in-policy precedent.
func (s *Server) registerAdminReindexProgress(mux *http.ServeMux) {
	gated := s.instrumented(viaNative, s.d3ReadGate(http.HandlerFunc(s.serveAdminReindexProgressTerminated)))
	mux.Handle(adminReindexProgressRoute, gated)
	mux.Handle(adminReindexProgressRoute+"/", gated)
}

// serveAdminReindexProgressTerminated terminates the reindex-progress
// read for a shared-key caller: the family authorizer runs first,
// then the progress report serves locally from the reader-role store
// with no core contact.
func (s *Server) serveAdminReindexProgressTerminated(w http.ResponseWriter, r *http.Request) {
	s.serveAdminReindexProgressTerminatedAs(w, r, true)
}

// serveAdminReindexProgressTerminatedAs is the testable core of the
// terminated reader: allScopes is the resolved caller scope. The
// wave gate always passes true today (the shared key maps to
// AllScopes in core); the false branch is the hookup point for the
// day dest gains a scoped/session resolver, and it stays a plain
// deny until then. Like dispatch, it fails closed with the permanent
// 503 when no core origin is configured, so the unconfigured edge
// keeps the wave behavior bit for bit.
func (s *Server) serveAdminReindexProgressTerminatedAs(w http.ResponseWriter, r *http.Request, allScopes bool) {
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
	// the apilib progress rendering of the reader rows. A nil store
	// is the unconfigured lane: apilib answers its 503 locally and
	// the call never forwards. The apilib validates scope_id and
	// fleet_requested_at with the core-identical 400s first.
	s.dispatchOutcome(r, viaNative, outcomeLocalOK, 0, false)
	apilibadmin.ServeReindexProgress(w, r, s.reindexProgress, nil)
}
