// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// retirementRoute is the single native-dispatch route of the
// AdminRetirement family (tier C-a): the operator repository retirement
// trigger. It is registered explicitly inside RequireAuth, ahead of the
// generic /api/ proxy, with the AllScopes family authorizer; every other
// family still crosses the byte-relay.
const retirementRoute = "POST /api/v0/admin/repository-retirements"

// retirementWriteThrough is the tier C-a native dispatch for the
// operator retirement trigger: the request crosses an explicit
// per-family route with the AllScopes family authorizer instead of the
// generic byte-relay. The shape is HTTP write-through — method, path,
// query, headers (including the original Authorization header), and
// body forward to the same core route, and core bytes relay back
// unmodified.
//
// Trust model, binding: dest's only credential is the shared key, which
// core maps to AllScopes (so the dest allow branch is the live one and
// core re-judges every forwarded call as backstop — which is also where
// the live 403s come from). Dest has no scoped/session resolver, so the
// deny branch is unreachable in production until one lands; it is
// recorded as such, not faked, and the tests pin its shape for that day.
// Dest adds no handler validation: request/response semantics stay
// core-owned. Governance audit stays core-side on forwarded requests —
// the in-tree retirement handler records denied attempts and the core
// trigger records admin_retirement_action events, both from inside core
// code that still runs on every forwarded request, so write-through keeps
// every audit event core-side with no gap. Dest emits none.
func (s *Server) retirementWriteThrough(w http.ResponseWriter, r *http.Request) {
	s.retirementWriteThroughAs(w, r, true)
}

// retirementWriteThroughAs is the testable core of the tier C-a
// handler: allScopes is the resolved caller scope. The route always
// passes true today (the shared key maps to AllScopes in core); the
// false branch is the hookup point for the day dest gains a
// scoped/session resolver, and it stays a plain deny until then. Like
// dispatch, it fails closed with the permanent 503 when no core origin
// is configured, so the native route can never dereference an
// unconfigured origin.
func (s *Server) retirementWriteThroughAs(w http.ResponseWriter, r *http.Request, allScopes bool) {
	if s.core == nil {
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	if !authorizeRetirementAdmin(allScopes) {
		writeError(w, r, http.StatusForbidden, http.StatusText(http.StatusForbidden),
			"all-scopes operator authorization is required")
		return
	}
	forwardToCore(s, w, r, viaNative)
}

// authorizeRetirementAdmin reports whether an identity carrying
// allScopes may call the operator retirement trigger. The only identity
// dest can construct today is the shared-key holder, which core maps to
// AllScopes, so callers pass true and core re-judges as backstop. The
// false branch exists for the day dest gains a scoped/session resolver;
// until then it is unreachable in production and must stay a plain deny
// (no invented reason codes, no audit widening).
func authorizeRetirementAdmin(allScopes bool) bool {
	return allScopes
}
