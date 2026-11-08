// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"crypto/subtle"
	"net/http"
)

// D4 Native-next reads termination engine (Refs eshu-hq/eshu#9669,
// part of #6707). The wave table lives in d4_wave.go; this file owns
// the serving contract: the wave gate and the terminated reader.
//
// Lane rule (arbiter D-wave ruling, decision 4, via the D2/D3 waves):
// shared-key callers terminate natively; credential classes dest
// cannot resolve (scoped tokens, OIDC bearers, browser sessions) relay
// explicitly — core judges, core bytes come back untouched, dest
// invents no 401, 403, 404, or 500 for them; credential-less callers
// get the standing dest 401. The only dest-synthesized errors on the
// terminated path are the gate 401, the future-resolver family 403,
// and the permanent local 503 when no core origin is configured
// (never Retry-After).
//
// Cookie rule (D2 follow-up lesson, shared with the D3 gate):
// only session-cookie-shaped credentials relay. A Cookie header
// carrying the eshu_session (or __Host-eshu_session) member names a
// browser-session caller and relays explicitly; any other Cookie
// (junk) is not a credential and stays dest-401 with zero core contact,
// exactly like a credential-less caller. The gate reuses
// hasSessionCookie from the D3 lane — one shape check, not two.

// authorizeD4SharedKey reports whether an identity carrying
// allScopes may terminate a D4 read natively. The only identity dest
// can construct today is the shared-key holder, which core maps to
// AllScopes, so the wave gate passes true and core re-judges every
// forwarded call as backstop (tenant scope, permission catalog,
// governance mode — which is also where the live 403s and
// mode-dependent denials come from). The false branch is the hookup
// point for the day dest gains a scoped/session resolver; until then
// it stays a plain deny (no invented reason codes, no audit
// widening). One shared authorizer covers all 45 D4 wave routes: they
// share the shared-key trust root until D8 resolves per-class
// handling, and the contract record (contracts/d4-native-next.md)
// carries the scope-honesty label for each family.
func authorizeD4SharedKey(allScopes bool) bool {
	return allScopes
}

// d4FamilyDenyMessage is the documented family-403 text the dest
// authorizer adds on the unreachable-until-D8 deny branch.
const d4FamilyDenyMessage = "all-scopes operator authorization is required"

// serveD4Terminated terminates one D4 read for a shared-key caller:
// the family authorizer runs first, then the call crosses the versioned
// port transport (forwardToCore on the native lane) with the caller's
// negotiation headers and credential intact and core bytes come back
// unmodified.
func (s *Server) serveD4Terminated(w http.ResponseWriter, r *http.Request) {
	s.serveD4TerminatedAs(w, r, true)
}

// serveD4TerminatedAs is the testable core of the terminated reader:
// allScopes is the resolved caller scope. The wave gate always passes
// true today (the shared key maps to AllScopes in core); the false
// branch is the hookup point for the day dest gains a
// scoped/session resolver, and it stays a plain deny until then. Like
// dispatch, it fails closed with the permanent 503 when no core origin
// is configured, so a native route can never dereference an
// unconfigured origin.
func (s *Server) serveD4TerminatedAs(w http.ResponseWriter, r *http.Request, allScopes bool) {
	if s.core == nil {
		s.dispatchOutcome(r, viaNative, outcomeLocal503, 0, false)
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	// The family authorizer judgment counts on both branches, so the
	// unreachable deny branch stays observable from dest signals. A deny
	// never dispatches: it files the denied outcome on the native lane.
	family := classifyFamily(r.URL.Path)
	if !authorizeD4SharedKey(allScopes) {
		if tel := s.telemetry; tel != nil {
			tel.observeAuthorizer(family, decisionDeny)
		}
		s.dispatchOutcome(r, viaNative, outcomeDenied, 0, false)
		writeError(w, r, http.StatusForbidden, http.StatusText(http.StatusForbidden),
			d4FamilyDenyMessage)
		return
	}
	if tel := s.telemetry; tel != nil {
		tel.observeAuthorizer(family, decisionAllow)
	}
	forwardToCore(s, w, r, viaNative)
}

// d4ReadGate fronts the 45 D4 wave routes. A valid shared key
// terminates (next); a request carrying any other credential — a
// non-key Bearer (scoped token, OIDC) or a session-cookie-shaped
// Cookie — relays explicitly to the core on the relay lane, so core
// judges the class dest cannot resolve and dest invents no failure
// for it; a credential-less request (or a junk-cookie carrier, per the
// D2 follow-up lesson) gets the standing dest 401 with zero core
// contact. An empty configured key fails closed like RequireAuth.
func (s *Server) d4ReadGate(next http.Handler) http.Handler {
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
			// A non-key Bearer is a foreign credential class (scoped
			// token, OIDC): relay explicitly so core judges it.
			s.d4RelayExplicit(w, r)
			return
		}
		if hasSessionCookie(r) {
			// A session-cookie-shaped Cookie without a Bearer is a
			// browser-session caller: relay explicitly so core judges it.
			// Junk cookies fall through to the standing 401 below.
			s.d4RelayExplicit(w, r)
			return
		}
		WriteUnauthorized(w, r)
	})
}

// d4RelayExplicit forwards one wave call carrying an unresolvable
// credential class to the same core route and relays core bytes
// untouched on the relay lane. The instrumented middleware files the
// lane from the record, so the gate flips the record lane before
// forwarding; direct (uninstrumented) calls already carry the relay
// lane through forwardToCore.
func (s *Server) d4RelayExplicit(w http.ResponseWriter, r *http.Request) {
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
