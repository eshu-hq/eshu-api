// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"crypto/subtle"
	"net/http"
)

// W7-read admin sign-in-policy GET termination engine (Refs
// eshu-hq/eshu#9669, D13 tracker eshu-hq/eshu#10491, satellite
// eshu-hq/eshu-api#72). The wave table lives below; this file owns
// the serving contract: the family wave gate and the terminated
// reader. The route previously rode the generic D2 Leaf B wave gate;
// this lane promotes it to family ownership under the
// SignInPolicyReadAdmission v0 pin (core eshu-hq/eshu#10432,
// satellite record contracts/upstream/
// sign_in_policy_read_admission_contract_test.go) with zero behavior
// change: same credential matrix, same forward, same bytes. The
// family's PATCH terminates natively behind its family writer gate
// (see api/signin.go); trailing-slash twins ride the same family
// route for the never-split reason; deeper subtrees and non-GET
// spellings stay on the generic relay behind RequireAuth.
//
// Lane rule (arbiter D-wave ruling, decision 4, via the D2/D5 waves):
// shared-key callers terminate natively; credential classes dest
// cannot resolve (scoped tokens, OIDC bearers, browser sessions)
// relay explicitly — core judges, core bytes come back untouched,
// dest invents no 401, 403, 404, or 500 for them; credential-less
// callers get the standing dest 401. The only dest-synthesized
// errors on the terminated path are the gate 401, the
// future-resolver family 403, and the permanent local 503 when no
// core origin is configured (never Retry-After).
//
// Cookie rule: the gate reuses hasLeafBSessionCookie from the D2
// lane — name-matched, non-empty value — so the credential matrix
// is bit-identical to the LeafB loop this route leaves. An
// empty-valued session cookie stays credential-less (dest 401 with
// zero core contact), exactly as today.

// adminSignInPolicyReadWave is the W7-read family wave: the single
// admin GET the lane terminates. It reuses the leafbRoute census
// shape with the wave-family serve kind so the LeafB frozen table
// keeps mirroring the core nativeNowWave entry by entry.
var adminSignInPolicyReadWave = []leafbRoute{
	{Method: "GET", Path: "/api/v0/auth/admin/sign-in-policy", Family: "sign_in_policy", Serve: leafbServeFamily, Fragment: "paths/auth/sign_in_policy.go"},
}

// registerAdminSignInPolicyRead registers the family wave explicitly
// inside the family gate, ahead of the generic proxy. Lives here (not
// in routes()) so the shared router stays under the file-size cap.
func (s *Server) registerAdminSignInPolicyRead(mux *http.ServeMux) {
	for _, entry := range adminSignInPolicyReadWave {
		route := entry.Method + " " + entry.Path
		gated := s.instrumented(viaNative, s.adminSignInPolicyReadGate(http.HandlerFunc(s.adminSignInPolicyTerminatedHandler(route))))
		mux.Handle(route, gated)
		mux.Handle(route+"/", gated)
	}
}

// serveAdminSignInPolicyTerminated terminates one admin
// sign-in-policy GET for a shared-key caller: the family authorizer
// runs first, then the call crosses the versioned port transport
// (forwardToCore on the native lane) with the caller's negotiation
// headers and credential intact and core bytes come back unmodified.
func (s *Server) serveAdminSignInPolicyTerminated(w http.ResponseWriter, r *http.Request) {
	s.serveAdminSignInPolicyTerminatedAs(w, r, true)
}

