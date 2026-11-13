// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"crypto/subtle"
	"net/http"
)

// Code-graph relay-leg termination engine (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#121). The wave table lives in
// code_graph_wave.go; this file owns the serving contract: the wave
// gate and the terminated reader.
//
// Lane rule (arbiter Wave-7 ruling, via the Wave-2, W6a, W4, and W8a
// lanes): shared-key callers terminate natively; credential classes
// dest cannot resolve (scoped tokens, OIDC bearers, browser sessions)
// relay explicitly — core judges, core bytes come back untouched, dest
// invents no 401, 403, 404, 409, 422, or 500 for them;
// credential-less callers get the standing dest 401. The only
// dest-synthesized errors on the terminated path are the gate 401,
// the future-resolver family 403, and the permanent local 503 when no
// core origin is configured (never Retry-After).
//
// Cookie rule (D2 follow-up lesson, shared with the W4 and W8a
// gates): only session-cookie-shaped credentials relay. A Cookie
// header carrying the eshu_session (or __Host-eshu_session) member
// names a browser-session caller and relays explicitly; any other
// Cookie (junk) is not a credential and stays dest-401 with zero core
// contact. The gate reuses hasSessionCookie from the D3 lane — one
// shape check, not four.
//
// Rollback is revert-to-relay: remove the
// s.registerCodeGraphWave(mux) call in routes() (or this lane's
// files) and the four graph POSTs return to the generic relay behind
// RequireAuth. The spec absences stay disclosed either way. No data
// migrates and no other family moves.

// authorizeCodeGraphSharedKey reports whether an identity carrying
// allScopes may terminate a G-a graph read natively. The only
// identity dest can construct today is the shared-key holder, which
// core maps to AllScopes, so the wave gate passes true and core
// re-judges every forwarded call as backstop (tenant scope,
// permission catalog, governance mode — which is also where the live
// 403s and mode-dependent denials come from). The false branch is the
// hookup point for the day dest gains a scoped/session resolver;
// until then it stays a plain deny (no invented reason codes, no audit
// widening). One shared authorizer covers all four wave routes: they
// share the shared-key trust root until D8 resolves per-class
// handling, and the contract record
// (contracts/code-graph-termination.md) carries the scope-honesty
// label.
func authorizeCodeGraphSharedKey(allScopes bool) bool {
	return allScopes
}

// codeGraphFamilyDenyMessage is the documented family-403 text the
// dest authorizer adds on the unreachable-until-D8 deny branch.
const codeGraphFamilyDenyMessage = "all-scopes operator authorization is required"

// serveCodeGraphTerminatedAs is the testable core of the terminated
// reader: allScopes is the resolved caller scope. The wave gate always
// passes true today (the shared key maps to AllScopes in core); the
// false branch is the hookup point for the day dest gains a
// scoped/session resolver, and it stays a plain deny until then. Like
// dispatch, it fails closed with the permanent 503 when no core origin
// is configured, so a native route can never dereference an
// unconfigured origin.
func (s *Server) serveCodeGraphTerminatedAs(w http.ResponseWriter, r *http.Request, allScopes bool) {
	if s.core == nil {
		s.dispatchOutcome(r, viaNative, outcomeLocal503, 0, false)
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	// The family authorizer judgment counts on both branches, so the
	// unreachable deny branch stays observable from dest signals. A
	// deny never dispatches: it files the denied outcome on the native
	// lane.
	family := classifyFamily(r.URL.Path)
	if !authorizeCodeGraphSharedKey(allScopes) {
		if tel := s.telemetry; tel != nil {
			tel.observeAuthorizer(family, decisionDeny)
		}
		s.dispatchOutcome(r, viaNative, outcomeDenied, 0, false)
		writeError(w, r, http.StatusForbidden, http.StatusText(http.StatusForbidden),
			codeGraphFamilyDenyMessage)
		return
	}
	if tel := s.telemetry; tel != nil {
		tel.observeAuthorizer(family, decisionAllow)
	}
	forwardToCore(s, w, r, viaNative)
}

// serveCodeGraphTerminated terminates one G-a graph read for a
// shared-key caller: the family authorizer runs first, then the call
// crosses the versioned port transport (forwardToCore on the native
// lane) with the caller's method, path, query, body, negotiation
// headers, and credential intact and core bytes come back unmodified.
func (s *Server) serveCodeGraphTerminated(w http.ResponseWriter, r *http.Request) {
	s.serveCodeGraphTerminatedAs(w, r, true)
}

// codeGraphReadGate fronts the 4 G-a routes. A valid shared key
// terminates (next); a request carrying any other credential — a
// non-key bearer naming a foreign credential class (scoped token,
// OIDC) or a session-cookie-shaped Cookie — relays explicitly to the
// same core route on the relay lane, so core judges the class dest
// cannot resolve and dest invents no failure for it; a credential-less
// request (or a junk-cookie carrier, per the D2 follow-up lesson) gets
// the standing dest 401 with zero core contact. An empty configured
// key fails closed like RequireAuth. The routes stay authed in this
// gate: no public carve-out exists in this lane.
func (s *Server) codeGraphReadGate(next http.Handler) http.Handler {
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
			// A non-key bearer names a foreign credential class (scoped
			// token, OIDC): relay explicitly so core judges it.
			s.codeGraphRelayExplicit(w, r)
			return
		}
		if hasSessionCookie(r) {
			// A session-cookie-shaped Cookie without a bearer names a
			// browser-session caller: relay explicitly so core judges it.
			// Junk cookies fall through to the standing 401 below.
			s.codeGraphRelayExplicit(w, r)
			return
		}
		WriteUnauthorized(w, r)
	})
}

// codeGraphRelayExplicit forwards one wave call carrying an
// unresolvable credential class to the same core route and relays core
// bytes untouched on the relay lane. The instrumented middleware files
// the lane from the record, so the gate flips the record lane before
// forwarding; direct (uninstrumented) calls already carry the relay
// lane through forwardToCore.
func (s *Server) codeGraphRelayExplicit(w http.ResponseWriter, r *http.Request) {
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
