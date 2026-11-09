// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

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
// Scope honesty (binding): this lane landed as tier-C dispatch only
// — HTTP write-through to the current core handlers — and the
// github-pair termination lane (see api/github_terminate.go)
// supersedes the write-through for the same two exact paths: dest now
// owns the route mapping over the versioned GitHubLoginAdmission v0
// port on D0-proven transport. Dest still holds no GitHub state,
// session, or provider material, mints no credential or idempotency
// value, keeps no ledger, appends no audit event, and runs no
// finalize: the state creation, the access-token exchange, the
// verified-email/org/team resolution, the group-to-role mapping, the
// session mint, and the allow/deny audit append all run inside the
// core handlers that still serve every fetch. The ledger deficit
// stays open in the contract; serialization dest-side (state stores,
// replay caches) is not a substitute for it.
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

// Termination supersedes the write-through (github-pair lane, Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#72): the 2 pre-auth
// public GETs terminate dest-side on serveGitHubLoginTerminated and
// serveGitHubCallbackTerminated (see api/github_terminate.go and
// contracts/github-termination.md), registered on the same exact paths
// outside every gate ahead of the generic proxy. The table above
// stays frozen: only these two spellings terminate, and the
// trailing-slash twins stay on the generic relay by design.
