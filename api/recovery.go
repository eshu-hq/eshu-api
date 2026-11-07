// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// Admin recovery mutation native-dispatch routes (tier C-c2): the four
// admin recovery mutation routes from the in-tree AdminHandler.Mount at
// the provenance pin (7c605839) and go/cmd/api/CUTOVER_OWNERSHIP.md —
// replay, reopen, refinalize, and recover-generations under
// /api/v0/admin. They are registered explicitly inside RequireAuth,
// ahead of the generic /api/ proxy, with the AllScopes family
// authorizer; the admin-four (backfill, dead-letter, reindex, skip)
// ride the D7 tier-C clone lane (api/d7.go) and the family's reads stay
// on the byte-relay. Route registration has no path-key collision: the
// four native paths share no prefix with the admin-four beyond
// /api/v0/admin, and Go
// matches method and path together, so the POSTs ride the native route
// while every other method and every other path stays relayed. The
// OpenAPI fragment is a line-verbatim subset of the pinned core file
// (see contracts/recovery-admission.md): it holds only the four route
// entries whose path keys are unique to this lane.
const (
	recoveryReplayRoute             = "POST /api/v0/admin/replay"
	recoveryReopenRoute             = "POST /api/v0/admin/reopen"
	recoveryRefinalizeRoute         = "POST /api/v0/admin/refinalize"
	recoveryRecoverGenerationsRoute = "POST /api/v0/admin/recover-generations"
)

// recoveryRoutes lists every native-dispatch route of the admin
// recovery mutation family in registration order.
var recoveryRoutes = []string{
	recoveryReplayRoute,
	recoveryReopenRoute,
	recoveryRefinalizeRoute,
	recoveryRecoverGenerationsRoute,
}

// recoveryWriteThrough is the tier C-c2 native dispatch for the admin
// recovery mutations: each request crosses an explicit per-family route
// with the AllScopes family authorizer instead of the generic
// byte-relay. The shape is HTTP write-through — method, path, query,
// headers (including the original Authorization header), and body
// forward to the same core route, and core bytes relay back
// unmodified. Dest holds no recovery or ledger state and performs no
// mutation of its own: the claim/complete pair the replay, reopen, and
// recover-generations routes ride on runs inside the core handler on
// every forwarded request, and refinalize runs its single-transaction
// enqueue there too.
//
// Trust model, binding: dest's only credential is the shared key, which
// core maps to AllScopes (so the dest allow branch is the live one and
// core re-judges every forwarded call as backstop — all-scope admin,
// admin tenant scope, and the recovery permission catalog — which is
// also where the live 403s come from). Dest has no scoped/session
// resolver, so the deny branch is unreachable in production until one
// lands; it is recorded as such, not faked, and the tests pin its shape
// for that day. Dest adds no handler validation: request/response
// semantics stay core-owned. Governance audit stays core-side on
// forwarded requests — the in-tree replay, reopen, refinalize, and
// recover-generations handlers at the provenance pin emit
// replay_accepted (and every denied cause: missing reason/key,
// unauthorized, unsafe-class, superseded, key-reused, in-progress, and
// the reopen/resolve refusals) from inside the core handlers that still
// run on every forwarded request, so write-through keeps every audit
// event core-side with no gap. Dest emits none. A family whose handlers
// emit audit must never be re-implemented dest-side without a versioned
// audit-append contract; this lane re-implements nothing.
//
// Cancel and timeout parity for long recoveries: dest never retries a
// forwarded mutation, so a caller cancel or dispatch-timeout answer is
// the permanent local 503 backend_unavailable with exactly one upstream
// attempt — and core still records the committed outcome on its
// cancel-proof finalize context, so the idempotency-key retry replays
// the stored outcome (or answers the in-progress 409) instead of
// losing it. Refinalize runs in one transaction, so a cancel rolls it
// back unless it lands while the commit itself runs.
func (s *Server) recoveryWriteThrough(w http.ResponseWriter, r *http.Request) {
	s.recoveryWriteThroughAs(w, r, true)
}

// recoveryWriteThroughAs is the testable core of the tier C-c2
// handler: allScopes is the resolved caller scope. The routes always
// pass true today (the shared key maps to AllScopes in core); the
// false branch is the hookup point for the day dest gains a
// scoped/session resolver, and it stays a plain deny until then. Like
// dispatch, it fails closed with the permanent 503 when no core origin
// is configured, so the native routes can never dereference an
// unconfigured origin.
func (s *Server) recoveryWriteThroughAs(w http.ResponseWriter, r *http.Request, allScopes bool) {
	if s.core == nil {
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	if !authorizeRecoveryAdmin(allScopes) {
		writeError(w, r, http.StatusForbidden, http.StatusText(http.StatusForbidden),
			"all-scopes operator authorization is required")
		return
	}
	forwardToCore(s, w, r, viaNative)
}

// authorizeRecoveryAdmin reports whether an identity carrying
// allScopes may call the admin recovery mutations. The only identity
// dest can construct today is the shared-key holder, which core maps to
// AllScopes, so callers pass true and core re-judges (tenant scope,
// permission catalog) as backstop. The false branch exists for the day
// dest gains a scoped/session resolver; until then it is unreachable in
// production and must stay a plain deny (no invented reason codes, no
// audit widening).
func authorizeRecoveryAdmin(allScopes bool) bool {
	return allScopes
}
