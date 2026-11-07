# Setup wizard admission contract

Versioned admission-contract port for the first-run setup wizard
mutations at `POST /api/v0/auth/setup/claim`, `POST
/api/v0/auth/setup/admin`, and `POST /api/v0/auth/setup/mfa` (plus the
public pre-auth `GET /api/v0/auth/setup-state`) (Refs
`eshu-hq/eshu#8872`). The API handler keeps the HTTP shape below and
calls this port; the core-owned Postgres implementation satisfies it.
The wizard mutations persist through explicit core admission; `eshu-api`
never takes a direct table handle.

## Version

`SetupAdmissionContractVersion = "v0"`, with the port defined as:

```go
type SetupAdmission = SetupStore
```

Both lines are vendored verbatim in
`contracts/upstream/setup-wire-shapes.txt`, with the surrounding
contract comment, the `SetupStore` port, and the input/owner/response
shapes. The frozen method set is `frozenSetupAdmission` in
`contracts/upstream/setup_admission_contract_test.go`
(5 methods: `SetupNeeded`, `VerifyBootstrapCredential`,
`ResolveSetupOwner`, `RotateSetupPassword`, `CompleteSetupMFA`).

## HTTP shape

The operator-facing wizard bodies and results, vendored
verbatim in `contracts/upstream/setup-wire-shapes.txt`:

- Claim: submitted plaintext username/password checked against the
  sealed bootstrap credential envelope. A wrong guess answers 401 with
  no signal distinguishing missing, consumed, or mismatched
  credentials, so instance state cannot be probed.
- Admin: new administrator password (server-side 12+ character floor)
  rotated in the same store-layer transaction that revokes the prior
  credential.
- MFA: caller-hashed recovery codes rotated with the MFA factor while
  the bootstrap envelope is permanently consumed in one
  advisory-locked critical section; a concurrent winner answers 409
  for the loser rather than claiming success for unusable codes.
- Every mutating route re-checks `SetupNeeded` and fails closed with
  410 Gone once the wizard is sealed.
- Unconfigured handler (nil or typed-nil store unwrapped to nil): 503.

## Enforcement

`TestSetupAdmissionContractIsVersioned` pins the version and the
frozen method set; the wiring identity test pins the exact injected
port. A renamed, added, removed, or re-signed method fails the frozen
test until the list is deliberately updated; bump
`SetupAdmissionContractVersion` alongside (manually — the bump is
convention, the frozen list is the enforcement), and this repository
records the new pin before consuming it.

## Native dispatch (D10: terminated GET plus tier-C POSTs)

The four setup wizard routes are the arbiter-approved carve-out
(C-c1 APPROVED on `eshu-hq/eshu#9669`, strict; graduated to
D8-grade public-route handling on the satellite D10 wave, arbiter
APPROVED-WITH-CONDITIONS): `api/router.go` registers the explicit
routes outside `RequireAuth`, ahead of the generic proxy —

- `GET /api/v0/auth/setup-state` — terminates dest-side via the
  `SetupNeeded` port (see below),
- `POST /api/v0/auth/setup/claim`,
- `POST /api/v0/auth/setup/admin`,
- `POST /api/v0/auth/setup/mfa` — stay tier-C HTTP write-through
  (see `api/setup.go`),

— each plus the trailing-slash twin on its route's handler, so the
family is never split across dispatch paths. No other `/api/*`
route leaves `RequireAuth` in this lane: the carve-out is exactly
these four. Registration mirrors the core's own posture exactly:
these four paths are the `publicHTTPPaths` bypass entries in
`go/internal/query/auth.go` (see `go/internal/query/setup_handler.go`
`SetupHandler.Mount`, `go/cmd/api/setup.go` `CompleteSetupMFA`
atomic envelope, `go/cmd/api/CUTOVER_OWNERSHIP.md` Setup rows).

### Terminated GET (D10)

