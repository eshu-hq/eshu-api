// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// retireUnclaimedRoute is the single native-dispatch mutation route of
// the admin retire-unclaimed family: the operator never-claimed
// workflow retirement (Refs eshu-hq/eshu#9669, eshu-hq/eshu-api#103).
// It is registered explicitly inside RequireAuth, ahead of the generic
// /api/ proxy, with the AllScopes family authorizer; every other family
// still crosses the byte-relay.
const retireUnclaimedRoute = "POST /api/v0/admin/workflow-retire-unclaimed"

// retireUnclaimedWriteThrough is the writer native dispatch for the
// never-claimed workflow retirement: the request crosses an explicit
// per-family route with the AllScopes family authorizer instead of the
// generic byte-relay. The shape is HTTP write-through — method, path,
// query, headers (including the original Authorization header), and
// body forward to the same core route, and core bytes relay back
// unmodified with exactly one forward attempt and no retry.
// Dest holds no workflow control state and performs no mutation of its
// own: the gate probe, the retire, and the audit append all run inside
// the core handler that still runs on every forwarded request.
//
// Trust model, binding: dest's only credential is the shared key, which
// core maps to AllScopes (so the dest allow branch is the live one and
// core re-judges every forwarded call as backstop — all-scope admin —
// which is also where the live 403s come from today). Dest has no
// scoped/session resolver, so the deny branch is unreachable in
// production until one lands; it is recorded as such, not faked, and
// the tests pin its shape for that day. Dest adds no handler
// validation: request/response semantics stay core-owned. Core is the
// sole writer.
//
// Governance audit stays core-side on forwarded requests — the in-tree
// handler appends retire_unclaimed_accepted on success and the two
// retire_unclaimed_refused_* denies from inside the core handler that
// still runs on every forwarded request, so write-through keeps every
// audit event core-side with no gap. The core 400, 503, and 500 paths
// emit no governance audit, and dest emits none on any path. A family
// whose handler emits audit must never be re-implemented dest-side
// without a versioned audit-append contract; this lane re-implements
// nothing.
//
// Idempotency posture: the endpoint claims no idempotency key and holds
// no ledger row because the mutation is already state-idempotent, not
// exactly-once — a re-run touches only rows still pending with no
// claim. Dest synthesizes no key, no ledger, and no outcome: a
// sequential duplicate forwards twice and relays both core answers.
func (s *Server) retireUnclaimedWriteThrough(w http.ResponseWriter, r *http.Request) {
	s.retireUnclaimedWriteThroughAs(w, r, true)
}

// retireUnclaimedWriteThroughAs is the testable core of the writer
// handler: allScopes is the resolved caller scope. The route always
// passes true today (the shared key maps to AllScopes in core); the
// false branch is the hookup point for the day dest gains a
// scoped/session resolver, and it stays a plain deny until then. Like
// dispatch, it fails closed with the permanent 503 when no core origin
// is configured, so the native route can never dereference an
// unconfigured origin.
func (s *Server) retireUnclaimedWriteThroughAs(w http.ResponseWriter, r *http.Request, allScopes bool) {
	if s.core == nil {
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	if !authorizeRetireUnclaimedAdmin(allScopes) {
		writeError(w, r, http.StatusForbidden, http.StatusText(http.StatusForbidden),
			"all-scopes operator authorization is required")
		return
	}
	forwardToCore(s, w, r, viaNative)
}

// authorizeRetireUnclaimedAdmin reports whether an identity carrying
// allScopes may call the never-claimed workflow retirement. The only
// identity dest can construct today is the shared-key holder, which
// core maps to AllScopes, so callers pass true and core re-judges as
// backstop. The false branch exists for the day dest gains a
// scoped/session resolver; until then it is unreachable in production
// and must stay a plain deny (no invented reason codes, no audit
// widening).
func authorizeRetireUnclaimedAdmin(allScopes bool) bool {
	return allScopes
}
