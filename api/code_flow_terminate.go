// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"crypto/subtle"
	"net/http"
)

// Code-flow relay-leg termination engine (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#98, W6a). The wave table lives in
// code_flow_wave.go; this file owns the serving contract: the wave
// gate and the terminated reader.
//
// Lane rule (arbiter W-wave ruling, APPROVED-WITH-CONDITIONS):
// shared-key callers terminate natively; credential classes dest
// cannot resolve (scoped tokens, OIDC bearers, browser sessions)
// relay explicitly — core judges, core bytes come back untouched,
// dest invents no 401, 403, 404, or 500 for them;
// credential-less callers get the standing dest 401. The only
// dest-synthesized errors on the terminated path are the gate 401,
// the family 403, and the permanent local 503 when no core origin is
// configured (never Retry-After).
//
// Cookie rule (D2 follow-up lesson, shared with the D3/D4/D6 and
// gap-04 gates): only session-cookie-shaped credentials relay. A
// Cookie header carrying the eshu_session (or __Host-eshu_session)
// member names a browser-session caller and relays explicitly; any
// other Cookie (junk) is not a credential and stays dest-401 with
// zero core contact. The gate reuses hasSessionCookie from the D3
// lane — one shape check, not four.
//
// Rollback is revert-to-relay: remove the s.registerCodeFlowWave(mux)
// call in routes() (or this lane's files) and the four Code-flow
// POSTs return to the generic relay behind RequireAuth. The spec
// absences stay disclosed either way. No data migrates and no other
// family moves.

// authorizeCodeFlowSharedKey reports whether an identity carrying
// allScopes may terminate the Code-flow POSTs natively. The only
// identity dest can construct today is the shared-key holder, which
// core maps to AllScopes, so the wave gate passes true and core
// re-judges every forwarded call as backstop (tenant scope,
// permission catalog, governance mode — which is also where the
// live 403s and mode-dependent denials come from). The false branch
// is the hookup point for the day dest gains a scoped/session
// resolver; until then it stays a plain deny (no invented reason
// codes, no audit widening).
func authorizeCodeFlowSharedKey(allScopes bool) bool {
	return allScopes
}

// codeFlowFamilyDenyMessage is the documented family-403 text the
// dest authorizer adds on the unreachable-until-D8 deny branch.
const codeFlowFamilyDenyMessage = "all-scopes operator authorization is required"

// serveCodeFlowTerminated terminates one Code-flow POST for a
// shared-key caller: the family authorizer runs first, then the call
// crosses the versioned port transport (forwardToCore on the native
// lane) with the caller's method, path, query, body, negotiation
// headers, and credential intact and core bytes come back
// unmodified.
func (s *Server) serveCodeFlowTerminated(w http.ResponseWriter, r *http.Request) {
	s.serveCodeFlowTerminatedAs(w, r, true)
}

// serveCodeFlowTerminatedAs is the testable core of the terminated
// reader: allScopes is the resolved caller scope. The wave gate
// always passes true today (the shared key maps to AllScopes in
// core); the false branch is the hookup point for the day dest
// gains a scoped/session resolver, and it stays a plain deny until
// then. Like dispatch, it fails closed with the permanent 503 when
// no core origin is configured.
func (s *Server) serveCodeFlowTerminatedAs(w http.ResponseWriter, r *http.Request, allScopes bool) {
	if s.core == nil {
		s.dispatchOutcome(r, viaNative, outcomeLocal503, 0, false)
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	family := classifyFamily(r.URL.Path)
	if !authorizeCodeFlowSharedKey(allScopes) {
		if tel := s.telemetry; tel != nil {
			tel.observeAuthorizer(family, decisionDeny)
		}
		s.dispatchOutcome(r, viaNative, outcomeDenied, 0, false)
		writeError(w, r, http.StatusForbidden, http.StatusText(http.StatusForbidden),
			codeFlowFamilyDenyMessage)
		return
	}
	if tel := s.telemetry; tel != nil {
		tel.observeAuthorizer(family, decisionAllow)
	}
	forwardToCore(s, w, r, viaNative)
}

// codeFlowReadGate fronts the Code-flow wave routes. A valid shared
// key terminates (next); a request carrying any other credential — a
// non-key Bearer (a foreign credential class: scoped token, OIDC) or
// a session-cookie-shaped Cookie — relays explicitly to the same
// core route on the relay lane, so core judges the class dest cannot
// resolve and dest invents no failure for it; a credential-less
// request (or a junk-cookie carrier, per the D2 follow-up lesson)
// gets the standing dest 401 with zero core contact. An empty
// configured key fails closed like RequireAuth. The routes stay
// authed in this gate: no public carve-out exists in this lane.
func (s *Server) codeFlowReadGate(next http.Handler) http.Handler {
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
			s.codeFlowRelayExplicit(w, r)
			return
		}
		if hasSessionCookie(r) {
			s.codeFlowRelayExplicit(w, r)
			return
		}
		WriteUnauthorized(w, r)
	})
}

// codeFlowRelayExplicit forwards one wave call carrying an
// unresolvable credential class to the same core route and relays
// core bytes untouched on the relay lane. The instrumented
// middleware files the lane from the record, so the gate flips the
// record lane before forwarding; direct (uninstrumented) calls
// already carry the relay lane through forwardToCore.
func (s *Server) codeFlowRelayExplicit(w http.ResponseWriter, r *http.Request) {
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
