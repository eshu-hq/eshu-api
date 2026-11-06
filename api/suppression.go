// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// suppressionRoute is the single native-dispatch pilot route: the
// operator suppression upsert. It is registered explicitly inside
// RequireAuth, ahead of the generic /api/ proxy, with the AllScopes
// family authorizer; every other family still crosses the byte-relay.
const suppressionRoute = "POST /api/v0/supply-chain/impact/suppressions"

// suppressionWriteThrough is the native-dispatch pilot: the operator
// suppression upsert crosses an explicit per-family route with the
// AllScopes family authorizer instead of the generic byte-relay. The
// shape is HTTP write-through — method, path, query, headers (including
// the original Authorization header), and body forward to the same core
// route, and core bytes relay back unmodified.
//
// Trust model, binding: dest's only credential is the shared key, which
// core maps to AllScopes (so the dest allow branch is the live one and
// core re-judges every forwarded call as backstop, which is also where
// the live 403s come from today). Dest has no scoped/session resolver,
// so the deny branch is unreachable in production until one lands; it
// is recorded as such, not faked, and the tests pin its shape for that
// day rather than a live path. Dest adds no handler validation:
// request/response semantics stay core-owned. Governance audit stays
// core-side on forwarded
// requests — Suppression emits no handler audit in core and dest emits
// none, so there is no audit gap on this route (unlike families whose
// handlers do emit, which stay on the generic relay until a versioned
// audit-append contract exists).
func (s *Server) suppressionWriteThrough(w http.ResponseWriter, r *http.Request) {
	s.suppressionWriteThroughAs(w, r, true)
}

// suppressionWriteThroughAs is the testable core of the pilot handler:
// allScopes is the resolved caller scope. The route always passes true
// today (the shared key maps to AllScopes in core); the false branch is
// the hookup point for the day dest gains a scoped/session resolver,
// and it stays a plain deny until then. Like dispatch, it fails closed
// with the permanent 503 when no core origin is configured, so the
// native route can never dereference an unconfigured origin.
func (s *Server) suppressionWriteThroughAs(w http.ResponseWriter, r *http.Request, allScopes bool) {
	if s.core == nil {
		s.dispatchOutcome(r, viaNative, outcomeLocal503, 0, false)
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	// The family authorizer judgment counts on both branches, so the
	// unreachable deny branch stays observable from dest signals. A deny
	// never dispatches: it files the denied outcome on the native lane.
	if !authorizeSuppressionOperator(allScopes) {
		if tel := s.telemetry; tel != nil {
			tel.observeAuthorizer("suppression", decisionDeny)
		}
		s.dispatchOutcome(r, viaNative, outcomeDenied, 0, false)
		writeError(w, r, http.StatusForbidden, http.StatusText(http.StatusForbidden),
			"all-scopes operator authorization is required")
		return
	}
	if tel := s.telemetry; tel != nil {
		tel.observeAuthorizer("suppression", decisionAllow)
	}
	forwardToCore(s, w, r, viaNative)
}

// authorizeSuppressionOperator reports whether an identity carrying
// allScopes may call the suppression upsert. The only identity dest can
// construct today is the shared-key holder, which core maps to
// AllScopes, so callers pass true and core re-judges as backstop. The
// false branch exists for the day dest gains a scoped/session resolver;
// until then it is unreachable in production and must stay a plain deny
// (no invented reason codes, no audit widening).
func authorizeSuppressionOperator(allScopes bool) bool {
	return allScopes
}
