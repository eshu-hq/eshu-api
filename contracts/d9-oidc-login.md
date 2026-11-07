# D9 OIDC-login tier-C dispatch clones contract

Versioned pin record and clone-lane record for the 2 OIDC-login routes
(Refs `eshu-hq/eshu#9669`, part of `eshu-hq/eshu#6707`). This is the
first of three sequential family PRs in the D9 wave (OIDC, then GitHub,
then SAML per the arbiter APPROVED-WITH-CONDITIONS verdict,
`eshu-hq/eshu#9669` comment `6701998359`): the two OIDC GETs register
exactly those paths outside every gate ahead of the relay. Method,
path, query, headers, and (empty) body forward to the same core route;
core bytes come back unmodified and core re-judges every forwarded
call as backstop. Dest holds no OIDC state, session, or provider
material. GitHub and SAML routes are NOT in this lane.

## Pin records (satellite-side, D7/D8 precedent)

Pins minted core-side in `eshu-hq/eshu` (`OIDCLoginAdmissionContractVersion
= "v0"`, 2 methods: `StartOIDCLogin` for start = IDs + `return_to`,
`CompleteOIDCLogin` for complete = `state` + `code`), recorded here at
the fresh D9 wave SHA `40e88be128ca876f7333588137af5a780bf31b93`
(`origin/main` at implementation time, a descendant of the D8 pin
`91f2df28db`; the stale `1e034786` anchors do not discharge this
lane). All three sources are byte-identical at the stale and fresh
pins, so the vendored copies move by re-anchor only — no content
change:

| Core source at pin | Satellite record | sha256 | Treatment |
| --- | --- | --- | --- |
| `go/internal/query/oidc_login_admission_contract_test.go` | `contracts/upstream/oidc_login_admission_contract_test.go` | `03342f7ad0d28aef72583ce21a7747fe360fe755367fef40660e58361f19d2c0` | verbatim |
| `go/internal/query/oidc_login_handler.go` (version, alias, `OIDCLoginService`, request/response shapes, `Mount`) + `go/cmd/api/oidc_login.go` (adapter, 503) | `contracts/upstream/oidc-login-wire-shapes.txt` | excerpt (line-presence at pin) | verbatim excerpt |
| `go/internal/query/openapi/paths/auth/routes.go` (OIDC login + callback entries only) | `openapi/paths/auth/oidc_login.go` | `709fe6b3e96967fa615bff59a8c02cb8c1c5fbc6a05744bf57835170a5ad633a` (whole file; this fragment is the line-verbatim OIDC subset) | verbatim subset |

`scripts/check-parity.sh` gates all three at `SRC_SHA_D9`
(`40e88be128ca876f7333588137af5a780bf31b93`).

The frozen v0 method set stays `StartOIDCLogin` +
`CompleteOIDCLogin`. The optional `OIDCProviderLister` is
deliberately not part of v0 (see the `OIDCLoginAdmission` doc
comment): a dest client implementing only the 2 admission methods
silently drops runtime provider discovery and must implement the
lister or accept DB-row-only discovery. Browser-session issuance on
callback belongs to the BrowserSessions family's own seam, not this
contract.

## Per-exact-route C-c1 carve-out (recorded before code)

Core serves exactly these two paths pre-auth, and only as GET:

- `go/internal/query/auth_public_routes.go` (`publicHTTPRoute`):
  `GET /api/v0/auth/oidc/login` and `GET /api/v0/auth/oidc/callback`
  bypass `AuthMiddleware` by exact-path match. The method gate is
  load-bearing: a POST to either path is NOT public.
- `go/internal/query/oidc_login_handler.go` (`Mount`): registers
  only `GET /api/v0/auth/oidc/login` and `GET
  /api/v0/auth/oidc/callback` — no other method, no other path.
- Conditional mount: `go/internal/query/handler.go` mounts only `if
  a.OIDCLogin != nil`; `newOIDCLoginHandler` (`go/cmd/api/oidc_login.go`)
  returns nil when OIDC is unset
  (`TestNewOIDCLoginHandlerDisabledByDefault`), so a disabled provider
  leaves both paths unmounted core-side and the relay carries core's
  real unmounted answer. Dest synthesizes nothing.

Dest mirrors the carve-out exactly: the two GETs register as exact
paths outside every gate, ahead of the relay, on the public
write-through handler. The carve-out is exactly these two paths:

- No trailing-slash twins (`.../login/`, `.../callback/` would be
  subtree patterns swallowing past the bypass). Slash spellings stay
  on the generic relay: keyless callers keep the standing dest 401
  with zero core contact; keyed callers get core's answer for the
  spelled path.
- No other method: POST (or any non-GET) to either path is not carved
  out and keeps the standing 401 keyless.
- No GitHub or SAML route leaves the gate in this lane.
- No shared-key injection: `forwardToCore` clones the caller headers
  and invents none, so a caller key that happens to arrive crosses as
  an ordinary header, untouched and unjudged.
- Query values (`state`, `code`, plus the start IDs and `return_to`)
  are credential material: dest parses none of them (no `ParseForm`,
  no `Query()` read on this lane), and no `RequestURI`, `Location`,
  or `Set-Cookie` value reaches logs, metrics, or spans (the
  middleware logs method, family, via, status, and latencies only).
