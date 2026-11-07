// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"crypto/subtle"
	"net/http"
)

// Session-resolver tier-C dispatch clones (Refs eshu-hq/eshu#9669, part
// of #6707; pins minted as SessionResolver, BrowserSession,
// LocalIdentity, and APIToken Admission v0; arbiter
// APPROVED-WITH-CONDITIONS on that wave). The 18 authed routes have
// their explicit per-family routes inside the D8 credential-class gate,
// ahead of the generic proxy; the 4 pre-auth public routes register
// exactly those paths outside every gate, ahead of the relay, with
// credential bodies crossing as opaque bytes. Route registration has no
// path-key collision: the native paths share no prefix with each other
// beyond /api/v0/auth, and Go matches method and path together, so each
// POST rides its native route while every other method and every other
// path stays on its existing lane — except the two d8SlashSkip
// spellings, whose slash twins would panic against the
// invitations/{invite_id}/revoke wildcard sibling and stay on the
// generic relay instead. The OpenAPI entries are line-verbatim subsets
// of the pinned core file (see contracts/d8-session-resolver.md).
//
// Scope honesty (binding): this lane is tier-C dispatch only — HTTP
// write-through to the current core handlers — not termination, not
// removal readiness. Dest holds no session, identity, token, or TOTP
// state, mints no credential or idempotency value, keeps no ledger,
// appends no audit event, and runs no finalize: the session mint, the
// token hash, the TOTP seal, the allow/deny audit append, and the
// bounded finalize all run inside the core handlers that still serve
// every forwarded request. The ledger deficit stays open in the
// contract; serialization dest-side (queues, dedup caches) is not a
// substitute for it. Termination stays last on D0-proven transport
// only.
const (
	d8BrowserSessionCreate  = "POST /api/v0/auth/browser-session"
	d8BrowserSessionRead    = "GET /api/v0/auth/browser-session"
	d8BrowserSessionRevoke  = "DELETE /api/v0/auth/browser-session"
	d8BrowserSessionContext = "PATCH /api/v0/auth/browser-session/context"
	d8SessionList           = "GET /api/v0/auth/sessions"
	d8Profile               = "GET /api/v0/auth/profile"
	d8Bootstrap             = "POST /api/v0/auth/local/bootstrap"
	d8Login                 = "POST /api/v0/auth/local/login"
	d8InvitationsCreate     = "POST /api/v0/auth/local/invitations"
	d8InvitationsAccept     = "POST /api/v0/auth/local/invitations/accept"
	d8PasswordReset         = "POST /api/v0/auth/local/users/{user_id}/password"
	d8PasswordRotate        = "POST /api/v0/auth/local/password/rotate"
	d8MFAReset              = "POST /api/v0/auth/local/users/{user_id}/mfa-reset"
	d8Disable               = "POST /api/v0/auth/local/users/{user_id}/disable"
	d8BreakGlass            = "POST /api/v0/auth/local/break-glass"
	d8BreakGlassSession     = "POST /api/v0/auth/local/break-glass/session"
	d8APITokensList         = "GET /api/v0/auth/local/api-tokens"
	d8APITokensCreate       = "POST /api/v0/auth/local/api-tokens"
	d8APITokenRevoke        = "POST /api/v0/auth/local/api-tokens/{token_id}/revoke"
	d8APITokenRotate        = "POST /api/v0/auth/local/api-tokens/{token_id}/rotate"
	d8TOTPBegin             = "POST /api/v0/auth/local/mfa/totp/begin"
	d8TOTPConfirm           = "POST /api/v0/auth/local/mfa/totp/confirm"
)

// d8GatedRoutes lists the authed D8 routes in registration order: the
// 18 routes that cross the D8 credential-class gate (shared-key callers
// ride the native write-through; unresolvable credential classes relay
// explicitly so core judges them; credential-less callers keep the
// standing 401).
var d8GatedRoutes = []string{
	d8BrowserSessionCreate,
	d8BrowserSessionRead,
	d8BrowserSessionRevoke,
	d8BrowserSessionContext,
	d8SessionList,
	d8Profile,
	d8Bootstrap,
	d8InvitationsCreate,
	d8PasswordReset,
	d8MFAReset,
	d8Disable,
	d8BreakGlass,
	d8APITokensList,
	d8APITokensCreate,
	d8APITokenRevoke,
	d8APITokenRotate,
	d8TOTPBegin,
	d8TOTPConfirm,
}

