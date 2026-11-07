# D9 SAML-login tier-C dispatch clones contract

Versioned pin record and clone-lane record for the 3 SAML-login routes
(Refs `eshu-hq/eshu#9669`, part of `eshu-hq/eshu#6707`). This is the
third of three sequential family PRs in the D9 wave (OIDC, then GitHub,
then SAML per the arbiter APPROVED-WITH-CONDITIONS verdict,
`eshu-hq/eshu#9669` comment `6701998359`): the two GETs plus the ACS
POST register exactly those paths outside every gate ahead of the
relay. Method, path, query, headers, and body forward to the same core
route; core bytes come back unmodified and core re-judges every
forwarded call as backstop. Dest holds no provider material, session,
replay reservation, or request record. OIDC and GitHub stay on their
own landed lanes.

## Pin records (satellite-side, D7/D8/D9-OIDC/D9-GitHub precedent)

Pins minted core-side in `eshu-hq/eshu` (`SAMLLoginAdmissionContractVersion`
= `v0`, 5 methods: `GetSAMLProvider`, `CreateSAMLRequest`,
`ConsumeSAMLRequest`, `ReserveSAMLReplay`, `ResolveSAMLPrincipal`),
recorded here at the fresh D9 SAML wave SHA
`8c2324693320c8f69ef3e8078c795956b549d7ad`
(`origin/main` at implementation time, a descendant of the D9 GitHub
wave `8c23246933` — the wave SHA is unchanged from the GitHub lane
because the SAML sources are byte-identical between the two; the stale
`aac271ad` anchors do not discharge this lane). All three sources are
byte-identical at the stale and fresh pins (`git diff
aac271ad..8c23246933` over the 5 SAML files is empty, arbiter
verified), so the vendored copies move by re-anchor only — no content
change:

| Core source at pin | Satellite record | sha256 | Treatment |
| --- | --- | --- | --- |
| `go/internal/query/saml_login_admission_contract_test.go` | `contracts/upstream/saml_login_admission_contract_test.go` | `bc04787bd208bf07428f4a9bc4660946342c7e4f3b60121202cb7f0a070556d0` | verbatim |
| `go/internal/query/saml_admission.go` (version, alias, `SAMLStore`, `SAMLProviderConfig`, `SAMLRequestCreateRecord`, `SAMLProviderIDLister`) + `saml_handler.go` (`Mount`) | `contracts/upstream/saml-login-wire-shapes.txt` | excerpt (line-presence at pin) | verbatim excerpt |
| `go/internal/query/openapi/paths/auth/routes.go` (SAML metadata + login + ACS entries only) | `openapi/paths/auth/saml_login.go` | `709fe6b3e96967fa615bff59a8c02cb8c1c5fbc6a05744bf57835170a5ad633a` (whole file; this fragment is the line-verbatim SAMLLogin subset) | verbatim subset |

`scripts/check-parity.sh` gates all three at `SRC_SHA_D9_SAML`
(`8c2324693320c8f69ef3e8078c795956b549d7ad`).

The frozen v0 method set stays the 5 store methods. The optional
`SAMLProviderIDLister` is deliberately not part of v0 (see the
`SAMLLoginAdmission` doc comment): a dest client implementing only the
5 admission methods silently drops runtime provider discovery and must
implement the lister or accept DB-row-only discovery. Browser-session
issuance on ACS belongs to the BrowserSessions family's own seam, not
this contract.

## Per-exact-route C-c1 carve-out (recorded before code)

Core serves exactly these three paths pre-auth, with the method pinned
per path:

- `go/internal/query/auth_public_routes.go` (`publicHTTPRoute`):
  exact-path bypass for the SAML metadata GET, login GET, and ACS
  POST. The method gate is load-bearing: a POST to metadata/login or
  a GET to ACS is NOT public.
- `go/internal/query/saml_handler.go` (`Mount`, lines 136–140):
  registers only `GET
  /api/v0/auth/saml/providers/{provider_id}/metadata`, `GET
  /api/v0/auth/saml/providers/{provider_id}/login`, and `POST
  /api/v0/auth/saml/providers/{provider_id}/acs` — no other method, no
  other path, single-segment `{provider_id}` only.