The setup-state GET terminates in `api/setup_terminate.go` via the
dest-side `SetupNeeded` port: the single seal-signal method of the
frozen SetupAdmission v0 5-method set
(`contracts/upstream/setup_admission_contract_test.go`), satisfied
in production by a core-fetching adapter over D0-proven transport
(one anonymous GET of the canonical core setup-state route — the
twin spelling never crosses — no retry). Dest synthesizes the 200
from the port (`needs_setup` plus the `bootstrap_mode` echo, same
keys and order as the core `SetupStateResponse`); parity past this
lane is semantic (decoded keys), not byte-for-byte, and the
C-c1 byte-relay proofs for this route are superseded by the
termination proof box in `api/setup_terminate_test.go`:

- Sealed box: the three POSTs keep their relayed 410s (dest
  synthesizes none) while the GET answers the dest-synthesized 200
  with `needs_setup:false`
  (`TestSetupTerminated_SealedStateBox`).
- Fail-closed mapping: any non-200 core answer (the 500 store
  failure, the defensive 410), an unreadable body, or a missing
  `needs_setup` key answers the permanent local 503
  `backend_unavailable` — never a 410, never a relayed failure
  (`TestSetupTerminated_CoreErrorFailsClosed`). A missing seal key
  defaults to 503, never to a sealed `false`.
- Single-hit, no retry: dead-core, caller-cancel, unconfigured
  origin, and sealed/concurrent-409 POST cases each prove exactly
  one upstream hit with no `Retry-After`
  (`TestSetupTerminated_DeadCoreIs503WithoutRetry`,
  `TestSetupTerminated_CanceledContextFailsClosedWithoutRetry`,
  `TestSetupTerminated_UnconfiguredOriginFailsClosed`, plus the
  re-run POST proofs).
- No credential on the fetch: the caller `Authorization` value —
  key or bogus — never crosses (the adapter builds an anonymous
  request), the dest shared key is never injected, and the twin
  spelling fetches the canonical core path
  (`TestSetupTerminated_OpenStateSemanticParity`,
  `TestSetupTerminated_SlashTwinUsesCanonicalFetch`,
  `TestSetupTerminated_SlashTwinSubtree`,
  `TestSetupTerminated_V1AliasTerminates`).
- Body-non-logging and telemetry extension without renames: one
  closed-label log line per request, no credential bytes; the family
  files the existing `setup`/`native` lane with the closed outcome
  vocabulary (`core_ok`, `local_503`) plus the port-fetch core
  latency, and no authorizer judgment (the route is public)
  (`TestSetupNative_CredentialBodyNeverLogged` for the POST secrets,
  `TestSetupTerminated_TelemetryExtended` for the GET).

Dest keeps no setup state on the terminated path either: no
`SetupNeeded` cache (the adapter is request-scoped; repeat GETs
fetch again), no dest-side gate, and no synthesized 410. Fail-closed
is total: unconfigured store answers 503, never 410 — mirroring the
core's own `ready()` 503 when its store is unavailable.

### Tier-C POSTs (unchanged C-c1)

The three credential POSTs stay tier-C HTTP write-through, with
termination forbidden: reaching the seal signal dest-side for a
mutation would require parsing credential bodies, which this lane
never does. Every C-c1 proof re-runs unchanged — the sealed core's
three 410s relay byte-for-byte
(`TestSetupNative_SealedStateSuite`), credential bodies cross as
opaque bytes never parsed, logged, or telemetered
(`TestSetupNative_CredentialBodyNeverLogged`), dest never injects
its shared key (`TestSetupNative_NoSharedKeyInjection`), the core's
`Set-Cookie` and one-time recovery-code answers relay unmodified, a
forwarded POST is never retried
(`TestSetupNative_DeadCoreIs503WithoutRetry`,
`TestSetupNative_SealedCoreSingleHit`,
`TestSetupNative_CanceledContextFailsClosedWithoutRetry`,
`TestSetupNative_WriteThroughRelaysCoreBytes`), and a wrong
bootstrap credential relays the core's single 401 with no added
signal (`TestSetupNative_Indistinguishable401`).