- No retries: these GETs are census mutations (login start creates
  server-side state), so dest makes exactly one upstream attempt
  including on 500, retryable-503, dead-core, and cancel shapes.
- Redirects are never followed dest-side (`dispatchClient`
  `ErrUseLastResponse`, pinned by test): a 302/303 relays as a
  302/303.
- Audit stays core-side: `auditOIDCSSOLogin` runs inside the core
  callback handler on every forwarded request. Dest emits none.
- Nil core answers the permanent 503 `backend_unavailable` without
  `Retry-After`, in both negotiated shapes.

## HTTP shape

- Login start: the start request selects one configured provider and
  the tenant/workspace boundary plus an optional same-origin
  `return_to`; the 302 answer carries the provider authorization
  `Location`. Failures fail closed without leaking provider or claim
  details: 503 unconfigured (`oidc login is unavailable`), 400
  invalid, 401, 500 with the cause on the request span only.
- Callback: `state` + `code` resolve into the auth context (provider
  subject id, hashed group claims, proof timestamp). The 201 answer
  carries the browser-session JSON plus the two `Set-Cookie` session
  cookies; the 303 answer carries both cookies before redirecting to
  the stored same-origin return path. Denied logins (403, unmapped
  groups or revoked grants) create no session.
- Every row above relays core bytes unmodified, including redirect
  status, `Location`, multiple `Set-Cookie` values, and both error
  envelope shapes.

## Native dispatch (tier C, D9 OIDC)

`api/d9.go` registers the 2 pre-auth public GETs outside every gate
ahead of the generic proxy on the public write-through handler (no
dest-side gate at all — an anonymous browser beginning or completing
a login carries no session, bearer, or prior credential to present).
The shape is HTTP write-through — method, path, query, headers, and
body forward to the same core route, and core bytes come back
unmodified — because every trust decision stays core-side: the state
creation and hash comparison, the issuer/JWKS verification, the
group-to-role mapping, the session mint, and the audit append all run
inside the core handlers that still serve every forwarded request.

Scope honesty: dest adds no handler validation and keeps no state.
The credential query values cross as opaque bytes — never parsed,
logged, or telemetered — the dest shared key is never injected, the
core's `Location` and `Set-Cookie` answers relay unmodified, and a
forwarded GET is never retried.

Cancel and timeout parity: dest never retries a forwarded login, so a
caller cancel or dispatch-timeout answer is the permanent local 503
`backend_unavailable` with exactly one upstream attempt.

Audit parity: governance audit stays core-side on forwarded requests
(`identity_authentication` / `sso_login_authenticated` and the
denied classifications emit from inside the core callback handler).
This lane re-implements nothing. `scripts/check-parity.sh` gates the
lane with the no-core-import proof (`go list -deps` over the runtime
packages must show no `eshu-hq/eshu` import) alongside the carve-out,
401-boundary, no-retry, redirect-unfollowed, envelope-parity, and
ledger-deficit proofs above.

OpenAPI: the two route entries move as a line-verbatim subset of the
pinned core `routes.go` (the OIDC login + callback entries,
`openapi/paths/auth/oidc_login.go` const `OIDCLogin`), referencing
only the `BrowserSessionResponse` schema the D8 lane already ported —
no new schemas. Assembly adds no new components;
`TestSpec_LocalRefsResolve` fails closed on a dangling `$ref`. The
served dest spec therefore covers exactly the landed families, and no
served key duplicates a relayed route — the GitHub/SAML keys stay
relay-only until their lanes.

## Telemetry family note (disclosed, not a second contract)

Telemetry family is the closed metrics label, not the admission
family: both OIDC routes file under the pre-existing `oidc_login`
prefix (longest-prefix match on `/auth/oidc/`), `via="native"`. The
proof box above is organized per admission family; the ahead-of-relay
test pins the per-label counts.

## Envelope scope-honesty label

Client-visible scope posture for the 2 routes: `tier-C dispatch` —
the destination forwards the request to the authoritative in-tree
route and relays its answer; it does not terminate the route, keep
OIDC state, resolve session/scoped identity, mint credentials or
idempotency values, keep a ledger, append audit, or bound finalize.
Termination and removal readiness are explicitly not claimed.

## Rollback

Additive. Revert this lane's dest commit (or re-pin the prior digest)
and the 2 OIDC routes fall back to the generic byte-relay — answering
dest 401s keyless without ever reaching core — until the lane
returns. No core change, no migration, no flag; no dest-side OIDC,
session, or provider state exists to unwind. Verify by re-running the
lane conformance suite against the reverted tree: the ahead-of-relay
assertions go red (the routes relay again) while every other suite
stays green. No Plan A rehearsal is consumed by this rollback — it
restores a proxy path, not durable state.

## NEVER-list (binding on this lane)

No dest-side state store, replay cache, session mint, metadata
rendering, provider config, client secret, or keyring; no
dest-invented pins, contracts, or codes; no termination claims. The
GitHub and SAML families stay relay-only until their own sequential
PRs with fresh line-verbatim pins.