- Conditional mount: `go/internal/query/handler.go` mounts SAML only
  `if a.SAML != nil`; the store-backed handler builder returns nil
  when SAML is unset, so a disabled provider leaves all three paths
  unmounted core-side and the relay carries core's real unmounted
  answer. Dest synthesizes nothing.

Dest mirrors the carve-out exactly: the 3 routes register as exact
paths outside every gate, ahead of the relay, on the public
write-through handler. The carve-out is exactly these three paths:

- No trailing-slash twins (`.../metadata/`, `.../login/`,
  `.../acs/` would be subtree patterns swallowing past the bypass).
  Slash spellings stay on the generic relay: keyless callers keep the
  standing dest 401 with zero core contact; keyed callers get core's
  answer for the spelled path.
- No other method: a POST to metadata/login or a GET to ACS is not
  carved out and keeps the standing 401 keyless.
- No OIDC or GitHub route leaves the gate in this lane. Those ride
  their own landed lanes.
- The empty provider spelling (`.../providers//login`) never reaches
  a route: Go's mux sanitizes the doubled slash with a redirect to
  the cleaned path, and the cleaned path matches no carve-out route,
  so keyless callers end at the standing 401 with zero core contact.
  The multi-segment spelling (`.../providers/a/b/login`) matches no
  single-segment wildcard and keeps the standing 401 the same way.
  The escaped-slash spelling (`.../providers/a%2Fb/login`) matches
  the wildcard as one segment and forwards opaquely — dest and core
  see the same bytes.
- HEAD on the metadata/login GETs routes to the GET pattern (Go mux)
  and forwards once for core to re-judge; dest adds no HEAD branch.
- No shared-key injection: `forwardToCore` clones the caller headers
  and invents none, so a caller key that happens to arrive crosses as
  an ordinary header, untouched and unjudged.
- The provider path value, the `return_to` query value, and the ACS
  form fields (`RelayState`, `SAMLResponse`) are credential material:
  dest parses none of them (no form/body read on this lane), and no
  `RequestURI`, `Location`, `Set-Cookie`, `RelayState`, or
  `SAMLResponse` value reaches logs, metrics, or spans (the
  middleware logs method, family, via, status, and latencies only).
- No body cap dest-side: the ACS body crosses as a stream and only
  core's own cap applies (core wraps the ACS body in its 1MiB
  MaxBytesReader before parsing the form). Near-1MiB and over-1MiB
  bodies forward byte-intact with exactly one upstream attempt each;
  core's answer (including its oversize 400) relays unmodified.
- No retries: start and ACS are census mutations (login start creates
  server-side state; an ACS consume spends the request), so dest makes
  exactly one upstream attempt on every shape, including POST ACS on
  500, retryable-503, dead core, and caller cancel.
- No dest-side SAML client ever: no store, no provider config, no
  verifier, no keyring, no client library — HTTP bytes cross via the
  existing shared streaming forward. Gated by the no-client-symbols
  source test and the `check-parity.sh` negative grep.
- Redirects are never followed dest-side (`dispatchClient`
  `ErrUseLastResponse`, pinned by test): a 302/303 relays as a
  302/303.
- Audit stays core-side: `auditSAMLLogin` runs inside the core login
  and ACS handlers on every forwarded request. Dest emits none.
- Nil core answers the permanent 503 `backend_unavailable` without
  `Retry-After`, in both negotiated shapes.

## HTTP shape

- Metadata: the public metadata GET advertises the
  provider-specific EntityID and POST ACS endpoint only; the XML
  answer relays with its `application/samlmetadata+xml` content
  type. Unknown providers fail closed with core's 404, relayed
  unmodified.
- Login: the public SP-initiated login GET creates the AuthnRequest,
  stores only the RelayState hash with the request ID and short
  expiry, and answers the 302 to the IdP SSO endpoint. Failures fail
  closed without leaking provider or claim details: 404
  unknown-provider, 500 with the cause on the request span only (499
  on client cancel).
- ACS: the public ACS POST accepts the IdP-posted `RelayState` +
  `SAMLResponse` form fields. The 201 answer carries the
  browser-session JSON plus the two `Set-Cookie` session cookies; the
  303 answer carries both cookies before redirecting to the stored
  same-origin return path. Consumed, unknown, or invalid requests fail
  closed with core's 400/401, relayed byte-identical — a spent request
  answers 401 and the user restarts login.