// d8PublicRoutes lists the D8 pre-auth public carve-out in registration
// order: exactly the four paths core serves pre-auth
// (TestHandleAcceptInvitationHandleBreakGlassSessionLocalIdentityPublicPathsAreNarrow
// at the D8 pin). They register OUTSIDE every gate, ahead of the relay,
// one exact path each — the C-c1 carve-out grade. No other D8 route
// leaves the gate in this lane, and no shared key is ever injected on
// these routes: a caller key that happens to arrive crosses as an
// ordinary header, untouched and unjudged.
var d8PublicRoutes = []string{
	d8Login,
	d8InvitationsAccept,
	d8BreakGlassSession,
	d8PasswordRotate,
}

// d8Routes lists every native-dispatch route of the D8 lane in
// registration order: the 18 gated routes plus the 4 public
// carve-out routes.
var d8Routes = append(append([]string(nil), d8GatedRoutes...), d8PublicRoutes...)

// d8SlashSkip names the two D8 routes whose trailing-slash twin would
// panic against the admin-identity wildcard sibling (POST
// /api/v0/auth/local/invitations/{invite_id}/revoke/): the subtree
// twin "invitations/" and "invitations/accept/" both overlap that
// sibling with no precedence either way. Those two spellings stay on
// the generic relay (keyed callers get core's answer for the spelled
// path; keyless callers keep the standing 401) while the exact
// spellings ride the native lane — the same never-split compromise
// the D4 lane records in d4SlashSkip.
var d8SlashSkip = map[string]bool{
	"POST /api/v0/auth/local/invitations":        true,
	"POST /api/v0/auth/local/invitations/accept": true,
}

// d8WriteThrough is the tier-C clone for the 18 authed D8 routes: each
// request crosses an explicit per-family route inside the D8 gate with
// the family authorizer instead of the generic byte-relay. The shape is
// HTTP write-through — method, path, query, headers (including the
// original Authorization header), and body forward to the same core
// route, and core bytes come back unmodified. Dest holds no session,
// identity, token, or TOTP state and performs no mutation of its own:
// the session mint and hash comparison, the token hash, the TOTP seal,
// the audit append, and the bounded finalize all run core-side on every
// forwarded request.
//
// Per-operation obligations (from the pin contracts, still core-side):
//
//   - BrowserSessions (BrowserSessionAdmission v0): dest creates no
//     session, revokes none, switches no workspace, and lists none —
//     the Current flag stays server-computed by hash comparison
//     core-side. A retried create mints a second session; dest dedups
//     nothing.
//   - SessionList (same pin): metadata-only core-side; hashes never
//     cross to dest because dest never reads the rows — it relays the
//     core bytes.
//   - Profile and LocalIdentity (LocalIdentityAdmission v0): dest
//     verifies no password, proof, code, or secret — raw proofs cross
//     as opaque bytes and core judges them. Login client secrets and
//     SAML keys are NOT in this lane (D9 NEVER-list).
//   - API tokens (APITokenAdmission v0): dest creates hash-only tokens
//     never — creation, revocation, rotation, and the metadata-only
//     list all run core-side. Token hashes never cross to dest.
//
// Trust model, binding: dest's only constructible identity is the
// shared key, which core maps to AllScopes (so the dest allow branch is
// the live one and core re-judges every forwarded call as backstop —
// tenant scope, the permission catalog, and the session/cookie
// judgments — which is also where the live 403s and 401s come from).
// Dest has no session resolver, so the deny branch is unreachable in
// production until one lands; it is recorded as such, not faked, and
// the tests pin its shape for that day. Dest adds no handler
// validation: request/response semantics stay core-owned. Governance
// audit stays core-side on forwarded requests. Dest emits none.
//
// Cancel and timeout parity: dest never retries a forwarded mutation,
// so a caller cancel or dispatch-timeout answer is the permanent local
// 503 with exactly one upstream attempt — and core still records the
// committed outcome on its cancel-proof finalize context where the core
// handler has one. Transport 500s/503s are retryable only end-to-end by
// the client, never by dest: a retried clone is a new single forward.
func (s *Server) d8WriteThrough(w http.ResponseWriter, r *http.Request) {
	s.d8WriteThroughAs(w, r, true)
}

