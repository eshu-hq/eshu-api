// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"crypto/subtle"
	"net/http"
)

// signInPolicyRoute is the single native-dispatch route of the
// SignInPolicyMutations family (tier C-a): the admin sign-in policy
// partial update. It is registered explicitly inside RequireAuth, ahead
// of the generic /api/ proxy, with the AllScopes family authorizer; the
// family's reads and every other family still cross the byte-relay.
const signInPolicyRoute = "PATCH /api/v0/auth/admin/sign-in-policy"

// signInPolicyWriteThrough is the W7-writers terminated writer for
// the admin sign-in policy PATCH: the request crosses an explicit
// per-family route behind the family credential-class gate (see
// signInPolicyWriterGate) with the AllScopes family authorizer
// instead of the generic byte-relay. The shape is HTTP write-through
// — method, path, query, headers (including the original
// Authorization header), and body forward to the same core route,
// and core bytes come back unmodified with exactly one forward
// attempt and no retry.
//
// Trust model, binding: dest's only credential is the shared key, which
// core maps to AllScopes (so the dest allow branch is the live one and
// core re-judges every forwarded call as backstop — all-scope admin,
// admin tenant scope, and the identity_admin.sign_in_policy_update
// permission catalog — which is also where the live 403s come from).
// Dest has no scoped/session resolver, so the deny branch is unreachable
// in production until one lands; it is recorded as such, not faked, and
// the tests pin its shape for that day. Dest adds no handler validation:
// request/response semantics stay core-owned. Governance audit stays
// core-side on forwarded requests — unlike suppression, the in-tree
// sign-in handler DOES emit audit (EventTypeIDPConfigChange for the
// allowed outcome and for every denied outcome, including guardrail
// rejections), and it emits from inside the core handler that still runs
// on every forwarded request, so write-through keeps every audit event
// core-side with no gap. Dest emits none.
func (s *Server) signInPolicyWriteThrough(w http.ResponseWriter, r *http.Request) {
	s.signInPolicyWriteThroughAs(w, r, true)
}

// signInPolicyWriteThroughAs is the testable core of the tier C-a
// handler: allScopes is the resolved caller scope. The route always
// passes true today (the shared key maps to AllScopes in core); the
// false branch is the hookup point for the day dest gains a
// scoped/session resolver, and it stays a plain deny until then. Like
// dispatch, it fails closed with the permanent 503 when no core origin
// is configured, so the native route can never dereference an
// unconfigured origin.
func (s *Server) signInPolicyWriteThroughAs(w http.ResponseWriter, r *http.Request, allScopes bool) {
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

// authorizeSignInPolicyAdmin reports whether an identity carrying
// allScopes may call the admin sign-in policy family legs: the GET
// read (terminated, see api/signinpolicy_admin_terminate.go) and the
// PATCH update (terminated writer below). The only identity
// dest can construct today is the shared-key holder, which core maps to
// AllScopes, so callers pass true and core re-judges (tenant scope,
// permission catalog) as backstop. The false branch exists for the day
// dest gains a scoped/session resolver; until then it is unreachable in
// production and must stay a plain deny (no invented reason codes, no
// audit widening).
func authorizeSignInPolicyAdmin(allScopes bool) bool {
	return allScopes
}

// signInPolicyWriterGate fronts the admin sign-in-policy PATCH. A
// valid shared key terminates natively (next); a request carrying any
// other credential — a non-key Bearer (scoped token, OIDC) or a
// session-cookie-shaped Cookie — relays explicitly to the same core
// route on the relay lane, so core judges the class dest cannot
// resolve and dest invents no failure for it; a credential-less
// request (or a junk-cookie carrier) gets the standing dest 401 with
// zero core contact. An empty configured key fails closed like
// RequireAuth. The shape mirrors the W7-read family gate (see
// api/signinpolicy_admin_terminate.go): one trust root, one
// credential matrix for the family's authed legs.
func (s *Server) signInPolicyWriterGate(next http.Handler) http.Handler {
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
			s.signInPolicyRelayExplicit(w, r)
			return
		}
		if hasLeafBSessionCookie(r) {
			// A session-cookie-shaped Cookie without a Bearer [REDACTED] a
			// browser-session caller: relay explicitly so core judges
			// it. Junk cookies fall through to the standing 401 below.
			s.signInPolicyRelayExplicit(w, r)
			return
		}
		WriteUnauthorized(w, r)
	})
}

// signInPolicyRelayExplicit forwards one family call carrying an
// unresolvable credential class to the same core route and relays
// core bytes untouched on the relay lane. The instrumented middleware
// files the lane from the record, so the gate flips the record lane
// before forwarding; direct (uninstrumented) calls already carry the
// relay lane through forwardToCore.
func (s *Server) signInPolicyRelayExplicit(w http.ResponseWriter, r *http.Request) {
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