- Every row above relays core bytes unmodified, including redirect
  status, `Location`, multiple `Set-Cookie` values, and every error
  envelope shape.

## Native dispatch (tier C, D9 SAML)

`api/d9saml.go` registers the 3 pre-auth public routes outside every
gate ahead of the generic proxy on the public write-through handler
(no dest-side gate at all — an anonymous browser beginning a login,
or the IdP posting the ACS form, carries no session, bearer, or prior
credential to present). The shape is HTTP write-through — method,
path, query, headers, and body forward to the same core route, and
core bytes come back unmodified — because every trust decision stays
core-side: the request creation and RelayState hashing, the
signed-assertion verification, the replay reservation, the
group-to-role mapping, the session mint, and the audit append all run
inside the core handlers that still serve every forwarded request.

Scope honesty: dest adds no handler validation and keeps no state.
The provider path value, the return_to query value, and the ACS form
fields cross as opaque bytes — never parsed, logged, or telemetered
— the dest shared key is never injected, the core's `Location` and
`Set-Cookie` answers relay unmodified, and a forwarded login is never
retried.

Cancel and timeout parity: dest never retries a forwarded login, so a
caller cancel or dispatch-timeout answer is the permanent local 503
`backend_unavailable` with exactly one upstream attempt.

Audit parity: governance audit stays core-side on forwarded requests
(`identity_authentication` / `sso_login_authenticated` and the
denied classifications emit from inside the core login and ACS
handlers). This lane re-implements nothing. `scripts/check-parity.sh`
gates the lane with the no-core-import proof (`go list -deps` over the
runtime packages must show no `eshu-hq/eshu` import), the
no-client-symbols negative grep, and the carve-out, 401-boundary,
no-retry, no-form-read, redirect-unfollowed, envelope-parity, and
ledger-deficit proofs above.

OpenAPI: the three route entries move as a line-verbatim subset of the
pinned core `routes.go` (the SAML metadata + login + ACS entries,
`openapi/paths/auth/saml_login.go` const `SAMLLogin`),
referencing only the `BrowserSessionResponse` schema the D8 lane
already ported — no new schemas. Assembly adds no new components;
`TestSpec_LocalRefsResolve` fails closed on a dangling `$ref`. The
served dest spec therefore covers exactly the landed families, and no
served key duplicates a relayed route — every relayed family stays
relay-only until its lane.

## Telemetry family note (disclosed, not a second contract)

Telemetry family is the closed metrics label, not the admission
family: all three SAML routes file under the pre-existing
`saml_login` prefix (longest-prefix match on `/auth/saml/`,
`via="native"`). The proof box above is organized per admission
family; the ahead-of-relay test pins the per-label counts.

## Envelope scope-honesty label

Client-visible scope posture for the 3 routes: `tier-C dispatch` —
the destination forwards the request to the authoritative in-tree
route and relays its answer; it does not terminate the route, keep
provider material, resolve session/scoped identity, mint credentials
or idempotency values, keep a ledger, append audit, or bound finalize.
Termination and removal readiness are explicitly not claimed.

## Rollback

Additive. Revert this lane's dest commit (or re-pin the prior digest)
and the 3 SAML routes fall back to the generic byte-relay —
answering dest 401s keyless without ever reaching core — until the
lane returns. No core change, no migration, no flag; no dest-side
SAML, session, or provider state exists to unwind. Verify by
re-running the lane conformance suite against the reverted tree: the
ahead-of-relay assertions go red (the routes relay again) while every
other suite stays green. No Plan A rehearsal is consumed by this
rollback — it restores a proxy path, not durable state.

## NEVER-list (binding on this lane)

No dest-side SAML client of any kind (no store, provider config,
verifier, keyring, or client library); no dest-side state store,
replay cache, session mint, metadata rendering, request record,
provider config, client secret, form parsing, or body cap; no
dest-invented pins, contracts, or codes; no retries, no
serialization; no termination claims. The D9 wave is complete with
this lane: all three sequential family lanes (OIDC, GitHub, SAML)
are tier-C dispatch clones.