// d8WriteThroughAs is the testable core of the tier-C clone:
// allScopes is the resolved caller scope. The gate always passes true
// today (the shared key maps to AllScopes in core); the false branch is
// the hookup point for the day dest gains a session resolver, and it
// stays a plain deny until then. Like dispatch, it fails closed with
// the permanent 503 when no core origin is configured, so the native
// routes can never dereference an unconfigured origin.
func (s *Server) d8WriteThroughAs(w http.ResponseWriter, r *http.Request, allScopes bool) {
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
	if !authorizeD8(allScopes) {
		if tel := s.telemetry; tel != nil {
			tel.observeAuthorizer(family, decisionDeny)
		}
		s.dispatchOutcome(r, viaNative, outcomeDenied, 0, false)
		writeError(w, r, http.StatusForbidden, http.StatusText(http.StatusForbidden),
			d8FamilyDenyMessage)
		return
	}
	if tel := s.telemetry; tel != nil {
		tel.observeAuthorizer(family, decisionAllow)
	}
	forwardToCore(s, w, r, viaNative)
}

// d8FamilyDenyMessage is the single deny shape for the D8 lane: the
// only identity dest can construct today is the shared-key (AllScopes)
// holder, so anything the gate resolved but the authorizer refused is a
// future-resolver identity. The branch is unreachable in production
// until the session resolver lands; the live 403s and 401s on these
// routes are core's. The contract records the branch as such.
const d8FamilyDenyMessage = "all-scopes authorization is required"

// authorizeD8 reports whether an identity carrying allScopes may call
// the D8 lane. The only identity dest can construct today is the
// shared-key holder, which core maps to AllScopes, so callers pass true
// and core re-judges (tenant scope, permission catalog, session and
// cookie judgments) as backstop. The false branch exists for the day
// dest gains a session resolver; until then it is unreachable in
// production and must stay a plain deny (no invented reason codes, no
// audit widening).
func authorizeD8(allScopes bool) bool {
	return allScopes
}

// d8Gate fronts the 18 authed D8 routes. A valid shared key rides the
// native write-through (next); a request carrying any other credential
// — a non-key Bearer [REDACTED] foreign credential class (scoped
// token, OIDC) or a session-cookie-shaped Cookie — relays explicitly to
// the same core route on the relay lane, so core judges the class dest
// cannot resolve and dest invents no failure for it; a credential-less
// request (or a junk-cookie carrier, per the D2 follow-up lesson) gets
// the standing dest 401 with zero core contact. An empty configured key
// fails closed like RequireAuth.
func (s *Server) d8Gate(next http.Handler) http.Handler {
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
			s.d8RelayExplicit(w, r)
			return
		}
		if hasSessionCookie(r) {
			// A session-cookie-shaped Cookie without a Bearer [REDACTED] a
			// browser-session caller: relay explicitly so core judges it.
			// Junk cookies fall through to the standing 401 below.
			s.d8RelayExplicit(w, r)
			return
		}
		WriteUnauthorized(w, r)
	})
}

// d8RelayExplicit forwards one D8 call carrying an unresolvable
// credential class to the same core route and relays core bytes
// untouched on the relay lane. The instrumented middleware files the
// lane from the record, so the gate flips the record lane before
// forwarding; direct (uninstrumented) calls already carry the relay
// lane through forwardToCore.
func (s *Server) d8RelayExplicit(w http.ResponseWriter, r *http.Request) {
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

// d8PublicWriteThrough is the tier-C clone for the 4 pre-auth public
// D8 routes (C-c1 carve-out grade): each request crosses an explicit
// public per-route registration outside every gate instead of the
// generic byte-relay, with no dest-side gate at all — a fresh caller
// has no session, Bearer [REDACTED], or prior credential to present.
// The shape is HTTP write-through — method, path, query, headers, and
// body forward to the same core route, and core bytes come back
// unmodified — because every trust decision stays core-side: core's
// AuthMiddleware public-path bypass plus the per-request re-proofs the
// core handlers run (current-password and MFA re-proof on rotate, the
// invite-code proof on accept, the break-glass code proof on session)
// still run on every forwarded request.
//
// Dest adds no validation and keeps no state. The credential bodies
// (passwords, invite codes, break-glass codes, TOTP proofs) cross as
// opaque bytes — never parsed, logged, or telemetered — the dest shared
// key is never injected as Authorization on these routes (forwardToCore
// clones the caller headers and invents none), the core's Set-Cookie
// answers relay unmodified, and a forwarded POST is never retried.
//
// Governance audit stays core-side on forwarded requests: the in-tree
// local-identity handlers emit audit (identity-authentication,
// password rotation, invitation acceptance, break-glass) from inside
// the core handlers that still run on every forwarded request, so
// write-through keeps every audit event core-side with no gap. Dest
// emits none. A family whose handlers emit audit must never be
// re-implemented dest-side without a versioned audit-append contract;
// this lane re-implements nothing.
func (s *Server) d8PublicWriteThrough(w http.ResponseWriter, r *http.Request) {
	if s.core == nil {
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	forwardToCore(s, w, r, viaNative)
}
