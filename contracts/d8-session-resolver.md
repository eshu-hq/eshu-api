# D8 session-resolver tier-C dispatch clones contract

Versioned pin record and clone-lane record for the 22 D8
session/token-lifecycle routes (Refs `eshu-hq/eshu#9669`, part of
`eshu-hq/eshu#6707`). The 18 authed routes register explicitly inside
the D8 credential-class gate ahead of the generic proxy with the
family authorizer; the 4 pre-auth public routes register exactly those
paths outside every gate ahead of the relay. Method, path, query,
headers, and body forward to the same core route; core bytes come
back unmodified and core re-judges every forwarded call as backstop.
Dest holds no session, identity, token, or TOTP state.

## Pin records (satellite-side, D1/D6/D7 precedent)

Pins minted core-side in `eshu-hq/eshu` at `91f2df28dbe9e64ecf5812c5f89a76475a304d84`
(unwired seams; arbiter APPROVED-WITH-CONDITIONS on the satellite D8
wave), recorded here byte-identical under `contracts/upstream/`:

| Core source at pin | Satellite record | sha256 | Treatment |
| --- | --- | --- | --- |
| `go/internal/query/session_admission.go` | `contracts/upstream/session_admission.go` | `11d1dd3415a7a925a181016eb8aefbb4dedccbd542a2798c729972c06062676d` | verbatim |
| `go/internal/query/session_admission_contract_test.go` | `contracts/upstream/session_admission_contract_test.go` | `209a3c2dac30e2060dc8012341f3fa79d2e47ed61f2cc4048645d04a2496b341` | verbatim |

Each file pins all four `*AdmissionContractVersion = "v0"` values plus
the four frozen method sets:

- `SessionResolverAdmission v0`: `ResolveBrowserSession` only (the
  narrow browser-session resolver behind the fail-closed middleware
  path); the CSRF and external-proof-staleness sentinels keep their
  existing 403/401 semantics alongside it.
- `BrowserSessionAdmission v0`: `BrowserSessionStore` (create,
  revoke, workspace switch) plus `ListSessionsBySubject`
  (metadata-only; the session hash computes the Current flag
  server-side and never crosses the boundary).
- `LocalIdentityAdmission v0`: the narrow identity profile lister
  behind the 10 local mounts plus the profile MFA-status read. Inbound
  params mix raw caller proofs (verified core-side) with pre-hashed
  values; outbound rows stay hash/metadata-only. Login client secrets
  and SAML keys are explicitly NOT in this contract (D9 NEVER-list).
- `APITokenAdmission v0`: the narrow generated-token store (create,
  revoke, rotate, metadata-only list). It deliberately overlaps
  LocalIdentityAdmission on the same four methods, mirroring the D1
  overlap precedent: a dest consumer of only this contract can serve
  the four token routes but cannot authenticate, reset, or touch MFA.

`scripts/check-parity.sh` gates both files at `SRC_SHA_D8`
(`91f2df28dbe9e64ecf5812c5f89a76475a304d84`).

## Arbiter binding conditions (satellite D8 wave, APPROVED-WITH-CONDITIONS)

The ruling's clone-lane conditions are binding on this lane, and each
is discharged or recorded below:

1. **Record all four pins byte-identical satellite-side first (D7
   #57 precedent).** Done above: both pin files, hashes,
   check-parity gates, and the line-verbatim pin docs in this
   file. No D8 route consumes an unminted pin.
2. **Per-exact-route C-c1-grade carve-out for the 4 pre-auth public
   routes.** `POST local/login`, `local/invitations/accept`,
   `local/break-glass/session`, and `local/password/rotate` — exactly
   the four paths core serves pre-auth (narrow list pinned
   core-side by
   `TestHandle...PublicPathsAreNarrow` at the D8 pin) — register
   outside every gate, ahead of the relay, on the public
   write-through handler. The carve-out is exactly these four paths:
   no trailing-slash twins (a slash twin would be a subtree
   swallowing `/login/*` past the bypass), no other D8 route leaves
   the gate, the dest shared key is never injected on these routes,
   credential bodies cross as opaque bytes (never parsed, logged, or
   telemetered), POSTs are never retried, and audit stays core-side.
3. **Own per-route review for POST local/bootstrap preserving the
   shared-operator gate.** See the per-route review below: bootstrap
   stays gated (keyless callers get the standing dest 401; keyed
   calls forward once; core's operator seal answers untouched).
4. **Tier-C dispatch only, no termination claims.** The lane forwards
   over HTTP and relays core bytes; dest resolves no session, mints
   no credential, keeps no ledger, appends no audit, and bounds no
   finalize. Termination stays last on D0-proven transport only.
5. **Per-wave proof box + latency.** Zero dest retries (single
   attempt in dest; transport 500s/503s retryable only end-to-end by
   the client, never by dest): every write-through case pins exactly
   one upstream hit, including the core-500, retryable-503,
   dead-core, pre-cancel, and mid-flight-cancel rows.
   Duplicate/no-worse-than-core proof:
   `TestD8Native_DuplicateIsNoWorseThanCore` delivers each route
   twice and pins 2 upstream hits with matching landed statuses.
   Scope-honesty label: tier-C dispatch only, not termination, not
   removal readiness (this document, `api/d8.go`, and the envelope
   below). Latency: `docs/d8-latency.md` (five legs, all ≤ 1.01).
6. **NEVER-list holds.** No dest-side session gate, session state, or
   synthesized codes; no dest-invented contracts or pins; no login
   client secrets or SAML keys dest-owned (D9); no shared-key
   injection on public routes; no POST retries; no parsing, logging,
   or telemetry of credential bodies; no termination claims.

## HTTP shape

- BrowserSessions: create exchanges an already-authenticated explicit
  API credential for a server-managed session (201 sets the
  HttpOnly session cookie plus the readable CSRF cookie; both relay
  unmodified); read returns the cookie-bound auth context (401 when
  the cookie is absent or stale); revoke needs the CSRF header and
  answers 204 clearing both cookies; context PATCH moves the active
  all-scopes session across tenant/workspace (scoped sessions cannot
  switch — core 403s).
- SessionList: metadata-only rows for the caller's own subject
  (issued/last-seen/expiry timestamps, tenant/workspace, Current
  flag); never session hashes or secrets.
- Profile: the caller MFA-status read (never credential material).
- LocalIdentity: bootstrap (gated — see review), login/accept/session
  (public carve-out), invitations create, password reset/rotate,
  MFA-reset, disable, break-glass enable (each gated; core judges
  operator scope per request).
- API tokens: create returns the Bearer [REDACTED] once only (storage persists
  the hash); list is metadata-only; revoke/rotate scope by token id
  with atomic ownership.
- TOTP: begin seals a pending factor and returns the plaintext secret
  once (provisioning URI + manual string); confirm verifies the first
  code before the factor can satisfy logins.
- Every gated route: no key → dest 401; foreign credential (scoped
  token, OIDC, session cookie) → explicit relay so core judges; empty
  configured key → dest 401 like RequireAuth.
- Unconfigured handler (nil core origin): permanent 503
  `backend_unavailable`, never `Retry-After`.

## Per-operation idempotency/audit obligations (from the pin contracts)

Each pin self-gates its operation the same way: until the termination
lane lands the per-request key claimed core-side, the allow/deny audit
event via the audit appender, and the bounded finalize, with fresh
arbiter review, the route stays tier-C dispatch. The clone lane does
not discharge these — it preserves them core-side by forwarding to the
handlers that own them:

- BrowserSessions: dest creates, revokes, switches, and lists
  nothing — the hash comparison, the Current flag, and the cookie
  mint all run core-side. A retried create mints a second session.
- SessionList: metadata-only core-side; hashes never cross because
  dest never reads the rows — it relays the core bytes.
- LocalIdentity/Profile: dest verifies no password, proof, code, or
  secret — raw proofs cross as opaque bytes and core judges them.
- API tokens: dest creates hash-only tokens never — creation,
  revocation, rotation, and the metadata-only list all run core-side.
  Token hashes never cross to dest.

## Per-route review: POST local/bootstrap (shared-operator gate)

Core serves bootstrap gated, not public: `publicHTTPPaths` must not
contain `/api/v0/auth/local/bootstrap` or `/api/v0/auth/local/break-glass`
(pinned core-side), and the handler requires the shared operator
before touching the body (`requireSharedOperator`, then
`VerifyBootstrapCredential` plus the permanent `SetupNeeded` seal —
the seal answers 410/403 once any identity exists, and dest caches
no part of it).

Dest preserves the gate three ways. First, registration: bootstrap
rides the gated half (`d8GatedRoutes`), inside the credential-class
gate — never the public carve-out. Second, admission: a keyless
caller gets the standing dest 401 with zero core contact; a keyed
call forwards exactly once with the caller bytes opaque. Third,
judgment: the operator scope, the bootstrap credential proof, and
the seal all stay core-side — core's 403/410 answers relay untouched
and dest synthesizes none.

`TestD8Native_BootstrapKeepsSharedOperatorGate` pins all three: 401
with zero hits keyless, explicit relay for a foreign bearer, one
forward with opaque bytes keyed, and core's seal answer relayed. Any
future change that moves bootstrap toward the public half must redo
this review and re-run that test; moving it without both is a
contract breach, not a refactor.

## Native dispatch (tier C, D8)

`api/d8.go` registers the 18 gated routes inside the D8
credential-class gate ahead of the generic proxy, plus the 4 public
carve-out routes outside every gate — each with the family authorizer
(shared key ⇒ allow; the deny branch is unreachable until the session
resolver lands and stays a plain deny). Method, path, query, headers
(including the original `Authorization` header on gated routes), and
body forward; core bytes come back unmodified and core re-judges every
forwarded call as backstop.

Scope honesty: dest holds the shared-key identity only, but the gate
— not the authorizer — is what foreign-credential callers meet: scoped
tokens, OIDC bearers, and session-cookie carriers relay explicitly on
the relay lane so core judges the class dest cannot resolve. The deny
branch is the hookup point for the resolver-landing day, not a live
path. The deny-shape test pins the contract for that day; the live
403s and 401s on these routes are core's. Label the family tier-C
dispatch (not termination, not removal readiness) until the dest
session resolver lands with fresh arbiter review.

Cancel and timeout parity: dest never retries a forwarded mutation,
so a caller cancel or dispatch-timeout answer is the permanent local
503 `backend_unavailable` with exactly one upstream attempt
(`TestD8Native_CanceledContextFailsClosedWithoutRetry`, pinned on the
TOTP-confirm route; `TestD8Native_DeadCoreIs503WithoutRetry`, pinned
on the token-create route; both negotiated shapes).

Audit parity: governance audit stays core-side on forwarded requests.
This lane re-implements nothing. `scripts/check-parity.sh` gates the
lane with the no-core-import proof (`go list -deps` over the runtime
packages must show no `eshu-hq/eshu` import) alongside the carve-out,
sealed-state, 401, no-retry, envelope-parity, and ledger-deficit
proofs above.

OpenAPI: the route entries move as line-verbatim subsets of the
pinned core files (`routes.go` for the BrowserSessions, SessionList,
AuthProfile, LocalIdentity, and AdminInvitations-carrying-the-POST
fragments; `tokens.go` and `totp.go` whole-file byte-identical as
the Tokens and TOTP fragments), plus the 11 referenced schemas
ported line-verbatim into the shared components block. The four
invitations POST lines merge into the existing D3 fragment key, so
no served key duplicates another lane's route. Assembly adds no new
components; `TestSpec_LocalRefsResolve` fails closed on a dangling
`$ref`. The served dest spec therefore covers exactly the landed
families, and no served key duplicates a relayed route — except the
two `d8SlashSkip` slash spellings, which stay relayed by mux
necessity and are disclosed as such.

## Telemetry family note (disclosed, not a second contract)

Telemetry family is the closed metrics label, not the admission
family: the invitations create POST and the invitations-accept
carve-out file under the pre-existing `admin_identity` prefix
(longest-prefix match on `/auth/local/invitations`); the other 20 D8
routes file `browser_session` (4), `session_list` (1), `profile`
(1), and `local_identity` (14). The proof box above is organized per
admission family; the ahead-of-relay test pins the per-label counts.

## Envelope scope-honesty label

Client-visible scope posture for the 22 routes: `tier-C dispatch` —
the destination forwards the request to the authoritative in-tree
route and relays its answer; it does not terminate the route,
resolve session/scoped identity, mint credentials or idempotency
values, keep a ledger, append audit, or bound finalize. Termination
and removal readiness are explicitly not claimed.

## Rollback

Additive. Revert this lane's dest commit (or re-pin the prior digest)
and the 22 D8 routes fall back to the generic byte-relay — where the
gated 18 answer dest 401s without a key (foreign-credential callers
lose the explicit relay and get the same 401s) and the public 4
answer dest 401s keyless, until the lane returns. No core change, no
migration, no flag; no dest-side session, identity, token, or TOTP
state exists to unwind. Verify by re-running the lane conformance
suite against the reverted tree: the ahead-of-relay assertions go
red (the routes relay again) while every other suite stays green. No
Plan A rehearsal is consumed by this rollback — it restores a proxy
path, not durable state.