// serveAdminSignInPolicyTerminatedAs is the testable core of the
// terminated reader: allScopes is the resolved caller scope. The
// family gate always passes true today (the shared key maps to
// AllScopes in core); the false branch is the hookup point for the
// day dest gains a scoped/session resolver, and it stays a plain
// deny until then. Like dispatch, it fails closed with the permanent
// 503 when no core origin is configured, so a native route can never
// dereference an unconfigured origin.
func (s *Server) serveAdminSignInPolicyTerminatedAs(w http.ResponseWriter, r *http.Request, allScopes bool) {
	if s.core == nil {
		s.dispatchOutcome(r, viaNative, outcomeLocal503, 0, false)
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	// The family authorizer judgment counts on both branches, so the
	// unreachable deny branch stays observable from dest signals. A
	// deny never dispatches: it files the denied outcome on the native
	// lane. The authorizer is the sign-in-policy family authorizer
	// shared with the PATCH write-through (see api/signin.go): one
	// trust root for the family's authed legs.
	family := classifyFamily(r.URL.Path)
	if !authorizeSignInPolicyAdmin(allScopes) {
		if tel := s.telemetry; tel != nil {
			tel.observeAuthorizer(family, decisionDeny)
		}
		s.dispatchOutcome(r, viaNative, outcomeDenied, 0, false)
		writeError(w, r, http.StatusForbidden, http.StatusText(http.StatusForbidden),
			"all-scopes operator authorization is required")
		return
	}
	if tel := s.telemetry; tel != nil {
		tel.observeAuthorizer(family, decisionAllow)
	}
	forwardToCore(s, w, r, viaNative)
}

// adminSignInPolicyTerminatedHandler selects the terminated handler
// for one family wave route: the admin GET. Any other route fails
// closed with 404 instead of silently riding the leg: routes() only
// passes family wave entries, so a future table add without a
// handler here breaks loudly.
func (s *Server) adminSignInPolicyTerminatedHandler(route string) func(http.ResponseWriter, *http.Request) {
	switch route {
	case "GET /api/v0/auth/admin/sign-in-policy":
		return s.serveAdminSignInPolicyTerminated
	default:
		return s.serveAdminSignInPolicyUnknownRouteTerminated
	}
}

// serveAdminSignInPolicyUnknownRouteTerminated answers any route the
// family table does not know: 404, never a leg handler. Unreachable
// through routes(); it makes a future table add without a handler
// fail loudly instead of silently riding the leg.
func (s *Server) serveAdminSignInPolicyUnknownRouteTerminated(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, http.StatusNotFound, http.StatusText(http.StatusNotFound),
		"eshu-api: unknown admin sign-in-policy terminated route; no core routes are served")
}

// adminSignInPolicyReadGate fronts the family wave route. A valid
// shared key terminates (next); a request carrying any other
// credential — a non-key Bearer (scoped token, OIDC) or a
// session-cookie-shaped Cookie — relays explicitly to the same core
// route on the relay lane, so core judges the class dest cannot
// resolve and dest invents no failure for it; a credential-less
// request (or a junk-cookie carrier, per the D2 follow-up lesson)
// gets the standing dest 401 with zero core contact. An empty
// configured key fails closed like RequireAuth.
func (s *Server) adminSignInPolicyReadGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.apiKey == "" {
			WriteUnauthorized(w, r)
			return
		}
		if got, ok := bearerKey(r); ok {
			if subtle.ConstantTimeCompare([]byte(got), []byte(s.apiKey)) == 1 {
				next.ServeHTTP(w, r)
				return
			}
			// A non-key Bearer [REDACTED] a foreign credential class (scoped
			// token, OIDC): relay explicitly so core judges it.
			s.adminSignInPolicyRelayExplicit(w, r)
			return
		}
		if hasLeafBSessionCookie(r) {
			// A session-cookie-shaped Cookie without a Bearer [REDACTED] a
			// browser-session caller: relay explicitly so core judges
			// it. Junk cookies fall through to the standing 401 below.
			s.adminSignInPolicyRelayExplicit(w, r)
			return
		}
		WriteUnauthorized(w, r)
	})
}

// adminSignInPolicyRelayExplicit forwards one family call carrying an
// unresolvable credential class to the same core route and relays
// core bytes untouched on the relay lane. The instrumented middleware
// files the lane from the record, so the gate flips the record lane
// before forwarding; direct (uninstrumented) calls already carry the
// relay lane through forwardToCore.
func (s *Server) adminSignInPolicyRelayExplicit(w http.ResponseWriter, r *http.Request) {
	if rec := dispatchNote(r); rec != nil {
		rec.via = viaRelay
	}
	if s.core == nil {
		s.dispatchOutcome(r, viaRelay, outcomeLocal503, 0, false)
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	forwardToCore(s, w, r, viaRelay)
}
