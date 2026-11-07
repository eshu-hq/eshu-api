// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// GitHub-login tier-C dispatch clones (Refs eshu-hq/eshu#9669, part of
// #6707; pin GitHubLoginAdmission v0; arbiter APPROVED-WITH-CONDITIONS
// on the D9 GitHub wave). This is the second of three sequential family
// lanes (OIDC, then GitHub, then SAML): the 2 pre-auth public GETs
// register exactly those paths outside every gate, ahead of the relay,
// with the credential query values (state, code, provider IDs,
// return_to) crossing as opaque bytes. Route registration has no
// path-key collision: the native paths share no prefix with each other
// beyond /api/v0/auth, and Go matches method and path together, so
// each GET rides its native route while every other method and every
// other path stays on its existing lane. The trailing-slash twins
// (.../login/, .../callback/) are subtree patterns and stay on the
// generic relay instead: keyless callers keep the standing 401 there,
// keyed callers get core's answer for the spelled path. The OpenAPI
// entries are a line-verbatim subset of the pinned core file (see
// contracts/d9-github-login.md).
//
// Scope honesty (binding): this lane is tier-C dispatch only — HTTP
// write-through to the current core handlers — not termination, not
// removal readiness. Dest holds no GitHub state, session, or provider
// material, mints no credential or idempotency value, keeps no
// ledger, appends no audit, and runs no finalize: the state
// creation, the access-token exchange, the verified-email/org/team
// resolution, the group-to-role mapping, the session mint, and the
// allow/deny audit append all run inside the core handlers that still
// serve every forwarded request. The ledger deficit stays open in the
// contract; serialization dest-side (state stores, replay caches) is
// not a substitute for it. Termination stays last on D0-proven
// transport only.
const (
	d9GitHubLogin    = "GET /api/v0/auth/github/login"
	d9GitHubCallback = "GET /api/v0/auth/github/callback"
)

// d9GitHubRoutes lists every native-dispatch route of the D9 GitHub
// lane in registration order: exactly the two pre-auth public carve-out
// GETs. No OIDC route rides this table; no SAML route rides this lane.
var d9GitHubRoutes = []string{
	d9GitHubLogin,
	d9GitHubCallback,
}

// d9GitHubPublicWriteThrough is the tier-C clone for the 2 pre-auth
// public GitHub routes (C-c1 carve-out grade): each request crosses an
// explicit public per-route registration outside every gate instead of
// the generic byte-relay, with no dest-side gate at all — an anonymous
// browser beginning or completing a GitHub login carries no session,
// bearer, or prior credential to present. The shape is HTTP
// write-through — method, path, query, headers, and (empty) body
// forward to the same core route, and core bytes come back unmodified —
// because every trust decision stays core-side: core's AuthMiddleware
// public-path bypass plus the per-request proofs the core handlers run
// (provider selection and state creation on start; one-time state,
// redirect-URI proof, and verified-email/org/team resolution on
// callback) still run on every forwarded request.
//
// Dest adds no validation and keeps no state. The credential query
// values (state, code, provider IDs, return_to) cross as opaque bytes
// — never parsed, logged, or telemetered — the dest shared key is
// never injected as Authorization on these routes (forwardToCore
// clones the caller headers and invents none), the core's Location
// and Set-Cookie answers relay unmodified, redirects are never
// followed dest-side, and a forwarded login is never retried: these
// GETs are census mutations (login start creates server-side state),
// so dest makes exactly one upstream attempt on every shape.
//
// Governance audit stays core-side on forwarded requests: the
// identity-authentication event (allowed "sso_login_authenticated" or
// a denied classification such as "org_not_allowed" carrying the
// allow-list reason) emits from inside the core callback handler that
// still runs on every forwarded request, so write-through keeps every
// audit event core-side with no gap. Dest emits none. A family whose
// handlers emit audit must never be re-implemented dest-side without a
// versioned audit-append contract; this lane re-implements nothing.
func (s *Server) d9GitHubPublicWriteThrough(w http.ResponseWriter, r *http.Request) {
	if s.core == nil {
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	forwardToCore(s, w, r, viaNative)
}