Scope honesty: label the POSTs native-dispatch (HTTP write-through,
tier C) and the GET terminated-via-`SetupNeeded` — not removal-ready
either way. Dest holds no setup gate and resolves no caller
identity on the POSTs; the trust boundary there is the forwarded
bootstrap-credential proof plus the core-side seal, exactly as on
the monolith. This carve-out expires at the flip/removal review:
Setup option-C transport plus carve-out retirement needs a later
arbiter review before core Setup removal.
Proved, not faked: the MFA step's issued browser session is usable
through dest end to end — the `__Host-eshu_session`/`eshu_session`
cookie the core issues is the session-cookie shape the D8 gate
recognizes, so it relays explicitly for core judgment instead of
taking a dest 401
(`TestSetupMFAIssuedSessionUsableThroughD8`). Dest itself still
resolves no session until the terminating resolver lands (arbiter
TBD5-resolver); the live session judgments stay core-side.

Audit parity: governance audit stays core-side on forwarded
requests. The in-tree setup handlers emit audit
(`setup_claimed`, `setup_admin_password_set`, `setup_completed`,
and every denied cause including the concurrent-completion 409)
from inside the core handlers that still run on every forwarded
request, so write-through keeps every audit event core-side with no
gap. Dest emits none. A family whose handlers emit audit must never
be re-implemented dest-side without a versioned audit-append
contract; this lane re-implements no mutation — and the terminated
GET re-implements no audit either, because the core state handler
it replaces emits none (`handleState` runs no `auditSetup`).
`scripts/check-parity.sh` gates the lane with the no-core-import
proof (`go list -deps` over the runtime packages must show no
`eshu-hq/eshu` import) alongside the carve-out, sealed-state, 401,
no-retry, and body-non-logging proofs above.

Telemetry: the family stays `setup` on the existing `native` lane —
extended, not renamed. The terminated GET files the closed outcome
vocabulary (`core_ok` on a resolved seal, `local_503` on any closed
failure) plus the port-fetch core latency; the POSTs keep their
write-through rows. No authorizer judgment is filed on this family
(the routes are public).
(`TestSetupTerminated_TelemetryExtended`).

Latency: `docs/setup-latency.md` records the before/after on all
four routes (stub-core medians, 1.50 bound): the terminated GET is
no slower than the write-through it replaces, and the claim POST is
unchanged transport. `scripts/measure-setup-latency.sh` gates both
legs in CI via `.github/workflows/read-latency.yml`.

OpenAPI: no fragment ported in this lane (satellite-design Decision
5(c) recorded N/A). The core setup fragment
(`go/internal/query/openapi/paths/auth/setup.go`) marks every
operation `"security": []` — public pre-auth documentation — while
the dest-served `GET /api/v0/openapi.json` stays inside
`RequireAuth` like every other dest `/api/*` route. Porting the
fragment now would either gate public-route docs behind the shared
key (misrepresenting the documented security posture) or open the
spec endpoint (widening the public surface beyond the four approved
routes). The served dest spec therefore still covers exactly the
previously landed families; setup routes stay relayed-from-core for
spec purposes permanently: the eshu-hq/eshu-api#44 follow-up resolves
the posture question in favor of core-side (no later lane moves the
fragment, no expiry review). The four core operations all carry
`"security": []`, while `GET /api/v0/openapi.json` stays inside
`RequireAuth` (`api/router.go`, pinned by
`TestOpenAPINative_UnauthenticatedIs401`): porting the fragment would
either gate public-route docs behind the shared key (a reader must
present the key to fetch a document that claims no key is needed) or
open a fifth public spec route beyond the four arbiter-approved
routes (`api/setup.go` route consts plus the terminated GET in
`api/setup_terminate.go`). Wizard clients never read the dest spec —
the POSTs are pure HTTP write-through and the GET synthesizes the
same core shape — so the fragment is discoverability-only. Pinned by
`TestSetupSpecPosture_StaysCoreSide`: the four setup routes answer
without a credential while the spec answers 401 to the same caller,
and the authed document carries no setup path keys and no
`"security"` markers.

Rollback: additive. Revert this lane's dest commit (or re-pin the
prior digest) and the four setup routes fall back to the generic
byte-relay — where they answer dest 401s until the carve-out
returns. The terminated GET specifically restores the C-c1
write-through commit. No core change, no migration, no flag; no
dest-side setup state exists to unwind (the port adapter is
request-scoped).
