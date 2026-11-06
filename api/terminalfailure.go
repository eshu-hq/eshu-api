// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// terminalFailureRoute is the single native-dispatch mutation route of
// the AdminTerminalFailure family (tier C-a): the operator
// collector-failure acknowledgement upsert. It is registered explicitly
// inside RequireAuth, ahead of the generic /api/ proxy, with the
// AllScopes family authorizer. The family's reads
// (acknowledgements/query, collector-failures/query) and every other
// family still cross the byte-relay.
const terminalFailureRoute = "POST /api/v0/admin/collector-failures/acknowledge"

// terminalFailureWriteThrough is the tier C-a native dispatch for the
// operator terminal-failure acknowledgement: the request crosses an
// explicit per-family route with the AllScopes family authorizer instead
// of the generic byte-relay. The shape is HTTP write-through — method,
// path, query, headers (including the original Authorization header),
// and body forward to the same core route, and core bytes relay back
// unmodified.
//
// Trust model, binding: dest's only credential is the shared key, which
// core maps to AllScopes (so the dest allow branch is the live one and
// core re-judges every forwarded call as backstop — all-scope admin with
// an admin tenant scope, plus the core-side scoped-token deny — which is
// also where the live 403s come from today). Dest has no scoped/session
// resolver, so the deny branch is unreachable in production until one
// lands; it is recorded as such, not faked, and the tests pin its shape
// for that day. Dest adds no handler validation: request/response
// semantics stay core-owned. Core is the sole writer: the acknowledgement
// and the shared replay-ledger claim/complete pair run inside the core
// handler that still runs on every forwarded request.
//
// Governance audit stays core-side on forwarded requests — like
// sign-in-policy and unlike suppression, the in-tree acknowledgement
// handler at the provenance pin DOES emit audit
// (EventTypeAdminRecoveryAction for every decision: the allowed outcome,
// every guardrail refusal, and every denied outcome including the
// idempotency-conflict answers), and it emits from inside the core
// handler that still runs on every forwarded request, so write-through
// keeps every audit event core-side with no gap. Dest emits none. A
// family whose handler emits audit must never be re-implemented
// dest-side without a versioned audit-append contract; this lane
// re-implements nothing.
func (s *Server) terminalFailureWriteThrough(w http.ResponseWriter, r *http.Request) {
	s.terminalFailureWriteThroughAs(w, r, true)
}

// terminalFailureWriteThroughAs is the testable core of the tier C-a
// handler: allScopes is the resolved caller scope. The route always
// passes true today (the shared key maps to AllScopes in core); the
// false branch is the hookup point for the day dest gains a
// scoped/session resolver, and it stays a plain deny until then. Like
// dispatch, it fails closed with the permanent 503 when no core origin
// is configured, so the native route can never dereference an
// unconfigured origin.
func (s *Server) terminalFailureWriteThroughAs(w http.ResponseWriter, r *http.Request, allScopes bool) {
	if s.core == nil {
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	if !authorizeTerminalFailureAdmin(allScopes) {
		writeError(w, r, http.StatusForbidden, http.StatusText(http.StatusForbidden),
			"all-scopes operator authorization is required")
		return
	}
	forwardToCore(s, w, r, viaNative)
}

// authorizeTerminalFailureAdmin reports whether an identity carrying
// allScopes may call the terminal-failure acknowledgement. The only
// identity dest can construct today is the shared-key holder, which core
// maps to AllScopes, so callers pass true and core re-judges (admin
// tenant scope, scoped-token deny) as backstop. The false branch exists
// for the day dest gains a scoped/session resolver; until then it is
// unreachable in production and must stay a plain deny (no invented
// reason codes, no audit widening).
func authorizeTerminalFailureAdmin(allScopes bool) bool {
	return allScopes
}
