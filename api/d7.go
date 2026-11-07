// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// Admin-four tier-C dispatch clones (Refs eshu-hq/eshu#9669, part of
// #6707; pins minted in eshu-hq/eshu#10312 as Backfill, DeadLetter,
// Reindex, and Skip Admission v0; arbiter APPROVED-WITH-CONDITIONS on
// that PR). The four operator mutations have their explicit per-family
// routes inside RequireAuth, ahead of the generic /api/ proxy, with
// the AllScopes family authorizer; every other family still crosses
// the byte-relay below. Route registration has no path-key collision:
// the four native paths share no prefix with each other beyond
// /api/v0/admin, and Go matches method and path together, so the POSTs
// ride the native route while every other method and every other path
// stays relayed. The OpenAPI fragment is a line-verbatim subset of the
// pinned core file (see contracts/d7-admin-four.md): it holds only the
// four route entries whose path keys are unique to this lane.
//
// Scope honesty (binding): this lane is tier-C dispatch only — HTTP
// write-through to the current core handlers — not termination, not
// removal readiness. Dest holds no ledger, mints no idempotency key,
// appends no audit event, and runs no finalize: the per-request key
// claim, the request fingerprint, the allow/deny audit append, and the
// WithoutCancel + 2s finalize stack (with the nested 500ms audit
// append) all run inside the core handlers that still serve every
// forwarded request. The ledger deficit stays open in the contract;
// serialization dest-side (queues, dedup caches) is not a substitute
// for it. Termination stays last among mutations on D0-proven
// transport only.
const (
	d7BackfillRoute   = "POST /api/v0/admin/backfill"
	d7DeadLetterRoute = "POST /api/v0/admin/dead-letter"
	d7ReindexRoute    = "POST /api/v0/admin/reindex"
	d7SkipRoute       = "POST /api/v0/admin/skip"
)

// d7Routes lists every native-dispatch route of the admin-four family
// in registration order.
var d7Routes = []string{
	d7BackfillRoute,
	d7DeadLetterRoute,
	d7ReindexRoute,
	d7SkipRoute,
}

// d7WriteThrough is the tier-C clone for the admin-four mutations:
// each request crosses an explicit per-family route with the AllScopes
// family authorizer instead of the generic byte-relay. The shape is
// HTTP write-through — method, path, query, headers (including the
// original Authorization header), and body forward to the same core
// route, and core bytes relay back unmodified. Dest holds no backfill,
// dead-letter, reindex, or skip state and performs no mutation of its
// own: the idempotency-key claim (where the core handler has one),
// the request fingerprint, the audit append, and the bounded finalize
// all run core-side on every forwarded request.
//
// Per-operation obligations (from the pin contracts, still core-side):
//
//   - Backfill (BackfillAdmission v0): the endpoint claims no
//     idempotency key and holds no ledger row, appends no audit event,
//     and runs no bounded finalize — a re-run records a second
//     backfill request row. The mutation is not even state-idempotent;
//     a retried clone inserts again until the termination lane lands
//     the client-stable key claimed core-side.
//   - Dead-letter (DeadLetterAdmission v0): the endpoint claims no key
//     and holds no ledger row, and appends no audit event. A re-run
//     sets the same dead_letter state again (state-convergent), but
//     concurrent duplicates both execute with no ledger serializing
//     them.
//   - Reindex (ReindexAdmission v0): the endpoint claims no key, holds
//     no ledger row, appends no audit event, and runs no bounded
//     finalize. A re-run is watermark-safe (the fleet upsert never
//     lowers the watermark; the repository-scoped branch records
//     nothing unless every selector resolves), but concurrent
//     duplicates both execute with no ledger serializing them. No
//     single core object satisfies ReindexAdmission — the D0/transport
//     lane owns composing the three branch ports (fleet requester,
//     repository catalog matcher, repository requester); this clone
//     forwards to the already-composed core route and composes nothing.
//     The fleet branch and the repository-scoped branch both arrive on
//     this one route; the per-backend fan-out (which scopes the core
//     handler fans out to) is named in contracts/d7-admin-four.md.
//   - Skip (SkipAdmission v0): the endpoint claims no key and holds no
//     ledger row, and runs no bounded finalize. A re-run touches only
//     rows still pending, retrying, or failed (state-convergent), but
//     concurrent duplicates both execute with no ledger serializing
//     them. The only audit today is the deny path
//     (skip_refused_ambiguous_scope); there is no allow-path event
//     naming what was skipped.
//
// Trust model, binding: dest's only credential is the shared key, which
// core maps to AllScopes (so the dest allow branch is the live one and
// core re-judges every forwarded call as backstop — all-scope admin,
// admin tenant scope, and the mutation permission catalog — which is
// also where the live 403s come from). Dest has no scoped/session
// resolver, so the deny branch is unreachable in production until one
// lands; it is recorded as such, not faked, and the tests pin its shape
// for that day. Dest adds no handler validation: request/response
// semantics stay core-owned. Governance audit stays core-side on
// forwarded requests. Dest emits none.
//
// Cancel and timeout parity: dest never retries a forwarded mutation,
// so a caller cancel or dispatch-timeout answer is the permanent local
// 503 backend_unavailable with exactly one upstream attempt — and core
// still records the committed outcome on its cancel-proof finalize
// context where the core handler has one, so the idempotency-key retry
// replays the stored outcome (or answers the in-progress 409) instead
// of losing it. Transport 500s/503s are retryable only end-to-end by
// the client, never by dest: a retried clone is a new single forward.
func (s *Server) d7WriteThrough(w http.ResponseWriter, r *http.Request) {
	s.d7WriteThroughAs(w, r, true)
}

// d7WriteThroughAs is the testable core of the tier-C clone:
// allScopes is the resolved caller scope. The routes always pass true
// today (the shared key maps to AllScopes in core); the false branch is
// the hookup point for the day dest gains a scoped/session resolver,
// and it stays a plain deny until then. Like dispatch, it fails closed
// with the permanent 503 when no core origin is configured, so the
// native routes can never dereference an unconfigured origin.
func (s *Server) d7WriteThroughAs(w http.ResponseWriter, r *http.Request, allScopes bool) {
	if s.core == nil {
		s.dispatchOutcome(r, viaNative, outcomeLocal503, 0, false)
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	// The family authorizer judgment counts on both branches, so the
	// unreachable deny branch stays observable from dest signals. A deny
	// never dispatches: it files the denied outcome on the native lane.
	if !authorizeD7Admin(allScopes) {
		if tel := s.telemetry; tel != nil {
			tel.observeAuthorizer("admin", decisionDeny)
		}
		s.dispatchOutcome(r, viaNative, outcomeDenied, 0, false)
		writeError(w, r, http.StatusForbidden, http.StatusText(http.StatusForbidden),
			"all-scopes operator authorization is required")
		return
	}
	if tel := s.telemetry; tel != nil {
		tel.observeAuthorizer("admin", decisionAllow)
	}
	forwardToCore(s, w, r, viaNative)
}

// authorizeD7Admin reports whether an identity carrying allScopes may
// call the admin-four mutations. The only identity dest can construct
// today is the shared-key holder, which core maps to AllScopes, so
// callers pass true and core re-judges (tenant scope, permission
// catalog) as backstop. The false branch exists for the day dest gains
// a scoped/session resolver; until then it is unreachable in production
// and must stay a plain deny (no invented reason codes, no audit
// widening).
func authorizeD7Admin(allScopes bool) bool {
	return allScopes
}
