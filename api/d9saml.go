// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

// SAML-login tier-C dispatch clones (Refs eshu-hq/eshu#9669, part of
// #6707; pin SAMLLoginAdmission v0; arbiter APPROVED-WITH-CONDITIONS
// on the D9 SAML wave). This is the third of three sequential family
// lanes (OIDC, then GitHub, then SAML): the 2 pre-auth public GETs
// plus the pre-auth public ACS POST register exactly those paths
// outside every gate, ahead of the relay, with the provider path
// value, the return_to query value, and the IdP-posted form fields
// crossing as opaque bytes. Route registration has no path-key
// collision: the native paths share no prefix with each other beyond
// /api/v0/auth, and Go matches method and path together, so each GET
// and the ACS POST ride their native routes while every other method
// and every other path stays on its existing lane. The trailing-slash
// twins (.../metadata/, .../login/, .../acs/) are subtree patterns
// and stay on the generic relay instead: keyless callers keep the
// standing 401 there, keyed callers get core's answer for the spelled
// path. The OpenAPI entries are a line-verbatim subset of the pinned
// core file (see contracts/d9-saml-login.md).
//
// Scope honesty (binding): this lane landed as tier-C dispatch only
// — HTTP write-through to the current core handlers — and the
// saml-triple termination lane (see api/saml_terminate.go)
// supersedes the write-through for the same three exact paths: dest
// now owns the route mapping over the versioned SAMLLoginAdmission v0
// port on D0-proven transport. Dest still holds no SAML state,
// session, or provider material, mints no credential or idempotency
// value, keeps no replay cache, appends no audit event, and runs no
// finalize: the request creation and RelayState hashing, the
// signed-assertion verification, the replay reservation, the
// group-to-role mapping, the session mint, and the allow/deny audit
// append all run inside the core handlers that still serve every
// fetch. The ledger deficit stays open in the contract; serialization
// dest-side (state stores, replay caches) is not a substitute for it.
const (
	d9SAMLMetadata = "GET /api/v0/auth/saml/providers/{provider_id}/metadata"
	d9SAMLLogin    = "GET /api/v0/auth/saml/providers/{provider_id}/login"
	d9SAMLACS      = "POST /api/v0/auth/saml/providers/{provider_id}/acs"
)

// d9SAMLRoutes lists every native-dispatch route of the D9 SAML lane
// in registration order: exactly the two pre-auth public GETs plus
// the pre-auth public ACS POST. No OIDC route rides this table; no
// GitHub route rides this lane.
var d9SAMLRoutes = []string{
	d9SAMLMetadata,
	d9SAMLLogin,
	d9SAMLACS,
}

// Termination supersedes the write-through (saml-triple lane, Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#72): the 2 pre-auth
// public GETs plus the pre-auth public ACS POST terminate dest-side
// on serveSAMLMetadataTerminated, serveSAMLLoginTerminated, and
// serveSAMLACSTerminated (see api/saml_terminate.go and
// contracts/saml-termination.md), registered on the same exact paths
// outside every gate ahead of the generic proxy. The table above
// stays frozen: only these three spellings terminate, and the
// trailing-slash twins stay on the generic relay by design. The
// write-through method lived here before this lane; rollback restores
// it from this lane's diff.
