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

## Native dispatch (tier C-c1: HTTP write-through carve-out)

The four setup wizard routes are the tier C-c1 native-dispatch lane
(arbiter C-c1 APPROVED on `eshu-hq/eshu#9669`, strict carve-out):
`api/setup.go` registers the explicit routes outside `RequireAuth`,
ahead of the generic proxy —

- `GET /api/v0/auth/setup-state`,
- `POST /api/v0/auth/setup/claim`,
- `POST /api/v0/auth/setup/admin`,
- `POST /api/v0/auth/setup/mfa`

— each plus the trailing-slash twin on the same native-public
handler, so the family is never split across dispatch paths. No
other `/api/*` route leaves `RequireAuth` in this lane. Registration
mirrors the core's own posture exactly: these four paths are the
`publicHTTPPaths` bypass entries in `go/internal/query/auth.go`, and
every mutating route's seal re-check (`requireSetupOpen`,
`VerifyBootstrapCredential` proof) still runs core-side on every
forwarded request (see `go/internal/query/setup_handler.go`,
`go/cmd/api/setup.go` `CompleteSetupMFA` atomic envelope,
`go/cmd/api/CUTOVER_OWNERSHIP.md` Setup rows).

Dest keeps no setup state: no `SetupNeeded` gate, cache, or
synthesized 410 — a sealed core's three 410s and the
`needs_setup:false` 200 relay byte-for-byte
(`TestSetupNative_SealedStateSuite`). Credential bodies (bootstrap
username/password, `new_password`, MFA proof) cross as opaque bytes:
never parsed, logged, or telemetered
(`TestSetupNative_CredentialBodyNeverLogged`); dest never injects
its shared key as `Authorization` on these routes
(`TestSetupNative_NoSharedKeyInjection`); the core's `Set-Cookie`
and one-time recovery-code answers relay unmodified; a forwarded
POST is never retried — dead-core, sealed-410, concurrent-409, and
caller-cancel cases each prove exactly one upstream hit
(`TestSetupNative_DeadCoreIs503WithoutRetry`,
`TestSetupNative_SealedCoreSingleHit`,
`TestSetupNative_CanceledContextFailsClosedWithoutRetry`, and the
hit counts inside
`TestSetupNative_WriteThroughRelaysCoreBytes`). A wrong bootstrap
credential relays the core's single 401 shape with no added signal,
so missing, consumed, and mismatched credentials stay
indistinguishable at dest
(`TestSetupNative_Indistinguishable401`).

Scope honesty: label the family native-dispatch (HTTP write-through,
tier C) — not native termination, not removal-ready. Dest holds no
setup gate and resolves no caller identity on these routes; the
trust boundary is the forwarded bootstrap-credential proof plus the
core-side seal, exactly as on the monolith. This carve-out expires
at the flip/removal review: Setup option-C transport plus carve-out
retirement needs a later arbiter review before core Setup removal.
Disclosed, not faked: the MFA step's issued browser session relays
through dest today but is unusable at dest until the dest session
resolver lands (arbiter TBD 5) — a fresh-deployment operator
completes the wizard through dest and then signs in through the
core-served session path until then.

Audit parity: governance audit stays core-side on forwarded
requests. The in-tree setup handlers emit audit
(`setup_claimed`, `setup_admin_password_set`, `setup_completed`,
and every denied cause including the concurrent-completion 409)
from inside the core handlers that still run on every forwarded
request, so write-through keeps every audit event core-side with no
gap. Dest emits none. A family whose handlers emit audit must never
be re-implemented dest-side without a versioned audit-append
contract; this lane re-implements nothing.
`scripts/check-parity.sh` gates the lane with the no-core-import
proof (`go list -deps` over the runtime packages must show no
`eshu-hq/eshu` import) alongside the carve-out, sealed-state, 401,
no-retry, and body-non-logging proofs above.

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
spec purposes until a later lane moves the fragment with a matching
public-spec posture (tracked in eshu-hq/eshu-api#44, due at or before
the flip/removal review). `TestSpec_*` and `TestOpenAPINative_*` are
accordingly untouched.

Rollback: additive. Revert this lane's dest commit (or re-pin the
prior digest) and the four setup routes fall back to the generic
byte-relay — where they answer dest 401s until the carve-out
returns. No core change, no migration, no flag; no dest-side setup
state exists to unwind.
